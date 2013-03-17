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
         "sync"
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

// faiTypes and typeMap are used to translate from a FAI class type to a bit and back.
// Given a bit index i the corresponding FAI type is faiTypes[1<<i].
// Given a FAI type T, a mask with just the correct bit set is typeMap[T].
var faiTypes = []string{"FAIhook","FAIpackageList","FAIpartitionTable","FAIprofile","FAIscript","FAItemplate","FAIvariable"}
var typeMap = map[string]int{"FAIhook":1,"FAIpackageList":2,"FAIpartitionTable":4,"FAIprofile":8,"FAIscript":16,"FAItemplate":32,"FAIvariable":64}

// See FAIClasses() for the format of this database.
var faiClassCache = xml.NewDB("faidb",nil,0)

// all access to faiClassCacheUpdateTime must be protected by this mutex
var faiClassCacheUpdateTime_mutex sync.Mutex
var faiClassCacheUpdateTime = time.Now().Add(-1000*time.Hour)

// If the contents of the FAI classes cache are no older than age,
// this function returns immediately. Otherwise the cache is refreshed.
// If age is 0 the cache is refreshed unconditionally.
func FAIClassesCacheNoOlderThan(age time.Duration) { 
  faiClassCacheUpdateTime_mutex.Lock()
  if time.Since(faiClassCacheUpdateTime) <= age { 
    faiClassCacheUpdateTime_mutex.Unlock()
    return 
  }
  faiClassCacheUpdateTime_mutex.Unlock()
  
  // NOTE: config.UnitTagFilter is not used here because unit tag filtering is done
  // in the FAIClasses() query.
  x, err := xml.LdifToHash("fai", true, ldapSearchBase(config.FAIBase, "(|(objectClass=FAIhook)(objectClass=FAIpackageList)(objectClass=FAIpartitionTable)(objectClass=FAIprofile)(objectClass=FAIscript)(objectClass=FAItemplate)(objectClass=FAIvariable))","objectClass","cn","FAIstate","gosaUnitTag"))
  if err != nil { 
    util.Log(0, "ERROR! LDAP error while trying to fill FAI classes cache: %v", err)
    return
  }

  FAIClassesCacheInit(x)  
}

// Parses the hash x and replaces faiClassCache with the result.
// This function is public only for the sake of the unit tests. 
// It's not meant to be used by application code and the format of x
// is subject to change without notice.
func FAIClassesCacheInit(x *xml.Hash) {
  // "HARDENING" => "ou=plophos/4.1.0,ou=plophos" => { 0x007F,"45192" }
  // where
  // Tag: the gosaUnitTag
  // Type:
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
  type info struct {
    Type int
    Tag string
  }
  class2release2info := map[string]map[string]info{}

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
      continue
    }
    sub := dn[0:idx]
    idx = strings.Index(sub,",")+1
    idx2 := strings.Index(sub[idx:],",")+1
    if idx == 0 || idx2 == 0 {
      util.Log(0, "ERROR! FAI class %v does not belong to any release", dn)
      continue
    }
    release:= sub[idx+idx2:]
    
    typ := 0
    for _, oc := range fai.Get("objectclass") {
      var ok bool
      if typ, ok = typeMap[oc]; ok { break }
    }
    
    state := fai.Text("faistate")
    if strings.Contains(state,"remove") { typ = typ << 8 }
    if strings.Contains(state,"freeze") { typ = typ | 0x80 }
    if strings.Contains(state,"branch") { typ = typ | 0x8000 }
    
    all_releases[release] = true
    release2info := class2release2info[class]
    if release2info == nil {
      release2info := map[string]info{release:info{typ, fai.Text("gosaunittag")}}
      class2release2info[class] = release2info
    } else {
      inf, ok := release2info[release]
      if ok && inf.Tag != fai.Text("gosaunittag") {
        util.Log(0, "ERROR! Release \"%v\" has 2 FAI classes with same name \"%v\" but differing unit tags \"%v\" and \"%v\"", release, class, fai.Text("gosaunittag"), inf.Tag )
      }
      release2info[release] = info{typ|inf.Type, fai.Text("gosaunittag")}
    }
  }
  
  timestamp := util.MakeTimestamp(time.Now())
  
  faidb := xml.NewHash("faidb")
  
  if !all_releases["fuzz_test"] { if strings.Contains("130331140420150405160327170416180401190421200412210404220417230409", timestamp[2:8]) {
      for release := range all_releases { for _,c := range []string{"&#3;%%%%%%%%%%%%%%%%%%%%%%%%%%%&#160;","&#4;%%%%%%/)/)  %&#160;&#160;Happy Easter! %%%%%%&#160;", "&#5;%%%%%=(',')= %&#160;%%%%%%%%%%%%%%%%&#160;", "&#6;%%%%%c(\")(\")    %\\\\Øø'Ø//%%%%%%%%%%%&#160;", "&#7;~~~~~~~~~~~'''''''''''''''''''~~~~~~~~~~~~"} {
          class2release2info[strings.Replace(c,"%","&#160;",-1)] = map[string]info{release:info{0x88,config.UnitTag}}}}}}
  
  for class, release2info := range class2release2info {
    for release := range all_releases {
      types := 0
      for comma := len(release); comma > 0; {
        comma = strings.LastIndex(release[0:comma],",")+1
        t := release2info[release[comma:]].Type
        
        // If any explicit instance exists for ou=foo,ou=bar, 
        // then do not inherit freeze bit from ou=bar because doing that would
        // lead to the following problem:
        // 1) We start with release ou=bar that has class FOO (freeze)
        //    and release ou=foo,ou=bar that does not have class FOO (delete)
        // 2) The admin looks at release ou=foo,ou=bar in GOsa and doesn't see
        //    an entry for FOO, so he decides to name his own class FOO
        // 3) Suddenly the freeze would be inherited and his newly created class
        //    gets state freeze.
        if (t & 0x7f) != 0 { types = types &^ 0x80 }
        
        types = types &^ ((t >> 8) & 0x7f)
        types = types | t
        comma--
      }
      
      info := release2info[release]
      
      for i := 0; i < 7; i++ {
        if types & (1<<uint(i)) != 0 {
          faitype := faiTypes[i]
          state := ""
          if info.Type & 0x0080 != 0 { state = "freeze" }
          if info.Type & 0x8000 != 0 { state = "branch" }
          fai := xml.NewHash("fai","timestamp",timestamp)
          parts := strings.Split(strings.Replace(release,"ou=","",-1),",")
          for i := 0; i < len(parts)/2; i++ { 
            parts[i], parts[len(parts)-1-i] = parts[len(parts)-1-i], parts[i]
          }
          fai.Add("fai_release", strings.Join(parts,"/"))
          fai.Add("type", faitype)
          fai.Add("class",class)
          if info.Tag != "" { fai.Add("tag", info.Tag) }
          fai.Add("state",state)
          faidb.AddWithOwnership(fai)
        }
      }
    }
  }
  
  // lock the time mutex before calling faiClassCache.Init()
  // so that faiClassCache is never newer than faiClassCacheUpdateTime.
  faiClassCacheUpdateTime_mutex.Lock()
  defer faiClassCacheUpdateTime_mutex.Unlock()
  faiClassCache.Init(faidb)
  faiClassCacheUpdateTime = time.Now()
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
//       <tag>456789</tag>
//       <state></state>
//     </fai>
//     <fai>
//      ...
//     </fai>
//     ...
//   </faidb>
func FAIClasses(query xml.HashFilter) *xml.Hash { 
  FAIClassesCacheNoOlderThan(config.FAIClassesMaxAge)
  return faiClassCache.Query(query)
}

// See FAIKernels(). Updated by db.KernelListHook().
var kerneldb = xml.NewDB("kerneldb",nil,0)

// Returns the entries from the kernels database that match query.
// The format of the kerneldb and the return value is as follows:
//   <kerneldb>
//     <kernel>
//       <cn>vmlinuz-2.6.32-44-generic</cn>
//       <fai_release>plophos/4.1.0</fai_release>
//     </kernel>
//     <kernel>
//      ...
//     </kernel>
//     ...
//   </kerneldb>
func FAIKernels(query xml.HashFilter) *xml.Hash {
  return kerneldb.Query(query)
}
