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

// Unit tests run by run-tests.go.
package tests

import (
         "os"
         "fmt"
         "net"
         "time"
         "crypto/tls"

         "../security"
         "../config"
         
         "github.com/mbenkmann/golib/util"
       )


// Unit tests for the package security.
func Security_test() {
  fmt.Printf("\n==== security ===\n\n")

  config.CACertPath = "testdata/certs/ca.cert"
  
  // do not spam console with expected errors but do
  // store them in the log file (if any is configured)
  util.LoggerRemove(os.Stderr)
  defer util.LoggerAdd(os.Stderr)
  
  cli, srv := tlsTest("1", "1")
  check(cli!=nil, true)
  check(srv!=nil, true)
  
  cli, srv = tlsTest("1", "2")
  check(cli!=nil, true)
  check(srv!=nil, true)

  cli, srv = tlsTest("nocert", "1")
  check(cli, nil)
  check(srv, nil)
  
  cli, srv = tlsTest("signedbywrongca", "1")
  check(cli!=nil, true)
  check(srv, nil)
  
  cli, srv = tlsTest("1","signedbywrongca")
  check(cli, nil)
  check(srv!=nil, true)
  
  cli, srv = tlsTest("local", "2")
  check(cli!=nil, true)
  check(srv!=nil, true)
  
  cli, srv = tlsTest("badip", "2")
  check(cli!=nil, true)
  check(srv, nil)
  
  cli, srv = tlsTest("badname", "2")
  check(cli!=nil, true)
  check(srv, nil)
  
  cli, srv = tlsTest("localname", "2")
  check(cli!=nil, true)
  check(srv!=nil, true)
  
  security.SetMyServer("8.8.8.8")
  cli, srv = tlsTest("myserver", "2")
  check(cli!=nil, true)
  check(srv, nil)
  
  security.SetMyServer("127.0.0.1")
  cli, srv = tlsTest("myserver", "2")
  check(cli!=nil, true)
  check(srv!=nil, true)
}

func tlsTest(client, server string) (*security.Context, *security.Context) {
  config.CertPath = "testdata/certs/" + server + ".cert"
  config.CertKeyPath = "testdata/certs/" + server + ".key"
  config.ReadCertificates()
  server_conf := config.TLSServerConfig
  
  client_conf := config.TLSClientConfig
  if client == "nocert" {
    client_conf.Certificates = nil
  } else { 
    config.CertPath = "testdata/certs/" + client + ".cert"
    config.CertKeyPath = "testdata/certs/" + client + ".key"
    config.ReadCertificates()
    client_conf = config.TLSClientConfig
  }
  
  c1 := make(chan *security.Context)
  c2 := make(chan *security.Context)
  
  go func() {
    tcp_addr, err := net.ResolveTCPAddr("tcp4", "127.0.0.1:18746")
    if err != nil { panic(err) }
    listener, err := net.ListenTCP("tcp4", tcp_addr)
    if err != nil { panic(err) }
    defer listener.Close()
    tcpConn, err := listener.AcceptTCP()
    if err != nil { panic(err) }
    defer tcpConn.Close()
    buf := []byte{'S','T','A','R','T','T','L','S','\n'}
    tcpConn.Read(buf)
    conn := tls.Server(tcpConn, server_conf)
    c2 <- security.ContextFor(conn)
  }()
  
  go func() {
    time.Sleep(1*time.Second)
    tcpConn, err := net.Dial("tcp4", "127.0.0.1:18746")
    if err != nil { panic(err) }
    defer tcpConn.Close()
    buf := []byte{'S','T','A','R','T','T','L','S','\n'}
    tcpConn.Write(buf)
    conn := tls.Client(tcpConn, client_conf)
    c1 <- security.ContextFor(conn)
  }()
  
  
  return  <-c1, <-c2
}
