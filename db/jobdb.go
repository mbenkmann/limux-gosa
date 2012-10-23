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
var jobDB *xml.DB

type jobDBRequest struct {
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
  Request *xml.Hash
  Reply chan *xml.Hash
}

/* buffered outgoing foreign_job_updates queue. handleJobDBRequests() puts
new messages into this queue. It guarantees that they are in the same order as
edits made on the database, so that when these messages are sent in order over
dedicated connections to the peer (i.e. <sync>ordered</sync>) the peer can apply
them in order and will be guaranteed to have a consistent jobdb at all times.
Each message has a <target> tag that specifies which peer it should go to.
Most messages have <target></target> (empty) to specify they should go to all
peers. But some (in particular those with <sync>all</sync> are only directed
at specifc peers.
*/
var ForeignJobUpdates chan *xml.Hash

// the incoming requests handled by handleJobDBRequests. This queue is buffered,
// but each request may contain an unbuffered Reply channel (although this should
// not usually be necessary).
var jobDBRequests chan jobDBRequest

// The next number to use for generating the next GUID for a local job.
var nextID chan uint64 = util.Counter(1)

// Initializes JobDB with data from the file config.JobDBPath if it exists.
func JobsInit() {
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
}

func handleJobDBRequests() {
/*  Runs in a goroutine an processes incoming messages on jobDBRequests channel.
  
  Also responsible for starting local jobs whose time has come.
  When a periodic job is done, a new job is created with a new GUID.  */
}

// Queries the JobDB according to where (see xml.WhereFilter() for the format)
// and returns the results (as clones, not references into the database).
func JobsQuery(where *xml.Hash) *xml.Hash {
  filter, err := xml.WhereFilter(where)
  if err != nil {
    util.Log(0, "ERROR! JobsQuery: Error parsing <where>: %v", err)
    filter = xml.FilterNone
  }
  return jobDB.Query(filter)
}

// Tries to remove all jobs matching the given filter, treating local and
// foreign jobs differently. Local jobs are removed and a
// foreign_job_updates message is broadcast for them. The removal of
// foreign jobs is attempted by passing the request to the responsible
// siserver via gosa_delete_jobdb_entry.
func JobsRemove(filter xml.HashFilter) {
  JobsRemoveLocal(xml.FilterAnd([]xml.HashFilter{xml.FilterSimple("siserver",config.ServerSourceAddress),filter}))
  
  //FIXME: Placeholder code
  JobsRemoveForeign(xml.FilterAnd([]xml.HashFilter{xml.FilterNot(xml.FilterSimple("siserver",config.ServerSourceAddress)),filter}))
}

// Returns a copy of the complete job database in the following format:
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
//       <id>1127008059018865</id>
//       <original_id>4</original_id>
//       <headertag>trigger_action_wake</headertag>
//       <result>none</result>
//       <xmlmessage>PHhtbD48aGVhZGVyPmpvYl90cmlnZ2VyX2FjdGlvbl93YWtlPC9oZWFkZXI+PHNvdXJjZT5HT1NBPC9zb3VyY2U+PHRhcmdldD4wMDowYzoyOTo1MDphMzo1MjwvdGFyZ2V0Pjx0aW1lc3RhbXA+MjAxMjA5MDYxNjQ3MzQ8L3RpbWVzdGFtcD48bWFjYWRkcmVzcz4wMDowYzoyOTo1MDphMzo1MjwvbWFjYWRkcmVzcz48L3htbD4=</xmlmessage>
//     </job>
//     <job>
//       ...
//     </job>
//   </jobdb>
func Jobs() *xml.Hash {
  return jobDB.Query(xml.FilterAll)
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
  jobDB.AddClone(job)
  
  //FIXME: Missing: foreign_job_updates broadcast!
}

// Removes from the JobDB the jobs matching filter.
// Calling this method triggers a foreign_job_updates broadcast.
//
// NOTE: The filter must include the siserver==config.ServerSourceAddress check,
// so that it only affects local jobs.
func JobsRemoveLocal(filter xml.HashFilter) {
  jobdb_xml := jobDB.Remove(filter)
  
  for _, tag := range jobdb_xml.Subtags() {
    for job := jobdb_xml.First(tag); job != nil; job = job.Next() {
      job.FirstOrAdd("status").SetText("done")
      job.FirstOrAdd("periodic").SetText("none")
    }
  }
  
  //message.Broadcast_foreign_job_updates(jobdb_xml)
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

