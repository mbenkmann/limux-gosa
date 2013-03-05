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
         "strings"
         
         "../xml"
         "../util"
       )

var mapUserToMAC map[string]string

// Handles the message "CLMSG_CURRENTLY_LOGGED_IN".
//  xmlmsg: the decrypted and parsed message
func clmsg_currently_logged_in(xmlmsg *xml.Hash) {
  add_remove_users(true, xmlmsg, "CLMSG_CURRENTLY_LOGGED_IN")
}

// Handles the message "CLMSG_LOGIN".
//  xmlmsg: the decrypted and parsed message
func clmsg_login(xmlmsg *xml.Hash) {
  add_remove_users(true, xmlmsg, "CLMSG_LOGIN")
}

// Handles the message "CLMSG_LOGOUT".
//  xmlmsg: the decrypted and parsed message
func clmsg_logout(xmlmsg *xml.Hash) {
  add_remove_users(false, xmlmsg, "CLMSG_LOGOUT")
}


func add_remove_users(add bool, xmlmsg *xml.Hash, tag string) {
  mac := xmlmsg.Text("macaddress")
  if mac == "" {
    util.Log(0, "ERROR! Missing <macaddress> in message: %v", xmlmsg)
    return
  }
  
  for c := xmlmsg.First(tag); c != nil; c = c.Next() {
    for _, user := range strings.Split(c.Text()," ") {
      if user != "" {
        if add {
          mapUserToMAC[user] = mac
        } else {
          delete(mapUserToMAC, user) 
        }
      }
    }
  }
  
  util.Log(2, "DEBUG! Big brother says: %v", mapUserToMAC)
}

