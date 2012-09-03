/* 
Copyright (c) 2012 Matthias S. Benkmann

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
         "time"
         "../xml"
         "../config"
         "../util"
       )

// Handles the message "gosa_query_jobdb".
//  encrypted: the original encrypted message
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_query_jobdb(encrypted string, xmlmsg *xml.Hash) string {
  // NOTE: It's important that the channel is buffered to avoid leaking
  // a goroutine in the case of timeout. See here:
  // http://golang.org/doc/articles/concurrency_patterns.html
  fallback_jobdb := make(chan string, 1)
  go func() {
    fallback_jobdb <- fallback(encrypted)
  }()
  
  var fallback_xml *xml.Hash
  timeout := time.NewTimer(5 * time.Second)
  select {
    case fb := <- fallback_jobdb :  
                          timeout.Stop() // so that it can be GC'd immediately
                          decrypted := ""
                          for _, key := range config.ModuleKeys {
                            decrypted = GosaDecrypt(fb, key)
                            if decrypted != "" { break }
                          }
                          if decrypted == "" {
                            util.Log(0, "ERROR! gosa_query_jobdb(): Could not decrypt fallback jobdb: %v", fb)
                            decrypted = "<xml></xml>"
                          }
                          
                          fallback_xml, xmlerr := xml.StringToHash(decrypted)
                          if xmlerr != nil {
                            util.Log(0, "ERROR! gosa_query_jobdb(): Error parsing fallback jobdb: %v", xmlerr)
                          }
                          
    case <- timeout.C :
                          util.Log(0, "ERROR! gosa_query_jobdb(): Timeout waiting for fallback jobdb")
                          fallback_xml = xml.NewHash("xml")
  }
  
  filter := xml.XMLToFilter(xmlmsg.First("where"))
  jobdb_xml := JobDB.Query(filter)
  makeAnswerList(jobdb_xml, fallback_xml)
  
  return jobdb_xml.String()
}

// Fixes lst so that the outer element is <xml> and the children are
// <answerXX> with <id>XX</id>. If additional are provided, they will be merged
// into lst.
func makeAnswerList(lst *xml.Hash, additional... *xml.Hash) {
  TODO()  
}

