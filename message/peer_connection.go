package message

import (
         "fmt"
         "net"
         "sync"
         "time"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

type PeerConnection struct {
  is_gosusi bool
  overflow bool
  addr string
  err error
}

// Returns the time this peer has been down (0 if everything is okay).
func (conn *PeerConnection) Downtime() time.Duration {
/*
After 7 days of downtime, the PeerConnection gives up, issues a JobsRemoveForeign
for all jobs belonging to the peer and removes itself (LOCK!!) from the connections
list, then the goroutine terminates.


gosa_query_jobdb uses JobsQuery to query for the respective jobs, then it
postprocesses the query and for each siserver that is down (cache the Downtime()
to get consistent results for all jobs) replace the status with error and inserts
a result with the message "SERVERNAME(from reverse lookup of ip) has been down for DURATION"


gosa_delete_jobdb_entry can not delete jobs from servers that are down. This is
a) good because it prevents overzealous admins from removing errors that other
   admins haven't seen yet.
b) an automatic result from the fact that foreign jobs are never removed directly
   but converted to fju+full sync (which fails if the server is down, leaving the
   old jobs intact)
*/
  return 0
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

// Returns true if this connection's peer 
// advertises <loaded_modules>goSusi</loaded_modules>. If this method
// returns false the peer does either not support the goSusi protocol or
// it is yet unknown whether it does.
func (conn *PeerConnection) IsGoSusi() bool {
  return conn.is_gosusi
}

// Encrypts msg with key and sends it to the peer without waiting for a reply.
// If key == "" the first key from db.ServerKeys(peer) is used.
func (conn *PeerConnection) Tell(msg, key string) {
  if conn.err != nil { return }
  if key == "" {
   keys := db.ServerKeys(conn.addr)
   if len(keys) == 0 {
     util.Log(0, "ERROR! PeerConnection.Tell: No key known for peer %v", conn.addr)
     return
   }
   key = keys[0]
  }
  // if the request channel overflows, conn.overflow is set to true
  util.Log(2, "DEBUG! Telling %v: %v", conn.addr, msg)
  go util.WithPanicHandler(func(){util.SendLnTo(conn.addr, GosaEncrypt(msg, key), config.Timeout)})
}

// Encrypts request with key, sends it to the peer and returns a channel 
// from which the peer's reply can be received (already decrypted with
// the same key). It is guaranteed that a reply will
// be available from this channel even if the peer connection breaks
// or the peer does not reply within a certain time. In the case of
// an error, the reply will be an error reply (as returned by
// message.ErrorReply()). The returned channel will be buffered and
// the producer goroutine will close it after writing the reply. This
// means it is permissible to ignore reply without risk of a 
// goroutine leak.
func (conn *PeerConnection) Ask(request, key string) <-chan string {
  // if the request channel overflows, conn.overflow is set to true
  
  c := make(chan string, 1)
  
  if conn.err != nil {
    c<-ErrorReply(conn.err)
    return c
  }
  
  go util.WithPanicHandler(func(){
    tcpconn, err := net.Dial("tcp", conn.addr)
    if err != nil {
      c<-ErrorReply(err)
    } else {
      defer tcpconn.Close()
      util.Log(2, "DEBUG! Asking %v: %v", conn.addr, request)
      util.SendLn(tcpconn, GosaEncrypt(request, key), config.Timeout)
      reply := GosaDecrypt(util.ReadLn(tcpconn, config.Timeout), key)
      if reply == "" { reply = "General communication error" } 
      util.Log(2, "DEBUG! Reply from %v: %v", conn.addr, reply)
      c<-reply
    }
  })
  return c
}

// Calls SyncAll() after a few seconds delay if this connection's peer is not
// a go-susi. This is used after foreign_job_updates has been sent, because
// gosa-si (unlike go-susi) does not broadcast changes it has done in reaction
// to foreign_job_updates.
func (conn* PeerConnection) SyncNonGoSusi() {
  if conn.IsGoSusi() { return }
  go func() {
    time.Sleep(5*time.Second) // 5s should be enough, even for gosa-si
    conn.SyncAll()
  }()
}

// Sends all local jobs to the peer. If the peer is not a go-susi, also
// requests all of the peer's local jobs and converts them to a <sync>all</sync>
// message and feeds it into foreign_job_updates().
func (conn *PeerConnection) SyncAll() {
  if conn.IsGoSusi() {
    util.Log(1, "INFO! Full sync (go-susi protocol) with %v", conn.addr)
    db.JobsSyncAll(conn.addr, nil)
  } else 
  { // peer is not go-susi (or not known to be one, yet)
    go func() {
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
    }()
  }
}

func (conn *PeerConnection) handleConnection() {
/*  siehe auch NOTE unten bei GetConnection()
  
  wenn eine abgebrochene Verbindung re-established wird bzw. wenn
  eine Verbindung zum ersten mal established wird,
  wird aufgerufen db.JobsSendAllLocalJobsToPeer(addr), was einen
  Request in die JobDBRequestQueue tut. Wenn dieser abgearbeitet wird,
  dann werden alle lokalen Jobs aus der JobDB zusammengesammelt und
  als foreign_job_updates Message mit <sync>all</sync> in die
  ForeignJobUpdatesQueue gesteckt, wobei das target auf addr gesetzt wird
  (also nicht all). Die Demultiplexer-Goroutine, die die ForeignJobUpdatesQueue
  abarbeitet (vielleicht auch die Hauptschleife in go-susi.go?) ruft dann
  PeerConnectionManager.GetConnection(addr).ForeignJobUpdates(data) auf.
  
  Siehe auch weiter unten: GetAllLocalJobsFromPeer()
  
  */

  /* Ask() ist immer asynchron über neue Verbindung. 
  Das eliminiert Synchronisationsprobleme durch verzögerte oder fehlende Antworten.
  Trotzdem müssen rogue Daten der Tell-Verbindung ausgelesen (und geloggt) werden
  zur Sicherheit.
  
  */
  
  /* if overflow, first make sure there is an overflow (because maybe
  we removed a message after the overflow was set) by pushing dummy requests
  into the channel until it is full. This makes sure that from now on all
  new requests will immediately send an error reply on their reply channels
  and we can be sure the following operations don't race with new incoming
  requests:
  - replace this PeerConnection in connections map with
  a new one that has double the size channel buffer
  - then send ErrorReply on all reply channels from pending requests. Because we
    made sure the channel is actually full, this operation doesn't race with new
    incoming requests. They will notice immediately that the channel is full and
    will send an ErrorReply on their reply channel.
  - then shut down the goroutine.
  */
}



/*
Erweiterung foreign_job_updates:
<sync>none</sync> oder kein <sync> element: f_j_u kann in beliebiger Reihenfolge
            zu anderen f_j_u stehen
<sync>all</sync> die f_j_u enthält alle Jobs des sendenden Servers. Der empfangende
             Server sollte alle Jobs in seiner Datenbank die den sendenden Server
             als <siserver> ausweisen löschen und durch die Jobs aus dieser Nachricht
             ersetzen.
<sync>ordered</sync> Der sendende Server garantiert, dass
             a) er alle f_j_u über eine dauerhafte Verbindung sendet.
             b) die Reihenfolge der <answerX> in aufsteigender Folge der X
                innerhalb einer f_j_u, sowie die Reihenfolge in der die f_j_u
                über die dauerhaften Verbindung gesendet werden der Reihenfolge
                der Edits auf der Datenbank des sendenden Servers entspricht.
                D.h. <answer1> des ersten f_j_u entspricht einer Änderung, die
                zeitlich vor <answer2> des ersten f_j_u liegt, was wiederum vor
                <answer1> des zweiten f_j_u liegt.
*/

/*func (conn *PeerConnection) DoStuff(...) ... {
  
  create peerConnectionRequest and put into queue
    NOTE: There are 2 possible ways to set the Reply channel in the
    peerConnectionRequest.
      1) create a fresh channel. In this case, concurrent calls to DoStuff() are
         independent and their replies may arrive in an order different from
         the order of DoStuff() calls.
      2) use a DoStuffReplyChannel channel that is shared by all concurrent calls
         to DoStuff(). In this case, because the peerConnectionRequests are
         processed by a single goroutine, the order of replies in the ReplyChannel
         will reflect the order of requests in the peerConnectionRequest channel
         which is the order in which requests are sent to the peer.
         A specific example would be GetAllLocalJobsFromPeer(). Concurrent calls
         to this method will request the jobdb from the peer multiple times
         and using a single GetAllLocalJobsFromPeerReplyChannel ensures that
         the oldest result is read first from the channel and the most current
         is read last, so that if these answers are pushed into our jobdb in
         the same order, the most recent data will overwrite the out of date data.
  
  possibly read from return channel
  possibly return read value to caller
} */

func (conn *PeerConnection) GetAllLocalJobsFromPeerWithDelay() {
  
// evtl. besser Gosa_query_jobdb_from(addr) in gosa_query_jobdb.go


  /*sendet gosa_query_jobdb an den peer mit einem where das alle lokalen Jobs
  des peers selektiert. Die Antwort wird in den globalen Kanal
  GetAllLocalJobsFromPeerReplyChannel geschoben.
  An diesem Kanal hängt eine Dauer-Goroutine, die alles was dort
  eingeht konvertiert in foreign_job_updates mit <sync>all</sync> und
  weiterschiebt in die JobDBRequestQueue. Der Übersichtlichkeit halber
  sollte diese Dauer-Goroutine in go-susi.go angesiedelt sein. Evtl. wird
  diese Funktion einfach in die Hauptschleife integriert.
  
  Diese Funktion wird aufgerufen
  a) von New_server() wenn der neue Server nicht goSusi in seinen loadedModules
     hat. Bei einem goSusi-Server ist der Aufruf dieser Methode nicht nötig, da
     ein Server der goSusi advertised sich verpflichtet, nur synchronisierte
     foreign_job_updates zu schicken und bei Erst- bzw. Wiederaufnahme einer
     Verbindung einen <sync>all</sync> zu senden.
  b) von Foreign_job_updates() wenn <sync>none</sync> bzw. nicht vorhanden ist oder
     wenn ein server X ein f_j_u betreffs eines Jobs des Servers Y an einen
     Server Z schickt. In dem Fall ruft Server Z diese Funktion mit Verzögerung
     auf, um von Server Y zu erfahren, was wirklich Sache ist. Die Verzögerung
     ist nötig, weil Server Y selbst erst auf das f_j_u reagieren muss.
     Ein Sonderfall für go-susi könnte hier zwar gemacht werden, wäre aber
     übertrieben, weil der Fall von 3 Verteilservern
     in der Praxis selten vorkommt und das Risiko für falsche Daten aufgrund der
     aktiven Abfrage extrem gering ist.
  c) von PeerConnection selbst, wenn es eine abgebrochene Verbindung
     wiederherstellt zu einem Peer der nicht goSusi in seinen loadedModules hat.
     Bei einem goSusi ist der Aufruf nicht nötig, weil einer von 2 Fällen eintritt:
     1) die ausgehende Verbindung des Peers über die der Peer seine
        synchronisierten foreign_job_updates sendet ist nicht zusammengebrochen.
        Dann ist der Fluss synchronisierter fju auch nicht gestört.
     2) der Peer sendet bei Wiederaufbau seiner ausgehenden Verbindung ohnehin einen
        <sync>all</sync> so dass sich ein explizites Abfragen der Datenbank erübrigt.
  */
}

type peerConnectionRequest struct {
  Request string
  Message string
  Reply chan string
}

var connections = map[string]*PeerConnection{}

var connections_mutex sync.Mutex

func Peer(addr string) *PeerConnection {
  host, port, err := net.SplitHostPort(addr)
  if err != nil {
    return &PeerConnection{err:err}
  }
  
  addrs, err := net.LookupIP(host)
  if err != nil {
    return &PeerConnection{err:err}
  }
  
  if len(addrs) == 0 {
    return &PeerConnection{err:fmt.Errorf("No IP address for %v",host)}
  }
  
  addr = addrs[0].String() + ":" + port
  
  connections_mutex.Lock()
  defer connections_mutex.Unlock()
  
  conn, ok := connections[addr]
  if !ok {
    conn = &PeerConnection{is_gosusi:false, overflow:false, addr:addr}
    connections[addr] = conn 
  }
  
  /*if connections does not contain mapping for addr {
    create new connection
    go conn.HandleConnection()
       NOTE: This goroutine never completes. At some point it will no longer set
       a timer to try to re-establish the connection, so that potentially (if
       a peer is permanently shut down) the goroutine hangs around forever,
       waiting on its peerConnectionRequest channel. This is harmless, because
       even under the assumption that a go-susi server runs uninterrupted for
       years there are never going to accumulate a significant number of dead
       peers. The alternative of shutting down the goroutine at some point
       (and possibly removing the PeerConnection from the Manager) would
       be difficult to implement without race conditions, because just while
       trying to shut down the connection, a request might come in.
  }*/
  return conn
}

// Infinite loop to forward db.ForeignJobUpdates (see jobdb.go) 
// to the respective targets.
func init() {
  go func() {
    for fju := range db.ForeignJobUpdates {
      target := fju.Text("target")
      
      // see explanation in jobdb.go for var ForeignJobUpdates
      syncNonGoSusi := fju.RemoveFirst("SyncNonGoSusi")
      if syncNonGoSusi != nil && target != "" && !Peer(target).IsGoSusi() { 
        target = ""
      }
      
      if target != "" {
        Peer(target).Tell(fju.String(), "")
        if syncNonGoSusi != nil {
          Peer(target).SyncNonGoSusi()
        }
      } else
      { // send to ALL peers
        connections_mutex.Lock()
        for addr, peer := range connections {
          fju.First("target").SetText(addr)
          peer.Tell(fju.String(), "")
          if syncNonGoSusi != nil {
            peer.SyncNonGoSusi()
          }
        }
        connections_mutex.Unlock()
      }
    }
  }()
}



