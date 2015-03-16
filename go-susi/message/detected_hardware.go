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

// var macAddressRegexp is in job_trigger_action.go

// Handles the message "detected_hardware".
//  xmlmsg: the decrypted and parsed message
func detected_hardware(xmlmsg *xml.Hash) {
  // extract attributes with all attribute names converted to lowercase
  // at the same time unions together all <detected_hardware> elements.
  detected := xml.NewHash("xml")
  for dh := xmlmsg.First("detected_hardware"); dh != nil; dh = dh.Next() {
    for child := dh.FirstChild(); child != nil; child = child.Next() {
      detected.Add(strings.ToLower(child.Element().Name()), child.Element().Text())
    }
  }
  
  // if <detected_hardware> does not have a valid MAC, look it up in clientdb
  macaddress := detected.Text("macaddress")
  if macaddress == "00:00:00:00:00:00" || !macAddressRegexp.MatchString(macaddress) {
    client := db.ClientWithAddress(xmlmsg.Text("source"))
    if client == nil {
      util.Log(0, "ERROR! detected_hardware message without valid MAC from unknown client: %v", xmlmsg)
      return
    }  
    macaddress = client.Text("macaddress")
    detected.FirstOrAdd("macaddress").SetText(macaddress)
  }
  
  // if <detected_hardware> does not have ipHostNumber, extract it from <source>
  ip := detected.Text("iphostnumber")
  if ip == "" {
    ip = strings.SplitN(xmlmsg.Text("source"),":",2)[0]
    if ip != "" {
      detected.FirstOrAdd("iphostnumber").SetText(ip)
    }
  }

  system := detected.Clone()
  
  // Sanity check for dn changes: Reject multi-value dn attributes and
  // DNs not under config.LDAPBase.
  // At the same time, extract a cn from the dn if necessary and possible
  new_dn := system.Get("dn")
  if len(new_dn) != 0 {
    if len(new_dn) != 1 || !strings.HasSuffix(new_dn[0], config.LDAPBase) {
      util.Log(0, "ERROR! detected_hardware message requests illegal DN change: %v", xmlmsg)
      return
    }
    if system.Text("cn") == "" && strings.HasPrefix(new_dn[0],"cn=") {
      system.FirstOrAdd("cn").SetText(strings.SplitN(new_dn[0],",",2)[0][3:])
    }
  }
  
  oldentry,_ := db.SystemGetAllDataForMAC(macaddress, false)
  if oldentry != nil { // If we have an existing entry, merge it with the new hardware data
  
    // copy attributes that db.SystemFillInMissingData() does not copy
    for attr := range db.DoNotCopyAttribute {
      if system.First(attr) == nil && oldentry.First(attr) != nil {
        system.Add(attr, oldentry.Text(attr))
      }
    }
    
    db.SystemFillInMissingData(system, oldentry)
  
  } else { // If there is no existing entry, generate cn if necessary and look for a template
    
    // If no cn is supplied in detected_hardware, generate one
    if system.Text("cn") == "" {
      name := db.SystemNameForIPAddress(ip) // this is a fully qualified name if it could be determined
      if !config.FullQualifiedCN {
        name = strings.SplitN(name,".",2)[0]
      }
      if name == "none" {
        name = config.CNAutoPrefix + strings.Replace(macaddress,":","-", -1) + config.CNAutoSuffix
        util.Log(0, "WARNING! detected_hardware message from client %v with broken reverse DNS => Using generated name \"%v\"", ip, name)
      }
      system.Add("cn", name)
    }
    
    // Apply the first matching template (if any)
    templates := db.SystemGetTemplatesFor(system)
    if template := templates.First("xml"); template != nil {
      if template.Next() != nil {
        util.Log(0, "WARNING! System %v matches more than 1 template: %v and %v (and possibly others)", system.Text("cn"), template.Text("cn"), template.Next().Text("cn"))
      }
    
      // If necessary db.SystemFillInMissingData() also generates a dn 
      // derived from system's cn and template's dn.
      db.SystemFillInMissingData(system, template)
      
      // Add system to the same object groups template is member of (if any).
      db.SystemAddToGroups(system.Text("dn"), db.SystemGetGroupsWithMember(template.Text("dn")))
    }
  }
  
  // Fallback if neither an existing entry nor a template provided us with
  // some essential attributes.
  if system.First("objectclass") == nil {
    system.Add("objectclass", "GOhard")
    system.Add("objectclass", "FAIobject")
    if config.UnitTag != "" {
      system.Add("objectclass", "gosaAdministrativeUnitTag")
      system.Add("gosaunittag", config.UnitTag)
    }
  }
  if system.First("gotomode") == nil {
    system.Add("gotomode", "locked")
  }
  if system.First("faistate") == nil {
    system.Add("faistate", "install")
  }
  if system.First("dn") == nil {
    system.Add("dn","cn=%v,%v", system.Text("cn"), config.IncomingOU)
  }
  // I don't know what gotoSysStatus is good for. Let's see if things work
  // without it. If something breaks, activate these lines again and add a
  // comment explaining why they are necessary.
  //if system.First("gotosysstatus") == nil {
  //  system.Add("gotosysstatus", "new-system")
  //}

  // Update LDAP data or create new entry (if oldentry==nil)
  err := db.SystemReplace(oldentry, system)
  if err != nil {
    util.Log(0, "ERROR! Could not create/update LDAP object %v: %v", system.Text("dn"), err)
    return
  }

  // Read final system info including data from object groups
  system, err = db.SystemGetAllDataForMAC(macaddress, true)
  if err != nil {
    util.Log(0, "ERROR! %v", err)
    // Don't abort. Send_set_activated_for_installation() can still
    // do something, even if system data is not available.
  }
  
  // if the system is not locked, tell it to start the installation right away
  if system.Text("gotomode") == "active" {
    Send_set_activated_for_installation(xmlmsg.Text("source"), system)
  } else { // otherwise we can at least tell the system its LDAP and NTP server(s)
    Send_new_ldap_config(xmlmsg.Text("source"), system)
  }
}
