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
         "fmt"
         "net"
         "time"
         "strconv"
         "strings"
         "encoding/base64"
         
         "../xml"
         "../config"
         "../util"
         "../util/deque"
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
// <target> is always present and the following types of <target> are supported:
//   <target></target>  an empty target means that the message should be sent to 
//                      ALL peers. 
//   <target>gosa-si</target> the target should be sent to all peers that are
//                      not known to be go-susi (IOW presumed gosa-si peers).
//   <target>...</target> Otherwise the <target> is the single peer
//                      the message should be sent to.
// The large size of the buffer is to make sure we don't delay even if a couple
// 1000 machines are sending us progress updates at the same time (which we need to
// forward to our peers).
// The code that reads from this channel and forwards the messages to the
// appropriate peers is in peer_connection.go:DistributeForeignJobUpdates()
//
// NOTE: A message in this queue may have an empty tag <SyncIfNotGoSusi> attached
//       at the level of <source>. In this case, PeerConnection.SyncIfNotGoSusi() will
//       be called after delivery of the foreign_job_updates to the peer.
//       <SyncIfNotGoSusi> is only permitted if <target> is a specific peer. It will not
//       be transmitted as part of the foreign_job_updates.
//       A second effect of <SyncIfNotGoSusi> is that if the <target> is not a go-susi,
//       instead of sending the foreign_job_updates just to the target, it will be
//       sent to all known peers. This is done to compensate for the fact that
//       unlike go-susi gosa-si does not rebroadcast changes to its jobdb when those
//       changes are the result of foreign_job_updates.
//       The <SyncIfNotGoSusi> tag is used when forwarding change requests for
//       jobs belonging to other servers to them via foreign_job_updates.
var ForeignJobUpdates = make(chan *xml.Hash, 16384)

// Stores exactly 1 time.Time that indicates when the most recent job modification
// request was forwarded to a peer. A Deque is used only for its synchronization.
// Access is permitted with At() and Put() only.
var MostRecentForwardModifyRequestTime = deque.New([]interface{}{time.Now().Add(-1*time.Hour)}, deque.DropFarEndIfOverflow)

// Fields that can be updated via Jobs*Modify*()
var updatableFields = []string{"progress", "status", "periodic", "timestamp", "result"}

// A packaged request to perform some action on the jobDB.
// Most db.Job...() functions attach their core code to a jobDBRequest,
// push that into the jobDBRequests channel and then return.
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

// a ping on this channel causes handleJobDBRequests() to push
// all jobs whose time has come into the PendingActions queue (see below).
var processPendingActions = make(chan bool)

// Whenever the time of a local job with status "waiting" has come it is 
// put into this queue after its status has been changed to "processing".
// Whenever a local job is removed, it is put into this queue after
// its status has been changed to "done". 
// The consumer of this queue is action/process_act.go:init()
var PendingActions = deque.New()

// The next number to use for <id> when storing a new local job.
var nextID chan uint64

// Initializes JobDB with data from the file config.JobDBPath if it exists and
// starts the goroutine that runs handleJobDBRequests.
// Not an init() because main() needs to set up some things first.
func JobsInit() {
  if jobDB != nil { panic("JobsInit() called twice") }
  jobdb_storer := &LoggingFileStorer{xml.FileStorer{config.JobDBPath}}
  var delay time.Duration = config.DBPersistDelay
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
  
  // Remove all non-local jobs (they may be stale and new_server
  // causes full sync anyway)
  jobDB.Remove(xml.FilterNot(xml.FilterSimple("siserver",config.ServerSourceAddress)))
  
  // The following loop goes through all jobs and does the following
  // * find the the greatest id number used in the db
  // * schedule processing of pending actions for all timestamps
  var count uint64 = 0
  for job := jobDB.Query(xml.FilterAll).First("job"); job != nil; job = job.Next() {
    id, err := strconv.ParseUint(job.Text("id"), 10, 64)
    if err != nil { panic(err) }
    if id > count { count = id }
    
    scheduleProcessPendingActions(job.Text("timestamp"))
  }
  nextID = util.Counter(count+1)
  
  go handleJobDBRequests()
}

// Persists the jobDB and prevents all further changes to it.
// This function does not return until the database has been persisted.
func JobsShutdown() {
  util.Log(1, "INFO! Shutting down job database")
  jobDB.Shutdown()
  util.Log(1, "INFO! Job database has been saved")
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
  groom_ticker := time.Tick(config.JobDBGroomInterval)
  var request *jobDBRequest
  for {
    select {
      case request = <-jobDBRequests : request.Action(request)
      
      case _ = <-groom_ticker: go util.WithPanicHandler(groomJobDB)
      
      case _ = <-processPendingActions :
               /*** WARNING! WARNING! ***
               Using the function JobsQuery() here will cause deadlock!
               Other functions like JobsModifyLocal() are okay, but remember
               that they will not be executed until this case ends.
               *************************/
               localwait := xml.FilterSimple("siserver", config.ServerSourceAddress, 
                                             "status", "waiting")
               beforenow := xml.FilterRel("timestamp", util.MakeTimestamp(time.Now()), -1, 0)
               filter := xml.FilterAnd([]xml.HashFilter{localwait,beforenow})
               JobsModifyLocal(filter, xml.NewHash("job","status","launch"))
    }
  }
}

// Schedules handleJobDBRequests() to scan jobDB for jobs whose time has come.
// at the time specified by the timestamp argument.
func scheduleProcessPendingActions(timestamp string) {
  go func() {
    util.WaitUntil(util.ParseTimestamp(timestamp))
    processPendingActions <- true
  }()
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
    util.Log(2, "DEBUG! JobsForwardModifyRequest applying %v to %v", request.Job, jobdb_xml)
    count := make(map[string]uint64)
    fju   := make(map[string]*xml.Hash)
    for child := jobdb_xml.FirstChild(); child != nil; child = child.Next() {
        job := child.Remove()
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
        JobUpdateXMLMessage(job)
        count[siserver]++
        fju[siserver].AddWithOwnership(job)
    }
    
    for siserver := range fju {
      fju[siserver].Add("source", config.ServerSourceAddress)
      fju[siserver].Add("target", siserver) // affected by SyncIfNotGoSusi!
      fju[siserver].Add("sync", "ordered")
      fju[siserver].Add("SyncIfNotGoSusi") // see doc at var ForeignJobUpdates
      ForeignJobUpdates <- fju[siserver]
    }
    
    if len(fju) > 0 {
      // Note: The timestamp is updated even if all siservers to which
      // we forwarded are go-susi. While a go-susi peer does not require
      // a full sync, we do need to wait a little until we receive
      // the foreign_job_updates message from the peer.
      // Because go-susi is faster than gosa-si, we could
      // improve responsiveness of gosa_query_jobdb a little if
      // we checked whether fju contains only go-susi peers and
      // if that is the case instead of setting the timestamp to
      // time.Now() we could use time.Now.Add(-1*time.Second) to
      // make the wait 1s less.
      MostRecentForwardModifyRequestTime.Put(0, time.Now())
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
// NOTE: This function expects job to be complete and well formed (except for
//       plainname which will be automatically filled in if "none" or missing
//       and xmlmessage which will be re-generated).
//       No error checking is performed on the data.
//       The job added to the database will be a clone, however the <id> and
//       <original_id> will be attached to the original job hash (and so can
//       be used by the caller).
func JobAddLocal(job *xml.Hash) {
  id := strconv.FormatUint(<-nextID, 10)
  job.FirstOrAdd("id").SetText(id)
  job.FirstOrAdd("original_id").SetText(id)
  
  addjob := func(request *jobDBRequest) {
    plainname := request.Job.Text("plainname")
    if plainname == "" { 
      plainname = "none"
      request.Job.FirstOrAdd("plainname").SetText("none")
    }
    if plainname == "none" { 
      JobsUpdateNameForMAC(request.Job.Text("macaddress")) 
    }
    util.Log(1, "INFO! New job for me to execute: %v", request.Job)
    JobUpdateXMLMessage(request.Job)
    jobDB.AddClone(request.Job)
    scheduleProcessPendingActions(request.Job.Text("timestamp"))
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
// If stop_periodic == true, then the job's periodic state will be forced to none
// and no follow-up job will be scheduled. If stop_periodic == false, a follow-up
// job will be scheduled if the job is periodic.
//
// NOTE: The filter must include the siserver==config.ServerSourceAddress check,
// so that it only affects local jobs.
func JobsRemoveLocal(filter xml.HashFilter, stop_periodic bool) {
  deljob := func(request *jobDBRequest) {
    jobdb_xml := jobDB.Remove(request.Filter)
    util.Log(2, "DEBUG! JobsRemoveLocal(stop_periodic=%v) removing job(s): %v", stop_periodic, jobdb_xml)
    fju := xml.NewHash("xml","header","foreign_job_updates")
    var count uint64 = 1
    for child := jobdb_xml.FirstChild(); child != nil; child = child.Next() {
        job := child.Remove()
        job.FirstOrAdd("status").SetText("done")
        if stop_periodic {
          job.FirstOrAdd("periodic").SetText("none")
        }
        job.RemoveFirst("original_id")
        JobUpdateXMLMessage(job)
        PendingActions.Push(job.Clone())
        job.Rename("answer"+strconv.FormatUint(count, 10))
        count++
        fju.AddWithOwnership(job)
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
//         JobsRemoveLocal(filter, stop_periodic)
//       where stop_periodic is true iff update has a <periodic> element
//       that is empty or "none".
//
//       If update has status=="launch", the matching jobs will be
//       set to status "processing" and
//       will be pushed into the PendingActions queue. 
//       This will cause the job's action to be performed asap.
func JobsModifyLocal(filter xml.HashFilter, update *xml.Hash) {
  if update.Text("status") == "done" {
    stop_periodic := 
                ( update.Get("periodic") != nil && 
                  ( update.Text("periodic") == "" || 
                    update.Text("periodic") == "none" ) )
    JobsRemoveLocal(filter, stop_periodic)
    return
  }
  
  modifylocaljobs := func(request *jobDBRequest) {
    jobdb_xml := jobDB.Query(request.Filter)
    util.Log(2, "DEBUG! JobsModifyLocal applying %v to %v", request.Job, jobdb_xml)
    fju := xml.NewHash("xml","header","foreign_job_updates")
    var count uint64 = 1
    for child := jobdb_xml.FirstChild(); child != nil; child = child.Next() {
        job := child.Remove()
        for _, field := range updatableFields {
          x := request.Job.First(field)
          if x != nil {
            if field == "status" && x.Text() == "launch" {
              job.FirstOrAdd(field).SetText("processing")
              if job.Text("headertag") == "trigger_action_reinstall" || job.Text("headertag") == "trigger_action_update" {
                // Only one install or update job can be in status "processing" for the same machine at the same time,
                // so remove all other local install and update jobs in status "processing".
                // NOTE: If 2 jobs launch at exactly the same time (no matter if they are the same
                // kind or not), actions will be taken for both jobs, but they will both be
                // deleted from the jobdb, even if they are install or update jobs. This is
                // because JobsRemoveLocal() is asynchronous.
                // One could see this as a bug, in particular in the case where 2 identical jobs
                // are planned at the same time. However I don't see a reason to fix this ATM.
                util.Log(1, "INFO! New %v job => Removing other reinstall/update jobs currently processing for %v", job.Text("headertag"), job.Text("macaddress"))
                local_processing := xml.FilterSimple("siserver", config.ServerSourceAddress, "macaddress", job.Text("macaddress"), "status", "processing")
                install_or_update := xml.FilterOr([]xml.HashFilter{xml.FilterSimple("headertag", "trigger_action_reinstall"),xml.FilterSimple("headertag", "trigger_action_update")})
                not_the_new_job := xml.FilterNot(xml.FilterSimple("id", job.Text("id")))
                JobsRemoveLocal(xml.FilterAnd([]xml.HashFilter{local_processing, install_or_update, not_the_new_job}), false)
              }
              util.Log(1, "INFO! Launching job: %v",job)
              PendingActions.Push(job.Clone()) 
            } else {
              job.FirstOrAdd(field).SetText(x.Text())
            }
            
            if field == "timestamp" {
              scheduleProcessPendingActions(job.Text("timestamp"))
            }
          }
        }
        JobUpdateXMLMessage(job)
        jobDB.Replace(xml.FilterSimple("id", job.Text("id")), true, job)
        job.RemoveFirst("original_id")
        job.Rename("answer"+strconv.FormatUint(count, 10))
        count++
        fju.AddWithOwnership(job)
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
        
        JobUpdateXMLMessage(found)
        jobDB.Replace(xml.FilterSimple("id", found.Text("id")), true, found)
      }
    } else // no job matches filter => add job
    {
      job := request.Job
      job.Rename("job")
      job.FirstOrAdd("original_id").SetText(job.Text("id"))
      job.FirstOrAdd("id").SetText("%d", <-nextID)
      plainname := job.Text("plainname")
      if plainname == "" { 
        plainname = "none" 
        job.FirstOrAdd("plainname").SetText("none")
      }
      if plainname == "none" { 
        JobsUpdateNameForMAC(job.Text("macaddress")) 
      }
      JobUpdateXMLMessage(job)
      jobDB.AddClone(job)  
    }
  }
  
  jobDBRequests <- &jobDBRequest{ addmodify, filter, job.Clone(), nil }
}

// Launches a background job that queries the systemdb for the name of
// the machine with the given macaddress and if/when the answer arrives,
// schedules an update of all entries in the jobdb that match the macaddress.
func JobsUpdateNameForMAC(macaddress string) {
  updatename := func(request *jobDBRequest) {
    plainname := request.Job.Text("plainname")
    found := jobDB.Query(request.Filter).First("job")
    for ; found != nil; found = found.Next() {
      if found.Text("plainname") != plainname {
        found.FirstOrAdd("plainname").SetText(plainname)
        jobDB.Replace(xml.FilterSimple("id", found.Text("id")), true, found)
        
        // if the job is one of ours, then send out fju to gosa-si peers
        // because they don't look up the name themselves
        if found.Text("siserver") == config.ServerSourceAddress {
          fju := xml.NewHash("xml","header","foreign_job_updates")
          clone := found.Clone()
          clone.Rename("answer1")
          
          fju.AddWithOwnership(clone)
          
          fju.Add("source", config.ServerSourceAddress)
          fju.Add("target","gosa-si") // only gosa-si peers
          fju.Add("sync", "ordered")
          ForeignJobUpdates <- fju
        }
      }
    }
  }
  
  go util.WithPanicHandler(func(){
    filter := xml.FilterSimple("macaddress", macaddress)
    plainname := SystemPlainnameForMAC(macaddress)
    if plainname != "none" {
      job := xml.NewHash("job","plainname",plainname)
      jobDBRequests <- &jobDBRequest{ updatename, filter, job, nil }
    }
  })
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
    for child := myjobs.FirstChild(); child != nil; child = child.Next() {
      job := child.Element()
      fju.Remove(xml.FilterSimple("headertag",job.Text("headertag"),"macaddress", job.Text("macaddress")))
    }
    
    // Next set all remaining old jobs in fju to "done" non-periodic and renumber them.
    var count uint64 = 1
    for child := fju.FirstChild(); child != nil; child = child.Next() {
      if !strings.HasPrefix(child.Element().Name(),"answer") { continue }
      job := child.Element()
      job.FirstOrAdd("status").SetText("done")
      job.FirstOrAdd("periodic").SetText("none")
        // If the target is an as-yet unidentified go-susi, it would
        // use the id to identify the job. So make sure it has a unique
        // one and won't accidentally interfere with some other job.
        // gosa-si doesn't care about the id. It uses headertag+macaddress.
      job.FirstOrAdd("id").SetText("%d", <-nextID)
      job.Rename("answer"+strconv.FormatUint(count, 10))
      count++
    }
    
    // Now add the jobs from myjobs to fju.
    for child := myjobs.FirstChild(); child != nil; child = child.Next() {
      job := child.Remove()
      job.RemoveFirst("original_id")
      job.Rename("answer"+strconv.FormatUint(count, 10))
      count++
      fju.AddWithOwnership(job)
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

// Takes a job and adds or replaces the <xmlmessage> element with one that
// matches the job.
func JobUpdateXMLMessage(job *xml.Hash) {
  peri := job.Text("periodic")
  if peri == "" { peri = "none" }
  peri = "<periodic>" + peri + "</periodic>"
  xmlmess := fmt.Sprintf("<xml><source>GOSA</source><header>job_%v</header><target>%v</target><macaddress>%v</macaddress><timestamp>%v</timestamp>%v</xml>", job.Text("headertag"), job.Text("targettag"), job.Text("macaddress"), job.Text("timestamp"), peri)
  job.FirstOrAdd("xmlmessage").SetText(base64.StdEncoding.EncodeToString([]byte(xmlmess)))
}

// Checks the jobdb for stale jobs and cleans them up.
func groomJobDB() {
  util.Log(1, "INFO! Grooming jobdb")
  
  older_than_5_minutes := xml.FilterRel("timestamp", util.MakeTimestamp(time.Now().Add(-5*time.Minute)), -1, 0)
  jobs := JobsQuery(older_than_5_minutes)
  
  for jobi := jobs.FirstChild(); jobi != nil; jobi = jobi.Next() {
    job := jobi.Element()
    
    macaddress := job.Text("macaddress")
    siserver := job.Text("siserver")
    
    if (job.Text("headertag") == "trigger_action_reinstall" || 
        job.Text("headertag") == "trigger_action_update") && job.Text("status") == "processing" {
        // job is update or install job that is (believed to be) currently running
        
        var faistate string
        system, err := SystemGetAllDataForMAC(macaddress, false)
        if err == nil { faistate = system.Text("faistate") } else { faistate = err.Error() }
        
        state := (faistate + "12345")[0:5]
        if (job.Text("headertag") == "trigger_action_reinstall" && (state == "reins" || state == "insta")) || 
           (job.Text("headertag") == "trigger_action_update"    && (state == "updat" || state == "softu")) {
           // FAIstate matches job => OK
           continue
        }
        
        // FAIstate does not match job
        util.Log(0, "WARNING! FAIstate \"%v\" for %v is inconsistent with job that is supposedly processing: %v", faistate, macaddress, job)
        
        if siserver == config.ServerSourceAddress { // job is local => remove
          util.Log(1, "INFO! Removing inconsistent job: %v", job)
          JobsRemoveLocal(xml.FilterSimple("id", job.Text("id")), false)
        }
        
    } else { // whatever the job is, it shouldn't be like this.
             // It has apparently not been launched (or has not been removed after launching).
             
      if siserver == config.ServerSourceAddress {
        util.Log(0, "ERROR! Job has not launched. This is a bug. Please report. Job: %v", job)
        update := xml.NewHash("job", "status", "error")
        update.Add("result", "Job has not launched. This is a bug. Please report.")
        JobsModifyLocal(xml.FilterSimple("id", job.Text("id")), update)
        
      } else {
        util.Log(0, "WARNING! Peer %v has not taken action for job or not told us about it: %v", siserver, job)
      }
    }
  }
}
