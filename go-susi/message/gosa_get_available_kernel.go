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

// Handles the message "gosa_get_available_kernel".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_get_available_kernel(xmlmsg *xml.Hash) *xml.Hash {
  reply := xml.NewHash("xml","header","get_available_kernel")
  reply.Add("source", config.ServerSourceAddress)
  reply.Add("target", xmlmsg.Text("source"))
  reply.Add("session_id","1")
  reply.Add("get_available_kernel")
  
  var count uint64 = 1
  kernels := db.FAIKernels(xml.FilterSimple("fai_release",xmlmsg.Text("fai_release")))
  for kernel := kernels.First("kernel"); kernel != nil; kernel = kernel.Next() {
    answer := xml.NewHash("answer"+strconv.FormatUint(count, 10))
    answer.SetText(kernel.Text("cn"))
    reply.AddWithOwnership(answer)
    count++
  }
  
  return reply
}
