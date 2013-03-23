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

// Unit tests run by run-tests.go.
package tests

import (
         "fmt"
         "time"
         "runtime"
         
         "../bytes"
       )

// Unit tests for the package go-susi/bytes.
func Bytes_test() {
  fmt.Printf("\n==== bytes ===\n\n")

  testBuffer()
  // The following is for testing if the free()s are called. To do this,
  // use the program ltrace(1).
  runtime.GC()
  time.Sleep(1*time.Second)
  runtime.GC()
}

func testBuffer() {
  var b bytes.Buffer
  check(b.String(),"") // String() on fresh variable
  b.Reset()            // Reset() on fresh variable
  check(b.String(),"") // String() after Reset()
  b.Reset()            // Reset() after Reset()
  check(b.String(),"")
  
  // same tests as above with pointer
  b2 := &bytes.Buffer{}
  check(b2.String(),"") 
  b2.Reset()            
  check(b2.String(),"") 
  b2.Reset()           
  check(b2.String(),"")
  
  b2.WriteString("Dies ist ein Test!")
  check(b2.String(), "Dies ist ein Test!")
  
  n, err := b.Write(nil)
  check(n,0)
  check(err,nil)
  check(b.String(),"")
  
  n, err = b.Write([]byte{})
  check(n,0)
  check(err,nil)
  check(b.String(),"")
  check(b.Pointer(),nil)
  check(b.Capacity(),0)
  
  func() {
    defer func(){
      check(recover(),bytes.ErrTooLarge)
    }()
    b.Grow(-1)
  }()
  
  n, err = b.Write([]byte{'a'})
  check(n,1)
  check(err,nil)
  check(b.String(),"a")
  check(b.Capacity()>=1, true)
  check(b.Pointer()!=nil, true)
  
  check(b.Grow(11), 1)
  check(b.Capacity()>=12, true)
  c := b.Capacity()
  p := b.Pointer()
  check(b.Grow(11), 1) // should not cause actual growth
  check(b.Pointer(), p)
  check(b.Capacity(), c)
  
  n, err = b.WriteString("Hallo")
  check(n,5)
  check(err,nil)
  check(b.String(),"aHallo")
  check(b.Pointer(), p)
  check(b.Capacity(), c)
  
  b.Reset()
  check(b.String(),"")
  check(b.Pointer(), nil)
  check(b.Capacity(),0)
  
  b.WriteString("Hallo")
  b.WriteByte(' ')
  b.Write([]byte{'d','i','e','s'})
  b.WriteByte(' ')
  b.WriteString("ist ")
  b.WriteString("ein ")
  b.Write([]byte("Test"))
  check(b.String(), "Hallo dies ist ein Test")
}

