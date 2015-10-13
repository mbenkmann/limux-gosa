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

package action

import (
         "../db"
         "../xml"
         "github.com/mbenkmann/golib/util"
         "../message"
       )

func Activate(job *xml.Hash) {
  macaddress := job.Text("macaddress")
  db.SystemSetState(macaddress, "gotoMode", "active")
  if client := db.ClientWithMAC(macaddress); client != nil {
    system, err := db.SystemGetAllDataForMAC(macaddress, true)
    if err != nil {
      util.Log(0, "ERROR! %v", err)
        // Don't abort. Send_set_activated_for_installation() can still
        // do something, even if system data is not available.
    }
    message.Send_set_activated_for_installation(client.Text("client"), system)
  } else {
    util.Log(0, "ERROR! Unknown client %v => Cannot send set_activated_for_installation", macaddress)
  }
  
  return
}
