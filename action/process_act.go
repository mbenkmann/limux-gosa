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

// The real action of any job happens here.
package action

import ( 
         "net"
         "time"
         "strings"
         "strconv"
         
         "../db"
         "../xml"
         "../util"
         "../config"
         "../message"
       )


// Infinite loop that consumes *xml.Hash job descriptors from
// db.PendingActions and launches goroutines to perform the appropriate
// action depending on the job's status ("done" or "processing").
// This function is also responsible for adding a new job when a periodic
// job is done.
func Init() { // not init() because we need to call it from go-susi.go
  go func() {
    for {
      job := db.PendingActions.Next().(*xml.Hash)
      
      if job.Text("status") != "done" {
        
        util.Log(1, "INFO! Taking action for job: %v", job)
        
        go util.WithPanicHandler(func(){
          
          if !Forward(job) {
            
            // Tell the lucky winner what we're going to do with it.
            
            macaddress := job.Text("macaddress")
            headertag  := job.Text("headertag")
            if headertag != "send_user_msg" { // send_user_msg does not target a machine
              client := db.ClientWithMAC(macaddress)
              if client == nil {
                util.Log(0, "ERROR! Client with MAC %v unknown. Cannot send %v", macaddress, headertag)
                // Don't abort. Some jobs work even if we can't reach the client.
              } else { 
                client_addr := client.Text("client")
                util.Log(1, "INFO! Sending %v to %v", headertag, client_addr)
                trigger_action := "<xml><header>"+headertag+"</header><"+headertag+"></"+headertag+"><source>"+config.ServerSourceAddress+"</source><target>"+client_addr+"</target></xml>"
                message.Client(client_addr).Tell(trigger_action, config.LocalClientMessageTTL)
              }
            }
            
            // Now that the client is rightfully excited, give it our best shot.
            
            done := true
            switch headertag {
              case "send_user_msg":            SendUserMsg(job)
              case "trigger_action_wake":      Wake(job)      // "Aufwecken"
              case "trigger_action_lock":      Lock(job)      // "Sperre"
              case "trigger_action_localboot": Localboot(job) // "Erzwinge lokalen Start"
              case "trigger_action_halt":      Halt(job)      // "Anhalten"
              case "trigger_action_reboot":    Reboot(job)    // "Neustarten"
              case "trigger_action_faireboot": FAIReboot(job) // "Job abbrechen"
              case "trigger_action_activate":  Activate(job)  // "Sperre aufheben"
              case "trigger_action_update":    Update(job)    // "Aktualisieren"
                                               done = false    
              case "trigger_action_reinstall": Reinstall(job) // "Neuinstallation"
                                               done = false
              default:
                   util.Log(0, "ERROR! Unknown headertag in PendingActions for job: %v", job)
            }
            
            if done { db.JobsRemoveLocal(xml.FilterSimple("id", job.Text("id")), false) }
          }
        })
        
      } else // if status == "done"
      {
        util.Log(1, "INFO! Job is done or cancelled: %v", job)
        
        go util.WithPanicHandler(func(){
        
          switch job.Text("headertag") {
            case "send_user_msg":
            case "trigger_action_lock":      // "Sperre"
            case "trigger_action_halt":      // "Anhalten"
            case "trigger_action_localboot": // "Erzwinge lokalen Start"
            case "trigger_action_reboot":    // "Neustarten"
            case "trigger_action_faireboot": // "Job abbrechen"
            case "trigger_action_activate":  // "Sperre aufheben"
            case "trigger_action_wake":      // "Aufwecken"
            
            case "trigger_action_update",    // "Aktualisieren"
                 "trigger_action_reinstall": // "Neuinstallation"
                 macaddress := job.Text("macaddress")
                 faistate := db.SystemGetState(macaddress, "faiState")
                 if strings.HasPrefix(faistate, "softupdat") || strings.HasPrefix(faistate, "install") {
                   util.Log(1, "INFO! Setting faiState \"localboot\" for client with MAC %v", macaddress)
                   db.SystemSetState(macaddress, "faiState", "localboot")
                 } else if faistate != "localboot" {
                   util.Log(1, "INFO! Leaving faiState \"%v\" alone for client with MAC %v", faistate, macaddress)
                 }
            
            default:
                 util.Log(0, "ERROR! Unknown headertag \"%v\" in PendingActions",job.Text("headertag"))
          }

          periodic := job.Text("periodic")
          if periodic != "none" && periodic != "" {
            t := util.ParseTimestamp(job.Text("timestamp"))
            p := strings.Split(periodic, "_")
            if len(p) != 2 {
              util.Log(0, "ERROR! Illegal <periodic>: %v", periodic)
              return
            }
            period, err := strconv.ParseUint(p[0], 10, 64)
            if err != nil || period == 0 {
              util.Log(0, "ERROR! Illegal <periodic>: %v: %v", periodic, err)
              return
            }
            
            for ; t.Before(time.Now()) ; {
              switch p[1] {
                case "seconds": t = t.Add(time.Duration(period) * time.Second)
                case "minutes": t = t.Add(time.Duration(period) * time.Minute)
                case "hours":   t = t.Add(time.Duration(period) * time.Hour)
                case "days":    t = t.AddDate(0,0,int(period))
                case "weeks":   t = t.AddDate(0,0,int(period*7))
                case "months":  t = t.AddDate(0,int(period),0)
                case "years":   t = t.AddDate(int(period),0,0)
                default:
                     util.Log(0, "ERROR! Unknown periodic unit: %v", p[1])
                     return
              }
            }
            job.FirstOrAdd("timestamp").SetText(util.MakeTimestamp(t))
            job.FirstOrAdd("result").SetText("none")
            job.FirstOrAdd("progress").SetText("none")
            job.FirstOrAdd("status").SetText("waiting")
            util.Log(1, "INFO! Scheduling next instance of periodic job: %v", job)
            db.JobAddLocal(job)
          }
        })
        
      }
    }
  }()
}

// If job belongs to an unknown client or a client registered here or if
// job has <progress>forward-failed</progress> or if the job's <headertag>
// is trigger_action_wake,_lock or _localboot or send_user_msg, 
// this function returns false.
// Otherwise this function removes the job from the jobdb and then tries to 
// forward the job to the siserver where the client is registered.
// If forwarding fails, the job is re-added to the jobdb but marked with 
// <progress>forward-failed</progress>. No matter if forwarding succeeds or fails, 
// if it is attempted this function returns true. Note, that the re-added job has
// a different id from the original job (which has been removed from the database)
// and will independently come up in PendingActions. This is why it doesn't make
// sense to return false in the case of a failed forward.
func Forward(job *xml.Hash) bool {
  if job.Text("progress") == "forward-failed" { return false }
  switch job.Text("headertag") { 
    case "send_user_msg": return false
    case "trigger_action_wake", "trigger_action_lock", "trigger_action_localboot": 
      return false
  }
  
  macaddress := job.Text("macaddress")
  
  client := db.ClientWithMAC(macaddress)
  if client == nil || client.Text("source") == config.ServerSourceAddress {
    return false
  }
  
  siserver := client.Text("source")
  headertag := job.Text("headertag")
  
  util.Log(1, "INFO! %v for client %v must be forwarded to server %v where client is registered", headertag, macaddress, siserver)
  
  if message.Peer(siserver).Downtime() != 0 {
    util.Log(0, "ERROR! Peer %v is down => Will try to execute %v for client %v myself.", siserver, headertag, macaddress)
    return false
  }
  
  // remove job with stop_periodic=true
  db.JobsRemoveLocal(xml.FilterSimple("id", job.Text("id")), true)
  
  if !message.Peer(siserver).IsGoSusi() {
    // Wait if the peer is not a go-susi, to prevent the fju caused by 
    // db.JobsRemoveLocal() above from killing the forwarded job; which 
    // might otherwise happen because gosa-si uses macaddress+headertag to
    // identify jobs and therefore cannot differentiate between the old and
    // the new job.
    time.Sleep(5*time.Second)
  }
    
  util.Log(1, "INFO! Forwarding %v for client %v to server %v", headertag, macaddress, siserver)

  // gosa-si-server does not seem to process some jobs when sent as job_...
  // So we use gosa_.... However this means that <periodic> won't work properly with
  // non-go-susi peers :-(
  gosa_trigger_action := xml.NewHash("xml","header","gosa_"+headertag)
  gosa_trigger_action.Add("source","GOSA")
  gosa_trigger_action.Add("macaddress",macaddress)
  gosa_trigger_action.Add("target",macaddress)
  if job.First("timestamp") != nil { gosa_trigger_action.Add("timestamp", job.Text("timestamp")) }
  if job.First("periodic") != nil  { gosa_trigger_action.Add("periodic",  job.Text("periodic")) }
  
  request := gosa_trigger_action.String()
  
  // clone job, because we want to use it in a new goroutine and don't want to risk
  // having it changed concurrently.
  job_clone := job.Clone()
  
  go util.WithPanicHandler(func(){
    tcpconn, err := net.Dial("tcp", siserver)
    if err == nil {
      defer tcpconn.Close()
      util.Log(2, "DEBUG! Forwarding to %v: %v", siserver, request)
      err = util.SendLn(tcpconn, message.GosaEncrypt(request, config.ModuleKey["[GOsaPackages]"]), 10*time.Second)
      if err == nil { return }
    }
    
    util.Log(0, "ERROR! %v: Could not forward %v for client %v to server %v => Will try to execute job myself.", err, headertag, macaddress, siserver)
    
    job_clone.FirstOrAdd("result").SetText("none")
    job_clone.FirstOrAdd("progress").SetText("forward-failed")
    job_clone.FirstOrAdd("status").SetText("waiting")
    
    util.Log(1, "INFO! Re-Scheduling job tagged with \"forward-failed\": %v", job_clone)
    db.JobAddLocal(job_clone)
  })
  
  return true
}
