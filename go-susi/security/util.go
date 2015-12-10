/*
Copyright (c) 2015 Landeshauptstadt München
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


package security

import (
         "net"
         "crypto/tls"
         "time"
         
         "github.com/mbenkmann/golib/util"
         "../config"
       )

var starttls = []byte{'S','T','A','R','T','T','L','S','\n'}

// Opens a connection to target (e.g. "foo.example.com:20081"),
// sends msg followed by \r\n.
// If keep_open == false, the connection is closed, otherwise it is
// returned together with the corresponding security.Context.
// The connection will be secured according to
// the config settings. If a certificate is configured, the connection
// will use TLS (and the key argument will be ignored). Otherwise, key
// will be used to GosaEncrypt() the message before sending it over
// a non-TLS connection.
// If an error occurs, it is logged and nil is returned even if keep_open.
func SendLnTo(target, msg, key string, keep_open bool) (net.Conn, *Context) {
  conn, err := net.Dial("tcp", target)
  if err != nil {
    util.Log(0, "ERROR! Could not connect to %v: %v\n", target, err)
    return nil, nil
  }
  if !keep_open {
    defer conn.Close()
  }
  
  // enable keep alive to avoid connections hanging forever in case of routing issues etc.
  err = conn.(*net.TCPConn).SetKeepAlive(true)
  if err != nil {
    util.Log(0, "ERROR! SetKeepAlive: %v", err)
    // This is not fatal => Don't abort send attempt
  }
  
  if config.TLSClientConfig != nil {
    conn.SetDeadline(time.Now().Add(config.TimeoutTLS)) // don't allow stalling on STARTTLS
    
    _, err = util.WriteAll(conn, starttls)
    if err != nil {
      util.Log(0, "ERROR! [SECURITY] Could not send STARTTLS to %v: %v\n", target, err)
      conn.Close() // even if keep_open
      return nil, nil
    }

    var no_deadline time.Time
    conn.SetDeadline(no_deadline)
    
    conn = tls.Client(conn, config.TLSClientConfig)

  } else {
    msg = GosaEncrypt(msg, key)
  }
  
  context := ContextFor(conn)
  if context == nil { 
    conn.Close() // even if keep_open
    return nil, nil
  }

  err = util.SendLn(conn, msg, config.Timeout)
  if err != nil {
    util.Log(0, "ERROR! [SECURITY] While sending message to %v: %v\n", target, err)
    conn.Close() // even if keep_open
    return nil, nil
  }

  if keep_open {
    return conn, context
  }
  
  return nil, nil
}
