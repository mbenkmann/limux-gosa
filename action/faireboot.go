package action

import (
         "strings"
         
         "../db"
         "../xml"
         "../util"
       )

func FAIReboot(job *xml.Hash) {
  macaddress := job.Text("macaddress")
  
  util.Log(0, "INFO! Aborting all running install and softupdate jobs for %v", macaddress)
  
  delete_system := false
  faistate := "error:fiddledidoo:-1:crit:Job aborted by admin. System in unknown state."
  sys, err := db.SystemGetAllDataForMAC(macaddress, false)
  if err != nil {
    util.Log(0, "ERROR! FAIReboot(): %v", err)
    // do not abort. Killing jobs may still work.
  } else {
    // If the system is in incoming, delete it because faimond-ldap does not
    // cope well with incomplete LDAP objects and tries to boot them from local disk.
    dnparts := strings.Split(sys.Text("dn"),",")
    if len(dnparts) > 1 && dnparts[1] == "ou=incoming" { delete_system = true }
  }
  
  db.SystemForceFAIState(macaddress, faistate)
  
  if delete_system { 
    util.Log(1, "INFO! System %v is in ou=incoming => Deleting LDAP entry", macaddress)
    err = db.SystemReplace(sys, nil) 
    if err != nil {
      util.Log(0, "ERROR! LDAP error while deleting %v: %v", macaddress, err)
    }
  }
}
