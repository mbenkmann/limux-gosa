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
         
         "../db"
         "../util"
         "../bytes"
         "../config"
       )

var actionRegexp = regexp.MustCompile("^[_a-zA-Z-]+$")

func match(data []byte, i int, s string) bool {
  for k := range s {
    if i+k == len(data) { return false }
    if data[i+k] != s[k] { return false }
  }
  return true
}

// Handles the message "CLMSG_save_fai_log".
//  buf: the decrypted message
func clmsg_save_fai_log(buf *bytes.Buffer) {
  macaddress := ""
  action := ""
  start := 0
  end := 0
  data := buf.Bytes()
  for i := 0; i < len(data)-19; i++ {
    if data[i] == '<' {
      if i+12+17 <= len(data) && match(data, i, "<macaddress>") {
        macaddress = string(data[i+12:i+12+17])
      } else 
      if match(data, i, "<fai_action>") {
        for k := i + 12; k < len(data); k++ {
          if data[k] == '<' {
            action = string(data[i+12:k])
            i = k
            break
          }
        }
      } else
      if match(data, i, "<CLMSG_save_fai_log>") {
        start = i+20
      } else
      if match(data, i, "</CLMSG_save_fai_log>") {
        end = i
      }
    }
  }

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

  files := []int{}
  for i := start; i < end; i++ {
    if data[i] == ':' && match(data, i-8, "log_file") {
      k := i
      i++
      for i < end { 
        if data[i] == ':' { 
          if k+1 < i { files = append(files, k+1, i) }
          break 
        }
        i++
      }
    }
  }
  
  files = append(files, end+8)
  
  for i := 0; i < len(files)-1; i+=2 {
    fname := string(data[files[i]:files[i+1]])
    logdata := data[files[i+1]+1:files[i+2]-8]
    util.Log(1, "INFO! Processing \"%v\" (%vkB)", fname, len(logdata)/1000)
    
    logdata = util.Base64DecodeInPlace(logdata)
    
    // As a precaution, make sure fname contains no slashes.
    fname = strings.Replace(fname,"/","_",-1)
    err = ioutil.WriteFile(path.Join(logdir, fname), logdata, 0644)
    if err != nil {
      util.Log(0, "ERROR! Could not store \"%v\": %v", path.Join(logdir, fname), err)
      continue
    }
  }
}
