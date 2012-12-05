/*
Copyright (c) 2012 Landeshauptstadt München
Author: Matthias S. Benkmann

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

package main

import (
          "io"
          "os"
          "os/signal"
          "fmt"
          "net"
          "time"
          "bytes"
          "strconv"
          "strings"
          "syscall"
          "regexp"
          
          "../db"
          "../util"
          "../config"
//          "../message"
       )

const HELP_MESSAGE = `# TODO: Write help message`

func main() {
  config.ReadArgs(os.Args[1:])
  
  if config.PrintVersion {
    fmt.Printf(`sibridge %v (revision %v)
Copyright (c) 2012 Landeshauptstadt München
Author: Matthias S. Benkmann
This is free software; see the source for copying conditions.  There is NO
warranty; not even for MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

`, config.Version, config.Revision)
  }
  
  if config.PrintHelp {
    fmt.Println(`USAGE: sibridge [args]
sibridge listens on the siserver port +10. Connect to that port with nc
and issue the "help" command to get instructions.

--help       print this text and exit
--version    print version and exit

-c <file>    read config from <file> instead of default location
`)
  }
  
  if config.PrintVersion || config.PrintHelp { os.Exit(0) }
  
  config.ReadConfig()
  colon := strings.Index(config.ServerListenAddress, ":")
  port,_ := strconv.Atoi(config.ServerListenAddress[colon+1:])
  config.ServerListenAddress = fmt.Sprintf("127.0.0.1:%d", port+10) 
  config.ServerSourceAddress = config.IP + config.ServerListenAddress[strings.Index(config.ServerListenAddress,":"):]
  
  util.LogLevel = config.LogLevel
  
  config.ReadNetwork() // after config.ReadConfig()
  
  tcp_addr, err := net.ResolveTCPAddr("ip4", config.ServerListenAddress)
  if err != nil {
    util.Log(0, "ERROR! ResolveTCPAddr: %v", err)
    os.Exit(1)
  }

  listener, err := net.ListenTCP("tcp4", tcp_addr)
  if err != nil {
    util.Log(0, "ERROR! ListenTCP: %v", err)
    os.Exit(1)
  }
  
  // Create channels for receiving events. 
  // The main() goroutine receives on all these channels 
  // and spawns new goroutines to handle the incoming events.
  tcp_connections := make(chan *net.TCPConn, 32)
  signals         := make(chan os.Signal, 32)
  
  signals_to_watch := []os.Signal{ syscall.SIGUSR1, syscall.SIGUSR2 }
  signal.Notify(signals, signals_to_watch...)
  
  util.Log(1, "INFO! Accepting connections on %v", tcp_addr);
  go acceptConnections(listener, tcp_connections)
  
  /********************  main event loop ***********************/  
  for{ 
    select {
      case sig := <-signals : //os.Signal
                    util.Log(1, "Received signal \"%v\"", sig)
                    
      case conn:= <-tcp_connections : // *net.TCPConn
                    util.Log(1, "INFO! Incoming TCP request from %v", conn.RemoteAddr())
                    go util.WithPanicHandler(func(){handle_request(conn)})
    }
  }
}

// Accepts TCP connections on listener and sends them on the channel tcp_connections.
func acceptConnections(listener *net.TCPListener, tcp_connections chan<- *net.TCPConn) {
  for {
    tcpConn, err := listener.AcceptTCP()
    if err != nil { 
      util.Log(0, "ERROR! AcceptTCP: %v", err) 
    } else {
      tcp_connections <- tcpConn
    }
  }
}

// Handles one or more messages received over conn. Each message is a single
// line terminated by \n. The message may be encrypted as by message.GosaEncrypt().
func handle_request(conn *net.TCPConn) {
  defer conn.Close()
  defer util.Log(1, "INFO! Connection to %v closed", conn.RemoteAddr())
  
  var err error
  
  err = conn.SetKeepAlive(true)
  if err != nil {
    util.Log(0, "ERROR! SetKeepAlive: %v", err)
  }
  
  // If the user does not specify any machines in the command,
  // the list of machines from the previous command will be used.
  // The following slice is passed via pointer with every call of
  // processMessage() so that each call can access the previous call's data
  jobs := []jobDescriptor{}
  
  util.SendLn(conn, "# Enter \"help\" to get a list of commands.\n# Ctrl-D terminates the connection.\n", config.Timeout)
  
  repeat := time.Duration(0)
  repeat_command := ""
  var buf = make([]byte, 65536)
  i := 0
  n := 1
  for n != 0 {
    util.Log(2, "DEBUG! Receiving from %v", conn.RemoteAddr())
    
    var deadline time.Time // zero value means "no deadline"
    if repeat > 0 { deadline = time.Now().Add(repeat) }
    conn.SetDeadline(deadline)
    n, err = conn.Read(buf[i:])
    if neterr,ok := err.(net.Error); ok && neterr.Timeout() {
      n = copy(buf[i:], repeat_command)
      err = nil
    }
    
    repeat = 0  
    i += n
    
    if err != nil && err != io.EOF {
      util.Log(0, "ERROR! Read: %v", err)
    }
    if err == io.EOF {
      util.Log(2, "DEBUG! Connection closed by %v", conn.RemoteAddr())
      
    }
    if n == 0 && err == nil {
      util.Log(0, "ERROR! Read 0 bytes but no error reported")
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
      message := strings.ToLower(strings.TrimSpace(string(buf[start:start+eol])))
      start += eol+1
      if message != "" { // ignore empty lines
        var reply string
        reply,repeat = processMessage(message, &jobs, conn.RemoteAddr().(*net.TCPAddr))
        repeat_command = message + "\n"
        
        // if we already have more data, cancel repeat immediately
        if start < i { repeat = 0 }
        
        if reply != "" {
          util.Log(2, "DEBUG! Sending reply to %v: %v", conn.RemoteAddr(), reply)
          util.SendLn(conn, reply, config.Timeout)
        }
      }
    }
  }
  
  if  i != 0 {
    util.Log(0, "ERROR! Incomplete message (i.e. not terminated by \"\\n\") of %v bytes: %v", i, buf[0:i])
  }
}

var jobs      = []string{"update","softupdate","reboot","halt","install",  "reinstall","wakeup","localboot","lock","unlock",  "activate"}
var commands  = append(jobs,                                                                                                             "help","x",      "examine", "query_jobdb","query_jobs","jobs", "delete_jobs","delete_jobdb_entry","qq","xx")
var canonical = []string{"update","update"    ,"reboot","halt","reinstall","reinstall",  "wake","localboot","lock","activate","activate","help","examine","examine", "query",      "query",     "query","delete",     "delete"            ,"qq","xx"}

type jobDescriptor struct {
  MAC string
  IP string
  Name string // "*" means all machines
  Timestamp string
  Job string  // "*" means all jobs
}

// msg must be non-empty.
// joblist: see comment in handle_request() for explanation
//
// Returns:
//  reply: text to send back to the requestor
//  repeat: if non-0, if the requestor does not send anything within that time, repeat the same command
func processMessage(msg string, joblist *[]jobDescriptor, remote_addr *net.TCPAddr) (reply string, repeat time.Duration) {
  fields := strings.Fields(msg)
  cmd := fields[0] // always present because msg is non-empty
  i := 0
  for ; i < len(commands); i++ {
    if commands[i] == "halt" && cmd == "h" { continue }  // "h" should be short for "help" not "halt"
    if strings.HasPrefix(commands[i], cmd) { break }
  }
  
  if i == len(commands) {
    return "! Unrecognized command: " + cmd, 0
  }
  
  cmd = canonical[i]
  
  if cmd == "help" { return HELP_MESSAGE,0 }
  
  is_job_cmd := (i < len(jobs))
  
  allowed := map[string]bool{"machine":true}
  if is_job_cmd { allowed["time"] = true }
  if cmd == "delete" { allowed["job"]=true }
  
  template := jobDescriptor{Name:"*", Job:"*", Timestamp:util.MakeTimestamp(time.Now())}
  
  if is_job_cmd {
    for k := range *joblist {
      (*joblist)[k].Job = cmd
    }
    
    template.Job = cmd
  }
  
  have_machine := false
  
  for i=1; i < len(fields); i++ {
    if allowed["time"] {
      if parseTime(fields[i], &template) {
        if !have_machine {
          for k := range *joblist {
            (*joblist)[k].Timestamp = template.Timestamp
          }
        }
        continue
      }
    }
    
    // test machine names before jobs. Otherwise many valid machine names such as "rei" would
    // be interpreted as job types ("reinstall" in the example)
    if allowed["machine"] {
      if parseMachine(fields[i], &template) {
        if !have_machine { *joblist = []jobDescriptor{} }
        have_machine = true
        *joblist = append(*joblist, template)
        continue
      }
    }
    
    if allowed["job"] {
      if parseJob(fields[i], &template) {
        if !have_machine {
          for k := range *joblist {
            (*joblist)[k].Job = template.Job
          }
        }
        continue
      }
    }
    
    return "! Illegal argument: "+fields[i],0
  }
  
  reply = ""
  repeat = 0
  
  util.Log(2, "DEBUG! Handling command \"%v\"", cmd)
  
  if is_job_cmd {
    reply = commandJob(joblist)
  } else if cmd == "qq" {
    reply = commandGosa("gosa_query_jobdb", false,joblist)
    repeat = 5*time.Second
  } else if cmd == "xx" {
    reply = commandExamine(joblist)
    repeat = 2*time.Second
  } else if cmd == "examine" {
    reply = commandExamine(joblist)
  } else if cmd == "query" {
    reply = commandGosa("gosa_query_jobdb",false,joblist)
  } else if cmd == "delete" {
    reply = commandGosa("gosa_delete_jobdb_entry",true,joblist)
  }
  
  return reply,repeat
}

func commandJob(joblist *[]jobDescriptor) (reply string) {
  for _, j := range *joblist {
    if j.Job == "*" { continue }
    if j.Name == "*" { continue }
    
    if reply != "" {reply = reply + "\n" }
    reply = reply + fmt.Sprintf("=> %-10v %v  %v (%v)", j.Job, util.ParseTimestamp(j.Timestamp).Format("2006-01-02 15:04:05"), j.MAC, j.Name)
    xmlmess := fmt.Sprintf("<xml><header>job_trigger_action_%v</header><source>GOSA</source><target>%v</target><macaddress>%v</macaddress><timestamp>%v</timestamp></xml>", j.Job, j.MAC, j.MAC, j.Timestamp)
    util.Log(2, "DEBUG! %v",xmlmess)
  }
  return reply
}

// + active 1c:6f:65:08:b5:4d (nova) "localboot" :plophos
// - active 1c:6f:65:08:b5:4d (nova) "localboot" :plophos/4.1.0
func commandExamine(joblist *[]jobDescriptor) (reply string) {
  for _, j := range *joblist {
    if j.Name == "*" { continue }
    
    ssh_reachable := make(chan bool, 2)
    go func() {
      conn, err := net.Dial("tcp", j.Name+":22")
      if err != nil {
        util.Log(0, "ERROR! Dial(\"tcp\",%v:22): %v",j.Name,err)
        ssh_reachable <- false
      } else {
        conn.Close()
        ssh_reachable <- true
      }
    }()
    
    go func() {
      time.Sleep(250*time.Millisecond)
      ssh_reachable <- false
    }()
    
    gotomode := db.SystemGetState(j.MAC, "gotoMode")
    faistate := db.SystemGetState(j.MAC, "faistate")
    faiclass := db.SystemGetState(j.MAC, "faiclass")
    release := "unknown"
    if strings.Index(faiclass,":")>=0 { release = faiclass[strings.Index(faiclass,":"):] }
    ssh := <- ssh_reachable
    if reply != "" { reply += "\n" }
    if ssh { reply += "+ " } else { reply += "- " }
    reply += fmt.Sprintf("%v %v (%v) \"%v\" %v",gotomode,j.MAC,j.Name,faistate,release)
  }
  
  return reply
}

// This difficult function is only necessary because stupid gosa-si requires queries to be in CNF.
// So we need to convert our DNF jobDescriptors into long and ugly CNF clauses.
func generate_clauses(joblist *[]jobDescriptor, idx int, machines *map[string]bool, jobtypes *map[string]bool, clauses *string) {
  if idx == len(*joblist) {
    if len(*machines) > 0 || len(*jobtypes) > 0 {
      *clauses = *clauses + "<clause><connector>or</connector>"
      for m := range *machines {
        *clauses = *clauses + "<phrase><macaddress>"+m+"</macaddress></phrase>"
      }
      for j := range *jobtypes {
        *clauses = *clauses + "<phrase><headertag>trigger_action_"+j+"</headertag></phrase>"
      }
      *clauses = *clauses + "</clause>"
    }
  } else {
    job := (*joblist)[idx]
    if job.Name == "*" && job.Job == "*" {
      // do nothing. Don't even recurse because this is an always true case
      // In fact if this case is encountered we could abort the whole generate_clauses because
      // it must end up being empty.
    } else if job.Name != "*" && job.Job != "*" {
      // We can optimize away one branch of the recursion if it doesn't add anything new,
      // but we must not trim both, because we must recurse to i==len(*joblist) for the
      // clause to be generated.
      
      one_branch_done := false
      if !(*jobtypes)[job.Job] {
        (*jobtypes)[job.Job] = true
        generate_clauses(joblist, idx+1, machines, jobtypes, clauses)
        delete(*jobtypes, job.Job)
        one_branch_done = true
      }
      
      have_machine := (*machines)[job.MAC]
      if !have_machine || !one_branch_done {
        (*machines)[job.MAC] = true
        generate_clauses(joblist, idx+1, machines, jobtypes, clauses)
        if !have_machine { delete(*machines, job.MAC) }
      }
    } else { // if either job.Name != "*" or job.Job != "*" but not both
      if job.Job != "*" {
        have_type := (*jobtypes)[job.Job]
        (*jobtypes)[job.Job] = true
        generate_clauses(joblist, idx+1, machines, jobtypes, clauses)
        if !have_type { delete(*jobtypes, job.Job) }
      } else {
        have_machine := (*machines)[job.MAC]
        (*machines)[job.MAC] = true
        generate_clauses(joblist, idx+1, machines, jobtypes, clauses)
        if !have_machine { delete(*machines, job.MAC) }
      }
    }
  }
}

func commandGosa(header string, use_job_type bool, joblist *[]jobDescriptor) (reply string) { 
  clauses := ""
  if use_job_type {
    machines := map[string]bool{}
    jobtypes := map[string]bool{}
    generate_clauses(joblist, 0, &machines, &jobtypes, &clauses)
  } else {
    for _, job := range *joblist {
      clauses = clauses + "<phrase><macaddress>"+job.MAC+"</macaddress></phrase>"
    }
    
    if clauses != "" {
      clauses = "<clause><connector>or</connector>" + clauses + "</clause>"
    }
  }

  gosa_cmd := "<xml><header>"+header+"</header><source>GOSA</source><target>GOSA</target><where>"+clauses+"</where></xml>"
  return gosa_cmd
}

const re_1xx = "(1([0-9]?[0-9]?))"
const re_2xx = "(2([6-9]|([0-4][0-9]?)|(5[0-5]?))?)"
const re_xx  = "([3-9][0-9]?)"
const ip_part = "(0|"+re_1xx+"|"+re_2xx+"|"+re_xx+")"
var ipRegexp = regexp.MustCompile("^"+ip_part+"([.]"+ip_part+"){3}$")
var macAddressRegexp = regexp.MustCompile("^[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}$")

func parseMachine(machine string, template *jobDescriptor) bool {
  var name string
  var ip string
  var mac string
  
  if machine == "*" {
    mac = "*"
    ip = "0.0.0.0"
    name = "*"
  } else if macAddressRegexp.MatchString(machine) {
    mac = machine
    name = db.SystemPlainnameForMAC(mac)
    if name == "none" { return false }
    ip = db.SystemIPAddressForName(name)
    if ip == "none" { ip = "0.0.0.0" }
  } else if ipRegexp.MatchString(machine) {
    ip = machine
    name = db.SystemNameForIPAddress(ip)
    if name == "none" { return false }
    mac = db.SystemMACForName(name)
    if mac == "none" { return false }
  } else {
    name = machine
    ip = db.SystemIPAddressForName(name)
    if ip == "none" { ip = "0.0.0.0" }
    mac = db.SystemMACForName(name)
    if mac == "none" { return false }
  }
  
  template.MAC = mac
  template.IP = ip
  template.Name = name
  
  return true
}


var dateRegexp = regexp.MustCompile("^20[0-9][0-9]-[0-1][0-9]-[0-3][0-9]$")
var timeRegexp = regexp.MustCompile("^[0-2]?[0-9]:[0-5]?[0-9](:[0-5]?[0-9])?$")
var duraRegexp = regexp.MustCompile("^[0-9]+[smhd]$")

func parseTime(t string, template *jobDescriptor) bool {
  if dateRegexp.MatchString(t) {
    template.Timestamp = strings.Replace(t,"-","",-1) + template.Timestamp[8:]
    return true
  }
  
  if timeRegexp.MatchString(t) {
    parts := strings.Split(t,":")
    t = parts[0]
    if len(t) < 2 { t = "0" + t }
    if len(parts[1]) < 2 { t = t + "0" }
    t += parts[1]
    if len(parts) < 3 { t = t + "00" 
    } else {
      if len(parts[2]) < 2 { t = t + "0" }
      t += parts[2]
    }
    
    template.Timestamp = template.Timestamp[:8] + t
    return true
  }
  
  if duraRegexp.MatchString(t) {
    n,_ := strconv.ParseUint(t[0:len(t)-1], 10, 64)
    var dura time.Duration
    switch t[len(t)-1] {
      case 's': dura = time.Duration(n)*time.Second
      case 'm': dura = time.Duration(n)*time.Minute
      case 'h': dura = time.Duration(n)*time.Hour
      case 'd': dura = time.Duration(n)*24*time.Hour
    }
    
    template.Timestamp = util.MakeTimestamp(util.ParseTimestamp(template.Timestamp).Add(dura))
    return true
  }
  
  return false
}

func parseJob(j string, template *jobDescriptor) bool {
  for i := range jobs {
    if strings.HasPrefix(jobs[i],j) {
      template.Job = canonical[i]
      return true
    }
  }

  return false
}
