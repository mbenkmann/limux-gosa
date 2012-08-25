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

// Various re-usable utility functions.
package util

import (
         "io"
         "fmt"
         "net"
         "time"
         "bytes"
         "crypto/md5"
       )

// Returns the md5sum of its argument as a string of hex digits.
func Md5sum(s string) string {
  md5 := md5.New()
  io.WriteString(md5, s)
  return fmt.Sprintf("%x",md5.Sum(nil))
}

// Number of consecutive short writes before WriteAll() will give up
// ATTENTION! The wait time between tries increases exponetially, so don't
// blindly increase this number.
const write_all_max_tries = 8

// Writes data to w, with automatic handling of short writes.
// A short write error will only be returned if multiple attempts
// failed in a row.
func WriteAll(w io.Writer, data []byte) (n int, err error) {
  // Yeah, I know. Other people just ignore the issue of
  // short writes. That's why their code fails more often than mine :-P
  tries := write_all_max_tries
  var bytes_written int
  for n = 0; n < len(data); {
    bytes_written, err = w.Write(data[n:])
    n += bytes_written
    
    if err != nil && err != io.ErrShortWrite {
      return n, err
    }
    
    if bytes_written == 0 {
      tries--
      if tries <= 0 {
        if err == nil {
          err = io.ErrShortWrite
        }
        return n, err
      }
      
      // The first time we don't sleep. The 2nd time we sleep 1ms. The 3rd time 2ms.
      // The 4th time 4ms. Then 8ms, 16ms, 32ms, 64ms,...
      var wait time.Duration = (1 << (write_all_max_tries-2)) >> uint(tries)
      time.Sleep(wait * time.Millisecond)
      
    } else {
      tries = write_all_max_tries  // every time we succeed at writing we start tries again
    }
  }
  
  return n, nil
}


// Sends strings via connection conn, followed by "\r\n"
func SendLn(conn net.Conn, s string) {
  sendbuf := make([]byte, len(s)+2)
  copy(sendbuf, s)
  sendbuf[len(s)]='\r'
  sendbuf[len(s)+1]='\n'

  conn.SetWriteDeadline(time.Now().Add(5*time.Minute))
  _, err := WriteAll(conn, sendbuf)
  if err != nil {
    Log(0, "ERROR! WriteAll: %v", err)
  }
}

// Reads from the connection until \n is seen (or timeout or error) and
// returns the first line with trailing \n and \r removed.
func ReadLn(conn net.Conn) string {
  var buf = make([]byte, 65536)
  i := 0
  n := 1
  var err error
  for n != 0 {
    conn.SetReadDeadline(time.Now().Add(5*time.Minute))
    n, err = conn.Read(buf[i:])
    if err != nil && err != io.EOF {
      Log(0, "ERROR! Read: %v", err)
    }
    if err == io.EOF && i != 0 {
      Log(0, "ERROR! Incomplete message (i.e. not terminated by \"\\n\") of %v bytes", i)
    }

    i += n
    
    if i == len(buf) {
      buf_new := make([]byte, len(buf)+65536)
      copy(buf_new, buf)
      buf = buf_new
    }

    // Find complete line terminated by '\n' and return it
    eol := bytes.IndexByte(buf[0:i], '\n')
      
    if eol >= 0 {
      for ; eol >= 0 && (buf[eol] == '\n' || buf[eol] == '\r') ; { eol-- }
      return string(buf[0:eol+1])
    }
  }
  
  return ""
}
