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
         
         "../xml"
         "../util"
         "../config"
       )

var gotoLdapServerRegexp = regexp.MustCompile("^([0-9]+):([^:]+):([^:/]+:/{0,2}[^/]+)/(.*)$")

// If system == nil or <xml></xml>, this function does nothing; otherwise it
// takes the information from system (format as returned by db.SystemGetAllDataForMAC())
// and sends new_ldap_config and new_ntp_config messages to client_addr (IP:PORT).
func Send_new_ldap_config(client_addr string, system *xml.Hash) {
  message_start := "<xml><source>"+config.ServerSourceAddress+"</source><target>"+client_addr+"</target>"
  
  if system != nil && len(system.Subtags()) > 0 { // if LDAP data for system is available

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




