package action

import "../db"
import "../xml"

func Update(job *xml.Hash) bool {
  db.SystemSetState(job.Text("macaddress"), "faiState", "softupdate")
  Reboot(job)
  return false
}
