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
// The name will not include a domain. Use FullyQualifiedNameForMAC() if
// you need a domain.
// Returns "none" if the name could not be determined. Since this is a valid
// system name, you should NOT special case this (e.g. use it to check if
// the system is known).
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemPlainnameForMAC(macaddress string) string {
  name := SystemCommonNameForMAC(macaddress)
  if name == "" { return "none" }
  
  // return only the name without the domain (in case the cn includes a domain)
  return strings.SplitN(name, ".", 2)[0]
}  

// Returns the fully qualified name for the system with the given MAC address.
// Returns "none" if the fully qualified name could not be determined, even 
// if a short name might be available, so you should used PlainnameForMAC() if
// you don't need a domain.
//
// ATTENTION! This function accesses LDAP and may perform multiple DNS lookups.
// It may therefore take a while. If possible you should use it asynchronously.
func SystemFullyQualifiedNameForMAC(macaddress string) string {
  system, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOHard)(macAddress=%v)%v)",macaddress, config.UnitTagFilter),"cn","ipHostNumber"))
  name := system.Text("cn")
  if name == "" {
    util.Log(0, "ERROR! Error getting cn for MAC %v: %v", macaddress, err)
    return "none"
  }

  name = strings.ToLower(name)
  has_domain := strings.Index(name,".") > 0
  
  // try name lookup. If the name has no domain, we will then
  // use reverse lookup to try to determine the domain.
  // If the name already has a domain, then this call serves
  // to verify it is correct.
  ip := SystemIPAddressForName(name)
  if ip != "none" {
    // if the name has a domain already, we're done
    if has_domain { return name }
    
    // Otherwise we try to find the domain by reverse lookup of the IP.
    longname := SystemNameForIPAddress(ip)
    if strings.Index(longname,".") > 0 { return longname }
    return "none" // if reverse lookup failed, something is broken => give up
    
  } else // DNS lookup of name failed
  {
    // I feel ambivalent about the following line.
    // If the CN for the system does include a domain name that is incorrect,
    // that's a serious admin mistake. It doesn't seem like a good idea to
    // try working around this by looking for the name in other domains.
    // Especially since the result may be a different machine that just happens
    // to have the same short name.
    // On the other hand, I like the idea of having things "Just Work" even if
    // things happen like a domain being renamed.
    // For the time being I've decided to be strict and abort.
    // NOTE: There's no ERROR being logged, because IPAddressForName() already
    // includes that.
    if has_domain { return "none" }
    
    // The name is a short name that can't be resolved. This happens in
    // networks with multiple subdomains some of which are not listed
    // in resolv.conf, possibly because of the limits of the "search"
    // directive.
    // If the ipHostNumber field of the LDAP entry is properly set, we try
    // a reverse lookup to see if we get a name that matches.
    if ip := system.Text("iphostnumber"); ip != "" {
      longname := SystemNameForIPAddress(ip)
      if strings.HasPrefix(longname, name+".") { return longname }
    }
    
    // Now we enter the realm of black magic and guesswork.
    // We try appending all domains we know to the system's name and check
    // if that way we can find a name that resolves.
    for _, domain := range SystemDomainsKnown() {
      longname := strings.ToLower(name + domain)
      if SystemIPAddressForName(longname) != "none" { return longname }
    }
  }
  
  return "none"
}

// Returns the CN stored in LDAP for the system with the given MAC address.
// It may or may not include a domain.
// Use PlainnameForMAC() or FullyQualifiedNameForMAC() if you want
// predictability.
//
// Returns "" (NOT "none" like the other functions!) 
// if the name could not be determined. 
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemCommonNameForMAC(macaddress string) string {
  system, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOHard)(macAddress=%v)%v)",macaddress, config.UnitTagFilter),"cn"))
  name := system.Text("cn")
  if name == "" {
    util.Log(0, "ERROR! Error getting cn for MAC %v: %v", macaddress, err)
  }
  return name
}

// Returns the IP address (IPv4 if possible) for the machine with the given name.
// The name may or may not include a domain.
// Returns "none" if the IP address could not be determined.
//
// ATTENTION! This function accesses a variety of external sources
// and may therefore take a while. If possible you should use it asynchronously.
func SystemIPAddressForName(host string) string {
  addrs, err := net.LookupIP(host)
  if err != nil || len(addrs) == 0 { 
    // if host already contains a domain, give up
    if strings.Index(host, ".") >= 0 {
      util.Log(0, "ERROR! LookupIP(%v): %v", host, err)
      return "none" 
    }
    
    // if host does not contain a domain the DNS failure may simple be
    // caused by the machine being in a different subdomain. Try to
    // work around this by searching LDAP for the machine and use its
    // ipHostNumber if it is accurate.
    util.Log(1, "INFO! Could not resolve short name %v (error: %v). Trying LDAP.", host, err)
    var system *xml.Hash
    system, err = xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOHard)(|(cn=%v)(cn=%v.*))%v)",host, host,config.UnitTagFilter),"ipHostNumber"))
    // the search may give multiple results. Use reverse lookup of ipHostNumber to
    // find the correct one (if there is one)
    for ihn := system.First("iphostnumber"); ihn != nil; ihn = ihn.Next() {
      ip := ihn.Text()
      fullname := SystemNameForIPAddress(ip)
      if strings.HasPrefix(fullname, host+".") { 
        util.Log(1, "INFO! Found \"%v\" with IP %v in LDAP", fullname, ip)
        // use forward lookup for the full name to be sure we get the proper address
        return SystemIPAddressForName(fullname)
      }
    }
    util.Log(0, "ERROR! Could not get reliable IP address for %v from LDAP", host)
    return "none" 
  }

  ip := addrs[0].String() // this may be an IPv6 address
  
  // try to find a non-loopback address
  for _, a := range addrs {
    if !a.IsLoopback() {
      ip = a.String()
      break
    } else { 
      ip = config.IP // translate loopback address to our own IP for consistency
    }
  }
  
  // try to find an IPv4 non-loopback address
  for _, a := range addrs {
    if !a.IsLoopback() && a.To4() != nil {
      ip = a.To4().String()
      break
    }
  }
  
  return ip
}

// Returns the name for the given ip (fully qualified if possible), or "none" if it can't be determined.
func SystemNameForIPAddress(ip string) string {
  names, err := net.LookupAddr(ip)
  if err != nil || len(names) == 0 {
    util.Log(0, "ERROR! Reverse lookup of \"%v\" failed: %v",ip,err)
    return "none"
  }
  
  // find longest name (that should be the one with the domain)
  best := 0
  for i := range names {
    if len(names[i]) > len(names[best]) { best = i }
  }
  return strings.ToLower(strings.TrimRight(names[best],".")) // trim off trailing dot from reverse lookup
}

// Returns the MAC address for the given host name or "none" if it can't be determined.
func SystemMACForName(host string) string {
  system, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOHard)(cn=%v)%v)",host, config.UnitTagFilter),"macaddress"))
  mac := system.Text("macaddress")
  if mac == "" {
    parts := strings.SplitN(host,".",2)
    if len(parts) == 2 {
      system, err = xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOHard)(cn=%v)%v)",parts[0], config.UnitTagFilter),"macaddress"))
      mac = system.Text("macaddress")
    } else {
      system, err = xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOHard)(cn=%v.*)%v)",parts[0], config.UnitTagFilter),"macaddress"))
      mac = system.Text("macaddress")
    }
    if mac == "" {  
      util.Log(0, "ERROR! Error getting MAC for cn %v: %v", host, err)
      return "none"
    }
  }
  return mac
}

// Returns true if the system identified by macaddress is known to be
// a workstation (rather than a server).
func SystemIsWorkstation(macaddress string) bool {
  system, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=gotoWorkstation)(macaddress=%v)%v)",macaddress, config.UnitTagFilter),"macaddress"))
  return (err == nil && system.First("macaddress") != nil)
}

// Returns a list of domains (all beginning with a ".", e.g. ".example.com")
// go-susi has learned from a variety of sources. These domains can be used
// to guess fully qualified names from short names that cannot be resolved by 
// DNS.
func SystemDomainsKnown() []string { return append(config.LookupDomains,"."+config.Domain) }

// Returns a list of IP addresses that are representatives of different subnets
// go-susi has learned. The returned addresses are not necessarily broadcast
// addresses.
func SystemNetworksKnown() []string { return []string{config.IP} }

// Changes the attribute attrname to attrvalue for the system
// identified by the given macaddress. If the system has multiple attribute
// values for attrname they will all be removed and after this function call
// only the single value attrvalue will remain.
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemSetState(macaddress string, attrname, attrvalue string) {
  err := SystemSetStateMulti(macaddress, attrname, []string{attrvalue})
  if err != nil {
    util.Log(0, "ERROR! %v", err)
  }
}

// Replaces the attribute attrname with the list of attrvalues for the system
// identified by the given macaddress. If attrvalues is empty, the attribute is
// removed from the object. If an error occurs or no system is found with the
// given macaddress an error will be returned (otherwise nil).
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemSetStateMulti(macaddress string, attrname string, attrvalues []string) error {
  system, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOHard)(macAddress=%v)%v)",macaddress, config.UnitTagFilter),"dn"))
  dn := system.Text("dn")
  if dn == "" {
    return fmt.Errorf("Could not get dn for MAC %v: %v", macaddress, err)
  }
  out, err := ldapModifyAttribute(dn, "replace", attrname, attrvalues).CombinedOutput()
  if err != nil {
    return fmt.Errorf("Could not change state of object %v: %v (%v)",dn,err,string(out))
  }
  return nil
}


// Returns all values of attribute attrname for the system
// identified by the given macaddress concatenated into a single string
// separated by \u241e (symbol for record separator). If the system is not
// found or has no such attribute, the empty string "" is returned.
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemGetState(macaddress string, attrname string) string {
  system, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOHard)(macAddress=%v)%v)",macaddress, config.UnitTagFilter),attrname))
  if err != nil {
    util.Log(0, "ERROR! Could not get attribute %v for MAC %v: %v", attrname, macaddress, err)
    return ""
  }
  return system.Text(strings.ToLower(attrname))
}

// Returns the complete data available for the system identified by the given 
// macaddress. If an error occurs or the system is not found, 
// the returned data is "<xml></xml>" and the 2nd return value is the error.
// The format of the returned data is
// <xml>
//  <dn>...</dn>
//  <faiclass>...</faiclass>
//  <objectclass>objectclass_1</objectclass>
//  <objectclass>objectclass_2</objectclass>
//   ...
// <xml>
//
// If use_groups is true, then data from object groups the system is a member of
// will be used to complete the data. If use_groups is false, only the system
// object's data is returned.
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemGetAllDataForMAC(macaddress string, use_groups bool) (*xml.Hash, error) {
  x, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOHard)(macAddress=%v)%v)",macaddress, config.UnitTagFilter)))
  if err != nil { return x, err }
  if x.First("dn") == nil { return x, fmt.Errorf("Could not find system with MAC %v", macaddress) }
  if use_groups {
    dn := x.Text("dn")
    groups := SystemGetGroupsWithMember(dn)
    for group := groups.First("xml"); group != nil; group = group.Next() {
      SystemFillInMissingData(x, group)
    }
  }
  return x, err
}

// Returns all system templates that apply to the given system (which may be
// incomplete). 
//
// The format of the reply is:
//   <systemdb>
//     <xml>
//       <dn>...</dn>
//       <objectclass>objectclass_1</objectclass>
//       <objectclass>objectclass_2</objectclass>
//       ...
//     </xml>
//     <xml>
//       <dn>...</dn>
//       ...
//     </xml>
//     ...
//   </systemdb>
//
// The order of the template objects is such that the first entry is the
// best match (i.e. the template with the most specific matching rules).
// See "detected_hardware" documentation in the go-susi operator's manual for 
// information on how to mark a system object as a template and how to specify
// the systems to which it should apply.
//
// If there is no matching template, the returned hash is <systemdb></systemdb>.
//
// NOTE: The returned template objects always contain only the attributes from
// the objects themselves, not from any groups they may be members of.
// 
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemGetTemplatesFor(system *xml.Hash) *xml.Hash {
  return xml.NewHash("systemdb")
}

// Returns all gosaGroupOfNames objects that have the given dn as a member.
// The format is the same as for SystemGetTemplatesFor().
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemGetGroupsWithMember(dn string) *xml.Hash {
  x, err := xml.LdifToHash("xml", true, ldapSearch(fmt.Sprintf("(&(objectClass=gosaGroupOfNames)(member=%v)%v)",dn, config.UnitTagFilter)))
  if err != nil { 
    util.Log(0, "ERROR! %v", err)
    return xml.NewHash("systemdb")
   }
  x.Rename("systemdb")
  return x
}

// Takes 2 hashes in the format returned by SystemGetAllDataForMAC() and adds
// attributes from defaults to system where appropriate. This function understands
// system objects and will not add inappropriate attributes. For instance if
// defaults represents a gosaGroupOfNames, this function will not copy the "member"
// attributes to system.
// If defaults has objectClass gosaAdministrativeUnitTag but system doesn't,
// this function will add that objetClass and the gosaUnitTag to system. Other
// objectClasses are never touched.
//
// If system has no dn but defaults has one, then system will get a dn
// derived by replacing the last component of defaults' dn by
// cn=<system's cn>  (unless system has no cn).
//
// NOTE: The attribute names are treated as case-insensitive. It is not
// necessary that defaults and system use the same case for the same
// attributes.
func SystemFillInMissingData(system *xml.Hash, defaults *xml.Hash) {
}

// Adds the system with the given dn as a member to the gosaGroupOfNames
// objects in groups which must have the same format as returned by
// SystemGetGroupsWithMember().
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemAddToGroups(dn string, groups *xml.Hash) {
  for group := groups.First("xml"); group != nil; group = group.Next() {
    out, err := ldapModifyAttribute(group.Text("dn"), "add", "member", []string{dn}).CombinedOutput()
    if err != nil {
      util.Log(0, "ERROR! Could not add new member \"%v\" to group \"%v\": %v (%v)",dn, group.Text("dn"),err,string(out))
    }
  }
}

// Removes the system with the given dn from all gosaGroupOfNames
// objects in groups which must have the same format as returned by
// SystemGetGroupsWithMember().
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemRemoveFromGroups(dn string, groups *xml.Hash) {
  for group := groups.First("xml"); group != nil; group = group.Next() {
    out, err := ldapModifyAttribute(group.Text("dn"), "delete", "member", []string{dn}).CombinedOutput()
    if err != nil {
      util.Log(0, "ERROR! Could not remove member \"%v\" from group \"%v\": %v (%v)",dn, group.Text("dn"),err,string(out))
    }
  }
}

// Updates the data for the given system, creating it if it does not yet exist.
// The format of system is the same as returned by SystemGetAllDataForMAC().
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemUpdate(system *xml.Hash) {
}

// Removes the system with the given dn from the database.
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemRemove(dn string) {
}


func ldapSearch(query string, attr... string) *exec.Cmd {
  args := []string{"-x", "-LLL", "-H", config.LDAPURI, "-b", config.LDAPBase}
  if config.LDAPUser != "" { args = append(args,"-D",config.LDAPUser,"-y",config.LDAPUserPasswordFile) }
  args = append(args, query)
  args = append(args, attr...)
  util.Log(2, "DEBUG! ldapsearch %v",args)
  return exec.Command("ldapsearch", args...)
}

func ldapModifyAttribute(dn, modifytype, attrname string, attrvalues []string) *exec.Cmd {
  args := []string{"-x", "-H", config.LDAPURI}
  args = append(args,"-D",config.LDAPAdmin,"-y",config.LDAPAdminPasswordFile)
  util.Log(2, "DEBUG! ldapmodify %v (%v %v -> %v for %v)",args, modifytype, attrname, attrvalues, dn)
  cmd := exec.Command("ldapmodify", args...)
  bufstr := bytes.NewBufferString(fmt.Sprintf(`dn:: %v
changetype: modify
%v: %v
`,base64.StdEncoding.EncodeToString([]byte(dn)),
  modifytype,
  attrname))

  for i := range attrvalues {
    bufstr.WriteString(fmt.Sprintf(`%v:: %v
`, attrname, base64.StdEncoding.EncodeToString([]byte(attrvalues[i]))))
  }
  
  cmd.Stdin = bufstr
  return cmd
}
