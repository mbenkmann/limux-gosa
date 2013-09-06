package action

import "../db"
import "../xml"
import "../util"

func Reinstall(job *xml.Hash) {
  util.Log(1, "INFO! Changing faistate of %v to install", job.Text("macaddress"))
  db.SystemSetState(job.Text("macaddress"), "faiState", "install")
  Wake(job)
}
