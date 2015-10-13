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
         "strings"
         
         "../db"
         "../xml"
         "github.com/mbenkmann/golib/util"
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
  msg.Add("loaded_modules", "gosaTriggered")
  msg.Add("loaded_modules", "siTriggered")
  msg.Add("loaded_modules", "logHandling")
  msg.Add("loaded_modules", "databases")
  msg.Add("loaded_modules", "server_server_com")
  msg.Add("loaded_modules", "clMessages")
  msg.Add("loaded_modules", "goSusi")
  msg.Add("key", keys[0])
  msg.Add("target", target)
  
  serverpackageskey := config.ModuleKey["[ServerPackages]"]

  util.Log(2, "DEBUG! Sending %v to %v encrypted with key %v", header, target, serverpackageskey)
  Peer(target).Tell(msg.String(), serverpackageskey)
}


// Handles the message "new_server".
//  xmlmsg: the decrypted and parsed message
func new_server(xmlmsg *xml.Hash) {
  server, _ := util.Resolve(xmlmsg.Text("source"), config.IP)
  if server == config.ServerSourceAddress { return } // never accept our own address as peer
  setGoSusi(xmlmsg)
  db.ServerUpdate(xmlmsg)
  handleClients(xmlmsg)
  go util.WithPanicHandler(func() {
    Send_new_server("confirm_new_server", server)
    Peer(server).SyncAll()
  })
  return
}

// Handles the message "confirm_new_server".
//  xmlmsg: the decrypted and parsed message
func confirm_new_server(xmlmsg *xml.Hash) {
  server, _ := util.Resolve(xmlmsg.Text("source"), config.IP)
  if server == config.ServerSourceAddress { return } // never accept our own address as peer
  setGoSusi(xmlmsg)
  handleClients(xmlmsg)
  Peer(xmlmsg.Text("source")).SyncAll()
  db.ServerUpdate(xmlmsg)
}

type ClientsToUpdate struct {
  Server string
  // maps MAC => IP:PORT
  Clients map[string]string
}

// If x is in up with the same Server and IP:PORT, remove it from up.
func (up *ClientsToUpdate) Accepts(x *xml.Hash) bool {
  if x == nil { return false }
  macaddress := x.Text("macaddress")
  if c_addr,ok := up.Clients[macaddress]; ok {
    if c_addr == x.Text("client") && x.Text("source") == up.Server {
      delete(up.Clients, macaddress)
    }
  }
  return false
}


// Takes a confirm_new_server or new_server message and evaluates the <client>
// elements, converts them into new_foreign_client messages and passes these
// to the new_foreign_client() handler.
func handleClients(xmlmsg *xml.Hash) {
  clientsToUpdate := ClientsToUpdate{Server:xmlmsg.Text("source"),Clients:map[string]string{}}
  
  for client := xmlmsg.First("client"); client != nil; client = client.Next() {
    cli := strings.Split(client.Text(),",")
    if len(cli) != 2 {
      util.Log(0, "ERROR! Illegal <client> value: %v", client.Text())
      continue
    }
    
    clientsToUpdate.Clients[cli[1]]=cli[0]
  }
   
  // remove all unchanged clients from clientsToUpdate
  db.ClientsQuery(&clientsToUpdate)
  
  for macaddress, client := range clientsToUpdate.Clients {
    cxml := xml.NewHash("xml","header","new_foreign_client")
    cxml.Add("source", clientsToUpdate.Server)
    cxml.Add("target", config.ServerSourceAddress)
    cxml.Add("client", client)
    cxml.Add("macaddress", macaddress)
    cxml.Add("new_foreign_client")
    new_foreign_client(cxml)
  }
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

