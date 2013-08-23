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
         "net"
         "time"
        
         "../db"
         "../xml"
         "../util"
         "../config"
       )

// Handles the message "gosa_ping".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply 
func gosa_ping(xmlmsg *xml.Hash) string {
  macaddress := xmlmsg.Text("target")
  
  portscan := false
  target := ""
  if system := db.ClientWithMAC(macaddress); system != nil {
    target = system.Text("client")
  } else
  if system := db.ServerWithMAC(macaddress); system != nil {
    target =system.Text("source")
  } else
  if system := db.SystemFullyQualifiedNameForMAC(macaddress); system != "none" {
    addrs, err := net.LookupIP(system)
    if err == nil && len(addrs) > 0 {
      target = addrs[0].String()
      portscan = true
    }
  }
  
  if target == "" {
    util.Log(0, "ERROR! gosa_ping can't determine IP for MAC \"%v\"", macaddress)
    return ""
  }
  
  reachable := make(chan bool, 2)
  
  if portscan {
    // Scan ports sequentially in one goroutine.
    // This gives a fast reachable<-false if the network reports that all
    // ports are unreachable. It may be slow, however, if the network simply
    // DROPs request packages for closed ports.
    // For this reason, see further below...
    go func() {
      for _, port := range config.ClientPorts {
        conn, err := net.Dial("tcp", target+":"+port)
        if err == nil {
          conn.Close()
          reachable <- true
          return
        }
      }
      reachable <- false
    }()
    
    // ... Scan the same ports in parallel. This gives a fast reachable<-true if
    // at least one port is reachable.
    for _, port := range config.ClientPorts {
      go func(p string) {
        conn, err := net.Dial("tcp", target+":"+p)
        if err == nil {
          conn.Close()
          reachable <- true
          return
        }
      }(port)
    }
  } else { // no portscan necessary. Use target directly.
    go func() {
      conn, err := net.Dial("tcp", target)
      if err != nil {
        reachable <- false
      } else {
        conn.Close()
        reachable <- true
      }
    }()
  }
    
  // Make sure we don't wait too long for a result.
  go func() {
    time.Sleep(100*time.Millisecond)
    reachable <- false
  }()
    
  if <-reachable { 
    util.Log(1, "INFO! gosa_ping says client %v/%v is ON", macaddress, target) 
    return "<xml><header>got_new_ping</header><got_new_ping></got_new_ping></xml>"
  }
  
  util.Log(1, "INFO! gosa_ping says client %v/%v is OFF", macaddress, target) 
  return ""
}
