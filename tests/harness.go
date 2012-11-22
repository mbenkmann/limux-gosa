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

// Unit tests run by run_tests.go.
package tests

import (
         "fmt"
         "strings"
         "reflect"
         "runtime"
       )

// true => show test output even for PASSED tests.
var Show_output = false

// counts the number of tests run.
var Count = 0

// How many tests passed.
var Pass  = 0

// How many tests failed.
var Fail  = 0

// How many tests that were expected to fail did so.
var ExpectedFail = 0

// How many tests that were expected to fail did instead pass.
var UnexpectedPass = 0

// It's ridiculously hard to check if a value is nil in Go.
// At the time of this writing the following prints "false":
//
//   var x *int
//   var y interface{} = x
//   fmt.Println(y == nil)
//
// Apparently a nil pointer wrapped in an interface{} is not equal to nil.
// And that despite the fact that fmt.Println(y) prints "<nil>".
func isNil(x interface{}) (ret bool) {
  if x == nil { return true }
  defer func() {
    if recover() != nil { ret = false } 
  }()
  return reflect.ValueOf(x).IsNil()
}

// Compares x with expected and prints PASSED if equal and FAILED if not.
// Returns true if PASSED.
func check(x interface{}, expected interface{}) bool {
  return checkLevel(x,expected,2)
}

func checkLevel(x interface{}, expected interface{}, level int) bool {
  Count++
  _, file, line, _ := runtime.Caller(level)
  file = file[strings.LastIndex(file, "/")+1:]
  fmt.Printf("Test %2v (%v:%v) ", Count, file, line)
  
  // The isNil() test is here because otherwise in the case x is a nil pointer,
  // the evaluation of Sprintf() will run into a SIGSEGV. 
  // Sprintf() catches this error and converts it to the string "<nil>" so that it is
  // no problem when running the tests standalone. However when running the tests
  // under gdb, it's annoying to have gdb stop at the SIGSEGV.
  if (isNil(expected) && isNil(x)) || fmt.Sprintf("%v", expected) == fmt.Sprintf("%v", x) {
    fmt.Println("PASSED")
    Pass++
    if Show_output {
      fmt.Printf("OUTPUT  : %v\n", x)
    }
    return true
  } else {
    fmt.Println("FAILED")
    Fail++
    fmt.Printf("OUTPUT  : %v\nEXPECTED: %v\n", x, expected)
    return false
  }
  return true
}

// Like check() but failure is expected and the counts will adjusted accordingly.
// Returns true if PASSED.
func checkFail(x interface{}, expected interface{}) bool {
  return checkFailLevel(x,expected,2)
}

func checkFailLevel(x interface{}, expected interface{}, level int) bool {
  Count++
  _, file, line, _ := runtime.Caller(level)
  file = file[strings.LastIndex(file, "/")+1:]
  fmt.Printf("Test %2v (%v:%v) ", Count, file, line)
  
  // The isNil() test is here because otherwise in the case x is a nil pointer,
  // the evaluation of Sprintf() will run into a SIGSEGV. 
  // Sprintf() catches this error and converts it to the string "<nil>" so that it is
  // no problem when running the tests standalone. However when running the tests
  // under gdb, it's annoying to have gdb stop at the SIGSEGV.
  if (isNil(expected) && isNil(x)) || fmt.Sprintf("%v", expected) == fmt.Sprintf("%v", x) {
    fmt.Println("PASSED UNEXPECTEDLY")
    Pass++
    UnexpectedPass++
    fmt.Printf("OUTPUT  : %v\n", x)
    return true
  } else {
    fmt.Println("FAILED AS EXPECTED")
    Fail++
    ExpectedFail++
    if Show_output {
      fmt.Printf("OUTPUT  : %v\nSHOULD BE: %v\n", x, expected)
    }
    return false
  }
  return true
}
