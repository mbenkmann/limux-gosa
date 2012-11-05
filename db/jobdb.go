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
         "strconv"
         "strings"
         
         "../xml"
         "../config"
         "../util"
       )

// Stores jobs to be executed at some point in the future.
// Format of the database:
//   <jobdb>
//     <job>
//       <plainname>grisham</plainname>
//       <progress>none</progress>
//       <status>done</status>
//       <siserver>1.2.3.4:20081</siserver>
//       <modified>1</modified>
//       <targettag>00:0c:29:50:a3:52</targettag>
//       <macaddress>00:0c:29:50:a3:52</macaddress>
//       <timestamp>20120906164734</timestamp>
//       <periodic>7_days</periodic>
//       <id>2</id>
//       <original_id>4</original_id>
//       <headertag>trigger_action_wake</headertag>
//       <result>none</result>
//       <xmlmessage>PHhtbD48aGVhZGVyPmpvYl90cmlnZ2VyX2FjdGlvbl93YWtlPC9oZWFkZXI+PHNvdXJjZT5HT1NBPC9zb3VyY2U+PHRhcmdldD4wMDowYzoyOTo1MDphMzo1MjwvdGFyZ2V0Pjx0aW1lc3RhbXA+MjAxMjA5MDYxNjQ3MzQ8L3RpbWVzdGFtcD48bWFjYWRkcmVzcz4wMDowYzoyOTo1MDphMzo1MjwvbWFjYWRkcmVzcz48L3htbD4=</xmlmessage>
//     </job>
//     <job>
//       ...
//     </job>
//   </jobdb>
var jobDB *xml.DB

// When an action on the database requires sending updates to peers, they are
// queued in this channel. Every message is a complete foreign_job_updates message
// as per the protocol (i.e. <xml><header>foreign_job_updates</header>...</xml>)
// <target> is always present but may be the empty string. This means that the
// message should be sent to ALL peers. Otherwise the <target> is the single peer
// the message should be sent to.
// The large size of the buffer is to make sure we don't delay even if a couple
// 1000 machines are sending us progress updates at the same time (which we need to
// forward to our peers).
// The code that reads from this channel and forwards the messages to the
// appropriate peers is in peer_connection.go:init()
//
// NOTE: A message in this queue may have an empty tag <SyncNonGoSusi> attached
//       at the level of <source>. In this case, PeerConnection.SyncNonGoSusi() will
//       be called after delivery of the foreign_job_updates to the peer.
//       <SyncNonGoSusi> is only permitted if <target> is non-empty. It will not
//       be transmitted as part of the foreign_job_updates.
//       A second effect of <SyncNonGoSusi> is that if the <target> is not a go-susi,
//       instead of sending the foreign_job_updates just to the target, it will be
//       sent to all known peers. This is done to compensate for the fact that
//       unlike go-susi gosa-si does not rebroadcast changes to its jobdb when those
//       changes are the result of foreign_job_updates.
//       The <SyncNonGoSusi> tag is used when forwarding change requests for
//       jobs belongting to other servers to them via foreign_job_updates.
var ForeignJobUpdates = make(chan *xml.Hash, 16384)

// Fields that can be updated via Jobs*Modify*()
var updatableFields = []string{"progress", "status", "periodic", "timestamp", "result"}

// A packaged request to perform some action on the jobDB.
// Most db.Job...() functions are just stubs that push
// a jobDBRequest into the jobDBRequests channel and then return.
type jobDBRequest struct {
  // The function to execute. It is passed a pointer to its own request.
  Action func(request *jobDBRequest)
  // Selects the jobs to act upon.
  Filter xml.HashFilter
  // Additional job data to be added in whole or part to the database.
  Job *xml.Hash
  // If a reply is necessary, it will be transmitted over this channel.
  Reply chan *xml.Hash
}

// The incoming requests handled by a single goroutine running 
// handleJobDBRequests().
// The large size of the buffer is to make sure we don't delay even if a couple
// 1000 machines are sending us progress updates at the same time.
var jobDBRequests = make(chan *jobDBRequest, 16384)

// The next number to use for <id> when storing a new local job.
var nextID chan uint64

// Initializes JobDB with data from the file config.JobDBPath if it exists and
// starts the goroutine that runs handleJobDBRequests.
// Not an init() because main() needs to set up some things first.
func JobsInit() {
  if jobDB != nil { panic("JobsInit() called twice") }
  jobdb_storer := &LoggingFileStorer{xml.FileStorer{config.JobDBPath}}
  var delay time.Duration = 0
  jobDB = xml.NewDB("jobdb", jobdb_storer, delay)
  if !config.FreshDatabase {
    xml, err := xml.FileToHash(config.JobDBPath)
    if err != nil {
      if os.IsNotExist(err) { 
        /* File does not exist is not an error that needs to be reported */ 
      } else
      {
        util.Log(0, "ERROR! JobsInit() reading '%v': %v", config.JobDBPath, err)
      }
    } else
    {
      jobDB.Init(xml)
    }
  }
  
  var count uint64 = 0
  for job := jobDB.Query(xml.FilterAll).First("job"); job != nil; job = job.Next() {
    id, err := strconv.ParseUint(job.Text("id"), 10, 64)
    if err != nil { panic(err) }
    if id > count { count = id }
  }
  nextID = util.Counter(count+1)
  
  go handleJobDBRequests()
}

// This function runs in a single goroutine and is responsible for handling
// all actions that affect the jobDB as well as starting local jobs whose time
// has come.
// The general idea behind synchronized job processing is this:
//  * a single goroutine processes all requests that affect the jobDB and pushes
//    the resulting changes into the ForeignJobUpdates queue
//  * a single goroutine processes the items from ForeignJobUpdates and passes them
//    on to the appropriate PeerConnection(s).
//  * each PeerConnection has a single goroutine forwarding the updates over a 
//    single TCP connection to its respective peer.
//  * The above ensures that each peer receives foreign_job_updates messages in
//    exactly the same order in which the corresponding edits are made on the jobDB.
//    This makes sure that a peer that applies foreign_job_updates messages in
//    order will always have a consistent jobdb.
//  * Because <sync>all</sync> messages are prepared by the same single goroutine
//    that performs the edits and creates <sync>ordered</sync> messages and because
//    all these messages go over the same channels, they cannot overtake each other
//    and will always fit together.
func handleJobDBRequests() {
  var request *jobDBRequest
  for {
    select {
      case request = <-jobDBRequests : request.Action(request)
      
        
  /*TODO: start local jobs whose time has come.
  When a periodic job is done, a new job is created with a new id. */

    }
  }
}

// Returns all jobs matching filter (as clones, not references into the database).
func JobsQuery(filter xml.HashFilter) *xml.Hash {
  query := func(request *jobDBRequest) {
    request.Reply <- jobDB.Query(request.Filter)
  }
  reply := make(chan *xml.Hash, 1)
  jobDBRequests <- &jobDBRequest{ query, filter, nil, reply }
  return <-reply
}

// Tries to remove all jobs matching the given filter, treating local and
// foreign jobs differently. Local jobs are removed directly and a
// foreign_job_updates message is broadcast for them. 
// Foreign jobs on the other hand are not removed immediately, but a
// foreign_job_updates with status=done and periodic=none is sent either to
// the affected server(s) only (if it is a go-susi) or broadcast to all peers (if
// the affected server is not go-susi). If the affected server is a go-susi it
// will upon receipt of the message remove the job and send a foreign_job_updates,
// which will then cause the job to be deleted from our database.
// To deal with non-go-susi servers, we wait a few seconds and then
// actively query the affected server's database.
func JobsRemove(filter xml.HashFilter) {
  update := xml.NewHash("job","status", "done")
  update.Add("periodic", "none")
  JobsModify(filter, update)
}

// Tries to update all the updatableFields (see var further above in this file)
// of the jobs selected by filter with the respective values from the update data.
// Fields not present in update are left unchanged.
// Local and foreign jobs are treated differently.
// Local jobs are modified directly and a foreign_job_updates message is broadcast
// for them.
// Foreign jobs on the other hand are not modified directly, but instead a
// foreign_job_updates message with the changed job data is sent either to
// the affected server(s) only (if it is a go-susi) or broadcast to all peers (if
// the affected server is not go-susi). If the affected server is a go-susi it
// will upon receipt of the message modify the job and send a foreign_job_updates,
// which will then cause the job to be modified in our database.
// To deal with non-go-susi servers, we wait a few seconds and then
// actively query the affected server's database.
func JobsModify(filter xml.HashFilter, update *xml.Hash) {
  JobsModifyLocal(xml.FilterAnd([]xml.HashFilter{xml.FilterSimple("siserver",config.ServerSourceAddress),filter}), update)
  JobsForwardModifyRequest(xml.FilterAnd([]xml.HashFilter{xml.FilterNot(xml.FilterSimple("siserver",config.ServerSourceAddress)),filter}), update)
}

// Goes through all jobs selected by the filter and sends to all the responsible
// servers a foreign_job_updates request that incorporates the changes to
// all the updatableFields (see var further above in this file) from the
// respective fields in the update data. Fields not present in update are left
// unchanged.
// For go-susi peers the above is all that's required, because they will inform
// us (and everyone else) of any changes they actually apply. For non-go-susi
// servers, we broadcast the foreign_job_updates to everyone and 
// schedule a full sync with the responsible server after a few seconds delay.
//
// NOTE: The filter must include a siserver!=config.ServerSourceAddress check,
//       so that it will not affect local jobs.
func JobsForwardModifyRequest(filter xml.HashFilter, update *xml.Hash) {
  modifyjobs := func(request *jobDBRequest) {
    jobdb_xml := jobDB.Query(request.Filter)
    count := make(map[string]uint64)
    fju   := make(map[string]*xml.Hash)
    for _, tag := range jobdb_xml.Subtags() {
      // Use RemoveFirst() so that we can use Rename() and AddWithOwnership()
      for job := jobdb_xml.RemoveFirst(tag); job != nil; job = jobdb_xml.RemoveFirst(tag) {
        siserver := job.Text("siserver")
        if count[siserver] == 0 { 
          count[siserver] = 1 
          fju[siserver] = xml.NewHash("xml","header","foreign_job_updates")
        }
        
        for _, field := range updatableFields {
          x := request.Job.First(field)
          if x != nil {
            job.FirstOrAdd(field).SetText(x.Text())
          }
        }
        job.Rename("answer"+strconv.FormatUint(count[siserver], 10))
        job.First("id").SetText(job.RemoveFirst("original_id").Text())
        count[siserver]++
        fju[siserver].AddWithOwnership(job)
      }
    }
    
    for siserver := range fju {
      fju[siserver].Add("source", config.ServerSourceAddress)
      fju[siserver].Add("target", siserver) // affected by SyncNonGoSusi!
      fju[siserver].Add("sync", "ordered")
      fju[siserver].Add("SyncNonGoSusi") // see doc at var ForeignJobUpdates
      ForeignJobUpdates <- fju[siserver]
    }
  }
  
  jobDBRequests <- &jobDBRequest{ modifyjobs, filter, update.Clone(), nil }
}


// Adds a local job to the database (i.e. a job to be executed by this server), 
// creating a new id for it that will be both
// <id> and <original_id>. Multiple jobs with the same <headertag>/<macaddress>
// combination are permitted.
// Calling this method triggers a foreign_job_updates broadcast.
//   job: Has the following format 
//        <job>
//          <headertag>trigger_action_wake</headertag>
//          <macaddress>00:0c:29:50:a3:52</macaddress>
//          ...
//        </job>
//
// NOTE: This function expects job to be complete and well formed. No error 
//       checking is performed on the data.
//       The job added to the database will be a clone, however the <id> and
//       <original_id> will be attached to the original job hash (and so can
//       be used by the caller).
func JobAddLocal(job *xml.Hash) {
  id := strconv.FormatUint(<-nextID, 10)
  job.FirstOrAdd("id").SetText(id)
  job.FirstOrAdd("original_id").SetText(id)
  
  addjob := func(request *jobDBRequest) {
    jobDB.AddClone(request.Job)
    request.Job.Rename("answer1")
    fju := xml.NewHash("xml","header","foreign_job_updates")
    fju.Add("source", config.ServerSourceAddress)
    fju.Add("target") // empty target => all peers
    fju.Add("sync", "ordered")
    request.Job.RemoveFirst("original_id")
    fju.AddWithOwnership(request.Job)
    ForeignJobUpdates <- fju
  }
  // Note: We need to use job.Clone() here even though addjob() uses
  // jobDB.AddClone() and thereby creates another clone. If we attached job
  // to the request directly, the caller might change the job
  // before it has been processed.
  jobDBRequests <- &jobDBRequest{ addjob, nil, job.Clone(), nil }
}

// Removes from the JobDB the jobs matching filter.
// Calling this method triggers a foreign_job_updates broadcast (if at least
// 1 job was removed).
//
// NOTE: The filter must include the siserver==config.ServerSourceAddress check,
// so that it only affects local jobs.
func JobsRemoveLocal(filter xml.HashFilter) {
  deljob := func(request *jobDBRequest) {
    jobdb_xml := jobDB.Remove(request.Filter)
    fju := xml.NewHash("xml","header","foreign_job_updates")
    var count uint64 = 1
    for _, tag := range jobdb_xml.Subtags() {
      // Use RemoveFirst() so that we can use Rename() and AddWithOwnership()
      for job := jobdb_xml.RemoveFirst(tag); job != nil; job = jobdb_xml.RemoveFirst(tag) {
        job.FirstOrAdd("status").SetText("done")
        job.FirstOrAdd("periodic").SetText("none")
        job.RemoveFirst("original_id")
        job.Rename("answer"+strconv.FormatUint(count, 10))
        count++
        fju.AddWithOwnership(job)
      }
    }
    
    if count > 1 {
      fju.Add("source", config.ServerSourceAddress)
      fju.Add("target") // empty target => all peers
      fju.Add("sync", "ordered")
      ForeignJobUpdates <- fju
    }
  }
  jobDBRequests <- &jobDBRequest{ deljob, filter, nil, nil }
}

// Updates the updatableFields (see var further up in this file) 
// of all jobs selected by filter with the respective values from update. 
// Fields not present in update are left unchanged.
// Calling this method triggers a foreign_job_updates broadcast (if at least
// 1 job was modified).
//
// NOTE: The filter must include the siserver==config.ServerSourceAddress check,
// so that it only affects local jobs.
//
// NOTE: If update has status=="done", this call is equivalent to
//       JobsRemoveLocal(filter)
func JobsModifyLocal(filter xml.HashFilter, update *xml.Hash) {
  if update.Text("status") == "done" {
    JobsRemoveLocal(filter)
    return
  }
  
  modifylocaljobs := func(request *jobDBRequest) {
    jobdb_xml := jobDB.Query(request.Filter)
    fju := xml.NewHash("xml","header","foreign_job_updates")
    var count uint64 = 1
    for _, tag := range jobdb_xml.Subtags() {
      // Use RemoveFirst() so that we can use Rename() and AddWithOwnership()
      for job := jobdb_xml.RemoveFirst(tag); job != nil; job = jobdb_xml.RemoveFirst(tag) {
        for _, field := range updatableFields {
          x := request.Job.First(field)
          if x != nil {
            job.FirstOrAdd(field).SetText(x.Text())
          }
        }
        jobDB.Replace(xml.FilterSimple("id", job.Text("id")), true, job)
        job.Rename("answer"+strconv.FormatUint(count, 10))
        count++
        fju.AddWithOwnership(job)
      }
    }
    
    if count > 1 {
      fju.Add("source", config.ServerSourceAddress)
      fju.Add("target") // empty target => all peers
      fju.Add("sync", "ordered")
      ForeignJobUpdates <- fju    
    }
  }
  
  jobDBRequests <- &jobDBRequest{ modifylocaljobs, filter, update.Clone(), nil }
}

// Removes the jobs selected by filter without further actions (in particular no
// foreign_job_updates will be broadcast). Therefore this function must only be
// used with a filter that checks siserver to avoid local jobs.
func JobsRemoveForeign(filter xml.HashFilter) {
  deljob := func(request *jobDBRequest) {
    jobDB.Remove(request.Filter)
  }
  jobDBRequests <- &jobDBRequest{ deljob, filter, nil, nil }
}

// If no job matching filter is in the jobDB, job is added to it (using its
// <id> as <original_id> and generating a new <id>).
// If one or more jobs match the filter, their updatableFields (see var further
// above in this file) are updated
// with the respective values from job. 
// Fields not present in job are left unchanged.
//
// NOTE: No foreign_job_updates will be broadcast. Therefore this function must only be
//       used with a filter that checks siserver to avoid local jobs.
//
// NOTE: If job has status=="done", this call is equivalent to
//       JobsRemoveForeign(filter)
func JobsAddOrModifyForeign(filter xml.HashFilter, job *xml.Hash) {
  if job.Text("status") == "done" {
    JobsRemoveForeign(filter)
    return
  }
  
  addmodify := func(request *jobDBRequest) {
    found := jobDB.Query(request.Filter).First("job")
    
    if found != nil { // if jobs found => update their fields
      for ; found != nil; found = found.Next() {
        for _, field := range updatableFields {
          x := request.Job.First(field)
          if x != nil {
            found.FirstOrAdd(field).SetText(x.Text())
          }
        }
        
        jobDB.Replace(xml.FilterSimple("id", found.Text("id")), true, found)
      }
    } else // no job matches filter => add job
    {
      job := request.Job
      job.FirstOrAdd("original_id").SetText(job.Text("id"))
      job.FirstOrAdd("id").SetText("%d", <-nextID)
      plainname := SystemNameForMAC(job.Text("macaddress"))
      job.FirstOrAdd("plainname").SetText(plainname)
      jobDB.AddClone(job)  
    }
  }
  jobDBRequests <- &jobDBRequest{ addmodify, filter, job, nil }
}

// Sends a foreign_job_updates message to target containing all local
// jobs (<sync>all</sync>). If old != nil it must be the reply of target
// to a gosa_query_jobdb asking for all jobs with
// siserver==config.ServerSourceAddress. If any of the old jobs (as identified
// by the combination headertag/macaddress) has no counterpart in the current
// list of local jobs, then it will be added to the foreign_job_updates message
// with <status>none</status><periodic>none</periodic>.
//
// If there are no jobs to update, no fju will be sent.
func JobsSyncAll(target string, old *xml.Hash) {
  if old == nil { old = xml.NewHash("xml") }
  fju := old.Clone()
  fju.FirstOrAdd("header").SetText("foreign_job_updates")
  fju.FirstOrAdd("target").SetText(target)
  fju.FirstOrAdd("source").SetText(config.ServerSourceAddress)
  fju.Add("sync", "all")
  
  syncall := func(request *jobDBRequest) {
    fju := request.Job
    
    // Get current list of LOCAL jobs into myjobs
    myjobs := jobDB.Query(xml.FilterSimple("siserver",config.ServerSourceAddress))
    
    // Remove all old jobs from fju that have a counterpart in the current list
    for _, tag := range myjobs.Subtags() {
      for job := myjobs.First(tag); job != nil; job = job.Next() {
        fju.Remove(xml.FilterSimple("headertag",job.Text("headertag"),"macaddress", job.Text("macaddress")))
      }
    }
    
    // Next set all remaining old jobs in fju to "done" non-periodic and renumber them.
    // Note that the following loop is a bit tricky, because the renumbered
    // <answerX> may be the same as an existing and yet unprocessed <answerX>.
    // The reason why this works is because AddWithOwnership()'s contract says that it
    // will always make the renumbered job the LAST child with name <answerX>.
    // So when we finally reach the unrenumbered "answerX" in the subtags list,
    // RemoveFirst() will reliably remove only the unrenumbered job.
    var count uint64 = 1
    for _, tag := range fju.Subtags() {
      if !strings.HasPrefix(tag,"answer") { continue }
      job := fju.RemoveFirst(tag)
      job.FirstOrAdd("status").SetText("done")
      job.FirstOrAdd("periodic").SetText("none")
        // If the target is an as-yet unidentified go-susi, it would
        // use the id to identify the job. So make sure it has a unique
        // one and won't accidentally interfere with some other job.
        // gosa-si doesn't care about the id. It uses headertag+macaddress.
      job.FirstOrAdd("id").SetText("%d", <-nextID)
      job.Rename("answer"+strconv.FormatUint(count, 10))
      count++
      fju.AddWithOwnership(job)
    }
    
    // Now add the jobs from myjobs to fju.
    for _, tag := range myjobs.Subtags() {
      // Use RemoveFirst() so that we can use Rename() and AddWithOwnership()
      for job := myjobs.RemoveFirst(tag); job != nil; job = myjobs.RemoveFirst(tag) {
        job.RemoveFirst("original_id")
        job.Rename("answer"+strconv.FormatUint(count, 10))
        count++
        fju.AddWithOwnership(job)
      }
    }
    
    if count > 1 {
      ForeignJobUpdates <- fju
    }
  }
  
  jobDBRequests <- &jobDBRequest{ syncall, nil, fju, nil }
}

// Creates a GUID from the ip:port address addr and the number num.
// An illegal address will cause a panic.
func JobGUID(addr string, num uint64) string {
  host, port, err := net.SplitHostPort(addr)
  if err != nil { panic(err) }
  ip := net.ParseIP(host).To4()
  if ip == nil { panic("Not an IPv4 address") }
  p, err := strconv.ParseUint(port, 10, 64)
  if err != nil { panic(err) }
  var n uint64 = uint64(ip[0]) + uint64(ip[1])<<8 + uint64(ip[2])<<16 + 
                 uint64(ip[3])<<24 + p<<32
  return strconv.FormatUint(num,10)+strconv.FormatUint(n,10)
}

