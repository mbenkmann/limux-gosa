package tests

import (
         "io"
         "net"
         "fmt"
         "time"
         "sync"
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

// keys[0] is the key of the test server started by listener(). The other
// elements are copies of config.ModuleKeys
var keys []string

// start time of SystemTest()
var StartTime time.Time

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
  go listener()
  
  config.ReadNetwork()
  listen_address = config.IP + ":" + listen_port
  
  // if we got a program path (i.e. not host:port), create config and launch program
  if launched_daemon {
    var confdir string
    config.ServerConfigPath, confdir = createConfigFile()
    //defer os.RemoveAll(confdir)
    defer fmt.Printf("\nLog file directory: %v\n", confdir)
    cmd := exec.Command(daemon, "-f", "-c", config.ServerConfigPath, "-vvvvvv")
    cmd.Stderr,_ = os.Create(confdir+"/go-susi+panic.log")
    err := cmd.Start()
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
  //   listen_address is the address of the test server run by listener()
  //   config.ServerSourceAddress is the address of the go-susi or gosa-si being tested  
  
  
  keys = make([]string, len(config.ModuleKeys)+1)
  for i := range config.ModuleKeys { keys[i+1] = config.ModuleKeys[i] }
  keys[0] = "none"
  
  test_mac := "01:02:03:04:05:06"
  test_name := "none"
  test_timestamp := "20990914131742"
  test_periodic := "7_days"
  if launched_daemon {
    check_new_server_on_startup(test_mac, test_name, test_timestamp)
  } else {
    // We need this in the database for the later test whether go-susi reacts
    // to new_server by sending its jobdb. This same call is contained in
    // check_new_server_on_startup()
    trigger_first_test_job(test_mac, test_name, test_timestamp)
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
  siFail(checkTags(msg.XML, "header,source,target,answer1"), "")
  if checkTags(msg.XML, "header,source,target,answer1") == "" {
    check_foreign_job_updates(msg, "new_server_key", test_name, "7_days", "waiting", test_mac, "trigger_action_wake", test_timestamp)
  }

  t0 = time.Now()
  test_mac2 := "11:22:33:44:55:66"
  test_name2 := "none"
  test_timestamp2 := "20770101000000"
  test_periodic2 := "1_minutes"
  x := gosa("job_trigger_action_lock", hash("xml(target(%v)timestamp(%v)macaddress(%v)periodic(%v))",test_mac2, test_timestamp2, test_mac2, test_periodic2))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  check(x.Text("header"), "answer")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  check(x.Text("answer1"), "0")
  
  msg = wait(t0, "foreign_job_updates")
  check_foreign_job_updates(msg, "new_server_key", test_name2, "1_minutes", "waiting", test_mac2, "trigger_action_lock", test_timestamp2)
  
  check_connection_drop_on_error1()
  check_connection_drop_on_error2()
  
  check_multiple_requests_over_one_connection()
  
  // query for trigger_action_lock on test_mac2
  x = gosa("query_jobdb", hash("xml(where(clause(phrase(macaddress(%v)))))", test_mac2))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  check(x.Text("header"), "query_jobdb")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  a := x.First("answer1")
  check(a != nil, true)
  if a != nil {
    check_answer(a, test_name2, "none", "waiting", config.ServerSourceAddress, test_mac2, test_timestamp2, test_periodic2, "trigger_action_lock")
  }
  
  // query for trigger_action_wake on test_mac (via "ne test_mac2")
  x = gosa("query_jobdb", hash("xml(where(clause(connector(and)phrase(operator(ne)macaddress(%v)))))", test_mac2))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  check(x.Text("header"), "query_jobdb")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  a = x.First("answer1")
  check(a != nil, true)
  if a != nil {
    check_answer(a, test_name, "none", "waiting", config.ServerSourceAddress, test_mac, test_timestamp, test_periodic, "trigger_action_wake")
  }
  
  // delete trigger_action_wake on test_mac (via "ne test_mac2" plus redundant "like ...")
  t0 = time.Now()
  x = gosa("delete_jobdb_entry", hash("xml(where(clause(connector(and)phrase(operator(like)headertag(trigger_action_%%))phrase(operator(ne)macaddress(%v)))))", test_mac2))
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
  old_job := a.Clone()
  check(a != nil, true)
  if a != nil {
    check_answer(a, test_name2, "none", "waiting", config.ServerSourceAddress, test_mac2, test_timestamp2, test_periodic2, "trigger_action_lock")
  }
  
  // check for foreign_job_updates with status "done"
  msg = wait(t0, "foreign_job_updates")
  check_foreign_job_updates(msg, keys[0], test_name, "", "done", test_mac, "trigger_action_wake", test_timestamp)
  
  // Send foreign_job_updates with following changes:
  //   change <siserver> of the existing job
  //   add a new job
  old_job.SetText("siserver", listen_address)
  old_job.SetText("id", "100")
  new_job := hash("answer2(plainname(foo)progress(none)status(waiting)siserver(localhost)modified(1)macaddress(00:0c:29:50:a3:52)timestamp(20660906164734)id(66)headertag(trigger_action_wake)result(none))")
  new_job.SetText("xmlmessage", base64.StdEncoding.EncodeToString([]byte(hash("xml(header(job_trigger_action_wake)source(GOSA)target(00:0c:29:50:a3:52)timestamp(20660906164734)macaddress(00:0c:29:50:a3:52))").String())))
  x = hash("xml(header(foreign_job_updates)source(%v)target(%v))",listen_address,config.ServerSourceAddress)
  x.AddClone(old_job)
  x.AddClone(new_job)
  send("", x)
  
  // Check the jobdb for the above changes
  x = gosa("query_jobdb", hash("xml(where())"))
  check(checkTags(x, "header,source,target,answer1,answer2,session_id?"),"")
  check(x.Text("header"), "query_jobdb")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  a1 := x.First("answer1")
  a2 := x.First("answer2")
  if a1 != nil && a2 != nil{
    if a1.Text("plainname") == "foo" { // make sure a1 is the old and a2 is new job
      temp := a1
      a1 = a2
      a2 = temp
    }
    check_answer(a1, test_name2, "none", "waiting", listen_address, test_mac2, test_timestamp2, test_periodic2, "trigger_action_lock")
    check_answer(a2, "foo", "none", "waiting", listen_address, "00:0c:29:50:a3:52", "20660906164734", "", "trigger_action_wake")
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
  defer conn.Close()
  
  for i :=0 ; i < 3; i++ {
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
func check_new_server_on_startup(test_mac, test_name, test_timestamp string) {
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
    
    trigger_first_test_job(test_mac, test_name, test_timestamp)

    msg = wait(t0, "foreign_job_updates")
    check_foreign_job_updates(msg, "confirm_new_server_key", test_name, "7_days", "waiting", test_mac, "trigger_action_wake", test_timestamp)
  }
}

func trigger_first_test_job(test_mac, test_name, test_timestamp string) {
  x := gosa("job_trigger_action_wake", hash("xml(target(%v)timestamp(%v)macaddress(%v)periodic(7_days))",test_mac, test_timestamp, test_mac))
  check(checkTags(x, "header,source,target,answer1,session_id?"),"")
  check(x.Text("header"), "answer")
  siFail(x.Text("source"), config.ServerSourceAddress)
  check(x.Text("target"), "GOSA")
  check(x.Text("answer1"), "0")
}

func check_foreign_job_updates(msg *queueElement, test_key, test_name, test_periodic, test_status, test_mac, action, test_timestamp string) {
  _, file, line, _ := runtime.Caller(1)
  file = file[strings.LastIndex(file, "/")+1:]
  fmt.Printf("== check_foreign_job_updates sub-tests (%v:%v) ==\n", file, line)
      
  check(checkTags(msg.XML, "header,source,target,answer1"), "")
  check(msg.Key, test_key)
  check(msg.XML.Text("header"), "foreign_job_updates")
  siFail(msg.XML.Text("source"), config.ServerSourceAddress)
  siFail(msg.XML.Text("target"), listen_address)
  job := msg.XML.First("answer1")
  check(job != nil, true)
  if job != nil {
    check(checkTags(job, "plainname,periodic?,progress,status,siserver,modified,targettag,macaddress,timestamp,id,headertag,result,xmlmessage"),"")
    check(job.Text("plainname"), test_name)
    peri := job.Text("periodic")
    if peri == "none" { peri = "" }
    siFail(peri, test_periodic)
    check(job.Text("progress"), "none")
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
      
  check(checkTags(a, "plainname,periodic?,progress,status,siserver,modified,targettag,macaddress,timestamp,id,headertag,result,xmlmessage"),"")
  check(a.Text("plainname"), name)
  check(a.Text("progress"), progress)
  check(a.Text("status"), status)
  siFail(a.Text("siserver"), siserver)
  check(a.Text("targettag"), mac)
  check(a.Text("macaddress"), mac)
  check(a.Text("timestamp"), timestamp)
  siFail(a.Text("periodic"), periodic)
  check(a.Text("headertag"), headertag)
  check(a.Text("result"), "none")
  
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
    check(xmlmessage.Text("periodic"), periodic)
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
  
  if err == nil && (i < 2 || buf[i-1] != '\n' /*|| buf[i-2] != '\r'*/) {
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
    keys = append(keys, keys[0])
    keys[0] = msg.XML.Text("key")
  }
  
  msg.Time = time.Now()
  msg.SenderIP = senderIP
  //fmt.Printf("Received %v\n", msg.XML.String())
  
  queue_mutex.Lock()
  defer queue_mutex.Unlock()
  queue = append(queue, &msg)
}

