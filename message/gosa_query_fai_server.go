/* 
Copyright (c) 2013 Landeshauptstadt München
Author: Matthias S. Benkmann

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
         "strconv"
         
         "../db"
         "../xml"
         "../config"
       )

// Handles the message "gosa_query_fai_server".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_query_fai_server(xmlmsg *xml.Hash) string {
  servers := db.FAIServers()
  
  var count uint64 = 1
  for _, tag := range servers.Subtags() {
    answer := servers.RemoveFirst(tag)
    for ; answer != nil; answer = servers.RemoveFirst(tag) {
      answer.Rename("answer"+strconv.FormatUint(count, 10))
      servers.AddWithOwnership(answer)
      count++
    }
  }
  
  servers.Add("header", "query_fai_server")
  servers.Add("source", config.ServerSourceAddress)
  servers.Add("target", xmlmsg.Text("source"))
  servers.Add("session_id", "1")
  servers.Rename("xml")
  return servers.String()
}
