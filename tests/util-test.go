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
         "io"
         "fmt"
         "math/rand"
         
         "../util"
       )

var util_test_rng = rand.New(rand.NewSource(0x0dd))

// tick is incremented on every Write(). Every crappyConnection will only
// write more than 0 bytes if tick % 4 == 0.
var util_test_tick = 0

// A Writer that only writes a random number of bytes up to 16 (possibly 0) 
// and then returns the number written and io.ErrShortWrite.
type crappyConnection1 []byte

func (self *crappyConnection1) Write(data []byte) (n int, err error) {
  util_test_tick++
  if (util_test_tick % 4 == 0) {
    n = len(data)
    if n > 16 { n = 16 }
    n = util_test_rng.Intn(n) + 1
    *self = append(*self, data[0:n]...)
    if n < len(data) {
      err = io.ErrShortWrite
    }
    return
  }
  
  return 0, io.ErrShortWrite
}

// A Writer that only writes a random number of bytes up to 16 (possibly 0)
// and then returns the number written without error.
type crappyConnection2 []byte

func (self *crappyConnection2) Write(data []byte) (n int, err error) {
  err = nil
  util_test_tick++
  if (util_test_tick % 4 == 0) {
    n = len(data)
    if n > 16 { n = 16 }
    n = util_test_rng.Intn(n) + 1
    *self = append(*self, data[0:n]...)
    return
  }
  
  return 0, nil
}


// A Writer that always returns (0, io.ErrShortWrite) after writing the first
// 16 bytes.
type stalledConnection1 []byte

func (self *stalledConnection1) Write(data []byte) (n int, err error) {
  if len(*self) == 16 { return 0, io.ErrShortWrite }
  var s *crappyConnection1
  s = (*crappyConnection1)(self)
  n, err = s.Write(data)
  if len(*self) > 16 {
    n -= len(*self) - 16
    *self = (*self)[0:16]
  }
  return
}


// A Writer that always returns (0, nil) after writing the first 16 bytes.
type stalledConnection2 []byte

func (self *stalledConnection2) Write(data []byte) (n int, err error) {
  err = nil
  if len(*self) == 16 { return 0, nil }
  var s *crappyConnection1
  s = (*crappyConnection1)(self)
  n, _ = s.Write(data)
  if len(*self) > 16 {
    n -= len(*self) - 16
    *self = (*self)[0:16]
  }
  return
}

// A Writer that always returns (0, io.ErrClosedPipe) after writing the first 16 bytes.
type brokenConnection []byte

func (self *brokenConnection) Write(data []byte) (n int, err error) {
  if len(*self) == 16 { return 0, io.ErrClosedPipe }
  var s *crappyConnection1
  s = (*crappyConnection1)(self)
  n, _ = s.Write(data)
  if len(*self) > 16 {
    n -= len(*self) - 16
    *self = (*self)[0:16]
  }
  if len(*self) == 16 {
    return n, io.ErrClosedPipe
  }
  
  return n, io.ErrShortWrite
}



// Unit tests for the package go-susi/util.
func Util_test() {
  fmt.Printf("\n==== util ===\n\n")

  buf := make([]byte, 80)
  for i := range buf {
    buf[i] = byte(util_test_rng.Intn(26) + 'a')
  }
  
  crap1 := &crappyConnection1{}
  n, err := util.WriteAll(crap1, buf)
  check(string(*crap1), string(buf))
  check(n, len(buf))
  check(err, nil)
  
  crap2 := &crappyConnection2{}
  n, err = util.WriteAll(crap2, buf)
  check(string(*crap2), string(buf))
  check(n, len(buf))
  check(err, nil)
  
  stalled1 := &stalledConnection1{}
  n, err =util.WriteAll(stalled1, buf)
  check(string(*stalled1), string(buf[0:16]))
  check(n, 16)
  check(err, io.ErrShortWrite)
  
  stalled2 := &stalledConnection2{}
  n, err = util.WriteAll(stalled2, buf)
  check(string(*stalled2), string(buf[0:16]))
  check(n, 16)
  check(err, io.ErrShortWrite)
  
  broken := &brokenConnection{}
  n, err = util.WriteAll(broken, buf)
  check(string(*broken), string(buf[0:16]))
  check(n, 16)
  check(err, io.ErrClosedPipe)
}

