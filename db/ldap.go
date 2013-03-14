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
         "os/exec"
         "encoding/base64"
         
         "../xml"
         "../util"
         "../config"
       )

// Returns the dn and ou of the first object under config.LDAPBase that matches
// (&(objectClass=gosaAdministrativeUnit)(gosaUnitTag=<config.UnitTag>)).
// Logs and error and returns "","" if an error occurs or no object is found.
func LDAPAdminBase() (dn string, ou string) {
  adminunit, err := xml.LdifToHash("adminunit", true, ldapSearch(fmt.Sprintf("(&(objectClass=gosaAdministrativeUnit)%v)", config.UnitTagFilter),"ou"))
  if err != nil || adminunit.First("adminunit") == nil {
    util.Log(0, "ERROR! Could not find gosaAdministrativeUnit for gosaUnitTag %v under base %v: %v", config.UnitTag, config.LDAPBase, err)
    return "",""
  }
  adminunit = adminunit.First("adminunit")
  return adminunit.Text("dn"),adminunit.Text("ou")
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

func ldapModify(ldif string) *exec.Cmd {
  args := []string{"-x", "-H", config.LDAPURI}
  args = append(args,"-D",config.LDAPAdmin,"-y",config.LDAPAdminPasswordFile)
  util.Log(2, "DEBUG! ldapmodify %v (LDIF:\n%v)",args, ldif)
  cmd := exec.Command("ldapmodify", args...)
  bufstr := bytes.NewBufferString(ldif)
  cmd.Stdin = bufstr
  return cmd
}
