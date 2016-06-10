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
         "fmt"
         "io/ioutil"
         "strings"
         "time"
         
         "github.com/mbenkmann/golib/util"
         "../xml"
       )


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
    AuditScanDir(dir, ts1, ts2, xmlname, mac, contains, f, propTree, returnothers, &nonmatch, &noaudit)
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
      AuditScanDir(dir, ts1, ts2, xmlname, fname, contains, f, propTree, len(props), returnothers, &nonmatch, &noaudit)
    }
  }
}

/*
  Scans a single directory dir+"/"+mac that is expected to contain
  subdirectories named audit-<timestamp>. See AuditScanSubdirs for details.
*/
func AuditScanDir(dir, ts1, ts2, xmlname, mac, contains string, f AuditScanFunc, propTree *elementTree, entrysize int, returnothers bool, nonmatch *[]AuditID, noaudit *[]AuditID) {
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
          if auditname > best_auditname && isInTimestampRange(s, ts1, ts2) {
            best_auditname = auditname
          }
        }
      }
      
      if best_auditname == "" {
        if last_auditname == "" {
          *noaudit = append(*noaudit, AuditID{MAC:mac})
        } else {
          // extract as much information as possible. At the very least
          // the timestamp can be extracted from last_auditname (by removing
          // the "audit-" prefix and any contained "_" characters). But
          // if a file xmlname exists in the directory, it can be read partially
          // and scanned for <ipaddress> and <hostname>.
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
            auditScanFile(data,i,f,propTree, entrysize)
          }
        }
      }
    }
  }
}


func findFirstEntry(data []byte) (i int,ipaddress,hostname string) {
  // If the input is malformed, we may read past end of data
  defer func() {
    if recover() != nil {
      i = -1
    }
  }()
  
  i = 0
  for data[i] <= ' ' { i++ }
  if data[i:i+7] != "<audit>" { return -1, "", "" }
  i+=7
  
  for {
    for data[i] <= '<' { i++ }
    switch data[i] {
      case 'h': // hostname
                k := i+9
                if data[i:k] == "hostname>" {
                  i = nextTag(data,k)
                  hostname = data[k:i]
                  i+=11
                } else {
                  // unknown tag => skip
                  for data[i] != '<' { i++ }
                  for data[i] != '>' { i++ }
                  i++
                }
      case 'i': // ipaddress
                k := i+10
                if data[i:k] == "ipaddress>" {
                  i = nextTag(data,k)
                  ipaddress = data[k:i]
                  i+=12
                } else {
                   // unknown tag => skip
                  for data[i] != '<' { i++ }
                  for data[i] != '>' { i++ }
                  i++
                }
      case 'e': // entry
                if data[i:i+6] == "entry>" {
                  i+=6
                  return
                } else {
                  fallthrough
                }
      default:  // unknown tag => skip
                for data[i] != '<' { i++ }
                for data[i] != '>' { i++ }
                i++
    }
  }
}
  

func auditScanFile(data []byte, i int, auditScanFunc AuditScanFunc, tree *elementTree, entrysize int) {
  // If the input is malformed, we may read past end of data
  defer func() {
    if recover() != nil {
      i = -1
    }
  }()

  ip_index := tree.IndexOf("ipaddress>")
  host_index := tree.IndexOf("hostname>")

  for {
    for data[i] <= '<' { i++ }
    if data[i:i+6] != "entry>" { goto err }
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
      if data[i] == '/' { // </entry>
        i+=7
        auditScanFunc(entry)
        break
      }
      
      k := i
      etree := tree
      for {
        etree = etree.link[data[k]]
        if etree == nil { goto err }
        if etree.name != "" {
          k = i+len(etree.name)
          if etree.name != data[i:k] {
            // special case for <empty/> tag
            if etree.name[0:len(etree.name)-1] == data[i:k-1] && data[k-1] == '/' && data[k] == '>' {
              entry[etree.index] = ""
              i = k+1
              break
            } else {
              goto err
            }
          } else {
            i = nextTag(data,k)
            entry[etree.index] = data[k:i]
            i+=len(etree.name)+2 // 2 for <, /  (the > is include in etree.name)
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

type elementTree struct {
  name string // something like "version>", i.e. without '<' but with '>'
  index int
  link [256]*elementTree // includes extra entry for '/' as in <foo/>
}
