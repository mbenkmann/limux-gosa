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
         "../config"
       )

// Handles the message "gosa_query_jobdb".
//  encrypted: the original encrypted message
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_query_jobdb(encrypted string, xmlmsg *xml.Hash) string {
  jobdb_xml := db.JobsQuery(xmlmsg.First("where"))
  MakeAnswerList(jobdb_xml)
  jobdb_xml.Add("header", "query_jobdb")
  jobdb_xml.Add("source", config.ServerSourceAddress)
  jobdb_xml.Add("target", xmlmsg.Text("source"))
  jobdb_xml.Add("session_id", "1")
  return jobdb_xml.String()
}


