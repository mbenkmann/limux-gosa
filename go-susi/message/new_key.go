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
         "../db"
         "../xml"
         "../util"
       )

// Handles the message "new_key".
//  xmlmsg: the decrypted and parsed message
func new_key(xmlmsg *xml.Hash) {
  client := db.ClientWithAddress(xmlmsg.Text("source"))
  if client == nil {
    util.Log(0, "ERROR! Got new_key message from unknown source %v", xmlmsg.Text("source"))
    return
  }
  
  for ; client.RemoveFirst("key") != nil; { }
  
  client.Add("key", xmlmsg.Text("new_key"))
  
  db.ClientUpdate(client)
}
