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

// Unit tests run by run-tests.go.
package tests

import (
         "os"
         "fmt"
         "log"
         "sort"
         "time"
         "bytes"
         "strings"
         "io/ioutil"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

// Unit tests for the package go-susi/db.
func DB_test() {
  fmt.Printf("\n==== db ===\n\n")

  serverdb_test()
  clientdb_test()
  systemdb_test()
  jobdb_test()
  faidb_test()
}

func faidb_test() {
  oldlogger := util.Logger
  defer func(){ util.Logger = oldlogger }()
  var buffy bytes.Buffer
  buflogger := log.New(&buffy,"",0)
  util.Logger = buflogger
  
  faiclasses,_ := os.Open("testdata/faiclasses.ldif")
  x, err := xml.LdifToHash("fai", true, faiclasses)
  check(err, nil)
  config.FAIBase = "ou=fai,ou=configs,ou=systems,o=go-susi,c=de"
  db.FAIClassesCacheInit(x)
  
  check(hasWords(buffy.String(),"ERROR!","does not belong to any release","cn=_fdgdfsgdsf,ou=scripts,ou=fai,ou=configs,ou=systems,o=go-susi,c=de","cn=_sadggfdsg,ou=templates,ou=fai,ou=configs,ou=systems,o=go-susi,c=de","cn=_fdsagdfagsf,ou=hooks,ou=fai,ou=configs,ou=systems,o=go-susi,c=de","cn=_fdsgffdsgfgfsdgdfg,ou=variables,ou=fai,ou=configs,ou=systems,o=go-susi,c=de", "cn=_fsdgffdgsfsd,ou=disk,ou=fai,ou=configs,ou=systems,o=go-susi,c=de", "cn=_fdlkgjdfksohgfhgfgfdjhhjfg,ou=profiles,ou=fai,ou=configs,ou=systems,o=go-susi,c=de"),"")
  
  fai1 := map[string]bool{}
  for fai := db.FAIClasses(xml.FilterAll).First("fai"); fai != nil; fai = fai.Next() {
    fai.RemoveFirst("tag")
    fai.RemoveFirst("timestamp")
    fai1[fai.InnerXML("class","type","fai_release","state")] = true
  }  
  
  x, err = xml.FileToHash("testdata/query_fai_release.log")
  check(err, nil)
  fai2 := map[string]bool{}
  x.RemoveFirst("header")
  x.RemoveFirst("source")
  x.RemoveFirst("target")
  x.RemoveFirst("session_id")
  for _, tag := range x.Subtags() {
    fai := x.First(tag)
    fai.RemoveFirst("timestamp")
    fai2[fai.InnerXML("class","type","fai_release","state")] = true
  }
  
  for cls := range fai2 { 
    cls1 := cls
    if _, ok := fai1[cls1]; !ok {
      cls1 = strings.Replace(cls,"<state></state>","<state>freeze</state>",1)
    }
    if _, ok := fai1[cls1]; ok { delete(fai2,cls) }
    delete(fai1,cls1)
  }
  check(fai1,map[string]bool{})
  check(fai2,map[string]bool{})
  
}

func clientdb_test() {
  db.ClientsInit()
  
  client := []string{"1.2.3.4:5,00:00:00:00:AA:01", "11.22.33.44:55,00:00:00:00:AA:02", "111.222.33.4:555,00:00:00:00:AA:03","2.3.4.5:6,00:00:00:00:AA:04", "22.33.44.55:66,00:00:00:00:AA:05", "222.33.4.55:666,00:00:00:00:AA:06" }
  
  for i := range client {
    ip  := strings.Split(client[i],",")[0]
    mac := strings.Split(client[i],",")[1]
    
    check(db.ClientWithMAC(mac), nil)
    check(db.ClientWithAddress(ip), nil)
    db.ClientUpdate(hash("xml(header(new_foreign_client)new_foreign_client()source(%v)target(%v)client(%v)macaddress(%v))", listen_address, config.ServerSourceAddress, ip,mac))
    if c := db.ClientWithMAC(mac); check(c!=nil, true) {
      check(c.Text("client"), ip)
      check(c.Text("macaddress"), mac)
      check(c.Text("key"),"")
      check(db.ClientKeys(ip), []string{})
      check(db.ClientKeys(strings.Split(ip,":")[0]), []string{})
    }
    check(db.ClientWithMAC(mac), db.ClientWithAddress(ip))
  }
  check(len(db.ClientKeysForAllClients()),0)
  
  for i := range client {
    ip  := strings.Split(client[i],",")[0]
    mac := strings.Split(client[i],",")[1]
    
    // change key to "key_for_"+mac
    db.ClientUpdate(hash("xml(key(key_for_%v)header(new_foreign_client)new_foreign_client()source(%v)target(%v)client(%v)macaddress(%v))", mac,listen_address, config.ServerSourceAddress, ip,mac))
    if c := db.ClientWithMAC(mac); check(c!=nil, true) {
      check(c.Text("client"), ip)
      check(c.Text("macaddress"), mac)
      check(c.Text("key"),"key_for_"+mac)
      check(db.ClientKeys(ip), []string{"key_for_"+mac})
      check(db.ClientKeys(strings.Split(ip,":")[0]), []string{"key_for_"+mac})
    }
  }
  check(len(db.ClientKeysForAllClients()),len(client))
  
  for i := range client {
    ip  := strings.Split(client[i],",")[0]
    mac := strings.Split(client[i],",")[1]
    
    // change key to "new_key_for_"+mac
    db.ClientUpdate(hash("xml(key(new_key_for_%v)header(new_foreign_client)new_foreign_client()source(%v)target(%v)client(%v)macaddress(%v))", mac,listen_address, config.ServerSourceAddress, ip,mac))
    if c := db.ClientWithMAC(mac); check(c!=nil, true) {
      check(c.Text("client"), ip)
      check(c.Text("macaddress"), mac)
      check(c.Get("key"),[]string{"new_key_for_"+mac,"key_for_"+mac})
      check(db.ClientKeys(ip), c.Get("key"))
      check(db.ClientKeys(strings.Split(ip,":")[0]), c.Get("key"))
    }
  }
  check(len(db.ClientKeysForAllClients()),2*len(client))
  
  allkeys := []string{}
  for i := range client {
    ip  := strings.Split(client[i],",")[0]
    mac := strings.Split(client[i],",")[1]
    
    // change key to "3rd_key_for_"+mac, which shifts out the "key_for_"+mac
    db.ClientUpdate(hash("xml(key(3rd_key_for_%v)header(new_foreign_client)new_foreign_client()source(%v)target(%v)client(%v)macaddress(%v))", mac,listen_address, config.ServerSourceAddress, ip,mac))
    if c := db.ClientWithMAC(mac); check(c!=nil, true) {
      check(c.Text("client"), ip)
      check(c.Text("macaddress"), mac)
      check(c.Get("key"),[]string{"3rd_key_for_"+mac,"new_key_for_"+mac})
      check(db.ClientKeys(ip), c.Get("key"))
      check(db.ClientKeys(strings.Split(ip,":")[0]), c.Get("key"))
      allkeys = append(allkeys, c.Get("key")...)
    }
  }
  check(len(db.ClientKeysForAllClients()),2*len(client))
  
  sort.Strings(allkeys)
  allkeys2 := db.ClientKeysForAllClients()
  sort.Strings(allkeys2)
  check(allkeys2, allkeys)
  
  addr0  := strings.Split(client[0],",")[0]
  ip0 := strings.Split(addr0,":")[0]
  check(ip0 != addr0, true)
  mac0 := strings.Split(client[0],",")[1]
  addr1  := strings.Split(client[1],",")[0]
  ip1 := strings.Split(addr1,":")[0]
  check(ip1 != addr1, true)
  mac1 := strings.Split(client[1],",")[1]
  client0 := db.ClientWithMAC(mac0)
  client0.First("macaddress").SetText(mac1) // client0 now has MAC 1 with IP 0
  client0.RemoveFirst("key")
  client0.FirstOrAdd("key").SetText("foobar")
  check(db.ClientWithMAC(mac0) != nil, true)
  check(db.ClientWithAddress(addr1) != nil, true)
  check(db.ClientWithAddress(ip1) != nil, true)
  check(len(db.ClientKeysForAllClients()), len(allkeys))
  db.ClientUpdate(client0) // replaces MAC 1 and IP 0 entry
  check(db.ClientWithMAC(mac0), nil)
  check(db.ClientWithAddress(addr1), nil)
  check(db.ClientWithAddress(ip1), nil)
  check(db.ClientWithAddress(addr0), db.ClientWithMAC(mac1))
  check(db.ClientWithAddress(ip0), db.ClientWithMAC(mac1))
  check(len(db.ClientKeysForAllClients()), 2*(len(client)-1))
}

func serverdb_test() {  
  db.ServersInit()
  
  server1 := hash("xml(header(new_server)key(foo)macaddress(00:17:31:a1:f8:19)source(172.16.2.52:20081)target(172.16.2.83:20081))")
  db.ServerUpdate(server1)
  check(db.ServerKeys("172.16.2.52:20081"),[]string{"foo"})
  check(db.ServerRemove("172.16.2.52:20081"), server1)
  check(db.ServerKeys("172.16.2.52:20081"),[]string{})
  check(db.ServerRemove("172.16.2.52:20081"), nil)
  db.ServerUpdate(server1)
  server2 := hash("xml(key(foobar)source(172.99.9.99:20081))")
  db.ServerUpdate(server2)
  
  check(db.ServerWithMAC("00:17:31:a1:f8:19"),server1)
  check(db.ServerWithMAC("00:ff:cc:aa:ff:11"),nil)
}

func systemdb_test() {
  check(db.SystemPlainnameForMAC(Jobs[0].MAC), Jobs[0].Plainname)
  check(db.SystemPlainnameForMAC(Jobs[1].MAC), Jobs[1].Plainname)
  check(db.SystemPlainnameForMAC(Jobs[2].MAC), Jobs[2].Plainname)
  check(db.SystemPlainnameForMAC(Jobs[3].MAC), Jobs[3].Plainname)
  oldlogger := util.Logger
  defer func(){ util.Logger = oldlogger }()
  var buffy bytes.Buffer
  buflogger := log.New(&buffy,"",0)
  util.Logger = buflogger
  check(db.SystemPlainnameForMAC("99:99:00:99:11:00"), "none")
  check(strings.Index(buffy.String(),"ERROR")>0,true)
  
  check(db.SystemFullyQualifiedNameForMAC(Jobs[0].MAC), "none")
  check(db.SystemFullyQualifiedNameForMAC(Jobs[3].MAC), "www.mit.edu")
  
  buffy.Reset()
  check(db.SystemFullyQualifiedNameForMAC("99:99:00:99:11:00"), "none")
  check(strings.Index(buffy.String(),"ERROR")>0,true)
  
  check(db.SystemFullyQualifiedNameForMAC("00:C4:d2:10:10:20"), "wikipedia-lb.esams.wikimedia.org")
  
  check(db.SystemCommonNameForMAC("foobar"),"")
  check(db.SystemCommonNameForMAC("11:22:33:33:22:11"),"")
  check(db.SystemCommonNameForMAC(Jobs[0].MAC), "systest1")
  check(db.SystemCommonNameForMAC(Jobs[1].MAC), "systest2")
  check(db.SystemCommonNameForMAC(Jobs[2].MAC), "systest3")
  check(db.SystemCommonNameForMAC(Jobs[3].MAC), "www.mit.edu")
  check(db.SystemCommonNameForMAC("00:C4:d2:10:10:20"), "wikipedia-lb")
  
  check(db.SystemIPAddressForName("localhost"), config.IP)
  buffy.Reset()
  check(db.SystemIPAddressForName("sdfjnsdjfbsdfjb32"), "none")
  check(strings.Index(buffy.String(),"ERROR")>0,true)
  check(db.SystemIPAddressForName(config.Hostname), config.IP)
  check(db.SystemIPAddressForName("www.mit.edu"), "18.9.22.169")
  
  check(db.SystemNameForIPAddress("18.9.22.169"), "www.mit.edu")
  
  check(db.SystemMACForName("systest1.foo.bar"), Jobs[0].MAC)
  check(db.SystemMACForName("systest1"), Jobs[0].MAC)
  check(db.SystemMACForName("rotz"), "none")
  
  check(len(db.SystemDomainsKnown())>0, true)
  for _, dom := range db.SystemDomainsKnown() {
    if check(dom != "", true) {
      check(dom[0], '.')
      check(dom[len(dom)-1] != '.', true)
    }
  }
  
  check(len(db.SystemNetworksKnown())>0, true)
  
  check(db.SystemIsWorkstation("dontexist"), false)
  check(db.SystemIsWorkstation(db.SystemMACForName("www.mit.edu")), false)
  check(db.SystemIsWorkstation(db.SystemMACForName("wikipedia-lb")), false)
  check(db.SystemIsWorkstation(db.SystemMACForName("systest1")), true)
  check(db.SystemIsWorkstation(db.SystemMACForName("systest2")), true)
  check(db.SystemIsWorkstation(db.SystemMACForName("systest3")), true)
  
  check(db.SystemGetState(Jobs[0].MAC, "objectclass"),"GOhard␞gotoWorkstation␞FAIobject␞gosaAdministrativeUnitTag")
  
  data, err := db.SystemGetAllDataForMAC("no-mac", true)
  check(data, nil)
  check(err, "Could not find system with MAC no-mac")
  
  ldapUri := config.LDAPURI
  config.LDAPURI = "broken"
  data, err = db.SystemGetAllDataForMAC(db.SystemMACForName("systest1"), true)
  check(data, nil)
  check(err, "Could not parse LDAP URI(s)=broken (3)\n")
  config.LDAPURI = ldapUri
  
  data, err = db.SystemGetAllDataForMAC(db.SystemMACForName("systest1"), true)
  check(err, nil)
  check(data.Text("dn"), "cn=systest1,ou=workstations,ou=systems,o=go-susi,c=de")
  check(data.Text("macaddress"), "01:02:03:04:05:06")
  check(data.Text("cn"), "systest1")
  ocls := data.Get("objectclass")
  sort.Strings(ocls)
  check(ocls, []string{"FAIobject","GOhard","gosaAdministrativeUnitTag","gotoWorkstation"})
  
  // check db.SystemSetStateMulti()
  systest1,_ := db.SystemGetAllDataForMAC(db.SystemMACForName("systest1"), true)
  check(db.SystemGetState(systest1.Text("macaddress"), "gotontpserver"), "cool.ntp.org")
  check(db.SystemSetStateMulti(systest1.Text("macaddress"), "gotontpserver", []string{}), nil)
  check(db.SystemGetState(systest1.Text("macaddress"), "gotoNtpServer"), "")
  check(db.SystemSetStateMulti(systest1.Text("macaddress"), "gOtontpserver", []string{"ntp1.example.com","ntp2.example.com","ntp3.example.com"}), nil)
  check(db.SystemGetState(systest1.Text("macaddress"), "gotoNtpServer"), "ntp1.example.com␞ntp2.example.com␞ntp3.example.com")
  
  // restore old gotoNtpServer
  db.SystemSetState(systest1.Text("macaddress"), "gotoNtpServer", "cool.ntp.org")
  data,_ = db.SystemGetAllDataForMAC(systest1.Text("macaddress"), true)
  check(data, systest1)
  
  // Check that changing "dn" fails (it's not a real attribute)
  err = db.SystemSetStateMulti(systest1.Text("macaddress"), "dn", []string{"broken"})
  check(err != nil, true)
  
  ogmember1,_ := db.SystemGetAllDataForMAC("fe:ce:5f:ec:e5:00", true) // with groups
  check(ogmember1.Text("gotontpserver"),"ntp01.example.com␞ntp02.example.com")
  check(ogmember1.Text("gotoldapserver"),"1:ldap01.tvc.example.com:ldap://ldap01.tvc.example.com/o=go-susi,c=de␞2:ldap02.tvc.example.com:ldap://ldap02.tvc.example.com:389/o=go-susi,c=de")
  ogmember1,_ = db.SystemGetAllDataForMAC("fe:ce:5f:ec:e5:00", false) // without groups
  check(ogmember1.First("gotontpserver"),nil)
  check(ogmember1.First("gotoldapserver"),nil)
  
  groupsOf := func(dn string) []string {
    r := []string{}
    for g := db.SystemGetGroupsWithMember(dn).First("xml"); g!=nil; g=g.Next() {
      r = append(r, g.Text("cn"))
    }
    sort.Strings(r)
    return r
  }
  
  ogmember1_dn := "cn=ogmember1,ou=workstations,ou=systems,o=go-susi,c=de"
  groups := db.SystemGetGroupsWithMember(ogmember1_dn)
  check(groups.Name(), "systemdb")
  check(groups.Subtags(), []string{"xml"})
  check(groupsOf(ogmember1_dn), []string{"Objektgruppe"})
  
  notebooks := db.SystemGetGroupsWithMember("cn=notebook-template,ou=workstations,ou=systems,o=go-susi,c=de")
  db.SystemAddToGroups(ogmember1_dn, notebooks)
  check(groupsOf(ogmember1_dn), []string{"Notebooks","Objektgruppe"})
  
  desktops := db.SystemGetGroupsWithMember("cn=desktop-template,ou=workstations,ou=systems,o=go-susi,c=de")
  groups.AddClone(desktops.First("xml"))
  check(groups.First("xml").Next().Text("cn"), "Desktops")
  db.SystemAddToGroups(ogmember1_dn, groups)
  check(groupsOf(ogmember1_dn), []string{"Desktops","Notebooks","Objektgruppe"})
  
  groups = desktops.Clone()
  groups.AddClone(notebooks.First("xml"))
  db.SystemRemoveFromGroups(ogmember1_dn, groups)
  check(groupsOf(ogmember1_dn), []string{"Objektgruppe"})
  
  sys := xml.NewHash("xml")
  dfl := xml.NewHash("xml")
  db.SystemFillInMissingData(sys, dfl)
  check(sys, "<xml></xml>")
  dfl.Add("dn")
  db.SystemFillInMissingData(sys, dfl)
  check(sys, "<xml></xml>")
  dfl.First("dn").SetText("bullshit")
  db.SystemFillInMissingData(sys, dfl)
  check(sys, "<xml></xml>")
  dfl.First("dn").SetText("cn=bull,ou=shit")
  db.SystemFillInMissingData(sys, dfl)
  check(sys, "<xml></xml>")
  sys.Add("cn")
  db.SystemFillInMissingData(sys, dfl)
  check(sys, "<xml><cn></cn></xml>")
  sys.First("cn").SetText("horse")
  db.SystemFillInMissingData(sys, dfl)
  check(sys, "<xml><cn>horse</cn><dn>cn=horse,ou=shit</dn></xml>")
  dfl.Add("objectclass","gosaAdministrativeUnitTag")
  dfl.Add("gosaunittag","4567")
  db.SystemFillInMissingData(sys, dfl)
  check(sys.Text("objectclass"), "gosaAdministrativeUnitTag")
  check(sys.Text("gosaunittag"), "4567")
  dfl.First("gosaunittag").SetText("999999")
  db.SystemFillInMissingData(sys, dfl)
  check(sys.Text("objectclass"), "gosaAdministrativeUnitTag")
  check(sys.Text("gosaunittag"), "4567")
  
  sys = xml.NewHash("xml","dn","")
  dfl.RemoveFirst("gosaunittag")
  dfl.RemoveFirst("objectclass")
  for _, bad := range []string{"cn","member","gosagroupobjects","description","gocomment"} {
    dfl.Add(bad, "do not copy")
    db.SystemFillInMissingData(sys, dfl)
    check(sys, "<xml><dn></dn></xml>")
  }
  dfl.Add("foo", "bar")
  dfl.Add("gotomodules", "pups")
  db.SystemFillInMissingData(sys, dfl)
  check(sys, "<xml><dn></dn><foo>bar</foo><gotomodules>pups</gotomodules></xml>")
  
  // object groups support in db.SystemGetState()
  check(db.SystemGetState(db.SystemMACForName("ogmember1"), "gotoNtpserver"),"ntp01.example.com␞ntp02.example.com")
  check(db.SystemGetState(db.SystemMACForName("ogmember1"), "member"),"") // "member" should not be inherited from object groups
  check(db.SystemGetState(db.SystemMACForName("ogmember1"), "doesnotexist"),"")
  
  // db.SystemGetTemplatesFor()
  notebook_template := xml.NewHash("systemdb")
  n,_ := db.SystemGetAllDataForMAC("00:00:11:11:00:00",false)
  notebook_template.AddWithOwnership(n)
  desktop_template  := xml.NewHash("systemdb")
  d,_ := db.SystemGetAllDataForMAC("00:00:22:22:00:00",false)
  desktop_template.AddWithOwnership(d)
  check(db.SystemGetTemplatesFor(xml.NewHash("xml")), "<systemdb></systemdb>")
  check(db.SystemGetTemplatesFor(hash("xml(cn(note))")), "<systemdb></systemdb>")
  check(db.SystemGetTemplatesFor(hash("xml(cn(tele))")), "<systemdb></systemdb>")
  check(db.SystemGetTemplatesFor(hash("xml(cn(note101))")), "<systemdb></systemdb>")
  check(db.SystemGetTemplatesFor(hash("xml(cn(n202))")), "<systemdb></systemdb>")
  check(db.SystemGetTemplatesFor(hash("xml(cn(n2022))")), "<systemdb></systemdb>")
  check(db.SystemGetTemplatesFor(hash("xml(cn(note2))")), notebook_template)
  check(db.SystemGetTemplatesFor(hash("xml(cn(tele2))")), notebook_template)
  check(db.SystemGetTemplatesFor(hash("xml(ghcputype(GenuineIntel / Intel))")), desktop_template)
  // specificity sorting test 1: notebook-template is better match
  templates := db.SystemGetTemplatesFor(hash("xml(cn(note2)ghcputype(GenuineIntel / Intel))"))
  if check(templates.First("xml") != nil, true) {
    check(templates.First("xml"), notebook_template.First("xml"))
    check(templates.First("xml").Next(), desktop_template.First("xml"))
  }
  // specificity sorting test 2: desktop-template is better match
  templates = db.SystemGetTemplatesFor(hash("xml(cn(note2)w(w)x(x)y(y)z(z))"))
  if check(templates.First("xml") != nil, true) {
    check(templates.First("xml"), desktop_template.First("xml"))
    check(templates.First("xml").Next(), notebook_template.First("xml"))
  }
  
  check(db.SystemReplace(nil,nil), nil)
  check(hasWords(db.SystemReplace(xml.NewHash("xml","dn","cn=foo,ou=systems,o=go-susi,c=de"),nil),"Error","No such object"), "")
  syssy := xml.NewHash("xml","dn","cn=syssy,ou=systems,o=go-susi,c=de")
  syssy.Add("cn","syssy")
  syssy.Add("objectclass","GOhard")
  syssy.Add("gotoldapserver","Heike")
  syssy.Add("gotoldapserver","Julia")
  syssy.Add("gotontpserver","Bettina")
  syssy.Add("gotontpserver","Andrea")
  syssy.Add("gotoxmousetype","Langschwanzratte")
  syssy.Add("gotoxdriver")
  syssy.Add("macaddress","KeineAhnungWieDieHeisst")
  check(db.SystemReplace(nil,syssy), nil)
  check(hasWords(db.SystemReplace(nil,syssy),"Error","Already exists"), "")
  syssy_new := syssy.Clone()
  syssy_new.First("cn").SetText("judith")
  syssy_new.First("gotoxmousetype").SetText("")
  syssy_new.Add("gotomodules","Kirsten")
  syssy_new.Add("gotomodules","Jutta")
  syssy_new.RemoveFirst("gotoldapserver")
  syssy_new.RemoveFirst("gotoldapserver")
  syssy_new.First("gotontpserver").SetText("Margot")
  syssy_new.Add("gotontpserver", "Simona")
  syssy_compare := syssy_new.Clone()
  syssy_compare.First("dn").SetText("cn=judith,ou=systems,o=go-susi,c=de")
  syssy_compare.RemoveFirst("gotoxdriver") // empty elements are removed
  syssy_compare.RemoveFirst("gotoxmousetype") // empty elements are removed
  check(db.SystemReplace(syssy,syssy_new), nil) //fixes up dn, too
  check(strings.SplitN(syssy_new.Text("dn"),",",2)[0],"cn=judith")
  syssy_compare2,_ := db.SystemGetAllDataForMAC("KeineAhnungWieDieHeisst",false)
  check(syssy_compare2,syssy_compare)
  syssy_new.First("dn").SetText("cn=judith,ou=workstations,ou=systems,o=go-susi,c=de")
  check(db.SystemReplace(syssy_compare,syssy_new), nil)
  syssy_compare2,_ = db.SystemGetAllDataForMAC("KeineAhnungWieDieHeisst",false)
  syssy_new.RemoveFirst("gotoxdriver") // empty elements are removed
  syssy_new.RemoveFirst("gotoxmousetype") // empty elements are removed
  check(syssy_compare2,syssy_new)
  check(db.SystemReplace(syssy_new,nil), nil)
  _,err = db.SystemGetAllDataForMAC("KeineAhnungWieDieHeisst",false)
  check(hasWords(err,"Could not find","KeineAhnungWieDieHeisst"), "")
}

func jobdb_test() {
  check(db.JobGUID("0.0.0.0:0", 0), "00")
  check(db.JobGUID("255.255.255.255:65535", 18446744073709551615), "18446744073709551615281474976710655")
  check(db.JobGUID("1.2.3.4:20081", 18446744073709551615), "1844674407370955161586247305576961")

  data, err := ioutil.ReadFile("testdata/jobdb-test.xml")
  if err != nil { panic(err) }
  data = []byte(strings.Replace(strings.Join(strings.Fields(string(data)),""),"LOCAL",config.ServerSourceAddress,-1))
  err = ioutil.WriteFile(config.JobDBPath, data, 0644)
  if err != nil { panic(err) }
  
  db.JobsInit()
  
  // wait a little for jobs with timestamp in the past to go to status "processing"
  time.Sleep(1*time.Second)
  
  jobs := db.JobsQuery(xml.FilterAll)
  check(len(jobs.Get("job")),1)
  check(jobs.First("job").Text("siserver"), config.ServerSourceAddress)
  check(jobs.First("job").Text("status"), "processing")
  fju := getFJU()
  if check(len(fju),1) {
    check(fju[0].First("answer1").Text("status"), "processing")
    check(fju[0].First("answer1").Text("id"), "4")
  }
  
  if check(db.PendingActions.Count(), 1) {
    check(db.PendingActions.Next().(*xml.Hash).Text("headertag"), "trigger_action_lock")
  }
  
  db.JobsAddOrModifyForeign(xml.FilterNone, hash("xml(progress(none)status(waiting)siserver(1.2.3.4:20081)macaddress(11:22:33:44:55:6F)targettag(11:22:33:44:55:6F)timestamp(11110102030405)id(2)headertag(trigger_action_halt))"))
  time.Sleep(1*time.Second) // wait for plainname to be asynchronously updated
  jobs = db.JobsQuery(xml.FilterSimple("siserver","1.2.3.4:20081"))
  if check(jobs.Subtags(), []string{"job"}) {
    job := jobs.First("job")
    check(job.Next(), nil)
    check(job.Text("plainname"), "systest2")
    check(job.Text("id"), "5")
    check(job.Text("original_id"), "2")
    check(job.Text("status"), "waiting")
    check(job.Text("macaddress"), "11:22:33:44:55:6F")
  }
  
  db.JobsAddOrModifyForeign(xml.FilterNone, hash("xml(progress(none)status(waiting)siserver(1.2.3.4:20081)macaddress(11:22:33:44:55:6F)targettag(11:22:33:44:55:6F)timestamp(11110102030405)id(3)headertag(trigger_action_lock))"))
  db.JobsAddOrModifyForeign(xml.FilterSimple("siserver","1.2.3.4:20081"), hash("xml(timestamp(99991111222222))"))
  jobs = db.JobsQuery(xml.FilterSimple("siserver","1.2.3.4:20081"))
  if check(jobs.First("job")!=nil, true) {
    if check(jobs.First("job").Next()!=nil, true) {
      check(jobs.First("job").Text("timestamp"), "99991111222222")
      check(jobs.First("job").Next().Text("timestamp"), "99991111222222")
      check(jobs.First("job").Next().Text("headertag")!=jobs.First("job").Text("headertag"), true)
    }
  }
  
  db.JobsAddOrModifyForeign(xml.FilterSimple("siserver","1.2.3.4:20081"), hash("job(status(done))"))
  check(db.JobsQuery(xml.FilterSimple("siserver","1.2.3.4:20081")), hash("jobdb()"))
  
  check(len(getFJU()),0)
  
  db.JobsAddOrModifyForeign(xml.FilterNone, hash("xml(progress(none)status(waiting)siserver(1.2.3.4:20081)macaddress(11:22:33:44:55:6F)targettag(11:22:33:44:55:6F)timestamp(11110102030405)id(3)headertag(trigger_action_lock))"))
  db.JobsAddOrModifyForeign(xml.FilterNone, hash("xml(progress(none)status(waiting)siserver(1.2.3.4:20081)macaddress(11:22:33:44:55:6F)targettag(11:22:33:44:55:6F)timestamp(11110102030405)id(2)headertag(trigger_action_halt))"))
  db.JobsAddOrModifyForeign(xml.FilterNone, hash("xml(progress(none)status(waiting)siserver(7.7.7.7:20081)macaddress(11:22:33:44:55:6F)targettag(11:22:33:44:55:6F)timestamp(11110102030405)id(2)headertag(trigger_action_halt))"))
  db.JobsForwardModifyRequest(xml.FilterNot(xml.FilterSimple("siserver",config.ServerSourceAddress)), hash("job(status(done))"))
  
  fju = getFJU()
  if check(len(fju), 2) {
    if fju[1].Text("target") == "1.2.3.4:20081" { fju[0],fju[1] = fju[1],fju[0] }
    check(fju[0].First("answer1") != nil, true)
    check(fju[0].First("answer2") != nil, true)
    check(fju[0].First("answer1").Text("status"), "done")
    check(fju[0].First("answer2").Text("status"), "done")
    
    check(fju[1].First("answer1") != nil, true)
    check(fju[1].First("answer1").Text("status"), "done")
  }
  
  db.JobsRemoveForeign(xml.FilterAll)
  check(db.JobsQuery(xml.FilterAll), hash("jobdb()"))
  
  db.JobAddLocal(hash("job(progress(none)status(waiting)siserver(%v)macaddress(11:22:33:44:55:6F)targettag(11:22:33:44:55:6F)timestamp(91110102030405)headertag(trigger_action_lock)periodic(1_minutes))",config.ServerSourceAddress))
  time.Sleep(1*time.Second) // wait for plainname to be filled in
  db.JobAddLocal(hash("job(progress(none)status(waiting)siserver(%v)macaddress(11:22:33:44:55:6F)targettag(11:22:33:44:55:6F)timestamp(81110102030405)headertag(trigger_action_lock)periodic(1_minutes))",config.ServerSourceAddress))
  time.Sleep(1*time.Second) // wait for plainname to be filled in
  
  fju = getFJU()
  if check(len(fju), 4) { // 2 without and 2 with plain name
    check(fju[0].First("answer1").Text("original_id"), "")
    check(fju[2].First("answer1").Text("original_id"), "")
    check(fju[0].First("answer1").Text("periodic"), "1_minutes")
    check(fju[2].First("answer1").Text("periodic"), "1_minutes")
    check(fju[1].First("answer1").Text("plainname"), "systest2")
    check(fju[3].First("answer1").Text("plainname"), "systest2")
  }
  
  jobs = db.JobsQuery(xml.FilterAll)
  job := jobs.First("job")
  check(job.Text("plainname"), "systest2")
  check(job.Text("id"), job.Text("original_id"))
  check(job.Text("timestamp"), "91110102030405")
  job = job.Next()
  check(job.Text("plainname"), "systest2")
  check(job.Text("id"), job.Text("original_id"))
  check(job.Text("timestamp"), "81110102030405")
  
  db.JobsModifyLocal(xml.FilterAll, hash("job(status(done)periodic())"))
  fju = getFJU()
  if check(len(fju), 1) {
    if check(fju[0].First("answer1")!=nil,true) {
      check(fju[0].First("answer1").Text("original_id"), "")
      check(fju[0].First("answer1").Text("periodic"), "none")
      check(fju[0].First("answer1").Text("status"), "done")
    }
    if check(fju[0].First("answer2")!=nil,true) {
      check(fju[0].First("answer2").Text("periodic"), "none")
      check(fju[0].First("answer2").Text("status"), "done")
      check(fju[0].First("answer2").Text("original_id"), "")
    }
  }
  check(db.JobsQuery(xml.FilterAll), hash("jobdb()"))
  if check(db.PendingActions.Count(), 2) {
    job = db.PendingActions.Next().(*xml.Hash)
    check(job.Text("original_id"), "")
    check(job.Text("periodic"), "none")
    check(job.Text("status"), "done")
    job = db.PendingActions.Next().(*xml.Hash)
    check(job.Text("original_id"), "")
    check(job.Text("periodic"), "none")
    check(job.Text("status"), "done")
  }
  
  db.JobAddLocal(hash("job(progress(none)status(waiting)siserver(%v)macaddress(11:22:33:44:55:6F)targettag(11:22:33:44:55:6F)timestamp(91110102030405)headertag(trigger_action_lock)periodic(1_minutes))",config.ServerSourceAddress))
  db.JobAddLocal(hash("job(progress(none)status(waiting)siserver(%v)macaddress(11:22:33:44:55:6F)targettag(11:22:33:44:55:6F)timestamp(81110102030405)headertag(trigger_action_lock)periodic(1_minutes))",config.ServerSourceAddress))
  
  time.Sleep(1*time.Second) // wait for plainname to be filled in
  getFJU()
  
  db.JobsModifyLocal(xml.FilterNone, hash("job(status(error))"))
  check(getFJU(), []*xml.Hash{})
  
  db.JobsModifyLocal(xml.FilterAll, hash("job(timestamp(10001011000000))"))
  time.Sleep(200*time.Millisecond) // wait for job to enter processing state
  fju = getFJU()
  if check(len(fju), 2) { // 1 for the timestamp change, 1 for waiting=>processing
    check(fju[0].First("answer1").Text("periodic"), "1_minutes")
    check(fju[0].First("answer1").Text("status"), "waiting")
    check(fju[0].First("answer1").Text("timestamp"), "10001011000000")
    check(fju[1].First("answer1").Text("periodic"), "1_minutes")
    check(fju[1].First("answer1").Text("status"), "processing")
    check(fju[1].First("answer1").Text("timestamp"), "10001011000000")
  }
  
  if check(db.PendingActions.Count(), 2) {
    job = db.PendingActions.Next().(*xml.Hash)
    check(job.Text("timestamp"), "10001011000000")
    check(job.Text("periodic"), "1_minutes")
    check(job.Text("status"), "processing")
    job = db.PendingActions.Next().(*xml.Hash)
    check(job.Text("timestamp"), "10001011000000")
    check(job.Text("periodic"), "1_minutes")
    check(job.Text("status"), "processing")
  }
  
  jobs = db.JobsQuery(xml.FilterAll)
  job = jobs.First("job")
  check(job.Text("plainname"), "systest2")
  check(job.Text("id"), job.Text("original_id"))
  check(job.Text("timestamp"), "10001011000000")
  check(job.Text("periodic"), "1_minutes")
  check(job.Text("status"), "processing")
  check(job.Text("headertag"), "trigger_action_lock")
  job = job.Next()
  check(job.Text("plainname"), "systest2")
  check(job.Text("id"), job.Text("original_id"))
  check(job.Text("timestamp"), "10001011000000")
  check(job.Text("periodic"), "1_minutes")
  check(job.Text("status"), "processing")
  check(job.Text("headertag"), "trigger_action_lock")
  
}


