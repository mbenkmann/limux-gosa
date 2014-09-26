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

package message

import (
         "strconv"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

// Handles the message "gosa_query_fai_release".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_query_fai_release(xmlmsg *xml.Hash) *xml.Hash {
  where := xmlmsg.First("where")
  if where == nil { where = xml.NewHash("where") }
  filter, err := xml.WhereFilter(where)
  if err != nil {
    util.Log(0, "ERROR! gosa_query_fai_release: Error parsing <where>: %v", err)
    filter = xml.FilterNone
  }
  
  faiclassesdb := db.FAIClasses(filter)
  faiclasses := xml.NewHash("xml","header","query_fai_release")
  
  var count uint64 = 1
  for child := faiclassesdb.FirstChild(); child != nil; child = child.Next() {
    answer := child.Remove()
    answer.Rename("answer"+strconv.FormatUint(count, 10))
    faiclasses.AddWithOwnership(answer)
    count++
  }
  
  faiclasses.Add("source", config.ServerSourceAddress)
  faiclasses.Add("target", xmlmsg.Text("source"))
  faiclasses.Add("session_id", "1")
  return faiclasses
}
