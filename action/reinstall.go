package action

import "../db"
import "../xml"

func Reinstall(job *xml.Hash) bool {
  db.SystemSetState(job.Text("macaddress"), "faiState", "install")
  Reboot(job)
  return false
}
