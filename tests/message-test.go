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
         "bytes"
         "strings"
         
         "../db"
         "../xml"
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
  
  check(error_string(<-message.Peer("Broken").Ask("foo","")),"lookup Broken: no such host")
  check(error_string(<-message.Peer("192.168.250.128:55").Ask("foo","")),"PeerConnection.Ask: No key known for peer 192.168.250.128:55")
  check(hasWords(error_string(<-message.Peer("127.0.0.1:55551").Ask("foo","bar")),"connection refused"),"")
  
  listen()
  defer func() {listen_stop()}()
  listen_address = config.IP + ":" + listen_port
  init_keys()
  keys[0] = "MessageTestKey"
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
  
  // set maximum downtime before removal
  defer func(mpd time.Duration){ config.MaxPeerDowntime = mpd }(config.MaxPeerDowntime)
  config.MaxPeerDowntime = 4*time.Second
  time.Sleep(6*time.Second) // we need to wait for the old 10s pinger to expire
  // stop listener
  listen_stop()
  // ping to make handleConnection() aware that we're down
  message.Peer(listen_address).Tell("<xml><header>Ping</header></xml>", keys[0])
  time.Sleep(1*time.Second)
  message.Peer(listen_address).Tell("<xml><header>Ping</header></xml>", keys[0])
  time.Sleep(1*time.Second)
  // check we haven't been removed from serverdb, yet
  check(db.ServerKeys(listen_address), []string{keys[0]})
  time.Sleep(3*time.Second)
  // ping again. This time handleConnection() should notice we're down to long
  message.Peer(listen_address).Tell("<xml><header>Ping</header></xml>", keys[0])
  time.Sleep(1*time.Second)
  // check that we've been removed from serverdb
  check(db.ServerKeys(listen_address), []string{})
  
  // restore old log level
  time.Sleep(1*time.Second)
  util.LogLevel = oldlevel
  
  check(error_string(<-message.Peer("broken").Ask("", "")),"lookup broken: no such host")
  check(error_string(<-message.Peer("doesnotexist.domain:10").Ask("", "")),"lookup doesnotexist.domain: no such host")
  
  util.LoggersSuspend()
  oldloglevel := util.LogLevel
  defer func(){ util.LoggersRestore(); util.LogLevel = oldloglevel }()
  var buffy bytes.Buffer
  util.LoggerAdd(&buffy)
  util.LogLevel = 2 // we want all messages, including INFO! and DEBUG!
  client := xml.NewHash("xml","header","new_foreign_client")
  client.Add("source",config.ServerSourceAddress)
  client.Add("target","127.0.0.1:12345")
  client.Add("client",listen_address)
  client.Add("macaddress","11:22:33:44:55:66")
  keys[len(keys)-1] = "weissnich"
  client.Add("key",keys[len(keys)-1])
  db.ClientUpdate(client)
  
  t0 = time.Now()
  message.Client(listen_address).Tell("<xml><header>Alle meine Entchen</header></xml>", 0)
  time.Sleep(reply_timeout)
  check(get(t0), []*queueElement{})
  check(hasWords(buffy.String(),"ERROR","Cannot send message"),"")

  buffy.Reset()
  t0 = time.Now()
  message.Client(listen_address).Tell("<xml><header>Alle meine Hündchen</header></xml>", -1200*time.Millisecond)
  time.Sleep(reply_timeout)
  check(get(t0), []*queueElement{})
  check(hasWords(buffy.String(),"ERROR","Cannot send message"),"")
  
  buffy.Reset()
  t0 = time.Now()
  message.Client(listen_address).Tell("<xml><header>Alle meine Kätzchen</header></xml>", 2*time.Second)
  time.Sleep(3*time.Second)
  check(get(t0), []*queueElement{})
  check(hasWords(buffy.String(),"ERROR","Cannot send message","Attempt #1","Attempt #2"),"")
  
  buffy.Reset()
  t0 = time.Now()
  message.Client(listen_address).Tell("<xml><header>Alle meine Häschen</header></xml>", 3*time.Second)
  time.Sleep(2500*time.Millisecond)
  listen()
  time.Sleep(1*time.Second)
  x := get(t0)
  if check(len(x),1) {
    check(x[0].XML.Text("header"),"Alle meine Häschen")
    check(x[0].Key, keys[len(keys)-1])
  }
  check(hasWords(buffy.String(),"Attempt #1","Attempt #2","Attempt #3","Successfully sent message"),"")
  
  buffy.Reset()
  t0 = time.Now()  
  message.Client(listen_address).Tell("<xml><header>Alle meine Vöglein</header></xml>", -1200*time.Millisecond)
  time.Sleep(reply_timeout)
  x = get(t0)
  if check(len(x),1) {
    check(x[0].XML.Text("header"),"Alle meine Vöglein")
    check(x[0].Key, keys[len(keys)-1])
  }
  check(hasWords(buffy.String(),"Successfully sent message"),"")
  
  listen_stop()

}

