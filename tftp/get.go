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
         "fmt"
         "net"
         "time"
         "strings"
         "strconv"
       )

// Performs a TFTP get for path at host (which may optionally include a port;
// if it doesn't, port 69 is used). If timeout != 0 and any individual read
// operation (not the whole get()!) takes longer than that time, get() will
// return an error and potentially a buffer with incomplete data.
// NOTE: path is usually a relative path that does not start with "/".
//
//   ATTENTION!
//   At the moment this function will abort with an error if a packet is
//   lost instead of requesting retransmission.
func Get(host, path string, timeout time.Duration) ([]byte, error) {
  blocksize := 512
  
  if strings.Index(host,":") < 0 { host = host+":69" }
  
  remote_addr, err := net.ResolveUDPAddr("udp4", host)
  if err != nil { return []byte{}, err }
  
  local_addr, err := net.ResolveUDPAddr("udp4", ":0")
  if err != nil { return []byte{}, err }
  
  udp_conn, err := net.ListenUDP("udp4", local_addr)
  if err != nil { return []byte{}, err }
  defer udp_conn.Close()
  local_addr = udp_conn.LocalAddr().(*net.UDPAddr)
  
  _, err = udp_conn.WriteToUDP([]byte("\000\001"+path+"\000octet\000tsize\0000\000"), remote_addr)
  if err != nil { return []byte{}, err }
  
  buf := make([]byte,blocksize)
  
  if timeout != 0 {
    udp_conn.SetReadDeadline(time.Now().Add(timeout))
  }
  n, remote_addr, err := udp_conn.ReadFromUDP(buf)
  if err != nil { return []byte{}, err }
  raddr := remote_addr
  
  ack := []byte{0,4,0,0}
  
  too_short := fmt.Errorf("Received TFTP packet shorter than 4 bytes")
  if n < 4 { return []byte{}, too_short }
  
  for buf[0] == 0 && buf[1] == 6 { //OACK
    parts := strings.Split(string(buf[2:n]),"\000")
    err = fmt.Errorf("TFTP OACK invalid: %#v", string(buf[0:n]))
    if len(parts) != 3 || parts[0] != "tsize" { return []byte{}, err }
    tsize, err := strconv.Atoi(parts[1])
    if err != nil || tsize < 0 { return []byte{}, err }
    buf = make([]byte, tsize+blocksize) // +blocksize to have at least size for a whole block (because an error block may be longer than tsize)
    _, err = udp_conn.WriteToUDP(ack, remote_addr) // send ACK
    if err != nil { return []byte{}, err }
    if timeout != 0 {
      udp_conn.SetReadDeadline(time.Now().Add(timeout))
    }
    n, raddr, err = udp_conn.ReadFromUDP(buf)
    if err != nil { return []byte{}, err }
    if n < 4 { return []byte{}, too_short }
  }
  
  blockid := 1
  sz := 0
  
  // we read directly into the result buffer. Because of the 4 bytes header
  // we overwrite 4 bytes of the old data. These are saved in this slice.
  
  header := buf[0:4]
  buf = buf[4:]
  save := []byte{buf[0],buf[1],buf[2],buf[3]}
  
  for {
    if header[0] == 0 && header[1] == 3 { // DATA
      if header[2] != byte(blockid >> 8) || header[3] != byte(blockid & 0xff) {
        if header[2] == byte((blockid-1) >> 8) && header[3] == byte((blockid-1) & 0xff) {
          // DATA is retransmission. Probably because ACK has been lost. => Resend ACK
          _, err = udp_conn.WriteToUDP(ack, remote_addr) // send ACK
          if err != nil { return buf[0:sz], err }
        } else {
          return buf[0:sz], fmt.Errorf("TFTP packet with incorrect sequence number")
        }
      } else { // correct blockid => Next packet
        ack[2] = byte(blockid >> 8)
        ack[3] = byte(blockid & 0xff)
        copy(header, save)
        sz += n-4
        copy(save, buf[sz-4:])
        if sz-4+blocksize > len(buf) {
          new_buf := make([]byte, sz-4+blocksize)
          copy(new_buf, buf)
          buf = new_buf
        }
        
        _, err = udp_conn.WriteToUDP(ack, remote_addr) // send ACK
        if err != nil { return buf[0:sz], err }
        
        if n < blocksize { break }
        
        blockid++
      }
      
      for {
        if timeout != 0 {
          udp_conn.SetReadDeadline(time.Now().Add(timeout))
        }
        n, raddr, err = udp_conn.ReadFromUDP(buf[sz-4:])
        if raddr.Port != remote_addr.Port { continue } // verify sender
        if err != nil { return buf[0:sz], err }
        if n < 4 { return buf[0:sz], too_short }
        header = buf[sz-4:sz]
        break
      }
    } else { // not a DATA packet
      return buf[0:sz], fmt.Errorf("Unexpected TFTP packet. Expected DATA, got %#v...",string(header))
    }
  }
  
  return buf[0:sz], nil
}
