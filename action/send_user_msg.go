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

package action

import (
        "time"
        "os"
        "os/exec"
        "strings"
        
        "../xml"
        "../util"
        "../config"
       )

func SendUserMsg(job *xml.Hash) {
  start := time.Now()
  util.Log(1, "INFO! Running user-msg-hook %v", config.UserMessageHookPath)
  env := []string{}
  for _, tag := range job.Subtags() {
    env = append(env, tag+"="+strings.Join(job.Get(tag),"\n"))
  }
  cmd := exec.Command(config.UserMessageHookPath)
  env = append(env, "xml="+job.String())
  cmd.Env = append(env, os.Environ()...)
  out, err := cmd.CombinedOutput()
  if err != nil {
    util.Log(0, "ERROR! user-msg-hook %v: %v (%v)", config.UserMessageHookPath, err, string(out))
    return
  }
  util.Log(1, "INFO! Finished user-msg-hook. Running time: %v", time.Since(start))
  return
}
