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
         "time"
         "bytes"
         "regexp"
         "strings"
         "strconv"
         "encoding/base64"
         
         "../xml"
         "../util"
         "../util/deque"
         "../config"
       )

// Set of attributes that should not be copied by SystemFillInMissingData() even
// if the target does not have them. 
var DoNotCopyAttribute = map[string]bool{"dn":true, "cn":true,
                                         "macaddress":true, "iphostnumber":true,
                                         "member":true, "gosagroupobjects":true,
                                         "description":true, "gocomment":true }

// Set of objectclasses that should be copied by SystemFillInMissingData().
var CopyObjectclass = map[string]bool{"top":true,"FAIobject":true, "GOhard":true,
                                      "gosaAdministrativeUnitTag":true, 
                                      "gotoWorkstation":true,
                                      "goServer":true,
                                      "FAIrepositoryServer":true,
                                      "goCupsServer":true,
                                      "goEnvironmentServer":true,
                                      "goFaxServer":true,
                                      "goFonHomeServer":true,
                                      "goFonServer":true,
                                      "goGlpiServer":true,
                                      "goImapServer":true,
                                      "goImapSieveServer":true,
                                      "goKrbServer":true,
                                      "goLdapServer":true,
                                      "goLogDBServer":true,
                                      "goMailServer":true,
                                      "goNfsServer":true,
                                      "goNtpServer":true,
                                      "goShareServer":true,
                                      "goSpamServer":true,
                                      "goSyslogServer":true,
                                      "goTerminalServer":true,
                                      "goVirusServer":true,
                                      "gosaLogServer":true,
                                      "gosaMailServer":true,
                                      "gotoLdapServer":true,
                                      "gotoLpdServer":true,
                                      "gotoNtpServer":true,
                                      "gotoProfileServer":true,
                                      "gotoSwapServer":true,
                                      "gotoSyslogServer":true,
                                      "gotoXdmcpServer":true,
                                      }

// template matching rules are rejected if an attribute name does not match this re.
var attributeNameRegexp = regexp.MustCompile("^[a-zA-Z]+$")

// Returns a short name for the system with the given MAC address.
// The name will not include a domain. Use FullyQualifiedNameForMAC() if
// you need a domain.
// Returns "none" if the name could not be determined. Since this is a valid
// system name, you should NOT special case this (e.g. use it to check if
// the system is known).
//
// ATTENTION! This function may access LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemPlainnameForMAC(macaddress string) string {
  name := "none"
  
  // if we have an IP for the client, try reverse DNS, unless the client is
  // running on a non-standard port (test client)
  client := ClientWithMAC(macaddress)
  if client != nil {
    ipport := strings.SplitN(client.Text("client"),":",2)
    ip := ipport[0]
    port := ipport[1]
    for _, standard_port := range config.ClientPorts {
      if port == standard_port {
        name = SystemNameForIPAddress(ip)
        break
      }
    }
  }
  
  // if DNS failed (probably because we don't know the client), try LDAP
  if name == "none" {
    name = SystemCommonNameForMAC(macaddress)
    if name == "" { name = "none" }
  }
  
  // return only the name without the domain (if there is one)
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
  system, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOhard)(macAddress=%v)%v)",LDAPFilterEscape(macaddress), config.UnitTagFilter),"cn","ipHostNumber"))
  names := system.Get("cn")
  if len(names) == 0 {
    util.Log(0, "ERROR! Error getting cn for MAC %v: %v", macaddress, err)
    return "none"
  }
  if len(names) != 1 {
    util.Log(0, "ERROR! Multiple LDAP objects with same MAC %v: %v", macaddress, names)
    return "none"
  }

  name := strings.ToLower(names[0])
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
  system, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOhard)(macAddress=%v)%v)",LDAPFilterEscape(macaddress), config.UnitTagFilter),"cn"))
  names := system.Get("cn")
  if len(names) == 0 {
    util.Log(0, "ERROR! Error getting cn for MAC %v: %v", macaddress, err)
    return ""
  }
  if len(names) != 1 {
    util.Log(0, "ERROR! Multiple LDAP objects with same MAC %v: %v", macaddress, names)
    return ""
  }
  return names[0]
}

// Returns the IP address (IPv4 if possible) for the machine with the given name.
// The name may or may not include a domain.
// Returns "none" if the IP address could not be determined.
//
// ATTENTION! This function accesses a variety of external sources
// and may therefore take a while. If possible you should use it asynchronously.
func SystemIPAddressForName(host string) string {
  ip, err := util.Resolve(host, config.IP)
  if err != nil { 
    // if host already contains a domain, give up
    if strings.Index(host, ".") >= 0 {
      util.Log(0, "ERROR! Resolve(\"%v\"): %v", host, err)
      return "none" 
    }
    
    // if host does not contain a domain the DNS failure may simple be
    // caused by the machine being in a different subdomain. Try to
    // work around this by searching LDAP for the machine and use its
    // ipHostNumber if it is accurate.
    util.Log(1, "INFO! Could not resolve short name %v (error: %v). Trying LDAP.", host, err)
    var system *xml.Hash
    system, err = xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOhard)(|(cn=%v)(cn=%v.*))%v)",LDAPFilterEscape(host), LDAPFilterEscape(host),config.UnitTagFilter),"ipHostNumber"))
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
  system, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOhard)(cn=%v)%v)",LDAPFilterEscape(host), config.UnitTagFilter),"macaddress"))
  mac := system.Text("macaddress")
  if mac == "" {
    parts := strings.SplitN(host,".",2)
    if len(parts) == 2 {
      system, err = xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOhard)(cn=%v)%v)",LDAPFilterEscape(parts[0]), config.UnitTagFilter),"macaddress"))
      mac = system.Text("macaddress")
    } else {
      system, err = xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOhard)(cn=%v.*)%v)",LDAPFilterEscape(parts[0]), config.UnitTagFilter),"macaddress"))
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
  system, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=gotoWorkstation)(macaddress=%v)%v)",LDAPFilterEscape(macaddress), config.UnitTagFilter),"macaddress"))
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
  system, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOhard)(macAddress=%v)%v)",LDAPFilterEscape(macaddress), config.UnitTagFilter),"dn"))
  dns := system.Get("dn")
  if len(dns) == 0 {
    return fmt.Errorf("Could not get dn for MAC %v: %v", macaddress, err)
  }
  if len(dns) > 1 {
    return fmt.Errorf("Multiple LDAP objects for MAC %v: %v", macaddress, dns)
  }
  dn := dns[0]
  out, err := ldapModifyAttribute(dn, "replace", attrname, attrvalues).CombinedOutput()
  if err != nil {
    return fmt.Errorf("Could not change state of object %v: %v (%v)",dn,err,string(out))
  } else {
    util.Log(2, "DEBUG! ldapmodify successful. Output: \"%v\"",string(out))
  }
  return nil
}

// Returns all values of attribute attrname for the system
// identified by the given macaddress concatenated into a single string
// separated by '␞' (\u241e, i.e. symbol for record separator). If the system is not
// found or has no such attribute, the empty string "" is returned.
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemGetState(macaddress string, attrname string) string {
  attrname = strings.ToLower(attrname)
  system, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOhard)(macAddress=%v)%v)",LDAPFilterEscape(macaddress), config.UnitTagFilter),attrname))
  dns := system.Get("dn")
  if err == nil && len(dns) == 0 {
    err = fmt.Errorf("Object not found")
  }
  if err != nil {
    util.Log(0, "ERROR! Could not get attribute %v for MAC %v: %v", attrname, macaddress, err)
    return ""
  }
  if len(dns) > 1 {
    util.Log(0, "ERROR! Multiple LDAP objects with MAC %v: %v", macaddress, dns)
    return ""
  }
  
  if system.First(attrname) == nil && !DoNotCopyAttribute[attrname] {
    dn := system.Text("dn")
    groups, err := xml.LdifToHash("xml", true, ldapSearch(fmt.Sprintf("(&(objectClass=gosaGroupOfNames)(member=%v)%v)",LDAPFilterEscape(dn), config.UnitTagFilter), attrname))  
    if err != nil {
      util.Log(0, "ERROR! Could not get groups with member %v: %v", dn, err)
      return ""
    }
    count := 0
    for group := groups.First("xml"); group != nil; group = group.Next() {
      if group.First(attrname) != nil {
        system = group
        count++
      }
    }
    if count > 1 {
      util.Log(0, "WARNING! Multiple groups provide attribute %v for %v: %v", attrname, dn, groups)
    }
  }
  
  return system.Text(attrname)
}

// Sets the selected system's faistate and removes all running install and update
// jobs affecting the system.
//
// ATTENTION! This function takes a while to complete because it tries multiple
// times if necessary and verifies that the faistate has actually been set.
func SystemForceFAIState(macaddress, faistate string) {
  util.Log(1, "INFO! Forcing faiState for %v to %v", macaddress, faistate)
  
  // retry for 30s
  endtime := time.Now().Add(30*time.Second)
  
  for ; time.Now().Before(endtime);  {
    SystemSetState(macaddress, "faiState", faistate)
    
    // remove softupdate and install jobs ...
    job_types_to_kill := xml.FilterOr(
                         []xml.HashFilter{xml.FilterSimple("headertag","trigger_action_reinstall"),
                                          xml.FilterSimple("headertag","trigger_action_update")})
    // ... that are already happening or scheduled within the next 5 minutes ...
    timeframe := xml.FilterRel("timestamp", util.MakeTimestamp(time.Now().Add(5*time.Minute)),-1,0)
    // ... that affect the machine for which we force the faistate
    target := xml.FilterSimple("macaddress", macaddress)
    filter := xml.FilterAnd([]xml.HashFilter{ job_types_to_kill,
                                                  timeframe,
                                                  target })
    JobsRemove(filter)
    
    // Wait a little and see if the jobs are gone
    time.Sleep(3*time.Second)
    if JobsQuery(filter).FirstChild() == nil { // if all jobs are gone
      // set state again just in case the job removal raced with something that set faistate
      SystemSetState(macaddress, "faiState", faistate)
      return // we're done
    } // else if some jobs remained
    
    util.Log(2, "DEBUG! ForceFAIState(%v, %v): Some install/softupdate jobs remain => Retrying", macaddress, faistate)
  }
  
  util.Log(0, "ERROR! ForceFAIState(%v, %v): Some install/softupdate jobs could not be removed.", macaddress, faistate)
}

type SystemNotFoundError struct {
  error
}

// Returns the complete data available for the system identified by the given 
// macaddress. If an error occurs or the system is not found, 
// the returned data is nil and the 2nd return value is the error.
// If the system is not found, the error will be of type SystemNotFoundError.
// This can be used to distinguish this case from other kinds of failures.
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
  x, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOhard)(macAddress=%v)%v)",LDAPFilterEscape(macaddress), config.UnitTagFilter)))
  if err != nil { return nil, err }
  dns := x.Get("dn")
  if len(dns) == 0 { return nil, SystemNotFoundError{fmt.Errorf("Could not find system with MAC %v", macaddress)} }
  if len(dns) > 1 { return nil, fmt.Errorf("Multiple LDAP objects with MAC %v: %v", macaddress, dns)}
  if use_groups {
    dn := dns[0]
    groups := SystemGetGroupsWithMember(dn)
    for group := groups.First("xml"); group != nil; group = group.Next() {
      SystemFillInMissingData(x, group)
    }
  }
  return x, err
}

// Returns the LDAP object for the local printer of the system identified by the given 
// macaddress. If an error occurs or the object is not found, 
// the returned data is nil and the 2nd return value is the error.
// If the object is not found, the error will be of type SystemNotFoundError.
// This can be used to distinguish this case from other kinds of failures.
// The format of the returned data is
// <xml>
//  <dn>...</dn>
//  <cn>...</cn>
//  <labeledURI>...</labeledURI>
//  <gotoPrinterPPD>...</gotoPrinterPPD>
//  <ipHostNumber>...</ipHostNumber>
//  <macAddress>...</macAddress>
//  <objectclass>objectclass_1</objectclass>
//  <objectclass>objectclass_2</objectclass>
//   ...
// <xml>
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemLocalPrinterForMAC(macaddress string) (*xml.Hash, error) {
  x, err := xml.LdifToHash("", true, ldapSearch(fmt.Sprintf("(&(objectClass=gotoPrinter)(macAddress=%v)%v)",LDAPFilterEscape(macaddress), config.UnitTagFilter)))
  if err != nil { return nil, err }
  dns := x.Get("dn")
  if len(dns) == 0 { return nil, SystemNotFoundError{fmt.Errorf("Could not find local printer for MAC %v", macaddress)} }
  if len(dns) > 1 { return nil, fmt.Errorf("Multiple LDAP objects with objectClass=gotoPrinter and MAC %v: %v", macaddress, dns)}
  
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
  // ATTENTION: No space between "for" and "*" because there may be some other
  // kind of whitespace (such as CR)
  x, err := xml.LdifToHash("xml", true, ldapSearch(fmt.Sprintf("(&(objectClass=GOhard)(gocomment=Template for*/*/*)%v)", config.UnitTagFilter)))
  if err != nil { 
    util.Log(0, "ERROR! LDAP error while looking for template objects: %v", err)
    return xml.NewHash("systemdb")
  }
  
  templates := deque.New() // templates.At(0) is the most specific match
  templates.Push(xml.NewHash("xml","TEMPLATE_MATCH_SPECIFICITY","0")) //sentinel
  
  for t := x.RemoveFirst("xml"); t != nil; t = x.RemoveFirst("xml") {
    specificity := templateMatch(system, t.Text("gocomment"))
    if specificity > 0 {
      t.Add("TEMPLATE_MATCH_SPECIFICITY", specificity)
      for i := 0; i < templates.Count(); i++ {
        tspec,_ := strconv.Atoi(templates.At(i).(*xml.Hash).Text("TEMPLATE_MATCH_SPECIFICITY"))
        if specificity >= tspec {
          templates.InsertAt(i, t)
          break
        }
      }
    }
  }
  
  templates.Pop() // remove sentinel
  
  ret := xml.NewHash("systemdb")
  
  for ; !templates.IsEmpty() ; {
    t := templates.Next().(*xml.Hash)
    t.RemoveFirst("TEMPLATE_MATCH_SPECIFICITY")
    ret.AddWithOwnership(t)
  }
  
  return ret
}

// Returns how well the rule gocomment matches system. 
// 0 means no match. Greater values are better (more specific) matches.
// Matching is done case-insensitive. Attribute names from system
// must be lowercase.
func templateMatch(system *xml.Hash, gocomment string) int {
  
  /*
    If you come here to debug a problem in the matching of template rules, 
    please accept my sincere apologies. I should have tested this code better.
    I didn't do it for the same reason you will hate debugging this code:
    It's 2 nested state machines with lots of if-then-else alternatives.
    It's too long and too clever.
    If it works, it's impressive. If it fails, it's bad engineering.
    
    But you have to give me some credit. At least I documented the meaning of the states.
  */
  
  rules := strings.ToLower(gocomment)
  if strings.Index(rules,"template for") == 0 { rules = rules[12:] }

  state := 0  // 0 => expect attribute name, 1 => expect match operator, 2 => expect regex start, 3 => expect regex end
  matchstate := 0 // 0 => processing negative matches, 1 => waiting for positive match, 2 => have positive match for current group
  
  attrname := "" // the attribute name of the matching rule being parsed
  regex := ""    // the regex of the matching rule being parsed
  score := 0     // counts the number of successful regex matches
  
  parts := strings.Fields(rules)
  
  for i := 0; i < len(parts); i++ {
    if parts[i] == "" { continue }
    
    switch state {
      case 0: attrname = parts[i]
              k := 0
              for k < len(attrname) && (attrname[k] != '!' && attrname[k] != '~' && attrname[k] != '=') { k++ }
              
              attrname = attrname[0:k]
              if !attributeNameRegexp.MatchString(attrname) {
                util.Log(0, "ERROR! Matching rule \"%v\" contains illegal attribute name \"%v\"", gocomment, attrname)
                return 0
              }
              if k != len(parts[i]) {
                parts[i] = parts[i][k:]
                i--
              }
              state++
      
      case 1: k := 0
              for k < len(parts[i]) && (parts[i][k] == '=' || parts[i][k] == '~' || parts[i][k] == '!') { k++ }
              op := parts[i][0:k]
              
              if op == "~=" || op == "=~" || op == "="||  op == "~" {
                if matchstate != 2 {
                  matchstate = 1
                }
              } else if op == "!=" || op == "!~" || op == "!" || op == "~!" {
                if matchstate == 1 { // no positive match for the preceding positive matching group
                  return 0
                }
                matchstate = 0
              } else {
                util.Log(0, "ERROR! Encountered \"%v\" but expected \"=~\" or \"!~\" in matching rule \"%v\"", parts[i], gocomment)
                return 0
              }
              if k != len(parts[i]) {
                parts[i] = parts[i][k:]
                i--
              }
              state++
      
      case 2: if parts[i][0] != '/' {
                util.Log(0, "ERROR! Encountered \"%v\" but expected \"/\" in matching rule \"%v\"", parts[i], gocomment)
                return 0
              }
              
              regex = ""
              parts[i] = parts[i][1:]
              state++
              fallthrough
      case 3:
              // except in the fallthrough case from state==2 (which we recognize by regex=="") whenever we
              // come here that means that the original gocomment contained whitespace within the regex.
              // We translate such whitespace to \s+.
              if regex != "" { regex += "\\s+" }

              chars := strings.Split(parts[i],"") // split parts[i] int UTF-8 sequences
              for k := 0; k < len(chars); k++ {
                ch := chars[k]
                if ch == "\\" && k == len(chars)-1 {
                  // If the part ends with an unescaped backslash that means that the original string
                  // contained backslash followed by some whitespace character. Since we replace all
                  // whitespace characters with "\s+" the backslash needs to be removed or it would
                  // combine with the "\s" to "\\s" which is wrong.
                } else if ch == "\\" {
                  regex += ch
                  k++
                  regex += chars[k]
                } else if ch == "/" { // regex end marker
                  re, err := regexp.Compile(regex)
                  if err != nil {
                    util.Log(0, "ERROR! Cannot parse regular expression \"%v\" in matching rule \"%v\"", regex, gocomment)
                    return 0
                  }
                  
                  attrs := system.Get(attrname)
                  
                  // pseudo-attribute "siserver" resolves to our own IP address and name
                  if attrname == "siserver" {
                    attrs = []string{config.IP, SystemNameForIPAddress(config.IP)}
                  }
                  
                  if matchstate == 0 {
                    if len(attrs) == 0 && re.MatchString("") { return 0 }
                    for _, attr := range attrs {
                      if re.MatchString(strings.ToLower(attr)) { return 0 }
                    }
                    score++
                  } else { // if matchstate == 1 || matchstate == 2
                    if len(attrs) == 0 && re.MatchString("") { 
                      matchstate = 2 
                      score++
                    } 
                    for _, attr := range attrs {
                      if re.MatchString(strings.ToLower(attr)) { 
                        matchstate = 2
                        score++
                        break
                      }
                    }
                  }
                  
                  k++ // skip "/"
                  if k < len(chars) {
                    parts[i] = strings.Join(chars[k:],"")
                    i--
                  }
                  
                  state = 0
                  break
                  
                } else {
                  regex += ch
                }
              } // for (case 3)
    } // switch state
  } // for 
  
  switch state {
    case 0: if matchstate == 1 { return 0 } else { return score }
    case 1: util.Log(0, "ERROR! Premature end of matching rule. Expected \"=~\" or \"!~\" in matching rule \"%v\"", gocomment)
    case 2: util.Log(0, "ERROR! Premature end of matching rule. Expected \"/\" in matching rule \"%v\"", gocomment)
    case 3: util.Log(0, "ERROR! Unterminated regex in matching rule \"%v\"", gocomment)
  }
  
  return 0
}

// Returns all gosaGroupOfNames objects that have the given dn as a member.
// The format is the same as for SystemGetTemplatesFor().
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemGetGroupsWithMember(dn string) *xml.Hash {
  x, err := xml.LdifToHash("xml", true, ldapSearch(fmt.Sprintf("(&(objectClass=gosaGroupOfNames)(member=%v)%v)",LDAPFilterEscape(dn), config.UnitTagFilter)))
  if err != nil { 
    util.Log(0, "ERROR! %v searching for (&(objectClass=gosaGroupOfNames)(member=%v)%v)", err, dn, config.UnitTagFilter)
    return xml.NewHash("systemdb")
   }
  x.Rename("systemdb")
  return x
}

// Returns all gosaGroupOfNames objects with the given cn.
// The format is the same as for SystemGetTemplatesFor().
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemGetGroupsWithName(cn string) *xml.Hash {
  x, err := xml.LdifToHash("xml", true, ldapSearch(fmt.Sprintf("(&(objectClass=gosaGroupOfNames)(cn=%v)%v)",LDAPFilterEscape(cn), config.UnitTagFilter)))
  if err != nil { 
    util.Log(0, "ERROR! %v searching for (&(objectClass=gosaGroupOfNames)(cn=%v)%v)", err, cn, config.UnitTagFilter)
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
// Objectclasses are copied selectively based on the CopyObjectclass map.
//
// If system has no dn but defaults has one, then system will get a dn
// derived by replacing the last component of defaults' dn by
// cn=<system's cn>  (unless system has no cn).
//
// NOTE: All attribute names must be lowercase.
func SystemFillInMissingData(system *xml.Hash, defaults *xml.Hash) {
  if system.First("dn") == nil {
    if dn := defaults.Text("dn"); dn != "" {
      if parts := strings.SplitN(dn, ",", 2); len(parts) == 2 {
        if cn := system.Text("cn"); cn != "" {
          dn = "cn=" + cn + "," + parts[1]
          system.Add("dn", dn)
        }
      }
    }
  }
  
  for _, tag := range defaults.Subtags() {
    
    if DoNotCopyAttribute[tag] { continue }
    
    if system.First(tag) == nil {
      for ele := defaults.First(tag); ele != nil; ele = ele.Next() {
        system.AddClone(ele)
      }
    }
  }
  
  // add missing objectClasses if whitelisted
  oclasses := system.Get("objectclass")
  outer: for _,oc := range defaults.Get("objectclass") {
    if CopyObjectclass[oc] { 
      for _,oc2 := range oclasses { if oc2 == oc { continue outer } }
      system.Add("objectclass", oc)
    }
  }
  
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

// General method for making changes (additions, removals, modifications) to
// the system database.
// If non-nil, old and new must have the same format as returned by
// SystemGetAllDataForMAC().
// If old == nil, then a new object is added to the database with the
// data from nu. It is an error if an object with the same dn already exists.
// If nu == nil, then the object whose dn is old.Text("dn") will be deleted
// from the database. It is an error if it does not exist.
// If old and nu are both non-nil, the object old is modified so that its
// data is nu. The changes may include a change of dn. If the dn of
// old and nu is the same but the cn of nu is different, the
// dn of nu will be changed accordingly. This means that to change the cn
// of an object you can just change the cn attribute and SystemReplace() will
// take care of the dn.
// This doesn't work the other way, around, though. If you change the dn,
// you must change the cn accordingly.
// If the dn is changed, object groups that have the system as member will
// be updated.
// If the system is removed, it will be removed from object groups' member lists
// as well.
//
// ATTENTION! This function accesses LDAP and may therefore take a while.
// If possible you should use it asynchronously.
func SystemReplace(old *xml.Hash, nu *xml.Hash) error {
  if nu == nil { // delete
    if old == nil { return nil } // nothing to do
    dn := old.Text("dn")
    bufstr := bytes.NewBufferString(fmt.Sprintf("dn:: %v\nchangetype: delete\n",base64.StdEncoding.EncodeToString([]byte(dn))))
    groups := SystemGetGroupsWithMember(dn)
    for group := groups.First("xml"); group != nil; group = group.Next() {
      bufstr.WriteString(fmt.Sprintf(`
dn:: %v
changetype: modify
delete: member
member:: %v
-
`,
base64.StdEncoding.EncodeToString([]byte(group.Text("dn"))),
base64.StdEncoding.EncodeToString([]byte(dn)),
))
    }
    out, err := ldapModify(bufstr.String()).CombinedOutput()
    if err != nil {
      return fmt.Errorf("Error while attempting to delete %v: %v (%v)",dn,err,string(out))
    }
    return nil
  }
  
  if old == nil { // create
    dn := nu.Text("dn")
    bufstr := bytes.NewBufferString(fmt.Sprintf(`dn:: %v
changetype: add
`,base64.StdEncoding.EncodeToString([]byte(dn))))

    for _, tag := range nu.Subtags() {
      if tag == "dn" { continue }
      for x := nu.First(tag); x != nil; x = x.Next() {
        txt := x.Text()
        if txt != "" {
          bufstr.WriteString(fmt.Sprintf("%v:: %v\n", tag, base64.StdEncoding.EncodeToString([]byte(txt))))
        }
      }
    }
    
    out, err := ldapModify(bufstr.String()).CombinedOutput()
    if err != nil {
      return fmt.Errorf("Error while attempting to add %v: %v (%v)",dn, err, string(out))
    }
    return nil
  }
  
  // modify
  
  olddn := old.Text("dn")
  dn := nu.Text("dn")
  cn := nu.Text("cn")
  if olddn == dn && old.Text("cn") != cn { // if cn has changed but dn has not,
    // we must update dn to match the changed cn
    i := strings.Index(dn,",")
    if i < 0 {
      return fmt.Errorf("Broken or missing dn: %v", dn)
    }
    dn = "cn="+cn+dn[i:]
    nu.First("dn").SetText(dn)
  }
  
  bufstr := bytes.NewBufferString(fmt.Sprintf(`dn:: %v
changetype: modify
`,base64.StdEncoding.EncodeToString([]byte(olddn))))

  for _, tag := range nu.Subtags() {
    if tag == "dn" || tag == "cn" { continue }
    skip_because_empty := false
    if old.First(tag) == nil { 
      if nu.Text(tag) != "" {
        bufstr.WriteString("add: ") 
      } else {
        skip_because_empty = true
      }
    } else {
      bufstr.WriteString("replace: ")
    }
    
    if !skip_because_empty {
      bufstr.WriteString(tag)
      bufstr.WriteString("\n")

      for x := nu.First(tag); x != nil; x = x.Next() {
        txt := x.Text()
        if txt != "" {
          bufstr.WriteString(fmt.Sprintf("%v:: %v\n", tag, base64.StdEncoding.EncodeToString([]byte(txt))))
        }
      }

      bufstr.WriteString("-\n")
    }
  }

  for _, tag := range old.Subtags() {
    if tag == "dn" || tag == "cn" { continue }
    if nu.First(tag) != nil { continue }

    bufstr.WriteString("delete: ") 
    bufstr.WriteString(tag)
    bufstr.WriteString("\n-\n")
  }
  
  if dn != olddn {
    i := strings.Index(dn,",") + 1
    
    bufstr.WriteString(fmt.Sprintf(`
dn:: %v
changetype: modrdn
newrdn: cn=%v
deleteoldrdn: 1
newsuperior:: %v
`,base64.StdEncoding.EncodeToString([]byte(olddn)),
cn,
base64.StdEncoding.EncodeToString([]byte(dn[i:])),
))
  
    groups := SystemGetGroupsWithMember(olddn)
    for group := groups.First("xml"); group != nil; group = group.Next() {
      bufstr.WriteString(fmt.Sprintf(`
dn:: %v
changetype: modify
delete: member
member:: %v
-
add: member
member:: %v
-
`,
base64.StdEncoding.EncodeToString([]byte(group.Text("dn"))),
base64.StdEncoding.EncodeToString([]byte(olddn)),
base64.StdEncoding.EncodeToString([]byte(dn)),
))
    }
  }

  out, err := ldapModify(bufstr.String()).CombinedOutput()
  if err != nil {
    return fmt.Errorf("Error while attempting to change %v/%v: %v (%v)",olddn, dn, err, string(out))
  }

  return nil
}
