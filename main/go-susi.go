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

// The go-susi.go main program as well as associated programs.
//
//  go-susi - the daemon.
//  decrypt - decrypt messages encrypted with the GOsa/gosa-si scheme.
//  encrypt - encrypt messages with the GOsa/gosa-si scheme.
//  run_tests - runs the unit tests.
//  sibridge - interactive/scripting interface to go-susi
package main

import (
          "io"
          "os"
          "os/signal"
          "fmt"
          "net"
          "log"
          "path"
          "bytes"
          "strings"
          "syscall"
          
          "../db"
          "../util"
          "../config"
          "../action"
          "../message"
       )

const USAGE = `go-susi

Starts the daemon.
`

func main() {
  config.ReadArgs(os.Args[1:])
  
  if config.PrintVersion {
    fmt.Printf(`go-susi %v (revision %v)
Copyright (c) 2012 Matthias S. Benkmann
This is free software; see the source for copying conditions.  There is NO
warranty; not even for MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

`, config.Version, config.Revision)
  }
  
  if config.PrintHelp {
    fmt.Println(`USAGE: go-susi [args]

--help       print this text and exit
--version    print version and exit

-v           print operator debug messages (INFO)
-vv          print developer debug messages (DEBUG)
             ATTENTION! developer messages include keys!

-f           start with a fresh database; discard old /var/lib/go-susi

--test=<dir> test mode:
             * read config files from <dir> instead of /etc/gosa-si
             * use <dir>/go-susi.log as log file
             * use <dir> as database directory instead /var/lib/go-susi

-c <file>    read config from <file> instead of default location
`)
  }
  
  if config.PrintVersion || config.PrintHelp { os.Exit(0) }
  
  config.ReadConfig()
  
  logfile, err := os.OpenFile(config.LogFilePath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
  if err != nil {
    util.Log(0, "ERROR! %v", err)
    // Do not exit. We can go on without logging to a file.
  
  } else {
    // Send log output to both stderr AND the log file
    logfile.Close() // will be re-opened on the first write
    util.Logger = log.New(io.MultiWriter( os.Stderr, util.LogFile(logfile.Name())), "",0)
  }
  util.LogLevel = config.LogLevel
  
  os.MkdirAll(path.Dir(config.JobDBPath), 0750)
  
  config.ReadNetwork() // after config.ReadConfig()
  db.ServersInit() // after config.ReadNetwork()
  db.JobsInit() // after config.ReadConfig()
  action.Init()
  
  tcp_addr, err := net.ResolveTCPAddr("ip4", config.ServerListenAddress)
  if err != nil {
    util.Log(0, "ERROR! ResolveTCPAddr: %v", err)
    os.Exit(1)
  }

  listener, err := net.ListenTCP("tcp4", tcp_addr)
  if err != nil {
    util.Log(0, "ERROR! ListenTCP: %v", err)
    os.Exit(1)
  }
  
  // Create channels for receiving events. 
  // The main() goroutine receives on all these channels 
  // and spawns new goroutines to handle the incoming events.
  tcp_connections := make(chan *net.TCPConn, 32)
  signals         := make(chan os.Signal, 32)
  
  signals_to_watch := []os.Signal{ syscall.SIGUSR1, syscall.SIGUSR2, syscall.SIGHUP }
  util.Log(1, "INFO! Intercepting these signals: %v", signals_to_watch)
  signal.Notify(signals, signals_to_watch...)
  
  util.Log(1, "INFO! Accepting connections on %v", tcp_addr);
  go acceptConnections(listener, tcp_connections)
  
  go message.Broadcast_new_server()

  /********************  main event loop ***********************/  
  for{ 
    select {
      case sig := <-signals : //os.Signal
                    util.Log(1, "Received signal \"%v\"", sig)
                    if sig == syscall.SIGUSR2 { 
                      go util.WithPanicHandler(message.Recreate_packages_db)
                    }
                    
      case conn:= <-tcp_connections : // *net.TCPConn
                    util.Log(1, "INFO! Incoming TCP request from %v", conn.RemoteAddr())
                    go util.WithPanicHandler(func(){handle_request(conn)})
    }
  }
}

// Accepts TCP connections on listener and sends them on the channel tcp_connections.
func acceptConnections(listener *net.TCPListener, tcp_connections chan<- *net.TCPConn) {
  for {
    tcpConn, err := listener.AcceptTCP()
    if err != nil { 
      util.Log(0, "ERROR! AcceptTCP: %v", err) 
    } else {
      tcp_connections <- tcpConn
    }
  }
}

// Handles one or more messages received over conn. Each message is a single
// line terminated by \n. The message may be encrypted as by message.GosaEncrypt().
func handle_request(conn *net.TCPConn) {
  defer conn.Close()
  defer util.Log(1, "INFO! Connection to %v closed", conn.RemoteAddr())
  
  var err error
  
  err = conn.SetKeepAlive(true)
  if err != nil {
    util.Log(0, "ERROR! SetKeepAlive: %v", err)
  }
  
  var buf = make([]byte, 65536)
  i := 0
  n := 1
  for n != 0 {
    util.Log(2, "DEBUG! Receiving from %v", conn.RemoteAddr())
    n, err = conn.Read(buf[i:])
    i += n
    
    if err != nil && err != io.EOF {
      util.Log(0, "ERROR! Read: %v", err)
    }
    if err == io.EOF {
      util.Log(2, "DEBUG! Connection closed by %v", conn.RemoteAddr())
      
    }
    if n == 0 && err == nil {
      util.Log(0, "ERROR! Read 0 bytes but no error reported")
    }
    
    if i == len(buf) {
      buf_new := make([]byte, len(buf)+65536)
      copy(buf_new, buf)
      buf = buf_new
    }

    // Find complete lines terminated by '\n' and process them.
    for start := 0;; {
      eol := bytes.IndexByte(buf[start:i], '\n')
      
      // no \n found, go back to reading from the connection
      // after purging the bytes processed so far
      if eol < 0 {
        copy(buf[0:], buf[start:i]) 
        i -= start
        break
      }
      
      // process the message and get a reply (if applicable)
      encrypted_message := strings.TrimSpace(string(buf[start:start+eol]))
      start += eol+1
      if encrypted_message != "" { // ignore empty lines
        reply, disconnect := message.ProcessEncryptedMessage(encrypted_message, conn.RemoteAddr().(*net.TCPAddr))
        
        if reply != "" {
          util.Log(2, "DEBUG! Sending reply to %v: %v", conn.RemoteAddr(), reply)
          util.SendLn(conn, reply, config.Timeout)
        }
        
        if disconnect {
          util.Log(1, "INFO! Forcing disconnect because of error")
          return
        }
      }
    }
  }
  
  if  i != 0 {
    util.Log(0, "ERROR! Incomplete message (i.e. not terminated by \"\\n\") of %v bytes: %v", i, buf[0:i])
  }
}

