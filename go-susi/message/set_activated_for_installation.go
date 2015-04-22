/*
Copyright (c) 2013 Landeshauptstadt München
Author Matthias S. Benkmann

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
         "strings"
         "os"
         "os/exec"
         
         "../xml"
         "../util"
         "../config"
       )

// Handles "set_activated_for_installation".
//  xmlmsg: the decrypted and parsed message
func set_activated_for_installation(xmlmsg *xml.Hash) {
  start := time.Now()
  env := config.HookEnvironment()
  for _, tag := range xmlmsg.Subtags() {
    env = append(env, tag+"="+strings.Join(xmlmsg.Get(tag),"\n"))
  }
  cmd := exec.Command(config.ActivatedHookPath)
  env = append(env, "xml="+xmlmsg.String())
  env = append(env, "gotomode="+xmlmsg.Text("gotomode"))
  env = append(env, "faistate="+xmlmsg.Text("faistate"))
  cmd.Env = append(env, os.Environ()...)
  util.Log(1, "INFO! Running activated-hook %v with parameters %v", config.ActivatedHookPath, env)
  out, err := cmd.CombinedOutput()
  if err != nil {
    util.Log(0, "ERROR! activated-hook %v: %v (%v)", config.ActivatedHookPath, err, string(out))
    return
  }
  util.Log(1, "INFO! Finished activated-hook. Running time: %v", time.Since(start))
}


// Sends "set_activated_for_installation" to client_addr and calls
// Send_new_ldap_config(client_addr, system)
// old_gotomode is the gotoMode attribute before activation.
func Send_set_activated_for_installation(client_addr string, old_gotomode string, system *xml.Hash) {
  // gosa-si-server sends LDAP config both before and after set_activated_for_installation
  // Personally I think that sending it BEFORE should be enough. But it doesn't hurt
  // to do it twice. Better safe than sorry.
  Send_new_ldap_config(client_addr, system)
  gotomode := ""
  if old_gotomode != "active" { gotomode = "<gotomode>active</gotomode>" }
  faistate := "<faistate>" + system.Text("faistate") + "</faistate>"
  set_activated_for_installation := "<xml><header>set_activated_for_installation</header><set_activated_for_installation></set_activated_for_installation><source>"+ config.ServerSourceAddress +"</source><target>"+ client_addr +"</target>" + gotomode + faistate + "</xml>"
  Client(client_addr).Tell(set_activated_for_installation, config.NormalClientMessageTTL)
  Send_new_ldap_config(client_addr, system)
}
