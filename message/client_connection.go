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
         "../util"
         "../config"
       )

// Sends garbage to all clients listed in db.ClientsWeMayHave to
// prompt them to send new here_i_am messages.
func CheckPossibleClients() {
  for _, tag := range db.ClientsWeMayHave.Subtags() {
    for client := db.ClientsWeMayHave.First(tag); client != nil; client = client.Next() {
      addr := client.Text("client")
      util.Log(1, "INFO! Sending 'Müll' to %v", addr)
      go util.SendLnTo(addr, "Müll", config.Timeout)
    }
  }
}
