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

package message

import (
         "strings"
        
         "../xml"
         "../util"
       )

// Handles all messages of the form "gosa_trigger_action_*".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_trigger_action(xmlmsg *xml.Hash) *xml.Hash {
  util.Log(2, "DEBUG! gosa_trigger_action(%v)", xmlmsg)
  // translate gosa_trigger_* to job_trigger_*
  header := "job_" + strings.SplitN(xmlmsg.Text("header"),"_",2)[1]
  xmlmsg.First("header").SetText(header)
  return job_trigger_action(xmlmsg)
}
