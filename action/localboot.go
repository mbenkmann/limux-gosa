package action

import "../db"
import "../xml"

func Localboot(job *xml.Hash) {
  db.SystemSetState(job.Text("macaddress"), "faiState", "localboot")
}
