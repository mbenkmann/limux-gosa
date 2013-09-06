package action

import "../db"
import "../xml"
import "../util"

func Update(job *xml.Hash) {
  util.Log(1, "INFO! Changing faistate of %v to softupdate", job.Text("macaddress"))
  db.SystemSetState(job.Text("macaddress"), "faiState", "softupdate")
  Wake(job)
}
