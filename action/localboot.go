package action

import (
         "../db"
         "../xml"
       )

func Localboot(job *xml.Hash) {
  db.SystemForceFAIState(job.Text("macaddress"), "localboot")
}

