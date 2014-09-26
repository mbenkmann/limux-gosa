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

// Handles all messages of the form "trigger_action_*".
//  xmlmsg: the decrypted and parsed message
func trigger_action_foo(xmlmsg *xml.Hash) {
  start := time.Now()
  env := config.HookEnvironment()
  for _, tag := range xmlmsg.Subtags() {
    env = append(env, tag+"="+strings.Join(xmlmsg.Get(tag),"\n"))
  }
  cmd := exec.Command(config.TriggerActionHookPath)
  env = append(env, "xml="+xmlmsg.String())
  cmd.Env = append(env, os.Environ()...)
  util.Log(1, "INFO! Running trigger-action-hook %v with parameters %v", config.TriggerActionHookPath, env)
  out, err := cmd.CombinedOutput()
  if err != nil {
    util.Log(0, "ERROR! trigger-action-hook %v: %v (%v)", config.TriggerActionHookPath, err, string(out))
    return
  }
  util.Log(1, "INFO! Finished trigger-action-hook. Running time: %v", time.Since(start))
}
