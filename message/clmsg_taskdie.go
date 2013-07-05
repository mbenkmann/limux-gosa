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
         
         "../db"
         "../xml"
       )

// Handles the message "CLMSG_TASKDIE".
//  xmlmsg: the decrypted and parsed message
func clmsg_taskdie(xmlmsg *xml.Hash) {
  macaddress := xmlmsg.Text("macaddress")
  text := xmlmsg.Text("CLMSG_TASKDIE")
  colon := strings.Index(text,":")
  if colon < 0 { colon = len(text) }
  msg := text[colon:]
  tech := strings.Fields(text[0:colon])
  component := "fiddledidoo"
  code := "-1"
  criticality := "fatal"
  if len(tech) > 0 { component = tech[0] }
  if len(tech) > 1 { code = tech[1] }
  if len(tech) > 2 { criticality = tech[2] }
  faistate := "error:"+component+":"+code+":"+criticality+":"+msg
  
  db.SystemForceFAIState(macaddress, faistate)
}

