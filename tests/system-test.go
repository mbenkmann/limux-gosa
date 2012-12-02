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
         "os"
         "os/exec"
         "container/list"
         "encoding/base64"
         
         "../db"
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
// Regexp for recognizing valid <siserver> elements
var serverRegexp = regexp.MustCompile("^[0-9]{1,3}[.][0-9]{1,3}[.][0-9]{1,3}[.][0-9]{1,3}:[0-9]+$")

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

// Returns an XML hash for the job. Optional args can be the following:
//   int/uint: the name of the enclosing element will be answerX where X is the int
//             and the <id> will be X, too.
//   IP:PORT(string) : siserver  (default is listen_address)
func (job *Job) Hash(args... interface{}) *xml.Hash {
  x := xml.NewHash("answer1")
  x.Add("plainname", job.Plainname)
  x.Add("progress", "none")
  x.Add("status", "waiting")
  x.Add("siserver", listen_address)
  x.Add("modified", "1")
  x.Add("targettag", job.MAC)
  x.Add("macaddress", job.MAC)
  x.Add("timestamp", job.Timestamp)
  x.Add("periodic", job.Periodic)
  x.Add("id", "1")
  x.Add("headertag", job.Trigger())
  x.Add("result", "none")
  
  for _, arg := range args {
    switch arg := arg.(type) {
      case int:  x.Rename(fmt.Sprintf("answer%d",arg))
                 x.First("id").SetText("%d",arg)
      case uint: x.Rename(fmt.Sprintf("answer%d",arg))
                 x.First("id").SetText("%d",arg)
      case string:
                 if serverRegexp.MatchString(arg) {
                   x.First("siserver").SetText(arg)
                 } else {
                   panic("Unknown string format in Job.Hash()")
                 }
      default: panic("Unknown type in Job.Hash()")
    }
  }
  
  xm := xml.NewHash("xml","header", "job_" + x.Text("headertag"))
  xm.Add("source", "GOSA")
  xm.Add("target", x.Text("targettag"))
  xm.Add("timestamp", x.Text("timestamp"))
  xm.Add("macaddress", x.Text("macaddress"))
  x.Add("xmlmessage", base64.StdEncoding.EncodeToString([]byte(xm.String())))
  return x
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
    config.ServerConfigPath, confdir = createConfigFile("system-test-", listen_address)
    //defer os.RemoveAll(confdir)
    defer fmt.Printf("\nLog file directory: %v\n", confdir)
    args := []string{ "-f", "-vvvvvv"}
    if !gosasi { args = append(args,"--test="+confdir) }
    args = append(args, "-c", config.ServerConfigPath)
    cmd = exec.Command(daemon, args...)
    cmd.Stderr,_ = os.Create(confdir+"/go-susi+panic.log")
    err = cmd.Start()
    if err != nil { panic(err) }
    defer cmd.Process.Signal(syscall.SIGTERM)
    daemon = ""
    
    // Give daemon time to process data and write logs before sending SIGTERM
    defer time.Sleep(reply_timeout)
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
  

  run_startup_tests()
  
  check_connection_drop_on_error1()
  check_connection_drop_on_error2()
  
  check_multiple_requests_over_one_connection()
  
  // query for trigger_action_lock on test_mac2
  x := gosa("query_jobdb", hash("xml(where(clause(phrase(macaddress(%v)))))", Jobs[1].MAC))
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
  t0 := time.Now()
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
  msg := wait(t0, "foreign_job_updates")
  check_foreign_job_updates(msg, keys[0], config.ServerSourceAddress, Jobs[0].Plainname, "", "done", "none", Jobs[0].MAC, Jobs[0].Trigger(), Jobs[0].Timestamp)
  
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
  check_foreign_job_updates(msg, keys[0], config.ServerSourceAddress, Jobs[1].Plainname, Jobs[1].Periodic, "waiting", "42", Jobs[1].MAC, Jobs[1].Trigger(), Jobs[1].Timestamp)
  
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

  // Test if server understands messages with ";IP:PORT" attached (gosa-si 2.7 protocol)
  conn, err := net.Dial("tcp", config.ServerSourceAddress)
  check(err, nil)
  encrypted_msg := message.GosaEncrypt("<xml><header>gosa_query_jobdb</header><where></where><source>GOSA</source><target>GOSA</target></xml>", config.ModuleKey["[GOsaPackages]"])
  util.SendLn(conn, encrypted_msg + ";"+listen_address , config.Timeout)
  reply := message.GosaDecrypt(util.ReadLn(conn, config.Timeout), config.ModuleKey["[GOsaPackages]"])
  x, err = xml.StringToHash(reply)
  if err != nil { x = xml.NewHash("error") }
  if conn != nil { conn.Close() }
  check(checkTags(x, "header,source,target,answer1,answer2,session_id?"),"")
  
  if gosasi {
    // The job processing tests require the following go-susi extensions:
    // * multiple jobs with the same headertag+macaddress combination
    // * <periodic> with unit "seconds"
    // Therefore we skip them if we're testing gosa-si.
    fmt.Print("Job processing tests need go-susi extensions => ")
    siFail(true,false)
  } else {
    run_job_processing_tests()
  }
  
  run_foreign_job_updates_tests()
}



func run_job_processing_tests() {
  // clear database
  send("",hash("xml(header(foreign_job_updates)source(%v)target(%v)sync(all))",listen_address,config.ServerSourceAddress))
  gosa("delete_jobdb_entry", hash("xml(where())"))  
  time.Sleep(reply_timeout)
  
  var t0 time.Time

  gotoMode := func(name string) string {
    i := name[len(name)-1] - '1'
    return db.SystemGetState(Jobs[i].MAC, "gotoMode")
  }
  lock := func(name string){
    i := name[len(name)-1] - '1'
    db.SystemSetState(Jobs[i].MAC, "gotoMode", "locked")
  }
  unlock := func(name string){
    i := name[len(name)-1] - '1'
    db.SystemSetState(Jobs[i].MAC, "gotoMode", "active")
  }
  job := func(typ string, start int, name string, period int){
    i := name[len(name)-1] - '1'
    if typ == "unlock" { typ = "activate" }
    timestamp := util.MakeTimestamp(t0.Add(time.Duration(start)*time.Second))
    periodic := fmt.Sprintf("%d_seconds",period)
    if period == 0 { periodic = "none" }
    gosa("job_trigger_action_"+typ, hash("xml(target(%v)timestamp(%v)macaddress(%v)periodic(%v))",Jobs[i].MAC, timestamp, Jobs[i].MAC, periodic))
  }
  now := func(typ string, name string){
    i := name[len(name)-1] - '1'
    if typ == "unlock" { typ = "activate" }
    gosa("gosa_trigger_action_"+typ, hash("xml(target(%v)macaddress(%v))",Jobs[i].MAC, Jobs[i].MAC))
  }
  del := func(typ, name string) {
    i := name[len(name)-1] - '1'
    if typ == "unlock" { typ = "activate" }
    gosa("delete_jobdb_entry", hash("xml(where(clause(phrase(macaddress(%v))phrase(headertag(trigger_action_%v)))))",Jobs[i].MAC,typ))  
  }
  done := func(typ, name string) {
    i := name[len(name)-1] - '1'
    if typ == "unlock" { typ = "activate" }
    x := gosa("query_jobdb", hash("xml(where(clause(phrase(macaddress(%v))phrase(headertag(trigger_action_%v)))))",Jobs[i].MAC,typ))
    if x == nil || x.First("header") == nil { 
      util.Log(0, "ERROR! done(%v, %v)", typ, name)
      return 
    }
    x.First("header").SetText("foreign_job_updates")
    x.First("source").SetText(listen_address)
    x.First("target").SetText(config.ServerSourceAddress)
    x.First("answer1").First("status").SetText("done")
    // DO NOT x.First("answer1").First("periodic").SetText("none") because 
    // we want to test that go-susi treats this properly. An fju with status==done
    // for a job not belonging to the fju's sender can only mean that the job should
    // be removed completely, because the sender cannot know when a job is just "done"
    // and should be started again because of periodic.
    send("", x)
  }
  change_timestamp := func(typ, name string, new_start int) {
    i := name[len(name)-1] - '1'
    if typ == "unlock" { typ = "activate" }
    x := gosa("query_jobdb", hash("xml(where(clause(phrase(macaddress(%v))phrase(headertag(trigger_action_%v)))))",Jobs[i].MAC,typ))
    if x == nil || x.First("header") == nil { 
      util.Log(0, "ERROR! change_timestamp(%v, %v, %v)", typ, name, new_start)
      return 
    }
    x.First("header").SetText("foreign_job_updates")
    x.First("source").SetText(listen_address)
    x.First("target").SetText(config.ServerSourceAddress)
    x.First("answer1").First("timestamp").SetText(util.MakeTimestamp(t0.Add(time.Duration(new_start)*time.Second)))
    // remove periodic to simulate a message from a pre-2.7 gosa-si. This tests
    // that go-susi leaves periodic unchanged if it is missing from fju.
    x.RemoveFirst("periodic")
    send("", x)
    time.Sleep(50*time.Millisecond)
  }
  pause := func(typ, name string) {
    i := name[len(name)-1] - '1'
    if typ == "unlock" { typ = "activate" }
    x := gosa("query_jobdb", hash("xml(where(clause(phrase(macaddress(%v))phrase(status(waiting))phrase(headertag(trigger_action_%v)))))",Jobs[i].MAC,typ))
    if x == nil || x.First("header") == nil { 
      util.Log(0, "ERROR! pause(%v, %v)", typ, name)
      return 
    }
    x.First("header").SetText("foreign_job_updates")
    x.First("source").SetText(listen_address)
    x.First("target").SetText(config.ServerSourceAddress)
    x.First("answer1").First("status").SetText("paused")
    send("", x)
    time.Sleep(50*time.Millisecond)
  }
  unpause := func(typ, name string) {
    i := name[len(name)-1] - '1'
    if typ == "unlock" { typ = "activate" }
    x := gosa("query_jobdb", hash("xml(where(clause(phrase(macaddress(%v))phrase(status(paused))phrase(headertag(trigger_action_%v)))))",Jobs[i].MAC,typ))
    if x == nil { 
      util.Log(0, "ERROR! unpause(%v, %v): query_jobdb returned nil", typ, name)
      return 
    }
    if x.First("header") == nil || x.First("answer1") == nil { 
      util.Log(0, "ERROR! unpause(%v, %v): query_jobdb returned %v", typ, name, x)
      return 
    }
    x.First("header").SetText("foreign_job_updates")
    x.First("source").SetText(listen_address)
    x.First("target").SetText(config.ServerSourceAddress)
    x.First("answer1").First("status").SetText("waiting")
    send("", x)
    time.Sleep(50*time.Millisecond)
  }

  unlock("systest1")
  unlock("systest2")
  unlock("systest3")
  check(gotoMode("systest1"), "active")
  check(gotoMode("systest2"), "active")
  check(gotoMode("systest3"), "active")
  now("lock","systest1")
  now("lock","systest2")
  now("lock","systest3")
  time.Sleep(reply_timeout)
  check(gotoMode("systest1"), "locked")
  check(gotoMode("systest2"), "locked")
  check(gotoMode("systest3"), "locked")
  now("unlock","systest1")
  now("unlock","systest2")
  now("unlock","systest3")
  time.Sleep(reply_timeout)
  check(gotoMode("systest1"), "active")
  check(gotoMode("systest2"), "active")
  check(gotoMode("systest3"), "active")

  /*
   ATTENTION! The following tests are very timing sensitive. If you enter
   a delay in the wrong place, everything will come crashing down and you
   may even see a panic due to nil-reference in the test code.
  */

  // Because siserver timestamps have only a 1s resolution, we must make sure that
  // we start the following test on a full second. Otherwise when we later compute
  // differences to t0 and round to seconds we may be off by one which makes a lot
  // of tests fail.
  util.WaitUntil(util.ParseTimestamp(util.MakeTimestamp(time.Now().Add(2*time.Second))))

  t0 = time.Now()

  job("lock",0,   "systest1", 0)   //0: lock "systest1"
  job("lock",2,   "systest1", 0)   //2: lock "systest1"
  job("lock",4,   "systest1", 0)   //4: lock "systest1"
  job("unlock", 1,"systest1", 2)   //1, 3, 5: unlock "systest1"
  go func(){
    time.Sleep(6*time.Second)      
    del("unlock", "systest1")      //6: kill periodic unlock "systest1" preventing 7
  }()
  
  job("lock",10,  "systest2", 3)
  go func(){
    time.Sleep(100*time.Millisecond); 
    change_timestamp("lock","systest2", -1) // 0, 2, 5: lock "systest2"
    time.Sleep(6*time.Second)
    done("lock","systest2")        //6: kill periodic lock "systest2" preventing 8
  }()
  
  job("lock", 10,  "systest3", 0)
  go func(){
    time.Sleep(100*time.Millisecond); 
    pause("lock",   "systest3")
    change_timestamp("lock","systest3", 1) // 1(paused): lock "systest3"
    time.Sleep(3*time.Second)
    unpause("lock","systest3")             // 3: lock "systest3"
  }()
  job("unlock", 10, "systest3", 0)
  go func(){
    time.Sleep(100*time.Millisecond); 
    change_timestamp("unlock","systest3", 7) // 7: unlock "systest3"
  }()
  
  go func(){
    util.WaitUntil(t0.Add(1*time.Second))
    unlock("systest2")
    util.WaitUntil(t0.Add(4*time.Second))
    unlock("systest2")
    util.WaitUntil(t0.Add(6*time.Second))
    lock("systest1")
    unlock("systest2")
  }()
  
  // Complete action timeline:
  // 0: lock "systest1"
  //    lock "systest2"
  // 1: unlock "systest1"
  //    unlock "systest2" (explicit in code => no "processing"/"done" fju)
  // 2: lock "systest1"
  //    lock "systest2"
  // 3: unlock "systest1"
  //    lock "systest3"
  // 4: lock "systest1"
  //    unlock "systest2" (explicit in code => no "processing"/"done" fju)
  // 5: unlock "systest1"
  //    lock "systest2"
  // 6: lock "systest1"   (explicit in code => no "processing"/"done" fju)
  //    unlock "systest2" (explicit in code => no "processing"/"done" fju)
  // 7: unlock "systest3"
  
  
  // test that all actions happen at the correct times
  // also test that the cancelled actions do not occur at 7 and 8
  util.WaitUntil(t0.Add(500*time.Millisecond))
  check(gotoMode("systest1"), "locked")
  check(gotoMode("systest2"), "locked")
  check(gotoMode("systest3"), "active")
  util.WaitUntil(t0.Add(1500*time.Millisecond))
  check(gotoMode("systest1"), "active")
  check(gotoMode("systest2"), "active")
  check(gotoMode("systest3"), "active")
  util.WaitUntil(t0.Add(2500*time.Millisecond))
  check(gotoMode("systest1"), "locked")
  check(gotoMode("systest2"), "locked")
  check(gotoMode("systest3"), "active")
  util.WaitUntil(t0.Add(3500*time.Millisecond))
  check(gotoMode("systest1"), "active")
  check(gotoMode("systest2"), "locked")
  check(gotoMode("systest3"), "locked")
  util.WaitUntil(t0.Add(4500*time.Millisecond))
  check(gotoMode("systest1"), "locked")
  check(gotoMode("systest2"), "active")
  check(gotoMode("systest3"), "locked")
  util.WaitUntil(t0.Add(5500*time.Millisecond))
  check(gotoMode("systest1"), "active")
  check(gotoMode("systest2"), "locked")
  check(gotoMode("systest3"), "locked")
  util.WaitUntil(t0.Add(6500*time.Millisecond))
  check(gotoMode("systest1"), "locked")
  check(gotoMode("systest2"), "active")
  check(gotoMode("systest3"), "locked")
  util.WaitUntil(t0.Add(7500*time.Millisecond))
  check(gotoMode("systest1"), "locked")
  check(gotoMode("systest2"), "active")
  check(gotoMode("systest3"), "active")
  util.WaitUntil(t0.Add(8500*time.Millisecond))
  check(gotoMode("systest1"), "locked")
  check(gotoMode("systest2"), "active")
  check(gotoMode("systest3"), "active")
  
  unlock("systest1")
  
  // fju timeline (excluding "processing" and "done" which occur for each
  // of the actions in the timeline above):
  // 0: lock   "systest1" 0,0  waiting
  //    lock   "systest1" 2,0  waiting
  //    lock   "systest1" 4,0  waiting
  //    unlock "systest1" 1,2  waiting
  //    lock   "systest2" 10,3 waiting
  //    lock   "systest2" -1,3 waiting
  //    lock   "systest3" 10,0 waiting
  //    lock   "systest3" 10,0 paused
  //    lock   "systest3" 1,0  paused
  //    unlock "systest3" 10,0 waiting
  //    unlock "systest3" 7,0  waiting
  //    lock   "systest2" 2,3  waiting
  // 1: unlock "systest1" 3,2  waiting
  // 2: lock   "systest2" 5,3  waiting
  // 3: lock   "systest3" 1,0  waiting
  //    unlock "systest1" 5,2  waiting
  // 4: ----
  // 5: unlock "systest1" 7,2  waiting
  //    lock   "systest2" 8,3  waiting
  // 6: unlock "systest1" 7,0(2)  done
  //    lock   "systest2" 8,0(3)  done

  quantize := func(t time.Time) int {
    dur := t.Sub(t0)
    if dur < 0 { return int((dur-500*time.Millisecond)/time.Second) }
    return int((dur+500*time.Millisecond)/time.Second)
  }
  
  // collect all fjus in format (6 unlock systest1 7,2  done)
  var messages []*queueElement = get(t0)
  msgset := map[string]string{}
  for _, msg := range messages {
    if msg.XML.Text("header") != "foreign_job_updates" { continue }
    for _, tag := range msg.XML.Subtags() {
      if !strings.HasPrefix(tag, "answer") { continue }
      for job := msg.XML.First(tag); job != nil; job = job.Next() {
        typ := job.Text("headertag")
        if strings.HasPrefix(typ, "trigger_action_") { typ = typ[15:] }
        if typ == "activate" { typ = "unlock" }
        
        peri := job.Text("periodic")
        if peri == "none" || peri == "" { 
          peri = "0" 
        } else {
          peri = peri[0:strings.Index(peri,"_")]
        }
        
        name := db.PlainnameForMAC(job.Text("macaddress"))
        ts := quantize(util.ParseTimestamp(job.Text("timestamp")))
        //fmt.Printf("%v: %v => %v => %v\n",name,job.Text("timestamp"),util.ParseTimestamp(job.Text("timestamp")),ts)
        msgset[fmt.Sprintf("(%v %v %v %v,%v %v)",quantize(msg.Time),typ,name,ts,peri,job.Text("status"))] = ""
      }
    }
  }
  
  fju := func(when int, typ, name string, ts,peri int, status string) string {
    s := fmt.Sprintf("(%v %v %v %v,%v %v)",when,typ,name,ts,peri,status)
    if _, ok := msgset[s]; ok { delete(msgset,s); return "" }
    return "Missing fju: "+s
  }
  
  // now check that all fjus we expect are present
  check(fju(0,"lock",   "systest1", 0,0, "waiting"),"")
  check(fju(0,"lock",   "systest1", 2,0, "waiting"),"")
  check(fju(0,"lock",   "systest1", 4,0,  "waiting"),"")
  check(fju(0,"unlock", "systest1", 1,2,  "waiting"),"")
  check(fju(0,"lock",   "systest2", 10,3, "waiting"),"")
  check(fju(0,"lock",   "systest2", -1,3, "waiting"),"")
  check(fju(0,"lock",   "systest3", 10,0, "waiting"),"")
  check(fju(0,"lock",   "systest3", 10,0, "paused"),"")
  check(fju(0,"lock",   "systest3", 1,0,  "paused"),"")
  check(fju(0,"unlock", "systest3", 10,0, "waiting"),"")
  check(fju(0,"unlock", "systest3", 7,0,  "waiting"),"")
  check(fju(0,"lock",   "systest2", 2,3,  "waiting"),"")
  check(fju(1,"unlock", "systest1", 3,2,  "waiting"),"")
  check(fju(2,"lock",   "systest2", 5,3,  "waiting"),"")
  check(fju(3,"lock",   "systest3", 1,0,  "waiting"),"")
  check(fju(3,"unlock", "systest1", 5,2,  "waiting"),"")
  check(fju(5,"unlock", "systest1", 7,2,  "waiting"),"")
  check(fju(5,"lock",   "systest2", 8,3,  "waiting"),"")
  check(fju(6,"unlock", "systest1", 7,0,  "done"),"")
  check(fju(6,"lock",   "systest2", 8,0,  "done"),"")
  
  check(fju(0,"lock",   "systest1", 0,0, "done"),"")
  check(fju(0,"lock",   "systest2",-1,3, "done"),"")
  check(fju(1,"unlock", "systest1", 1,2, "done"),"")
  check(fju(2,"lock",   "systest1", 2,0, "done"),"")
  check(fju(2,"lock",   "systest2", 2,3, "done"),"")
  check(fju(3,"unlock", "systest1", 3,2, "done"),"")
  check(fju(3,"lock",   "systest3", 1,0, "done"),"")
  check(fju(4,"lock",   "systest1", 4,0, "done"),"")
  check(fju(5,"unlock", "systest1", 5,2, "done"),"")
  check(fju(5,"lock",   "systest2", 5,3, "done"),"")
  check(fju(7,"unlock", "systest3", 7,0, "done"),"")
  
  check(fju(0,"lock",   "systest1", 0,0, "processing"),"")
  check(fju(0,"lock",   "systest2",-1,3, "processing"),"")
  check(fju(1,"unlock", "systest1", 1,2, "processing"),"")
  check(fju(2,"lock",   "systest1", 2,0, "processing"),"")
  check(fju(2,"lock",   "systest2", 2,3, "processing"),"")
  check(fju(3,"unlock", "systest1", 3,2, "processing"),"")
  check(fju(3,"lock",   "systest3", 1,0, "processing"),"")
  check(fju(4,"lock",   "systest1", 4,0, "processing"),"")
  check(fju(5,"unlock", "systest1", 5,2, "processing"),"")
  check(fju(5,"lock",   "systest2", 5,3, "processing"),"")
  check(fju(7,"unlock", "systest3", 7,0, "processing"),"")
  
  check(msgset, map[string]string{})
  
  // now test that the testee doesn't execute jobs from other siservers,
  // even when 
  // * their timestamp is changed via gosa_update_status_jobdb_entry
  // * their timestamp is changed via fju
  // * their status is set to "processing" via gosa_update_status_jobdb_entry
  // * their status is set to "processing" via fju
  
}

func run_startup_tests() {
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
  if siFail(checkTags(msg.XML, "header,source,target,answer1,sync?"), "") {
    check_foreign_job_updates(msg, "new_server_key", config.ServerSourceAddress, Jobs[0].Plainname, Jobs[0].Periodic, "waiting", "none", Jobs[0].MAC, Jobs[0].Trigger(), Jobs[0].Timestamp)
  }

  t0 = time.Now()
  x := gosa(Jobs[1].Type, hash("xml(target(%v)timestamp(%v)macaddress(%v)periodic(%v))",Jobs[1].MAC, Jobs[1].Timestamp, Jobs[1].MAC, Jobs[1].Periodic))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  check(x.Text("header"), "answer")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  check(x.Text("answer1"), "0")
  
  msg = wait(t0, "foreign_job_updates")
  check_foreign_job_updates(msg, "new_server_key", config.ServerSourceAddress,  Jobs[1].Plainname, Jobs[1].Periodic, "waiting", "none",Jobs[1].MAC, Jobs[1].Trigger(), Jobs[1].Timestamp)
}

func run_foreign_job_updates_tests() {
  // clear jobdb  
  x := gosa("delete_jobdb_entry", hash("xml(where())"))
  
  // Because the above delete may affect jobs belonging to the test server,
  // go-susi may not delete all jobs directly and instead forward the request to the
  // test server. Wait a little to make sure the communication is finished.
  time.Sleep(reply_timeout)
  
  // try to add jobs with incorrect or missing <source> and/or incorrect or missing <siserver>
  // All of these should be rejected by the server, except for the 1 correct job with
  // listen_address/"localhost"
  count := 10
  for _, source := range []string{listen_address, "foo","1.2.3.4","1.2.3:9999","","missing"} {
    x := hash("xml(header(foreign_job_updates)source(%v)target(%v))",source,config.ServerSourceAddress)
    if source == "missing" { x.Remove(xml.FilterSimple("source")) }
    i := 1
    for _, siserver := range []string{"localhost","foo","1.2.3.4","1.2.3:9999","","missing" } {
      job := hash("answer%d(plainname(%v)progress(none)status(waiting)siserver(%v)modified(1)macaddress(00:00:00:00:00:%d)targettag(00:00:00:00:00:%d)timestamp(%v)id(%d)headertag(%v)result(none))",
                   i, Jobs[0].Plainname, siserver, count, count, Jobs[0].Timestamp, count, Jobs[0].Trigger())
      count++
      i++
      if siserver == "missing" { job.Remove(xml.FilterSimple("siserver")) }
      job.FirstOrAdd("xmlmessage").SetText(base64.StdEncoding.EncodeToString([]byte(hash("xml(header(%v)source(%v)target(%v)timestamp(%v)macaddress(%v))",Jobs[0].Type,"GOSA",job.Text("macaddress"),Jobs[0].Timestamp,job.Text("macaddress")).String())))
      x.AddClone(job)
      util.SendLnTo(config.ServerSourceAddress, message.GosaEncrypt(x.String(), keys[0]), config.Timeout)
    }  
  }
  
  // Wait for messages to be processed, because send() doesn't wait.
  time.Sleep(reply_timeout)
  
  // Check the jobdb to verify that only the one correct job was added to the database
  x = gosa("query_jobdb", hash("xml(where())"))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  job := x.First("answer1")
  if job != nil {
     // NOTE: go-susi converts "localhost" => source
    check_answer(job, Jobs[0].Plainname, "none", "waiting", listen_address, "00:00:00:00:00:10", Jobs[0].Timestamp, "none", Jobs[0].Trigger())
  }

  // Send empty foreign_job_updates with sync "ordered" and test that this does NOT clear out the above job
  x = hash("xml(header(foreign_job_updates)source(%v)target(%v)sync(ordered))",listen_address,config.ServerSourceAddress)
  send("",x)
  
  // Wait for messages to be processed, because send() doesn't wait.
  time.Sleep(reply_timeout)
  
  // Check the jobdb to verify that the job's still there
  x = gosa("query_jobdb", hash("xml(where())"))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")

  // Send empty foreign_job_updates with sync all and test that this clears out the above job
  x = hash("xml(header(foreign_job_updates)source(%v)target(%v)sync(all))",listen_address,config.ServerSourceAddress)
  send("",x)
  
  // Wait for messages to be processed, because send() doesn't wait.
  time.Sleep(reply_timeout)
  
  // Check the jobdb to verify that it is now empty.
  x = gosa("query_jobdb", hash("xml(where())"))
  check(checkTags(x, "header,source,target,session_id?"),"")
  
  // Now we test that when the testee forwards deletions it includes the 
  // original id (rather than its own)
  job = Jobs[0].Hash()
  job.First("id").SetText("0815")
  fju := xml.NewHash("xml", "header", "foreign_job_updates")
  fju.Add("source", listen_address)
  fju.AddClone(job)
  send("", fju)
  time.Sleep(reply_timeout)
  x = gosa("query_jobdb", hash("xml(where())"))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  t0 := time.Now()
  gosa("delete_jobdb_entry", hash("xml(where())"))
  msg := wait(t0, "foreign_job_updates")
  if check(checkTags(msg.XML, "header,source,target,answer1,sync?"), "") {
    check_foreign_job_updates(msg, keys[0], listen_address, Jobs[0].Plainname, "", "done", "none", Jobs[0].MAC, Jobs[0].Trigger(), Jobs[0].Timestamp)
    siFail(msg.XML.First("answer1").Text("id"), "0815")
  }
  
  // Clear out the above test job. For gosa-si, the gosa_delete_jobdb above has
  // already done that but go-susi waits for our fju.
  fju.First("answer1").First("status").SetText("done")
  t0 = time.Now()
  send("", fju)
  // check that we receive NO fju
  msg = wait(t0, "foreign_job_updates")
  check(msg.XML.Text("header"), "")
  // Check the jobdb to verify that it is now empty.
  x = gosa("query_jobdb", hash("xml(where())"))
  check(checkTags(x, "header,source,target,session_id?"),"")
  
  // Now we test that when the testee forwards a modification it includes
  // the original id (rather than its own)
  job = Jobs[0].Hash()
  job.First("id").SetText("textID")
  fju = xml.NewHash("xml", "header", "foreign_job_updates")
  fju.Add("source", listen_address)
  fju.AddClone(job)
  send("", fju)
  time.Sleep(reply_timeout)
  x = gosa("query_jobdb", hash("xml(where())"))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  t0 = time.Now()
  gosa("update_status_jobdb_entry", hash("xml(where()update(progress(20)))"))
  msg = wait(t0, "foreign_job_updates")
  
  // update_status_jobdb_entry is not implemented yet, so the following checks fail ATM
  
  if checkFail(checkTags(msg.XML, "header,source,target,answer1,sync?"), "") {
    check_foreign_job_updates(msg, keys[0], config.ServerSourceAddress, Jobs[0].Plainname, Jobs[0].Periodic, "done", "20", Jobs[0].MAC, Jobs[0].Trigger(), Jobs[0].Timestamp)
    check(msg.XML.First("answer1").Text("id"), "textID")
  }
  
  // Clear out the test job. 
  send("",hash("xml(header(foreign_job_updates)source(%v)target(%v)sync(all))",listen_address,config.ServerSourceAddress))
  gosa("delete_jobdb_entry", hash("xml(where())"))
  
  // create a job on the testee
  gosa(Jobs[0].Type, hash("xml(target(%v)timestamp(%v)macaddress(%v))",Jobs[0].MAC,Jobs[0].Timestamp,Jobs[0].MAC))
  time.Sleep(reply_timeout)
  // check if it's been successfully created
  x = gosa("query_jobdb", hash("xml(where())"))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  job = x.First("answer1")
  check_answer(job, Jobs[0].Plainname, "none", "waiting", config.ServerSourceAddress, Jobs[0].MAC, Jobs[0].Timestamp, "none", Jobs[0].Trigger())
  
  // send fju with wrong id. Because our test server has identified itself as
  // goSusi the testee should ignore the fju because goSusi protocol requires
  // sending the correct id.
  id := job.Text("id")
  job.First("id").SetText("not_the_actual_id")
  job.First("status").SetText("done")
  fju = xml.NewHash("xml", "header", "foreign_job_updates")
  fju.Add("source", listen_address)
  fju.AddClone(job)
  t0 = time.Now()
  send("", fju)
  // check that we receive NO fju
  msg = wait(t0, "foreign_job_updates")
  check(msg.XML.Text("header"), "")
  // check that the job's still there
  x = gosa("query_jobdb", hash("xml(where())"))
  if siFail(checkTags(x, "header,source,target,answer1,session_id?"),"") {
    job = x.First("answer1")
    check_answer(job, Jobs[0].Plainname, "none", "waiting", config.ServerSourceAddress, Jobs[0].MAC, Jobs[0].Timestamp, "none", Jobs[0].Trigger())  
  }
  
  // now send the fju with the correct id and check it causes the job to be removed.
  // This is Case 1.1 in foreign_job_updates.go
  job.First("id").SetText(id)
  job.First("status").SetText("done")
  fju = xml.NewHash("xml", "header", "foreign_job_updates")
  fju.Add("source", listen_address)
  fju.AddClone(job)
  t0 = time.Now()
  send("", fju)
  // check that we receive fju (this fails on gosa-si because gosa-si does not
  // send fju for changes received via fju)
  msg = wait(t0, "foreign_job_updates")
  if siFail(checkTags(msg.XML, "header,source,target,answer1,sync?"), "") {
    check_foreign_job_updates(msg, keys[0], config.ServerSourceAddress, Jobs[0].Plainname, "", "done", "none", Jobs[0].MAC, Jobs[0].Trigger(), Jobs[0].Timestamp)
  }
  
  // check that the job's gone
  x = gosa("query_jobdb", hash("xml(where())"))
  check(checkTags(x, "header,source,target,session_id?"),"")
  
  // Send new_server without "goSusi" to make testee treat us as gosa-si
  t0 = time.Now()
  send("[ServerPackages]", hash("xml(header(new_server)new_server()key(%v)loaded_modules(foo)macaddress(00:00:00:00:00:00))", keys[0]))
  msg = wait(t0, "confirm_new_server")
  
  // create a job on the testee
  gosa(Jobs[0].Type, hash("xml(target(%v)timestamp(%v)macaddress(%v))",Jobs[0].MAC,Jobs[0].Timestamp,Jobs[0].MAC))
  time.Sleep(reply_timeout)
  // check if it's been successfully created
  x = gosa("query_jobdb", hash("xml(where())"))
  if check(checkTags(x, "header,source,target,answer1,session_id?"),"") {
    job = x.First("answer1")
    check_answer(job, Jobs[0].Plainname, "none", "waiting", config.ServerSourceAddress, Jobs[0].MAC, Jobs[0].Timestamp, "none", Jobs[0].Trigger())  
  }
  
  // check that the job is removed even when we send the wrong id
  // This is Case 1.2 in foreign_job_updates.go
  job.First("id").SetText("not_the_actual_id")
  job.First("status").SetText("done")
  fju = xml.NewHash("xml", "header", "foreign_job_updates")
  fju.Add("source", listen_address)
  fju.AddClone(job)
  t0 = time.Now()
  send("", fju)
  // check that we receive fju
  msg = wait(t0, "foreign_job_updates")
  siFail(msg.XML.Text("header"), "foreign_job_updates")
  // Check the job's actually gone
  x = gosa("query_jobdb", hash("xml(where())"))
  check(checkTags(x, "header,source,target,session_id?"),"")
  
  // Now identify our test server as goSusi again
  send("[ServerPackages]", hash("xml(header(new_server)new_server()key(%v)loaded_modules(goSusi)macaddress(00:00:00:00:00:00))", keys[0]))
  
  // Case 2 in foreign_job_updates.go
  // Updated job belongs to the sender (i.e. our test server)
  job.First("id").SetText("my_id")
  job.First("siserver").SetText(listen_address)
  job.First("status").SetText("waiting")
  fju = xml.NewHash("xml", "header", "foreign_job_updates")
  fju.Add("source", listen_address)
  fju.AddClone(job)
  t0 = time.Now()
  send("", fju)
  // check that we receive NO fju
  msg = wait(t0, "foreign_job_updates")
  check(msg.XML.Text("header"), "")
  // check that the job is there
  x = gosa("query_jobdb", hash("xml(where())"))
  if check(checkTags(x, "header,source,target,answer1,session_id?"),"") {
    job = x.First("answer1")
    check_answer(job, Jobs[0].Plainname, "none", "waiting", listen_address, Jobs[0].MAC, Jobs[0].Timestamp, "none", Jobs[0].Trigger())  
  }
  // modify the job
  fju.First("answer1").First("progress").SetText("99")
  fju.First("answer1").First("status").SetText("paused")
  t0 = time.Now()
  send("", fju)
  // check that we receive NO fju
  msg = wait(t0, "foreign_job_updates")
  check(msg.XML.Text("header"), "")
  // check that the job is updated
  x = gosa("query_jobdb", hash("xml(where())"))
  if check(checkTags(x, "header,source,target,answer1,session_id?"),"") {
    job = x.First("answer1")
    check_answer(job, Jobs[0].Plainname, "99", "paused", listen_address, Jobs[0].MAC, Jobs[0].Timestamp, "none", Jobs[0].Trigger())  
  }
  
  // Now delete the job, using an incorrect headertag. This fails on gosa-si but
  // succeeds on go-susi because go-susi used the id to select the job
  fju.First("answer1").First("headertag").SetText("incorrect")
  fju.First("answer1").First("status").SetText("done")
  send("", fju)
  time.Sleep(reply_timeout)
  // check that the job is gone
  x = gosa("query_jobdb", hash("xml(where())"))
  siFail(checkTags(x, "header,source,target,session_id?"),"")
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
    check_foreign_job_updates(msg, "confirm_new_server_key", config.ServerSourceAddress, test_name, "7_days", "waiting", "none", test_mac, "trigger_action_wake", test_timestamp)
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

func check_foreign_job_updates(msg *queueElement, test_key, test_server, test_name, test_periodic, test_status, test_progress, test_mac, action, test_timestamp string) {
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
    siFail(job.Text("siserver"), test_server)
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
func siFail(x interface{}, expected interface{}) bool {
  if gosasi {
    return checkFailLevel(x, expected,2)
  } else {
    return checkLevel(x, expected,2)
  }
  return true
}


// Takes a format string like "xml(foo(%v)bar(%v))" and parameters and creates
// a corresponding xml.Hash.
func hash(format string, args... interface{}) *xml.Hash {
  format = fmt.Sprintf(format, args...)
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
  
  hash, err := xml.StringToHash(strings.Join(output, ""))
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

