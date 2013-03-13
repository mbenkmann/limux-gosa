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
         "time"
         "strings"
         
         "../xml"
         "../util"
         "../config"
       )

// Handles the message "gosa_query_fai_release".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_query_fai_release(xmlmsg *xml.Hash) string {
  release := xmlmsg.String()
  release = strings.SplitN(release, "</fai_release>", 2)[0]
  release = strings.SplitN(release, "<fai_release>", 2)[1]
  
  reply := xml.NewHash("xml","header","query_fai_release")
  reply.Add("source", config.ServerSourceAddress)
  reply.Add("target", "GOSA")
  reply.Add("session_id","1")
  answer := xml.NewHash("answer1")
  answer.Add("class", "Modul_Standard")
  answer.Add("timestamp", util.MakeTimestamp(time.Now()))
  answer.Add("fai_release", release)
  answer.Add("type", "FAIprofile")
  answer.Add("state")
  
  reply.AddWithOwnership(answer)
  
  return reply.String()
}
