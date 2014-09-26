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
       )

// Handles the message "new_foreign_client".
//  xmlmsg: the decrypted and parsed message
func new_foreign_client(xmlmsg *xml.Hash) {
  db.ClientUpdate(xmlmsg)
  // If the client is not the same as the sending server,
  // it can't be a server. Remove from peer database if it is there.
  // NOTE: This deals with the fact that go-susi uses the same port
  // for client-only instances as for server instances. A server that
  // is re-installed to become a client may therefore be listed in
  // serverdb.
  if xmlmsg.Text("source") != xmlmsg.Text("client") {
    db.ServerRemove(xmlmsg.Text("client"))
  }
}

