package action

import "../xml"

func Reboot(job *xml.Hash) bool {
  /*
  send reboot command to gosa-si-client
  */
  
  // In case the machine is sleeping, wake it up, because user
  // expects machine to be on after a reboot.
  // NOTE: Other jobs call Reboot() and expect the Wake().
  Wake(job)
  return true
}
