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
         "math/rand"
         
         "../xml"
         "../util"
         "../util/deque"
       )


// 0 => not registered, 1 => registered, but in the process of verifying
// 2 => registered
var registrationState = 0

var currentServer = ""

var registrationQueue deque.Deque

var serverList []string

var indexInList int

var secs_between_candidates = 10

var timeout_for_confirmation = 60*time.Second

// Infinite loop that handles registering and staying registered at
// a server.
func RegistrationHandler() {
  registrationQueue.Push("register")
  
  for {
    r := registrationQueue.Next()
    switch r {
      case "register" :
        if registrationState == 0 {
          serverList = serversToTry()
          currentServer = "" // AFTER serversToTry() because it uses currentServer
          util.Log(1, "INFO! New server registration started. Candidates: %v", serverList)
          indexInList = -1
          registrationQueue.Push("timeout")
        }
      case "confirm_timeout":
        if registrationState == 1 {
          util.Log(0, "WARNING! Could not confirm that I'm still registered at %v => Start new registration", currentServer)
          registrationState = 0
          registrationQueue.Push("register")
        }
      case "timeout":
        if registrationState == 0 {
          for indexInList++; indexInList < len(serverList) && serverList[indexInList] == ""; indexInList++ {}
          if indexInList < len(serverList) {
            if indexInList > 0 {
              util.Log(0, "WARNING! Registration at %v failed => Will try next candidate server", currentServer)
            }
            currentServer = serverList[indexInList]
            util.Log(1, "INFO! Trying to register at %v", currentServer)
            Send_here_i_am(currentServer)
            go func() {
              time.Sleep((secs_between_candidates+rand.IntN(10))*time.Second)
              registrationQueue.Push("timeout")
            }
          } else {
            util.Log(0, "WARNING! Registration failed. No more servers left to try. Will wait 1 minute then try again.")
            // wait with random element to disband any client swarms
            time.Sleep((55+rand.IntN(20))*time.Second)
            registrationQueue.Clear()
            registrationQueue.Insert("register")
          }
        }
      case currentServer:
        util.Log(1, "INFO! Successfully registered at %v", currentServer)
        registrationState = 2
       
      case "confirm":
        if registrationState == 2 {
          registrationState = 1
          Send_here_i_am(currentServer)
          go func() {
            time.Sleep(timeout_for_confirmation)
            registrationQueue.Push("confirm_timeout")
          }
        } 
      
      default:
        util.Log(0, "WARNING! Received \"registered\" from unexpected server %v => Confirming if I'm still registered at %v", r, currentServer)
        registrationQueue.Push("confirm")
    }
  }
}

// Causes a re-registration at the current server to make sure we're still
// registered there. If the server doesn't reply, registration starts from
// scratch. 
func ConfirmRegistration() {
  registrationQueue.Push("confirm")
}

// ATTENTION! The returned list may contain empty strings.
func serversToTry() []string {
  // If we're running a server, never register anywhere else.
  if config.RunServer { return []string{config.ServerSourceAddress} }
  
  servers = []string{currentServer, config.PreferredServer }
  servers = append(servers, config.PeerServers...)
  servers = append(servers, config.ServersFromDNS()...)
}
