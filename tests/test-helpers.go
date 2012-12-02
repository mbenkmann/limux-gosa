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

package tests

import (
         "io/ioutil"
       )

// creates a temporary config file and returns the path to it as well as the
// path to the containing temporary directory.
func createConfigFile(prefix, addresses string) (conffile, confdir string) {
  tempdir, err := ioutil.TempDir("", prefix)
  if err != nil { panic(err) }
  fpath := tempdir + "/server.conf"
  ioutil.WriteFile(fpath, []byte(`
[general]
log-file = `+tempdir+`/go-susi.log
pid-file = `+tempdir+`/go-susi.pid

[bus]
enabled = false
key = bus

[server]
port = 20087
max-clients = 10000
ldap-uri = ldap://127.0.0.1:20088
ldap-base = o=go-susi,c=de
ldap-admin-dn = cn=admin,o=go-susi,c=de
ldap-admin-password = password

[ClientPackages]
key = ClientPackages

[ArpHandler]
enabled = false

[GOsaPackages]
enabled = true
key = GOsaPackages

[ldap]
bind_timelimit = 5

[pam_ldap]
bind_timelimit = 5

[nss_ldap]
bind_timelimit = 5

[ServerPackages]
key = ServerPackages
dns-lookup = false
address = `+addresses+`

`), 0644)
  return fpath, tempdir
}

