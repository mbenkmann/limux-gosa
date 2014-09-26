/* 
Copyright (c) 2013 Matthias S. Benkmann

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

// Handles the message "gosa_set_activated_for_installation".
//  xmlmsg: the decrypted and parsed message
func gosa_set_activated_for_installation(xmlmsg *xml.Hash) {
  if xmlmsg.Text("header")[0:4] == "gosa" { 
    util.Log(2, "DEBUG! gosa_set_activated_for_installation -> gosa_trigger_action")
    gosa_trigger_action(xmlmsg) 
  } else {
    util.Log(2, "DEBUG! job_set_activated_for_installation -> job_trigger_action")
    job_trigger_action(xmlmsg) 
  }
}
