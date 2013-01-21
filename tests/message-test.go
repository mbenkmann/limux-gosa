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

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, 
MA  02110-1301, USA.
*/

// Unit tests run by run-tests.go.
package tests

import (
         "fmt"
         
         "../db"
         "../config"
         "../message"
       )

// Unit tests for the package go-susi/message.
func Message_test() {
  fmt.Printf("\n==== message ===\n\n")
  
  getFJU() // clean up remnants from DB_test()

  go func(){
    defer func(){recover()}()
    message.DistributeForeignJobUpdates()
  }()
  // This is a nasty trick. Send nil over the channel to cause a
  // a panic in the message.DistributeForeignJobUpdates()
  // goroutine so that it will terminate.
  // ATM it's not actually necessary to make it terminate but it's
  // a precaution against future test code that may not expect someone
  // to be reading from this channel.
  defer func(){db.ForeignJobUpdates <- nil}()
  
  listen()
  defer func() {listener.Close()}()
  listen_address = config.IP + ":" + listen_port
  
  message.Peer(listen_address).SetGoSusi(true)
  check(message.Peer(listen_address).IsGoSusi(), true)
  message.Peer(listen_address).SetGoSusi(false)
  check(message.Peer(listen_address).IsGoSusi(), false)
  message.Peer(listen_address).SetGoSusi(true)
  check(message.Peer(listen_address).IsGoSusi(), true)
}

