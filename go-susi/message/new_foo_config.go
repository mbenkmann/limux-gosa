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
         "os"
         "os/exec"
         "strings"
         
         "../xml"
         "../util"
         "../config"
       )

// Handles all messages of the form "new_*_config" by calling config.NewConfigHookPath.
//  xmlmsg: the decrypted and parsed message
func new_foo_config(xmlmsg *xml.Hash) {
  target := xmlmsg.Text("target")
  if target != "" && target != config.ServerSourceAddress {
    // See https://code.google.com/p/go-susi/issues/detail?id=126
    util.Log(0, "WARNING! Ignoring message with incorrect target: %v", xmlmsg)
    return
  }
  
  header := xmlmsg.Text("header")
  env := config.HookEnvironment()
  for _, tag := range xmlmsg.Subtags() {
    if tag == header { continue }
    env = append(env, tag+"="+strings.Join(xmlmsg.Get(tag),"\n"))
  }
  env = append(env, header+"=1")
    
  cmd := exec.Command(config.NewConfigHookPath)
  cmd.Env = append(env, os.Environ()...)
  util.Log(1, "INFO! Running %v with parameters %v", config.NewConfigHookPath, env)
  out, err := cmd.CombinedOutput()
  if err != nil {
    util.Log(0, "ERROR! Error executing %v: %v (%v)", config.NewConfigHookPath, err, string(out))
  }
}
