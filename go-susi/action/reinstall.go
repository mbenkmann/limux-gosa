package action

import "time"
import "../db"
import "../xml"
import "../util"
import "../config"

func Reinstall(job *xml.Hash) {
  util.Log(1, "INFO! Changing faistate of %v to install", job.Text("macaddress"))
  db.SystemSetState(job.Text("macaddress"), "faiState", "install")
  // Wait before sending WOL to prevent the situation in issue #169.
  time.Sleep(config.ActionAnnouncementTTL)
  Wake(job)
}
