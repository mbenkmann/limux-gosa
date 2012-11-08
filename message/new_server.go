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

package message

import (
         "../db"
         "../xml"
         "../util"
         "../config"
       )

// Sends a new_server message to all known peer servers.
func Broadcast_new_server() {
  for _, server := range db.ServerAddresses() {
    srv := server
    go util.WithPanicHandler(func(){ Send_new_server("new_server", srv) })
  }
}

// Sends a "new_server" or "confirm_new_server" message to target.
//  header: "new_server" or "confirm_new_server"
//  target: e.g. 1.2.3.4:20081
func Send_new_server(header string, target string) {
  keys := db.ServerKeys(target)
  if len(keys) == 0 {
    util.Log(0, "ERROR! Send_new_server: No key known for %v", target)
    return
  }
  
  msg := xml.NewHash("xml","header", header)
  msg.Add(header)
  msg.Add("source", config.ServerSourceAddress)
  msg.Add("macaddress", config.MAC)
  msg.Add("loaded_modules", "gosaTriggered", "siTriggered", 
                            "clMessages", "server_server_com", 
                            "databases", "logHandling", 
                            "goSusi")
  msg.Add("key", keys[0])
  msg.Add("target", target)
  
  serverpackageskey := config.ModuleKey["[ServerPackages]"]

  util.Log(2, "DEBUG! Sending %v to %v encrypted with key %v", header, target, serverpackageskey)
  Peer(target).Tell(msg.String(), serverpackageskey)
}


// Handles the message "new_server".
//  xmlmsg: the decrypted and parsed message
func new_server(xmlmsg *xml.Hash) {
  setGoSusi(xmlmsg)
  db.ServerUpdate(xmlmsg)
  server := xmlmsg.Text("source")
  go util.WithPanicHandler(func() {
    Send_new_server("confirm_new_server", server)
    Peer(server).SyncAll()
  })
  return
}

// Handles the message "confirm_new_server".
//  xmlmsg: the decrypted and parsed message
func confirm_new_server(xmlmsg *xml.Hash) {
  setGoSusi(xmlmsg)
  Peer(xmlmsg.Text("source")).SyncAll()
  db.ServerUpdate(xmlmsg)
}

// Takes the new_server/confirm_new_server message xmlmsg and if
// it contains <loaded_modules>goSusi</loaded_modules>, marks the
// peer identified by the message's <source> as a go-susi. Otherwise
// it is marked as a non-go-susi. This mark affects whether the more
// efficient and more reliable server-server communication protocol
// will be used to talk to that peer or if the inferior protocol for
// compatibility with gosa-si will be used.
// For instance after re-establishing a lost connection to a non-go-susi
// server, an active gosa_query_jobdb request will be made to get an
// up-to-date copy of its jobs list. This is not required when the peer
// is a go-susi because go-susi automatically sends its jobs when necessary.
func setGoSusi(xmlmsg *xml.Hash) {
  server := xmlmsg.Text("source")
  gosusi := false
  for mod := xmlmsg.First("loaded_modules"); mod != nil; mod = mod.Next() {
    if mod.Text() == "goSusi" {
      gosusi = true
      break
    }
  }
  Peer(server).SetGoSusi(gosusi)
}

