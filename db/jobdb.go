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
         "time"
         "../xml"
         "../config"
         "../util"
       )

// Stores jobs to be executed at some point in the future.
var jobDB *xml.DB

// Initializes JobDB with data from the file config.JobDBPath if it exists.
func JobsInit() {
  jobdb_storer := &xml.FileStorer{config.JobDBPath}
  var delay time.Duration = 0
  jobDB = xml.NewDB("jobdb", jobdb_storer, delay)
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
//       <id>4</id>
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

// Replaces (or adds) the job identified by <headertag> and <macaddress> with
// the new data.
//   job: Has the following format 
//        <job>
//          <headertag>trigger_action_wake</headertag>
//          <macaddress>00:0c:29:50:a3:52</macaddress>
//          ...
//        </job>
func JobUpdate(job *xml.Hash) {
  if job.Name() != "job" {
    panic("Surrounding tag must be <job>...</job>")
  }
  
  jobDB.Replace(xml.FilterSimple(
                  "headertag", job.Text("headertag"), 
                  "macaddress", job.Text("macaddress")), 
                false, 
                job)
}
