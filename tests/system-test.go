package tests

import (
         "io"
         "net"
         "fmt"
         "time"
         "sync"
         "bytes"
         "regexp"
         "runtime"
         "syscall"
         "strings"
         "io/ioutil"
         "os"
         "os/exec"
         "container/list"
         "encoding/base64"
         
         "../xml"
         "../util"
         "../util/deque"
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

// sends the xml message x to the gosa-si/go-susi server being tested
// (config.ServerSourceAddress) encrypted with the module key identified by keyid
// (e.g. "[ServerPackages]"). Use keyid "" to select the server key exchanged via
// new_server/confirm_new_server
// If x does not have <target> and/or <source> elements, they will be added
// with the values config.ServerSourceAddress and listen_address respectively.
//
// ATTENTION! This method does not wait for a reply from the server.
// Therefore you will usually need to wait a little for the server to have
// processed the message before checking for effects.
func send(keyid string, x *xml.Hash) {
  var key string
  if keyid == "" { key = keys[0] } else 
  { 
    key = config.ModuleKey[keyid] 
  }
  if x.First("source") == nil {
    x.Add("source", listen_address)
  }
  if x.First("target") == nil {
    x.Add("target", config.ServerSourceAddress)
  }
  util.SendLnTo(config.ServerSourceAddress, message.GosaEncrypt(x.String(), key), config.Timeout)
}

// Sends a GOSA message to the server being tested and
// returns the reply.
// Automatically adds <header>gosa_typ</header> (unless typ starts with "job_" 
// or "gosa_" in which case <header>typ</header> will be used.)
// and <source>GOSA</source> as well as <target>GOSA</target> (unless a subelement
// of the respective name is already present).
func gosa(typ string, x *xml.Hash) *xml.Hash {
  if !strings.HasPrefix(typ, "gosa_") && !strings.HasPrefix(typ, "job_") {
    typ = "gosa_" + typ
  }
  if x.First("header") == nil {
    x.Add("header", typ)
  }
  if x.First("source") == nil {
    x.Add("source", "GOSA")
  }
  if x.First("target") == nil {
    x.Add("target", "GOSA")
  }
  conn, err := net.Dial("tcp", config.ServerSourceAddress)
  if err != nil {
    util.Log(0, "ERROR! Dial: %v", err)
    return xml.NewHash("error")
  }
  defer conn.Close()
  util.SendLn(conn, message.GosaEncrypt(x.String(), config.ModuleKey["[GOsaPackages]"]), config.Timeout)
  reply := message.GosaDecrypt(util.ReadLn(conn, config.Timeout), config.ModuleKey["[GOsaPackages]"])
  x, err = xml.StringToHash(reply)
  if err != nil { x = xml.NewHash("error") }
  return x
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

// the listener of the test server
var listener net.Listener

// Elements of type net.Conn for all current active incoming connections
// handled by handleConnection()
var active_connections = deque.New()

// keys[0] is the key of the test server started by listen(). The other
// elements are copies of config.ModuleKeys
var keys []string

// start time of SystemTest()
var StartTime time.Time

type Job struct {
  Type string
  MAC string
  Plainname string
  Timestamp string
  Periodic string
}

// returns Type with the "job_" removed.
func (self *Job) Trigger() string {
  return self.Type[4:]
}

var Jobs = []Job{
{"job_trigger_action_wake","01:02:03:04:05:06","systest1","20990914131742","7_days"},
{"job_trigger_action_lock","11:22:33:44:55:6F","systest2","20770101000000","1_minutes"},
{"job_trigger_action_wake","77:66:55:44:33:2a","systest3","20660906164734","none"},
}

// Runs the system test.
//  daemon: either "", host:port or the path to a binary. 
//         If "", the default from the config will be used.
//         If host:port, the daemon running at that address will be tested. 
//         Some tests cannot be run in this case.
//         If a program path is used, the program will be launched with
//         -f -c tempfile  where tempfile is a generated config file that
//         specifies the SystemTest's listener as a peer server, so that
//         e.g. new_server messages can be tested.
//  is_gosasi: if true, test evaluation will be done for gosa-si. This does not
//         affect the tests being done, only whether fails/passes are counted as
//         expected or unexpected.
func SystemTest(daemon string, is_gosasi bool) {
  gosasi = is_gosasi
  launched_daemon = !strings.Contains(daemon,":")
  if gosasi { reply_timeout *= 10 }
  
  StartTime = time.Now()
  
  // start our own "server" that will take messages
  listen()
  
  config.ReadNetwork()
  listen_address = config.IP + ":" + listen_port
  
  // if we got a program path (i.e. not host:port), create config and launch program
  if launched_daemon {
    //first launch the test ldap server
    cmd := exec.Command("/usr/sbin/slapd","-f","./slapd.conf","-h","ldap://127.0.0.1:20088","-d","0")
    cmd.Dir = "./testdata"
    err := cmd.Start()
    if err != nil { panic(err) }
    defer cmd.Process.Signal(syscall.SIGTERM)
    
    var confdir string
    config.ServerConfigPath, confdir = createConfigFile()
    //defer os.RemoveAll(confdir)
    defer fmt.Printf("\nLog file directory: %v\n", confdir)
    cmd = exec.Command(daemon, "-f", "-c", config.ServerConfigPath, "-vvvvvv")
    cmd.Stderr,_ = os.Create(confdir+"/go-susi+panic.log")
    err = cmd.Start()
    if err != nil { panic(err) }
    defer cmd.Process.Signal(syscall.SIGTERM)
    daemon = ""
  }
  
  // this reads either the default config or the one we created above
  config.ReadConfig()
  
  config.Timeout = reply_timeout
  
  if daemon != "" {
    config.ServerSourceAddress = daemon
  }

  // At this point:
  //   listen_address is the address of the test server run by listen()
  //   config.ServerSourceAddress is the address of the go-susi or gosa-si being tested  
  
  
  keys = make([]string, len(config.ModuleKeys)+1)
  for i := range config.ModuleKeys { keys[i+1] = config.ModuleKeys[i] }
  keys[0] = "none"
  
  if launched_daemon {
    check_new_server_on_startup(Jobs[0])
  } else {
    // We need this in the database for the later test whether go-susi reacts
    // to new_server by sending its jobdb. This same call is contained in
    // check_new_server_on_startup()
    trigger_first_test_job(Jobs[0])
  }
  
  // Send new_server and check that we receive confirm_new_server in response
  t0 := time.Now()
  keys = append(keys, keys[0])
  keys[0] = "new_server_key"
  send("[ServerPackages]", hash("xml(header(new_server)new_server()key(%v)loaded_modules(goSusi)macaddress(00:00:00:00:00:00))", keys[0]))
  msg := wait(t0, "confirm_new_server")
  check(checkTags(msg.XML,"header,confirm_new_server,source,target,key,loaded_modules*,client*,macaddress"), "")
  check(msg.Key, config.ModuleKey["[ServerPackages]"])
  check(strings.Split(msg.XML.Text("source"),":")[0], msg.SenderIP)
  check(msg.XML.Text("source"), config.ServerSourceAddress)
  check(msg.XML.Text("target"), listen_address)
  check(msg.XML.Text("key"), "new_server_key")
  check(len(msg.XML.Get("confirm_new_server"))==1 && msg.XML.Text("confirm_new_server")=="", true)
  siFail(strings.Contains(msg.XML.Text("loaded_modules"), "goSusi"), true)
  check(macAddressRegexp.MatchString(msg.XML.Text("macaddress")), true)
  clientsOk := true
  for _, client := range msg.XML.Get("client") {
    if !clientRegexp.MatchString(client) { clientsOk = false }
  }
  check(clientsOk, true)
  
  // go-susi also sends foreign_job_updates in response to new_server
  msg = wait(t0, "foreign_job_updates")
  siFail(checkTags(msg.XML, "header,source,target,answer1,sync?"), "")
  if checkTags(msg.XML, "header,source,target,answer1") == "" {
    check_foreign_job_updates(msg, "new_server_key", Jobs[0].Plainname, Jobs[0].Periodic, "waiting", "none", Jobs[0].MAC, Jobs[0].Trigger(), Jobs[0].Timestamp)
  }

  t0 = time.Now()
  x := gosa(Jobs[1].Type, hash("xml(target(%v)timestamp(%v)macaddress(%v)periodic(%v))",Jobs[1].MAC, Jobs[1].Timestamp, Jobs[1].MAC, Jobs[1].Periodic))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  check(x.Text("header"), "answer")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  check(x.Text("answer1"), "0")
  
  msg = wait(t0, "foreign_job_updates")
  check_foreign_job_updates(msg, "new_server_key", Jobs[1].Plainname, Jobs[1].Periodic, "waiting", "none",Jobs[1].MAC, Jobs[1].Trigger(), Jobs[1].Timestamp)
  
  check_connection_drop_on_error1()
  check_connection_drop_on_error2()
  
  check_multiple_requests_over_one_connection()
  
  // query for trigger_action_lock on test_mac2
  x = gosa("query_jobdb", hash("xml(where(clause(phrase(macaddress(%v)))))", Jobs[1].MAC))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  check(x.Text("header"), "query_jobdb")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  a := x.First("answer1")
  check(a != nil, true)
  if a != nil {
    check_answer(a, Jobs[1].Plainname, "none", "waiting", config.ServerSourceAddress, Jobs[1].MAC, Jobs[1].Timestamp, Jobs[1].Periodic, Jobs[1].Trigger())
  }
  
  // query for trigger_action_wake on test_mac (via "ne Jobs[1].MAC")
  x = gosa("query_jobdb", hash("xml(where(clause(connector(and)phrase(operator(ne)macaddress(%v)))))", Jobs[1].MAC))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  check(x.Text("header"), "query_jobdb")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  a = x.First("answer1")
  check(a != nil, true)
  if a != nil {
    check_answer(a, Jobs[0].Plainname, "none", "waiting", config.ServerSourceAddress, Jobs[0].MAC, Jobs[0].Timestamp, Jobs[0].Periodic, Jobs[0].Trigger())
  }
  
  // delete trigger_action_wake on test_mac (via "ne test_mac2" plus redundant "like ...")
  t0 = time.Now()
  x = gosa("delete_jobdb_entry", hash("xml(where(clause(connector(and)phrase(operator(like)headertag(trigger_action_%%))phrase(operator(ne)macaddress(%v)))))", Jobs[1].MAC))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  check(x.Text("header"), "answer")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  check(x.Text("answer1"), "0")
  
  // query complete jobdb. The reply should only be one remaining job.
  // Depending on timing this may fail on gosa-si because it allows jobs to
  // be observed in "done" status. However on go-susi this should always
  // give the expected result.
  x = gosa("query_jobdb", hash("xml(where())"))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  check(x.Text("header"), "query_jobdb")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  a = x.First("answer1")
  if a == nil { a = xml.NewHash("answer1") }
  old_job := a.Clone()
  check(a != nil, true)
  if a != nil {
    check_answer(a, Jobs[1].Plainname, "none", "waiting", config.ServerSourceAddress, Jobs[1].MAC, Jobs[1].Timestamp, Jobs[1].Periodic, Jobs[1].Trigger())
  }
  
  // check for foreign_job_updates with status "done"
  msg = wait(t0, "foreign_job_updates")
  check_foreign_job_updates(msg, keys[0], Jobs[0].Plainname, "", "done", "none", Jobs[0].MAC, Jobs[0].Trigger(), Jobs[0].Timestamp)
  
  // Send foreign_job_updates with following changes:
  //   change <progress> of the existing job
  //   add a new job
  old_job.FirstOrAdd("progress").SetText("42")
  new_job := hash("answer2(plainname(%v)progress(none)status(waiting)siserver(localhost)modified(1)macaddress(%v)targettag(%v)timestamp(%v)id(66)headertag(%v)result(none))",Jobs[2].Plainname,Jobs[2].MAC,Jobs[2].MAC,Jobs[2].Timestamp,Jobs[2].Trigger())
  new_job.FirstOrAdd("xmlmessage").SetText(base64.StdEncoding.EncodeToString([]byte(hash("xml(header(%v)source(GOSA)target(%v)timestamp(%v)macaddress(%v))",Jobs[2].Type,Jobs[2].MAC,Jobs[2].Timestamp,Jobs[2].MAC).String())))
  x = hash("xml(header(foreign_job_updates)source(%v)target(%v))",listen_address,config.ServerSourceAddress)
  x.AddClone(old_job)
  x.AddClone(new_job)
  send("", x)
  
  // Wait for message to be processed, because send() doesn't wait.
  time.Sleep(reply_timeout)
  
  // Check the jobdb for the above changes
  x = gosa("query_jobdb", hash("xml(where())"))
  check(checkTags(x, "header,source,target,answer1,answer2,session_id?"),"")
  check(x.Text("header"), "query_jobdb")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  a1 := x.First("answer1")
  a2 := x.First("answer2")
  if a1 != nil && a2 != nil{
    if a1.Text("plainname") == Jobs[2].Plainname { // make sure a1 is the old and a2 is new job
      a1, a2 = a2, a1
    }
    
    check_answer(a1, Jobs[1].Plainname, "42", "waiting", config.ServerSourceAddress, Jobs[1].MAC, Jobs[1].Timestamp, Jobs[1].Periodic, Jobs[1].Trigger())
    
    check_answer(a2, Jobs[2].Plainname, "none", "waiting", listen_address, Jobs[2].MAC, Jobs[2].Timestamp, Jobs[2].Periodic, Jobs[2].Trigger())
  }

  // Shut down our test server and active connections
  listener.Close()
  for {
    connection := active_connections.PopAt(0)
    if connection == nil { break }
    connection.(net.Conn).Close()
  }
  // Wait a little so that the testee notices
  time.Sleep(1*time.Second)
  // Now test that our test server's jobs are marked as state "error" in query_jobdb
  x = gosa("query_jobdb", hash("xml(where(clause(phrase(siserver(%v)))))",listen_address))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  a2 = x.First("answer1")
  check_answer(a2, Jobs[2].Plainname, "none", "error", listen_address, Jobs[2].MAC, Jobs[2].Timestamp, Jobs[2].Periodic, Jobs[2].Trigger())
  
  // Restart our test server
  t0 = time.Now()
  listen()
  
  // Wait for the peer to re-establish the connection
  for i:=0; i<30 && active_connections.IsEmpty(); i++ {
    time.Sleep(1*time.Second)
  }
  
  // Check for the <sync>all</sync> message we should get after the connection
  // is re-established
  for _,msg = range get(t0) {
    if msg.XML.Text("sync") == "all" { break }
  }
  check(msg.XML.Text("sync"),"all")
  check_foreign_job_updates(msg, keys[0], Jobs[1].Plainname, Jobs[1].Periodic, "waiting", "42", Jobs[1].MAC, Jobs[1].Trigger(), Jobs[1].Timestamp)
  
  // Now test that our test server's jobs are no longer in error state in query_jobdb
  x = gosa("query_jobdb", hash("xml(where(clause(phrase(siserver(%v)))))",listen_address))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  a2 = x.First("answer1")
  check_answer(a2, Jobs[2].Plainname, "none", "waiting", listen_address, Jobs[2].MAC, Jobs[2].Timestamp, Jobs[2].Periodic, Jobs[2].Trigger())

  // clear jobdb  
  x = gosa("delete_jobdb_entry", hash("xml(where())"))
  
  // Because the above delete affects a job belonging to the test server,
  // go-susi doesn't delete it directly but forwards the request to the
  // test server. Wait a little to make sure the communication is finished.
  time.Sleep(reply_timeout)
  
  // now add 2 jobs that are the same in all respects except the timestamp
  x = gosa("job_trigger_action_lock", hash("xml(target(%v)timestamp(%v)macaddress(%v)periodic(%v))",Jobs[1].MAC, Jobs[0].Timestamp, Jobs[1].MAC, Jobs[1].Periodic))
  check(x.Text("answer1"), "0")
  x = gosa("job_trigger_action_lock", hash("xml(target(%v)timestamp(%v)macaddress(%v)periodic(%v))",Jobs[1].MAC, Jobs[1].Timestamp, Jobs[1].MAC, Jobs[1].Periodic))
  check(x.Text("answer1"), "0")
  
  x = gosa("query_jobdb", hash("xml(where())"))
  siFail(checkTags(x, "header,source,target,answer1,answer2,session_id?"),"")
  a1 = x.First("answer1")
  a2 = x.First("answer2")
  if a1 != nil && a2 != nil{
    if a1.Text("timestamp") == Jobs[1].Timestamp { // make sure a1 has Jobs[0].Timestamp
      temp := a1
      a1 = a2
      a2 = temp
    }
    
    check_answer(a1, Jobs[1].Plainname, "none", "waiting", config.ServerSourceAddress, Jobs[1].MAC, Jobs[0].Timestamp, Jobs[1].Periodic, Jobs[1].Trigger())
    check_answer(a2, Jobs[1].Plainname, "none", "waiting", config.ServerSourceAddress, Jobs[1].MAC, Jobs[1].Timestamp, Jobs[1].Periodic, Jobs[1].Trigger())
  }

  
  
// TODO: Testfall für das Löschen eines <periodic> jobs via foreign_job_updates (z.B.
//       den oben hinzugefügten Test-Job)
//       (wegen des Problems dass ein done job mit periodic neu gestartet wird)
//       Komplementären Testfall für ein normales "done" eines periodic Jobs,
//       bei dem der Job tatsächlich neu gestartet werden soll.

// TODO: weiter oben bei test_mac und test_name Daten aus den LDAP-Testdaten
// eintragen

  // Give daemon time to process data and write logs before sending SIGTERM
  time.Sleep(reply_timeout)
}

func check_multiple_requests_over_one_connection() {
  get_all_jobs := hash("xml(header(gosa_query_jobdb)source(GOSA)target(GOSA)where())")
  
  conn, err := net.Dial("tcp", config.ServerSourceAddress)
  check(err, nil)
  if err != nil { return }
  defer conn.Close()
  
  for i :=0 ; i < 3; i++ {
    util.SendLn(conn, "\n\n\r\r\r\n\r\r\n", config.Timeout) // test that empty lines don't hurt
    util.SendLn(conn, message.GosaEncrypt(get_all_jobs.String(), config.ModuleKey["[GOsaPackages]"]), config.Timeout)
    reply := message.GosaDecrypt(util.ReadLn(conn, config.Timeout), config.ModuleKey["[GOsaPackages]"])
    x, err := xml.StringToHash(reply)
    check(err, nil)
    check(checkTags(x,"header,source,target,session_id?,answer1,answer2"), "")
  }
}


// Check that go-susi forcibly closes the connection if it encounters an
// unknown <header>.
func check_connection_drop_on_error1() {
  x := hash("xml(header(gibberish)source(GOSA)target(GOSA))")
  
  conn, err := net.Dial("tcp", config.ServerSourceAddress)
  check(err, nil)
  if err != nil { return }
  defer conn.Close()
  
  util.SendLn(conn, message.GosaEncrypt(x.String(), config.ModuleKey["[GOsaPackages]"]), config.Timeout)
  reply := message.GosaDecrypt(util.ReadLn(conn, config.Timeout), config.ModuleKey["[GOsaPackages]"])
  x, err = xml.StringToHash(reply)
  check(err, nil)
  
  check(len(x.Text("error_string")) > 0, true)
  
  // Server should drop connection immediately after sending error reply.
  // Give it just a little bit of time.
  time.Sleep(1 * time.Second)
  
  t0 := time.Now()
  conn.SetDeadline(time.Now().Add(5*time.Second))
  _, err = conn.Read(make([]byte, 1)) // should terminate with error immediately
  check(err, io.EOF)
  check(time.Since(t0) < 1 * time.Second, true)
}

// Check that go-susi forcibly closes the connection if it encounters an 
// undecryptable message.
func check_connection_drop_on_error2() {
  x := hash("xml(header(gibberish)source(GOSA)target(GOSA))")
  
  conn, err := net.Dial("tcp", config.ServerSourceAddress)
  check(err, nil)
  if err != nil { return }
  defer conn.Close()
  
  util.SendLn(conn, message.GosaEncrypt(x.String(), "wuseldusel"), config.Timeout)
  reply := util.ReadLn(conn, config.Timeout)
  x, err = xml.StringToHash(reply)
  check(err, nil)
  
  check(len(x.Text("error_string")) > 0, true)
  
  // Server should drop connection immediately after sending error reply.
  // Give it just a little bit of time.
  time.Sleep(1 * time.Second)
  
  t0 := time.Now()
  conn.SetDeadline(time.Now().Add(5*time.Second))
  _, err = conn.Read(make([]byte, 1)) // should terminate with error immediately
  check(err, io.EOF)
  check(time.Since(t0) < 1 * time.Second, true)
}

// Checks that on startup go-susi sends new_server to the test server listed
// in [ServerPackages]/address
func check_new_server_on_startup(job Job) {
  test_mac := job.MAC
  test_name:= job.Plainname
  test_timestamp:=job.Timestamp
  // Test if daemon sends us new_server upon startup
  msg := wait(StartTime, "new_server")
  
  // gosa-si ignored our address= entry because it thinks it refers to
  // itself because GosaSupportDaemon.pm:is_local does not consider the port
  siFail(len(msg.XML.Subtags()) > 0, true)
  if len(msg.XML.Subtags()) > 0 {

    // Verify that new_server message is according to spec
    check(checkTags(msg.XML,"header,new_server,source,target,key,loaded_modules*,client*,macaddress"), "")
    check(msg.Key, config.ModuleKey["[ServerPackages]"])

    siFail(strings.Split(msg.XML.Text("source"),":")[0], msg.SenderIP)
    siFail(msg.XML.Text("source"), config.ServerSourceAddress)
    siFail(msg.XML.Text("target"), listen_address)

    check(len(msg.XML.Get("new_server"))==1 && msg.XML.Text("new_server")=="", true)
    siFail(strings.Contains(msg.XML.Text("loaded_modules"), "goSusi"), true)

    check(macAddressRegexp.MatchString(msg.XML.Text("macaddress")), true)
    clientsOk := true
    for _, client := range msg.XML.Get("client") {
      if !clientRegexp.MatchString(client) { clientsOk = false }
    }
    check(clientsOk, true)

    // send confirm_new_server with a different key to check that c_n_s does not
    // need to use the same key as new_server
    keys = append(keys, keys[0])
    keys[0] = "confirm_new_server_key"
    send("[ServerPackages]", hash("xml(header(confirm_new_server)confirm_new_server()key(%v)loaded_modules(goSusi)macaddress(01:02:03:04:05:06))",keys[0]))

    // Wait a little to make sure the server has processed our confirm_new_server
    // and activated our provided key
    time.Sleep(reply_timeout)

    // send job_trigger_action to check that we get a foreign_job_updates encrypted 
    // with the key we set via confirm_new_server above
    t0 := time.Now()
    
    trigger_first_test_job(Jobs[0])

    msg = wait(t0, "foreign_job_updates")
    check_foreign_job_updates(msg, "confirm_new_server_key", test_name, "7_days", "waiting", "none", test_mac, "trigger_action_wake", test_timestamp)
  }
}

func trigger_first_test_job(job Job) {
  typ:=job.Type
  test_mac:=job.MAC
  test_timestamp:=job.Timestamp
  x := gosa(typ, hash("xml(target(%v)timestamp(%v)macaddress(%v)periodic(7_days))",test_mac, test_timestamp, test_mac))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  check(x.Text("header"), "answer")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  check(x.Text("answer1"), "0")
}

func check_foreign_job_updates(msg *queueElement, test_key, test_name, test_periodic, test_status, test_progress, test_mac, action, test_timestamp string) {
  _, file, line, _ := runtime.Caller(1)
  file = file[strings.LastIndex(file, "/")+1:]
  fmt.Printf("== check_foreign_job_updates sub-tests (%v:%v) ==\n", file, line)
      
  check(checkTags(msg.XML, "header,source,target,answer1,sync?"), "")
  check(msg.Key, test_key)
  check(msg.XML.Text("header"), "foreign_job_updates")
  siFail(msg.XML.Text("source"), config.ServerSourceAddress)
  siFail(msg.XML.Text("target"), listen_address)
  siFail(msg.XML.Text("sync") == "ordered" || msg.XML.Text("sync") == "all", true)
  job := msg.XML.First("answer1")
  check(job != nil, true)
  if job != nil {
    check(checkTags(job, "plainname,periodic?,progress,status,siserver,modified,targettag,macaddress,timestamp,id,original_id?,headertag,result,xmlmessage"),"")
    
    // plainname is optional but when it is supplied, make sure it's correct
    plainname := job.Text("plainname")
    if plainname != "" && plainname != "none" {
      check(job.Text("plainname"), test_name)
    }
    
    peri := job.Text("periodic")
    if peri == "none" { peri = "" }
    siFail(peri, test_periodic)
    check(job.Text("progress"), test_progress)
    check(job.Text("status"), test_status)
    siFail(job.Text("siserver"), config.ServerSourceAddress)
    check(job.Text("targettag"), test_mac)
    check(job.Text("macaddress"), test_mac)
    check(job.Text("timestamp"), test_timestamp)
    check(job.Text("headertag"), action)
    check(job.Text("result"), "none")
    
    // The strange Join/Fields combo gets rid of the whitespace which gosa-si introduces into xmlmessage
    xmlmessage_txt := strings.Join(strings.Fields(job.Text("xmlmessage")),"")
    siFail(xmlmessage_txt, job.Text("xmlmessage"))
    decoded, _ := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(job.Text("xmlmessage")),""))
    xmlmessage, err := xml.StringToHash(string(decoded))
    check(err, nil)
    check(checkTags(xmlmessage, "header,source,target,timestamp,periodic?,macaddress"), "")
    check(xmlmessage.Text("header"), "job_" + action)
    check(xmlmessage.Text("source"), "GOSA")
    check(xmlmessage.Text("target"), test_mac)
    check(xmlmessage.Text("timestamp"), test_timestamp)
    if test_periodic != "" { // do not test if periodic="" because this may be due to a delete in which case the xmlmessage doesn't match
      check(xmlmessage.Text("periodic"), test_periodic)
    }
    check(xmlmessage.Text("macaddress"), test_mac)
  }
}

func check_answer(a *xml.Hash, name, progress, status, siserver, mac, timestamp, periodic, headertag string) {
  _, file, line, _ := runtime.Caller(1)
  file = file[strings.LastIndex(file, "/")+1:]
  fmt.Printf("== check_answer sub-tests (%v:%v) ==\n", file, line)
      
  check(checkTags(a, "plainname,periodic?,progress,status,siserver,modified,targettag,macaddress,timestamp,id,original_id?,headertag,result,xmlmessage"),"")
  check(a.Text("plainname"), name)
  check(a.Text("progress"), progress)
  check(a.Text("status"), status)
  siFail(a.Text("siserver"), siserver)
  check(a.Text("targettag"), mac)
  check(a.Text("macaddress"), mac)
  check(a.Text("timestamp"), timestamp)
  peri := a.Text("periodic")
  if peri == "" { peri = "none" }
  if periodic == "" { periodic = "none" }
  siFail(peri, periodic)
  check(a.Text("headertag"), headertag)
  if status != "error" { // if status==error, result contains plaintext message
    check(a.Text("result"), "none")
  }
  
  // The strange Join/Fields combo gets rid of the whitespace which gosa-si introduces into xmlmessage
  xmlmessage_txt := strings.Join(strings.Fields(a.Text("xmlmessage")),"")
  siFail(xmlmessage_txt, a.Text("xmlmessage"))
  decoded, _ := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(a.Text("xmlmessage")),""))
  xmlmessage, err := xml.StringToHash(string(decoded))
  check(err, nil)
  if err == nil {
    check(checkTags(xmlmessage, "header,source,target,timestamp,periodic?,macaddress"), "")
    check(xmlmessage.Text("header"), "job_" + headertag)
    check(xmlmessage.Text("source"), "GOSA")
    check(xmlmessage.Text("target"), mac)
    check(xmlmessage.Text("timestamp"), timestamp)
    peri = xmlmessage.Text("periodic")
    if peri == "" { peri = "none" }
    check(peri, periodic)
    check(xmlmessage.Text("macaddress"), mac)
  }
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
ldap-uri = ldap://127.0.0.1:20088
ldap-base = o=go-susi,c=de
ldap-admin-dn = cn=admin,o=go-susi,c=de
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
dns-lookup = false
address = ` +listen_address+`

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
func listen() {
  var err error
  listener, err = net.Listen("tcp", ":" + listen_port)
  if err != nil { panic(fmt.Sprintf("Test cannot run. Fatal error: %v", err)) }
  
  go func() {
    defer listener.Close()
    
    for {
      conn, err := listener.Accept()
      if err != nil { return }
      
      go handleConnection(conn)
    }
  }()
}

// handles an individual connection received by listen().
func handleConnection(conn net.Conn) {
  defer conn.Close()
  active_connections.Push(conn)
  defer active_connections.Remove(conn)
  
  senderIP,_,_ := net.SplitHostPort(conn.RemoteAddr().String())
  // translate loopback address to our own external IP  
  if senderIP == "127.0.0.1" { senderIP = config.IP }
  
  conn.(*net.TCPConn).SetKeepAlive(true)
  
  var err error
  
  var buf = make([]byte, 65536)
  i := 0
  n := 1
  for n != 0 {
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

    // Find complete lines terminated by '\n' and process them.
    for start := 0;; {
      eol := bytes.IndexByte(buf[start:i], '\n')
      
      // no \n found, go back to reading from the connection
      // after purging the bytes processed so far
      if eol < 0 {
        copy(buf[0:], buf[start:i]) 
        i -= start
        break
      }
      
      // process the message and get a reply (if applicable)
      reply := processMessage(string(buf[start:start+eol]), senderIP)
      if reply != "" { util.SendLn(conn, reply, 5*time.Second) }
      start += eol+1
    }
  }
  
  if  i != 0 {
    err = fmt.Errorf("ERROR! Incomplete message (i.e. not terminated by \"\\n\") of %v bytes: %v", i, buf[0:i])
  }
  
  if err != nil {
    msg := queueElement{}
    msg.XML = hash("error(%v)", err)
    msg.Time = time.Now()
    msg.SenderIP = senderIP
  
    queue_mutex.Lock()
    defer queue_mutex.Unlock()
    queue = append(queue, &msg)
  }
}

func processMessage(str string, senderIP string) string {
  var err error
  msg := queueElement{}
  
  decrypted := ""
  for _, msg.Key = range keys {
    decrypted = message.GosaDecrypt(str, msg.Key)
    if decrypted != "" { break }
  }
  if decrypted == "" {
    err = fmt.Errorf("Could not decrypt message")
  } else {
    msg.XML, err = xml.StringToHash(decrypted)
  }

  if err != nil {
    msg.XML = hash("error(%v)", err)
  }

  // if we get a new_server or confirm_new_server message, update our server key  
  header := msg.XML.Text("header")
  if header == "new_server" || header == "confirm_new_server" {
    keys = append(keys, keys[0])
    keys[0] = msg.XML.Text("key")
  }
  
  // The test server advertises "goSusi" in loaded_modules, so it is
  // required to confirm changes made to its jobs via foreign_job_updates
  if header == "foreign_job_updates" {
    for _, tag := range msg.XML.Subtags() {
      if !strings.HasPrefix(tag, "answer") { continue }
      for job := msg.XML.First(tag); job != nil; job = job.Next() {
        if job.Text("siserver") == listen_address {
          fju := xml.NewHash("xml","header","foreign_job_updates")
          fju.AddClone(job)
          send("", fju)
        }
      }
    }
  }
  
  
  msg.Time = time.Now()
  msg.SenderIP = senderIP
  //fmt.Printf("Received %v\n", msg.XML.String())
  
  queue_mutex.Lock()
  defer queue_mutex.Unlock()
  queue = append(queue, &msg)
  
  // Because initially go-susi doesn't know that we're also "goSusi"
  // it may ask as for our database, so we need to be able to respond
  if header == "gosa_query_jobdb" {
    emptydb := fmt.Sprintf("<xml><header>query_jobdb</header><source>%v</source><target>GOSA</target></xml>",listen_address)
    return message.GosaEncrypt(emptydb, config.ModuleKey["[GOsaPackages]"])
  }
  
  return ""
}

