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
         "io"
         "fmt"
         "net"
         "sync"
         "time"
         "sync/atomic"
         
         "../db"
         "../xml"
         "github.com/mbenkmann/golib/util"
         "github.com/mbenkmann/golib/deque"
         "../config"
         "../security"
       )

// A permanent connection to a peer. A PeerConnection is obtained via the
// Peer() function. All communication with peers that go-susi initiates is
// performed via PeerConnections.
// There are 2 modes of communication:
//  1) The asynchronous Ask() function. This opens a new connection to the peer
//     and runs in a new goroutine that waits for a reply.
//  2) The synchronous Tell() function. This sends all messages over the
//     PeerConnection.queue channel and from there a single goroutine passes them
//     on over a permanent TCP connection (which on the other side is serviced by
//     a single goroutine/thread). Replies are not permitted in this case (because
//     proper synchronization is hard to achieve when some messages have and others
//     don't have replies).
//     The primary use for the synchronous channel is the sending of 
//     foreign_job_updates, to make sure they all arrive in a well-defined order.
//     See documentation in jobdb.go at handleJobDBRequests() for more information.
type PeerConnection struct {
  // true when the peer is known to speak the go-susi protocol.
  is_gosusi bool
  // FIFO for encrypted string-messages to be sent to the peer. 
  // Each string (typically foreign_job_updates) is sent to the peer
  // over the permanent TCP connection. The Tell() function enqueues messages here.
  queue deque.Deque
  // IP:PORT of the peer.
  addr string
  // nil for a normal PeerConnection. Non-nil for PeerConnection that is
  // created in non-working state and can only return this error on Ask().
  err error
  // The persistent TCP connection.
  tcpConn net.Conn
  // Unix time (seconds since the epoch) of the time the peer went down. 0 if it's up.
  // Needs to be accessed atomically because there is no locking on PeerConnection.
  whendown int64
}

// Tells this connection if its peer 
// advertises <loaded_modules>goSusi</loaded_modules>.
func (conn *PeerConnection) SetGoSusi(is_gosusi bool) {
  if is_gosusi {
    util.Log(1, "INFO! Peer %v uses go-susi protocol", conn.addr)
  } else {
    util.Log(1, "INFO! Peer %v uses old gosa-si protocol", conn.addr)
  }
  conn.is_gosusi = is_gosusi
}

// Returns the last value set via SetGoSusi() or false if SetGoSusi()
// has never been called.
func (conn *PeerConnection) IsGoSusi() bool {
  return conn.is_gosusi
}

// Returns how long this peer has been down (0 if everything is okay).
// After config.MaxPeerDowntime the PeerConnection will first tell the jobdb to
// remove all jobs whose <siserver> is the broken peer, then the PeerConnection
// will dismantle itself.
func (conn *PeerConnection) Downtime() time.Duration {
  down := atomic.LoadInt64(&(conn.whendown))
  if down == 0 { return 0 }
  return (time.Duration(time.Now().Unix() - down)) * time.Second
}

// Sets conn.whendown to the current time and logs a message.
func (conn *PeerConnection) startDowntime() {
  var down int64 = time.Now().Unix()
  atomic.StoreInt64(&(conn.whendown), down)
  util.Log(0, "ERROR! Peer %v is down.", conn.addr)
}

// Sets conn.whendown to 0 and logs a message.
func (conn *PeerConnection) stopDowntime() {
  atomic.StoreInt64(&(conn.whendown), 0)
  util.Log(1, "INFO! Peer %v is up again.", conn.addr)
}

// Encrypts msg with key and sends it to the peer without waiting for a reply.
// If key == "" the first key from db.ServerKeys(peer) is used.
func (conn *PeerConnection) Tell(msg, key string) {
  if conn.err != nil { return }
  keys := db.ServerKeys(conn.addr)
  // If we use TLS and the target does, too
  if config.TLSClientConfig != nil && len(keys) > 0 && keys[0] == "" {
    key = ""
  } else if key == "" {
   if len(keys) == 0 {
     util.Log(0, "ERROR! PeerConnection.Tell: No key known for peer %v", conn.addr)
     return
   }
   key = keys[0]
  }
  util.Log(1, "INFO! Telling %v: %v", conn.addr, msg)
  // If key == "" at this point, we're using TLS
  if key == "" {
    conn.queue.Push(msg)
  } else {
    conn.queue.Push(security.GosaEncrypt(msg, key))
  }
}

// Encrypts request with key, sends it to the peer and returns a channel 
// from which the peer's reply can be received (already decrypted with
// the same key). It is guaranteed that a reply will
// be available from this channel even if the peer connection breaks
// or the peer does not reply within a certain time. In the case of
// an error, the reply will be an error reply (as returned by
// message.ErrorReply()). The returned channel will be buffered and
// the producer goroutine will close it after writing the reply. This
// means it is permissible to ignore the reply without risk of a 
// goroutine leak.
// If key == "" the first key from db.ServerKeys(peer) is used.
func (conn *PeerConnection) Ask(request, key string) <-chan string {
  c := make(chan string, 1)
  
  if conn.err != nil {
    c<-ErrorReply(conn.err)
    close(c)
    return c
  }

  keys := db.ServerKeys(conn.addr)  
  // If we use TLS and the target does, too
  if config.TLSClientConfig != nil && len(keys) > 0 && keys[0] == "" {
    key = ""
  } else if key == "" {
   if len(keys) == 0 {
     c<-ErrorReply("PeerConnection.Ask: No key known for peer " + conn.addr)
     close(c)
     return c
   }
   key = keys[0]
  }
  
  go util.WithPanicHandler(func(){
    defer close(c)
    var tcpconn net.Conn
    var err error
    if key == "" { // TLS
      // We just use security.SendLnTo() to establish the TLS connection
      // The empty line that is sent is ignored by the receiving go-susi.
      tcpconn, _ = security.SendLnTo(conn.addr, "", "", true)
      if tcpconn == nil {
        // Unfortunately we don't have the actual error from SendLnTo(), so generate
        // a generic one.
        err = fmt.Errorf("Could not establish TLS connection to %v", conn.addr)
      }
    } else {
      tcpconn, err = net.Dial("tcp", conn.addr)
    }

    if err != nil {
      c<-ErrorReply(err)
      // make sure handleConnection()/monitorConnection() notice that the peer is unreachable
      if conn.tcpConn != nil { conn.tcpConn.Close() }
    } else {
      defer tcpconn.Close()
      util.Log(1, "INFO! Asking %v: %v", conn.addr, request)
      encrypted := request
      if key != "" {
        encrypted = security.GosaEncrypt(request, key)
      }
      err = util.SendLn(tcpconn, encrypted, config.Timeout)
      // make sure handleConnection()/monitorConnection() notice that the peer is unreachable
      if err != nil && conn.tcpConn != nil { conn.tcpConn.Close() }
      reply, err := util.ReadLn(tcpconn, config.Timeout)
      if err != nil && err != io.EOF {
        util.Log(0, "ERROR! ReadLn(): ", err)
      }
      if key != "" {
        reply = security.GosaDecrypt(reply, key)
      }
      if reply == "" { 
        reply = ErrorReply("Communication error in Ask()") 
        // make sure handleConnection()/monitorConnection() notice that the peer is unreachable
        if conn.tcpConn != nil { conn.tcpConn.Close() }
      }
      util.Log(1, "INFO! Reply from %v: %v", conn.addr, reply)
      c<-reply
    }
  })
  return c
}

// Calls SyncAll() after a few seconds delay if this connection's peer is not
// a go-susi. This is used after foreign_job_updates has been sent, because
// gosa-si (unlike go-susi) does not broadcast changes it has done in reaction
// to foreign_job_updates.
func (conn* PeerConnection) SyncIfNotGoSusi() {
  if conn.IsGoSusi() { return }
  go func() {
    // See documentation of config.GosaQueryJobdbMaxDelay for
    // an explanation of why we subtract a few seconds.
    time.Sleep(config.GosaQueryJobdbMaxDelay - 1*time.Second)
    conn.SyncAll()
  }()
}

// Sends all local jobs and clients to the peer. If the peer is not a go-susi, also
// requests all of the peer's local jobs and converts them to a <sync>all</sync>
// message and feeds it into foreign_job_updates().
func (conn *PeerConnection) SyncAll() {
  
  // send all our clients as new_foreign_client messages
  for nfc := db.ClientsRegisteredAtThisServer().First("xml"); nfc != nil; nfc = nfc.Next() {
    nfc.FirstOrAdd("target").SetText(conn.addr)
    conn.Tell(nfc.String(), "")
  }
  
  if conn.IsGoSusi() {
    util.Log(1, "INFO! Full sync (go-susi protocol) with %v", conn.addr)
    db.JobsSyncAll(conn.addr, nil)
  } else 
  { // peer is not go-susi (or not known to be one, yet)
    go util.WithPanicHandler(func() {
      util.Log(1, "INFO! Full sync (gosa-si fallback) with %v", conn.addr)
      
      // Query the peer's database for 
      // * all jobs the peer is responsible for
      // * all jobs the peer thinks we are responsible for
      query := xml.NewHash("xml","header","gosa_query_jobdb")
      query.Add("source", "GOSA")
      query.Add("target", "GOSA")
      clause := query.Add("where").Add("clause")
      clause.Add("connector", "or")
      clause.Add("phrase").Add("siserver","localhost")
      clause.Add("phrase").Add("siserver",conn.addr)
      clause.Add("phrase").Add("siserver",config.ServerSourceAddress)
      
      jobs_str := <- conn.Ask(query.String(), config.ModuleKey["[GOsaPackages]"])
      jobs, err := xml.StringToHash(jobs_str)
      if err != nil {
        util.Log(0, "ERROR! gosa_query_jobdb: Error decoding reply from peer %v: %v", conn.addr, err)
        // Bail out. Otherwise we would end up removing all of the peer's jobs from
        // our database if the peer is down. While that would be one way of dealing
        // with this case, we prefer to keep those jobs and convert them into
        // state "error" with an error message about the downtime. This happens
        // in gosa_query_jobdb.go.
        return 
      }
      
      if jobs.First("error_string") != nil { 
        util.Log(0, "ERROR! gosa_query_jobdb: Peer %v returned error: %v", conn.addr, jobs.Text("error_string"))
        // Bail out. See explanation further above.
        return 
      }
      
      // Now we extract from jobs those that are the responsibility of the
      // peer and synthesize a foreign_job_updates with <sync>all</sync> from them.
      // This leaves in jobs those the peer believes belong to us.
      
      fju := jobs.Remove(xml.FilterOr([]xml.HashFilter{xml.FilterSimple("siserver","localhost"),xml.FilterSimple("siserver",conn.addr)}))
      fju.Rename("xml")
      fju.Add("header","foreign_job_updates")
      fju.Add("source", conn.addr)
      fju.Add("target", config.ServerSourceAddress)
      fju.Add("sync", "all")
      
      util.Log(2, "DEBUG! Queuing synthetic fju: %v", fju)
      foreign_job_updates(fju)
      
      db.JobsSyncAll(conn.addr, jobs)
    })
  }
}

// Reads from connection tcpConn, logs any data received as an error and signals
// actual network errors by closing the connection and pinging the queue.
// This function returns when the first error is encountered on tcpConn. 
func monitorConnection(tcpConn net.Conn, queue *deque.Deque) {
  buf := make([]byte, 65536)
  for {
    n, err := tcpConn.Read(buf)
    if n > 0 {
      util.Log(0, "ERROR! Received %v bytes of unexpected data on Tell() channel to %v",n,tcpConn.RemoteAddr())
    }
    
    if err != nil {
      util.Log(2, "DEBUG! monitorConnection terminating: %v", err)
      tcpConn.Close() // make sure the connection is closed in case the error didn't
      queue.Push("") // ping to wake up handleConnection() if it's blocked
      return
    }
  }
}

var peerDownError = fmt.Errorf("Peer is down")

// Must be called in a separate goroutine for each newly created PeerConnection.
// Will not return without first removing all jobs associated with the peer from
// jobdb and removing the PeerConnection itself from the connections list.
func (conn *PeerConnection) handleConnection() {
  var err error
  var pingerRunning int32

  for {
    // gosa-si puts incoming messages into incomingdb and then
    // processes them in the order they are returned by the database
    // which causes messages to be processed in the wrong order.
    // To counteract this we wait a little between messages.
    // The wait time may seem long, but even with as much as 250ms
    // I observed the fju for a new job and the fju that adds the
    // plainname getting mixed up. Apparently gosa-si takes time
    // in the seconds range to process messages.
    // If we have >= 10 messages backlog, we don't wait. It's likely
    // that the later messages have more recent fju data anyway.
    if !conn.IsGoSusi() && conn.queue.Count() < 10 { time.Sleep(1000*time.Millisecond) }
    
    message := conn.queue.Next().(string)
    if conn.tcpConn != nil {
      err = util.SendLn(conn.tcpConn, message, config.Timeout) 
    } else {
      err = peerDownError
    }
    
    if err != nil {
      util.Log(2, "DEBUG! handleConnection() SendLn #1 to %v failed: %v", conn.addr, err)
      if conn.tcpConn != nil { conn.tcpConn.Close() } // make sure connection is closed in case the error didn't
      
      // try to re-establish connection
      keys := db.ServerKeys(conn.addr)
      // If we use TLS and the peer does, too, or we don't know => use TLS
      if config.TLSClientConfig != nil && ( len(keys) == 0 || keys[0] == "" ) {
        // We just use security.SendLnTo() to establish the TLS connection
        // The empty line that is sent is ignored by the receiving go-susi.
        conn.tcpConn, _ = security.SendLnTo(conn.addr, "", "", true)
        if conn.tcpConn == nil {
          // Unfortunately we don't have the actual error from SendLnTo(), so generate
          // a generic one.
          err = fmt.Errorf("Could not establish TLS connection to %v", conn.addr)
        }
      } else {
        conn.tcpConn, err = net.Dial("tcp", conn.addr)
        if err != nil {
          errkeepalive := conn.tcpConn.(*net.TCPConn).SetKeepAlive(true)
          if errkeepalive != nil {
            util.Log(0, "ERROR! SetKeepAlive: %v", errkeepalive)
          }
        }
      }

      if err == nil {
        util.Log(2, "DEBUG! handleConnection() re-connected to %v", conn.addr)

        conn.stopDowntime() 
        go monitorConnection(conn.tcpConn, &conn.queue)  
        // try to re-send message
        err = util.SendLn(conn.tcpConn, message, config.Timeout)
        if err != nil { 
          util.Log(2, "DEBUG! handleConnection() SendLn #2 to %v failed: %v", conn.addr,err)
          conn.tcpConn.Close() // if resending failed, make sure connection is closed
          // NOTE: There will be no further retransmission attempts of the message.
          //       It is now lost. However if the peer comes online again, we will do
          //       a full sync to make up for any lost foreign_job_updates messages.
        } else {
          // resending succeed => We're back in business. Do full sync.
          // If peer is not go-susi it generates a new key after re-starting,
          // so we need to send it a key, so that it understands our fju.
          // go-susi doesn't need this. See the long comment in db/serverdb.go:addServer()
          // However it is possible that the database of a go-susi has been nuked,
          // so just to be on the save side we send new_server anyway.
          // The new_server/confirm_new_server exchange will automatically trigger
          // a full sync.
          Send_new_server("new_server", conn.addr)
        }
      }
      
      // If either re-establishing the connection or resending the message failed
      // we wait a little and then ping the queue which will trigger another attempt
      // to re-establish the connection.
      // We increase the wait interval based on the length of the downtime. 
      // After a downtime of config.MaxPeerDowntime we give up, clean remaining 
      // jobs associated with the peer from the jobdb, remove the peer from serverdb
      // and then remove this PeerConnection from the list of connections and terminate.
      //
      // NOTE: Every message that comes in due to other events will also result
      //       in an attempt to re-establish the connection. In particular if
      //       the peer actually went down and then comes up again, it should
      //       send us a new_server message which in turn will cause a Tell()
      //       with our confirm_new_server message that will cause the
      //       connection to be re-established.
      //       The ping here is only a fallback for the case where nothing
      //       happens on our end and the peer doesn't send us new_server 
      //       (e.g. because its dns-lookup is disabled).
      if err != nil {
        util.Log(2, "DEBUG! handleConnection() connection to %v failed: %v", conn.addr, err)
        
        // start downtime if it's not already running
        if atomic.LoadInt64(&(conn.whendown)) == 0 { conn.startDowntime() } 
        
        // if we don't already have a pinger running, start one to ping us
        // after some time to make us try connecting again.
        if atomic.LoadInt32(&pingerRunning) == 0 {
          atomic.AddInt32(&pingerRunning, 1)
          
          down := conn.Downtime()
          maxdelay := config.MaxPeerDowntime - down
          var delay time.Duration
          // For the first 10 minutes we try every 10s to re-establish the connection
          if maxdelay > 0 && down < 10*time.Minute { delay = 10*time.Second } else
          // For the first day we try every 10 minutes
          if maxdelay > 0 && down < 24*time.Hour { delay = 10*time.Minute } else
          // Then we go to 30 minute intervals
          if maxdelay > 0 { delay = 30*time.Minute } else
          // Finally we give up
          {
            util.Log(2, "DEBUG! handleConnection() giving up. Removing jobs and PeerConnection for %v", conn.addr)
            db.JobsRemoveForeign(xml.FilterSimple("siserver",conn.addr))
            db.ServerRemove(conn.addr)
            connections_mutex.Lock()
            delete(connections,conn.addr)
            connections_mutex.Unlock()
            return
          }
          
          if delay > maxdelay { delay = maxdelay }
          
          // Wait and ping in the background, so that we don't miss other messages
          go func() {
            time.Sleep(delay)
            atomic.AddInt32(&pingerRunning, -1)
            conn.queue.Push("")
          }()
        }
      }
    }
  }
}

// Maps IP:ADDR to a PeerConnection object that talks to that peer. All accesses
// to connections are protected by connections_mutex.
var connections = map[string]*PeerConnection{}

// All access to connections must be protected by this mutex.
var connections_mutex sync.Mutex

// Returns a PeerConnection for talking to addr, which can be either
// IP:ADDR or HOST:ADDR (where HOST is something that DNS can resolve).
func Peer(addr string) *PeerConnection {
  addr, err := util.Resolve(addr, config.IP)
  if err != nil {
    return &PeerConnection{err:err}
  }
  
  host, port, err := net.SplitHostPort(addr)
  if err != nil {
    return &PeerConnection{err:err}
  }
  
  addr = host + ":" + port
  
  if addr == config.ServerSourceAddress { 
    panic("Peer() called with my own address. This is a bug!") 
  }
  
  connections_mutex.Lock()
  defer connections_mutex.Unlock()
  
  conn, have_already := connections[addr]
  if !have_already {
    conn = &PeerConnection{is_gosusi:false, addr:addr}
    connections[addr] = conn
    go util.WithPanicHandler(func(){conn.handleConnection()})
  }
  return conn
}

// Infinite loop to forward db.ForeignJobUpdates (see jobdb.go) 
// to the respective targets.
func DistributeForeignJobUpdates() {
    for fju := range db.ForeignJobUpdates {
      target := fju.Text("target")
      
      // see explanation in jobdb.go for var ForeignJobUpdates
      all_gosasi := (target == "gosa-si")
      if all_gosasi { target = "" }
      syncIfNotGoSusi := fju.RemoveFirst("SyncIfNotGoSusi")
      if syncIfNotGoSusi != nil && target != "" && !Peer(target).IsGoSusi() { 
        target = ""
      }
      
      if target != "" {
        Peer(target).Tell(fju.String(), "")
        if syncIfNotGoSusi != nil {
          Peer(target).SyncIfNotGoSusi()
        }
      } else
      { // send to ALL peers (possibly limited by all_gosasi)
        connections_mutex.Lock()
        for addr, peer := range connections {
          if all_gosasi && peer.IsGoSusi() { continue }
          fju.First("target").SetText(addr)
          peer.Tell(fju.String(), "")
          if syncIfNotGoSusi != nil {
            peer.SyncIfNotGoSusi()
          }
        }
        connections_mutex.Unlock()
      }
    }
}



