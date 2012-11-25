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
            case "trigger_action_lock":      Lock(job)      // "Sperre"
            case "trigger_action_halt":      Halt(job)      // "Anhalten"
            case "trigger_action_localboot": Localboot(job) // "Erzwinge lokalen Start"
            case "trigger_action_reboot":    Reboot(job)    // "Neustarten"
            case "trigger_action_activate":  Activate(job)  // "Sperre aufheben"
            case "trigger_action_wake":      Wake(job)      // "Aufwecken"
            case "trigger_action_update":    Update(job)    // "Aktualisieren"
                                             done = false
            case "trigger_action_reinstall": Reinstall(job) // "Neuinstallation"
                                             done = false
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
