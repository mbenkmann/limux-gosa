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
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

// Handles the message "gosa_delete_jobdb_entry".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_delete_jobdb_entry(xmlmsg *xml.Hash) *xml.Hash {
  where := xmlmsg.First("where")
  if where == nil { where = xml.NewHash("where") }
  filter, err := xml.WhereFilter(where)
  if err != nil {
    util.Log(0, "ERROR! gosa_delete_jobdb_entry: Error parsing <where>: %v", err)
    return ErrorReplyXML(err)
  }
  
  db.JobsRemove(filter)
  
  answer := xml.NewHash("xml", "header", "answer")
  answer.Add("source", config.ServerSourceAddress)
  answer.Add("target", xmlmsg.Text("source"))
  answer.Add("answer1", "0")
  answer.Add("session_id", "1")
  return answer
}


