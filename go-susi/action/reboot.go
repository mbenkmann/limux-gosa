package action

import "../xml"

func Reboot(job *xml.Hash) {
  // In case the machine is sleeping, wake it up, because user
  // expects machine to be on after a reboot.
  Wake(job)
}
