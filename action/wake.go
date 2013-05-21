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
         "../util"
         "../message"
       )

// Wakes up the machine with the MAC addres job.Text("macaddress"). No other
// elements from job are used.
func Wake(job *xml.Hash) {
  macaddress := job.Text("macaddress")
  
  woken := message.TriggerWake(macaddress)
  
  // While sending a few excess WOL packets should be harmless, spamming all
  // known networks with WOLs is very ugly, especially when it's completely
  // unnecessary. And if one of the above 3 WOL attempts succeeded, chances
  // are very good that we already hit our target. For that reason we only
  // perform network spamming if everything else has failed.
  if !woken {
    util.Log(0, "ERROR! Targetted Wake-On-LAN for MAC %v failed. Jericho protocol engaged!",macaddress)
    
    // We ask all known peers to join the fun. Let's raise hell and wake the dead!
    xmlmsg := xml.NewHash("xml","header","trigger_wake")
    xmlmsg.Add("source", "GOSA")
    xmlmsg.Add("macaddress",macaddress)
    xmlmsg.Add("trigger_wake") //empty element because gosa-si does it like that
    for _, server := range db.ServerAddresses() {
      xmlmsg.FirstOrAdd("target").SetText(server)
      message.Peer(server).Ask(xmlmsg.String(), "")
    }
    
    // Now spam all networks with our WOL packet.
    for _, network := range db.SystemNetworksKnown() {
      util.Log(1, "INFO! Spamming network %v with Wake-On-LAN for %v", network, macaddress)
      if err := util.Wake(macaddress, network); err != nil { 
        util.Log(0, "ERROR! Could not send Wake-On-LAN for MAC %v to %v: %v", macaddress,network,err)
      }
    }
  }
}
