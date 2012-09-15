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
         "../db"
         "../xml"
         "../util"
         "../config"
       )

// Sends a new_server message to all known peer servers.
func Broadcast_new_server() {
  msg := xml.NewHash("xml","header","new_server")
  msg.Add("source", config.ServerSourceAddress)
  msg.Add("macaddress", config.MAC)
  msg.Add("loaded_modules", "gosaTriggered", "siTriggered", 
                            "clMessages", "server_server_com", 
                            "databases", "logHandling", 
                            "goSusi")
  msg.Add("key")     // filled in later
  msg.Add("target")  // filled in later
  
  serverpackageskey := config.ModuleKey["[ServerPackages]"]

  for server := db.Servers().First("xml"); server != nil; server = server.Next() {
    target := server.Text("source")
    msg.First("target").SetText(target)
    msg.First("key").SetText(server.First("key").Text())
    encrypted := GosaEncrypt(msg.String(), serverpackageskey)
    util.Log(2, "DEBUG! Sending new_server to %v encrypted with key %v: %v", target, serverpackageskey, encrypted)
    go util.SendLnTo(target, encrypted)
  }
}

func new_server(encrypted string, xmlmsg *xml.Hash) string {
// TODO: Als Reaktion auf new_server die ganze jobdb als foreign_job_updates
// übermitteln, damit der neue Server auf dem aktuellen Stand ist.
  return ""
}
