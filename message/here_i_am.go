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
         "strings"
         
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
    
    // Update LDAP entry if cn != DNS name  or ipHostNumber != IP
    client_ip := strings.SplitN(client_addr,":",2)[0]
    client_name := db.SystemNameForIPAddress(client_ip)
    new_name := strings.SplitN(client_name,".",2)[0]
    if config.FullQualifiedCN { new_name = client_name }
    
    update_name := false
    update_ip := false
    cn := system.Text("cn")
    if client_name != "none" && cn != client_name && cn != strings.SplitN(client_name,".",2)[0] {
      if DoNotChangeCN(system) { 
        util.Log(1, "INFO! Client cn (%v) does not match DNS name (%v) but client is blacklisted for cn updates", cn, new_name)
      } else {
        util.Log(1, "INFO! Client cn (%v) does not match DNS name (%v) => Update cn", cn, new_name)
        update_name = true
      }
    }
    if client_ip != system.Text("iphostnumber") {
      util.Log(1, "INFO! Client ipHostNumber (%v) does not match IP (%v) => Update ipHostNumber", system.Text("iphostnumber"), client_ip)
      update_ip = true
    }
    
    if update_ip || update_name {
      system, err = db.SystemGetAllDataForMAC(macaddress, false) // need LDAP data without groups
      if system == nil {
        util.Log(0, "ERROR! LDAP error reading data for %v: %v", macaddress, err)
      } else {
        system_upd := system.Clone()
        if update_ip { system_upd.FirstOrAdd("iphostnumber").SetText(client_ip) }
        if update_name { system_upd.First("cn").SetText(new_name) }
        err = db.SystemReplace(system, system_upd)
        if err != nil {
          util.Log(0, "ERROR! LDAP error updating %v: %v", macaddress, err)
        }
      }
    }
  }
}

// Returns true if the CN of system must not be changed.
func DoNotChangeCN(system *xml.Hash) bool {
  cn := system.Text("cn")
  if strings.HasPrefix(cn, config.CNAutoPrefix) && strings.HasSuffix(cn, config.CNAutoSuffix) { return false }
  for i := range config.CNRenameBlacklist {
    if strings.HasSuffix(system.Text("dn"),config.CNRenameBlacklist[i]) { return true }
  }
  return false
}
