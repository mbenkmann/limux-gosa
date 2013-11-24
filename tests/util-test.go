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
         "io"
         "os"
         "os/exec"
         "fmt"
         "net"
         "sort"
         "time"
         "strings"
         "math/rand"
         "io/ioutil"
         "encoding/base64"
         
         "../util"
         "../bytes"
         "../config"
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
  
  addr, err := util.Resolve("1.2.3.4", "")
  check(err,nil)
  check(addr,"1.2.3.4")
  
  addr, err = util.Resolve("1.2.3.4:5", "")
  check(err,nil)
  check(addr,"1.2.3.4:5")
  
  addr, err = util.Resolve("::1:5", "")
  check(err, nil)
  check(addr,"[::1:5]")
  
  addr, err = util.Resolve("localhost:65535", "")
  check(err, nil)
  check(addr, "127.0.0.1:65535")
  
  addr, err = util.Resolve("localhost", "")
  check(err, nil)
  check(addr, "127.0.0.1")
  
  addr, err = util.Resolve("::1","")
  check(err, nil)
  check(addr, "127.0.0.1")
  
  addr, err = util.Resolve("[::1]","")
  check(err, nil)
  check(addr, "127.0.0.1")
  
  addr, err = util.Resolve("[::1]:12345","")
  check(err, nil)
  check(addr, "127.0.0.1:12345")
  
  addr, err = util.Resolve("localhost:65535", "foo")
  check(err, nil)
  check(addr, "foo:65535")
  
  addr, err = util.Resolve("localhost", "foo")
  check(err, nil)
  check(addr, "foo")
  
  addr, err = util.Resolve("::1","foo")
  check(err, nil)
  check(addr, "foo")
  
  addr, err = util.Resolve("[::1]","foo")
  check(err, nil)
  check(addr, "foo")
  
  addr, err = util.Resolve("[::1]:12345","foo")
  check(err, nil)
  check(addr, "foo:12345")
  
  addr, err = util.Resolve("","")
  check(hasWords(err,"no","such","host"), "")
  check(addr, "")
  
  addr, err = util.Resolve(":10","")
  check(hasWords(err,"no","such","host"), "")
  check(addr, ":10")
  
  h,_ := exec.Command("hostname").CombinedOutput()
  hostname := strings.TrimSpace(string(h))
  
  ipp,_ := exec.Command("hostname","-I").CombinedOutput()
  ips := strings.Fields(strings.TrimSpace(string(ipp)))
  addr, err = util.Resolve(hostname+":234", config.IP)
  check(err, nil)
  ip := ""
  for _, ip2 := range ips {
    if addr == ip2+":234" { ip = ip2 }
  }
  check(addr, ip+":234")
  
  testLogging()

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
  util.LoggersSuspend()
  util.LoggerAdd(&buffy)
  defer util.LoggersRestore()
  
  util.WithPanicHandler(panicker)
  time.Sleep(200*time.Millisecond) // make sure log message is written out
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
  time.Sleep(200*time.Millisecond) // make sure log message is written out
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
  time.Sleep(200*time.Millisecond) // make sure log message is written out
  check(buffy.String(), "")
  
  // Test that ReadLn() times out properly
  go func() {
    _, err := net.Dial("tcp", "127.0.0.1:39390")
    if err != nil { panic(err) }
  }()
  conn, err := listener.Accept()
  if err != nil { panic(err) }
  t0 = time.Now()
  st, err := util.ReadLn(conn, 5 * time.Second)
  duration = time.Since(t0)
  check(duration > 4 * time.Second && duration < 6 * time.Second, true)
  check(st,"")
  check(hasWords(err,"timeout"), "")
  
  // Test that ReadLn() returns io.EOF if last line not terminated by \n
  go func() {
    conn, err := net.Dial("tcp", "127.0.0.1:39390")
    if err != nil { panic(err) }
    conn.Write([]byte("foo\r"))
    conn.Close()
  }()
  conn, err = listener.Accept()
  if err != nil { panic(err) }
  st, err = util.ReadLn(conn, 5 * time.Second)
  check(err, io.EOF)
  check(st,"foo")
  
  go func() {
    conn, err := net.Dial("tcp", "127.0.0.1:39390")
    if err != nil { panic(err) }
    conn.Write([]byte("\r\r\n\rfo\ro\nbar\r\nfoxtrott"))
    conn.Close()
  }()
  conn, err = listener.Accept()
  if err != nil { panic(err) }
  // Test proper trimming of multiple \r
  st, err = util.ReadLn(conn,0)
  check(err,nil)
  check(st,"")
  // Test that the empty first line has actually been read 
  // and that the next ReadLn() reads the 2nd line
  // Also test that negative timeouts work the same as timeout==0
  // Also test that \r is not trimmed at start and within line.
  st, err = util.ReadLn(conn, -1*time.Second)
  check(err,nil)
  check(st,"\rfo\ro") 
  // Check 3rd line
  st, err = util.ReadLn(conn,0)
  check(err,nil)
  check(st,"bar") 
  // Check 4th line and io.EOF error
  st, err = util.ReadLn(conn,0)
  check(err,io.EOF)
  check(st,"foxtrott") 
  
  
  // Test that delayed reads work with timeout==0
  go func() {
    conn, err := net.Dial("tcp", "127.0.0.1:39390")
    if err != nil { panic(err) }
    time.Sleep(1*time.Second)
    _, err = conn.Write([]byte("foo\r\n"))
    if err != nil { panic(err) }
    time.Sleep(2*time.Second)
  }()
  conn, err = listener.Accept()
  if err != nil { panic(err) }
  t0 = time.Now()
  st,err = util.ReadLn(conn, time.Duration(0))
  duration = time.Since(t0)
  check(duration < 2 * time.Second, true)
  check(duration > 800 * time.Millisecond, true)
  check(err, nil)
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
  test_time  = time.Date(2013, time.January, 20, 14, 7, 21, 0, time.UTC)
  check(util.MakeTimestamp(test_time),"20130120140721")
  test_time  = time.Date(2013, time.January, 20, 14, 7, 21, 0, time.FixedZone("Fooistan", 45678))  
  check(util.MakeTimestamp(test_time),"20130120140721")
  illegal := time.Unix(0,0)
  buffy.Reset()
  check(util.ParseTimestamp(""), illegal)
  time.Sleep(200*time.Millisecond) // make sure log message is written out
  check(strings.Contains(buffy.String(), "ERROR"), true)
  buffy.Reset()
  check(util.ParseTimestamp("20139910101010"), illegal)
  time.Sleep(200*time.Millisecond) // make sure log message is written out
  check(strings.Contains(buffy.String(), "ERROR"), true)
  check(util.ParseTimestamp("20131110121314"), time.Date(2013, time.November, 10, 12, 13, 14, 0, time.Local))
  check(util.MakeTimestamp(util.ParseTimestamp(util.MakeTimestamp(test_time))), util.MakeTimestamp(test_time))
  test_time = test_time.Add(2400*time.Hour)
  check(util.MakeTimestamp(util.ParseTimestamp(util.MakeTimestamp(test_time))), util.MakeTimestamp(test_time))
  test_time = test_time.Add(2400*time.Hour)
  check(util.MakeTimestamp(util.ParseTimestamp(util.MakeTimestamp(test_time))), util.MakeTimestamp(test_time))
  test_time = test_time.Add(2400*time.Hour)
  check(util.MakeTimestamp(util.ParseTimestamp(util.MakeTimestamp(test_time))), util.MakeTimestamp(test_time))
  test_time = test_time.Add(2400*time.Hour)
  check(util.MakeTimestamp(util.ParseTimestamp(util.MakeTimestamp(test_time))), util.MakeTimestamp(test_time))
  
  diff := time.Since(util.ParseTimestamp(util.MakeTimestamp(time.Now())))
  if diff < time.Second { diff = 0 }
  check(diff, time.Duration(0))
  
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
  
  testBase64()
}

func testBase64() {
  check(util.Base64DecodeString("", nil), []byte{})
  check(util.Base64DecodeInPlace([]byte{}), []byte{})
  check(util.Base64DecodeString("=", nil), []byte{})
  check(util.Base64DecodeInPlace([]byte("=")), []byte{})
  check(util.Base64DecodeString("  =  ", nil), []byte{})
  check(util.Base64DecodeInPlace([]byte("  =  ")), []byte{})
  check(util.Base64DecodeString("/+/+", nil), []byte{0xff,0xef,0xfe})
  check(util.Base64DecodeInPlace([]byte("/+/+")), []byte{0xff,0xef,0xfe})
  check(util.Base64DecodeString("_-_-", nil), []byte{0xff,0xef,0xfe})
  check(util.Base64DecodeInPlace([]byte("_-_-")), []byte{0xff,0xef,0xfe})
  var devnull int
  check(string(util.Base64DecodeString("SGFsbG8=", nil)), "Hallo")
  check(string(util.Base64DecodeInPlace([]byte("SGFsbG8="))), "Hallo")
  check(string(util.Base64DecodeString("SGFsbA==", nil)), "Hall")
  check(string(util.Base64DecodeInPlace([]byte("SGFsbA=="))), "Hall")
  check(string(util.Base64DecodeString("SGFsbG8", nil)), "Hallo")
  check(string(util.Base64DecodeInPlace([]byte("SGFsbG8"))), "Hallo")
  check(string(util.Base64DecodeString("SGFsbA=", nil)), "Hall")
  check(string(util.Base64DecodeInPlace([]byte("SGFsbA="))), "Hall")
  check(string(util.Base64DecodeString("SGFsbG8===", nil)), "Hallo")
  check(string(util.Base64DecodeInPlace([]byte("SGFsbG8==="))), "Hallo")
  check(string(util.Base64DecodeString("SGFsbA", nil)), "Hall")
  check(string(util.Base64DecodeInPlace([]byte("SGFsbA"))), "Hall")
  check(string(util.Base64DecodeString("SGFsbG8=", &devnull)), "Hallo")
  check(devnull, 0)
  check(string(util.Base64DecodeString("SGFsbA==", &devnull)), "Hall")
  check(devnull, 0)
  check(string(util.Base64DecodeString("SGFsbA=", &devnull)), "Hall")
  check(devnull, 0)
  check(string(util.Base64DecodeString("SGFsbG8===", &devnull)), "Hallo")
  check(devnull, 0)
  check(string(util.Base64DecodeString("AA", nil)), "\000")
  check(string(util.Base64DecodeInPlace([]byte("AA"))), "\000")
  check(string(util.Base64DecodeString("AAA", nil)), "\000\000")
  check(string(util.Base64DecodeInPlace([]byte("AAA"))), "\000\000")
  var zerocarry int
  check(string(util.Base64DecodeString("AA", &zerocarry)), "")
  check(zerocarry != 0, true)
  check(string(util.Base64DecodeString("=", &zerocarry)), "\000")
  check(zerocarry, 0)
  check(string(util.Base64DecodeString("AAA", &zerocarry)), "")
  check(zerocarry != 0, true)
  check(string(util.Base64DecodeString("=", &zerocarry)), "\000\000")
  check(zerocarry, 0)
  
  testbuf := make([]byte, 1024)
  for i := range testbuf { testbuf[i] = byte(i) }
  
  
  error_list := ""
  for length := 0; length <= 12; length++ {
    for eq := 0; eq <= 4; eq++ {
      for err := 0; err <= 12; err++ {
        b64_1 := base64.StdEncoding.EncodeToString(testbuf[0:512-length])
        
        testslice := b64_1[0:]
        errors := []int{0}
        for e := 0 ; e < err; e++ {
          errors = append(errors, errors[e]+rand.Intn(len(testslice)-errors[e]))
        }
        errors = append(errors, len(testslice))
        teststr := ""
        for i := 0; i < len(errors)-1; i++ {
          if i != 0 { teststr = teststr + "\000\n" }
          teststr = teststr + testslice[errors[i]:errors[i+1]]
        }
        
        for i := 0; i < eq; i++ {
          teststr += "="
        }
        // because we're concatenating we need at least 1 "=" if the
        // first string ends in an incomplete block
        if eq == 0 && (length & 3) != 0 { teststr += "=" } 
        
        b64_2 := base64.URLEncoding.EncodeToString(testbuf[512-length:])

        testslice = b64_2[0:]
        errors = []int{0}
        for e := 0 ; e < err; e++ {
          errors = append(errors, errors[e]+rand.Intn(len(testslice)-errors[e]))
        }
        errors = append(errors, len(testslice))
        for i := 0; i < len(errors)-1; i++ {
          if i != 0 { teststr = teststr + " " }
          teststr = teststr + testslice[errors[i]:errors[i+1]]
        }
        
        for i := 0; i < eq; i++ {
          teststr += "="
        }
        
        for parts := 0; parts < 5; parts++ {
          stops := []int{0}
          for e := 0 ; e < parts; e++ {
            stops = append(stops, stops[e]+rand.Intn(len(teststr)-stops[e]))
          }
          stops = append(stops, len(teststr))
          
          decoded := ""
          carry := 0
          for i := 0; i < len(stops)-1; i++ {
            decoded += string(util.Base64DecodeString(teststr[stops[i]:stops[i+1]], &carry))
          }
          
          if decoded != string(testbuf) {
            error_list += fmt.Sprintf("(util.Base64DecodeString() failed for length=%v eq=%v err=%v parts=%v)\n",length,eq,err,parts)
          }
          
          buffy := []byte(string(teststr))
          decbuffy := util.Base64DecodeInPlace(buffy)
          if &(decbuffy[0]) != &(buffy[0]) || // verify the in-place property
            string(decbuffy) != string(testbuf) {
            error_list += fmt.Sprintf("(util.Base64DecodeInPlace() failed for length=%v eq=%v err=%v parts=%v)\n",length,eq,err,parts)
          }
        }  
      }
    }
  }
  
  check(error_list, "")
  
  buffy := make([]byte, 1024)
  st := "Nehmt, esst. Dies ist mein Leib - sprach der Schokohase"
  copy(buffy[32:], st)
  result := util.Base64EncodeInPlace(buffy[:32+len(st)],32)
  check(&(buffy[0]) == &(result[0]), true)
  check(cap(result), cap(buffy))
  check(string(result), "TmVobXQsIGVzc3QuIERpZXMgaXN0IG1laW4gTGVpYiAtIHNwcmFjaCBkZXIgU2Nob2tvaGFzZQ==")
  
  st = "Nehmt, esst. Dies ist mein Leib - sprach der Schokohase\n"
  copy(buffy[256:], st)
  result = util.Base64EncodeInPlace(buffy[:256+len(st)],256)
  check(&(buffy[0]) == &(result[0]), true)
  check(cap(result), cap(buffy))
  check(string(result), "TmVobXQsIGVzc3QuIERpZXMgaXN0IG1laW4gTGVpYiAtIHNwcmFjaCBkZXIgU2Nob2tvaGFzZQo=")
  
  for n := 0; n <= 12; n++ {
    buffy = make([]byte, n)
    for i := range buffy { buffy[i] = byte(i) }
    check(string(util.Base64EncodeString(string(buffy))), base64.StdEncoding.EncodeToString(buffy))
  }
}

type FlushableBuffer struct {
  Buf bytes.Buffer
  Flushes int
  first bool
}

func (b *FlushableBuffer) Write(p []byte) (n int, err error) {
  if !b.first {time.Sleep(1*time.Second)}
  b.first = true
  return b.Buf.Write(p)
}

func (b *FlushableBuffer) Flush() error {
  b.Flushes++
  return nil
}

type SyncableBuffer struct {
  Buf bytes.Buffer
  Flushes int
  first bool
}

func (b *SyncableBuffer) Write(p []byte) (n int, err error) {
  if !b.first {time.Sleep(1*time.Second)}
  b.first = true
  return b.Buf.Write(p)
}

func (b *SyncableBuffer) Sync() error {
  b.Flushes++
  return nil
}

func testLogging() {
  // Check that os.Stderr is the (only) default logger
  check(util.LoggersCount(),1)
  util.LoggerRemove(os.Stderr)
  check(util.LoggersCount(),0)
  
  // Check that default loglevel is 0
  check(util.LogLevel, 0)
  
  flushy := new(FlushableBuffer)
  synchy := new(SyncableBuffer)
  
  util.LoggerAdd(flushy)
  check(util.LoggersCount(),1)
  util.LoggerAdd(synchy)
  check(util.LoggersCount(),2)
  
  util.LogLevel = 4
  defer func(){ util.LogLevel = 0 }()
  oldfac := util.BacklogFactor
  defer func(){ util.BacklogFactor = oldfac }()
  util.BacklogFactor = 4
  
  util.Log(0, "a0")                                 // 0
  time.Sleep(200*time.Millisecond)
  for i := 1; i <= 4; i++ { util.Log(i, "a%d", i) } //   1,2,3,4
  for i := 0; i <= 4; i++ { util.Log(i, "b%d", i) } // 0,1,2,3
  for i := 0; i <= 4; i++ { util.Log(i, "c%d", i) } // 0,1,2
  for i := 0; i <= 4; i++ { util.Log(i, "d%d", i) } // 0,1
  for i := 0; i <= 4; i++ { util.Log(i, "e%d", i) } // 0,1
  util.Log(1,"x") // should be logged because when this Log() is executed, the backlog is only 15 long
  util.Log(1,"y") // should NOT be logged after the previous "x" the backlog is 16=4*BacklogFactor long
  
  check(flushy.Flushes, 0)
  check(synchy.Flushes, 0)
  
  time.Sleep(2*time.Second)
  check(flushy.Flushes, 1)
  check(synchy.Flushes, 1)
  
  util.Log(5, "Shouldnotbelogged!")
  util.Log(4, "Shouldbelogged!")
  time.Sleep(200*time.Millisecond)
  check(flushy.Flushes, 2)
  check(synchy.Flushes, 2)
  
  util.LoggersSuspend()
  check(util.LoggersCount(),0)
  util.LoggerAdd(os.Stderr)
  check(util.LoggersCount(),1)
  util.LoggersSuspend()
  check(util.LoggersCount(),0)
  util.Log(0, "This should disappear in the void")
  buffy := new(bytes.Buffer)
  util.LoggerAdd(buffy)
  joke := "Sagt die Katze zum Verkäufer: Ich hab nicht genug Mäuse. Kann ich auch in Ratten zahlen?"
  util.Log(0, joke)
  time.Sleep(200*time.Millisecond)
  check(strings.Index(buffy.String(),joke) >= 0, true)
  
  check(util.LoggersCount(),1)
  util.LoggersRestore()
  check(util.LoggersCount(),1)
  util.LoggersRestore()
  check(util.LoggersCount(),2)
  
  util.Log(0, "foo")
  time.Sleep(200*time.Millisecond)
  
  lines := flushy.Buf.Split("\n")
  for i := range lines {
    if strings.Index(lines[i],"missing") < 0 {
      idx := strings.LastIndex(lines[i]," ")
      lines[i] = lines[i][idx+1:]
    } else {
      idx := strings.Index(lines[i]," missing")
      lines[i] = lines[i][idx-2:idx]
    }
  }
  
  check(lines,[]string{"a0","a1","a2","a3","a4","b0","b1","b2","b3","c0","c1","c2","d0","d1","e0","e1","x","10","Shouldbelogged!","foo",""})
  
  
  check(flushy.Buf.String(), synchy.Buf.String())
  
  
  // Reset loggers so that only os.Stderr is a logger
  for util.LoggersCount() > 0 { util.LoggersRestore() }
  util.LoggerAdd(os.Stderr)
}
