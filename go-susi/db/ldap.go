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
         "time"
         "bytes"
         "os/exec"
         "strings"
         "encoding/base64"
         
         "../xml"
         "../util"
         "../config"
       )

// Waits until either the duration timeout has passed or LDAP is available.
// If timeout == 0, wait forever if necessary.
// Returns true if LDAP is available.
func LDAPAvailable(timeout time.Duration) bool {
  endtime := time.Now().Add(timeout)
  for {
    _, err := xml.LdifToHash("adminunit", true, ldapSearch(fmt.Sprintf("(&(objectClass=gosaAdministrativeUnit)%v)", config.UnitTagFilter),"ou"))
    if err == nil { return true }
    if timeout != 0 && time.Now().After(endtime) { break }
    waittime := endtime.Sub(time.Now())
    if waittime <= 0 || waittime > 1*time.Second { waittime = 1*time.Second }
    time.Sleep(waittime)
  }
  return false
}

// Returns the dn and ou of the first object under config.LDAPBase that matches
// (&(objectClass=gosaAdministrativeUnit)(gosaUnitTag=<config.UnitTag>)).
// Logs an error and returns "","" if an error occurs or no object is found.
func LDAPAdminBase() (dn string, ou string) {
  adminunit, err := xml.LdifToHash("adminunit", true, ldapSearch(fmt.Sprintf("(&(objectClass=gosaAdministrativeUnit)%v)", config.UnitTagFilter),"ou"))
  if err != nil || adminunit.First("adminunit") == nil {
    util.Log(0, "ERROR! Could not find gosaAdministrativeUnit for gosaUnitTag %v under base %v: %v", config.UnitTag, config.LDAPBase, err)
    return "",""
  }
  adminunit = adminunit.First("adminunit")
  return adminunit.Text("dn"),adminunit.Text("ou")
}

// Returns the dn of the first object under config.LDAPBase that matches
// (&(objectClass=organizationalUnit)(ou=fai)).
// Logs an error and returns "" if an error occurs or no object is found.
func LDAPFAIBase() string {
  // NOTE: config.UnitTagFilter is not used here because AFAICT gosa-si-server does
  // not use it either when looking for ou=fai.
  fai, err := xml.LdifToHash("fai", true, ldapSearch(fmt.Sprintf("(&(objectClass=organizationalUnit)(ou=fai))"),"dn"))
  if err != nil || fai.First("fai") == nil {
    util.Log(0, "ERROR! Could not find ou=fai under base %v: %v", config.LDAPBase, err)
    return ""
  }
  return fai.First("fai").Text("dn")
}

const hex = "0123456789ABCDEF"

// Takes a string and escapes special characters so that the result can safely be
// used in LDAP filters.
func LDAPFilterEscape(s string) string {
  res := make([]string,0,1)
  esc := []byte{'\\',0,0}
  i0 := 0
  for i := range s {
    if s[i] < 32 || s[i] == '\\' || s[i] == '*' || s[i] == '(' || s[i] == ')' {
      if i != i0 { res = append(res, s[i0:i]) }
      i0 = i+1
      esc[2] = hex[s[i] & 0xF]
      esc[1] = hex[s[i] >> 4]
      res = append(res, string(esc))
    }
  }
  if i0 != len(s)  { res = append(res, s[i0:]) }
  return strings.Join(res,"")
}

func ldapSearch(query string, attr... string) *exec.Cmd {
  return ldapSearchBase(config.LDAPBase, query, attr...)
}

func ldapSearchBase(base string, query string, attr... string) *exec.Cmd {
  return ldapSearchBaseScope(base, "sub", query, attr...)
}

func ldapSearchBaseScope(base string, scope string, query string, attr... string) *exec.Cmd {  
  args := []string{"-x", "-LLL", "-H", config.LDAPURI, "-b", base, "-s", scope}
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

func ldapModify(ldif string) *exec.Cmd {
  args := []string{"-x", "-H", config.LDAPURI}
  args = append(args,"-D",config.LDAPAdmin,"-y",config.LDAPAdminPasswordFile)
  util.Log(2, "DEBUG! ldapmodify %v (LDIF:\n%v)",args, ldif)
  cmd := exec.Command("ldapmodify", args...)
  bufstr := bytes.NewBufferString(ldif)
  cmd.Stdin = bufstr
  return cmd
}
