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

// Read-only TFTP server and other TFTP routines.
package tftp

import (
         "io"
         "os"
         "os/exec"
         "fmt"
         "net"
         "math/rand"
         "sync"
         "time"
         "strconv"
         "strings"
         "regexp"
        
         "../db"
         "github.com/mbenkmann/golib/util"
         "github.com/mbenkmann/golib/bytes"
         "../config"
       )

// Accepts UDP connections for TFTP requests on listen_address, serves read requests
// for path P based on request_re and reply as follows:
//
// request_re and reply have to be lists of
// equal length. Let request_re[i] be the first entry in request_re that
// matches P, then reply[i] specifies the data to return for the request.
// If reply[i] == "", then a file not found error is returned to the requestor.
// If reply[i] starts with the character '|', the remainder is taken as the path
// of a hook to execute and its stdout is returned to the requestor.
// Otherwise reply[i] is taken as the path of the file whose contents to send to
// the requestor.
//
// When executing a hook, an environment variable called "tftp_request"
// is passed containing P. If request_re[i] has a capturing
// group named "macaddress", the captured substring will be converted to
// a MAC address by converting to lowercase, removing all characters
// except 0-9a-f, left-padding to length 12 with 0s or truncating to length 12
// and inserting ":"s. The result will be added to
// the hook environment in a variable named "macaddress" and if there
// is an LDAP object for that macaddress, its attributes will be added
// to the environment, too.
//
// Named subexpressions in request_re[i] other than "macaddress" will be
// exported to the hook verbatim in like-named environment variables.
func ListenAndServe(listen_address string, request_re []*regexp.Regexp, reply []string) {
  for i := range request_re {
    util.Log(1, "INFO! TFTP: %v -> %v", request_re[i], reply[i])
  }
  
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
    
    go util.WithPanicHandler(func(){handleConnection(return_addr, payload, request_re, reply)})
    
  }
}

type cacheEntry interface {
  Bytes() []byte
  Release()
}

type bufCacheEntry struct {
  Data bytes.Buffer
  Mutex sync.Mutex
  LoadCount int
  Err error
  Afterlifetime time.Duration
}

func (f *bufCacheEntry) Release() {
  f.Mutex.Lock()
  defer f.Mutex.Unlock()
  
  // If there are still multiple users left, decrease loadCount immediately
  if f.LoadCount > 1 { f.LoadCount-- } else {
    // If this Release() call is by the last remaining user,
    // hold on to the data for a while longer in case a new
    // user pops up.
    go func() {
      time.Sleep(f.Afterlifetime)
      f.Mutex.Lock()
      defer f.Mutex.Unlock()
      f.LoadCount--
      if f.LoadCount == 0 { f.Data.Reset(); f.Err = nil; }
    }()
  }
}

func (f *bufCacheEntry) Bytes() []byte {
  f.Mutex.Lock()
  defer f.Mutex.Unlock()
  data := f.Data.Bytes()
  return data
}

var cache = map[string]*bufCacheEntry{}
var cacheMutex sync.Mutex

func getCacheEntry(key string, afterlife time.Duration) *bufCacheEntry {
  cacheMutex.Lock()
  defer cacheMutex.Unlock()
  entry, have_entry := cache[key]
  if !have_entry {
    entry = &bufCacheEntry{Afterlifetime: afterlife}
    cache[key] = entry
  }
  return entry
}

// Returns the data for the given request. request_re and reply are lists of
// equal length. If request matches request_re[i], then reply[i] specifies the
// data to return for the request. If reply[i] == "",
// then this function returns (nil,nil). If reply[i] starts with the
// character '|', the remainder is taken as the path of a hook to execute
// to generate the data. Otherwise reply[i] is taken as the path of the
// file whose contents to return as data.
//
// When executing a hook, an environment variable called "tftp_request"
// is passed containing the request string. If request_re[i] has a capturing
// group named "macaddress", the captured substring will be converted to
// a MAC address by converting to lowercase, removing all characters
// except 0-9a-f, left-padding to
// length 12 with 0s or truncating to length 12 and inserting ":"s. The
// result will be added to
// the hook environment in a variable named "macaddress" and if there
// is an LDAP object for that macaddress, its attributes will be added
// to the environment, too.
// Other named subexpressions in request_re[i] will be exported to the hook
// verbatim in like-named environment variables.
//
// ATTENTION! Do not forget to call Release() on the returned cacheEntry when you're
// done using it.
func getFile(request string, request_re []*regexp.Regexp, reply []string) (cacheEntry,error) {
  
  for i := range request_re {
    if subs := request_re[i].FindStringSubmatch(request); subs != nil {
      if reply[i] == "" { return nil, nil }
      
      if reply[i][0] != '|' { // plain file
        subsidx := request_re[i].FindStringSubmatchIndex(request)
        fpath := string(request_re[i].ExpandString(nil, reply[i], request, subsidx))
        util.Log(1, "INFO! TFTP mapping \"%v\" => \"%v\"", request, fpath)
        
        // We use fpath as cache key instead of request because
        // multiple requests may map to the same fpath and we want to avoid
        // caching the same file multiple times.
        entry := getCacheEntry(fpath, 60*time.Second)
        
        entry.Mutex.Lock()
        defer entry.Mutex.Unlock()
        
        if entry.LoadCount == 0 {
          file, err := os.Open(fpath) 
          entry.Err = err
          if err == nil {
            defer file.Close()
            
            buffy := make([]byte,65536)
            for {
              n, err := file.Read(buffy)
              entry.Data.Write(buffy[0:n])
              if err == io.EOF { break }
              if err != nil { 
                entry.Data.Reset()
                entry.Err = err
              }
              if n == 0 {
                util.Log(0, "WARNING! Read returned 0 bytes but no error. Assuming EOF")
                break
              }
            }
          }
        } else {
          util.Log(1, "INFO! TFTP: Serving %v from cache", fpath)
        }
        
        entry.LoadCount++
        
        return entry, entry.Err
        
      } else { // hook
        hook := reply[i][1:] // cut off '|'
        
        // We need a few seconds afterlife to deal with multiple requests in
        // short succession by the same loader due to delayed UDP packets.
        entry := getCacheEntry(request, 5*time.Second)
        
        entry.Mutex.Lock()
        defer entry.Mutex.Unlock()
        
        if entry.LoadCount == 0 {
          util.Log(1, "INFO! TFTP: Calling %v to generate %v", hook, request)
        
          env := config.HookEnvironment()
          env = append(env, "tftp_request="+request)
          
          for k, varname := range request_re[i].SubexpNames() {
            if varname == "" { continue }

            value := subs[k]
            
            if varname == "macaddress" {
              format_mac := func(r rune) rune {
                switch {
                case r >= 'a' && r <= 'f': return r
                case r >= '0' && r <= '9': return r
                case r >= 'A' && r <= 'F': return 'a'+(r-'A')
                }
                return -1
              }
              
              value = "000000000000" + strings.Map(format_mac, value)
              value = value[len(value)-12:]
              value = value[0:2] + ":" + value[2:4] + ":" + value[4:6] + ":" + value[6:8] + ":" + value[8:10] + ":" + value[10:12]
              
              sys, err := db.SystemGetAllDataForMAC(value, true)
              
              if err != nil {
                if _, not_found := err.(db.SystemNotFoundError); not_found {
                  util.Log(1, "INFO! TFTP: %v", err)
                } else {
                  util.Log(0, "ERROR! TFTP: %v", err)
                }
                // Don't abort. The hook will generate a default config.
              } else {
                // Add environment variables with system's data for the hook
                for _, tag := range sys.Subtags() {
                  env = append(env, tag+"="+strings.Join(sys.Get(tag),"\n"))
                }
              }
            }
            
            env = append(env, varname+"="+value)
          }
          
          hook_fields := strings.Fields(hook)
          cmd := exec.Command(hook_fields[0], hook_fields[1:]...)
          cmd.Env = append(env, os.Environ()...)
          var errbuf bytes.Buffer
          defer errbuf.Reset()
          cmd.Stdout = &entry.Data
          cmd.Stderr = &errbuf
          err := cmd.Run()
          if err != nil {
            util.Log(0, "ERROR! TFTP: error executing %v: %v (%v)", hook, err, errbuf.String())
            entry.Err = err
          } else {
            if entry.Data.Len() > 512 {
              util.Log(1, "INFO! TFTP: Generated %v: %v bytes", request, entry.Data.Len())
            } else {
              util.Log(1, "INFO! TFTP: Generated %v:\n%v", request, entry.Data.String())
            }
          }
        } else {
          util.Log(1, "INFO! TFTP: Serving %v from cache", request)
        }
        
        entry.LoadCount++
        
        return entry, entry.Err
      }
    }
  }
  
  errentry := &bufCacheEntry{LoadCount:1000, Err:fmt.Errorf("TFTP not configured to serve file \"%v\"", request)}
  return errentry, errentry.Err
}

// Sends a TFTP ERROR to addr with the given error code and error message emsg.
func sendError(udp_conn *net.UDPConn, addr *net.UDPAddr, code byte, emsg string) {
  util.Log(0, emsg)
  sendErrorWithoutLogging(udp_conn, addr, code, emsg)
}

func sendErrorWithoutLogging(udp_conn *net.UDPConn, addr *net.UDPAddr, code byte, emsg string) {
  sendbuf := make([]byte, 5+len(emsg))
  sendbuf[0] = 0
  sendbuf[1] = 5 // 5 => opcode for ERROR
  sendbuf[2] = 0
  sendbuf[3] = code // error code
  copy(sendbuf[4:], emsg)
  sendbuf[len(sendbuf)-1] = 0 // 0-terminator
  udp_conn.Write(sendbuf)
}

const total_timeout = 3 * time.Second

// Sends the data in sendbuf to peer_addr (with possible resends) and waits for
// an ACK with the correct block id, if sendbuf contains a DATA message.
// Returns true if the sending was successful and the ACK was received.
func sendAndWaitForAck(udp_conn *net.UDPConn, peer_addr *net.UDPAddr, sendbuf []byte, retransmissions, dups, strays *int) bool {
  // absolute deadline when this function will return false
  deadline := time.Now().Add(total_timeout)

  readbuf := make([]byte, 4096)
  
  *retransmissions-- // to counter the ++ being done at the start of the loop
  
  outer:
  for {
    // re/send
    *retransmissions++
    n,err := udp_conn.Write(sendbuf)
    if err != nil { 
      util.Log(0, "ERROR! TFTP error in Write(): %v", err)
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
      timo := time.Duration(rand.Int63n(int64(max_wait_retry-min_wait_retry))) + min_wait_retry
      endtime2 := time.Now().Add(timo)
      if endtime2.After(deadline) { endtime2 = deadline }
      udp_conn.SetReadDeadline(endtime2)
     
      n, from, err := udp_conn.ReadFromUDP(readbuf)
      
      if err != nil { 
        e,ok := err.(*net.OpError)
        if !ok || !e.Timeout() {
          util.Log(0, "ERROR! TFTP ReadFromUDP() failed while waiting for ACK from %v (local address: %v): %v", udp_conn.RemoteAddr(), udp_conn.LocalAddr(), err)
          break outer // retries make no sense => bail out
        } else {
          //util.Log(2, "DEBUG! TFTP timeout => resend %#v", sendbuf)
          continue outer // resend
        }
      }
      if from.Port != peer_addr.Port {
        *strays++
        emsg := fmt.Sprintf("WARNING! TFTP server got UDP packet from incorrect source: %v instead of %v", from.Port, peer_addr.Port)
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
            util.Log(0, "ERROR! TFTP ERROR received while waiting for ACK from %v: %v", peer_addr, string(readbuf[4:n]))
            break outer // retries make no sense => bail out
          } else {
            // if we sent DATA but the ACK is not for the block we sent,
            // increase dup counter. If we wanted to be anal we would need to check
            // if the block id is one less for it to be an actual dup, but
            // since the dup counter is only for reporting, we don't care.
            if sendbuf[1] == 3 && (readbuf[2] != sendbuf[2] || readbuf[3] != sendbuf[3]) {
              *dups++
              //util.Log(2, "DEBUG! TFTP duplicate ACK received: %#v => Ignored", string(readbuf[0:n]))
              
              // ONLY "continue", NOT "continue outer", i.e. DUPs DO NOT CAUSE A RESEND.
              // THIS PREVENTS http://en.wikipedia.org/wiki/Sorcerer's_Apprentice_Syndrome
              // When timeout happens, it will cause a resend.
              continue
            } else {
              emsg := fmt.Sprintf("ERROR! TFTP server waiting for ACK from %v but got: %#v",peer_addr, string(readbuf[0:n]))
              sendError(udp_conn, from, 0, emsg) // 0 => Unspecified error
              break outer // retries make no sense => bail out
            }
          }
        }
    }
  }
  
  util.Log(0, "ERROR! TFTP send not acknowledged by %v (retransmissions: %v, dups: %v, strays: %v)", peer_addr, *retransmissions, *dups, *strays)
  
  return false
}

func handleConnection(peer_addr *net.UDPAddr, payload string, request_re []*regexp.Regexp, reply []string) {
  retransmissions := 0
  dups := 0
  strays := 0
  
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
  
  filedata, err := getFile(request[0], request_re, reply)
  if filedata == nil {
    util.Log(1, "INFO! TFTP: Returning \"File not found\" as configured for \"%v\"", request[0])
    sendErrorWithoutLogging(udp_conn, peer_addr, 1, "File not found") // 1 => File not found
    return
  }
  
  defer filedata.Release()
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
    if !sendAndWaitForAck(udp_conn, peer_addr, sendbuf, &retransmissions, &dups, &strays) { return }
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
    if !sendAndWaitForAck(udp_conn, peer_addr, sendbuf[0:sz+4], &retransmissions, &dups, &strays) { return }
    start += sz
    blockid++    
    if sz < blocksize { break }
  }
  
  util.Log(1, "INFO! TFTP successfully sent %v to %v (retransmissions: %v, dups: %v, strays:%v)", request[0], peer_addr, retransmissions, dups, strays)
}
