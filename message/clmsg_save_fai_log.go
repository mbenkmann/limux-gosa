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
         "io/ioutil"
         "os"
         "path"
         "time"
         "regexp"
         "strings"
         "encoding/base64"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

var actionRegexp = regexp.MustCompile("^[_a-zA-Z-]+$")

// Handles the message "CLMSG_save_fai_log".
//  xmlmsg: the decrypted and parsed message
func clmsg_save_fai_log(xmlmsg *xml.Hash) {
  data := xmlmsg.Text("CLMSG_save_fai_log")
  macaddress := xmlmsg.Text("macaddress")
  action := xmlmsg.Text("fai_action")
  if !macAddressRegexp.MatchString(macaddress) {
    util.Log(0, "ERROR! CLMSG_save_fai_log with illegal <macaddress> \"%v\"",macaddress)
    return
  }
  
  if !actionRegexp.MatchString(action) {
    util.Log(0, "ERROR! CLMSG_save_fai_log with illegal <fai_action> \"%v\"",action)
    return
  }
  
  timestamp := util.MakeTimestamp(time.Now())
  logname := action+"_"+timestamp[0:8]+"_"+timestamp[8:]
  logdir := path.Join(config.FAILogPath, macaddress, logname)
  
  // NOTE: 1kB = 1000B, 1kiB = 1024B
  util.Log(1, "INFO! Storing %vkB of %v log files from %v in %v",len(data)/1000, action, macaddress, logdir)
  
  err := os.MkdirAll(logdir, 0755)
  if err != nil {
    util.Log(0, "ERROR! Error creating log directory \"%v\": %v", logdir, err)
    return
  }
  
  // Create convenience symlink with the system's name as alias for MAC address.
  go util.WithPanicHandler(func() {
    if plainname := db.SystemPlainnameForMAC(macaddress); plainname != "none" {
      err := os.Symlink(macaddress, path.Join(config.FAILogPath, plainname))
      if err != nil && !os.IsExist(err.(*os.LinkError).Err) {
        util.Log(0, "ERROR! Could not create symlink %v => %v: %v", path.Join(config.FAILogPath, plainname), macaddress, err)
      }
    }
  })

  // Remove all whitespace from data. This works around the issue that gosa-si-client
  // inserts spurious whitespace into base64 data which breaks Go's base64 decoder.
  data = strings.Join(strings.Fields(data),"")
  
  for _, logfile := range strings.Split(data, "log_file:") {
    colon := strings.Index(logfile,":")
    if colon < 1 { continue }
    
    fname := logfile[0:colon]
    util.Log(1, "INFO! Processing \"%v\" (%vkB)", fname, len(logfile)/1000)
    
    logdata, err := base64.StdEncoding.DecodeString(logfile[colon+1:])
    if err != nil {
      util.Log(0, "ERROR! base64 decoding failed for \"%v\": %v", fname, err)
      continue
    }
    
    // As a precaution, make sure fname contains no slashes.
    fname = strings.Replace(fname,"/","_",-1)
    err = ioutil.WriteFile(path.Join(logdir, fname), logdata, 0644)
    if err != nil {
      util.Log(0, "ERROR! Could not store \"%v\": %v", path.Join(logdir, fname), err)
      continue
    }
  }
}
