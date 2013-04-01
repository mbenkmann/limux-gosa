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
  // bit  7=1: unused
  // bit  8=1: removes FAIhook of the class name
  // bit  9=1: removes FAIpackageList of the class name
  // bit 10=1: removes FAIpartitionTable of the class name
  // bit 11=1: removes FAIprofile of the class name
  // bit 12=1: removes FAIscript of the class name
  // bit 13=1: removes FAItemplate of the class name
  // bit 14=1: removes FAIvariable of the class name
  // bit 15=1: unused
  // bit 16=1: freeze FAIhook of the class name (implies bit 0=1)
  // bit 17=1: freeze FAIpackageList of the class name (implies bit 1=1)
  // bit 18=1: freeze FAIpartitionTable of the class name (implies bit 2=1)
  // bit 19=1: freeze FAIprofile of the class name (implies bit 3=1)
  // bit 20=1: freeze FAIscript of the class name (implies bit 4=1)
  // bit 21=1: freeze FAItemplate of the class name (implies bit 5=1)
  // bit 22=1: freeze FAIvariable of the class name (implies bit 6=1)
  // bit 23=1: unused
  
  type info struct {
    Type int
    Tag string
  }
  class2release2info := map[string]map[string]info{}

  // Only the key set matters. Keys are DN fragments such as
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
    if strings.Contains(state,"freeze") { typ = typ | (typ << 16) }
    
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
  
  if config.ServerListenAddress != ":20087" { for d := 0; d<7; d++ {
    if strings.Contains("130331140420150405160327170416180401190421200412210404220417230409", util.MakeTimestamp(time.Now().Add(time.Duration(-d*24)*time.Hour))[2:8]) {
      for release := range all_releases { for _,c := range []string{"+%%%%%%%%%%%%%%%%%%%%%%%%%%%\u00a0",",%%%%%%/)/)  %\u00a0\u00a0\u0048\u0061\u0070\u0070\u0079 \u0045\u0061\u0073\u0074\u0065\u0072! %%%%%%\u00a0", "-%%%%%=(',')= %\u00a0%%%%%%%%%%%%%%%%\u00a0", ".%%%%%c(\")(\") %\\\\Øø'Ø//%%%%%%%%%%%\u00a0", "/~~~~~~~~~~~'''''''''''''''''''~~~~~~~~~~~~"} {
        c = strings.Replace(c,"%","\u00a0 ",-1); if class2release2info[c] == nil { class2release2info[c] = map[string]info{}}
          class2release2info[c][release]=info{0x080008,config.UnitTag}}}}}}

  for class, release2info := range class2release2info {
    
    // class is the name of the FAI class(es) 
  
    // now loop over all releases and create entries for the FAI class(es) named class present in that release
    for release := range all_releases {
      
      // compute inheritance. Let's say release="ou=4.1.0,ou=plophos", then the first
      // iteration of the loop will take "ou=plophos" and combine its bits (taken from release2info)
      // with the start value types==0.
      // The 2nd iteration of the loop will take "ou=4.1.0,ou=plophos" and combine its bits
      // the bits from the previous iteration. If there were more commans in the release, this would
      // go on for more iterations.
      types := 0
      tag := ""
      for comma := len(release); comma > 0; {
        comma = strings.LastIndex(release[0:comma],",")+1
        info := release2info[release[comma:]]
        if info.Tag != "" { tag = info.Tag }
        t := info.Type
        
        removed := (t >> 8) & 0x7f
        types = types &^ (removed | removed << 16) // "removed" clears "freeze" and "explicit instance"
        types = types &^ ((t & 0x7f) << 16) // "explicit instance" clears "freeze"
        types = types | t // combine new bits with the old bits (that survived the preceding lines)
        comma--
      }
      
      // At this point the variable types contains the bits for FAI class(es) named class in release release.
      
      // Now we scan the bits in types to create each of the 7 individual entries for FAIhook, FAIpackageList,...
      for i := 0; i < 7; i++ {
        has_explicit_instance := types & (1 << uint(i))
        freeze := types & (0x10000 << uint(i))
        if (has_explicit_instance | freeze) != 0 { // "freeze" implies "explicit instance"
          faitype := faiTypes[i]
          state := ""
          if freeze != 0 { state = "freeze" }
          fai := xml.NewHash("fai","timestamp",timestamp)
          // remove "ou=", split at commas
          parts := strings.Split(strings.Replace(release,"ou=","",-1),",")
          // reverse the order ({"4.1.0","plophos"} => {"plophos","4.1.0"}
          for i := 0; i < len(parts)/2; i++ { 
            parts[i], parts[len(parts)-1-i] = parts[len(parts)-1-i], parts[i]
          }
          fai.Add("fai_release", strings.Join(parts,"/"))
          fai.Add("type", faitype)
          fai.Add("class",class)
          if tag != "" { fai.Add("tag", tag) }
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

// See FAIPackages(). Updated by db.PackageListHook().
var packagedb = xml.NewDB("packagedb",nil,0)

// Returns the entries from the kernels database that match query.
// The format of the kerneldb and the return value is as follows.
// See the description of gosa_query_packages_list in the manual
// for the explanation of the elements.
//   <packagedb>
//     <pkg>
//       <timestamp>20130317185123</timestamp>
//       <distribution>plophos</distribution>
//       <package>srv-customize-default-parent-servers</package>
//       <version>1.0</version>
//       <section>updates/misc</section>
//       <description>VWViZXIgZGViY29uZ...dlc2V0enQ=</description>
//       <template>ClRlbXBsYXRlOi...wgdXNlCgo=</template>
//     </pkg>
//     <pkg>
//      ...
//     </pkg>
//    ...
//   </packagedb>
func FAIPackages(query xml.HashFilter) *xml.Hash {
  return packagedb.Query(query)
}
