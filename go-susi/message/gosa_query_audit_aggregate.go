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

package message

import (
         "strings"
         "strconv"
         
         "../db"
         "../xml"
         "../config"
         "../security"
         
         "github.com/mbenkmann/golib/util"
       )

// Handles the message "gosa_query_audit_aggregate".
//  xmlmsg: the decrypted and parsed message
//  context: the security context
// Returns:
//  reply as Hash (with <xml> as outer element)
func gosa_query_audit_aggregate(xmlmsg *xml.Hash, context *security.Context) *xml.Hash {
  /*
    A huge factor that determines query time is scanning the subdirectories
    for file names. If they are all in the OS cache, everything is fine. If not
    the query may take forever.
    Queries that select only results from a single MAC can be improved drastically
    by pulling that MAC out of the search filter and putting it in optimize_mac.
  */
  optimize_mac := ""

  /*
    A query with an exact match test can be made faster by converting it into
    a string such as "<key>foo</key>" and putting it into optimize_contains.
    optimize_contains is checked before the XML parsing step with a simple
    byte scan over the file data.
  */
  optimize_contains := ""
  
  timestamp1 := xmlmsg.Text("tstart")
  if timestamp1 == "" { timestamp1 = "0000_01_01_00_00_00" }
  timestamp2 := xmlmsg.Text("tend")
  if timestamp2 == "" { timestamp2 = "9999_12_31_23_59_59" }
  includeothers := (xmlmsg.First("includeothers") != nil)
  fname := xmlmsg.Text("audit")
  if fname == "" {
    util.Log(0, "ERROR! gosa_query_audit_aggregate: Need non-empty <audit> element")
    fname = "......"
  }

  audit := xml.NewHash("xml","header","query_audit_aggregate")
  audit.Add("source", config.ServerSourceAddress)
  audit.Add("target", xmlmsg.Text("source"))

  prop_index := map[string]int{}
  props := []string{}
  for _, col := range xmlmsg.Get("select") {
    if _, have_prop := prop_index[col]; have_prop { continue } // skip duplicates
    prop_index[col] = len(props)
    props = append(props, col)
  }

  selected := len(props) // only props[0],...,props[selected-1] will be returned in answers, the others are just for query purposes
  macindex, have_prop := prop_index["macaddress"]
  if !have_prop {
    macindex = len(props)
    props = append(props, "macaddress")
    prop_index["macaddress"] = macindex
  }
  
  aggregates := []*aggspec{}
  
  have_aggregate_name := map[string]bool{}
  for count := xmlmsg.First("count"); count != nil; count = count.Next() {
    name := strings.TrimSpace(count.Text("as"))
    if name == "" {
      util.Log(0, "WARNING! gosa_query_audit_aggregate: Ignoring <count> with empty/missing <as>")
      continue
    }
    if prop_index[name] < selected {
      util.Log(0, "WARNING! gosa_query_audit_aggregate: Ignoring <count><as>%v because of conflict with <select>%v", name, name)
      continue
    }
    if have_aggregate_name[name] {
      util.Log(0, "WARNING! gosa_query_audit_aggregate: Ignoring re-definition of <count><as>%v", name)
      continue
    }
    
    uindexes := []int{}
    
    for unique := count.First("unique"); unique != nil; unique = count.Next() {
      uname := strings.TrimSpace(unique.Text())
      if uname == "" {
        util.Log(0, "WARNING! gosa_query_audit_aggregate: Ignoring empty <unique>")
        continue
      }
      uindex, have_prop := prop_index[uname]
      if !have_prop {
        uindex = len(props)
        props = append(props, uname)
        prop_index[uname] = uindex
      }
      uindexes = append(uindexes, uindex)
    }
    
    where := count.First("where")
    if where == nil { where = xml.NewHash("where") }
    filter, err :=whereFilter(where, prop_index, &props)
    if err != nil {
      util.Log(0, "ERROR! gosa_query_audit_aggregate: Error parsing <where> of <count><as>%v: %v", name, err)
      filter = filterNone
    }

    aggregates = append(aggregates, &aggspec{name,filter,uindexes})
  }

  known := map[string]bool{}
  answers := map[string]*aggresult{}
  masterAggregate := make([]int, len(aggregates))
  masterAggregateUnique := make([]map[string]bool, len(aggregates))

  issue_warning := true // issue a warning if answer limit exceeded  

  f := func(entry []string){
    mac := entry[macindex]
    known[mac] = true
    
    result_key := ""
    
    if selected > 0 {
      for i := 0; i < selected; i++ {
        result_key += entry[i] + "<" // use < as separator because it cannot occur in entry[i] because it comes from XML text.
      }
      if answers[result_key] == nil {
        if len(answers) < context.Limits.MaxAnswers {
          answers[result_key] = &aggresult{make([]string,selected), 
                                           make([]int,len(aggregates)),
                                           make([]map[string]bool, len(aggregates))}
          copy(answers[result_key].Selected, entry[0:selected])
        } else {
          result_key = ""
          if issue_warning {
            util.Log(0, "WARNING! [SECURITY] Request from %v generated too many answers => Truncating answer list\n", context.PeerID.IP.String())
            issue_warning = false
          }
        }
      }
    }

    for i := range aggregates {
      if aggregates[i].Filter.Accepts(entry) {
        // if aggregates[i].Type == COUNT {
        uni := ""
        for _, k := range aggregates[i].UniqueIndex {
          uni += entry[k] + "<"
        }
        if masterAggregateUnique[i][uni] == false {
          masterAggregate[i]++
          if uni != "" {
            masterAggregateUnique[i][uni] = true
          }
        }
        if result_key != "" {
          if answers[result_key].AggregateUniqueMap[i][uni] == false {
            answers[result_key].Aggregate[i]++
            if uni != "" {
              answers[result_key].AggregateUniqueMap[i][uni] = true
            }
          }
        }
        // }
      }
    }
  }
  
  _, noaudit, unknown := db.AuditScanSubdirs(config.FAILogPath, timestamp1, timestamp2, fname, optimize_mac, optimize_contains,f, props, includeothers)
  
  audit.Add("known", strconv.Itoa(len(known)))
  audit.Add("unknown", strconv.Itoa(unknown))
  
  for i := range noaudit {
    na := audit.Add("noaudit")
    addAuditID(na, &noaudit[i])
  }
  
  masterAgg := audit.Add("aggregate")
  for i := range aggregates {
    masterAgg.Add(aggregates[i].Name, masterAggregate[i])
  }
  
  var count uint64 = 1
  for k := range answers {
    answer := audit.Add("answer"+strconv.FormatUint(count, 10))
    for i := 0; i < selected; i++ {
      answer.Add(props[i], answers[k].Selected[i])
    }
    for i := range aggregates {
      answer.Add(aggregates[i].Name, answers[k].Aggregate[i])
    }
    count++
  }


  return audit
}

type aggspec struct {
  Name string
  Filter entryFilter
  UniqueIndex []int
}

type aggresult struct {
  Selected []string
  Aggregate []int
  // AggregateUniqueMap[i] maps the concatenation of all
  // columns specified by aggregates[i].UniqueIndex to a bool giving whether
  // or not that combination has been counted yet.
  AggregateUniqueMap []map[string]bool
}


/*


HELPER FUNCTIONS AND CLASSES ARE IN gosa_query_audit.go


*/