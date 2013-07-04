/* Copyright (C) 2013 Matthias S. Benkmann
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this file (originally named pxelinux.go) and associated documentation files 
 * (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is furnished
 * to do so, subject to the following conditions:
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE. 
 */

// Read-only TFTP server for pxelinux-related files and other TFTP routines.
package tftp

import (
         "io"
         "os"
         "os/exec"
         "fmt"
         "net"
         "time"
         "strconv"
         "strings"
         "regexp"
        
         "../db"
         "../util"
         "../bytes"
       )

// Accepts UDP connections for TFTP requests on listen_address, serves read requests
// for path P by sending the file at local path files[P], with a special case
// for every path of the form "pxelinux.cfg/01-ab-cd-ef-gh-ij-kl" where the latter
// part is a MAC address. For these requests the LDAP object is extracted and passed
// via environment variables to the executable at path pxelinux_hook. Its stdout is
// sent to the requestor.
func ListenAndServe(listen_address string, files map[string]string, pxelinux_hook string) {
  util.Log(1, "INFO! TFTP: Serving actual files %v and virtual files pxelinux.cfg/01-MM-AA-CA-DD-RE-SS via hook %v", files, pxelinux_hook)
  
  udp_addr,err := net.ResolveUDPAddr("udp", listen_address)
  if err != nil {
    util.Log(0, "ERROR! Cannot start TFTP server: %v", err)
    return
  }
  
  udp_conn,err := net.ListenUDP("udp", udp_addr)
  if err != nil {
    util.Log(0, "ERROR! ListenUDP(): %v", err)
    return
  }
  defer udp_conn.Close()
  
  readbuf := make([]byte, 16384)
  for {
    n, return_addr, err := udp_conn.ReadFromUDP(readbuf)
    if err != nil {
      util.Log(0, "ERROR! ReadFromUDP(): %v", err)
      continue
    }

    // Make a copy of the buffer BEFORE starting the goroutine to prevent subsequent requests from
    // overwriting the buffer.
    payload := string(readbuf[:n])
    
    go util.WithPanicHandler(func(){handleConnection(return_addr, payload, files, pxelinux_hook)})
    
  }
}

var pxelinux_cfg_mac_regexp = regexp.MustCompile("^pxelinux.cfg/01-[0-9a-f]{2}(-[0-9a-f]{2}){5}$")

// Returns the data for the request "name" either from a mapping files[name]
// gives the real filesystem path, or by generating data using pxelinux_hook.
func getFile(name string, files map[string]string, pxelinux_hook string) (*bytes.Buffer,error) {
  var data = new(bytes.Buffer)
  if fpath, found := files[name]; found {
    util.Log(1, "INFO! TFTP mapping \"%v\" => \"%v\"", name, fpath)
    f, err := os.Open(fpath) 
    if err != nil { return data, err }
    defer f.Close()
    
    buffy := make([]byte,65536)
    for {
      n, err := f.Read(buffy)
      data.Write(buffy[0:n])
      if err == io.EOF { break }
      if err != nil { 
        data.Reset()
        return data, err 
      }
      if n == 0 {
        util.Log(0, "WARNING! Read returned 0 bytes but no error. Assuming EOF")
        break
      }
    }
    return data, nil
    
  } else if pxelinux_cfg_mac_regexp.MatchString(name) {
    mac := strings.Replace(name[16:],"-",":",-1)
    
    util.Log(1, "INFO! TFTP: Calling %v to generate pxelinux.cfg for %v", pxelinux_hook, mac)
    
    env := []string{}
    sys, err := db.SystemGetAllDataForMAC(mac, true)
    
    if err != nil {
      util.Log(0, "ERROR! TFTP: %v", err)
      // Don't abort. The hook will generate a default config.
      env = append(env,"macaddress="+mac)
    } else {
      // Add environment variables with system's data for the hook
      for _, tag := range sys.Subtags() {
        env = append(env, tag+"="+strings.Join(sys.Get(tag),"\n"))
      }
    }
    
    cmd := exec.Command(pxelinux_hook)
    cmd.Env = append(env, os.Environ()...)
    var errbuf bytes.Buffer
    defer errbuf.Reset()
    cmd.Stdout = data
    cmd.Stderr = &errbuf
    err = cmd.Run()
    if err != nil {
      util.Log(0, "ERROR! TFTP: error executing %v: %v (%v)", pxelinux_hook, err, errbuf.String())
      return data, err
     }
     
     util.Log(1, "INFO! TFTP: Generated %v:\n%v", name, data.String())
     return data,err
  }
  
  return data, fmt.Errorf("TFTP not configured to serve file \"%v\"", name)
}

// Sends a TFTP ERROR to addr with the given error code and error message emsg.
func sendError(udp_conn *net.UDPConn, addr *net.UDPAddr, code byte, emsg string) {
  util.Log(0, emsg)
  sendbuf := make([]byte, 5+len(emsg))
  sendbuf[0] = 0
  sendbuf[1] = 5 // 5 => opcode for ERROR
  sendbuf[2] = 0
  sendbuf[3] = code // error code
  copy(sendbuf[4:], emsg)
  sendbuf[len(sendbuf)-1] = 0 // 0-terminator
  udp_conn.Write(sendbuf)
}

const num_tries = 3
const timeout = 3

// Sends the data in sendbuf to peer_addr (with possible resends) and waits for
// an ACK with the correct block id, if sendbuf contains a DATA message.
// Returns true if the sending was successful and the ACK was received.
func sendAndWaitForAck(udp_conn *net.UDPConn, peer_addr *net.UDPAddr, sendbuf []byte) bool {
  // absolute deadline when this function will return false
  deadline := time.Now().Add(num_tries * timeout * time.Second)

  readbuf := make([]byte, 4096)
  
  outer:
  for {
    // re/send
    n,err := udp_conn.Write(sendbuf)
    if err != nil { 
      util.Log(0, "ERROR! Write(): %v", err)
      break
    }
    if n != len(sendbuf) {
      util.Log(0, "ERROR! TFTP: Incomplete write")
      break
    }
    //util.Log(2, "DEBUG! TFTP: Sent %v bytes to %v. Waiting for ACK...", len(sendbuf), peer_addr)
    
    for {
      // check absolute deadline
      if time.Now().After(deadline) { break outer}
      
      // set deadline for next read
      udp_conn.SetReadDeadline(time.Now().Add(timeout * time.Second))
     
      n, from, err := udp_conn.ReadFromUDP(readbuf)
      
      if err != nil { 
        if e,ok := err.(*net.OpError); !ok || !e.Timeout() {
          util.Log(0, "ERROR! ReadFromUDP(): %v", err)
        }
        continue 
      }
      if from.Port != peer_addr.Port {
        emsg := fmt.Sprintf("ERROR! UDP packet from incorrect source: %v instead of %v", from.Port, peer_addr.Port)
        sendError(udp_conn, from, 5, emsg) // 5 => Unknown transfer ID
        continue // This error is not fatal since it doesn't affect our peer
      }
      if n == 4 && readbuf[0] == 0 && readbuf[1] == 4 && // 4 => ACK
        (sendbuf[1] != 3 ||  // we did not send DATA 
          // or the ACK's block id is the same as the one we sent
        (readbuf[2] == sendbuf[2] && readbuf[3] == sendbuf[3])) {
          //util.Log(2, "DEBUG! TFTP: Received ACK from %v: %#v", peer_addr, readbuf[0:n])
          return true
        } else {
          if readbuf[0] == 0 && readbuf[1] == 5 { // error
            util.Log(2, "DEBUG! TFTP ERROR received while waiting for ACK from %v: %v", peer_addr, string(readbuf[4:n]))
            break outer // retries make no sense => bail out
          } else {
            util.Log(2, "DEBUG! TFTP waiting for ACK but received: %#v", string(readbuf[0:n]))
          }
        }
    }
  }
  
  util.Log(0, "ERROR! TFTP send not acknowledged by %v", peer_addr)
  
  return false
}

func handleConnection(peer_addr *net.UDPAddr, payload string, files map[string]string, pxelinux_hook string) {
  udp_conn, err := net.DialUDP("udp", nil, peer_addr)
  if err != nil {
    util.Log(0, "ERROR! DialUDP(): %v", err)
    return
  }
  defer udp_conn.Close()
  
  request := []string{}
  if len(payload) > 2 { 
    request = strings.SplitN(payload[2:], "\000", -1)
  }
  
  if len(payload) < 6 || payload[0] != 0 || payload[1] != 1 || 
     len(request) < 2 || strings.ToLower(request[1]) != "octet" ||
     // disallow empty file name as well as file names starting with "." or ending with "/"
     request[0] == "" || request[0][0] == '.' || request[0][len(request[0])-1] == '/' {
    
    if len(payload) > 256 { payload = payload[0:256] }
    emsg := fmt.Sprintf("ERROR! TFTP initial request from %v not understood: %#v", peer_addr, payload)
    sendError(udp_conn, peer_addr, 4, emsg) // 4 => illegal TFTP operation
    return
  }
  
  options := request[2:]
  util.Log(1, "INFO! TFTP read: %v requests %v with options %v", peer_addr, request[0], options)
  
  filedata, err := getFile(request[0], files, pxelinux_hook)
  defer filedata.Reset()
  if err != nil {
    emsg := fmt.Sprintf("ERROR! TFTP read error: %v", err)
    sendError(udp_conn, peer_addr, 1, emsg) // 1 => File not found
    return
  }
  
  data := filedata.Bytes()
  
  blocksize := 512
  
  // Process options in request
  oack := []string{}
  for i := 0; i+1 < len(options); i+=2 {
    option := strings.ToLower(options[i])
    value := options[i+1]
    
    if option == "blksize" {
      new_bs, err := strconv.Atoi(value)
      if err == nil && new_bs > 0 && new_bs <= 65536 {
        blocksize = new_bs
        oack = append(oack, option, value)
      }
    }
    
    if option == "tsize" {
      oack = append(oack, option, strconv.Itoa(len(data)))
    }
  }

  // Send OACK if we support any of the requested options
  if len(oack) > 0 {
    opts := strings.Join(oack,"\000")
    sendbuf := make([]byte, 3+len(opts))
    sendbuf[0] = 0
    sendbuf[1] = 6 // 6 => opcode for OACK
    copy(sendbuf[2:], opts)
    sendbuf[len(sendbuf)-1] = 0 // 0-terminator
    util.Log(2, "DEBUG! TFTP: Sending OACK to %v for options %v", peer_addr, oack)
    if !sendAndWaitForAck(udp_conn, peer_addr, sendbuf) { return }
  }
  
  
  blockid := 1
  
  sendbuf := make([]byte, blocksize+4)
  sendbuf[0] = 0
  sendbuf[1] = 3 // 3 => DATA
  
  start := 0
  
  for {
    sz := len(data) - start
    if sz > blocksize { sz = blocksize }
    sendbuf[2] = byte(blockid >> 8)
    sendbuf[3] = byte(blockid & 0xff)
    copy(sendbuf[4:],data[start:start+sz])
    if !sendAndWaitForAck(udp_conn, peer_addr, sendbuf[0:sz+4]) { return }
    start += sz
    blockid++    
    if sz < blocksize { break }
  }
  
  util.Log(1, "INFO! TFTP successfully sent %v to %v", request[0], peer_addr)
}
