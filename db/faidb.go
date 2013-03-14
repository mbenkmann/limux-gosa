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
         "../config"
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

var faiTypes = []string{"FAIhook","FAIpackageList","FAIpartitionTable","FAIprofile","FAIscript","FAItemplate","FAIvariable"}
var typeMap = map[string]int{"FAIhook":1,"FAIpackageList":2,"FAIpartitionTable":4,"FAIprofile":8,"FAIscript":16,"FAItemplate":32,"FAIvariable":64}

// If the contents of the FAI classes cache are no older than age,
// this function returns immediately. Otherwise the cache is refreshed.
// If age is 0 the cache is refreshed unconditionally.
func FAIClassesCacheNoOlderThan(age time.Duration) { 
  
  // test age
  
  
  
  // NOTE: config.UnitTagFilter is not used here because AFAICT from the
  // gosa-si-server code it doesn't use it either for the faidb.
  x, err := xml.LdifToHash("fai", true, ldapSearchBase(config.FAIBase, "(|(objectClass=FAIhook)(objectClass=FAIpackageList)(objectClass=FAIpartitionTable)(objectClass=FAIprofile)(objectClass=FAIscript)(objectClass=FAItemplate)(objectClass=FAIvariable))","objectClass","cn","FAIstate"))
  if err != nil { 
    util.Log(0, "ERROR! LDAP error while trying to fill FAI classes cache: %v", err)
    return
  }

  // "HARDENING" => "ou=plophos/4.1.0,ou=plophos" => 0x007F
  //
  // bit  0=1: has explicit instance of FAIhook of the class name
  // bit  1=1: has explicit instance of FAIpackageList of the class name
  // bit  2=1: has explicit instance of FAIpartitionTable of the class name
  // bit  3=1: has explicit instance of FAIprofile of the class name
  // bit  4=1: has explicit instance of FAIscript of the class name
  // bit  5=1: has explicit instance of FAItemplate of the class name
  // bit  6=1: has explicit instance of FAIvariable of the class name
  // bit  7=1: freeze
  // bit  8=1: removes FAIhook of the class name
  // bit  9=1: removes FAIpackageList of the class name
  // bit 10=1: removes FAIpartitionTable of the class name
  // bit 11=1: removes FAIprofile of the class name
  // bit 12=1: removes FAIscript of the class name
  // bit 13=1: removes FAItemplate of the class name
  // bit 14=1: removes FAIvariable of the class name
  // bit 15=1: branch
  // 
  class2release2type := map[string]map[string]int{}

  // Only the key set matters. Keys are releases such as
  // "ou=plophos" and "ou=halut/2.4.0,ou=halut".
  all_releases := map[string]bool{}
  
  for fai := x.First("fai"); fai != nil; fai = fai.Next() {
    class := fai.Text("cn")
    if class == "" {
      util.Log(0, "ERROR! Encountered FAI class without cn: %v", fai)
      continue
    }
    
    dn := fai.Text("dn")
    idx := strings.LastIndex(dn, ","+config.FAIBase)
    if idx < 0 {
      util.Log(0, "ERROR! Huh? I guess there's something about DNs I don't understand. \",%v\" is not a suffix of \"%v\"", config.FAIBase, dn)
      idx = len(dn)
    }
    sub := dn[0:idx]
    release := sub[strings.Index(sub[strings.Index(sub,",")+1:],",")+1:]
    
    typ := typeMap[fai.Text("objectclass")]
    
    state := fai.Text("faistate")
    if strings.Contains(state,"remove") { typ = typ << 8 }
    if strings.Contains(state,"freeze") { typ = typ | 0x80 }
    if strings.Contains(state,"branch") { typ = typ | 0x8000 }
    
    all_releases[release] = true
    class2release2type[class][release] = typ
  }
  
  timestamp := util.MakeTimestamp(time.Now())
  
  faidb := xml.NewHash("faidb")
  
  for class, release2type := range class2release2type {
    for release := range all_releases {
      types := 0
      for comma := len(release); comma != 0; {
        comma = strings.LastIndex(release[0:comma],",")+1
        t := release2type[release[comma:]]
        types = types &^ (t >> 8)
        types = types | t
      }
      
      for i := 0; i < 7; i++ {
        if types & (1<<uint(i)) != 0 {
          faitype := faiTypes[i]
          state := ""
          if release2type[release] & 0x0080 != 0 { state = "freeze" }
          if release2type[release] & 0x8000 != 0 { state = "branch" }
          fai := xml.NewHash("fai","timestamp",timestamp)
          fai.Add("fai_release", strings.SplitN(strings.SplitN(release,",",2)[0],"=",2)[1])
          fai.Add("type", faitype)
          fai.Add("class",class)
          fai.Add("state",state)
          faidb.AddWithOwnership(fai)
        }
      }
    }
  }
}

// Returns the entries from the FAI classes database that match query.
// The entries will be no older than config.FAIClassesMaxAge.
// The format of the faidb and the return value is as follows:
//   <faidb>
//     <fai>
//       <timestamp>20130304093210</timestamp>
//       <fai_release>plophos/4.1.0</fai_release>
//       <type>FAIscript</type>
//       <class>HARDENING</class>
//       <state></state>
//     </fai>
//   </faidb>
func FAIClasses(query xml.HashFilter) *xml.Hash { return nil }


