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
         "../util"
         "../config"
       )

// Handles the message "detected_hardware".
//  xmlmsg: the decrypted and parsed message
func detected_hardware(xmlmsg *xml.Hash) {
  source := xmlmsg.Text("source")
  colon := strings.Index(source, ":")
  if colon <= 0 {
    util.Log(0, "ERROR! detected_hardware message without proper <source>: %v", xmlmsg)
    return
  }
  ip := source[0:colon]
  
  client := db.ClientWithAddress(source)
  if client == nil {
    util.Log(0, "ERROR! detected_hardware message from unknown client: %v", xmlmsg)
    return
  }
  
  system := xml.NewHash("xml")
  system.Add("iphostnumber", ip)
  system.AddClone(client.First("macaddress"))
  
  // copy all hardware-related attributes
  detected := xmlmsg.First("detected_hardware")
  if detected == nil {
    util.Log(0, "ERROR! detected_hardware message without <detected_hardware> element: %v", xmlmsg)
    return
  }
  for ; detected != nil ; detected = detected.Next() {
    for _, tag := range detected.Subtags() {
      for ele := detected.First(tag); ele != nil; ele = ele.Next() {
        system.Add(strings.ToLower(tag), ele.Text())
      }
    }
  }
  
  // standard objectClasses
  system.Add("objectclass", "GOhard")
  if config.UnitTag != "" {
    system.Add("objectclass", "gosaAdministrativeUnitTag")
    system.Add("gosaunittag", config.UnitTag)
  }
  
  oldentry,_ := db.SystemGetAllDataForMAC(system.Text("macaddress"), false)
  if oldentry != nil { // If we have an existing entry, merge it with the new hardware data
  
    system.Add("cn", oldentry.Text("cn"))
    system.Add("dn", oldentry.Text("dn"))
    db.SystemFillInMissingData(system, oldentry)
  
  } else { // If there is no existing entry, generate cn and look for a template for the rest
    
    name := db.SystemNameForIPAddress(ip) // this is a fully qualified name if it could be determined
    if name == "none" {
      util.Log(0, "ERROR! detected_hardware message from client with broken reverse DNS => Using generated name")
      name = "system-" + strings.Replace(system.Text("macaddress"),":","-", -1)
    }
    system.Add("cn", name)
    
    templates := db.SystemGetTemplatesFor(system)
    if template := templates.First("xml"); template != nil {
      if template.Next() != nil {
        util.Log(0, "WARNING! System %v matches more than 1 template: %v and %v (and possibly others)", system.Text("cn"), template.Text("cn"), template.Next().Text("cn"))
      }
    
      // Also generates a dn derived from system's cn and template's dn.
      db.SystemFillInMissingData(system, template)
      
      // Add system to the same object groups template is member of (if any).
      db.SystemAddToGroups(system.Text("dn"), db.SystemGetGroupsWithMember(template.Text("dn")))
    }
  }
  
  // Fallback if neither an existing entry nor a template provided us with
  // some essential attributes.
  if system.First("gotomode") == nil {
    system.Add("gotomode", "locked")
  }
  if system.First("gotosysstatus") == nil {
    system.Add("gotosysstatus", "new-system")
  }
  if system.First("dn") == nil {
    system.Add("dn").SetText("cn=%v,ou=incoming,%v", system.Text("cn"), config.LDAPBase)
  }
  
  // Update LDAP data or create new entry
  db.SystemReplace(oldentry, system)
  
  // if the system is not locked, tell it to start the installation right away
  if system.Text("gotomode") == "active" {
    set_activated_for_installation := "<xml><header>set_activated_for_installation</header><set_activated_for_installation></set_activated_for_installation><source>"+ config.ServerSourceAddress +"</source><target>"+ source +"</target></xml>"
    Client(source).Tell(set_activated_for_installation, config.LocalClientMessageTTL)
  }
}

