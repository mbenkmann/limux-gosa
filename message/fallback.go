/* 
Copyright (c) 2012 Landeshauptstadt München
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
         "sync"
         "net"
         
         "../util"
       )

// The gosa-si server to ask as fallback if we can't deal with a message
var fallback_addr = "warmachine:20081"

var cache = map[string]string{}

var lock sync.Mutex

func checkCache(msg string) string {
  lock.Lock()
  defer lock.Unlock()
  reply, ok := cache[msg]
  if ok { return reply }
  return ""
}

func addToCache(msg, reply string) {
  util.Log(2, "DEBUG! Adding to cache: %v", msg)
  lock.Lock()
  defer lock.Unlock()
  cache[msg] = reply
}

// Passes the encrypted message to the gosa-si fallback server 
// and returns its reply.
func fallback(encrypted string) (reply string) {
  /*reply = checkCache(encrypted)
  if reply != "" {
    util.Log(2, "DEBUG! Returning answer from cache")
    return reply
  }*/
  
  util.Log(2, "DEBUG! Connecting to fallback server %v", fallback_addr)
  conn, err := net.Dial("tcp4", fallback_addr)
  if err != nil {
    util.Log(0, "ERROR! fallback: %v", err)
    return ""
  }
  defer conn.Close()
  defer util.Log(2, "DEBUG! Connection to fallback server %v closed", conn.RemoteAddr())
  
  util.Log(2, "DEBUG! Sending message to fallback server")
  util.SendLn(conn, encrypted)
  util.Log(2, "DEBUG! Reading reply from fallback server")
  reply = util.ReadLn(conn)
  util.Log(2, "DEBUG! Reply from fallback server: %v", reply)
  
  //addToCache(encrypted, reply)
  
  return reply
}
