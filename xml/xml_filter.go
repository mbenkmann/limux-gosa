/* Copyright (C) 2012 Matthias S. Benkmann
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this file (originally named xml_filter.go) and associated documentation files 
 * (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is furnished
 * to do so, subject to the following conditions:
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE. 
 */


package xml

import (
         "fmt"
         "math"
         "bytes"
         "regexp"
         "strings"
         "strconv"
       )

// Replaces "%" with ".*" and "_" with ".".
var replaceLikeMetaChars = strings.NewReplacer("%",".*","_",".")

// Interface for selecting certain items from the database.
type HashFilter interface{
  // Returns true if the given item should be in the result set. 
  // IMPORTANT! Must return false for a nil argument.
  Accepts(item *Hash) bool
}

// the type of FilterAll
type filterall struct{}
// Always returns true for non-nil items.
func (*filterall) Accepts(item *Hash) bool { return item != nil }
// HashFilter that accepts all non-nil items.
var FilterAll *filterall = &filterall{}

// the type of FilterNone
type filternone struct{}
// Always returns false.
func (*filternone) Accepts(item *Hash) bool { return false }
// HashFilter that does not accept any item.
var FilterNone *filternone = &filternone{}

type filterand []HashFilter

func (self *filterand) Accepts(item *Hash) bool {
  if item == nil { return false }
  for _, f := range *self {
    if !f.Accepts(item) { return false }
  }
  return true
}

// Returns a HashFilter that accepts an item if all elements of filter accept it.
// If no filter argument is provided, the resulting
// filter will accept everything but nil.
func FilterAnd(filter []HashFilter) HashFilter {
  var f filterand = make([]HashFilter, len(filter))
  copy(f, filter)
  return &f
}

type filteror []HashFilter

func (self *filteror) Accepts(item *Hash) bool {
  if item == nil { return false }
  for _, f := range *self {
    if f.Accepts(item) { return true }
  }
  return false
}

// Returns a HashFilter that accepts an item if any element of filter accepts it.
// If no filter argument is provided, the resulting
// filter will accept nothing.
func FilterOr(filter []HashFilter) HashFilter {
  var f filteror = make([]HashFilter, len(filter))
  copy(f, filter)
  return &f
}


type filterregexp struct {
  column string
  re *regexp.Regexp
}

func (self *filterregexp) Accepts(item *Hash) bool { 
  if item == nil { return false }
  for _, value := range item.Get(self.column) {
    if self.re.MatchString(value) { return true }
  }
  return false
}

// Returns a HashFilter that accepts an item if it has at least one subelement
// named column whose contents match regexp. If the item has multiple column
// subelements, it is enough if one of them matches. 
// If regexp is invalid, this function panics.
func FilterRegexp(column, regex string) HashFilter {
  return &filterregexp{column, regexp.MustCompile(regex)}
}

type filternot struct {
  HashFilter
}

func (self *filternot) Accepts(item *Hash) bool {
  if item == nil { return false }
  return !self.HashFilter.Accepts(item)
}

// Returns a HashFilter that accepts everything that filter does not (except for
// nil which is never accepted).
func FilterNot(filter HashFilter) HashFilter {
  return &filternot{filter}
}


type filterrel struct {
  column string
  compare_value string
  compare_number int64
  accept1 int
  accept2 int
}

func (self *filterrel) Accepts(item *Hash) bool {
  if item == nil { return false }
  for _, value := range item.Get(self.column) {
    if self.compare_number != math.MinInt64 {
      valnum, err := strconv.ParseInt(value, 10, 64)
      if err == nil {
        cmp := 0
        if valnum < self.compare_number { cmp = -1 } else
        if valnum > self.compare_number { cmp = 1 }
        if self.accept1 == cmp || self.accept2 == cmp { return true }
        
        continue
      }
    }
    
    cmp := bytes.Compare([]byte(value), []byte(self.compare_value))
    if self.accept1 == cmp || self.accept2 == cmp { return true }
  }
  return false
}

// Returns a HashFilter that accepts an item if it has at least one subelement 
// named column whose content has one of the 2 accepted relationships to 
// compare_value. accept1 and accept2 can be 0, -1 or +1 for equality, less-than
// or greater-than relationships. The comparison is attempted numerically first
// and if either compare_value or the column value can not be converted to a number
// a lexicographic comparison is performed (on the bytes of the strings, not the
// utf-8 code points).
func FilterRel(column, compare_value string, accept1, accept2 int) HashFilter {
  num, err := strconv.ParseInt(compare_value, 10, 64)
  if err != nil { num = math.MinInt64 }
  return &filterrel{column, compare_value, num, accept1, accept2}
}


// Returns a HashFilter for the where expression passed as argument. In
// case of an error, nil and the error are returned.
//
// A where expression looks like this:
//   <where>
//        <clause>
//            <connector>or</connector>
//            <phrase>
//                <operator>eq</operator>
//                <macaddress>00:1d:60:7e:9b:f6</macaddress>
//            </phrase>
//        </clause>
//   </where>
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
//              Possible values for <connector> are "AND" and "OR" (case-insensitive).
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
func WhereFilter(where *Hash) (HashFilter, error) {
  if where.Name() != "where" {
    return nil, fmt.Errorf("Wrapper element must be 'where', not '%v'", where.Name())
  }
  
  clauses := make([]HashFilter, 1, 0)
  for clause := where.First("clause"); clause != nil; clause = clause.Next() {
    connector := strings.ToLower(clause.Text("connector"))
    if connector == "" { connector = "and" }
    if connector != "and" && connector != "or" {
      return nil, fmt.Errorf("Only 'and' and 'or' are allowed as <connector>, not '%v'", connector)
    }
    
    phrases := make([]HashFilter, 1, 0)
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
      
      var phrase_filter HashFilter
      switch operator {
        case "eq": phrase_filter = FilterRegexp(column, "^"+regexp.QuoteMeta(compare_value)+"$")
        case "ne": phrase_filter = FilterNot(FilterRegexp(column, "^"+regexp.QuoteMeta(compare_value)+"$"))
        case "ge": phrase_filter = FilterRel(column, compare_value, 1, 0)
        case "gt": phrase_filter = FilterRel(column, compare_value, 1, 1)
        case "le": phrase_filter = FilterRel(column, compare_value, -1, 0)
        case "lt": phrase_filter = FilterRel(column, compare_value, -1, -1)
        case "like": phrase_filter = FilterRegexp(column, "^(?i)"+replaceLikeMetaChars.Replace(regexp.QuoteMeta(compare_value))+"$")
        case "unlike": phrase_filter = FilterNot(FilterRegexp(column, "^(?i)"+replaceLikeMetaChars.Replace(regexp.QuoteMeta(compare_value))+"$"))
        default: return nil, fmt.Errorf("Unsupported <operator>: %v", operator)
      }
    
      phrases = append(phrases, phrase_filter)
    }
    
    var clause_filter HashFilter
    if connector == "and" {
      clause_filter = FilterAnd(phrases)
    } else
    {
      clause_filter = FilterOr(phrases)
    }
    
    clauses = append(clauses, clause_filter)
  }
  
  return FilterAnd(clauses), nil
}
