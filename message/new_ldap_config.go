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
         "regexp"
         "strings"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

var gotoLdapServerRegexp = regexp.MustCompile("^([0-9]+):([^:]+):([^:/]+:/{0,2}[^/]+)/(.*)$")

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
  
  registered := message_start + "<header>registered</header><ldap_available>true</ldap_available><registered></registered></xml>"
  Client(client_addr).Tell(registered, config.LocalClientMessageTTL)
  
  system, err := db.SystemGetAllDataForMAC(macaddress, true)
  if err != nil { // if no LDAP data available for system, do hardware detection
    util.Log(1, "INFO! %v => Sending detect_hardware to %v", err, macaddress)
    
    detect_hardware := message_start + "<header>detect_hardware</header><detect_hardware></detect_hardware></xml>"
    Client(client_addr).Tell(detect_hardware, config.LocalClientMessageTTL)
    
  } else { // if LDAP data for system is available

    // send new_ntp_config if gotoNtpServer available
    ntps := system.Get("gotontpserver")
    if len(ntps) > 0 {
      new_ntp_config := message_start + "<header>new_ntp_config</header><new_ntp_config></new_ntp_config>"
      for _, ntp := range ntps {
        new_ntp_config += "<server>" + ntp + "</server>"
      }
      new_ntp_config += "</xml>"
      Client(client_addr).Tell(new_ntp_config, config.LocalClientMessageTTL)
    }
    
    // if a gotoLdapServer attribute is available for the client, send
    // a new_ldap_config message.
    if ldaps := system.Get("gotoldapserver"); len(ldaps) > 0 {
      new_ldap_config := xml.NewHash("xml","header","new_ldap_config")
      new_ldap_config.Add("new_ldap_config")
      new_ldap_config.Add("source", config.ServerSourceAddress)
      new_ldap_config.Add("target", client_addr)
    
      for i := range ldaps {
        l := gotoLdapServerRegexp.FindStringSubmatch(ldaps[i])
        if l!=nil  && len(l) == 5 {
          new_ldap_config.Add("ldap_uri", l[3])
          if new_ldap_config.First("ldap_base") == nil {
            new_ldap_config.Add("ldap_base", l[4])
          }
        } else {
          util.Log(0, "ERROR! Can't parse gotoLdapServer entry \"%v\"", ldaps[i])
        }
      }

      // Send our own values instead of computing them again from the
      // client's ldap_base. I don't see a real world situation where
      // client and the server would have different values here.
      if config.UnitTag != "" {
        new_ldap_config.Add("unit_tag", config.UnitTag)
        new_ldap_config.Add("admin_base", config.AdminBase)
        new_ldap_config.Add("department", config.Department)
      }
      
      faiclass := strings.Split(system.Text("faiclass"), ":")
      release := ""
      if len(faiclass) == 2 { release = faiclass[1] }
      if release != "" {
        new_ldap_config.Add("release", release)
      }
      
      Client(client_addr).Tell(new_ldap_config.String(), config.LocalClientMessageTTL)
    }
  }
}




