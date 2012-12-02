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

// Runs the go-susi unit tests.

package main

import ( 
         "os"
         "fmt"
         "strings"
         
         "../tests"
       )

var gosasi = false

func main() {
  systemtest := ""
  force_unittest := false
  for i:=1 ; i < len(os.Args) ; i++ {
    if os.Args[i] == "-v" { tests.Show_output=true }
    if strings.HasPrefix(os.Args[i],"--system=") { systemtest = os.Args[i][9:] }
    if os.Args[i] == "--gosa-si" { gosasi = true }
    if os.Args[i] == "--unit" { force_unittest = true }
  }

  if force_unittest || systemtest == "" {
    tests.UnitTests()
  }
  
  if systemtest != "" {
    tests.SystemTest(systemtest, gosasi)
  }
  
  // -----------------------------------------
  fmt.Printf("\n=== Results ===\n\n#Tests: %3v\nPassed: %3v (%v unexpected)\nFailed: %3v (%v expected)\n", 
  tests.Count, tests.Pass, tests.UnexpectedPass, tests.Fail, tests.ExpectedFail)
  
  fmt.Printf("\nPass '-v' on the command line to see test output\nPass --system=<host>:<port> to test a running daemon\nPass --system=<programpath> to start daemon <programpath> and test it\nPass --gosa-si if the server to test is a gosa-si\nPass --unit in addition to --system to perform all tests in one go.\n\n")
  
  if tests.Fail > tests.ExpectedFail { 
    os.Exit(1) 
  } else {
    os.Exit(0)
  }
}
