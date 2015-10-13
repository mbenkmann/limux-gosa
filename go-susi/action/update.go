package action

import "time"
import "../db"
import "../xml"
import "github.com/mbenkmann/golib/util"
import "../config"

func Update(job *xml.Hash) {
  util.Log(1, "INFO! Changing faistate of %v to softupdate", job.Text("macaddress"))
  db.SystemSetState(job.Text("macaddress"), "faiState", "softupdate")
  // Wait before sending WOL to prevent the situation in issue #169.
  time.Sleep(config.ActionAnnouncementTTL)
  Wake(job)
}
