package main

import (
         "io"
         "os"
         "fmt"
         "net"
         "time"
         "strings"
         "strconv"
         "crypto/tls"
         "crypto/x509"
         "github.com/mbenkmann/golib/argv"
       )

const (
  UNKNOWN = iota
  HELP
  SEND
  RECV
  CERT
  KEY
  STARTTLS
)

const DISABLED = 0
const ENABLED = 1

var usage = argv.Usage{
{ UNKNOWN, 1, "", "",        argv.ArgUnknown, "USAGE: trickler [options] host:port\n\n" +
                                              "Options:" },
{ HELP,    1, "", "help",    argv.ArgNone,     "  \t--help  \tPrint usage and exit." },
{ SEND,    1, "s", "send",   argv.ArgNonEmpty, "  \t--send count:block:delay  \v-s count:block:delay  \tWhen sending, after block bytes have been sent, insert delay milliseconds wait time. Repeat this until count bytes have been sent. Then go to the next --send rule if there is one or repeat the last one. " },
{ RECV,    1, "r", "recv",   argv.ArgNonEmpty, "  \t--recv count:block:delay  \v-r count:block:delay  \tWhen receiving, after block bytes have been read, insert delay milliseconds wait time. Repeat this until count bytes have been read. Then go to the next --recv rule if there is one or repeat the last one. " },
{ CERT,    1, "", "certificate",   argv.ArgNonEmpty, "  \t--certificate path  \tUse TLS client authentication with the certificate located at path. The server's certificate will not be checked." },
{ KEY,     1, "", "key",   argv.ArgNonEmpty, "  \t--key path  \tThe path to the key corresponding to --certificate." },
{ STARTTLS, 1, "", "starttls",   argv.ArgNone, "  \t--starttls  \tSend \"STARTTLS\\n\" before TLS handshake." },
{ UNKNOWN, 1, "", "",        argv.ArgUnknown,
`
Trickler opens a connection to host:port and copies between the remote host and stdin/stdout. The --recv and --send rules allow insertion of delays into the receiving and sending side independently. Note that --send and --recv rules are already in effect for the TLS handshake. You can specify a count of 0 to mean "until the end of TLS negotiation".
` },
}

func main() {
  options, nonoptions, err, _ := argv.Parse(os.Args[1:], usage, "gnu -perl --abb")
  if err != nil {
    fmt.Fprintf(os.Stderr, "%v\n", err)
    os.Exit(1)
  }

  if (options[HELP].Is(ENABLED) || len(os.Args) == 1 || len(nonoptions) != 1) {
    fmt.Fprintf(os.Stdout, "%v\n", usage)
    os.Exit(0)
  }

  conn, err := net.Dial("tcp", nonoptions[0])
  if err != nil {
    fmt.Fprintf(os.Stderr, "%v\n", err)
    os.Exit(1)
  }
  
  w := &trickleParam{1000000000,1000000000,1000000000,0,nil}
  cur := w
  mincount := 0
  
  for o := options[SEND]; o != nil; o = o.Next() {
    parm := strings.Split(o.Arg, ":")
    if len(parm) != 3 {
      fmt.Fprintf(os.Stderr, "Argument \"%v\" does not have format \"count:block:delay\"\n", o.Arg)
      os.Exit(1)
    }
    
    count, err := strconv.Atoi(parm[0])
    if err != nil || count < mincount {
      fmt.Fprintf(os.Stderr, "Illegal value for count: \"%v\"\n", parm[0])
      os.Exit(1)
    }
    cur.remaining = count
    if count == 0 {
      mincount = 1 // only 1 entry with count == 0 allowed
    } else {
      cur.blockremaining = count
    }
    
    block, err := strconv.Atoi(parm[1])
    if err != nil || block <= 0 {
      fmt.Fprintf(os.Stderr, "Illegal value for block: \"%v\"\n", parm[1])
      os.Exit(1)
    }
    cur.blocksize = block
    if block < cur.blockremaining {
      cur.blockremaining = block
    }
    
    delay, err := strconv.Atoi(parm[2])
    if err != nil || delay < 0 {
      fmt.Fprintf(os.Stderr, "Illegal value for delay: \"%v\"\n", parm[2])
      os.Exit(1)
    }
    cur.delay = time.Millisecond * time.Duration(delay)
    
    cur.next = &trickleParam{1000000000,cur.blocksize,cur.blocksize,cur.delay,nil}
    cur = cur.next
  }
  
  r := &trickleParam{1000000000,1000000000,1000000000,0,nil}
  cur = r
  mincount = 0
  
  for o := options[RECV]; o != nil; o = o.Next() {
    parm := strings.Split(o.Arg, ":")
    if len(parm) != 3 {
      fmt.Fprintf(os.Stderr, "Argument \"%v\" does not have format \"count:block:delay\"\n", o.Arg)
      os.Exit(1)
    }
    
    count, err := strconv.Atoi(parm[0])
    if err != nil || count < mincount {
      fmt.Fprintf(os.Stderr, "Illegal value for count: \"%v\"\n", parm[0])
      os.Exit(1)
    }
    cur.remaining = count
    if count == 0 {
      mincount = 1 // only 1 entry with count == 0 allowed
    } else {
      cur.blockremaining = count
    }
    
    block, err := strconv.Atoi(parm[1])
    if err != nil || block <= 0 {
      fmt.Fprintf(os.Stderr, "Illegal value for block: \"%v\"\n", parm[1])
      os.Exit(1)
    }
    cur.blocksize = block
    if block < cur.blockremaining {
      cur.blockremaining = block
    }
    
    delay, err := strconv.Atoi(parm[2])
    if err != nil || delay < 0 {
      fmt.Fprintf(os.Stderr, "Illegal value for delay: \"%v\"\n", parm[2])
      os.Exit(1)
    }
    cur.delay = time.Millisecond * time.Duration(delay)
    
    cur.next = &trickleParam{1000000000,cur.blocksize,cur.blocksize,cur.delay,nil}
    cur = cur.next
  }

  tconn := &TrickleConn{conn,r,w,false}
  conn = tconn
  
  if options[STARTTLS].Is(ENABLED) {
    _, err := conn.Write([]byte{'S','T','A','R','T','T','L','S','\n'})
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v\n", err)
      os.Exit(1)
    }
  }
  
  if options[CERT].Is(ENABLED) {
    tlscert, err := tls.LoadX509KeyPair(options[CERT].Arg, options[KEY].Arg)
    if err == nil {
      tlscert.Leaf, err = x509.ParseCertificate(tlscert.Certificate[0])
    }
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v\n", err)
      os.Exit(1)
    }
    
    TLSClientConfig := &tls.Config{Certificates:[]tls.Certificate{tlscert},
                                  RootCAs:nil,
                                  NextProtos:[]string{},
                                  ClientAuth:tls.RequireAnyClientCert,
                                  ClientCAs:nil,
                                  ServerName:"",
                                  SessionTicketsDisabled:true,
                                  InsecureSkipVerify:true,
                                  }
    conn = tls.Client(conn, TLSClientConfig)
    err = conn.(*tls.Conn).Handshake()
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v\n", err)
      os.Exit(1)
    }
  }
  
  tconn.tlsdone = true
  
  go func(){
    _, err := io.Copy(conn, os.Stdin)
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v\n", err)
      os.Exit(1)
    }
    time.Sleep(3*time.Second)
    os.Exit(0)
  }()
  
  _, err = io.Copy(os.Stdout, conn)
  if err != nil {
    fmt.Fprintf(os.Stderr, "%v\n", err)
    os.Exit(1)
  }
  os.Exit(0)
}

type trickleParam struct {
  remaining int
  blocksize int
  blockremaining int
  delay time.Duration
  next *trickleParam
}

func (t *trickleParam) reset() {
  t.blockremaining = t.blocksize
  
  if t.remaining > 0 && t.blockremaining > t.remaining {
    t.blockremaining = t.remaining
  }
}

type TrickleConn struct {
  conn net.Conn
  r *trickleParam
  w *trickleParam
  tlsdone bool
}

func (t *TrickleConn) Read(b []byte) (n int, err error) {
  if t.tlsdone && t.r.remaining <= 0 {
    t.r = t.r.next
    t.r.reset()
  }
  
  if len(b) > t.r.blockremaining {
    b = b[:t.r.blockremaining]
  } 
  
  n, err = t.conn.Read(b)
  if err != nil {
    return n, err
  }
  
  t.r.blockremaining -= n
  t.r.remaining -= n // if t.r.remaining started as 0 (standing for until end of TLS handshake), this goes negative
  
  if t.r.blockremaining == 0 {
    time.Sleep(t.r.delay)
  
    if t.r.remaining == 0 {
      t.r = t.r.next
    }

    t.r.reset()
  }
  
  return n, err
}

func (t *TrickleConn) Write(b []byte) (n int, err error) {
  if t.tlsdone && t.w.remaining <= 0 {
    t.w = t.w.next
    t.w.reset()
  }

  if len(b) > t.w.blockremaining {
    b = b[:t.w.blockremaining]
  } 
  
  n, err = t.conn.Write(b)
  if err != nil {
    return n, err
  }
  
  t.w.blockremaining -= n
  t.w.remaining -= n // if t.w.remaining started as 0 (standing for until end of TLS handshake), this goes negative
  
  if t.w.blockremaining == 0 {
    time.Sleep(t.w.delay)
    
    if t.w.remaining == 0 {
      t.w = t.w.next
    }
    t.w.reset()
  }
  
  return n, err
}

func (t *TrickleConn) Close() error {
  return t.conn.Close()
}

func (t *TrickleConn) LocalAddr() net.Addr {
  return t.conn.LocalAddr()
}

func (t *TrickleConn) RemoteAddr() net.Addr {
  return t.conn.RemoteAddr()
}

func (tr *TrickleConn) SetDeadline(t time.Time) error {
  return tr.conn.SetDeadline(t)
}

func (tr *TrickleConn) SetReadDeadline(t time.Time) error {
  return tr.conn.SetReadDeadline(t)
}

func (tr *TrickleConn) SetWriteDeadline(t time.Time) error {
  return tr.conn.SetWriteDeadline(t)
}

