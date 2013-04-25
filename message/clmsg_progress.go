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
         "time"
         "sync"
         "strconv"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

const ALMOST_DONE_PROGRESS = "95"

// Handles the message "CLMSG_PROGRESS".
//  xmlmsg: the decrypted and parsed message
func clmsg_progress(xmlmsg *xml.Hash) {
  macaddress := xmlmsg.Text("macaddress")
  progress   := xmlmsg.Text("CLMSG_PROGRESS")
  util.Log(1, "INFO! Progress info from client %v with MAC %v: %v",xmlmsg.Text("source"), progress, macaddress)
  // Because we don't know what kind of job the progress is for, we update
  // all local jobs in status processing for the client's MAC.
  // In theory only one job should be in status processing for a single client at
  // any given time, but sometimes jobs get "lost", typically through manual
  // intervention. Progressing all jobs in lockstep has the nice side effect of
  // taking such old stuck jobs along.
  filter := xml.FilterSimple("siserver",   config.ServerSourceAddress, 
                             "status",    "processing",
                             "macaddress", macaddress)
  db.JobsModifyLocal(filter, xml.NewHash("job","progress",progress))
  if progress == "100" {
    util.Log(1, "INFO! Progress 100%% => Setting status \"done\" for client %v with MAC %v",xmlmsg.Text("source"), macaddress)
    db.JobsModifyLocal(filter, xml.NewHash("job","status","done"))
  } else {
    p, err := strconv.Atoi(progress)
    if err == nil {
      ad,_ := strconv.Atoi(ALMOST_DONE_PROGRESS)
      if p > ad { 
        go processing_finished_watcher(macaddress, xmlmsg.Text("source"))
      }
    }
  }
}

var nextID = util.Counter(0)
var mutex sync.Mutex
var watchers = map[string]uint64{}

func processing_finished_watcher(macaddress, client_addr string) {
  id := <-nextID
  mutex.Lock()
  watchers[client_addr] = id
  mutex.Unlock()
  
  var err error
  var conn net.Conn
  for i := 0; i < 30; i++ {
    conn, err = net.Dial("tcp", client_addr)
    if err == nil { conn.Close() }
    mutex.Lock()
    quit := (watchers[client_addr] != id)
    mutex.Unlock()
    if quit { return }
    if err != nil { break }
    time.Sleep(10*time.Second)
  }
  
  processing := xml.FilterSimple("siserver",   config.ServerSourceAddress, 
                                 "status",    "processing",
                                 "macaddress", macaddress)
  progress := xml.FilterRel("progress", ALMOST_DONE_PROGRESS, 1, 1)
  filter := xml.FilterAnd([]xml.HashFilter{processing, progress})
  if db.JobsQuery(filter).FirstChild() != nil { // if we have stalled jobs
    util.Log(0, "WARNING! Client %v did not report progress 100%% => Removing stalled jobs (Triggered by %v)", macaddress, err)
    db.JobsModifyLocal(filter, xml.NewHash("job","status","done"))
  }
}
