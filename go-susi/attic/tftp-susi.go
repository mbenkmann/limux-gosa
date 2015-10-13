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
*/

// Standalone TFTP server for pxelinux-related files. Also includes FAI monitor.
package main

import (
          "io"
          "os"
          "fmt"
          "log"
          "net"
          "path"
          
          "../db"
          "github.com/mbenkmann/golib/util"
          "../tftp"
          "github.com/mbenkmann/golib/bytes"
          "../config"
       )

func main() {
  config.ReadArgs(os.Args[1:])
  
  if config.PrintVersion {
    fmt.Printf(`go-susi %v (revision %v)
Copyright (c) 2013 Matthias S. Benkmann
This is free software; see the source for copying conditions.  There is NO
warranty; not even for MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.

`, config.Version, config.Revision)
  }
  
  if config.PrintHelp {
    fmt.Println(`USAGE: tftp [args]

--help       print this text and exit
--version    print version and exit

-v           print operator debug messages (INFO)
-vv          print developer debug messages (DEBUG)
             ATTENTION! developer messages include keys!

-c <file>    read config from <file> instead of default location
`)
  }
  
  if config.PrintVersion || config.PrintHelp { os.Exit(0) }
  
  config.ReadConfig()
  
  logdir,_ := path.Split(config.LogFilePath)
  
  logfile, err := os.OpenFile(logdir+"go-susi-tftp.log", os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
  if err != nil {
    util.Log(0, "ERROR! %v", err)
    // Do not exit. We can go on without logging to a file.
  
  } else {
    // Send log output to both stderr AND the log file
    logfile.Close() // will be re-opened on the first write
    util.Logger = log.New(io.MultiWriter( os.Stderr, util.LogFile(logfile.Name())), "",0)
  }
  util.LogLevel = config.LogLevel
  
  config.ReadNetwork() // after config.ReadConfig()
  setConfigUnitTag() // after config.ReadNetwork()
  config.FAIBase = db.LDAPFAIBase()
  util.Log(1, "INFO! FAI base: %v", config.FAIBase)
  
  util.Log(1, "INFO! Accepting FAI monitoring messages on %v", config.FAIMonPort)
  go faimon(":"+config.FAIMonPort)
  
  util.Log(1, "INFO! Accepting TFTP requests on %v", config.TFTPPort)
  tftp.ListenAndServe(":"+config.TFTPPort, config.TFTPFiles, config.PXELinuxCfgHookPath)
}

func setConfigUnitTag() {
  util.Log(1, "INFO! Getting my own system's gosaUnitTag from LDAP")
  config.UnitTag = db.SystemGetState(config.MAC, "gosaUnitTag")
  if config.UnitTag == "" {
    util.Log(1, "INFO! No gosaUnitTag found for %v => gosaUnitTag support disabled", config.MAC)
  } else {
    config.UnitTagFilter = "(gosaUnitTag="+config.UnitTag+")"
    config.AdminBase, config.Department = db.LDAPAdminBase()
    util.Log(1, "INFO! gosaUnitTag: %v  Admin base: %v  Department: %v", config.UnitTag, config.AdminBase, config.Department)
  }
}

func faimon(listen_address string) {
  listener, err := net.Listen("tcp", listen_address)
  if err != nil {
    util.Log(0, "ERROR! Cannot start FAI monitor: %v", err)
    return
  }
  
  for {
    conn, err := listener.Accept()
    if err != nil { 
      util.Log(0,"ERROR! FAI monitor error: %v", err)
      continue
    }
    
    go util.WithPanicHandler(func(){faiConnection(conn.(*net.TCPConn))})
  }
}

func faiConnection(conn *net.TCPConn) {
  defer conn.Close()
  var err error
  
  err = conn.SetKeepAlive(true)
  if err != nil {
    util.Log(0, "ERROR! SetKeepAlive: %v", err)
  }
  
  var buf bytes.Buffer
  defer buf.Reset()
  readbuf := make([]byte, 4096)
  n := 1
  for n != 0 {
    n, err = conn.Read(readbuf)
    if err != nil && err != io.EOF {
      util.Log(0, "ERROR! Read: %v", err)
    }
    if n == 0 && err == nil {
      util.Log(0, "ERROR! Read 0 bytes but no error reported")
    }
    
    // Find complete lines terminated by '\n' and process them.
    for start := 0;; {
      eol := start
      for ; eol < n; eol++ {
        if readbuf[eol] == '\n' { break }
      }
      
      // no \n found, append to buf and continue reading
      if eol == n {
        buf.Write(readbuf[start:n])
        break
      }
      
      // append to rest of line to buffered contents
      buf.Write(readbuf[start:eol])
      start = eol+1
      
      buf.TrimSpace()
      
      util.Log(2, "DEBUG! FAI monitor message from %v: %v", conn.RemoteAddr(), buf.String())
      buf.Reset()
    }
  }
  
  if  buf.Len() != 0 {
    util.Log(2, "DEBUG! Incomplete FAI monitor message (i.e. not terminated by \"\\n\") from %v: %v", conn.RemoteAddr(), buf.String())
  }
}
