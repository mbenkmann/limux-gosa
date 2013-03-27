package action

import (
         "time"
         
         "../db"
         "../xml"
         "../util"
       )

func Localboot(job *xml.Hash) {
  
  // retry for 30s
  endtime := time.Now().Add(30*time.Second)
  
  for ; time.Now().Before(endtime);  {
    db.SystemSetState(job.Text("macaddress"), "faiState", "localboot")
    
    // remove softupdate and install jobs ...
    job_types_to_kill := xml.FilterOr(
                         []xml.HashFilter{xml.FilterSimple("headertag","trigger_action_reinstall"),
                                          xml.FilterSimple("headertag","trigger_action_update")})
    // ... that are already happening or scheduled within the next 5 minutes ...
    timeframe := xml.FilterRel("timestamp", util.MakeTimestamp(time.Now().Add(5*time.Minute)),-1,0)
    // ... that affect the machine for which we force localboot
    target := xml.FilterSimple("macaddress", job.Text("macaddress"))
    filter := xml.FilterAnd([]xml.HashFilter{ job_types_to_kill,
                                                  timeframe,
                                                  target })
    db.JobsRemove(filter)
    
    // Wait a little and see if the jobs are gone
    time.Sleep(3*time.Second)
    if db.JobsQuery(filter).FirstChild() == nil { // if all jobs are gone
      // set state again just in case the job removal raced with something that set faistate
      db.SystemSetState(job.Text("macaddress"), "faiState", "localboot")
      return // we're done
    } // else if some jobs remained
    
    util.Log(2, "DEBUG! Force localboot: Some install/softupdate jobs remain => Retrying")
  }
  
  util.Log(0, "ERROR! Can't remove install/softupdate jobs to force localboot.")
}
