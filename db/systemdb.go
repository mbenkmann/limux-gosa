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

// API for the various databases used by go-susi.
package db

import (
         "fmt"
         "bytes"
         "strings"
         "os/exec"
         "encoding/base64"
         
         "../xml"
         "../util"
         "../config"
       )

// Returns a short name for the system with the given MAC address.
// The name may or may not include a domain. In fact technically the name
// may be anything.
// Returns "none" if the name could not be determined. Since this is a valid
// system name, you should NOT special case this (e.g. use it to check if
// the system is known).
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func PlainnameForMAC(macaddress string) string {
  system, err := xml.LdifToHash("", true, LdapSearch(fmt.Sprintf("(&(objectClass=GOHard)(macAddress=%v)%v)",macaddress, config.UnitTagFilter),"cn"))
  name := system.Text("cn")
  if name == "" {
    util.Log(0, "ERROR! Error getting cn for MAC %v: %v", macaddress, err)
    return "none"
  }
  
  // return only the name without the domain
  return strings.SplitN(name, ".", 2)[0]
}  

// Replaces the attribute attrname with the sole value attrvalue for the system
// identified by the given macaddress.
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemSetState(macaddress string, attrname, attrvalue string) {
  system, err := xml.LdifToHash("", true, LdapSearch(fmt.Sprintf("(&(objectClass=GOHard)(macAddress=%v)%v)",macaddress, config.UnitTagFilter),"dn"))
  dn := system.Text("dn")
  if dn == "" {
    util.Log(0, "ERROR! Could not get dn for MAC %v: %v", macaddress, err)
    return
  }
  out, err := LdapModify(dn, attrname, attrvalue).CombinedOutput()
  if err != nil {
    util.Log(0, "ERROR! Could not change state of object %v: %v (%v)",dn,err,string(out))
  }
}

// Returns the 1st value of attribute attrname for the system
// identified by the given macaddress.
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemGetState(macaddress string, attrname string) string {
  system, err := xml.LdifToHash("", true, LdapSearch(fmt.Sprintf("(&(objectClass=GOHard)(macAddress=%v)%v)",macaddress, config.UnitTagFilter),attrname))
  if err != nil {
    util.Log(0, "ERROR! Could not get attribute %v for MAC %v: %v", attrname, macaddress, err)
    return ""
  }
  return system.Text(strings.ToLower(attrname))
}

func LdapSearch(query string, attr... string) *exec.Cmd {
  args := []string{"-x", "-LLL", "-H", config.LDAPURI, "-b", config.LDAPBase}
  if config.LDAPUser != "" { args = append(args,"-D",config.LDAPUser,"-w",config.LDAPUserPassword) }
  args = append(args, query)
  args = append(args, attr...)
  util.Log(2, "DEBUG! ldapsearch %v",args)
  return exec.Command("ldapsearch", args...)
}

func LdapModify(dn string, attrname, attrvalue string) *exec.Cmd {
  args := []string{"-x", "-H", config.LDAPURI}
  args = append(args,"-D",config.LDAPAdmin,"-w",config.LDAPAdminPassword)
  util.Log(2, "DEBUG! ldapmodify %v (Set %v to '%v' for %v)",args, attrname, attrvalue, dn)
  cmd := exec.Command("ldapmodify", args...)
  cmd.Stdin = bytes.NewBufferString(fmt.Sprintf(`dn:: %v
changetype: modify
replace: %v
%v:: %v
`,base64.StdEncoding.EncodeToString([]byte(dn)),
  attrname,
  attrname,base64.StdEncoding.EncodeToString([]byte(attrvalue))))
  return cmd
}
