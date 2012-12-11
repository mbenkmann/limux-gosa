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
         "fmt"
         "log"
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

  jobdb_test()
  serverdb_test()
}

func serverdb_test() {  
  db.ServersInit()
  
  server1 := hash("xml(header(new_server)key(foo)macaddress(00:17:31:a1:f8:19)source(172.16.2.52:20081)target(172.16.2.83:20081))")
  db.ServerUpdate(server1)
  server2 := hash("xml(key(foobar)source(172.99.9.99:20081))")
  db.ServerUpdate(server2)
  
  check(db.ServerWithMAC("00:17:31:a1:f8:19"),server1)
  check(db.ServerWithMAC("00:ff:cc:aa:ff:11"),nil)
  
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
  db.JobAddLocal(hash("job(progress(none)status(waiting)siserver(%v)macaddress(11:22:33:44:55:6F)targettag(11:22:33:44:55:6F)timestamp(81110102030405)headertag(trigger_action_lock)periodic(1_minutes))",config.ServerSourceAddress))
  
  time.Sleep(1*time.Second) // wait for plainname to be filled in
  
  fju = getFJU()
  if check(len(fju), 4) { // 2 without and 2 with plain name
    check(fju[0].First("answer1").Text("original_id"), "")
    check(fju[1].First("answer1").Text("original_id"), "")
    check(fju[0].First("answer1").Text("periodic"), "1_minutes")
    check(fju[1].First("answer1").Text("periodic"), "1_minutes")
    check(fju[2].First("answer1").Text("plainname"), "systest2")
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
  
  db.JobsRemoveLocal(xml.FilterAll, true)
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
}

func getFJU() []*xml.Hash {
  db.JobsQuery(xml.FilterNone) // make sure previous calls have been processed
  ret := []*xml.Hash{}
  for {
    select {
      case f := <- db.ForeignJobUpdates: ret = append(ret, f)
      default: return ret
    }
  }
  return ret
}

