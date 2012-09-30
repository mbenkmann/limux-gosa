package tests

import (
         "io"
         "net"
         "fmt"
         "time"
         "sync"
         "regexp"
         "syscall"
         "strings"
         "io/ioutil"
         "os"
         "os/exec"
         "container/list"
         
         "../xml"
         "../util"
         "../config"
         "../message"
       )

type queueElement struct {
  // the decoded message. If an error occurred during decoding, this will be
  // <error>Message</error>.
  XML *xml.Hash
  // The time at which the message was received.
  Time time.Time
  // The key with which the message was encrypted.
  Key string
  // IP address of sender.
  SenderIP string 
}

// All incoming messages are appended to the queue. Access protected by queue_mutex
var queue = []*queueElement{}
// queue must only be accessed while holding queue_mutex.
var queue_mutex sync.Mutex

// returns all messages currently in the queue that were received at time t or later.
func get(t time.Time) []*queueElement {
  queue_mutex.Lock()
  defer queue_mutex.Unlock()
  
  result := []*queueElement{}
  for _, q := range queue {
    if !q.Time.Before(t) {
      result = append(result, q)
    }
  }
  
  return result
}

// Waits at most until time t+reply_timeout for a message that is/was received
// at time t or later with the given header and returns that message. If none
// is received within the timeframe, a dummy message is returned.
func wait(t time.Time, header string) *queueElement {
  end_time := t.Add(reply_timeout)
  for ; !time.Now().After(end_time);  {
    queue_mutex.Lock()
    for _, q := range queue {
      if !q.Time.Before(t) && q.XML.Text("header") == header {
        queue_mutex.Unlock()
        return q
      }
    }
    queue_mutex.Unlock()
    time.Sleep(100*time.Millisecond)
  }
  
  return &queueElement{xml.NewHash("xml"), time.Now(), "", "0.0.0.0"}
}



// Regexp for recognizing valid MAC addresses.
var macAddressRegexp = regexp.MustCompile("^[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}$")
// Regexp for recognizing valid <client> elements of e.g. new_server messages.
var clientRegexp = regexp.MustCompile("^[0-9]{1,3}[.][0-9]{1,3}[.][0-9]{1,3}[.][0-9]{1,3}:[0-9]+,[:xdigit:](:[:xdigit:]){5}$")

// true if we're testing gosa-si instead of go-susi
var gosasi bool

// true if the daemon we're testing was launched by us (rather than being prelaunched)
var launched_daemon bool

// max time to wait for a reply
var reply_timeout = 2000 * time.Millisecond

// port the test server listens on for new_server etc.
var listen_port = "18340" 

// host:port address of the test server.
var listen_address string

// keys[0] is the key of the test server started by listener(). The other
// elements are copies of config.ModuleKeys
var keys []string


// Runs the system test.
//  daemon: either "", host:port or the path to a binary. 
//         If "", the default from the config will be used.
//         If host:port, the daemon running at that address will be tested. 
//         Some tests cannot be run in this case.
//         If a program path is used, the program will be launched with
//         -c tempfile  where tempfile is a generated config file that
//         specifies the SystemTest's listener as a peer server, so that
//         e.g. new_server messages can be tested.
//  is_gosasi: if true, test evaluation will be done for gosa-si. This does not
//         affect the tests being done, only whether fails/passes are counted as
//         expected or unexpected.
func SystemTest(daemon string, is_gosasi bool) {
  gosasi = is_gosasi
  launched_daemon = !strings.Contains(daemon,":")
  
  start_time := time.Now()
  
  // start our own "server" that will take messages
  go listener()
  
  // if we got a program path (i.e. not host:port), create config and launch program
  if launched_daemon {
    var confdir string
    config.ServerConfigPath, confdir = createConfigFile()
    //defer os.RemoveAll(confdir)
    defer fmt.Printf("\nLog file directory: %v\n", confdir)
    cmd := exec.Command(daemon, "-c", config.ServerConfigPath, "-vvv")
    cmd.Stderr,_ = os.Create(confdir+"/go-susi+panic.log")
    err := cmd.Start()
    if err != nil { panic(err) }
    defer cmd.Process.Signal(syscall.SIGTERM)
    daemon = ""
  }
  
  // this reads either the default config or the one we created above
  config.ReadConfig()
  config.ReadNetwork()
  
  listen_address = config.IP + ":" + listen_port
  
  if daemon != "" {
    config.ServerSourceAddress = daemon
  }

  // At this point:
  //   listen_address is the address of the test server run by listener()
  //   config.ServerSourceAddress is the address of the go-susi or gosa-si being tested  
  
  
  keys = make([]string, len(config.ModuleKeys)+1)
  for i := range config.ModuleKeys { keys[i+1] = config.ModuleKeys[i] }
  
  // Test if daemon sends us new_server upon startup
  msg := wait(start_time, "new_server")
  
  // Verify that new_server message is according to spec
  nonLaunchedFail(checkTags(msg.XML,"header,new_server,source,target,key,loaded_modules*,client*,macaddress"), "")
  nonLaunchedFail(msg.Key, config.ModuleKey["[ServerPackages]"])
  nonLaunchedFail(strings.Split(msg.XML.Text("source"),":")[0], msg.SenderIP)
  nonLaunchedFail(msg.XML.Text("source"), config.ServerSourceAddress)
  nonLaunchedFail(msg.XML.Text("target"), listen_address)
  nonLaunchedFail(len(msg.XML.Get("new_server"))==1 && msg.XML.Text("new_server")=="", true)
  if launched_daemon {
    siFail(strings.Contains(msg.XML.Text("loaded_modules"), "goSusi"), true)
  }
  nonLaunchedFail(macAddressRegexp.MatchString(msg.XML.Text("macaddress")), true)
  clientsOk := true
  for _, client := range msg.XML.Get("client") {
    if !clientRegexp.MatchString(client) { clientsOk = false }
  }
  check(clientsOk, true)
  
  // send confirm_new_server
  keys[0] = "foo"
  send("[ServerPackages]", hash("xml(header(confirm_new_server)confirm_new_server()source(%v)target(%v)key(%v)loaded_modules(goSusi)macaddress(01:02:03:04:05:06))", listen_address, config.ServerSourceAddress, keys[0]))

// TODO: Testfall für das Löschen eines <periodic> jobs via foreign_job_updates
//       (wegen des Problems dass ein done job mit periodic neu gestartet wird)

// TODO: Testen, dass foreign_job_updates den <siserver> ändern kann.


  // Give daemon time to process data and write logs before sending SIGTERM
  time.Sleep(1*time.Second)
}


// creates a temporary config file and returns the path to it as well as the
// path to the containing temporary directory.
func createConfigFile() (conffile string, confdir string) {
  tempdir, err := ioutil.TempDir("", "system-test-")
  if err != nil { panic(err) }
  fpath := tempdir + "/server.conf"
  ioutil.WriteFile(fpath, []byte(`
[general]
log-file = `+tempdir+`/go-susi.log
pid-file = `+tempdir+`/go-susi.pid

[bus]
enabled = false
key = bus

[server]
max-clients = 10000
ldap-uri = ldap://localhost:7346
ldap-base = c=de
ldap-admin-dn = cn=clientmanager,ou=incoming,c=de
ldap-admin-password = password

[ClientPackages]
key = ClientPackages

[ArpHandler]
enabled = false

[GOsaPackages]
enabled = true
key = GOsaPackages

[ldap]
bind_timelimit = 5

[pam_ldap]
bind_timelimit = 5

[nss_ldap]
bind_timelimit = 5

[ServerPackages]
key = ServerPackages
dns-lookup = true
address = localhost:`+listen_port+`

`), 0644)
  return fpath, tempdir
}

// Checks if x has the given tags and if there is a difference, returns a
// string describing the issue. If everything's okay, returns "".
//  taglist: A comma-separated string of tag names. A tag may be followed by "?"
//        if it is optional, "*" if 0 or more are allowed or "+" if 1 or more
//        are allowed.
//        x is considered okay if it has all non-optional tags from the list and
//        has no unlisted tags and no tags appear more times than permitted. 
func checkTags(x *xml.Hash, taglist string) string {
  tags := map[string]bool{}
  for _, tag := range strings.Split(taglist, ",") {
    switch tag[len(tag)-1] {
      case '?': tag := tag[0:len(tag)-1]
                tags[tag] = true
                if len(x.Get(tag)) > 1 {
                  return(fmt.Sprintf("More than 1 <%v>", tag))
                }
      case '*': tag := tag[0:len(tag)-1]
                tags[tag] = true
      case '+': tag := tag[0:len(tag)-1]
                tags[tag] = true
                if len(x.Get(tag)) == 0 {
                  return(fmt.Sprintf("Missing <%v>", tag))
                }
      default: 
                if len(x.Get(tag)) == 0 {
                  return(fmt.Sprintf("Missing <%v>", tag))
                }
                if len(x.Get(tag)) > 1 {
                  return(fmt.Sprintf("More than 1 <%v>", tag))
                }
                tags[tag] = true
    }
  }
  
  for _, tag := range x.Subtags() {
    if !tags[tag] {
      return(fmt.Sprintf("Unknown <%v>", tag))
    }
  }
  
  return ""
}


// Like check() but expects the test to fail if the daemon was not launched
// by the system test but was started beforehand.
func nonLaunchedFail(x interface{}, expected interface{}) {
  if launched_daemon { 
    checkLevel(x, expected,2) 
  } else {
    checkFailLevel(x, expected,2)
  }
}

// Like check() but expects the test to fail if testing a gosa-si instead of go-susi.
func siFail(x interface{}, expected interface{}) {
  if gosasi {
    checkFailLevel(x, expected,2)
  } else {
    checkLevel(x, expected,2)
  }
}

// sends the xml message x to the gosa-si/go-susi server being tested
// (config.ServerSourceAddress) encrypted with the module key identified by keyid
// (e.g. "[ServerPackages]")
func send(keyid string, x *xml.Hash) {
  util.SendLnTo(config.ServerSourceAddress, message.GosaEncrypt(x.String(), config.ModuleKey[keyid]))
}

// Takes a format string like "xml(foo(%v)bar(%v))" and parameters and creates
// a corresponding xml.Hash.
func hash(format string, args... interface{}) *xml.Hash {
  stack := list.New()
  output := []string{}
  a := 0
  for b := range format {
    switch format[b] {
      case '(' : tag := format[a:b]
                 stack.PushBack(tag)
                 if tag != "" {
                   output = append(output, "<" + tag + ">")
                 }
                 a = b + 1
      case ')' : output = append(output, format[a:b])
                 a = b + 1
                 tag := stack.Back().Value.(string)
                 stack.Remove(stack.Back())
                 if tag != "" {
                   output = append(output, "</" + tag + ">")
                 }
    }
  }
  
  hash, err := xml.StringToHash(fmt.Sprintf(strings.Join(output, ""), args...))
  if err != nil { panic(err) }
  return hash
}

// sets up a listening port that receives messages, decrypts them and
// stores them in the queue.
func listener() {
  listener, err := net.Listen("tcp", ":" + listen_port)
  if err != nil { panic(fmt.Sprintf("Test cannot run. Fatal error: %v", err)) }
  
  for {
    conn, err := listener.Accept()
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v", err)
      continue
    }
    
    go handleConnection(conn)
  }
}

// handles an individual connection received by listener().
func handleConnection(conn net.Conn) {
  defer conn.Close()
  
  senderIP,_,_ := net.SplitHostPort(conn.RemoteAddr().String())
  // translate loopback address to our own external IP  
  if senderIP == "127.0.0.1" { senderIP = config.IP }
  
  
  var buf = make([]byte, 65536)
  i := 0
  n := 1
  var err error
  for {
    // It's intentional that the timeout is short and the loop
    // aborts with an error on timeout even if a whole message was
    // read. The protocol says that it is the responsibility of the
    // one sending the message to close the connection. This indirectly
    // tests this requirement by not accepting even well-formed messages
    // whose connections are not closed.
    conn.SetReadDeadline(time.Now().Add(2*time.Second))
    n, err = conn.Read(buf[i:])
    i += n
    
    if err != nil && err != io.EOF { 
      break 
    }
    if err == io.EOF {
      err = nil
      break
    }
    if n == 0 && err == nil {
      err = fmt.Errorf("Read 0 bytes but no error reported")
      break
    }
    
    if i == len(buf) {
      buf_new := make([]byte, len(buf)+65536)
      copy(buf_new, buf)
      buf = buf_new
    }
  }
  
  if err == nil && (i < 2 || buf[i-1] != '\n' || buf[i-2] != '\r') {
    err = fmt.Errorf("Message not terminated by CRLF")
  }
  
  msg := queueElement{}
  
  if err == nil {
    decrypted := ""
    for _, msg.Key = range keys {
      decrypted = message.GosaDecrypt(string(buf[0:i]), msg.Key)
      if decrypted != "" { break }
    }
    if decrypted == "" {
      err = fmt.Errorf("Could not decrypt message")
    } else {
      msg.XML, err = xml.StringToHash(decrypted)
    }
  }
  
  if err != nil {
    msg.XML = hash("error(%v)", err)
  }

  // if we get a new_server or confirm_new_server message, update our server key  
  header := msg.XML.Text("header")
  if header == "new_server" || header == "confirm_new_server" {
    keys[0] = msg.XML.Text("key")
  }
  
  msg.Time = time.Now()
  msg.SenderIP = senderIP
  
  queue_mutex.Lock()
  defer queue_mutex.Unlock()
  queue = append(queue, &msg)
}

