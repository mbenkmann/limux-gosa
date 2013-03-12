package action

import "../db"
import "../xml"

func Reinstall(job *xml.Hash) {
  db.SystemSetState(job.Text("macaddress"), "faiState", "install")
  Wake(job)
}
