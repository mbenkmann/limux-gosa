package action

import (
         "time"
         
         "../db"
         "../xml"
         "../util"
       )

func Localboot(job *xml.Hash) {
  db.SystemSetState(job.Text("macaddress"), "faiState", "localboot")
  
  // remove softupdate and install jobs ...
  job_types_to_kill := xml.FilterOr(
                       []xml.HashFilter{xml.FilterSimple("headertag","trigger_action_install"),
                                        xml.FilterSimple("headertag","trigger_action_update")})
  // ... that are already happening or scheduled within the next 5 minutes ...
  timeframe := xml.FilterRel("timestamp", util.MakeTimestamp(time.Now().Add(5*time.Minute)),-1,0)
  // ... that affect the machine for which we force localboot
  target := xml.FilterSimple("macaddress", job.Text("macaddress"))
  
  db.JobsRemove(xml.FilterAnd([]xml.HashFilter{ job_types_to_kill,
                                                timeframe,
                                                target }))
  
  // Wait a little and set state to localboot again, just in case the job
  // removal raced with something that set the faiState
  time.Sleep(2*time.Second)
  db.SystemSetState(job.Text("macaddress"), "faiState", "localboot")
}
