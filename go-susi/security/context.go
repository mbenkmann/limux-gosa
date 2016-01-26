/*
Copyright (c) 2015 Matthias S. Benkmann

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


// Access controls, TLS, encryption, connection limits,...
package security

import (
         "fmt"
         "net"
         "math"
         "math/big"
         "sync"
         "time"
         "errors"
         "reflect"
         "strings"
         "crypto/tls"
         "crypto/x509"
         "encoding/asn1"
         
         "github.com/mbenkmann/golib/util"
         "../config"
       )

// Information related to a TCP connection that's used for security checks.
type Context struct {
  // true if the connection uses TLS
  TLS bool
  PeerID SubjectAltName
  Limits GosaConnectionLimits
  Access GosaAccessControl
}

// Information about a peer's identity. The name of the structure is
// derived from the subjectAltName extension in the TLS certificate that
// is related to most of this information, either by providing it or by
// being checked against it.
type SubjectAltName struct {
  // The IP of the other side of the connection.
  IP net.IP
  
  // List of IPs permitted to talk to us. Entries may end in one or more
  // 0-bytes which corresponds to whole subnets, e.g. 1.2.3.0 means 1.2.3.*
  AllowedIPs []net.IP
  
  // List of DNS names permitted to talk to us. These may be:
  // * name without domain: Reverse DNS of peer IP must produce a matching name.
  // * name with domain: Forward DNS of this name must produce peer IP, or
  //                     reverse DNS of peer IP must produce this name.
  // * "*" wildcard followed by domain (e.g. "*.foo.com"): reverse DNS of peer IP
  //       must produce a name that matches the pattern.
  AllowedNames []string
}

// Corresponds to the GosaConnectionLimits certificate extension.
// See its documentation for the meaning of the individual fields.
// NOTE: While these fields are optional in the certificate extension,
// they are always filled in when this struct is instantiated. Values
// not provided in the certificate will be filled in from defaults.
type GosaConnectionLimits struct {
  TotalTime time.Duration
  TotalBytes int64
  MessageBytes int64
  ConnPerHour int
  ConnParallel int
  MaxLogFiles int
  MaxAnswers int
  CommunicateWith []string
}

type GosaAccessControl struct {
  Misc GosaAccessMisc
  Query GosaAccessQuery
  Jobs GosaAccessJobs
  Incoming GosaAccessLDAPIncoming
  LDAPUpdate GosaAccessLDAPUpdate
  DetectedHW GosaAccessLDAPDetectedHardware
}

type GosaAccessMisc struct {
  Debug bool
  Wake bool
  Peer bool
}

type GosaAccessQuery struct {
  QueryAll bool
  QueryJobs bool
}

type GosaAccessJobs struct {
  JobsAll bool
  Lock bool
  Unlock bool
  Shutdown bool
  Wake bool
  Abort bool
  Install bool
  Update bool
  ModifyJobs bool
  NewSys bool
}

type GosaAccessLDAPIncoming []string

type GosaAccessLDAPUpdate struct {
  CN bool
  IP bool
  MAC bool
  DH bool
}

type GosaAccessLDAPDetectedHardware struct {
  Unprompted bool
  Template bool
  DN bool
  CN bool
  IPHostNumber bool
  MACAddress bool
}

func setLegacyDefaults(context *Context) {
  // everybody may connect
  context.PeerID.AllowedIPs = []net.IP{net.IPv4(0,0,0,0)}
  // no need for names, since AllowedIPs already allows everybody
  context.PeerID.AllowedNames = []string{}
  
  context.Limits.TotalTime = 0
  context.Limits.TotalBytes = math.MaxInt64
  context.Limits.MessageBytes = math.MaxInt64
  context.Limits.ConnPerHour = 36000 // 10 per second
  context.Limits.ConnParallel = 32
  context.Limits.MaxLogFiles = 64
  context.Limits.MaxAnswers = math.MaxInt32
  context.Limits.CommunicateWith = []string{ config.ServerSourceAddress }

  context.Access.Misc.Debug = true
  context.Access.Misc.Wake = true
  // if ip (or a name resolving to it) is listed in [ServerPackages]/address
  // set context.Access.Misc.Peer = true by default. The certificate value may
  // override this.
  context.Access.Misc.Peer = true
  
  // Because the default certificates are public, they can only have the
  // QueryJobs flag set in the GOsa certificate that is bound to localhost.
  // However peer_connection.go:SyncAll() queries the jobdb as part of
  // server-server communication. We want to enable server-server communication
  // with the default certificates (with servers explicitly listed in the
  // config file), so we set this flag by default for known peer servers.
  // This is only the default setting. An installation that uses its own
  // certificates instead of the default ones, may override this behaviour.
  /*context.Access.Query.QueryJobs = false
  peerIPStr := context.PeerID.IP.String()
  for _, known := range db.ServerAddresses() {
    knownip,_,_ := net.SplitHostPort(known)
    if knownip == peerIPStr {
      context.Access.Query.QueryJobs = true
      break
    }
  }*/
  context.Access.Query.QueryAll = true
  context.Access.Query.QueryJobs = true
  
  context.Access.Jobs.JobsAll = true
  context.Access.Jobs.Lock = true
  context.Access.Jobs.Unlock = true
  context.Access.Jobs.Shutdown = true
  context.Access.Jobs.Wake = true
  context.Access.Jobs.Abort = true
  context.Access.Jobs.Install = true
  context.Access.Jobs.Update = true
  context.Access.Jobs.ModifyJobs = true
  context.Access.Jobs.NewSys = true

  context.Access.Incoming = []string{config.LDAPURI+"/"+config.IncomingOU}
  
  context.Access.LDAPUpdate.CN = true
  context.Access.LDAPUpdate.IP = true
  context.Access.LDAPUpdate.MAC = true
  context.Access.LDAPUpdate.DH = true
  
  context.Access.DetectedHW.Unprompted = true
  context.Access.DetectedHW.Template = true
  context.Access.DetectedHW.DN = true
  context.Access.DetectedHW.CN = true
  context.Access.DetectedHW.IPHostNumber = true
  context.Access.DetectedHW.MACAddress = true
}

func setTLSDefaults(context *Context) {
  context.PeerID.AllowedIPs = []net.IP{}
  context.PeerID.AllowedNames = []string{}
  
  context.Limits.TotalTime = 0
  context.Limits.TotalBytes = math.MaxInt64
  context.Limits.MessageBytes = math.MaxInt64
  context.Limits.ConnPerHour = 36000 // 10 per second
  context.Limits.ConnParallel = 32
  context.Limits.MaxLogFiles = 64
  context.Limits.MaxAnswers = math.MaxInt32
  context.Limits.CommunicateWith = []string{ config.ServerSourceAddress }

  context.Access.Misc.Debug = false
  context.Access.Misc.Wake = false
  context.Access.Misc.Peer = false
  
  context.Access.Query.QueryAll = false
  context.Access.Query.QueryJobs = false
  
  context.Access.Jobs.JobsAll = false
  context.Access.Jobs.Lock = false
  context.Access.Jobs.Unlock = false
  context.Access.Jobs.Shutdown = false
  context.Access.Jobs.Wake = false
  context.Access.Jobs.Abort = false
  context.Access.Jobs.Install = false
  context.Access.Jobs.Update = false
  context.Access.Jobs.ModifyJobs = false
  context.Access.Jobs.NewSys = false

  context.Access.Incoming = []string{"ldap://*", "ldaps://*"}
  
  context.Access.LDAPUpdate.CN = false
  context.Access.LDAPUpdate.IP = false
  context.Access.LDAPUpdate.MAC = false
  context.Access.LDAPUpdate.DH = false
  
  context.Access.DetectedHW.Unprompted = false
  context.Access.DetectedHW.Template = false
  context.Access.DetectedHW.DN = false
  context.Access.DetectedHW.CN = false
  context.Access.DetectedHW.IPHostNumber = false
  context.Access.DetectedHW.MACAddress = false
}

// Returns a *security.Context for the provided connection.
// If conn is a *tls.Conn, the context will be filled in from the
// certificate presented by the peer. If conn is a *net.TCPConn,
// default values will be used.
// This function also performs some security checks. If one of them
// fails, an ERROR is logged and nil is returned.
func ContextFor(conn net.Conn) *Context {
  var context Context
  
  ip := conn.RemoteAddr().(*net.TCPAddr).IP
  if ip.IsLoopback() {
    context.PeerID.IP = net.ParseIP(config.IP)
  } else {
    context.PeerID.IP = make([]byte, len(ip))
    copy(context.PeerID.IP, ip)
  }
  
  // Defaults for non-TLS connections. Will be overwritten with TLS defaults
  // if this is a TLS connection.
  setLegacyDefaults(&context)

  if tlsconn, ok := conn.(*tls.Conn); ok {
    if !handle_tlsconn(tlsconn, &context) { return nil }
  }    
  
  if context.Verify() {
    return &context
  }
  
  return nil
}

func handle_tlsconn(conn *tls.Conn, context *Context) bool {
  conn.SetDeadline(time.Now().Add(config.TimeoutTLS))
  err := conn.Handshake()
  if err != nil {
    util.Log(0, "ERROR! [SECURITY] TLS Handshake: %v", err)
    return false
  }
  
  var no_deadline time.Time
  conn.SetDeadline(no_deadline)
  
  state := conn.ConnectionState()
  if len(state.PeerCertificates) == 0 {
    util.Log(0, "ERROR! [SECURITY] TLS peer has no certificate")
    return false
  }
  cert := state.PeerCertificates[0] // docs are unclear about this but I think leaf certificate is the first entry because that's as it is in tls.Certificate
  
  if util.LogLevel >= 2 { // because creating the dump is expensive
    util.Log(2, "DEBUG! [SECURITY] Peer certificate presented by %v:\n%v", conn.RemoteAddr(), CertificateInfo(cert))
  }
  
  for _, cacert := range config.CACert {
    err = cert.CheckSignatureFrom(cacert)
    if err == nil {
      if string(cacert.RawSubject) != string(cert.RawIssuer) {
        err = fmt.Errorf("Certificate was issued by wrong CA: \"%v\" instead of \"%v\"", cacert.Subject, cert.Issuer)
      } else {
        break // stop checking if we found a match for a CA. err == nil here!
      }
    }
  }
  
  if err != nil {
    util.Log(0, "ERROR! [SECURITY] TLS peer presented certificate not signed by trusted CA: %v", err)
    return false
  }
  
  setTLSDefaults(context)
  
  for _, e := range cert.Extensions {
    if len(e.Id) == 4 && e.Id[0] == 2 && e.Id[1] == 5 && e.Id[2] == 29 && e.Id[3] == 17 {
      parseSANExtension(e.Value, context)
    } else if len(e.Id) == 9 && e.Id[0] == 1 && e.Id[1] == 3 && e.Id[2] == 6 && e.Id[3] == 1 && e.Id[4] == 4 && e.Id[5] == 1 && e.Id[6] == 45753 && e.Id[7] == 1 {
      switch e.Id[8] {
        case 5: err = parseConnectionLimits(e.Value, context)
                if err != nil { util.Log(0, "ERROR! [SECURITY] GosaConnectionLimits: %v", err) }
        case 6: err = parseAccessControl(e.Value, context)
                if err != nil { util.Log(0, "ERROR! [SECURITY] GosaAccessControl: %v", err) }
      }
      
    }
  }
  
  context.TLS = true
  
  return true
}

var gosaGNMyServer = string([]byte{0x2B,0x06,0x01,0x04,0x01,0x82,0xE5,0x39,0x01,0x01})
var gosaGNConfigFile = string([]byte{0x2B,0x06,0x01,0x04,0x01,0x82,0xE5,0x39,0x01,0x02})
var gosaGNSRVRecord = string([]byte{0x2B,0x06,0x01,0x04,0x01,0x82,0xE5,0x39,0x01,0x03})
var gosaGNMyPeer = string([]byte{0x2B,0x06,0x01,0x04,0x01,0x82,0xE5,0x39,0x01,0x04})
const universal = 0
const context_specific = 2

// adapted from the function of the same name from crypto/x509/x509.go
func parseSANExtension(value []byte, context *Context) {
  // RFC 5280, 4.2.1.6

  // SubjectAltName ::= GeneralNames
  //
  // GeneralNames ::= SEQUENCE SIZE (1..MAX) OF GeneralName
  //
  // GeneralName ::= CHOICE {
  //      otherName                       [0]     OtherName,
  //      rfc822Name                      [1]     IA5String,
  //      dNSName                         [2]     IA5String,
  //      x400Address                     [3]     ORAddress,
  //      directoryName                   [4]     Name,
  //      ediPartyName                    [5]     EDIPartyName,
  //      uniformResourceIdentifier       [6]     IA5String,
  //      iPAddress                       [7]     OCTET STRING,
  //      registeredID                    [8]     OBJECT IDENTIFIER }
  var seq asn1.RawValue
  var rest []byte
  var err error
  if rest, err = asn1.Unmarshal(value, &seq); err != nil {
    return
  } else if len(rest) != 0 {
    return
  }
  if !seq.IsCompound || seq.Tag != 16 || seq.Class != universal {
    return
  }

  context.PeerID.AllowedNames = []string{}
  context.PeerID.AllowedIPs = []net.IP{}
 
  rest = seq.Bytes
  for len(rest) > 0 {
    var v asn1.RawValue
    rest, err = asn1.Unmarshal(rest, &v)
    if err != nil {
      return
    }
    switch v.Tag {
      case 2: // dNSName
              context.PeerID.AllowedNames = append(context.PeerID.AllowedNames, string(v.Bytes))
      case 7: // iPAddress
              switch len(v.Bytes) {
                case net.IPv4len, net.IPv6len:
                    context.PeerID.AllowedIPs = append(context.PeerID.AllowedIPs, net.IP(v.Bytes))
              }
      case 8: // registeredID
              oid := string(v.Bytes)
              switch oid {
                case gosaGNMyServer:
                         myServer = GetMyServer()
                         if !myServer.IsUnspecified() {
                           context.PeerID.AllowedIPs = append(context.PeerID.AllowedIPs, myServer)
                         }
                case gosaGNConfigFile:
                         context.PeerID.AllowedIPs = append(context.PeerID.AllowedIPs, config.ServerIPsFromConfigFile...)
                         context.PeerID.AllowedNames = append(context.PeerID.AllowedNames, config.ServerNamesFromConfigFile...)
                case gosaGNSRVRecord:
                         context.PeerID.AllowedNames = append(context.PeerID.AllowedNames, config.ServerNamesFromSRVRecords...)
                case gosaGNMyPeer:
                         // not implemented yet
              }
    }
  }

  return
}


func parseConnectionLimits(value []byte, context *Context) error {
  var seq asn1.RawValue
  var rest []byte
  var err error
  if rest, err = asn1.Unmarshal(value, &seq); err != nil {
    return err
  } else if len(rest) != 0 {
    return errors.New("Garbage after GosaConnectionLimits")
  }
  if !seq.IsCompound || seq.Tag != 16 || seq.Class != universal {
    return errors.New("GosaConnectionLimits has incorrect ASN.1 type")
  }

  rest = seq.Bytes
  for len(rest) > 0 {
    var v asn1.RawValue
    rest, err = asn1.Unmarshal(rest, &v)
    if err != nil {
      return err
    }
    if v.Class == context_specific {
      switch v.Tag {
        case 0: // totalTime  [0] INTEGER OPTIONAL
                n, err := asn1Int0(&v)
                if err != nil {
                  util.Log(0, "WARNING! [SECURITY] GosaConnectionLimits.TotalTime: %v", err)
                } else {
                  context.Limits.TotalTime = time.Millisecond * time.Duration(n)
                }
        case 1: // totalBytes  [1] INTEGER OPTIONAL
                n, err := asn1Int0(&v)
                if err != nil {
                  util.Log(0, "WARNING! [SECURITY] GosaConnectionLimits.TotalBytes: %v", err)
                } else {
                  context.Limits.TotalBytes = int64(n)
                }
        case 2: // messageBytes  [2] INTEGER OPTIONAL
                n, err := asn1Int0(&v)
                if err != nil {
                  util.Log(0, "WARNING! [SECURITY] GosaConnectionLimits.MessageBytes: %v", err)
                } else {
                  context.Limits.MessageBytes = int64(n)
                }
        case 3: // connPerHour  [3] INTEGER OPTIONAL
                n, err := asn1Int0(&v)
                if err != nil {
                  util.Log(0, "WARNING! [SECURITY] GosaConnectionLimits.ConnPerHour: %v", err)
                } else {
                  context.Limits.ConnPerHour = int(n)
                }
        case 4: // connParallel  [4] INTEGER OPTIONAL
                n, err := asn1Int0(&v)
                if err != nil {
                  util.Log(0, "WARNING! [SECURITY] GosaConnectionLimits.ConnParallel: %v", err)
                } else {
                  context.Limits.ConnParallel = int(n)
                }
        case 5: // maxLogFiles  [5] INTEGER OPTIONAL
                n, err := asn1Int0(&v)
                if err != nil {
                  util.Log(0, "WARNING! [SECURITY] GosaConnectionLimits.MaxLogFiles: %v", err)
                } else {
                  context.Limits.MaxLogFiles = int(n)
                }
        case 6: // maxAnswers  [6] INTEGER OPTIONAL
                n, err := asn1Int0(&v)
                if err != nil {
                  util.Log(0, "WARNING! [SECURITY] GosaConnectionLimits.MaxAnswers: %v", err)
                } else {
                  context.Limits.MaxAnswers = int(n)
                }
        case 7: // communicateWith  [7] SEQUENCE OF UTF8String OPTIONAL
                names, err := asn1SeqUtf8(&v)
                if err != nil {
                  util.Log(0, "WARNING! [SECURITY] GosaConnectionLimits.CommunicateWith: %v", err)
                } else {
                  context.Limits.CommunicateWith = names
                }
        default:
                util.Log(0, "WARNING! [SECURITY] GosaConnectionLimits contains data with unknown tag %v", v.Tag)
      }
    } else {
      util.Log(0, "WARNING! [SECURITY] GosaConnectionLimits contains data with strange tag (class not CONTEXT-SPECIFIC)")
    }
  }
  
  return nil
}

func parseAccessControl(value []byte, context *Context) error {
  var seq asn1.RawValue
  var rest []byte
  var err error
  if rest, err = asn1.Unmarshal(value, &seq); err != nil {
    return err
  } else if len(rest) != 0 {
    return errors.New("Garbage after GosaAccessControl")
  }
  if !seq.IsCompound || seq.Tag != 16 || seq.Class != universal {
    return errors.New("GosaAccessControl has incorrect ASN.1 type")
  }

  rest = seq.Bytes
  for len(rest) > 0 {
    var v asn1.RawValue
    rest, err = asn1.Unmarshal(rest, &v)
    if err != nil {
      return err
    }
    if v.Class == context_specific {
      switch v.Tag {
        case 0: // misc  [0] GosaAccessMisc  OPTIONAL
                asn1BitString("GosaAccessControl.Misc", &v, &context.Access.Misc)
        case 1: // query [1] GosaAccessQuery OPTIONAL
                asn1BitString("GosaAccessControl.Query", &v, &context.Access.Query)
        case 2: // jobs  [2] GosaAccessJobs  OPTIONAL
                asn1BitString("GosaAccessControl.Jobs", &v, &context.Access.Jobs)
        case 3: // incoming   [3] GosaAccessLDAPIncoming OPTIONAL
                names, err := asn1SeqUtf8(&v)
                if err != nil {
                  util.Log(0, "WARNING! [SECURITY] GosaAccessControl.Incoming: %v", err)
                } else {
                  context.Access.Incoming = names
                }
        case 4: // ldapUpdate [4] GosaAccessLDAPUpdate   OPTIONAL
                asn1BitString("GosaAccessControl.LDAPUpdate", &v, &context.Access.LDAPUpdate)
        case 5: // detectedHw [5] GosaAccessLDAPDetectedHardware OPTIONAL
                asn1BitString("GosaAccessControl.DetectedHW", &v, &context.Access.DetectedHW)
        default:
                util.Log(0, "WARNING! [SECURITY] GosaAccessControl contains data with unknown tag %v", v.Tag)
      }
    } else {
      util.Log(0, "WARNING! [SECURITY] GosaAccessControl contains data with strange tag (class not CONTEXT-SPECIFIC)")
    }
  }
  
  return nil
}

// Parses a BIT STRING in v and stores the bits in targetstruct which has to be
// a pointer to a struct containing only bool public fields.
// If targetstruct has an incorrect type this function will panic().
// If an ASN.1 parsing error occurs, it will be logged and the log message
// will refer to the field that causes the error by the name errorname.
func asn1BitString(errorname string, v *asn1.RawValue, targetstruct interface{}) {
  var bits asn1.BitString
  _, err := asn1.UnmarshalWithParams(v.FullBytes, &bits, fmt.Sprintf("tag:%d",v.Tag))
  if err != nil {
    util.Log(0, "WARNING! [SECURITY] %v: %v", errorname, err)
    return
  }
  
  target := reflect.ValueOf(targetstruct).Elem()
  n := target.NumField()
  for i := 0; i < n; i++ {
    target.Field(i).SetBool(bits.At(i)==1)
  }
}

var tooBig = big.NewInt(281474976710656)

// Parses an ASN.1 integer in v and converts all negative values and
// all values that exceed 281474976710656 to 0.
// If an error occurs during parsing, returns 0 and the error.
func asn1Int0(v *asn1.RawValue) (uint64, error) {
  i := big.NewInt(0)
  _, err := asn1.UnmarshalWithParams(v.FullBytes, &i, fmt.Sprintf("tag:%d",v.Tag))
  if err != nil {
    return 0, err
  }
  if i.Sign() <= 0 || i.Cmp(tooBig) >= 0 {
    return 0, nil
  }
  
  return uint64(i.Int64()), nil
}

// Parses an ASN.1 SEQUENCE OF UTF8String and returns it.
// If an error occurs during parsing, returns nil and the error.
func asn1SeqUtf8(v *asn1.RawValue) ([]string, error) {
  names := []string{}
  _, err := asn1.UnmarshalWithParams(v.FullBytes, &names, fmt.Sprintf("tag:%d",v.Tag))
  if err != nil {
    return nil, err
  }
  return names, nil
}


// Performs security checks on the context, in particular whether
// the 2 endpoints are allowed to communicate with each other.
// If a security check fails, an ERROR is logged and false is returned.
// If all security checks succeed, true is returned.
func (context *Context) Verify() bool {
  // Check if peer's IP matches at least on of the allowed IDs in SubjectAltName
  peerIP := context.PeerID.IP
  ok := false
  for _, ip := range context.PeerID.AllowedIPs {
    if ip.IsLoopback() {
      ip = net.ParseIP(config.IP)
    }
    if ip.Equal(peerIP) {
      ok = true
      break
    }
    if ip[len(ip)-1] == 0 { // if it is a wildcard address
      wildPeer := make(net.IP, len(peerIP))
      copy(wildPeer, peerIP)
      k := len(wildPeer)-1
      i := len(ip)-1
      for k >= 0 && i >= 0 && ip[i] == 0 {
        wildPeer[k] = 0
        i--
        k--
      }
      
      if ip.Equal(wildPeer) {
        ok = true
        break
      }
    }
  }
  
  if !ok { // peer not in AllowedIPs? Check AllowedNames (forward DNS)
    for _, name := range context.PeerID.AllowedNames {
      ips, err := net.LookupIP(name)
      if err != nil {
        // Only a DEBUG message because name may be a wildcard name (e.g. "*.foo.com")
        // or it may be a name in an internal subdomain that is not always available.
        util.Log(2, "DEBUG! LookupIP(%v) => %v", name, err)
      } else {
        for _, ip := range ips {
          if ip.Equal(peerIP) {
            ok = true
            break
          }
        }
        if ok { break }
      }
    }
  }
  
  if !ok { // peer not in AllowedIPs or AllowedNames (forward DNS)? Check AllowedNames (reverse DNS)
    peerNames, err := net.LookupAddr(peerIP.String())
    if err != nil {
      util.Log(0, "ERROR! LookupAddr(%v): %v", peerIP.String(), err)
    } else {
      util.Log(2, "DEBUG! Comparing peer names %v against names from certificate %v", peerNames, context.PeerID.AllowedNames)
      for _, name := range context.PeerID.AllowedNames {
        for _, peerName := range peerNames {
          peerName = strings.TrimRight(peerName, ".") // remove trailing "." returned by reverse lookup
          if strings.HasPrefix(name, "*") { // *.foo.de
            if strings.HasSuffix(peerName, name[1:]) {
              ok = true
              break
            }
          } else { // bar.foo.de
            if peerName == name {
              ok = true
              break
            }
          }
        }
        if ok { break }
      }
    }
  }
  
  if !ok {
    util.Log(0, "ERROR! [SECURITY] Certificate presented by %v is not valid for that IP (as determined by SubjectAltName extension)", peerIP)
    return false
  }

  // Check if peer's certificate is valid for talking to us
  ok = false
  fqname := config.Hostname + "." + config.Domain
  for _, comm := range context.Limits.CommunicateWith {
    idx := strings.Index(comm,":") // is there a port?
    if idx >= 0 { // if yes, make sure it's the same as ours
      if !strings.HasSuffix(config.ServerSourceAddress, comm[idx:]) {
        continue
      }
      comm = comm[0:idx] // cut off port
    }
    
    // At this point, comm does not contain a port
    
    // Check for exact match
    if comm == config.Hostname || comm == fqname || comm == config.IP {
      ok = true
      break
    }
    
    // Check for wildcard match
    if strings.HasPrefix(comm, "*") && strings.HasSuffix(fqname, comm[1:]) {
      ok = true
      break
    }
  }
  
  if !ok {
    util.Log(0, "ERROR! [SECURITY] Certificate presented by %v has GosaConnectionLimits extension with communicateWith that does not allow talking to me (%v, %v)", peerIP, config.ServerSourceAddress, fqname)
    return false
  }


  return true  
}

var myServer = net.IPv6unspecified
var myServer_mutex sync.Mutex

func GetMyServer() net.IP {
  myServer_mutex.Lock()
  defer myServer_mutex.Unlock()
  return myServer
}

// server may include a port but it will be ignored. server must be
// a numeric IP address.
func SetMyServer(server string) {
  myServer_mutex.Lock()
  defer myServer_mutex.Unlock()
  myServer = net.IPv6unspecified // in case something goes wrong
  host, _, err := net.SplitHostPort(server)
  if err != nil { host = server } // in case there is no port
  ip := net.ParseIP(host)
  if ip != nil {
    myServer = ip
  }
}

// Returns human-readable information about cert.
func CertificateInfo(cert *x509.Certificate) string {
  s := []string{}
  s = append(s, fmt.Sprintf("Version: %v\nSerial no.: %v\nIssuer: %v\nSubject: %v\nNotBefore: %v\nNotAfter: %v\n", cert.Version, cert.SerialNumber, cert.Issuer, cert.Subject, cert.NotBefore, cert.NotAfter))
  for k := range KeyUsageString {
    if cert.KeyUsage & k != 0 {
      s = append(s, fmt.Sprintf("KeyUsage: %v\n", KeyUsageString[k]))
    }
  }
  
  for _, ext := range cert.Extensions {
    s = append(s, fmt.Sprintf("Extension: %v critical=%v  % 02X\n", ext.Id, ext.Critical, ext.Value))
  }
  
  for _, ext := range cert.ExtKeyUsage {
    s = append(s, fmt.Sprintf("ExtKeyUsage: %v\n", ExtKeyUsageString[ext]))
  }
  
  for _, ext := range cert.UnknownExtKeyUsage {
    s = append(s, fmt.Sprintf("ExtKeyUsage: %v\n", ext))
  }
  
  if cert.BasicConstraintsValid {
    s = append(s, fmt.Sprintf("IsCA: %v\n", cert.IsCA))
    if cert.MaxPathLen > 0 {
      s = append(s, fmt.Sprintf("MaxPathLen: %v\n", cert.MaxPathLen))
    }
  }
  
  s = append(s, fmt.Sprintf("SubjectKeyId: % 02X\nAuthorityKeyId: % 02X\n", cert.SubjectKeyId, cert.AuthorityKeyId))
  s = append(s, fmt.Sprintf("OCSPServer: %v\nIssuingCertificateURL: %v\n", cert.OCSPServer, cert.IssuingCertificateURL))
  
  oids := []string{}
  for _, e := range cert.Extensions {
    if len(e.Id) == 4 && e.Id[0] == 2 && e.Id[1] == 5 && e.Id[2] == 29 && e.Id[3] == 17 {
      var seq asn1.RawValue
      var rest []byte
      var err error
      if rest, err = asn1.Unmarshal(e.Value, &seq); err != nil {
        continue
      } else if len(rest) != 0 {
        continue
      }
      if !seq.IsCompound || seq.Tag != 16 || seq.Class != 0 {
        continue
      }

      rest = seq.Bytes
      for len(rest) > 0 {
        var v asn1.RawValue
        rest, err = asn1.Unmarshal(rest, &v)
        if err != nil {
          break
        }
        if v.Tag == 8 { // registeredID
          var oid asn1.ObjectIdentifier
          _, err = asn1.Unmarshal(v.FullBytes, &oid)
          if err != nil {
            oids = append(oids, oid.String())
          }
        }
      }
    }
  }
  
  s = append(s, fmt.Sprintf("DNSNames: %v\nEmailAddresses: %v\nIPAddresses: %v\nRegisteredIDs: %v\n", cert.DNSNames, cert.EmailAddresses, cert.IPAddresses, oids))
  
  s = append(s, fmt.Sprintf("PermittedDNSDomainsCritical: %v\nPermittedDNSDomains: %v\n", cert.PermittedDNSDomainsCritical, cert.PermittedDNSDomains))
  
  s = append(s, fmt.Sprintf("CRLDistributionPoints: %v\nPolicyIdentifiers: %v\n", cert.CRLDistributionPoints, cert.PolicyIdentifiers))
  return strings.Join(s, "")
}

// Maps an x509.KeyUsage to a human-readable string.
var KeyUsageString = map[x509.KeyUsage]string{
  x509.KeyUsageDigitalSignature:  "digital signature",
  x509.KeyUsageContentCommitment: "content commitment",
  x509.KeyUsageKeyEncipherment:   "key encipherment",
  x509.KeyUsageDataEncipherment:  "data encipherment",
  x509.KeyUsageKeyAgreement:      "key agreement",
  x509.KeyUsageCertSign:          "certificate signing",
  x509.KeyUsageCRLSign:           "CRL signing",
  x509.KeyUsageEncipherOnly:      "encipherment ONLY",
  x509.KeyUsageDecipherOnly:      "decipherment ONLY",
}

// Maps an x509.ExtKeyUsage to a human-readable string.
var ExtKeyUsageString = map[x509.ExtKeyUsage]string {
  x509.ExtKeyUsageAny:        "any",
  x509.ExtKeyUsageServerAuth: "server authentication",
  x509.ExtKeyUsageClientAuth: "client authentication",
  x509.ExtKeyUsageCodeSigning: "code signing",
  x509.ExtKeyUsageEmailProtection: "email protection",
  x509.ExtKeyUsageIPSECEndSystem: "IPSEC end system",
  x509.ExtKeyUsageIPSECTunnel: "IPSEC tunnel",
  x509.ExtKeyUsageIPSECUser: "IPSEC user",
  x509.ExtKeyUsageTimeStamping: "time stamping",
  x509.ExtKeyUsageOCSPSigning: "OCSP signing",
  x509.ExtKeyUsageMicrosoftServerGatedCrypto: "MicrosoftServerGatedCrypto",
  x509.ExtKeyUsageNetscapeServerGatedCrypto: "NetscapeServerGatedCrypto",
}
