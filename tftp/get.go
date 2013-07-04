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
         "net"
         "time"
         "strings"
         "strconv"
         
         "../util"
       )

// Performs a TFTP get for path at host (which may optionally include a port;
// if it doesn't, port 69 is used). If timeout != 0 and any individual read
// operation (not the whole get()!) takes longer than that time, get() will
// return an error. All the read data is written into w. w is NOT closed!
// NOTE: path is usually a relative path that does not start with "/".
//
//   ATTENTION!
//   At the moment this function does not have code to request a
//   retransmission after a timeout.
func Get(host, path string, w io.Writer, timeout time.Duration) error {
  blocksize := 512
  
  if strings.Index(host,":") < 0 { host = host+":69" }
  
  remote_addr, err := net.ResolveUDPAddr("udp4", host)
  if err != nil { return err }
  
  local_addr, err := net.ResolveUDPAddr("udp4", ":0")
  if err != nil { return err }
  
  udp_conn, err := net.ListenUDP("udp4", local_addr)
  if err != nil { return err }
  defer udp_conn.Close()
  local_addr = udp_conn.LocalAddr().(*net.UDPAddr)
  
  _, err = udp_conn.WriteToUDP([]byte("\000\001"+path+"\000octet\000tsize\0000\000"), remote_addr)
  if err != nil { return err }
  
  buf := make([]byte,blocksize)
  
  if timeout != 0 {
    udp_conn.SetReadDeadline(time.Now().Add(timeout))
  }
  n, remote_addr, err := udp_conn.ReadFromUDP(buf)
  if err != nil { return err }
  raddr := remote_addr
  
  ack := []byte{0,4,0,0}
  
  too_short := fmt.Errorf("Received TFTP packet shorter than 4 bytes")
  if n < 4 { return too_short }
  
  for buf[0] == 0 && buf[1] == 6 { //OACK
    parts := strings.Split(string(buf[2:n]),"\000")
    err = fmt.Errorf("TFTP OACK invalid: %#v", string(buf[0:n]))
    if len(parts) != 3 || parts[0] != "tsize" { return err }
    tsize, err := strconv.Atoi(parts[1])
    if err != nil || tsize < 0 { return err }
    buf = make([]byte, tsize+blocksize) // +blocksize to have at least size for a whole block (because an error block may be longer than tsize)
    _, err = udp_conn.WriteToUDP(ack, remote_addr) // send ACK
    if err != nil { return err }
    if timeout != 0 {
      udp_conn.SetReadDeadline(time.Now().Add(timeout))
    }
    n, raddr, err = udp_conn.ReadFromUDP(buf)
    if err != nil { return err }
    if n < 4 { return too_short }
  }
  
  blockid := 1
  
  for {
    if buf[0] == 0 && buf[1] == 3 { // DATA
      if buf[2] != byte(blockid >> 8) || buf[3] != byte(blockid & 0xff) {
        if buf[2] == byte((blockid-1) >> 8) && buf[3] == byte((blockid-1) & 0xff) {
          // DATA is retransmission. Probably because ACK has been lost. => Resend ACK
          _, err = udp_conn.WriteToUDP(ack, remote_addr) // send ACK
          if err != nil { return err }
        } else {
          return fmt.Errorf("TFTP packet with incorrect sequence number")
        }
      } else { // correct blockid => Next packet
        
        _, err = util.WriteAll(w,buf[4:n])
        if err != nil { return err }
        
        ack[2] = byte(blockid >> 8)
        ack[3] = byte(blockid & 0xff)

        _, err = udp_conn.WriteToUDP(ack, remote_addr) // send ACK
        if err != nil { return err }
        
        if n < blocksize { break }
        
        blockid++
      }
      
      for {
        if timeout != 0 {
          udp_conn.SetReadDeadline(time.Now().Add(timeout))
        }
        n, raddr, err = udp_conn.ReadFromUDP(buf)
        if raddr.Port != remote_addr.Port { continue } // verify sender
        if err != nil { return err }
        if n < 4 { return too_short }
        break
      }
    } else { // not a DATA packet
      return fmt.Errorf("Unexpected TFTP packet. Expected DATA, got %#v...",string(buf[0:n]))
    }
  }
  
  return nil
}
