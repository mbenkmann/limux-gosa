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

// Unit tests run by run-tests.go.
package tests

import (
         "fmt"
         
         "../db"
       )

// Unit tests for the package go-susi/db.
func DB_test() {
  fmt.Printf("\n==== db ===\n\n")

  check(db.JobGUID("0.0.0.0:0", 0), "00")
  check(db.JobGUID("255.255.255.255:65535", 18446744073709551615), "18446744073709551615281474976710655")
  check(db.JobGUID("1.2.3.4:20081", 18446744073709551615), "1844674407370955161586247305576961")
}

