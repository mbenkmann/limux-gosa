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
*/

package tftp

import (
         "io"
         "fmt"
         "math/rand"
         "net"
         "time"
         "strings"
         
         "github.com/mbenkmann/golib/util"
       )

var too_short = fmt.Errorf("Received TFTP packet shorter than 4 bytes")

const min_wait_retry = 100*time.Millisecond
const max_wait_retry = 200*time.Millisecond

// Sends to_write via udp_conn to remote_addr, then reads from udp_conn into read_buf 
// and returns the number of bytes read and the UDP address from which they were received.
// Errors are reported in the 3rd return value.
// If timeout is non-0 the function will return (with an error if necessary) after no more
// than that duration. Before that time, if an error occurs during sending or reading, the
// function will retry the whole operation (beginning with the write).
// For each individual retry, a random timeout between min_wait_retry and max_wait_retry is
// used (but no more than the remaining time from timeout).
func writeReadUDP(udp_conn *net.UDPConn, remote_addr *net.UDPAddr, to_write, read_buf []byte, min_wait_retry, max_wait_retry, timeout time.Duration) (int, *net.UDPAddr, error) {
  var special_err error
  var err error
  if timeout == 0 { timeout = 365*86400*time.Second }
  endtime := time.Now().Add(timeout)
  if min_wait_retry <= 0 { min_wait_retry++ }
  if max_wait_retry <= min_wait_retry { max_wait_retry = min_wait_retry + 1 }
  
  for {
    _, err = udp_conn.WriteToUDP(to_write, remote_addr)
    if err == nil {
    
      timo := time.Duration(rand.Int63n(int64(max_wait_retry-min_wait_retry))) + min_wait_retry
      endtime2 := time.Now().Add(timo)
      if endtime2.After(endtime) { endtime2 = endtime }
      
      udp_conn.SetReadDeadline(endtime2)
      var n int
      var raddr *net.UDPAddr
      n, raddr, err = udp_conn.ReadFromUDP(read_buf)
      if err == nil { 
        if n < 4 { 
          err = too_short 
        } else {
          return n, raddr, err
        }
      }
    }
    
    if e,ok := err.(*net.OpError); !ok || !e.Timeout() { special_err = err }
    
    if time.Now().After(endtime) { break }
  }
  
  if special_err != nil { return 0, nil, special_err }
  
  return 0, nil, err
}

// Performs a TFTP get for path at host (which may optionally include a port;
// if it doesn't, port 69 is used). If timeout != 0 and any individual read
// operation (not the whole get()!) takes longer than that time, get() will
// return an error. All the read data is written into w. w is NOT closed!
// NOTE: path is usually a relative path that does not start with "/".
func Get(host, path string, w io.Writer, timeout time.Duration) error {
  blocksize := 512
  buf := make([]byte,blocksize)
  
  if strings.Index(host,":") < 0 { host = host+":69" }
  
  remote_addr, err := net.ResolveUDPAddr("udp4", host)
  if err != nil { return err }
  
  local_addr, err := net.ResolveUDPAddr("udp4", ":0")
  if err != nil { return err }
  
  udp_conn, err := net.ListenUDP("udp4", local_addr)
  if err != nil { return err }
  defer udp_conn.Close()
  local_addr = udp_conn.LocalAddr().(*net.UDPAddr)
  
  n, remote_addr, err := writeReadUDP(udp_conn,remote_addr,[]byte("\000\001"+path+"\000octet\000"),buf,min_wait_retry, max_wait_retry,timeout)
  if err != nil { return err }
  
  raddr := remote_addr
  
  ack := []byte{0,4,0,0}
  
  blockid := 1
  
  for {
    if buf[0] == 0 && buf[1] == 3 { // DATA
      if buf[2] != byte(blockid >> 8) || buf[3] != byte(blockid & 0xff) {
        if buf[2] == byte((blockid-1) >> 8) && buf[3] == byte((blockid-1) & 0xff) {
          // DATA is retransmission. Probably because ACK has been lost. => Ignore. We'll resend ACK further below
        } else {
          return fmt.Errorf("TFTP packet with incorrect sequence number")
        }
      } else { // correct blockid => Next packet
        
        _, err = util.WriteAll(w,buf[4:n])
        if err != nil { return err }
        
        ack[2] = byte(blockid >> 8)
        ack[3] = byte(blockid & 0xff)

        if n < blocksize { // was the received packet the last one?
          _, err = udp_conn.WriteToUDP(ack, remote_addr) // send ACK and ...
          break //... stop
        }
        
        blockid++
      }
      
      for {
        n, raddr, err = writeReadUDP(udp_conn,remote_addr,ack,buf,min_wait_retry, max_wait_retry,timeout)
        if raddr != nil && raddr.Port != remote_addr.Port { continue } // verify sender
        if err != nil { return err }
        break
      }
    } else { // not a DATA packet
      return fmt.Errorf("Unexpected TFTP packet. Expected DATA, got %#v...",string(buf[0:n]))
    }
  }
  
  return nil
}
