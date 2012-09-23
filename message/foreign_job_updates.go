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
         "strings"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

// Sends a foreign_job_updates message to target containing the given jobs.
//   target: e.g. "foo.example.com:20081"
//   jobs: Must have the following format (<...> is an arbitrary tag)
//         <...>
//           <...>
//               <plainname>grisham</plainname>
//               <progress>none</progress>
//               <status>done</status>
//               <siserver>1.2.3.4:20081</siserver>
//               <modified>1</modified>
//               <targettag>00:0c:29:50:a3:52</targettag>
//               <macaddress>00:0c:29:50:a3:52</macaddress>
//               <timestamp>20120906164734</timestamp>
//               <id>4</id>
//               <headertag>trigger_action_wake</headertag>
//               <result>none</result>
//               <xmlmessage>PHhtbD48aGVhZGVyPmpvYl90cmlnZ2VyX2FjdGlvbl93YWtlPC9oZWFkZXI+PHNvdXJjZT5HT1NBPC9zb3VyY2U+PHRhcmdldD4wMDowYzoyOTo1MDphMzo1MjwvdGFyZ2V0Pjx0aW1lc3RhbXA+MjAxMjA5MDYxNjQ3MzQ8L3RpbWVzdGFtcD48bWFjYWRkcmVzcz4wMDowYzoyOTo1MDphMzo1MjwvbWFjYWRkcmVzcz48L3htbD4=</xmlmessage>
//           </...>
//           <...>
//             ...
//           </...>
//         </...>
//
func Send_foreign_job_updates(target string, jobs *xml.Hash) {
  jobs = jobs.Clone()
  MakeAnswerList(jobs)
  jobs.Add("header", "foreign_job_updates")
  jobs.Add("source", config.ServerSourceAddress)
  jobs.Add("target", target)
  util.SendLnTo(target, EncryptForServer(target, jobs.String()))
}

// Asynchronously calls Send_foreign_job_updates(target, jobs) for all
// target servers in the serverDB.
func Broadcast_foreign_job_updates(jobs *xml.Hash) {
  jobs = jobs.Clone() // because we work asynchronously
  for _, server := range db.ServerAddresses() {
    go Send_foreign_job_updates(server, jobs)
  }
}

// Handles the message "foreign_job_updates".
//  xmlmsg: the decrypted and parsed message.
// Returns:
//  unencrypted reply
func foreign_job_updates(xmlmsg *xml.Hash) string {
  source := xmlmsg.Text("source")
  for _, tag := range xmlmsg.Subtags() {
  
    if !strings.HasPrefix(tag, "answer") { continue }
  
    for answer := xmlmsg.First(tag); answer != nil; answer = answer.Next() {
      job := answer.Clone()
      job.Rename("job")
      
      if job.Text("siserver") == "localhost" {
        job.First("siserver").SetText(source)
      }
      
      // remove all whitespace from xmlmessage
      // This works around gosa-si's behaviour of introducing whitespace
      // which breaks base64 decoding.
      xmlmess := job.First("xmlmessage")
      xmlmess.SetText(strings.Join(strings.Fields(xmlmess.Text()),""))
      db.JobUpdate(job)
    }
  }
  
  return ""
}
