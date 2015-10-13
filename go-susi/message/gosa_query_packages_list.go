/*
Copyright (c) 2013 Matthias S. Benkmann

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

package message

import (
         "strconv"
         
         "../db"
         "../xml"
         "github.com/mbenkmann/golib/util"
         "../config"
       )

// Handles the message "gosa_query_packages_list".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_query_packages_list(xmlmsg *xml.Hash) *xml.Hash {
  where := xmlmsg.First("where")
  if where == nil { where = xml.NewHash("where") }
  filter, err := xml.WhereFilter(where)
  if err != nil {
    util.Log(0, "ERROR! gosa_query_packages_list: Error parsing <where>: %v", err)
    filter = xml.FilterNone
  }
  
  /*
    GOsa does not understand package entries with data from multiple releases,
    so we have to clean up the replies to make sure there is only one 
    "distribution" element. We examine the query to find the requested
    distribution. The following code is primitive and only understands the
    specific query format used by GOsa. Should GOsa's query format ever be
    changed, the correct way to deal with this is to make GOsa understand
    entries with merged info from multiple releases and to remove the cleanup
    code here altogether.
  */
  distribution := ""
  for clause := where.First("clause"); clause != nil; clause = clause.Next() {
    phrase := clause.First("phrase")
    if phrase == nil { continue } // ignore empty clause
    if phrase.Next() != nil { continue } // don't understand multi-phrase clause
    if phrase.First("operator") != nil { continue } // don't understand operators
    distele := phrase.FirstChild().Element()
    if distele.Name() != "distribution" { continue } // not the element we're looking for
    distribution = distele.Text()
  }
  
  packagesdb := db.FAIPackages(filter)
  packages := xml.NewHash("xml","header","query_packages_list")
  
  var count uint64 = 1
  for child := packagesdb.FirstChild(); child != nil; child = child.Next() {
    answer := child.Remove()
    answer.Rename("answer"+strconv.FormatUint(count, 10))
    cleanup(distribution, answer)
    packages.AddWithOwnership(answer)
    count++
  }
  
  packages.Add("source", config.ServerSourceAddress)
  packages.Add("target", xmlmsg.Text("source"))
  packages.Add("session_id", "1")
  return packages
}

// If x contains data from multiple releases, remove all data except that
// for release distribution. Also make sure there is only 1 element of each name
// (in particular only 1 <version>). If distribution == "" or does not occur in
// x, then the last release in x is retained. Likewise among multiple <version>
// elements for a particular release it is the last one that is kept.
func cleanup(distribution string, x *xml.Hash) {
  distribution_idx := -1
  last_distribution_idx := -1
  version_idx := -1
  section_idx := -1
  description_idx := -1
  template_idx := -1
  timestamp_idx := -1
  package_idx := 0
  
  i := 0
  mode := 0
  for child := x.FirstChild(); child != nil; child = child.Next() {
    ele := child.Element()
    if mode == 1 && ele.Name() != "distribution" { mode = 2 }
    switch ele.Name() {
      case "package" :     package_idx = i
      case "distribution": last_distribution_idx = i
                           if mode == 0 {
                             if ele.Text() == distribution {
                               distribution_idx = i
                               mode = 1
                             }
                           } else {
                             if mode == 2 { mode = 3 }
                           }
      case "version":      if version_idx < 0 || mode < 3 { version_idx = i }
      case "section":      if section_idx < 0 || mode < 3 { section_idx = i }
      case "description":  if description_idx < 0 || mode < 3 { description_idx = i }
      case "template":     if template_idx < 0    || mode < 3 { template_idx = i }
      case "timestamp":    if timestamp_idx < 0   || mode < 3 { timestamp_idx = i }
    }
    i++
  }
  
  keep := make([]bool, i)
  keep[package_idx] = true
  if distribution_idx >= 0 { 
    keep[distribution_idx] = true 
  } else {
    keep[last_distribution_idx] = true 
  }
  if version_idx >= 0 { keep[version_idx] = true }
  if section_idx >= 0 { keep[section_idx] = true }
  if description_idx >= 0 { keep[description_idx] = true }
  if template_idx >= 0 { keep[template_idx] = true }
  if timestamp_idx >= 0 { keep[timestamp_idx] = true }
  
  i = 0
  for child := x.FirstChild(); child != nil; child = child.Next() {
    if !keep[i] { child.Remove() }
    i++
  }
}
