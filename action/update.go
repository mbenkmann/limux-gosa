package action

import "../db"
import "../xml"

func Update(job *xml.Hash) {
  db.SystemSetState(job.Text("macaddress"), "faiState", "softupdate")
  Wake(job)
}
