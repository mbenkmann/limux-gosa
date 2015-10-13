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
         "github.com/mbenkmann/golib/util"
         "../config"
       )

var gotoLdapServerRegexp = regexp.MustCompile("^([0-9]+):([^:]+):([^:/]+:/{0,2}[^/]+)/(.*)$")

// Partially handles the message "new_ldap_config", then passes it on to
// new_foo_config().
//  xmlmsg: the decrypted and parsed message
func new_ldap_config(xmlmsg *xml.Hash) {
  target := xmlmsg.Text("target")
  if target != "" && target != config.ServerSourceAddress {
    // See https://code.google.com/p/go-susi/issues/detail?id=126
    util.Log(0, "WARNING! Ignoring message with incorrect target: %v", xmlmsg)
    return
  }
  
  if config.RunServer {
    if config.ServerSourceAddress != xmlmsg.Text("source") {
      util.Log(0, "WARNING! Will not update internal LDAP settings because I'm not in client-only mode.")
    }
  } else {
    updateInternalLdapSettings(xmlmsg)
  }
  
  new_foo_config(xmlmsg)
}


func updateInternalLdapSettings(xmlmsg *xml.Hash) {  
  ldap_uri   := ""
  if ldap := xmlmsg.First("ldap_uri"); ldap != nil {
    ldap_uri = ldap.Text()
  }
  admin_base := xmlmsg.Text("admin_base")
  department := xmlmsg.Text("department")
  ldap_base  := xmlmsg.Text("ldap_base")
  unit_tag   := xmlmsg.Text("unit_tag")
  
  // NOTE: The tests config.FOO != FOO may seem pointless but don't forget that
  // even when the strings compare as equal, they are different pointers.
  // The additional test avoids unnecessary memory
  // writes which is a precaution against race conditions because we do not use
  // locking when accessing these variables.
  // In particular this avoids unsafe writes to these variables when we receive
  // the new_ldap_config message we send to ourselves after registering at ourselves.
  if ldap_uri   != "" && config.LDAPURI  != ldap_uri  { 
    util.Log(1, "INFO! LDAP URI changed: \"%v\" => \"%v\"", config.LDAPURI, ldap_uri)
    config.LDAPURI  = ldap_uri  
  }
  if ldap_base  != "" && config.LDAPBase != ldap_base { 
    util.Log(1, "INFO! LDAP base changed: \"%v\" => \"%v\"", config.LDAPBase, ldap_base)
    config.LDAPBase = ldap_base 
  }
  
  if unit_tag == "" {
    if config.UnitTag != "" {
      util.Log(1, "INFO! gosaUnitTag support DISABLED")
      config.UnitTag = ""
      config.UnitTagFilter = ""
      config.AdminBase = ""
      config.Department = ""
    }
  } else {
    if config.UnitTag != unit_tag { 
      util.Log(1, "INFO! gosaUnitTag changed: \"%v\" => \"%v\"", config.UnitTag, unit_tag)
      config.UnitTag = unit_tag
      config.UnitTagFilter = "(gosaUnitTag="+config.UnitTag+")"
    }
    if admin_base != "" && config.AdminBase  != admin_base { 
      util.Log(1, "INFO! Admin base changed: \"%v\" => \"%v\"", config.AdminBase, admin_base)
      config.AdminBase = admin_base 
    }
    if department != "" && config.Department != department { 
      util.Log(1, "INFO! Department changed: \"%v\" => \"%v\"", config.Department, department)
      config.Department = department 
    }
  }
}  

// If system == nil or <xml></xml>, this function does nothing; otherwise it
// takes the information from system (format as returned by db.SystemGetAllDataForMAC())
// and sends new_ldap_config and new_ntp_config messages to client_addr (IP:PORT).
func Send_new_ldap_config(client_addr string, system *xml.Hash) {
  message_start := "<xml><source>"+config.ServerSourceAddress+"</source><target>"+client_addr+"</target>"
  
  if system != nil && system.FirstChild() != nil { // if LDAP data for system is available

    // send new_ntp_config if gotoNtpServer available
    ntps := system.Get("gotontpserver")
    if len(ntps) > 0 {
      new_ntp_config := message_start + "<header>new_ntp_config</header><new_ntp_config></new_ntp_config>"
      for _, ntp := range ntps {
        new_ntp_config += "<server>" + ntp + "</server>"
      }
      new_ntp_config += "</xml>"
      Client(client_addr).Tell(new_ntp_config, config.NormalClientMessageTTL)
    }
    
    // We always send a new_ldap_config message. If a gotoLdapServer attribute
    // is available for the client we use it, otherwise we send our own config,
    // which in most cases will be the same as the client's.
    new_ldap_config := xml.NewHash("xml","header","new_ldap_config")
    new_ldap_config.Add("new_ldap_config")
    new_ldap_config.Add("source", config.ServerSourceAddress)
    new_ldap_config.Add("target", client_addr)

    if ldaps := system.Get("gotoldapserver"); len(ldaps) > 0 {
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
    }
    
    if new_ldap_config.First("ldap_uri") == nil {
      util.Log(0, "WARNING! No usable LDAP config for client %v found => Sending my own config as fallback", client_addr)
      new_ldap_config.Add("ldap_uri", config.LDAPURI)
      new_ldap_config.Add("ldap_base", config.LDAPBase)
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
    
    Client(client_addr).Tell(new_ldap_config.String(), config.NormalClientMessageTTL)
  }
}




