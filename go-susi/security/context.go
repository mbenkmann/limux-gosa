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
         "net"
         "math"
         "time"
         
         "../config"
       )

// Information related to a TCP connection that's used for security checks.
type Context struct {
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
  QueryJobs bool
}

type GosaAccessJobs struct {
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

// Returns a *security.Context for the provided connection.
// If conn is a *tls.Conn, the context will be filled in from the
// certificate presented by the peer. If conn is a *net.TCPConn,
// default values will be used.
func ContextFor(conn net.Conn) *Context {
  var context Context
  
  ip := conn.RemoteAddr().(*net.TCPAddr).IP
  context.PeerID.IP = make([]byte, len(ip))
  copy(context.PeerID.IP, ip)
  
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
  context.Access.Misc.Peer = true
  // if ip (or a name resolving to it) is listed in [ServerPackages]/address
  // context.Access.Misc.Peer = true
  
  context.Access.Query.QueryJobs = true
  
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
  
  
  return &context
}
