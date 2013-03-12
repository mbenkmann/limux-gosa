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
          done := true
          switch job.Text("headertag") {
                 // Jobs that I can always execute myself
            case "trigger_action_wake":      Wake(job)      // "Aufwecken"
            case "trigger_action_lock":      Lock(job)      // "Sperre"
            case "trigger_action_localboot": Localboot(job) // "Erzwinge lokalen Start"
            
                 // Jobs that I need to forward to a peer if the affected client
                 // is registered there
            case "trigger_action_halt":                     // "Anhalten"
                                             done = Forward(job) || Halt(job)
            case "trigger_action_reboot":                   // "Neustarten"
                                             done = Forward(job) || Reboot(job)    
            case "trigger_action_activate":                 // "Sperre aufheben"
                                             done = Forward(job) || Activate(job)  
            case "trigger_action_update":                   // "Aktualisieren"
                                             done = Forward(job) || Update(job)    
            case "trigger_action_reinstall":                // "Neuinstallation"
                                             done = Forward(job) || Reinstall(job)
            default:
                 util.Log(0, "ERROR! Unknown headertag in PendingActions for job: %v", job)
          }
          
          if done { db.JobsRemoveLocal(xml.FilterSimple("id", job.Text("id")), false) }
        })
        
      } else // if status == "done"
      {
        util.Log(1, "INFO! Job is done or cancelled: %v", job)
        
        go util.WithPanicHandler(func(){
        
          switch job.Text("headertag") {
            case "trigger_action_lock":      // "Sperre"
            case "trigger_action_halt":      // "Anhalten"
            case "trigger_action_localboot": // "Erzwinge lokalen Start"
            case "trigger_action_reboot":    // "Neustarten"
            case "trigger_action_activate":  // "Sperre aufheben"
            case "trigger_action_wake":      // "Aufwecken"
            case "trigger_action_update":    // "Aktualisieren"
            case "trigger_action_reinstall": // "Neuinstallation"
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
// job has <progress>forward-failed</progress>, this function returns false.
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
  
  macaddress := job.Text("macaddress")
  
  client := db.ClientWithMAC(macaddress)
  if client == nil || client.Text("source") == config.ServerSourceAddress {
    return false
  }
  
  siserver := client.Text("source")
  headertag := job.Text("headertag")
  
  util.Log(1, "INFO! %v for client %v must be forwarded to server %v where client is registered", headertag, macaddress, siserver)
  
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
    
  job_trigger_action := xml.NewHash("xml","header","job_"+headertag)
  job_trigger_action.Add("source","GOSA")
  job_trigger_action.Add("macaddress",macaddress)
  job_trigger_action.Add("target",macaddress)
  if job.First("timestamp") != nil { job_trigger_action.Add("timestamp", job.Text("timestamp")) }
  if job.First("periodic") != nil  { job_trigger_action.Add("periodic",  job.Text("periodic")) }
  
  reply := <-message.Peer(siserver).Ask(job_trigger_action.String(), config.ModuleKey["[GOsaPackages]"])
  if strings.Contains(reply,"error_string") {
    util.Log(0, "ERROR! Could not forward %v for client %v to server %v => Will try to execute job myself.", headertag, macaddress, siserver)
    
    job.FirstOrAdd("result").SetText("none")
    job.FirstOrAdd("progress").SetText("forward-failed")
    job.FirstOrAdd("status").SetText("waiting")
    
    util.Log(1, "INFO! Re-Scheduling job tagged with \"forward-failed\": %v", job)
    db.JobAddLocal(job)
  }
  
  return true
}
