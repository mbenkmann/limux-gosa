/*
Copyright (c) 2013 Landeshauptstadt München
Author: Matthias S. Benkmann

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
         "time"
         "strings"
         
         "../db"
         "../util"
         "../config"
         "../message"
       )

func error_string(st string) string {
  start := strings.Index(st,"<error_string>")
  stop  := strings.Index(st,"</error_string>")
  if start < 0 || stop < 0 { return "" }
  return st[start+14:stop]
}

// Unit tests for the package go-susi/message.
func Message_test() {
  fmt.Printf("\n==== message ===\n\n")
  
  getFJU() // clean up remnants from DB_test()

  go func(){
    defer func(){recover()}()
    message.DistributeForeignJobUpdates()
  }()
  // This is a nasty trick. Send nil over the channel to cause a
  // a panic in the message.DistributeForeignJobUpdates()
  // goroutine so that it will terminate.
  // ATM it's not actually necessary to make it terminate but it's
  // a precaution against future test code that may not expect someone
  // to be reading from this channel.
  defer func(){db.ForeignJobUpdates <- nil}()
  
  check(error_string(<-message.Peer("Broken").Ask("foo","")),"missing port in address Broken")
  check(error_string(<-message.Peer("192.168.250.128:55").Ask("foo","")),"PeerConnection.Ask: No key known for peer 192.168.250.128:55")
  check(error_string(<-message.Peer("127.0.0.1:55551").Ask("foo","bar")),"dial tcp 127.0.0.1:55551: connection refused")
  
  listen()
  defer func() {listener.Close()}()
  listen_address = config.IP + ":" + listen_port
  init_keys()
  db.ServerUpdate(hash("xml(key(%v)source(%v))", keys[0],listen_address))
  
  message.Peer(listen_address).SetGoSusi(true)
  check(message.Peer(listen_address).IsGoSusi(), true)
  message.Peer(listen_address).SetGoSusi(false)
  check(message.Peer(listen_address).IsGoSusi(), false)
  message.Peer(listen_address).SetGoSusi(true)
  check(message.Peer(listen_address).IsGoSusi(), true)
  
  check(message.Peer(listen_address).Downtime(), 0)
  t0 := time.Now()
  message.Peer(listen_address).Tell("<xml><header>Hallo</header></xml>", keys[0])
  check(wait(t0, "Hallo").XML.Text("header"), "Hallo")
  t0 = time.Now()
  message.Peer(listen_address).Tell("<xml><header>Aloha</header></xml>", "")
  check(wait(t0, "Aloha").XML.Text("header"), "Aloha")
  
  // suppress error logs
  oldlevel := util.LogLevel
  util.LogLevel = -1
  
  check(error_string(<-message.Peer(listen_address).Ask("<xml><header>gosa_query_jobdb</header></xml>", config.ModuleKey["[GOsaPackages]"])),"")
  check(error_string(<-message.Peer(listen_address).Ask("<xml><header>whatever</header></xml>", config.ModuleKey["[GOsaPackages]"])),"Communication error in Ask()")
  
  for repcount := 0; repcount < 2 ; repcount++ {
    // shut down listener
    listen_stop()
    // wait 3 seconds and check downtime
    time.Sleep(time.Duration(3+repcount)*time.Second)
    downtime := int64((message.Peer(listen_address).Downtime() + 500*time.Millisecond)/time.Second)
    check(downtime, 3+repcount)
    // verify that connection is really down
    t0 = time.Now()
    message.Peer(listen_address).Tell("<xml><header>down</header></xml>", keys[0])
    check(wait(t0, "down").XML.Text("header"), "")
    
    // restart listener
    listen()
    // send message and check if the listener receives it
    t0 = time.Now()
    message.Peer(listen_address).Tell("<xml><header>Wuseldusel</header></xml>", keys[0])
    check(wait(t0, "Wuseldusel").XML.Text("header"), "Wuseldusel")
    // check that downtime has stopped
    check(message.Peer(listen_address).Downtime(), 0)
  }
  
  // restore old log level
  time.Sleep(1*time.Second)
  util.LogLevel = oldlevel
  
  check(error_string(<-message.Peer("broken").Ask("", "")),"missing port in address broken")
  check(error_string(<-message.Peer("doesnotexist.domain:10").Ask("", "")),"lookup doesnotexist.domain: no such host")
}

