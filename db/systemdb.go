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
         "strings"
         "os/exec"
         
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

func LdapSearch(query string, attr... string) *exec.Cmd {
  args := []string{"-x", "-LLL", "-H", config.LDAPURI, "-b", config.LDAPBase}
  if config.LDAPUser != "" { args = append(args,"-D",config.LDAPUser,"-w",config.LDAPUserPassword) }
  args = append(args, query)
  args = append(args, attr...)
  util.Log(1, "INFO! ldapsearch %v",args)
  return exec.Command("ldapsearch", args...)
}
