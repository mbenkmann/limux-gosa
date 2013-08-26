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
         "os"
         "net"
         "time"
         "regexp"
         "strings"
         
         "../xml"
         "../config"
         "../util"
       )

// Stores info about server peers. Entries in serverDB have the following
// structure (<source> and at least one <key> are required):
//
//  <xml>
//    <source>172.16.2.143:20081</source>
//    <macaddress>00:50:1e:20:c3:20</macaddress>  (optional)
//    <key>currentserverkey</key>
//    <key>previousserverkey</key>
//  </xml>
var serverDB *xml.DB = xml.NewDB("serverdb",nil,0)

// Initializes serverDB with data from the file config.ServerDBPath if it exists,
// as well as the list of peer servers from DNS and [ServerPackages]/address.
// Not an init() because main() needs to set up some things first.
func ServersInit() {
  db_storer := &LoggingFileStorer{xml.FileStorer{config.ServerDBPath}}
  var delay time.Duration = config.DBPersistDelay
  serverDB = xml.NewDB("serverdb", db_storer, delay)
  if !config.FreshDatabase {
    xmldata, err := xml.FileToHash(config.ServerDBPath)
    if err != nil {
      if os.IsNotExist(err) { 
        /* File does not exist is not an error that needs to be reported */ 
      } else
      {
        util.Log(0, "ERROR! ServersInit reading '%v': %v", config.ServerDBPath, err)
      }
    } else
    {
      serverDB.Init(xmldata)
    }
  }
  
  if config.DNSLookup { 
    addServersFromDNS() 
  } else {
    util.Log(1, "INFO! DNS lookup disabled. Will not add peer servers from DNS.")
  }
  addServersFromConfig()
  util.Log(1,"INFO! All known peer addresses with duplicates removed: %v", ServerAddresses())
}  

// Persists the serverDB and prevents all further changes to it.
// This function does not return until the database has been persisted.
func ServersShutdown() {
  util.Log(1, "INFO! Shutting down servers database")
  serverDB.Shutdown()
  util.Log(1, "INFO! Servers database has been saved")
}

// Adds servers listed in config file to the serverDB.
func addServersFromConfig() {
  util.Log(1, "INFO! Config file lists %v peer server(s): %v", len(config.PeerServers), strings.Join(config.PeerServers,", "))
  for _, server := range config.PeerServers {
    addServer(server)
  }
}

// Adds servers listed in for service tcp/gosa-si to the serverDB.
func addServersFromDNS() {
  // add all servers listed in DNS to our database (skipping this server)
  for _, server := range config.ServersFromDNS() {
    addServer(server)
  }
}

// Adds server (host:port) to the database if it does not exist yet (and if it
// is not identical to this go-susi).
func addServer(server string) {
  server, err := util.Resolve(server)
  if err != nil {
    util.Log(0, "ERROR! util.Resolve(\"%v\"): %v", server, err)
    return
  }
  ip, port, err := net.SplitHostPort(server)
  if err != nil {
    util.Log(0, "ERROR! net.SplitHostPort(\"%v\"): %v", server, err)
    return
  }

  // translate loopback address to our own IP for consistency
  if ip == "127.0.0.1" { ip = config.IP }
  source := ip + ":" + port
  
  // do not add our own address
  if source == config.ServerSourceAddress { return }
  
  // if we don't have an entry for the server, generate a dummy entry.
  if len(ServerKeys(source)) == 0 {
    // There's no point in generating a random server key. 
    // First of all, the server key is only as secure as the ServerPackages
    // module key (because whoever has that can decrypt the message that
    // contains the server key).
    // Secondly the whole gosa-si protocol is not really secure. For instance
    // there is lots of known plaintext and no salting of messages. And the
    // really important messages are all encrypted with fixed keys anyway.
    // So instead of pretending more security by generating a random key,
    // we make debugging a little easier by generating a unique key derived
    // from the ServerPackages module key.
    var key string
    if ip < config.IP {
      key = ip + config.IP
    } else {
      key = config.IP + ip
    }
    key = config.ModuleKey["[ServerPackages]"] + strings.Replace(key, ".", "", -1)
    server_xml := xml.NewHash("xml", "source", source)
    server_xml.Add("key", key)
    ServerUpdate(server_xml)
  }
}

// Updates the data for server.
// server has the following format:
//   <xml>
//     <source>1.2.3.4:20081</source>
//     <macaddress>00:50:1e:20:c3:20</macaddress>  (optional)
//     <key>...</key>
//     ...
//   </xml>
func ServerUpdate(server *xml.Hash) {
  source := server.Text("source")
  keys := ServerKeys(source)
  if len(keys) > 0 {
    // Add previous key as 2nd key to server, because due to parallel processes
    // we might still have pending messages encrypted with the previous key.
    server.Add("key", keys[0])
  }
  util.Log(2, "DEBUG! ServerUpdate for %v: Keys are now %v", source, server.Get("key"))
  serverDB.Replace(xml.FilterSimple("source", source), false, server)
}

// Removes the server data for the server with the given IP:PORT address and
// returns the removed data or nil if the server was not in the database.
func ServerRemove(addr string) *xml.Hash {
  server := serverDB.Remove(xml.FilterSimple("source", addr))
  return server.First("xml")
}

// Returns all keys (0-length slice if none) known for the server identified by
// the given address. If the address is an IP without a port, the result may
// include keys from multiple servers running on the same machine. If the address
// includes a port, only keys from that specific server will be returned.
func ServerKeys(addr string) []string {
  result := make([]string, 0, 2)
  var filter xml.HashFilter
  if strings.Index(addr, ":") >= 0 {
    filter = xml.FilterSimple("source", addr)
  } else {
    filter = xml.FilterRegexp("source", "^"+regexp.QuoteMeta(addr+":")+"[0-9]+$")
  }
  for server := serverDB.Query(filter).First("xml");
      server != nil;
      server = server.Next() {
    result = append(result, server.Get("key")...)
  }
  return result
}

// Returns all server keys for all servers in the database.
func ServerKeysForAllServers() []string {
  return serverDB.ColumnValues("key")
}

// Returns a copy of the complete database in the following format:
//  <serverdb>
//    <xml>
//      <source>1.2.3.4:20081</source>
//      <macaddress>00:50:1e:20:c3:20</macaddress>  (optional)
//      <key>key11</key>
//      <key>key12</key>
//    </xml>
//    <xml>
//      <source>2.3.4.5:20081</source>
//      <key>key21</key>
//      <key>key22</key>
//    </xml>
//    ...
//  </serverdb>
func Servers() *xml.Hash {
  return serverDB.Query(xml.FilterAll)
}

// Returns all <source> addresses for all entries from the server DB.
func ServerAddresses() []string {
  return serverDB.ColumnValues("source")
}

// Returns the entry from the serverdb (format: <xml><source>...</xml>) of
// the server with the given MAC address, or nil if the server is either not
// in the serverDB or if its entry has no <macaddress> elememt.
func ServerWithMAC(macaddress string) *xml.Hash {
  server := serverDB.Query(xml.FilterSimple("macaddress", macaddress))
  return server.First("xml")
}
