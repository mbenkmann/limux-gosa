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
         "fmt"
         "time"
         "bytes"
         "strings"
         
         "../xml"
         "../config"
         "../util"
       )

// Stores info about server peers. Entries in serverDB have the following
// structure (<ip> and at least one <key> are required):
//
//  <xml>
//    <ip>172.16.2.143</ip>
//    <source>172.16.2.143:20081</source>
//    <key>currentserverkey</key>
//    <key>previousserverkey</key>
//  </xml>
var serverDB *xml.DB

// Initializes serverDB with data from the file config.ServerDBPath if it exists.
func ServersInit() {
  db_storer := &xml.FileStorer{config.ServerDBPath}
  var delay time.Duration = 0
  serverDB = xml.NewDB("serverdb", db_storer, delay)
  xmldata, err := xml.FileToHash(config.ServerDBPath)
  if err != nil {
    if os.IsNotExist(err) { 
      /* File does not exist is not an error that needs to be reported */ 
    } else
    {
      util.Log(0, "ERROR! ServerInit reading '%v': %v", config.ServerDBPath, err)
    }
  } else
  {
    serverDB.Init(xmldata)
  }
  
  addDNSServers()
}  

// Adds servers listed in for service tcp/gosa-si to the serverDB.
func addDNSServers() {
  var cname string
  var addrs []*net.SRV
  cname, addrs, err := net.LookupSRV("gosa-si", "tcp", config.Domain)
  if err != nil {
    util.Log(0, "ERROR! LookupSRV: %v", err) 
    return
  }
  
  if len(addrs) == 0 {
    util.Log(1, "INFO! No other go-susi or gosa-si servers listed in DNS for domain '%v'", config.Domain)
  } else {
    servers := make([]string, len(addrs))
    for i := range addrs {
      servers[i] = fmt.Sprintf("%v:%v", strings.TrimRight(addrs[i].Target,"."), addrs[i].Port)
    }
    util.Log(1, "INFO! DNS lists the following %v servers: %v", cname, strings.Join(servers,", "))
    
    // add all servers listed in DNS to our database (skipping this server)
    for _, server := range servers {
      if !strings.HasPrefix(server, config.Hostname + "." + config.Domain + ":") {
        host, _, _ := net.SplitHostPort(server)
        addrs, err := net.LookupIP(host)
        if err != nil || len(addrs) == 0 {
          if err != nil {
            util.Log(0, "ERROR! LookupIP: %v", err)
          } else {
            util.Log(0, "ERROR! No IP address for %v", host)
          }
        } else 
        {
          ip := addrs[0].String()
          
          // if we don't have an entry for the server, generate a dummy entry.
          if len(ServerKeys(ip)) == 0 {
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
            if bytes.Compare([]byte(ip), []byte(config.IP)) < 0 {
              key = ip + config.IP
            } else {
              key = config.IP + ip
            }
            key = config.ModuleKey["[ServerPackages]"] + strings.Replace(key, ".", "", -1)
            server_xml := xml.NewHash("xml", "ip", ip)
            server_xml.Add("key", key)
            serverDB.Replace(xml.FilterSimple("ip", ip), false, server_xml)
          }
        }
      }
    }
  }
}

// Returns all keys (0-length slice if none) known for the server identified by
// the given IP address.
func ServerKeys(ip string) []string {
  result := make([]string, 0, 2)
  for server := serverDB.Query(xml.FilterSimple("ip", ip)).First("xml");
      server != nil;
      server = server.Next() {
    result = append(result, server.Get("key")...)
  }
  return result
}

// Returns a copy of the complete database in the following format:
//  <serverdb>
//    <xml>
//      <ip>1.2.3.4</ip>
//      <source>1.2.3.4:20081</source>
//      <key>key11</key>
//      <key>key12</key>
//    </xml>
//    <xml>
//      <ip>2.3.4.5</ip>
//      <source>2.3.4.5:20081</source>
//      <key>key21</key>
//      <key>key22</key>
//    </xml>
//    ...
//  </serverdb>
func Servers() *xml.Hash {
  return serverDB.Query(xml.FilterAll)
}
