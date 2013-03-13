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
         "../xml"
         "../config"
       )

// Handles the message "gosa_get_available_kernel".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_get_available_kernel(xmlmsg *xml.Hash) string {
  reply := xml.NewHash("xml","header","get_available_kernel")
  reply.Add("source", config.ServerSourceAddress)
  reply.Add("target", "GOSA")
  reply.Add("session_id","1")
  reply.Add("answer1","default")
  reply.Add("answer2","kirschkernel")
  reply.Add("answer3","pfirsichkernel")
  reply.Add("get_available_kernel")
  
  return reply.String()
}
