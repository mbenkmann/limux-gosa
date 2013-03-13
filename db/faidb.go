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

// API for the various databases used by go-susi.
package db

import (
         "time"
         "strings"
         
         "../xml"
         "../util"
       )


// Returns a list of all known Debian software repositories as well as the
// available releases and their sections. If none are found, the return
// value is <faidb></faidb>. The general format of the return value is
// <faidb>
//    <repository>
//      <timestamp>20130304093211</timestamp>
//        <fai_release>halut/2.4.0</fai_release>
//        <tag>1154342234048479900</tag>
//        <server>http://vts-susi.example.de/repo</server>
//        <sections>main,contrib,non-free,lhm,ff</sections>
//    </repository>
//    <repository>
//      ...
//    </repository>
//    ...
// </faidb>
//
// See operator's manual, description of message gosa_query_fai_server for
// the meanings of the individual elements.
func FAIServers() *xml.Hash {
  // NOTE: We do NOT add config.UnitTagFilter here because the results are individually
  // tagged within the reply.
  x, err := xml.LdifToHash("repository", true, ldapSearch("(&(FAIrepository=*)(objectClass=FAIrepositoryServer))","FAIrepository","gosaUnitTag"))
  if err != nil { 
    util.Log(0, "ERROR! LDAP error while looking for FAIrepositorServer objects: %v", err)
    return xml.NewHash("faidb")
  }

  result := xml.NewHash("faidb")
  timestamp := util.MakeTimestamp(time.Now())
  
  for repo := x.First("repository"); repo != nil; repo = repo.Next() {
    tag := repo.Text("gosaunittag")
    
    // http://vts-susi.example.de/repo|parent-repo.example.de|plophos/4.1.0|main,restricted,universe,multiverse
    for _, fairepo := range repo.Get("fairepository") {
      repodat := strings.Split(fairepo,"|")
      if len(repodat) != 4 {
        util.Log(0, "ERROR! Cannot parse FAIrepository=%v", fairepo)
        continue
      }
      
      repository := xml.NewHash("repository", "timestamp", timestamp)
      repository.Add("fai_release", repodat[2])
      if tag != "" { repository.Add("tag", tag) }
      repository.Add("server",repodat[0])
      repository.Add("sections",repodat[3])
      result.AddWithOwnership(repository)
    }
  }
  
  return result 
}

