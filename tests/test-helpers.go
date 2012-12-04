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
         "fmt"
         "regexp"
         "strings"
         "io/ioutil"
         "container/list"
         "encoding/base64"
         
         "../xml"
       )

// Regexp for recognizing valid MAC addresses.
var macAddressRegexp = regexp.MustCompile("^[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}$")
// Regexp for recognizing valid <client> elements of e.g. new_server messages.
var clientRegexp = regexp.MustCompile("^[0-9]{1,3}[.][0-9]{1,3}[.][0-9]{1,3}[.][0-9]{1,3}:[0-9]+,[:xdigit:](:[:xdigit:]){5}$")
// Regexp for recognizing valid <siserver> elements
var serverRegexp = regexp.MustCompile("^[0-9]{1,3}[.][0-9]{1,3}[.][0-9]{1,3}[.][0-9]{1,3}:[0-9]+$")

type Job struct {
  Type string
  MAC string
  Plainname string
  Timestamp string
  Periodic string
}

// returns Type with the "job_" removed.
func (self *Job) Trigger() string {
  return self.Type[4:]
}

var Jobs = []Job{
{"job_trigger_action_wake","01:02:03:04:05:06","systest1","20990914131742","7_days"},
{"job_trigger_action_lock","11:22:33:44:55:6F","systest2","20770101000000","1_minutes"},
{"job_trigger_action_wake","77:66:55:44:33:2a","systest3","20660906164734","none"},
{"job_trigger_action_localboot","0f:C3:d2:Aa:11:22","www","20000209024017","none"},
}

// Returns an XML hash for the job. Optional args can be the following:
//   int/uint: the name of the enclosing element will be answerX where X is the int
//             and the <id> will be X, too.
//   IP:PORT(string) : siserver  (default is listen_address)
func (job *Job) Hash(args... interface{}) *xml.Hash {
  x := xml.NewHash("answer1")
  x.Add("plainname", job.Plainname)
  x.Add("progress", "none")
  x.Add("status", "waiting")
  x.Add("siserver", listen_address)
  x.Add("modified", "1")
  x.Add("targettag", job.MAC)
  x.Add("macaddress", job.MAC)
  x.Add("timestamp", job.Timestamp)
  x.Add("periodic", job.Periodic)
  x.Add("id", "1")
  x.Add("headertag", job.Trigger())
  x.Add("result", "none")
  
  for _, arg := range args {
    switch arg := arg.(type) {
      case int:  x.Rename(fmt.Sprintf("answer%d",arg))
                 x.First("id").SetText("%d",arg)
      case uint: x.Rename(fmt.Sprintf("answer%d",arg))
                 x.First("id").SetText("%d",arg)
      case string:
                 if serverRegexp.MatchString(arg) {
                   x.First("siserver").SetText(arg)
                 } else {
                   panic("Unknown string format in Job.Hash()")
                 }
      default: panic("Unknown type in Job.Hash()")
    }
  }
  
  xm := xml.NewHash("xml","header", "job_" + x.Text("headertag"))
  xm.Add("source", "GOSA")
  xm.Add("target", x.Text("targettag"))
  xm.Add("timestamp", x.Text("timestamp"))
  xm.Add("macaddress", x.Text("macaddress"))
  x.Add("xmlmessage", base64.StdEncoding.EncodeToString([]byte(xm.String())))
  return x
}



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

// Takes a format string like "xml(foo(%v)bar(%v))" and parameters and creates
// a corresponding xml.Hash.
func hash(format string, args... interface{}) *xml.Hash {
  format = fmt.Sprintf(format, args...)
  stack := list.New()
  output := []string{}
  a := 0
  for b := range format {
    switch format[b] {
      case '(' : tag := format[a:b]
                 stack.PushBack(tag)
                 if tag != "" {
                   output = append(output, "<" + tag + ">")
                 }
                 a = b + 1
      case ')' : output = append(output, format[a:b])
                 a = b + 1
                 tag := stack.Back().Value.(string)
                 stack.Remove(stack.Back())
                 if tag != "" {
                   output = append(output, "</" + tag + ">")
                 }
    }
  }
  
  hash, err := xml.StringToHash(strings.Join(output, ""))
  if err != nil { panic(err) }
  return hash
}

