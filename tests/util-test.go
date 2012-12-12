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
         "io"
         "os"
         "fmt"
         "log"
         "net"
         "sort"
         "time"
         "bytes"
         "strings"
         "math/rand"
         "io/ioutil"
         
         "../util"
       )

var util_test_rng = rand.New(rand.NewSource(0x0dd))
var foobar = "foo"

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

type UintArray []uint64

func (a *UintArray) Less(i, j int) bool { return (*a)[i] < (*a)[j] }
func (a *UintArray) Len() int { return len(*a) }
func (a *UintArray) Swap(i, j int) { (*a)[i], (*a)[j] = (*a)[j], (*a)[i] }

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
  
  panicker := func() {
    foobar = "bar"
    panic("foo")
  }
  
  var buffy bytes.Buffer
  oldlogger := util.Logger
  util.Logger = log.New(&buffy, "", 0)
  defer func(){ util.Logger = oldlogger }()
  
  util.WithPanicHandler(panicker)
  check(foobar, "bar")
  check(len(buffy.String()) > 10, true)
  
  listener, err := net.Listen("tcp", "127.0.0.1:39390")
  if err != nil { panic(err) }  
  
  go func() {
    _, err := listener.Accept()
    if err != nil { panic(err) }
    time.Sleep(10*time.Second)
  }()
  long := make([]byte, 10000000)
  longstr := string(long)
  buffy.Reset()
  t0 := time.Now()
  util.SendLnTo("127.0.0.1:39390", longstr, 5 * time.Second)
  duration := time.Since(t0)
  check(duration > 4 * time.Second && duration < 6 * time.Second, true)
  check(strings.Contains(buffy.String(), "ERROR"), true)
  
  go func() {
    conn, err := listener.Accept()
    if err != nil { panic(err) }
    ioutil.ReadAll(conn)
  }()
  long = make([]byte, 10000000)
  longstr = string(long)
  buffy.Reset()
  t0 = time.Now()
  util.SendLnTo("127.0.0.1:39390", longstr, 5 * time.Second)
  duration = time.Since(t0)
  check(duration < 2 * time.Second, true)
  check(buffy.String(), "")
  
  go func() {
    _, err := net.Dial("tcp", "127.0.0.1:39390")
    if err != nil { panic(err) }
  }()
  conn, err := listener.Accept()
  if err != nil { panic(err) }
  buffy.Reset()
  t0 = time.Now()
  util.ReadLn(conn, 5 * time.Second)
  duration = time.Since(t0)
  check(duration > 4 * time.Second && duration < 6 * time.Second, true)
  check(strings.Contains(buffy.String(), "ERROR"), true)
  
  go func() {
    conn, err := net.Dial("tcp", "127.0.0.1:39390")
    if err != nil { panic(err) }
    conn.Write([]byte{1,2,3,4})
  }()
  conn, err = listener.Accept()
  if err != nil { panic(err) }
  buffy.Reset()
  t0 = time.Now()
  util.ReadLn(conn, 5 * time.Second)
  duration = time.Since(t0)
  check(duration > 4 * time.Second && duration < 6 * time.Second, true)
  check(strings.Contains(buffy.String(), "ERROR"), true)
  
  go func() {
    conn, err := net.Dial("tcp", "127.0.0.1:39390")
    if err != nil { panic(err) }
    conn.Write([]byte("foo\r\n"))
  }()
  conn, err = listener.Accept()
  if err != nil { panic(err) }
  buffy.Reset()
  t0 = time.Now()
  st := util.ReadLn(conn, 0 * time.Second)
  duration = time.Since(t0)
  check(duration < 2 * time.Second, true)
  check(buffy.String(), "")
  check(st, "foo")
  
  counter := util.Counter(13)
  var b1 UintArray = make([]uint64, 100)
  var b2 UintArray = make([]uint64, 100)
  done := make(chan bool)
  fill := func(b UintArray) {
    for i := 0; i < 100; i++ { 
      b[i] = <-counter 
      time.Sleep(1*time.Millisecond)
    }
    done <- true
  }
  go fill(b1)
  go fill(b2)
  <-done
  <-done
  check(sort.IsSorted(&b1), true)
  check(sort.IsSorted(&b2), true)
  var b3 UintArray = make([]uint64, 200)
  i := 0
  j := 0
  k := 0
  for ; i < 100 || j < 100 ; {
    if i == 100 { b3[k] = b2[j]; j++; k++; continue }
    if j == 100 { b3[k] = b1[i]; i++; k++; continue }
    if b1[i] == b2[j] { check(b1[i] != b2[j], true); break }
    if b1[i] < b2[j] { b3[k] = b1[i]; i++ } else { b3[k] = b2[j]; j++ }
    k++
  }
  
  one_streak := true
  b5 := make([]uint64, 200)
  for i:=0; i<200; i++ {
    if i < 100 && b1[i] != uint64(13+i) && b2[i] != uint64(13+i) { one_streak = false }
    b5[i] = uint64(13+i)
  }
  
  check(b3, b5)
  check(one_streak, false) // Check whether goroutines were actually executed concurrently rather than in sequence
  
  
  tempdir, err := ioutil.TempDir("", "util-test-")
  if err != nil { panic(err) }
  defer os.RemoveAll(tempdir)
  fpath := tempdir+"/foo.log"
  logfile := util.LogFile(fpath)
  check(logfile.Close(), nil)
  n, err = util.WriteAll(logfile, []byte("Test"))
  check(err,nil)
  check(n,4)
  check(logfile.Close(), nil)
  n, err = util.WriteAll(logfile, []byte("12"))
  check(err,nil)
  check(n,2)
  n, err = util.WriteAll(logfile, []byte("3"))
  check(err,nil)
  check(n,1)
  check(os.Rename(fpath,fpath+".old"), nil)
  n, err = util.WriteAll(logfile, []byte("Fo"))
  check(err,nil)
  check(n,2)
  f2,_ := os.OpenFile(fpath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
  f2.Write([]byte("o"))
  f2.Close()
  n, err = util.WriteAll(logfile, []byte("bar"))
  check(err,nil)
  check(n,3)
  check(logfile.Close(), nil)
  data, err := ioutil.ReadFile(fpath)
  check(err,nil) 
  if err == nil {
    check(string(data),"Foobar")
  }
  data, err = ioutil.ReadFile(fpath+".old")
  check(err, nil)
  if err == nil {
    check(string(data),"Test123")
  }
  
  test_time := time.Date(2013, time.January, 20, 14, 7, 21, 0, time.Local)
  check(util.MakeTimestamp(test_time),"20130120140721")
  check(util.MakeTimestamp(test_time.UTC()),"20130120140721")
  check(util.MakeTimestamp(test_time.In(time.FixedZone("Fooistan", 45678))),"20130120140721")
  illegal := time.Unix(0,0)
  buffy.Reset()
  check(util.ParseTimestamp(""), illegal)
  check(strings.Contains(buffy.String(), "ERROR"), true)
  buffy.Reset()
  check(util.ParseTimestamp("20139910101010"), illegal)
  check(strings.Contains(buffy.String(), "ERROR"), true)
  check(util.ParseTimestamp("20131110121314"), time.Date(2013, time.November, 10, 12, 13, 14, 0, time.Local))
  
  t0 = time.Now()
  util.WaitUntil(t0.Add(-10*time.Second))
  util.WaitUntil(t0.Add(-100*time.Minute))
  dur := time.Now().Sub(t0)
  if dur < 1*time.Second { dur = 0 }
  check(dur, 0)
  t0 = time.Now()
  util.WaitUntil(t0.Add(1200*time.Millisecond))
  dur = time.Now().Sub(t0)
  if dur >= 1200*time.Millisecond && dur <= 1300*time.Millisecond { dur = 1200*time.Millisecond }
  check(dur, 1200*time.Millisecond)
  
  mess := "WaitUntil(Jesus first birthday) takes forever"
  go func() {
    util.WaitUntil(time.Date(1, time.December, 25, 0,0,0,0, time.UTC))
    mess=""
  }()
  time.Sleep(100*time.Millisecond)
  check(mess,"")
  
  mess = "WaitUntil(1000-11-10 00:00:00) takes forever"
  go func() {
    util.WaitUntil(time.Date(1000, time.October, 11, 0,0,0,0, time.UTC))
    mess=""
  }()
  time.Sleep(100*time.Millisecond)
  check(mess,"")
}

