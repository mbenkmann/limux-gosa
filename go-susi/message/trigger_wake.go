/* 
Copyright (c) 2012 Landeshauptstadt München
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
         "github.com/mbenkmann/golib/util"
       )

// Handles the message "trigger_wake".
//  xmlmsg: the decrypted and parsed message
func trigger_wake(xmlmsg *xml.Hash) {
  util.Log(2, "DEBUG! trigger_wake(%v)", xmlmsg)
  TriggerWake(xmlmsg.Text("macaddress"))
}

// Tries to send a WOL to the given MAC. Returns true if one or more WOL packets were sent
// or false if no subnet for the MAC is known or reachable.
// NOTE: That this function returns true does not mean that the WOL has reached its target.
func TriggerWake(macaddress string) bool {
  wake_target := []string{}
  if system := db.ServerWithMAC(macaddress); system != nil {
    wake_target = append(wake_target, strings.Split(system.Text("source"),":")[0])
  }
  if system := db.ClientWithMAC(macaddress); system != nil {
    wake_target = append(wake_target, strings.Split(system.Text("client"),":")[0])
  }
  if system := db.SystemFullyQualifiedNameForMAC(macaddress); system != "none" {
    wake_target = append(wake_target, system)
  }  
  
  woken := false
  for i := range wake_target {
    if err := util.Wake(macaddress, wake_target[i]); err == nil { 
      util.Log(1, "INFO! Sent Wake-On-LAN for MAC %v to %v", macaddress, wake_target[i])
      woken = true
      // We do not break here, because the data in the serverDB or clientDB may
      // be stale and since we're sending UDP packets, there's no guarantee
      // that util.Wake() will fail even if the system is no longer there.
      // Since the WOL packets include the MAC address it can't hurt to
      // send more than necessary.
    } else {
      util.Log(0, "ERROR! Could not send Wake-On-LAN for MAC %v to %v: %v", macaddress,wake_target[i],err)
    }
  }
  
  return woken
}
