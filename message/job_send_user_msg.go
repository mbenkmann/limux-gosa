/* 
Copyright (c) 2013 Matthias S. Benkmann

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
         "time"
         "strings"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

// Handles the message "job_send_user_msg".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func job_send_user_msg(xmlmsg *xml.Hash) *xml.Hash {
  job := xmlmsg.Clone()
  job.FirstOrAdd("progress").SetText("none")
  job.FirstOrAdd("status").SetText("waiting")
  job.FirstOrAdd("siserver").SetText(config.ServerSourceAddress)
  job.FirstOrAdd("modified").SetText("1")
  job.FirstOrAdd("targettag").SetText(xmlmsg.Text("target"))
  macaddress := config.MAC
  job.FirstOrAdd("macaddress").SetText(macaddress)
  job.FirstOrAdd("plainname").SetText("none") // updated automatically
  timestamp := xmlmsg.Text("timestamp")
    // go-susi does not use 19700101000000 as default timestamp as gosa-si does,
    // because that plays badly in conjunction with <periodic>
  if timestamp == "" { timestamp = util.MakeTimestamp(time.Now()) }
  job.FirstOrAdd("timestamp").SetText(timestamp)
  for _, periodic := range xmlmsg.Get("periodic") {
    job.FirstOrAdd("periodic").SetText(periodic) // last <periodic> wins if there are multiple
  }
  job.FirstOrAdd("headertag").SetText(strings.ToLower(xmlmsg.Text("header")[len("job_"):]))
  job.RemoveFirst("header")
  job.FirstOrAdd("result").SetText("none")
  
  db.JobAddLocal(job)
  
  answer := xml.NewHash("xml", "header", "answer")
  answer.Add("source", config.ServerSourceAddress)
  answer.Add("target", xmlmsg.Text("source"))
  answer.Add("answer1", "0")
  answer.Add("session_id", "1")
  return answer
}


