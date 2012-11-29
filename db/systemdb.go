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
         "net"
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

// Returns the IP address (IPv4 if possible) for the machine with the given name.
// The name may or may not include a domain.
// Returns "none" if the IP address could not be determined.
//
// ATTENTION! This function accesses a variety of external sources
// and may therefore take a while. If possible you should use it asynchronously.
func IPAddressForName(host string) string {
  addrs, err := net.LookupIP(host)
  if err != nil || len(addrs) == 0 { 
    util.Log(0, "ERROR! LookupIP(%v): %v", host, err)
    return "none" 
  }

  ip := addrs[0].String() // this may be an IPv6 address
  // try to find an IPv4 address
  for _, a := range addrs {
    if a.To4() != nil {
      ip = a.To4().String()
      break
    }
  }
  
  // translate loopback address to our own IP for consistency
  if ip == "127.0.0.1" { ip = config.IP }
  return ip
}

// Returns the name for the given ip (fully qualified if possible), or "none" if it can't be determined.
func NameForIPAddress(ip string) string {
  names, err := net.LookupAddr(ip)
  if err != nil || len(names) == 0 {
    util.Log(0, "ERROR! Reverse lookup of %v failed: %v",ip,err)
    return "none"
  }
  
  return strings.TrimRight(names[0],".")
}

// Returns the MAC address for the given host name or "none" if it can't be determined.
func MACForName(host string) string {
  system, err := xml.LdifToHash("", true, LdapSearch(fmt.Sprintf("(&(objectClass=GOHard)(cn=%v)%v)",host, config.UnitTagFilter),"macaddress"))
  mac := system.Text("macaddress")
  if mac == "" {
    parts := strings.SplitN(host,".",2)
    if len(parts) == 2 {
      system, err = xml.LdifToHash("", true, LdapSearch(fmt.Sprintf("(&(objectClass=GOHard)(cn=%v)%v)",parts[0], config.UnitTagFilter),"macaddress"))
      mac = system.Text("macaddress")
    }
    if mac == "" {  
      util.Log(0, "ERROR! Error getting MAC for cn %v: %v", host, err)
      return "none"
    }
  }
  return mac
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
