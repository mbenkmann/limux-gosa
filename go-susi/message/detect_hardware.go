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
*/

package message

import (
         "time"
         "os"
         "os/exec"
         "strings"
         
         "../xml"
         "../util"
         "../config"
       )

// Handles "detect_hardware".
//  xmlmsg: the decrypted and parsed message
func detect_hardware(xmlmsg *xml.Hash) {
  server := xmlmsg.Text("source")
  if server == "" {
    util.Log(0, "ERROR! Received detect_hardware from unknown source")
    return
  }

  c := make(chan *xml.Hash, 2)
  go func(){
    time.Sleep(config.DetectHardwareTimeout)
    c <- nil
  }()
  
  go util.WithPanicHandler(func(){sendDetectedHardwareReply(server, c)})

  start := time.Now()
  env := config.HookEnvironment()
  for _, tag := range xmlmsg.Subtags() {
    env = append(env, tag+"="+strings.Join(xmlmsg.Get(tag),"\n"))
  }
  cmd := exec.Command(config.DetectHardwareHookPath)
  env = append(env, "xml="+xmlmsg.String())
  cmd.Env = append(env, os.Environ()...)
  util.Log(1, "INFO! Running detect-hardware-hook %v with parameters %v", config.DetectHardwareHookPath, env)
  hwlist, err := xml.LdifToHash("detected_hardware", false, cmd) // !!C'n'P WARNING: casefold=false!!
  if err != nil {
    util.Log(0, "ERROR! detect-hardware-hook %v: %v", config.DetectHardwareHookPath, err)
    return
  }
  util.Log(1, "INFO! Finished detect-hardware-hook. Running time: %v", time.Since(start))
  for hwlist.RemoveFirst("dn") != nil {} // dn is ignored (see manual)
  util.Log(2, "DEBUG! Hardware detection result: %v", hwlist)
  
  c <- hwlist
}

func sendDetectedHardwareReply(target string, c <-chan *xml.Hash) {
  hwlist := <-c
  if hwlist == nil {
    util.Log(0, "ERROR! detect-hardware-hook timed out => Sending default detected_hardware message to server")
    hwlist = xml.NewHash("xml","detected_hardware","ghCpuType", "Z80/4Mhz")
  }
  hwlist.Add("header", "detected_hardware")
  hwlist.Add("source", config.ServerSourceAddress)
  hwlist.Add("target", target)
  clientpackageskey := config.ModuleKey["[ClientPackages]"]
  // If [ClientPackages]/key missing, take the last key in the list
  // (We don't take the 1st because that would be "dummy-key").
  if clientpackageskey == "" { clientpackageskey = config.ModuleKeys[len(config.ModuleKeys)-1] }
  util.Log(2, "DEBUG! Sending detected_hardware to %v: %v", target, hwlist)
  util.SendLnTo(target, GosaEncrypt(hwlist.String(), clientpackageskey), config.Timeout)
}
