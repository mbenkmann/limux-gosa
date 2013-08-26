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
*/

package message

import (
         "../xml"
       )

// Handles the message "usr_msg".
//  xmlmsg: the decrypted and parsed message
func usr_msg(xmlmsg *xml.Hash) {
  xmlmsg.First("header").SetText("job_send_user_msg")
  for child := xmlmsg.First("usr"); child != nil; child = child.Next() {
    child.Rename("user")
  }
  job_send_user_msg(xmlmsg)
}

