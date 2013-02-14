/*
Copyright (c) 2013 Matthias S. Benkmann

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

package db

import (
         "os"
         "time"
         "regexp"
         "strings"
         
         "../xml"
         "../util"
         "../config"
       )

// Stores our own and foreign clients. Format:
// <clientdb>
//   <xml>
//     <header>new_foreign_client</header>
//     <source>172.16.2.12:20081</source>   (the server where client is registered)
//     <target>1.2.3.4:200081</target>      (me)
//     <client>172.16.2.52:20083</client>
//     <macaddress>12:34:56:78:9A:BC</macaddress> (the client's MAC)
//     <key>current_key</key>  (optional)
//     <key>previous_key</key> (optional)
//     <new_foreign_client></new_foreign_client>
//   </xml>
//    ...
// </clientdb>
var clientDB *xml.DB

// When ClientsInit() restores clients from config.ClientDBPath, it removes
// all local clients and stores them in this hash, because before we can
// accept them as our own, we need to check them, since they may already have
// registered with a different server. The format of this hash is the same as
// clientDB.
// message/client_connection.go:CheckPossibleClients() pings each client
// from this hash so that they are re-added to clientdb if they reply.
var ClientsWeMayHave *xml.Hash = xml.NewHash("clientdb")

// Initializes clientDB with data from the file config.ClientDBPath if it exists.
// See ClientsWeMayHave above for important info.
// Not an init() because main() needs to set up some things first.
func ClientsInit() {
  db_storer := &LoggingFileStorer{xml.FileStorer{config.ClientDBPath}}
  var delay time.Duration = 0
  clientDB = xml.NewDB("clientdb", db_storer, delay)
  if !config.FreshDatabase {
    xmldata, err := xml.FileToHash(config.ClientDBPath)
    if err != nil {
      if os.IsNotExist(err) { 
        /* File does not exist is not an error that needs to be reported */ 
      } else
      {
        util.Log(0, "ERROR! ClientsInit reading '%v': %v", config.ClientDBPath, err)
      }
    } else
    {
      clientDB.Init(xmldata)
      ClientsWeMayHave = clientDB.Remove(xml.FilterSimple("source",config.ServerSourceAddress))
    }
  }
}  



// Returns the entry from the clientdb or nil if the client is unknown.
// Entries are formatted as new_foreign_client messages:
//   <xml>
//     <header>new_foreign_client</header>
//     <source>172.16.2.12:20081</source>   (the server where client is registered)
//     <target>1.2.3.4:200081</target>      (me)
//     <client>172.16.2.52:20083</client>
//     <macaddress>12:34:56:78:9A:BC</macaddress> (the client's MAC)
//     <key>current_key</key>  (optional)
//     <key>previous_key</key> (optional)
//     <new_foreign_client></new_foreign_client>
//   </xml>
//
// NOTE: Foreign clients are those with source != config.ServerSourceAddress and
//       our clients are those with     source == config.ServerSourceAddress.
func ClientWithMAC(macaddress string) *xml.Hash { 
  return clientDB.Query(xml.FilterSimple("macaddress", macaddress)).First("xml")
}

// Returns a hash with the same format as clientDB that contains
// all known clients that are registered at this server. If no client
// is registered here, the result will be <clientdb></clientdb>.
func ClientsRegisteredAtThisServer() *xml.Hash {
  return clientDB.Query(xml.FilterSimple("source",config.ServerSourceAddress))
}

// Updates the data for client. client has the same format as returned by
// ClientWithMAC(), i.e. a new_foreign_client message.
func ClientUpdate(client *xml.Hash) {
  macaddress := client.Text("macaddress")
  caddr := client.Text("client")
  keys := ClientKeys(caddr)
  if len(keys) > 0 {
    // Add previous key as 2nd key, because due to parallel processes
    // we might still have pending messages encrypted with the previous key.
    client.Add("key", keys[0])
  }
  util.Log(2, "DEBUG! ClientUpdate for %v, handled by %v.", caddr, client.Text("source"))
  old := clientDB.Replace(xml.FilterSimple("macaddress", macaddress), false, client)
  
  // if the update assigns a client that was previously assigned to this server to
  // another server, double-check this new assignment by sending Müll to the the
  // client, which will cause it to send us a here_i_am if it still feels attached
  // to us. That here_i_am will then undo the incorrect assignment.
  for _, tag := range old.Subtags() {
    for oldclient := old.First(tag); oldclient != nil; oldclient = oldclient.Next() {
      if oldclient.Text("source") == config.ServerSourceAddress {
        addr := oldclient.Text("client")
        util.Log(1, "INFO! Sending 'Müll' to %v", addr)
        go util.SendLnTo(addr, "Müll", config.Timeout)
      }
    }
  }
}

// Returns all keys (0-length slice if none) known for the client identified by
// the given address. If the address is an IP without a port, the result may
// include keys from multiple clients running on the same machine. If the address
// includes a port, only keys from that specific client will be returned.
func ClientKeys(addr string) []string {
  result := make([]string, 0, 2)
  var filter xml.HashFilter
  if strings.Index(addr, ":") >= 0 {
    filter = xml.FilterSimple("client", addr)
  } else {
    filter = xml.FilterRegexp("client", "^"+regexp.QuoteMeta(addr+":")+"[0-9]+$")
  }
  for client := clientDB.Query(filter).First("xml");
      client != nil;
      client = client.Next() {
    result = append(result, client.Get("key")...)
  }
  return result
}

// Returns all client keys for all clients in the database.
func ClientKeysForAllClients() []string {
  return clientDB.ColumnValues("key")
}

