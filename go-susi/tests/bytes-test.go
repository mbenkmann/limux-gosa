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
  check(b.Len(), 0)
  
  // same tests as above with pointer
  b2 := &bytes.Buffer{}
  check(b2.String(),"") 
  b2.Reset()            
  check(b2.String(),"") 
  b2.Reset()           
  check(b2.String(),"")
  check(b2.Len(), 0)
  
  b2.WriteString("Dies ist ein Test!")
  check(b2.String(), "Dies ist ein Test!")
  check(b2.Len(), 18)
  
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
  check(b.Len(),0)
  
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
  check(b.Len(), 1)
  check(b.Pointer()!=nil, true)
  
  check(b.Grow(11), 1)
  check(b.Capacity()>=12, true)
  c := b.Capacity()
  p := b.Pointer()
  check(b.Grow(11), 1) // should not cause actual growth
  check(b.Pointer(), p)
  check(b.Capacity(), c)
  check(b.Len(), 1)
  ((*[2]byte)(b.Pointer()))[1] = 'z'
  check(b.Contains("z"),false)
  
  n, err = b.WriteString("Hallo")
  check(n,5)
  check(err,nil)
  check(b.String(),"aHallo")
  check(b.Pointer(), p)
  check(b.Capacity(), c)
  check(b.Len(), 6)
  
  b.Reset()
  check(b.String(),"")
  check(b.Pointer(), nil)
  check(b.Capacity(),0)
  check(b.Contains(""), true)
  check(b.Contains("a"), false)
  
  b.WriteString("Hallo")
  b.WriteByte(' ')
  b.Write([]byte{'d','i','e','s'})
  b.WriteByte(' ')
  b.WriteString("ist ")
  b.WriteString("ein ")
  b.Write([]byte("Test"))
  check(b.String(), "Hallo dies ist ein Test")
  check(b.Contains("Hallo dies ist ein Test"), true)
  check(b.Contains("Test"), true)
  check(b.Contains("Hallo"), true)
  check(b.Contains("allo"), true)
  check(b.Contains(""), true)
  
  check(b.Split(" "), []string{"Hallo","dies","ist","ein","Test"})
  check(b.Split("X"), []string{"Hallo dies ist ein Test"})
  check(b.Split("Hallo dies ist ein Test"), []string{"",""})
  check(b.Split("H"), []string{"","allo dies ist ein Test"})
  check(b.Split("Test"), []string{"Hallo dies ist ein ",""})
  check(b.Split("es"), []string{"Hallo di"," ist ein T","t"})
  
  b.Reset()
  b.WriteString("  \n\t Hallo  \t\v\n")
  check(b.Len(), 15)
  p = b.Pointer()
  b.TrimSpace()
  check(b.String(), "Hallo")
  check(b.Len(), 5)
  check(b.Pointer(), p)
  
  b.Reset()
  b.WriteString("  \n\t   \t\v\n")
  check(b.Len(), 10)
  b.TrimSpace()
  check(b.Pointer(),nil)
  check(b.Len(),0)
  check(b.Capacity(),0)
  b.TrimSpace()
  check(b.Pointer(),nil)
  check(b.Len(),0)
  check(b.Capacity(),0)
  
  b.Reset()
  b.WriteString("  \n\t Hallo")
  check(b.Len(), 10)
  p = b.Pointer()
  b.TrimSpace()
  check(b.String(), "Hallo")
  check(b.Len(), 5)
  check(b.Pointer(), p)
  
  b.Reset()
  b.WriteString("Hallo  \t\v\n")
  check(b.Len(), 10)
  p = b.Pointer()
  b.TrimSpace()
  check(b.String(), "Hallo")
  check(b.Len(), 5)
  check(b.Pointer(), p)
  
  b.Reset()
  b.WriteString(" ")
  check(b.Len(), 1)
  b.TrimSpace()
  check(b.Pointer(),nil)
  check(b.Len(),0)
  check(b.Capacity(),0)
  
  b.Reset()
  b.WriteString("Der Cottbuser Postkutscher kotzt in den Cottbuser Postkotzkasten")
  n = b.Len()
  c = b.Capacity()
  p = b.Pointer()
  b.Trim(-10, 2000)
  check(b.Len(), n)
  check(b.Capacity(), c)
  check(b.Pointer(), p)
  
  b.Trim(2000, -10)
  check(b.Len(), 0)
  check(b.Capacity(), 0)
  check(b.Pointer(), nil)
  
  b.WriteString("Der Cottbuser Postkutscher kotzt in den Cottbuser Postkotzkasten")
  b.Trim(4,4)
  check(b.Len(), 0)
  check(b.Capacity(), 0)
  check(b.Pointer(), nil)
  
  b.WriteString("Der Cottbuser Postkutscher kotzt in den Cottbuser Postkotzkasten")
  n = b.Len()
  c = b.Capacity()
  p = b.Pointer()
  b.Trim(0,b.Len()-6)
  check(b.Len(), n-6)
  check(b.Capacity(), c)
  check(b.Pointer(), p)
  check(b.String(), "Der Cottbuser Postkutscher kotzt in den Cottbuser Postkotz")
  
  b.Trim(27,b.Len())
  check(b.Len(), n-6-27)
  check(b.Capacity(), c)
  check(b.Pointer(), p)
  check(b.String(), "kotzt in den Cottbuser Postkotz")
  
  b.Trim(1,b.Len()-1)
  check(b.Len(), n-6-27-2)
  check(b.Capacity(), c)
  check(b.Pointer(), p)
  check(b.String(), "otzt in den Cottbuser Postkot")
  
  b.Reset()
  b.Write0(-1)
  b.Write0(-100)
  b.Write0(0)
  check(b.Len(), 0)
  check(b.Capacity(), 0)
  check(b.Pointer(), nil)
  
  b.Write0(1)
  check(b.Len(), 1)
  check(b.Capacity(), 1)
  check(b.Bytes(), []byte{0})
  
  b.WriteByte(111)
  b.Write0(1)
  b.WriteByte(222)
  b.Write0(2)
  b.WriteByte(99)
  check(b.Len(), 7)
  check(b.Bytes(), []byte{0,111,0,222,0,0,99})
}

