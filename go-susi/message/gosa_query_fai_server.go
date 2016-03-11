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
         "../security"

         "github.com/mbenkmann/golib/util"
       )

// Handles the message "gosa_query_fai_server".
//  xmlmsg: the decrypted and parsed message
//  context: the security context
// Returns:
//  unencrypted reply
func gosa_query_fai_server(xmlmsg *xml.Hash, context *security.Context) *xml.Hash {
  serversdb := db.FAIServers()
  servers := xml.NewHash("xml","header", "query_fai_server")
  
  var count uint64 = 1
  for child := serversdb.FirstChild(); child != nil; child = child.Next() {
    if context.Limits.MaxAnswers > 0 && count == uint64(context.Limits.MaxAnswers) {
      util.Log(0, "WARNING! [SECURITY] Request from %v generated too many answers => Truncating answer list\n", context.PeerID.IP)
      break
    }
    answer := child.Remove()
    answer.Rename("answer"+strconv.FormatUint(count, 10))
    servers.AddWithOwnership(answer)
    count++
  }
  
  servers.Add("source", config.ServerSourceAddress)
  servers.Add("target", xmlmsg.Text("source"))
  servers.Add("session_id", "1")
  return servers
}
