/*
Copyright (c) 2012 Matthias S. Benkmann

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, 
MA  02110-1301, USA.
*/

// The go-susi.go main program as well as associated programs.
//
//  go-susi - the daemon.
//  run_tests - runs the unit tests.
//  sibridge - interactive/scripting interface to go-susi
package main

import (
          "io"
          "bufio"
          "os"
          "os/exec"
          "os/signal"
          "fmt"
          "net"
          "path"
          "time"
          "strings"
          "syscall"
          "sync/atomic"
          "crypto/tls"
          
          "../db"
          "../xml"
          "github.com/mbenkmann/golib/util"
          "github.com/mbenkmann/golib/deque"
          "../tftp"
          "github.com/mbenkmann/golib/bytes"
          "../config"
          "../action"
          "../message"
          "../security"
       )

//import _ "net/http/pprof"
//import "net/http"

// Set to true when a signal is received that triggers go-susi shutdown.
var Shutdown = false

// counts the number of active connections. Limited by config.MaxConnections
var ActiveConnections int32 = 0

// Whenever a request has been handled, the time it took to process it
// (a time.Duration) is Push()ed into this Deque at the top and the Next() element
// is taken from the Deque at the bottom. The difference is then added atomically
// to message.RequestProcessingTime, so that message.RequestProcessingTime always
// corresponds to the sum of the durations in RequestProcessingTimes.
var RequestProcessingTimes deque.Deque
func init() { for i := 0; i < 100; i++ { RequestProcessingTimes.Push(time.Duration(0)) } }


func main() {
  // Intercept signals asap (in particular intercept SIGTTOU before the first output)
  signals         := make(chan os.Signal,    32)
  signals_to_watch := []os.Signal{ syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGTTOU, syscall.SIGHUP, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT }
  signal.Notify(signals, signals_to_watch...)
  
  config.Init()
  config.ReadArgs(os.Args[1:])
  
  if config.PrintVersion {
    fmt.Printf(`go-susi %v (revision %v)
Copyright (c) 2013 Matthias S. Benkmann
This is free software; see the source for copying conditions.  There is NO
warranty; not even for MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

`, config.Version, config.Revision)
  }
  
  if config.PrintHelp {
    fmt.Println(`USAGE: go-susi [args]

--help       print this text and exit
--version    print version and exit
--stats      print sistats info from running go-susi process

-v           print operator debug messages (INFO)
-vv          print developer debug messages (DEBUG)
             ATTENTION! developer messages include keys!

-f           start with a fresh database; discard old /var/lib/go-susi

--test=<dir> test mode:
             * read config files from <dir> instead of /etc/gosa-si
             * use <dir>/go-susi.log as log file
             * use <dir> as database directory instead /var/lib/go-susi

-c <file>    read config from <file> instead of default location
`)
  }
  
  if config.PrintVersion || config.PrintHelp { os.Exit(0) }
  
  config.ReadConfig()
  config.ReadCertificates() // after config.ReadConfig()
  
  if config.PrintStats { os.Exit(printStats()) }
  
  logfile, err := os.OpenFile(config.LogFilePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
  if err != nil {
    util.Log(0, "ERROR! %v", err)
    // Do not exit. We can go on without logging to a file.
  
  } else {
    logfile.Close() // will be re-opened on the first write
    // Add file to loggers list. os.Stderr is on it by default.
    util.LoggerAdd(util.LogFile(logfile.Name()))
  }
  util.LogLevel = config.LogLevel
  
  util.Log(0, "=============================================================================")
  util.Log(0, "INFO! go-susi %v started", config.Version)
  
  if !config.RunServer {
    util.Log(1, "INFO! No ldap-admin-dn configured => Will run in client-only mode")
  }
  
  util.Log(1, "INFO! Expecting standard clients to communicate on these ports: %v", config.ClientPorts)
  
  util.Log(1, "INFO! Waiting up to 5 minutes for DNS to be available")
  if !util.WaitForDNS(5*time.Minute) {
    util.Log(0, "ERROR! DNS not available")
    util.LoggersFlush(5*time.Second)
    os.Exit(1)
  }
  util.Log(1, "INFO! DNS available")

  config.ReadNetwork() // after config.ReadConfig()

  // ATTENTION! DO NOT MOVE THE FOLLOWING CODE FURTHER DOWN!
  // We want to try listening on our socket as early in the program as possible,
  // so that we can bail out if another go-susi instance is already running
  // before potentially damaging the databases.
  tcp_addr, err := net.ResolveTCPAddr("tcp4", config.ServerListenAddress)
  if err != nil {
    util.Log(0, "ERROR! ResolveTCPAddr: %v", err)
    util.LoggersFlush(5*time.Second)
    os.Exit(1)
  }
  listener, err := net.ListenTCP("tcp4", tcp_addr)
  if err != nil {
    util.Log(0, "ERROR! ListenTCP: %v", err)
    util.LoggersFlush(5*time.Second)
    os.Exit(1)
  }
  
  if config.RunServer {
    util.Log(1, "INFO! Waiting up to 5 minutes for %v to be available", config.LDAPURI)
    if !db.LDAPAvailable(5*time.Minute) {
      util.Log(0, "ERROR! LDAP not available")
      util.LoggersFlush(5*time.Second)
      os.Exit(1)
    }
    util.Log(1, "INFO! LDAP available")
  
    setConfigUnitTag() // after config.ReadNetwork()
    config.FAIBase = db.LDAPFAIBase()
    util.Log(1, "INFO! FAI base: %v", config.FAIBase)
    util.Log(1, "INFO! ou=servers.conf: %v", config.LDAPServerOUs)
    os.MkdirAll(path.Dir(config.JobDBPath), 0750)
    db.ServersInit() // after config.ReadNetwork()
    db.JobsInit() // after config.ReadConfig()
    db.ClientsInit() // after config.ReadConfig()
    db.HooksExecute(true) // after config.ReadConfig()
    action.Init()
  }  
  
  // Create channels for receiving events. 
  // The main() goroutine receives on all these channels 
  // and spawns new goroutines to handle the incoming events.
  tcp_connections := make(chan *net.TCPConn, 32)
  // NOTE: signals channel is created at the beginning of main()
  
  util.Log(1, "INFO! Intercepting these signals: %v", signals_to_watch)
  
  util.Log(1, "INFO! Accepting gosa-si protocol connections on TCP port %v", strings.SplitN(config.ServerSourceAddress,":",2)[1]);
  go acceptConnections(listener, tcp_connections)
  
  go util.WithPanicHandler(faiProgressWatch)
  
  if config.RunServer {
    if config.FAIMonPort != "disabled" {
      util.Log(1, "INFO! Accepting FAI monitoring messages on TCP port %v", config.FAIMonPort)
      go faimon(":"+config.FAIMonPort)
    }
  
    util.Log(1, "INFO! Accepting TFTP requests on UDP port %v", config.TFTPPort)
    go tftp.ListenAndServe(":"+config.TFTPPort, config.TFTPRegexes, config.TFTPReplies)

    go message.CheckPossibleClients()
    go message.Broadcast_new_server()
    go message.DistributeForeignJobUpdates()
  }
  
  // http server for profiling
  //go func(){http.ListenAndServe("localhost:6060", nil)}()
  
  go message.RegistrationHandler()

  /********************  main event loop ***********************/  
  for{ 
    select {
      case sig := <-signals : //os.Signal
                    if sig != syscall.SIGTTOU { // don't log SIGTTOU as that may cause another
                      util.Log(1, "INFO! Received signal \"%v\"", sig)
                    }
                    if sig == syscall.SIGUSR2 && config.RunServer { 
                      db.HooksExecute(false)
                    }
                    if sig == syscall.SIGHUP || sig == syscall.SIGTERM || 
                       sig == syscall.SIGQUIT || sig == syscall.SIGINT {
                       Shutdown = true
                       util.Log(0, "WARNING! Shutting down!")
                       util.Log(1, "INFO! Shutting down listener")
                       listener.Close()
                       if config.RunServer {
                         wait := make(chan bool,16)
                         go func(){ db.JobsShutdown(); wait<-true }()
                         go func(){ db.ServersShutdown(); wait<-true }()
                         go func(){ db.ClientsShutdown(); wait<-true }()
                         <-wait // for jobdb
                         <-wait // for serverdb
                         <-wait // for clientdb
                       }
                       config.Shutdown()
                       util.Log(1, "INFO! Average request processing time: %v", time.Duration((atomic.LoadInt64(&message.RequestProcessingTime)+50)/100))
                       util.Log(1, "INFO! Databases have been saved => Exit program")
                       util.LoggersFlush(5*time.Second)
                       os.Exit(0)
                    }
                    
      case conn:= <-tcp_connections : // *net.TCPConn
                    if Shutdown { 
                      util.Log(1, "INFO! Rejecting TCP request from %v because of go-susi shutdown", conn.RemoteAddr())
                      conn.Close() 
                    } else {
                      //util.Log(2, "DEBUG! Incoming TCP request from %v", conn.RemoteAddr())
                      go util.WithPanicHandler(func(){handle_request(conn)})
                    }
    }
  }
}

// Accepts TCP connections on listener and sends them on the channel tcp_connections.
func acceptConnections(listener *net.TCPListener, tcp_connections chan<- *net.TCPConn) {
  for {
    message := true
    for { // if we've reached the maximum number of connections, wait
      if atomic.AddInt32(&ActiveConnections, 1) <= config.MaxConnections { break }
      atomic.AddInt32(&ActiveConnections, -1)
      if message {
        util.Log(0, "WARNING! Maximum number of %v active connections reached => Throttling", config.MaxConnections)
        message = false
      }
      time.Sleep(100*time.Millisecond)
    }
    tcpConn, err := listener.AcceptTCP()
    if err != nil { 
      if Shutdown { return }
      util.Log(0, "ERROR! AcceptTCP: %v", err) 
    } else {
      tcp_connections <- tcpConn
    }
  }
}

var starttls = []byte{'S','T','A','R','T','T','L','S','\n'}

// Handles one or more messages received over conn. Each message is a single
// line terminated by \n. The message may be encrypted as by message.GosaEncrypt().
func handle_request(tcpconn *net.TCPConn) {
  defer tcpconn.Close()
  defer atomic.AddInt32(&ActiveConnections, -1)
  //defer util.Log(2, "DEBUG! Connection to %v closed", conn.RemoteAddr())
  
  var err error
  
  err = tcpconn.SetKeepAlive(true)
  if err != nil {
    util.Log(0, "ERROR! SetKeepAlive: %v", err)
  }
  
  var buf bytes.Buffer
  defer buf.Reset()
  readbuf := make([]byte, 4096)
  
  var conn net.Conn
  conn = tcpconn
  n := 1
  
  if config.TLSServerConfig != nil {
    for i := range starttls {
      n, err = conn.Read(readbuf[0:1])
      if n == 0 {
        util.Log(0, "ERROR! Read error while looking for STARTTLS: %v", err)
        return
      }
      buf.Write(readbuf[0:1])
      if readbuf[0] != starttls[i] { 
        if config.TLSRequired {
          util.Log(0, "ERROR! No STARTTLS from %v, but TLS is required", conn.RemoteAddr())
          return
        }
        break 
      }
      if readbuf[0] == '\n' {
        buf.Reset() // purge STARTTLS\n from buffer
        conn = tls.Server(conn, config.TLSServerConfig)
      }
    }
  }
  
  context := security.ContextFor(conn)
  if context == nil { return }
  
  for n != 0 {
    //util.Log(2, "DEBUG! Receiving from %v", conn.RemoteAddr())
    n, err = conn.Read(readbuf)
    
    if err != nil && err != io.EOF {
      util.Log(0, "ERROR! Read: %v", err)
    }
    if err == io.EOF {
      util.Log(2, "DEBUG! Connection closed by %v", conn.RemoteAddr())
    }
    if n == 0 && err == nil {
      util.Log(0, "ERROR! Read 0 bytes but no error reported")
    }
    
    // Find complete lines terminated by '\n' and process them.
    for start := 0;; {
      eol := start
      for ; eol < n; eol++ {
        if readbuf[eol] == '\n' { break }
      }
      
      // no \n found, append to buf and continue reading
      if eol == n {
        buf.Write(readbuf[start:n])
        break
      }
      
      // append to rest of line to buffered contents
      buf.Write(readbuf[start:eol])
      start = eol+1
      
      buf.TrimSpace()
      
      // process the message and get a reply (if applicable)
      if buf.Len() > 0 { // ignore empty lines
        request_start := time.Now()
        reply, disconnect := message.ProcessEncryptedMessage(&buf, context)
        buf.Reset()
        request_time := time.Since(request_start)
        RequestProcessingTimes.Push(request_time)
        request_time -= RequestProcessingTimes.Next().(time.Duration)
        atomic.AddInt64(&message.RequestProcessingTime, int64(request_time))
        
        if reply.Len() > 0 {
          util.Log(2, "DEBUG! Sending %v bytes reply to %v", reply.Len(), conn.RemoteAddr())
          
          var deadline time.Time // zero value means "no deadline"
          if config.Timeout >= 0 { deadline = time.Now().Add(config.Timeout) }
          conn.SetWriteDeadline(deadline)
  
          _, err := util.WriteAll(conn, reply.Bytes())
          if err != nil {
            util.Log(0, "ERROR! WriteAll: %v", err)
          }
          reply.Reset()
          util.WriteAll(conn, []byte{'\r','\n'})
        }
        
        if disconnect {
          util.Log(1, "INFO! Forcing disconnect of %v because of error", conn.RemoteAddr())
          return
        }
        
        if Shutdown {
          util.Log(1, "INFO! Forcing disconnect of %v because of go-susi shutdown", conn.RemoteAddr())
          return
        }
      }
    }
  }
  
  if  buf.Len() != 0 {
    util.Log(0, "ERROR! Incomplete message from %v (i.e. not terminated by \"\\n\") of %v bytes: %v", conn.RemoteAddr(),buf.Len(), buf.String())
  }
}
func setConfigUnitTag() {
  util.Log(1, "INFO! Getting my own system's gosaUnitTag from LDAP")
  config.UnitTag = db.SystemGetState(config.MAC, "gosaUnitTag")
  if config.UnitTag == "" {
    util.Log(1, "INFO! No gosaUnitTag found for %v => gosaUnitTag support disabled", config.MAC)
  } else {
    config.UnitTagFilter = "(gosaUnitTag="+config.UnitTag+")"
    config.AdminBase, config.Department = db.LDAPAdminBase()
    util.Log(1, "INFO! gosaUnitTag: %v  Admin base: %v  Department: %v", config.UnitTag, config.AdminBase, config.Department)
  }
}

func printStats() int {
  msg := "<xml><header>sistats</header></xml>"
  encrypted := message.GosaEncrypt(msg, config.ModuleKey["[GOsaPackages]"])
  conn, err := net.Dial("tcp", config.ServerSourceAddress)
  if err != nil {
    fmt.Fprintf(os.Stderr, "Error connecting with %v: %v\n", config.ServerSourceAddress, err)
    return 1
  }
  defer conn.Close()
  err = util.SendLn(conn, encrypted, config.Timeout)
  if err != nil {
    fmt.Fprintf(os.Stderr, "Error sending to %v: %v\n", config.ServerSourceAddress, err)
    return 1
  }
  reply,_ := util.ReadLn(conn, 10*time.Second)
  decrypted := message.GosaDecrypt(reply, config.ModuleKey["[GOsaPackages]"])
  x,_ := xml.StringToHash(decrypted)
  x = x.First("answer1")
  if x == nil {
    fmt.Fprintf(os.Stderr, "No results received!\n")
    return 1
  }
  for c := x.FirstChild(); c != nil; c = c.Next() {
    fmt.Println(c.Element().Name()+": "+c.Element().Text())
  }
  return 0
}

func faimon(listen_address string) {
  listener, err := net.Listen("tcp", listen_address)
  if err != nil {
    util.Log(0, "ERROR! Cannot start FAI monitor: %v", err)
    return
  }
  
  for {
    conn, err := listener.Accept()
    if err != nil { 
      util.Log(0,"ERROR! FAI monitor error: %v", err)
      continue
    }
    
    go util.WithPanicHandler(func(){faiConnection(conn.(*net.TCPConn))})
  }
}

func faiConnection(conn *net.TCPConn) {
  defer conn.Close()
  var err error
  
  err = conn.SetKeepAlive(true)
  if err != nil {
    util.Log(0, "ERROR! SetKeepAlive: %v", err)
  }
  
  var buf bytes.Buffer
  defer buf.Reset()
  readbuf := make([]byte, 4096)
  n := 1
  for n != 0 {
    n, err = conn.Read(readbuf)
    if err != nil && err != io.EOF {
      util.Log(0, "ERROR! Read: %v", err)
    }
    if n == 0 && err == nil {
      util.Log(0, "ERROR! Read 0 bytes but no error reported")
    }
    
    // Find complete lines terminated by '\n' and process them.
    for start := 0;; {
      eol := start
      for ; eol < n; eol++ {
        if readbuf[eol] == '\n' { break }
      }
      
      // no \n found, append to buf and continue reading
      if eol == n {
        buf.Write(readbuf[start:n])
        break
      }
      
      // append to rest of line to buffered contents
      buf.Write(readbuf[start:eol])
      start = eol+1
      
      buf.TrimSpace()
      
      util.Log(2, "DEBUG! FAI monitor message from %v: %v", conn.RemoteAddr(), buf.String())
      buf.Reset()
    }
  }
  
  if  buf.Len() != 0 {
    util.Log(2, "DEBUG! Incomplete FAI monitor message (i.e. not terminated by \"\\n\") from %v: %v", conn.RemoteAddr(), buf.String())
  }
}

func faiProgressWatch() {
  clientpackageskey := config.ModuleKey["[ClientPackages]"]
  // If [ClientPackages]/key missing, take the last key in the list
  // (We don't take the 1st because that would be "dummy-key").
  if clientpackageskey == "" { clientpackageskey = config.ModuleKeys[len(config.ModuleKeys)-1] }
  
  util.Log(1, "INFO! Launching fai-progress-hook %v", config.FAIProgressHookPath)
  env := config.HookEnvironment()
  cmd := exec.Command(config.FAIProgressHookPath)
  cmd.Env = append(env, os.Environ()...)

  out, err := cmd.StdoutPipe()
  if err != nil {
    util.Log(0, "ERROR! Could not get stdout pipe for %v: %v => FAI progress monitoring disabled", config.FAIProgressHookPath, err)
    return
  }

  err = cmd.Start()
  if err != nil {
    util.Log(0, "ERROR! Could not launch %v: %v => FAI progress monitoring disabled", config.FAIProgressHookPath, err)
    return
  }
  
  reader := bufio.NewReader(out)
  for {
    line, err := reader.ReadString('\n')
    if err != nil {
      util.Log(0, "ERROR! Error reading stdout from FAI progress monitor %v: %v => FAI progress monitoring disabled", config.FAIProgressHookPath, err)
      return
    }

    line = strings.TrimSpace(line)
    util.Log(1, "INFO! FAI progress event: \"%v\"", line)
    idx := strings.Index(line," ")
    if idx < 0 {
      util.Log(0, "WARNING! Ignoring strange FAI progress event \"%v\"", line)
      continue
    }
    
    header := "CLMSG_" + line[0:idx]
    content := line[idx+1:]
    x := xml.NewHash("xml","header",header)
    x.Add("source", config.ServerSourceAddress)
    target := message.CurrentRegistrationServer()
    x.Add("target", target)
    x.Add(header,content)
    x.Add("macaddress", config.MAC)
    x.Add("timestamp", util.MakeTimestamp(time.Now()))
    
    util.Log(2, "DEBUG! Sending to %v: %v", target, x)
    msg := x.String()
    go func(){util.SendLnTo(target, message.GosaEncrypt(msg, clientpackageskey), config.Timeout)}()
    
    if strings.HasPrefix(line, "TASKEND savelog") { 
      message.Send_clmsg_save_fai_log(target, config.FAISavelogHookPath)
    }
  }
}
