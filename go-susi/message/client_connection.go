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

package message

import (
         "net"
         "sync"
         "time"
          
         "../db"
         "../xml"
         "github.com/mbenkmann/golib/util"
         "../config"
       )

// Sends garbage to all clients listed in db.ClientsWeMayHave to
// prompt them to send new here_i_am messages.
func CheckPossibleClients() {
  for child := db.ClientsWeMayHave.FirstChild(); child != nil; child = child.Next() {
    client := child.Element()
    addr := client.Text("client")
    if addr == config.ServerSourceAddress { continue } // do not send Müll to ourselves
    util.Log(1, "INFO! Sending 'Müll' to %v", addr)
    go util.SendLnTo(addr, "Müll", config.Timeout)
  }
}

// A message to be sent to the client paired with an expiration time after
// which the message is discarded even if it could not be sent successfully.
type ClientMessage struct {
  // unencrypted message text.
  Text string
  
  // Expiration time. If the message could not be sent until that time, it
  // will be discarded.
  Expires time.Time
}

// A connection to a client. 
// A ClientConnection is obtained via the Client() function. 
// All communication with clients that go-susi initiates is
// performed via ClientConnections.
//
// NOTE:
// Up to revision 5a9da13080de this
// was a permanent connection similar to peer_connection.go.
// However while examining issue #58 I came to the conclusion
// that there's no real need to maintain client connections
// as permanent connections, because unlike
// server-server-communication there's no need to preserve
// message order when sending messages to clients (at least
// at this time I'm not aware of such a need).
//
type ClientConnection struct {
  // IP:PORT of the peer.
  addr string
}

// Tell(msg, ttl): Tries to send text to the client.
//                 The ttl determines how long the message will be buffered for
//                 resend attempts if sending fails. ttl values smaller than
//                 100ms will be treated as 100ms.
func (conn *ClientConnection) Tell(text string, ttl time.Duration) {
  if ttl < 100*time.Millisecond {
    ttl = 100*time.Millisecond
  }
  util.Log(2, "DEBUG! Tell(): Queuing message for client %v with TTL %v: %v", conn.addr, ttl, text)

  msg := &ClientMessage{text, time.Now().Add(ttl)}
  
  go util.WithPanicHandler(func(){
    var try uint = 0
    
    if msg.Expires.Before(time.Now()) {
      util.Log(0, "ERROR! Scheduling of goroutine for sending message to %v delayed more than TTL %v => Message will not be sent", conn.addr, ttl)
    } else {
    for {
      if try > 0 {
        expiry := msg.Expires.Sub(time.Now())
        if expiry <= 0 { 
          break
        }
        delay := (1 << try) * time.Second
        if delay > 60*time.Second { delay = 60*time.Second }
        if delay > expiry { delay = expiry-1*time.Second }
        if delay <= 0 {
          break
        }
        util.Log(2, "DEBUG! Sleeping %v before next send attempt", delay)
        time.Sleep(delay)
      }
    
      try++
      
      util.Log(1, "INFO! Attempt #%v to send message to %v: %v", try, conn.addr, msg.Text)
      
      client := db.ClientWithAddress(conn.addr)
      if client == nil {
        if conn.addr == config.ServerSourceAddress {
          // If sending to myself (e.g. new_ldap_config), fake a client object
          client = xml.NewHash("xml","source",config.ServerSourceAddress)
          client.Add("key", config.ModuleKey["[ClientPackages]"])
        } else {
          util.Log(0, "ERROR! Client %v not found in clientdb", conn.addr)
          continue
        }
      }
      
      // if client is registered at a foreign server
      if client.Text("source") != config.ServerSourceAddress {
        util.Log(1, "INFO! Client %v is registered at %v => Forwarding message", conn.addr, client.Text("source"))
        
        // MESSAGE FORWARDING NOT YET IMPLEMENTED
        util.Log(0, "ERROR! Message forwarding not yet implemented")
        break
        
      } else { // if client is registered at our server
      
        keys := client.Get("key")
        if len(keys) == 0 {
          // This case should be impossible. A client's here_i_am message always contains a key (unless the client is buggy).
          util.Log(0, "ERROR! No key known for client %v", conn.addr)
          break
        }
      
        encrypted := GosaEncrypt(msg.Text, keys[0])
      
        tcpConn, err := net.Dial("tcp", conn.addr)
        if err != nil {
          util.Log(0,"ERROR! Dial() could not connect to %v: %v",conn.addr, err)
          continue
        }
        
        if msg.Expires.Before(time.Now()) {
          util.Log(0, "ERROR! Connection to %v established, but TTL %v has expired in the meantime => Message will not be sent", conn.addr, ttl)
          break
        }
        
        err = tcpConn.(*net.TCPConn).SetKeepAlive(true)
        if err != nil {
          util.Log(0, "ERROR! SetKeepAlive: %v", err)
          // This is not fatal => Don't abort send attempt
        }
        
        util.Log(2, "DEBUG! Sending message to %v encrypted with key %v", conn.addr, keys[0])
        err = util.SendLn(tcpConn, encrypted, config.Timeout) 
        tcpConn.Close()
        if err == nil { 
          util.Log(2, "DEBUG! Successfully sent message to %v: %v", conn.addr, msg.Text)
          return // not break! break would cause an error message to be logged
        } else {
          util.Log(0, "ERROR! SendLn() to %v failed: %v", conn.addr, err)
        }
      }
    }  
    }
    
    util.Log(0, "ERROR! Cannot send message to %v: %v", conn.addr, msg.Text)
  })
}

// Maps IP:ADDR to a ClientConnection object that talks to that peer. All accesses
// to client_connections are protected by client_connections_mutex.
var client_connections = map[string]*ClientConnection{}

// All access to client_connections must be protected by this mutex.
var client_connections_mutex sync.Mutex

// Returns a ClientConnection for talking to addr, which can be either
// IP:PORT or HOST:PORT (where HOST is something that DNS can resolve).
func Client(addr string) *ClientConnection {
  addr, err := util.Resolve(addr, config.IP)
  if err != nil {
    util.Log(0, "ERROR! Client(%v): %v", addr, err)
    return &ClientConnection{addr:"127.0.0.1:0"}
  }
  
  _, _, err = net.SplitHostPort(addr)
  if err != nil {
    util.Log(0, "ERROR! Client(%v): %v", addr, err)
    return &ClientConnection{addr:"127.0.0.1:0"}
  }
  
  client_connections_mutex.Lock()
  defer client_connections_mutex.Unlock()
  
  conn, have_already := client_connections[addr]
  if !have_already {
    conn = &ClientConnection{addr:addr}
    client_connections[addr] = conn
  }
  return conn
}



