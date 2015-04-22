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
         "fmt"
         "sort"
         "time"
         "strings"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

// var macAddressRegexp is in job_trigger_action.go


// Handles the messages "job_trigger_activate_new" and "gosa_trigger_activate_new".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func job_trigger_activate_new(xmlmsg *xml.Hash) *xml.Hash {
  util.Log(2, "DEBUG! job_trigger_activate_new(%v)", xmlmsg)
   
  //====== determine MAC address ======
  
  macaddress := xmlmsg.Text("mac")
  if macaddress == "" {
    macaddress = xmlmsg.Text("macaddress")
  } else {
    if xmlmsg.Text("macaddress") != "" && xmlmsg.Text("macaddress") != macaddress {
      util.Log(0, "WARNING! <mac> and <macaddress> differ: %v", xmlmsg)
      // not fatal, <mac> takes precedence
    }
  }
  if macaddress == "" {
    macaddress = xmlmsg.Text("target")
  }
  
  if !macAddressRegexp.MatchString(macaddress) {
    emsg := fmt.Sprintf("job_trigger_activate_new(): Not a valid MAC address: %v", macaddress)
    util.Log(0, "ERROR! %v", emsg)
    return ErrorReplyXML(emsg)
  }
  
  // ======== determine template object (if any) from <ogroup> ========
  
  var err error
  var template *xml.Hash
  ogroup := xmlmsg.First("ogroup")
  if ogroup != nil && ogroup.Text() != "" {
    template_mac := db.SystemMACForName(ogroup.Text())
    if template_mac == "none" { // ogroup is apparently not a system => try object group
      groups := db.SystemGetGroupsWithName(ogroup.Text())
      group := groups.First("xml")
      if group == nil {
        emsg := fmt.Sprintf("job_trigger_activate_new(): Could not find %v", ogroup)
        util.Log(0, "ERROR! %v", emsg)
        return ErrorReplyXML(emsg)
      }
      
      if group.Next() != nil {
        emsg := fmt.Sprintf("job_trigger_activate_new(): Multiple groups match %v", ogroup)
        util.Log(0, "ERROR! %v", emsg)
        return ErrorReplyXML(emsg)
      }
      
      members := group.Get("member")
      sort.Strings(members)
      for i := range members {
        member_name := strings.SplitN(strings.SplitN(members[i],",",2)[0],"=",2)[1]
        template_mac = db.SystemMACForName(member_name)
        if template_mac != "none" { break }
      }
      
      if template_mac == "none" {
        emsg := fmt.Sprintf("job_trigger_activate_new(): Could not get template system from %v", ogroup)
        util.Log(0, "ERROR! %v", emsg)
        return ErrorReplyXML(emsg)
      }
    } 
    
    template, err = db.SystemGetAllDataForMAC(template_mac, false)
    if err != nil {
      emsg := fmt.Sprintf("job_trigger_activate_new could not extract data for template system %v (from %v)", template_mac, ogroup)
      util.Log(0, "ERROR! %v", emsg)
      return ErrorReplyXML(emsg)
    }
  }
  
  if template != nil {  
    util.Log(1, "INFO! job_trigger_activate_new(): Using %v as template for %v", template.Text("dn"), macaddress)
  } else {
    util.Log(1, "INFO! job_trigger_activate_new(): No template for %v", macaddress)
  }
  
  existing_sys,_ := db.SystemGetAllDataForMAC(macaddress, false)
  
  if existing_sys != nil {
    util.Log(1, "INFO! job_trigger_activate_new(): LDAP entry for %v already exists: %v", macaddress, existing_sys.Text("dn"))
  }
  
  
  // ======== determine ou to put/move system into =========
  
  ou := config.IncomingOU
  base := xmlmsg.Text("base")
  if base == "" { 
    if template != nil {
      ou = strings.SplitN(template.Text("dn"),",",2)[1]
    } else {
      if existing_sys != nil {
        ou = strings.SplitN(existing_sys.Text("dn"),",",2)[1]
      } // else { ou remains config.IncomingOU }
    }
  } else { // if base != ""
    oclasses := []string{}
    if existing_sys != nil { oclasses = append(oclasses, existing_sys.Get("objectclass")...) }
    if template != nil { oclasses = append(oclasses, template.Get("objectclass")...) }
    for _, ocls := range oclasses {
      if ocls == "gotoWorkstation" {
        ou = "ou=workstations,ou=systems," + base
        break
      }
      if ocls == "goServer" {
        ou = "ou=servers,ou=systems," + base
        break
      }
    }
  }
  
  if !strings.HasSuffix(ou, config.LDAPBase) {
    emsg := fmt.Sprintf("job_trigger_activate_new(): Cannot put %v into %v (not under %v)", macaddress, ou, config.LDAPBase)
    util.Log(0, "ERROR! %v", emsg)
    return ErrorReplyXML(emsg)
  }

  util.Log(1, "INFO! job_trigger_activate_new(): Will put/move LDAP entry for %v into %v", macaddress, ou)
  
  
  // ======== determine ipHostNumber =======
  
  ip := xmlmsg.Text("ip")
  if ip == "" {
    fqdn := xmlmsg.Text("fqdn")
    if fqdn != "" {
      ip = db.SystemIPAddressForName(xmlmsg.Text())
      if ip == "none" { ip = "" }
    }
  }
  
  timestamp := xmlmsg.Text("timestamp")
    // go-susi does not use 19700101000000 as default timestamp as gosa-si does,
    // because that plays badly in conjunction with <periodic>
  if timestamp == "" { timestamp = util.MakeTimestamp(time.Now()) }

  
  // ============ create or modify LDAP entry ============

  old_gotomode := ""
  var system *xml.Hash
  if existing_sys == nil {
    system = xml.NewHash("xml")
    system.Add("macaddress", macaddress)
    system.Add("cn", config.CNAutoPrefix+strings.Replace(macaddress,":","-", -1)+config.CNAutoSuffix)
  } else {
    old_gotomode = existing_sys.Text("gotomode")
    system = existing_sys.Clone()
  }

  system.FirstOrAdd("dn").SetText("cn=%v,%v", system.Text("cn"), ou)

  if existing_sys == nil && template != nil {
    db.SystemFillInMissingData(system, template)
    // Add system to the same object groups template is member of (if any).
    db.SystemAddToGroups(system.Text("dn"), db.SystemGetGroupsWithMember(template.Text("dn")))
  }
  
  if existing_sys == nil { system.RemoveFirst("iphostnumber") }
  if ip != "" { system.FirstOrAdd("iphostnumber").SetText(ip) }
    
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
  if system.First("faistate") == nil {
    system.Add("faistate", "install")
  }
  
  // gotoMode is always active
  system.FirstOrAdd("gotomode").SetText("active")
  
  // Update LDAP data or create new entry (if existing_sys==nil)
  err = db.SystemReplace(existing_sys, system)
  if err != nil {
    emsg := fmt.Sprintf("Could not create/update LDAP object %v: %v", system.Text("dn"), err)
    util.Log(0, "ERROR! %v", emsg)
    // This error is fatal, unless there is an existing object. 
    // If there is an existing object we can continue and create the install job.
    if existing_sys == nil { return ErrorReplyXML(emsg) }
  }
  
  // ========== create install job ============
  job := xml.NewHash("job")
  job.Add("progress", "none")
  job.Add("status", "waiting")
  job.Add("siserver", config.ServerSourceAddress)
  job.Add("modified", "1")
  job.Add("targettag", macaddress)
  job.Add("macaddress", macaddress)
  job.Add("plainname", "none") // updated automatically
  job.Add("timestamp", timestamp)
  job.Add("headertag", "trigger_action_reinstall")
  job.Add("result", "none")
  
  util.Log(1, "INFO! job_trigger_activate_new(): Creating install job for %v (%v)", macaddress, system.Text("dn"))
  
  db.JobAddLocal(job)
  
  // ======== In case the system is already waiting, tell it it is activated. =======
  client := db.ClientWithMAC(macaddress)
  if client != nil {
    // get complete data, including from groups
    system,_ = db.SystemGetAllDataForMAC(macaddress, true)
    if system != nil {
      Send_set_activated_for_installation(client.Text("client"), old_gotomode, system)
    }
  }
  
  answer := xml.NewHash("xml", "header", "answer")
  answer.Add("source", config.ServerSourceAddress)
  answer.Add("target", xmlmsg.Text("source"))
  answer.Add("answer1", "0")
  answer.Add("session_id", "1")
  return answer
}


