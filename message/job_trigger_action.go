/* 
Copyright (c) 2012 Matthias S. Benkmann

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, 
MA  02110-1301, USA.
*/

package message

import (
         "regexp"
         "strings"
         "encoding/base64"
         
         "../db"
         "../xml"
         "../config"
       )

var macAddressRegexp = regexp.MustCompile("^[:xdigit:](:[:xdigit:]){5}$")

// Handles all messages of the form "job_trigger_action_*".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func job_trigger_action(xmlmsg *xml.Hash) string {
  job := xml.NewHash("job")
  job.Add("progress", "none")
  job.Add("status", "waiting")
  job.Add("siserver", config.ServerSourceAddress)
  job.Add("modified", "1")
  job.Add("targettag", xmlmsg.Text("target"))
  macaddress := xmlmsg.Text("macaddress")
  if macaddress == "" { macaddress = xmlmsg.Text("target") }
  macaddress = strings.ToLower(macaddress)
  if !macAddressRegexp.MatchString(macaddress) {
    return ErrorReply("job_trigger_action* with invalid or missing MAC address")
  }
  job.Add("macaddress", macaddress)
  plainname := db.SystemNameForMAC(macaddress)
  job.Add("plainname", plainname)
  timestamp := xmlmsg.Text("timestamp")
  if timestamp == "" { timestamp = "19700101000000" }
  job.Add("timestamp", timestamp)
  for _, periodic := range xmlmsg.Get("periodic") {
    job.FirstOrAdd("periodic").SetText(periodic) // last <periodic> wins if there are multiple
  }
  job.Add("headertag", xmlmsg.Text("header")[len("job_"):])
  job.Add("result", "none")
  job.Add("xmlmessage", base64.StdEncoding.EncodeToString([]byte(xmlmsg.String())))
  
  db.JobUpdate(job)
  
  jobdb_xml := xml.NewHash("jobdb")
  jobdb_xml.AddWithOwnership(job)
  
  Broadcast_foreign_job_updates(jobdb_xml)
  
  answer := xml.NewHash("xml", "header", "answer")
  answer.Add("source", config.ServerSourceAddress)
  answer.Add("target", xmlmsg.Text("source"))
  answer.Add("answer1", "0")
  answer.Add("session_id", "1")
  return answer.String()
}


