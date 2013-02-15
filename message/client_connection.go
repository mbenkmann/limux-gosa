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
         "../util"
         "../util/deque"
         "../config"
       )

// Sends garbage to all clients listed in db.ClientsWeMayHave to
// prompt them to send new here_i_am messages.
func CheckPossibleClients() {
  for _, tag := range db.ClientsWeMayHave.Subtags() {
    for client := db.ClientsWeMayHave.First(tag); client != nil; client = client.Next() {
      addr := client.Text("client")
      util.Log(1, "INFO! Sending 'Müll' to %v", addr)
      go util.SendLnTo(addr, "Müll", config.Timeout)
    }
  }
}

type ClientMessage struct {
  // unencrypted message text.
  Text string
  Expires time.Time
}

// A connection to a client, permanent if the client is locally registered.
// A ClientConnection is obtained via the Client() function. 
// All communication with clients that go-susi initiates is
// performed via ClientConnections.
type ClientConnection struct {
  // FIFO input queue of ClientMessage elements.
  // The Tell() function enqueues messages here.
  queue deque.Deque
  
  // ClientMessage elements from the input queue are moved into this buffer 
  // for processing. They are removed from this buffer only after they have
  // either been sent successfully or have expired.
  buffer deque.Deque
  
  // IP:PORT of the peer.
  addr string
  
  // If the client is locally registered, this TCP connection is used to contact
  // it. The connection is kept open and will be used until an error occurs or
  // we discover the client is no longer registered here.
  tcpConn net.Conn
}

// Tell(msg, ttl): Tries to send msg to the client. If the client is locally
//                 registered the message will be sent directly, otherwise it
//                 will be forwarded via the server responsible for the client.
//                 The ttl determines how long the message will be buffered for
//                 resend attempts if sending fails. ttl==0 (or some other
//                 small ttl) should be set for
//                 messages that are only of interest to locally registered
//                 clients (like registered, ore new_ldap_config)
func (conn *ClientConnection) Tell(msg string, ttl time.Duration) {
  conn.queue.Push(ClientMessage{msg, time.Now().Add(ttl)})
}

// Maps IP:ADDR to a ClientConnection object that talks to that peer. All accesses
// to client_connections are protected by client_connections_mutex.
var client_connections = map[string]*ClientConnection{}

// All access to client_connections must be protected by this mutex.
var client_connections_mutex sync.Mutex


// Returns a ClientConnection for talking to addr, which can be either
// IP:ADDR or HOST:ADDR (where HOST is something that DNS can resolve).
func Client(addr string) *ClientConnection {
  if addr == config.ServerSourceAddress { 
    panic("Client() called with my own address. This is a bug!") 
  }
  
  host, port, err := net.SplitHostPort(addr)
  if err != nil {
    util.Log(0, "ERROR! Client(%v): %v", addr, err)
    return &ClientConnection{addr:"127.0.0.1:0"}
  }
  
  addrs, err := net.LookupIP(host)
  if err != nil {
    util.Log(0, "ERROR! Client(%v): %v", addr, err)
    return &ClientConnection{addr:"127.0.0.1:0"}
  }
  
  if len(addrs) == 0 { // I don't think this is possible but just in case...
    util.Log(0, "ERROR! No IP address for %v",host)
    return &ClientConnection{addr:"127.0.0.1:0"}
  }
  
  addr = addrs[0].String() + ":" + port
  
  client_connections_mutex.Lock()
  defer client_connections_mutex.Unlock()
  
  conn, have_already := client_connections[addr]
  if !have_already {
    conn = &ClientConnection{addr:addr}
    client_connections[addr] = conn
    go util.WithPanicHandler(func(){conn.handleConnection()})
  }
  return conn
}

func (conn *ClientConnection) tryToSend(message ClientMessage) bool {
  var err error
  
  client := db.ClientWithAddress(conn.addr)
  if client == nil {
    if conn.tcpConn != nil { conn.tcpConn.Close() }
    conn.tcpConn = nil
    return false
  }
  
  // if client is registered at a foreign server
  if client.Text("source") != config.ServerSourceAddress {
    if conn.tcpConn != nil { conn.tcpConn.Close() }
    conn.tcpConn = nil
    
    // MESSAGE FORWARDING NOT YET IMPLEMENTED
    
  } else { // if client is registered at our server
  
    keys := client.Get("key")
    if len(keys) == 0 {
      util.Log(0, "ERROR! ClientConnection.tryToSend: No key known for peer %v", conn.addr)
      return false
    }
  
    encrypted := GosaEncrypt(message.Text, keys[0])
  
    if conn.tcpConn != nil { // try sending via existing connection if it exists
      err = util.SendLn(conn.tcpConn, encrypted, config.Timeout) 
      if err == nil { return true }
        
      conn.tcpConn.Close()
      conn.tcpConn = nil
    }
    
    // try to (re-)establish connection
    conn.tcpConn, err = net.Dial("tcp", conn.addr)
    if err != nil {
      util.Log(0,"ERROR! ClientConnection.tryToSend() failed to establish connection to %v: %v",conn.addr,err)
      return false
    }
    
    err = conn.tcpConn.(*net.TCPConn).SetKeepAlive(true)
    if err != nil {
      util.Log(0, "ERROR! SetKeepAlive: %v", err)
      // This is not fatal => Don't abort
    }
    
    // try to send message over newly established connection
    err = util.SendLn(conn.tcpConn, encrypted, config.Timeout) 
    if err == nil { return true }
    
    util.Log(0, "ERROR! ClientConnection.tryToSend() message to %v: %v", conn.addr, err)
    conn.tcpConn.Close()
    conn.tcpConn = nil
  }
  
  return false
}

func (conn *ClientConnection) handleConnection() {
  var delay time.Duration

  for {
    // if no messages buffered, reset resend delay to infinity
    if conn.buffer.IsEmpty() { delay = 0 }
    
    // wait for either new input or expiry of delay for resend attempt
    conn.queue.WaitForItem(delay)
    
    // append new input to buffer
    for m := conn.queue.RemoveAt(0); m != nil; m = conn.queue.RemoveAt(0) {
      conn.buffer.Push(m)
    }
    
    // try to send messages
    for ; !conn.buffer.IsEmpty(); {
      
      message := conn.buffer.Next().(ClientMessage)
      
      if !conn.tryToSend(message) { // sending failed
        // put message back into buffer
        conn.buffer.Insert(message)
        
        // increase delay until resend
        delay = (delay + 1*time.Second) * 2
        if delay > 60*time.Second { delay = 60*time.Second }
        
        // remove expired messages from buffer
        now := time.Now()
        for i := 0; i < conn.buffer.Count(); {
          if conn.buffer.Peek(i).(ClientMessage).Expires.Before(now) {
            conn.buffer.RemoveAt(i)
          } else {
            i++
          }
        }
        
        // back to main loop
        break
      }
    }
  }
}


