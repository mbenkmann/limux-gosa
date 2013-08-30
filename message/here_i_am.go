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
         "sync/atomic"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

var TotalRegistrations int32
var MissedRegistrations int32

// sends a here_i_am message to target (HOST:PORT).
func Send_here_i_am(target string) {
  here_i_am := xml.NewHash("xml", "header", "here_i_am")
  here_i_am.Add("here_i_am")
  here_i_am.Add("source", config.ServerSourceAddress)
  here_i_am.Add("target", target)
  here_i_am.Add("events","goSusi")
  here_i_am.Add("gotoHardwareChecksum", "unknown")
  here_i_am.Add("client_status", config.Version)
  here_i_am.Add("client_revision", config.Revision)
  here_i_am.Add("mac_address", config.MAC) //Yes, that's mac_address with "_"
  
  clientpackageskey := config.ModuleKey["[ClientPackages]"]
  // If [ClientPackages]/key missing, take the last key in the list
  // (We don't take the 1st because that would be "dummy-key").
  if clientpackageskey == "" { clientpackageskey = config.ModuleKeys[len(config.ModuleKeys)-1] }
  
  // We don't generate random keys as it adds no security.
  // Everybody who has the ClientPackages key can decrypt the
  // key exchange messages, so a random key would only be as
  // secure as the ClientPackages key itself.
  here_i_am.Add("key_lifetime","2147483647")
  here_i_am.Add("new_passwd", clientpackageskey)
  
  util.Log(2, "DEBUG! Sending here_i_am to %v: %v", target, here_i_am)
  util.SendLnTo(target, GosaEncrypt(here_i_am.String(), clientpackageskey), config.Timeout)
}

// Handles the message "here_i_am".
//  xmlmsg: the decrypted and parsed message
func here_i_am(xmlmsg *xml.Hash) {
  start := time.Now()
  client := xml.NewHash("xml","header","new_foreign_client")
  client.Add("new_foreign_client")
  client.Add("source",config.ServerSourceAddress)
  client.Add("target",config.ServerSourceAddress)
  client_addr := xmlmsg.Text("source")
  macaddress  := xmlmsg.Text("mac_address") //Yes, that's "mac_address" with "_"
  util.Log(1, "INFO! here_i_am from client %v (%v)", client_addr, macaddress)
  client.Add("client", client_addr)
  client.Add("macaddress",macaddress)
  client.Add("key",xmlmsg.Text("new_passwd"))
  db.ClientUpdate(client)
  // A client that sends here_i_am to us is either our own internal client or
  // a client-only client. In either case make sure we don't have an entry in the
  // peer database.
  db.ServerRemove(client_addr)
  
  util.Log(1, "INFO! Informing all peers about new registered client %v at %v", macaddress, client_addr)
  for _, server := range db.ServerAddresses() {
    client.First("target").SetText(server)
    Peer(server).Tell(client.String(), "")
  }
  checkTime(start, macaddress)

  message_start := "<xml><source>"+config.ServerSourceAddress+"</source><target>"+client_addr+"</target>"
  registered := message_start + "<header>registered</header><registered></registered>"
  
  util.Log(1, "INFO! Getting LDAP data for client %v (%v) including groups", client_addr, macaddress)
  system, err := db.SystemGetAllDataForMAC(macaddress, true)
  checkTime(start, macaddress)
  
  if system != nil && system.Text("gotoldapserver") != "" {
    registered += "<ldap_available>true</ldap_available>"
  }
  registered += "</xml>"
  Client(client_addr).Tell(registered, config.LocalClientMessageTTL)
  atomic.AddInt32(&TotalRegistrations, 1)
  if !checkTime(start, macaddress) { atomic.AddInt32(&MissedRegistrations, 1) }
  
  if err != nil { // if no LDAP data available for system, create install job, do hardware detection
    if client_addr == config.ServerSourceAddress {
      util.Log(1, "INFO! %v => Normally I would create an install job and send detect_hardware, but the here_i_am is from myself, so I better not saw the branch I'm sitting on.", err)
    } else {
      util.Log(1, "INFO! %v => Creating install job and sending detect_hardware to %v", err, macaddress)
    
      detect_hardware := message_start + "<header>detect_hardware</header><detect_hardware></detect_hardware></xml>"
      Client(client_addr).Tell(detect_hardware, config.LocalClientMessageTTL)
    
      makeSureWeHaveAppropriateProcessingJob(macaddress, "trigger_action_reinstall", "hardware-detection")
    }

  } else { // if LDAP data for system is available
    
    Send_new_ldap_config(client_addr, system)
    
    util.Log(1, "INFO! Making sure job database is consistent with faistate \"%v\"", system.Text("faistate"))
    
    switch (system.Text("faistate")+"12345")[0:5] {
      case "local":  local_processing := xml.FilterSimple("siserver", config.ServerSourceAddress, "macaddress", macaddress, "status", "processing")
                     install_or_update := xml.FilterOr([]xml.HashFilter{xml.FilterSimple("headertag", "trigger_action_reinstall"),xml.FilterSimple("headertag", "trigger_action_update")})
                     db.JobsRemoveLocal(xml.FilterAnd([]xml.HashFilter{local_processing, install_or_update}), false) // false => re-schedule if periodic
      case "reins",
           "insta": makeSureWeHaveAppropriateProcessingJob(macaddress, "trigger_action_reinstall", "none")
      case "updat",
           "softu": makeSureWeHaveAppropriateProcessingJob(macaddress, "trigger_action_update", "none")
      case "error":
    }
    
    // Update LDAP entry if cn != DNS name  or ipHostNumber != IP
    client_addr := strings.SplitN(client_addr,":",2)
    client_ip := client_addr[0]
    client_port := ""
    if len(client_addr) > 1 { client_port = client_addr[1] }
    client_name := db.SystemNameForIPAddress(client_ip)
    new_name := strings.SplitN(client_name,".",2)[0]
    if config.FullQualifiedCN { new_name = client_name }
    uses_standard_port := false
    for _, standard_port := range config.ClientPorts {
      if client_port == standard_port {
        uses_standard_port = true
        break
      }
    }
    
    update_name := false
    update_ip := false
    cn := system.Text("cn")
    if client_name != "none" && cn != client_name && cn != strings.SplitN(client_name,".",2)[0] {
      if !uses_standard_port {
        util.Log(1, "INFO! Client cn (%v) does not match DNS name (%v) but client runs on non-standard port (%v) => Assuming test and will not update cn", cn, new_name, client_port)
      } else if DoNotChangeCN(system) { 
        util.Log(1, "INFO! Client cn (%v) does not match DNS name (%v) but client is blacklisted for cn updates", cn, new_name)
      } else {
        util.Log(1, "INFO! Client cn (%v) does not match DNS name (%v) => Update cn", cn, new_name)
        update_name = true
      }
    }
    if client_ip != system.Text("iphostnumber") {
      if system.Text("iphostnumber") != "" && !uses_standard_port {
        util.Log(1, "INFO! Client ipHostNumber (%v) does not match IP (%v) but client runs on non-standard port (%v) => Assuming test and will not update ipHostNumber", system.Text("iphostnumber"), client_ip, client_port)
      } else {
        util.Log(1, "INFO! Client ipHostNumber (%v) does not match IP (%v) => Update ipHostNumber", system.Text("iphostnumber"), client_ip)
        update_ip = true
      }
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

// Returns true if less than 8s have passed since start.
// Otherwise logs a warning and returns false.
func checkTime(start time.Time, macaddress string) bool {
  if time.Since(start) < 8*time.Second { return true }
  util.Log(0, "WARNING! Could not complete registration of client %v within the time window", macaddress)
  return false 
}

func makeSureWeHaveAppropriateProcessingJob(macaddress, headertag, progress string) {
  job := xml.NewHash("job")
  job.Add("progress", progress)
  job.Add("status", "processing")
  job.Add("siserver", config.ServerSourceAddress)
  job.Add("targettag", macaddress)
  job.Add("macaddress", macaddress)
  job.Add("modified", "1")
  job.Add("timestamp", util.MakeTimestamp(time.Now()))
  job.Add("headertag", headertag)
  job.Add("result", "none")
  
  // Filter for selecting local jobs in status "processing" for the client's MAC.
  local_processing := xml.FilterSimple("siserver", config.ServerSourceAddress, "macaddress", macaddress, "status", "processing")
  
  // If we don't already have an appropriate job with status "processing", create one
  if db.JobsQuery(xml.FilterAnd([]xml.HashFilter{local_processing, xml.FilterSimple("headertag", headertag)})).FirstChild() == nil {
  
    // First cancel other local install or update jobs for the same MAC in status "processing",
    // because only one install or update job can be processing at any time.
    // NOTE: I'm not sure if clearing <periodic> is the right thing to do
    // in this case. See the corresponding note in foreign_job_updates.go
    install_or_update := xml.FilterOr([]xml.HashFilter{xml.FilterSimple("headertag", "trigger_action_reinstall"),xml.FilterSimple("headertag", "trigger_action_update")})
    db.JobsRemoveLocal(xml.FilterAnd([]xml.HashFilter{local_processing, install_or_update}), true)
    
    // Now add the new job.
    db.JobAddLocal(job)
  }
}
