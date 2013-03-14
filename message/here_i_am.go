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
         "time"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

// Handles the message "here_i_am".
//  xmlmsg: the decrypted and parsed message
func here_i_am(xmlmsg *xml.Hash) {
  client := xml.NewHash("xml","header","new_foreign_client")
  client.Add("new_foreign_client")
  client.Add("source",config.ServerSourceAddress)
  client.Add("target",config.ServerSourceAddress)
  client_addr := xmlmsg.Text("source")
  macaddress  := xmlmsg.Text("mac_address") //Yes, that's "mac_address" with "_"
  client.Add("client", client_addr)
  client.Add("macaddress",macaddress)
  client.Add("key",xmlmsg.Text("new_passwd"))
  db.ClientUpdate(client)
  
  util.Log(1, "INFO! Informing all peers about new registered client %v at %v", macaddress, client_addr)
  for _, server := range db.ServerAddresses() {
    client.First("target").SetText(server)
    Peer(server).Tell(client.String(), "")
  }

  message_start := "<xml><source>"+config.ServerSourceAddress+"</source><target>"+client_addr+"</target>"
  registered := message_start + "<header>registered</header><registered></registered>"
  
  system, err := db.SystemGetAllDataForMAC(macaddress, true)
  
  if system != nil && system.Text("gotoldapserver") != "" {
    registered += "<ldap_available>true</ldap_available>"
  }
  registered += "</xml>"
  Client(client_addr).Tell(registered, config.LocalClientMessageTTL)
  
  if err != nil { // if no LDAP data available for system, create install job, do hardware detection
    util.Log(1, "INFO! %v => Creating install job and sending detect_hardware to %v", err, macaddress)
    
    detect_hardware := message_start + "<header>detect_hardware</header><detect_hardware></detect_hardware></xml>"
    Client(client_addr).Tell(detect_hardware, config.LocalClientMessageTTL)
    
    job := xml.NewHash("job")
    job.Add("progress", "hardware-detection")
    job.Add("status", "processing")
    job.Add("siserver", config.ServerSourceAddress)
    job.Add("targettag", macaddress)
    job.Add("macaddress", macaddress)
    job.Add("modified", "1")
    job.Add("timestamp", util.MakeTimestamp(time.Now()))
    job.Add("headertag", "trigger_action_reinstall")
    job.Add("result", "none")
    
    db.JobAddLocal(job)
    
  } else { // if LDAP data for system is available
    Send_new_ldap_config(client_addr, system)
  }
}
