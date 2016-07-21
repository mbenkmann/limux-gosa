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
         "fmt"
         "math"
         "bytes"
         "regexp"
         "strings"
         "strconv"
         
         "../db"
         "../xml"
         "../config"
         "../security"
         
         "github.com/mbenkmann/golib/util"
       )

// Handles the message "gosa_query_audit".
//  xmlmsg: the decrypted and parsed message
//  context: the security context
// Returns:
//  reply as Hash (with <xml> as outer element)
func gosa_query_audit(xmlmsg *xml.Hash, context *security.Context) *xml.Hash {
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
    util.Log(0, "ERROR! gosa_query_audit: Need non-empty <audit> element")
    fname = "......"
  }

  audit := xml.NewHash("xml","header","query_audit")
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

  where := xmlmsg.First("where")
  if where == nil { where = xml.NewHash("where") }
  filter, err :=whereFilter(where, prop_index, &props)
  if err != nil {
    util.Log(0, "ERROR! gosa_query_fai_release: Error parsing <where>: %v", err)
    filter = filterNone
  }
  
  filter = limitFilter(filter, int64(context.Limits.MaxAnswers), context.PeerID.IP.String())

  known := map[string]bool{}
  match2 := map[string]bool{}
  nonmatch2 := map[string]db.AuditID{}
  var count uint64 = 1

  f := func(entry []string){
    mac := entry[macindex]
    known[mac] = true
    if filter.Accepts(entry) {
      match2[mac] = true
      answer := audit.Add("answer"+strconv.FormatUint(count, 10))
      for i := 0; i < selected; i++ {
        answer.Add(props[i], entry[i])
      }
      count++
    } else {
      if includeothers {
        if _, have_already := nonmatch2[mac]; !have_already {
          nonmatch2[mac] = db.AuditID{MAC:mac}
        }
      }
    }
  }
  
  nonmatch, noaudit, unknown := db.AuditScanSubdirs(config.FAILogPath, timestamp1, timestamp2, fname, optimize_mac, optimize_contains,f, props, includeothers)
  
  for _, nm2 := range nonmatch2 {
    if !match2[nm2.MAC] {
      nonmatch = append(nonmatch, nm2)
    }
  }
  
  for i := range nonmatch {
    nm := audit.Add("nonmatching")
    addAuditID(nm, &nonmatch[i])
  }
  for i := range noaudit {
    na := audit.Add("noaudit")
    addAuditID(na, &noaudit[i])
  }
  
  audit.Add("known", strconv.Itoa(len(known)))
  audit.Add("unknown", strconv.Itoa(unknown))

  return audit
}

func addAuditID(nm *xml.Hash, aid *db.AuditID) {
  if aid.Timestamp != "" {
    nm.Add("lastaudit", aid.Timestamp)
  }
  if aid.MAC != "" {
    nm.Add("macaddress", aid.MAC)
  }
  if aid.IP != "" {
    nm.Add("ipaddress", aid.IP)
  }
  if aid.Hostname != "" {
    nm.Add("hostname", aid.Hostname)
  }
}

type limitfilter struct {
  f entryFilter
  max int64
  requester string
}

func (f *limitfilter) Accepts(item []string) bool {
  if f.max < 0 { return false }
  accepts := f.f.Accepts(item)
  if accepts {
    if f.max--; f.max < 0 {
      util.Log(0, "WARNING! [SECURITY] Request from %v generated too many answers => Truncating answer list\n", f.requester)
      accepts = false
    }
  }
  return accepts
}


// Returns a filter that passes decisions on to filter f until
// f has accepted max entries (max<=0 means no limit).
// At that point a warning is logged
// and all further entries will be rejected.
// requester is a string that will be included in the warning as
// identifier of the party that made the request that caused
// excessive answers to be generated.
func limitFilter(f entryFilter, max int64, requester string) entryFilter {
  if max <= 0 { return f }
  return &limitfilter{f,max,requester}
}

// the type of FilterNone
type filternone struct{}
// Always returns false.
func (*filternone) Accepts(item []string) bool { return false }
// HashFilter that does not accept any item.
var filterNone *filternone = &filternone{}


// Replaces "%" with ".*" and "_" with ".".
var replaceLikeMetaChars = strings.NewReplacer("%",".*","_",".")

// Interface for selecting certain items from a hash.
type entryFilter interface{
  // Returns true if the given item should be in the result set. 
  // IMPORTANT! Must return false for a nil argument.
  Accepts(item []string) bool
}

type filterand []entryFilter

func (self *filterand) Accepts(item []string) bool {
  if item == nil { return false }
  for _, f := range *self {
    if !f.Accepts(item) { return false }
  }
  return true
}

// Returns a entryFilter that accepts an item if all elements of filter accept it.
// If no filter argument is provided, the resulting
// filter will accept everything but nil.
func filterAnd(filter []entryFilter) entryFilter {
  var f filterand = make([]entryFilter, len(filter))
  copy(f, filter) // create copy because someone could change the original array
  return &f
}

type filteror []entryFilter

func (self *filteror) Accepts(item []string) bool {
  if item == nil { return false }
  for _, f := range *self {
    if f.Accepts(item) { return true }
  }
  return false
}

// Returns a entryFilter that accepts an item if any element of filter accepts it.
// If no filter argument is provided, the resulting
// filter will accept nothing.
func filterOr(filter []entryFilter) entryFilter {
  var f filteror = make([]entryFilter, len(filter))
  copy(f, filter) // create copy because someone could change the original array
  return &f
}


type filterregexp struct {
  column int
  re *regexp.Regexp
}

func (self *filterregexp) Accepts(item []string) bool { 
  if item == nil { return false }
  return self.re.MatchString(item[self.column])
}

// Returns a entryFilter that accepts an item if the column with the specified
// index matches regexp. 
// If regexp is invalid or the column index is out of bounds, this function panics.
func filterRegexp(column int, regex string) entryFilter {
  return &filterregexp{column, regexp.MustCompile(regex)}
}

type filternot struct {
  entryFilter
}

func (self *filternot) Accepts(item []string) bool {
  if item == nil { return false }
  return !self.entryFilter.Accepts(item)
}

// Returns a entryFilter that accepts everything that filter does not (except for
// nil which is never accepted).
func filterNot(filter entryFilter) entryFilter {
  return &filternot{filter}
}


type filterrel struct {
  column int
  compare_value string
  compare_number int64
  accept1 int
  accept2 int
}

func (self *filterrel) Accepts(item []string) bool {
  if item == nil { return false }
  value := item[self.column] 
  {
    if self.compare_number != math.MinInt64 {
      valnum, err := strconv.ParseInt(value, 10, 64)
      if err == nil {
        cmp := 0
        if valnum < self.compare_number { cmp = -1 } else
        if valnum > self.compare_number { cmp = 1 }
        if self.accept1 == cmp || self.accept2 == cmp { return true }
        return false
      }
    }
    
    cmp := bytes.Compare([]byte(strings.ToLower(value)), []byte(strings.ToLower(self.compare_value)))
    if self.accept1 == cmp || self.accept2 == cmp { return true }
  }
  return false
}

// Returns a entryFilter that accepts an item if the column with the specified index
// has one of the 2 accepted relationships to 
// compare_value. accept1 and accept2 can be 0, -1 or +1 for equality, less-than
// or greater-than relationships. The comparison is attempted numerically first
// and if either compare_value or the column value can not be converted to a number
// a lexicographic comparison is performed (on the bytes of the strings after
// converting to lowercase, not the code points).
func filterRel(column int, compare_value string, accept1, accept2 int) entryFilter {
  num, err := strconv.ParseInt(compare_value, 10, 64)
  if err != nil { num = math.MinInt64 }
  return &filterrel{column, compare_value, num, accept1, accept2}
}

// Returns a entryFilter for the where expression passed as argument. In
// case of an error, nil and the error are returned.
// A where expression looks like this:
//
//   <where>
//         <clause>
//             <connector>or</connector>
//             <phrase>
//                <operator>eq</operator>
//                <macaddress>00:1d:60:7e:9b:f6</macaddress>
//             </phrase>
//         </clause>
//   </where>
//
// The tags have the following meaning:
//  <clause> (0 or more) A filter condition. All <clause> filter conditions 
//           within <where> are ANDed.
//
//  <connector> (0 or 1) If not provided, the default connector is "AND". 
//              All <phrase> filter conditions within a <clause> are 
//              combined by this operator like this:
//                   P1 c P2 c P3 c ... Pn
//              where Pi are the phrase filters and c is the connector. 
//              Possible values for <connector> are "AND" and "OR" 
//              (The case of the operator name doesn't matter).
//             
//  <phrase> (0 or more) A single primitive filter condition. In addition to 
//           one <operator> element (see below) a <phrase> must contain exactly
//           one other element. The element's name specifies the column name in 
//           the database and the element's text content the value to compare 
//           against. The comparison is performed according to <operator>.
//
//  <operator> (optional, assumed to be "eq" if missing) The comparison operator
//             for the <phrase>. Permitted operators are "eq", "ne", "ge", "gt", 
//             "le", "lt" with their obvious meanings, and "like" which performs
//             a case-insensitive match against a pattern that may include "%" 
//             to match any sequence of 0 or more characters and "_" to match
//             exactly one character. A literal "%" or "_" cannot be embedded
//             in such a pattern.
//             Operator names are case-insensitive.
//             All string comparisons are performed case-insensitive.
//             The operators "ge","gt","le" and "lt" try to convert their operands
//             to numbers and if that fails fall back to string comparison.
func whereFilter(where *xml.Hash, prop_index map[string]int, props *[]string) (entryFilter, error) {
  clauses := make([]entryFilter, 0, 1)
  for clause := where.First("clause"); clause != nil; clause = clause.Next() {
    connector := strings.ToLower(clause.Text("connector"))
    if connector == "" { connector = "and" }
    if connector != "and" && connector != "or" {
      return nil, fmt.Errorf("Only 'and' and 'or' are allowed as <connector>, not '%v'", connector)
    }
    
    phrases := make([]entryFilter, 0, 1)
    for phrase := clause.First("phrase"); phrase != nil; phrase = phrase.Next() {
      operator := strings.ToLower(phrase.Text("operator"))
      if operator == "" { operator = "eq" }
      
      column := ""
      for _, tag := range phrase.Subtags() {
        if tag != "operator" {
          if column != "" {
            return nil, fmt.Errorf("<phrase> may only contain one other element besides <operator>")
          }
          column = tag
        }
      }
      if column == "" {
        return nil, fmt.Errorf("<phrase> must have one other element besides <operator>")
      }
      
      compare_value := phrase.Text(column)
      col, have_prop := prop_index[column]
      if !have_prop {
        col = len(*props)
        *props = append(*props, column)
        prop_index[column] = col
      }
      
      var phrase_filter entryFilter
      switch operator {
        case "eq": phrase_filter = filterRegexp(col, "^(?i)"+regexp.QuoteMeta(compare_value)+"$")
        case "ne": phrase_filter = filterNot(filterRegexp(col, "^(?i)"+regexp.QuoteMeta(compare_value)+"$"))
        case "ge": phrase_filter = filterRel(col, compare_value, 1, 0)
        case "gt": phrase_filter = filterRel(col, compare_value, 1, 1)
        case "le": phrase_filter = filterRel(col, compare_value, -1, 0)
        case "lt": phrase_filter = filterRel(col, compare_value, -1, -1)
        case "like": phrase_filter = filterRegexp(col, "^(?i)"+replaceLikeMetaChars.Replace(regexp.QuoteMeta(compare_value))+"$")
        case "unlike": phrase_filter = filterNot(filterRegexp(col, "^(?i)"+replaceLikeMetaChars.Replace(regexp.QuoteMeta(compare_value))+"$"))
        default: return nil, fmt.Errorf("Unsupported <operator>: %v", operator)
      }
    
      phrases = append(phrases, phrase_filter)
    }
    
    var clause_filter entryFilter
    if connector == "and" {
      clause_filter = filterAnd(phrases)
    } else
    {
      clause_filter = filterOr(phrases)
    }
    
    clauses = append(clauses, clause_filter)
  }
  
  return filterAnd(clauses), nil
}
