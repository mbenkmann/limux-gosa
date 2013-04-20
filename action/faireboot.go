package action

import (
         "strings"
         
         "../db"
         "../xml"
         "../util"
       )

func FAIReboot(job *xml.Hash) {
  macaddress := job.Text("macaddress")
  
  delete_system := false
  faistate := "error:pxe:-1:crit:Job aborted. System in undefined state."
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
  
  ForceFAIState(macaddress, faistate)
  
  if delete_system { db.SystemReplace(sys, nil) }
}
