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
         "fmt"
         "net"
         "time"
         "strconv"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

// Handles the message "gosa_query_jobdb".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_query_jobdb(xmlmsg *xml.Hash) string {
  where := xmlmsg.First("where")
  if where == nil { where = xml.NewHash("where") }
  filter, err := xml.WhereFilter(where)
  if err != nil {
    util.Log(0, "ERROR! gosa_query_jobdb: Error parsing <where>: %v", err)
    filter = xml.FilterNone
  }
  
  // If necessary, wait for the effects of forwarded modify requests
  t_forward := db.MostRecentForwardModifyRequestTime.At(0).(time.Time)
  delay := config.GosaQueryJobdbMaxDelay - time.Since(t_forward)
  if delay > 0 { time.Sleep(delay) }
  
  jobdb_xml := db.JobsQuery(filter)

  // maps IP:PORT to a string representation of that peer's downtime
  // the empty string represents a peer that is up
  downtime := map[string]string{config.ServerSourceAddress:""}
  
  // maps IP:PORT to server name
  servername := map[string]string{}
  
  var count uint64 = 1
  for _, tag := range jobdb_xml.Subtags() {
    answer := jobdb_xml.RemoveFirst(tag)
    for ; answer != nil; answer = jobdb_xml.RemoveFirst(tag) {
      siserver := answer.Text("siserver")
      
      // If we encounter this siserver for the first time,
      // get its downtime (if any) and cache it.
      if _, found := downtime[siserver] ; !found {
        dur := Peer(siserver).Downtime()
        if t := dur/time.Second; t == 0 {
          downtime[siserver] = ""
        } else {
          downtime[siserver] = verbalDuration(dur)
        }
      }
      
      // If the server is down, set status="error" and result=<error message>
      if downtime[siserver] != "" {    
        
        // Look up server name if we don't have it cached, yet.
        if _, have := servername[siserver] ; !have {
          servername[siserver] = siserver
          host,_,err := net.SplitHostPort(siserver)
          if err != nil {
            util.Log(0, "ERROR! SplitHostPort(%v): %v",siserver,err)
          } else {
            names, err := net.LookupAddr(host)
            if err != nil {
              util.Log(0, "ERROR! LookupAddr: %v",err)
            } else {
              if len(names) == 0 { names = []string{siserver} }
              servername[siserver] = names[0]
            }
          }
        }
        
        answer.FirstOrAdd("status").SetText("error")
        answer.FirstOrAdd("result").SetText("%v has been down for %v.",servername[siserver],downtime[siserver])
      }
       
      answer.Rename("answer"+strconv.FormatUint(count, 10))
      jobdb_xml.AddWithOwnership(answer)
      count++
    }
  }
  
  jobdb_xml.Add("header", "query_jobdb")
  jobdb_xml.Add("source", config.ServerSourceAddress)
  jobdb_xml.Add("target", xmlmsg.Text("source"))
  jobdb_xml.Add("session_id", "1")
  jobdb_xml.Rename("xml")
  return jobdb_xml.String()
}

var unit_name = []string{"second","minute","hour","day","week","month","year","decade","century","millenium"}
var unit_name_plural = []string{"seconds","minutes","hours","days","weeks","months","years","decades","centuries","millenia"}
var unit_divisor = []float64{60  , 60   , 24  , 7   , 4.348125 , 12   , 10     ,   10    ,  10 }

func verbalDuration(dur time.Duration) string {
  t := float64(dur/time.Second)
  for i := range unit_divisor {
    p := t/unit_divisor[i]
    if p > 0.90 {
      t = p
      continue
    }
    
    switch {
      case t < 0.99: return fmt.Sprintf("almost 1 %v",unit_name[i])
      case t < 1.4: return fmt.Sprintf("more than 1 %v",unit_name[i])
      case t < 1.6: return fmt.Sprintf("1 1/2 %v",unit_name_plural[i])
      case t < 1.9: return fmt.Sprintf("more than 1 1/2 %v",unit_name_plural[i])
      case t < 1.99: return fmt.Sprintf("almost 2 %v",unit_name_plural[i])
      default: u := int(t+0.1)
               half := ""
               if int(t+0.6) > u { half = " 1/2" }
               return fmt.Sprintf("%d%v %v",u,half,unit_name_plural[i])
    }
  }
  
  return "ages"
}

