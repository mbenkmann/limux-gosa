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
         "os"
         "fmt"
         "path"
         "sort"
         "strings"
         
         "../xml"
         "github.com/mbenkmann/golib/util"
         "../config"
       )

// Handles the message "gosa_show_log_files_by_date_and_mac".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_show_log_files_by_date_and_mac(xmlmsg *xml.Hash) *xml.Hash {
  macaddress := xmlmsg.Text("mac")
  lmac := strings.ToLower(macaddress)
  subdir := xmlmsg.Text("date")
  
  if !macAddressRegexp.MatchString(macaddress) {
    emsg := fmt.Sprintf("Illegal or missing <mac> element in message: %v", xmlmsg)
    util.Log(0, "ERROR! %v", emsg)
    return ErrorReplyXML(emsg)
  }
  
  // As a precaution, make sure subdir contains no slashes.
  subdir = strings.Replace(subdir,"/","_",-1)
  
  if subdir == "" {
    emsg := fmt.Sprintf("Missing or empty <date> element in message: %v", xmlmsg)
    util.Log(0, "ERROR! %v", emsg)
    return ErrorReplyXML(emsg)
  }
  
  header := "show_log_files_by_date_and_mac"
  x := xml.NewHash("xml","header", header)
  x.Add(header)
  
  logdir := path.Join(config.FAILogPath, lmac, subdir)
  
  util.Log(2, "DEBUG! Listing log files from %v", logdir)
  
  names := []string{}
  
  dir, err := os.Open(logdir)
  if err == nil || !os.IsNotExist(err.(*os.PathError).Err) {
    if err != nil {
      util.Log(0, "ERROR! gosa_show_log_files_by_date_and_mac: %v", err)
    } else {
      defer dir.Close()
      
      fi, err := dir.Readdir(0)
      if err != nil {
        util.Log(0, "ERROR! gosa_show_log_files_by_date_and_mac: %v", err)
      } else {
        for _, info := range fi {
          // only list ordinary files
          if info.Mode() &^ os.ModePerm == 0 { names = append(names, info.Name()) }
        }
        sort.Strings(names)
        for _, n := range names { x.Add(header, n) }
      }
    }
  }
  
  x.Add("source", config.ServerSourceAddress)
  x.Add("target", "GOSA")
  x.Add("session_id","1")
  return x
}
