/*
Copyright (c) 2016 Landeshauptstadt München
Author: Matthias S. Benkmann

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.
*/

// Managing stored audit data.
package db

import (
         "os"
         "io/ioutil"
         "strings"
         "strconv"
         "fmt"
         
         "github.com/mbenkmann/golib/util"
       )


/*
  Signature of a function to be called for each <entry> element.
  Each child element of <entry> has a fixed index i based on its
  name (NOT based on the order of the elements in the file) and the value
  of that child element is stored in entry[i].
  E.g. If "<version>" is assigned index 4, then entry[4] will always
  be the value of the <version> child element, regardless of the
  order in which child elements appear in the file.
*/
type AuditScanFunc func(entry []string)

/*
  Identifies an audit instance or the lack thereof.
*/
type AuditID struct {
  /*
    If non-empty this is the time when the audit data was recorded.
    If empty, this signals that this AuditID identifies a system
    that has not been audited.
  */
  Timestamp string
  
  /*
     The MAC address of the system the audit (or lack thereof) refers to.
     This field is always non-empty.
  */
  MAC string
  
  /*
    IP address of the system identified by MAC. May be "" if unknown.
  */  
  IP string
  
  /*
    Host name of the system identified by MAC. May be "" if unknown.
    If non-empty this may be either a fully qualified name or a short name.
  */  
  Hostname string
}

/*
  dir: the name of the "fai" directory that contains subdirectories
       whose names correspond to MAC addresses of audited systems.
  ts1: lower bound timestamp (inclusive); may contain underscores
  ts2: upper bound timestamp (inclusive); may contain underscores
  xmlname: name of audit file(s) to look for, e.g. "packages.xml"
  mac: if non-empty, only audit logs from the system with that MAC address
       will be scanned.
  contains: if non-empty, only audit logs that contain this string
            will be scanned. E.g. "<key>foobar</key>"
  f: Function that will be called for each data set in the selected
     audit files.
  props: A list of element names such as "macaddress" and "key". This
         list determines which properties will be included in each data set
         that is passed to AuditScanFunc and in which order.
         Each name may only occur once. Available names are custom
         sub-elements of <entry> as well as "macaddress", "ipaddress",
         "lastaudit", "key", "hostname".
  returnothers: if true, the return value nonmatching will be a list
                of systems whose datasets are excluded because their
                audit file does not include the contains string;
                noaudit will include systems that are known but have
                no audit file called xmlname in the selected timestamp
                bounds.
                The field AuditID.Timestamp will be
                set to the most recent audit for systems that are
                included in noaudit because they don't have an audit
                in the requested timeframe.
                The field AuditID.Timestamp will be set to the
                timestamp of the most recent audit WITHIN THE requested
                timeframe for systems that are included in nonmatch
                because they do not have the contains string.
*/
func AuditScanSubdirs(dir, ts1, ts2, xmlname, mac, contains string, f AuditScanFunc, props []string, returnothers bool) (nonmatch, noaudit []AuditID){
  propTree := makePropTree(props)
  
  if mac != "" {
    auditScanDir(dir, ts1, ts2, xmlname, mac, contains, f, propTree, len(props), returnothers, &nonmatch, &noaudit)
    return
  }
  
  d, err := os.Open(dir)
  if err != nil {
    util.Log(0, "ERROR! Open(%v): %v", dir, err)
    return
  }
  fis, err := d.Readdir(-1)
  d.Close()
  if err != nil {
    util.Log(0, "ERROR! Readdir(%v): %v", dir, err)
    return
  }
  
  for _, fi := range fis {
    fname := fi.Name()
    if fi.IsDir() && isMAC(fname) {
      auditScanDir(dir, ts1, ts2, xmlname, fname, contains, f, propTree, len(props), returnothers, &nonmatch, &noaudit)
    }
  }
  
  return
}

/*
  Scans a single directory dir+"/"+mac that is expected to contain
  subdirectories named audit-<timestamp>. See AuditScanSubdirs for details.
*/
func auditScanDir(dir, ts1, ts2, xmlname, mac, contains string, f AuditScanFunc, propTree *elementTree, entrysize int, returnothers bool, nonmatch *[]AuditID, noaudit *[]AuditID) {
  subdir := dir + "/" + mac  // .../fai/MACADDRESS
  d, err := os.Open(subdir)
  if err != nil {
    util.Log(0, "ERROR! Open(%v): %v", subdir, err)
    if returnothers {
      *noaudit = append(*noaudit, AuditID{MAC:mac})
    }
  } else {
    subfis, err := d.Readdir(-1)
    d.Close()
    if err != nil {
      util.Log(0, "ERROR! Readdir(%v): %v", subdir, err)
      if returnothers {
        *noaudit = append(*noaudit, AuditID{MAC:mac})
      }
    } else {
      // find most recent audit dir in [ts1,ts2] window.
      best_auditname := ""
      last_auditname := ""
      for _, subfi := range subfis {
        auditname := subfi.Name()  // audit-timestamp
        if isAudit(auditname) {
          if auditname > last_auditname {
            last_auditname = auditname
          }
          if auditname > best_auditname && isInTimestampRange(auditname, ts1, ts2) {
            best_auditname = auditname
          }
        }
      }
      
      if best_auditname == "" {
        if last_auditname == "" {
          *noaudit = append(*noaudit, AuditID{MAC:mac})
        } else {
          *noaudit = append(*noaudit, extractAuditID(mac, subdir, last_auditname, xmlname))
        }
      } else {
        dataname := subdir + "/" + best_auditname + "/" + xmlname
        data, err := ioutil.ReadFile(dataname)
        if err != nil {
          util.Log(0, "ERROR! ReadFile(%v): %v", dataname, err)
          if returnothers {
            *noaudit = append(*noaudit, AuditID{MAC:mac})
          }
        } else {
          i, ipaddress, hostname := findFirstEntry(data)
          if i < 0 { // no <entry> found => treated as not audited
            if returnothers {
              *noaudit = append(*noaudit, AuditID{MAC:mac, IP:ipaddress, Hostname:hostname})
            }
          } else {
            if contains != "" {
              b := contains[0]
              for i := 0; i < len(data)-len(contains); i++ {
                if data[i] == b {
                  k := len(contains)
                  for {
                    k--
                    if data[i+k] != contains[k] { break }
                    if k == 0 { goto do_audit }
                  }
                }
              }
              
              if returnothers {
                *nonmatch = append(*nonmatch, AuditID{MAC:mac, IP:ipaddress, Hostname:hostname, Timestamp:auditFilenameToTimestamp(best_auditname)})
              }
              return
            }
do_audit:
            auditScanFile(ipaddress,hostname,data,i,f,propTree, entrysize)
          }
        }
      }
    }
  }
}

// returns true iff s is a lower-case MAC address with ":" separator.
func isMAC(s string) bool {
  if len(s) != 17 { return false }
  i := 0
  for {
    if ('0' <= s[i] && s[i] <= '9') || ('a' <= s[i] && s[i] <= 'f') {
      // hex digit
    } else {
      return false
    }
    i++
    if ('0' <= s[i] && s[i] <= '9') || ('a' <= s[i] && s[i] <= 'f') {
      // hex digit
    } else {
      return false
    }
    i++
    if i == 17 { break }
    if s[i] != ':' { return false }
    i++
  }
  return true
}

// Returns true iff s is of the form "audit-..." where "..." is
// 14 digits mixed with an arbitrary number of underscores.
func isAudit(s string) bool {
  if len(s) < 20 { return false }
  if s[0:6] != "audit-" { return false }
  
  count := 0
  for a := 6; a < len(s); a++ {
    if s[a] != '_' {
      if s[a] < '0' || s[a] > '9' { return false }
      count++
    }
  }
  return count == 14
}

// Assuming s is of the form "audit-..." where "..." is
// 14 digits mixed with an arbitrary number of underscores,
// this function returns true iff the timestamp contained in
// s sorts lexicographically between timestamp1 and timestamp2 (both inclusive)
// which have to be 14 digits plus underscores, too.
// Underscores in all 3 strings are ignored.
func isInTimestampRange(s, timestamp1, timestamp2 string) bool {
  a := 6
  b := 0
  
  for {
    for a < len(s) && s[a] == '_' { a++ }
    if a == len(s) { break }
    for b < len(timestamp1) && timestamp1[b] == '_' { b++ }
    if b == len(timestamp1) { break }
    if s[a] < timestamp1[b] { return false }
    if s[a] > timestamp1[b] { break }
    a++
    b++
  }
  
  a = 6
  b = 0
  
  for {
    for a < len(s) && s[a] == '_' { a++ }
    if a == len(s) { break }
    for b < len(timestamp2) && timestamp2[b] == '_' { b++ }
    if b == len(timestamp2) { break }
    if s[a] > timestamp2[b] { return false }
    if s[a] < timestamp2[b] { break }
    a++
    b++
  }
  return true
}


// extract as much information as possible. At the very least
// the timestamp can be extracted from last_auditname (by removing
// the "audit-" prefix and any contained "_" characters). But
// if a file xmlname exists in the directory, it can be read partially
// and scanned for <ipaddress> and <hostname>.
func extractAuditID(mac, subdir, last_auditname, xmlname string) AuditID {
  return AuditID{MAC:mac,Timestamp:auditFilenameToTimestamp(last_auditname)}
}

// Removes all non-digit characters from auditname and returns the result.
func auditFilenameToTimestamp(auditname string) string {
  ts := make([]byte,0,len(auditname))
  for i := range auditname {
    if auditname[i] >= '0' && auditname[i] <= '9' { ts = append(ts, auditname[i]) }
  }
  return string(ts)
}

// Scans data for the first occurrence of "<entry>" and returns the index
// of its location (i.e. of the "<" character).
// If "<hostname>" and/or "<ipaddress>" are
// encountered before "<entry>" the contents of these elements will
// be returned as well (otherwise the respective return string will be "").
// If "<entry>" is not found, -1 is returned (ipaddress and hostname are
// still returned if they are encountered).
// If the first non-whitespace string in data is not "<audit>", returns -2.
func findFirstEntry(data []byte) (i int,ipaddress,hostname string) {
  // If the input is malformed, we may read past end of data
  defer func() {
    if recover() != nil {
      i = -1
    }
  }()
  
  i = 0
  for data[i] <= ' ' { i++ }
  if string(data[i:i+7]) != "<audit>" { return -2, "", "" }
  i+=7
  
  for {
    for data[i] <= '<' { i++ }
    switch data[i] {
      case 'h': // hostname
                k := i+9
                if string(data[i:k]) == "hostname>" {
                  i = nextTag(data,k)
                  hostname = string(data[k:i])
                  i+=11
                } else {
                  // unknown tag => skip
                  for data[i] != '<' { i++ }
                  for data[i] != '>' { i++ }
                  i++
                }
      case 'i': // ipaddress
                k := i+10
                if string(data[i:k]) == "ipaddress>" {
                  i = nextTag(data,k)
                  ipaddress = string(data[k:i])
                  i+=12
                } else {
                   // unknown tag => skip
                  for data[i] != '<' { i++ }
                  for data[i] != '>' { i++ }
                  i++
                }
      case 'e': // entry
                if string(data[i:i+6]) == "entry>" {
                  i--
                  return
                } else {
                  // unknown tag => skip
                  for data[i] != '<' { i++ }
                  for data[i] != '>' { i++ }
                  i++
                }
      default:  // unknown tag => skip
                for data[i] != '<' { i++ }
                for data[i] != '>' { i++ }
                i++
    }
  }
}
  
// Scans data[i:] for <entry>...</entry> and calls auditScanFunc for each entry.
// The first tag encountered outside of an entry element that is not itself
// "<entry>" will terminate the scan. Usually this would be the "</audit>" end tag.
// The following describes how an <entry> element is converted into a []string
// used as argument for auditScanFunc:
//   * entrysize is the length of each []string slice passed to auditScanFunc.
//   * tree maps a child element name to its index in the entry slice.
//     It's important to note that the order of child elements within <entry>
//     does not matter.
//   * <entry> is not allowed to have more than one child element with the same
//     name. It is unspecified which value will be passed to auditScanFunc in
//     this case.
//   * If ipaddress and/or hostname are non-empty AND tree contains a mapping
//     for "ipaddress>" and/or "hostname>" respectively, the values will be
//     added to every entry.
//   * If <entry> has an <ipaddress> and/or <hostname> child that will override
//     the global ipaddress/hostname.
func auditScanFile(ipaddress string, hostname string, data []byte, i int, auditScanFunc AuditScanFunc, tree *elementTree, entrysize int) {
  // If the input is malformed, we may read past end of data
  defer func() {
    if recover() != nil {
      // nothing
    }
  }()

  ip_index := tree.IndexOf("ipaddress>")
  host_index := tree.IndexOf("hostname>")

  for {
    for data[i] <= '<' { i++ }
    if string(data[i:i+6]) != "entry>" {
      return // not <entry> ? => stop processing (in an ordinary file this would be </audit>
    }
    i+=6
    
    entry := make([]string, entrysize)
    if ip_index >= 0 {
      entry[ip_index] = ipaddress
    }
    if host_index >= 0 {
      entry[host_index] = hostname
    }
    
    for{
      for data[i] <= '<' { i++ }
      if data[i-1] == '/' { // </entry>
        i+=6
        auditScanFunc(entry)
        break
      }
      
      k := i
      etree := tree
      for {
        etree = etree.link[data[k]]
        if etree == nil {
          // tag that is not requested => skip
          for data[i] != '>' { i++ }
          if data[i-1] != '/' { // if this is not an <empty/> tag
            i = nextTag(data,i)+1 // this lands us on the "/" of the end tag
          }
          i = nextTag(data,i) // this lands us on the "<" of the next start tag
          break
        }
        if etree.name != "" {
          k = i+len(etree.name)
          if etree.name != string(data[i:k]) {
            // special case for <empty/> tag
            if data[k] == '>' && data[k-1] == '/' && etree.name[0:len(etree.name)-1] == string(data[i:k-1])  {
              entry[etree.index] = ""
              i = k+1
              break
            } else {
              // tag that is not requested => skip
              for data[i] != '>' { i++ }
              if data[i-1] != '/' { // if this is not an <empty/> tag
                i = nextTag(data,i)+1 // this lands us on the "/" of the end tag
              }
              i = nextTag(data,i) // this lands us on the "<" of the next start tag
              break
            }
          } else {
            i = nextTag(data,k)
            entry[etree.index] = string(data[k:i])
            i+=len(etree.name)+2 // 2 for <, /  (the > is included in etree.name)
            break
          }
        }
        k++
      }
    }
  }
}

func nextTag(data []byte, i int) int {
  for data[i] != '<' { i++ }
  return i
}

// A search tree for element names and the corresponding index in a name
// list. Each elementTree node has a link
// array with one entry per byte. To search for a name like "package"
// you follow these links byte by byte (i.e. you start with 'p', then 'a',...)
// until you arrive at an elementTree with non-empty name element or until
// the link is nil. In the latter case there is no element with that name.
// In the former case, the elementTree.name is the name of the only allowed
// element with the prefix corresponding to the bytes that led to the node.
// Each name field ends with ">" (but does NOT start with "<"), so it
// is guaranteed that even if there exist elements like "<foo>" and "<foobar>"
// where one name is a prefix of the other name, this does not create
// ambiguities in the search tree, because if you continue searching
// until and including the ">" you will always end up at exactly one node.
type elementTree struct {
  name string // something like "version>", i.e. without '<' but with '>'
  index int
  link [256]*elementTree
}

func (t *elementTree) String() string {
  s := []string{}
  t.stringify("",&s)
  return strings.Join(s,"")
}

func (t *elementTree) stringify(indent string, s *[]string) {
  if t.name != "" {
    *s = append(*s, " ", strconv.Itoa(t.index), " ", t.name)
  } else {
    for i,l := range t.link {
      if l != nil {
        if len(*s) != 0 { *s = append(*s, "\n") }
        *s = append(*s, indent, string([]byte{byte(i)}))
        l.stringify(indent+" ", s)
      }
    }
  }
}

// returns -1 if not found
func (t *elementTree) IndexOf(name string) int {
  if name == "" || name[len(name)-1] != '>' { return -1 }
  k := 0
  etree := t
  for {
    etree = etree.link[name[k]]
    if etree == nil {
      return -1
    }
    if etree.name != "" {
      if etree.name == name {
        return etree.index
      } else {
        return -1
      }
    }
    k++
  }
}

// Takes a list of strings that correspond to names in <entry>
// elements and creates an elementTree that is a search tree for
// these element names. The elementTree.index in the node for
// props[i] is i.
// The idea behind this function is to map props[i] to i so that
// you can have an array of len(props) elements that corresponding to the names
// in props.
// The assumption is that usually each element is uniquely identified by its
// first letter, so that one lookup based on the first byte is enough.
// In that case this search tree approach is faster than using a map[string]int.
func makePropTree(props []string) *elementTree {
  root := &elementTree{}
  
  for p, name := range props {
    if name == "" { continue } // empty entry in props => WTF?
    // The tree has inner nodes and leaf nodes but no dual nodes.
    // This requires that an entry X can not be a prefix of an entry Y.
    // E.g. "foo" and "foobar" cannot both be stored in the tree.
    // In order to achieve this, we store all entries with a ">" as final
    // character and disallow names that contain ">". This is not a
    // limitation because XML element names must not contain ">".
    if strings.Contains(name,">") { continue }
    name = props[p]+">"
   
    k := 0
    etree := root
    for {
      oldetree := etree
      etree = oldetree.link[name[k]]
      if etree == nil {
        // found insertion point into search tree
        oldetree.link[name[k]] = &elementTree{name:name, index:p}
        break
      }
      if etree.name != "" {
        // conflict with another element that has the same prefix
        
        if etree.name == name {
          break // duplicate entry in props => WTF
        } else {
          //insert an extra step
          etree.link[etree.name[k+1]] = &elementTree{name:etree.name,index:etree.index}
          etree.name = ""
          etree.index = 0
        }
      }
      k++
    }
  }
  
  return root
}

func AuditTest() string {
  props := []string{"foo", "foobar", "ill>egal1", "egal2>", ">illegal3", "", "entry", "entry", "bla", "dusel", "duschkopf"}
  t := makePropTree(props)
  res := t.String() +"\n\n"
  for i, name := range props {
    res += "\n"+strconv.Itoa(i)+" "+strconv.Itoa(t.IndexOf(name+">"))+" "+name
  }
  
  for _, mac := range []string{"11:22:33:44:55:6","11:22:33:44:55:666", "", "00:00:00:00:00:00", "99:99:99:99:99:99", "aa:aa:aa:aa:aa:aa", "ff:ff:ff:ff:ff:ff", "A0:00:00:00:00:00", "0A:00:00:00:00:00", "00-00:00:00:00:00" } {
    res += "\n" + mac + fmt.Sprintf(" %v", isMAC(mac))
  }
  
  for _, fname := range []string{"audit20161231235959","audit-20161231235959","Audit-20161231235959","audit-20161231235959_","audit-2016123123595_","audit-2016_12312_3595_9","audit-20161231235959_11" } {
    res += "\n" + fname + fmt.Sprintf(" %v", isAudit(fname))
  }

  stamps := []string{"0000_00_00_0000_00", "000_00_000_0000_00", "00_0_00_000_00_00_00___", "99990000_0000_00", "2015_31_12_1659_22", "2017_31_12_1659_22", "20183112165922_", "20183112165922" }
  for i, ts := range stamps {
    if i == 0 || i+1 == len(stamps) { continue }
    res += "\n" + stamps[i-1] + " " + ts + " " + stamps[i+1] + fmt.Sprintf(" %v", isInTimestampRange("audit-"+ts, stamps[i-1],stamps[i+1]))
  }
  
  res += "\n"+auditFilenameToTimestamp("dj1__2  3jDJ45xx")
  
  tests := []string{"",
                    "  <audit>",
                    "<foo><entry></entry></foo>",
                    " \n\t<audit><hoax>yy</hoax><a></a><igitt>z</igitt><ekel>erregend</ekel><ipaddress>1.2.3.4</ipaddress><hostname>Grutsch</hostname><entry>",
                    }
  for i, ts := range tests {
    idx, ipaddress, hostname := findFirstEntry([]byte(ts))
    res += fmt.Sprintf("\n%v %v %v %v", i, idx, ipaddress, hostname)
  }

  auditScanFunc := func(entry []string){
    res += "\n"
    for _, e := range entry {
      res += e
    }
  }
  
  tree := makePropTree([]string{"aaa","aab","ipaddress","D", "hostname","F"})
  
  data:=`
  <entry>
  <aab>b</aab>
  <D>d</D>
  <foo>x</foo>
  <aaa>a</aaa>
  </entry>
  <entry>
  <F>f</F>
  <aab>b</aab>
  <D>d</D>
  <foo>x</foo>
  <aaa/>
  </entry>
  
  </audit>
  `
  auditScanFile("c", "e", []byte(data), 0, auditScanFunc, tree, 8)
  
  return res
}
