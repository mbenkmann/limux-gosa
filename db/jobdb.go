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
// 1000 machines are sending us progess updates at the same time (which we need to
// forward to our peers).
// The code that reads from this channel and forwards the messages to the
// appropriate peers is in peer_connection.go:init()
var ForeignJobUpdates = make(chan *xml.Hash, 16384)

// A packaged request to perform some action on the jobDB.
// Most db.Job...() functions are just stubs that just push
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
var nextID chan uint64 = util.Counter(1)

// Initializes JobDB with data from the file config.JobDBPath if it exists and
// starts the goroutine that runs handleJobDBRequests.
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
//  * each PeerConnection has a single goroutine communicating over a single TCP
//    connection with its respective peer.
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
// foreign jobs differently. Local jobs are removed and a
// foreign_job_updates message is broadcast for them. The removal of
// foreign jobs is attempted by passing the request to the responsible
// siserver via gosa_delete_jobdb_entry.
func JobsRemove(filter xml.HashFilter) {
  JobsRemoveLocal(xml.FilterAnd([]xml.HashFilter{xml.FilterSimple("siserver",config.ServerSourceAddress),filter}))
  
  //FIXME: Placeholder code. Instead of removing the job from our database, we
  // should extract the original_id and siserver for all matching jobs and
  // use Peer(siserver).Ask(gosa_delete_jobdb_entry) to send a delete request to
  // responsible server. We DON'T remove the foreign job from our database! If
  // the foreign server reacts to the gosa_delete_jobdb_entry with the deletion
  // of the job, we will learn about this via a foreign_job_updates and that will
  // cause JobsRemoveForeign() to be called.
  JobsRemoveForeign(xml.FilterAnd([]xml.HashFilter{xml.FilterNot(xml.FilterSimple("siserver",config.ServerSourceAddress)),filter}))
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
// Calling this method triggers a foreign_job_updates broadcast.
//
// NOTE: The filter must include the siserver==config.ServerSourceAddress check,
// so that it only affects local jobs.
func JobsRemoveLocal(filter xml.HashFilter) {
  deljob := func(request *jobDBRequest) {
    jobdb_xml := jobDB.Remove(request.Filter)
    fju := xml.NewHash("xml","header","foreign_job_updates")
    var count uint64 = 1
    for _, tag := range jobdb_xml.Subtags() {
      for job := jobdb_xml.RemoveFirst(tag); job != nil; job = jobdb_xml.RemoveFirst(tag) {
        job.FirstOrAdd("status").SetText("done")
        job.FirstOrAdd("periodic").SetText("none")
        job.RemoveFirst("original_id")
        job.Rename("answer"+strconv.FormatUint(count, 10))
        count++
        fju.AddWithOwnership(job)
      }
    }
    fju.Add("source", config.ServerSourceAddress)
    fju.Add("target") // empty target => all peers
    fju.Add("sync", "ordered")
    ForeignJobUpdates <- fju
  }
  jobDBRequests <- &jobDBRequest{ deljob, filter, nil, nil }
}

// Fields that can be updated via JobsModifyLocal() and JobsAddOrModifyForeign()
var updatableFields = []string{"progress", "status", "periodic", "timestamp", "result"}

// Updates the fields <progress>, <status>, <periodic>, <timestamp> and <result>
// of all jobs selected by filter with the respective values from update. 
// Fields not present in update are left unchanged.
// Calling this method triggers a foreign_job_updates broadcast.
//
// NOTE: The filter must include the siserver==config.ServerSourceAddress check,
// so that it only affects local jobs.
func JobsModifyLocal(filter xml.HashFilter, update *xml.Hash) {
  
  
  //FIXME: This method is currently unsafe because it performs multiple
  //       jobDB accesses without proper synchronization.
  //       It will become goroutine-safe once the rewrite is done introducing
  //       the request queue.
  
  for job := jobDB.Query(filter).First("job"); job != nil; job = job.Next() {
    for _, field := range updatableFields {
      x := update.First(field)
      if x != nil {
        job.FirstOrAdd(field).SetText(x.Text())
      }
    }
    
    jobDB.Replace(xml.FilterSimple("id", job.Text("id")), true, job)
  }
  
  //FIXME: Missing: foreign_job_updates broadcast!
}

// Removes the jobs selected by filter without further actions (in particular no
// foreign_job_updates will be broadcast). Therefore this function must only be
// used with a filter that checks siserver to avoid local jobs.
func JobsRemoveForeign(filter xml.HashFilter) {
  jobDB.Remove(filter)
}

// If no job matching filter is in the jobDB, job is added to it (using its
// <id> as <original_id> and generating a new <id>).
// If one or more jobs match the filter, their fields 
// <progress>, <status>, <periodic>, <timestamp> and <result> are updated
// with the respective values from job. 
// Fields not present in job are left unchanged.
// No foreign_job_updates will be broadcast. Therefore this function must only be
// used with a filter that checks siserver to avoid local jobs.        
//
// NOTE: job may be modified by this function
func JobsAddOrModifyForeign(filter xml.HashFilter, job *xml.Hash) {
  
  
  //FIXME: This method is currently unsafe because it performs multiple
  //       jobDB accesses without proper synchronization.
  //       It will become goroutine-safe once the rewrite is done introducing
  //       the request queue.
  
  found := jobDB.Query(filter).First("job")
  
  if found != nil { // if jobs found => update their fields
    for ; found != nil; found = found.Next() {
      for _, field := range updatableFields {
        x := job.First(field)
        if x != nil {
          found.FirstOrAdd(field).SetText(x.Text())
        }
      }
      
      jobDB.Replace(xml.FilterSimple("id", found.Text("id")), true, found)
    }
  } else // no job matches filter => add job
  {
    job.FirstOrAdd("original_id").SetText(job.Text("id"))
    job.FirstOrAdd("id").SetText("%d", <-nextID)
    plainname := SystemNameForMAC(job.Text("macaddress"))
    job.FirstOrAdd("plainname").SetText(plainname)
    jobDB.AddClone(job)  
  }
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

/*  There are 6 different request types:
  1) trigger_action: add a job to be executed by this server
                     multiple jobs with the same headertag+mac are permitted
  2) query_jobdb: request an extract of jobs that match a filter
                  For foreign jobs uses PeerConnection.Downtime() to check
                  if the siserver is available (for consistency reasons only
                  1 check is done per siserver and cached for the request)
                  and if not sets the job to <status>error</status> with a <result>
                  saying that the responsible server is down (with a time how
                  long it has been down ("...has been down for 10h").
  3) modify_jobs: change some attributes of all jobs that match a filter
                  internally split up into subrequests for the different
                  siservers involved. Local jobs are modified directly.
                  Foreign jobs are modified by passing on the request
                  as gosa_update_status_jobdb_entry requests
                  When an existing job is modified, its GUID remains unchanged.
  4) delete_jobs: deletes all jobs matching a filter. Like modify_jobs this
                  is split into subrequests and foreign jobs are deleted via
                  gosa_delete_jobdb_entry on the foreign servers.
                  A job that is in state error because the server is down can not
                  be deleted until 48h have passed since the server went down.
                  This prevents overzealous admins from removing jobs from the
                  deployment status that other admins may want to see, but permits
                  jobs from servers that have been permanently decommissioned to
                  be removed eventually. After 7 days of downtime of a server, its
                  jobs are automatically removed from the list.
  5) foreign_job_update: (restriction: all jobs have the same siserver and have
                          <sync>ordered</sync> or <sync>all</sync>. f_j_u messages
                          that do not meet this requirement are handled
                          by message.Foreign_job_updates())
                          Open question: Should we accept f_j_u that try to change
                          our own jobs, i.e. the kind of f_j_u that are created when
                          a foreign job is deleted or modified on gosa-si? Such
                          a job would have to be translated to modify_jobs or
                          delete_jobs as appropriate and because it does not use
                          GUIDs would affect all jobs with the same combination
                          of headertag+mac. In either case we should schedule a full
                          sync for the requesting server so that it gets our
                          up-to-date jobdb (and recreates any jobs it has mistakenly
                          deleted). Of course this wont fix any misinformation on
                          other gosa-si servers that have blindly accepted the f_j_u,
                          but in the typical case with 1 gosa-si (production system)
                          and 1 go-susi (test system) it would work perfectly.
  6) send_full_sync: Creates a f_j_u message with <sync>all</sync> containing all
                     local jobs. The f_j_u is put into the ForeignJobUpdates queue
                     with a target corresponding to the target in the send_full_sync
                     request, so that the full sync is only sent to one peer rather
                     than all peers. If a full sync is sent to a non-goSusi peer, it
                     should be preceded by a gosa_delete_jobdb_entry that has a
                     <where> clause that selects all datasets where
                       siserver=me  AND
                       (headertag!=ht_of_my_1st_job or macaddress!=mac_of_my_1st_job) AND
                       (headertag!=ht_of_my_2st_job or macaddress!=mac_of_my_2st_job) AND
                       ...
                     which cleans up all old jobs the peer may still believe we have.
  */