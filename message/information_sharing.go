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
         "../util"
       )

// Handles the message "information_sharing".
//  xmlmsg: the decrypted and parsed message
func information_sharing(xmlmsg *xml.Hash) {
  maintag := ""
  for child := xmlmsg.FirstChild(); child != nil; child = child.Next() {
    tag := child.Element().Name()
    if tag == "header" || tag == "source" || tag == "target" || tag == "information_sharing" {
      // The usual suspects
      
    } else if tag == "user_db" || tag == "new_user" {
      // Check that the message doesn't combine user_db and new_user,
      // because AFAICT that's nonsensical
      
      if maintag != "" && maintag != tag {
        util.Log(0, "ERROR! information_sharing message with both \"user_db\" and \"new_user\" elements: %v", xmlmsg)
        break
      }
      maintag = tag
    } else {
      // We don't know this element. Better tell the admin about it.
      
      util.Log(0,"ERROR! information_sharing message with unknown element \"%v\": %v", tag, xmlmsg)
      break
    }
  }
}

