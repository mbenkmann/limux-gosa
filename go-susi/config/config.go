/*
Copyright (c) 2012 Landeshauptstadt München
Author: Matthias S. Benkmann

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

// Manages all user-configurable flags and variables.
package config

import (
         "io"
         "io/ioutil"
         "os"
         "net"
         "fmt"
         "time"
         "bufio"
         "regexp"
         "strings"
         "strconv"
         "crypto/aes"
         "crypto/tls"
         "crypto/x509"
         "encoding/pem"
         
         "github.com/mbenkmann/golib/util"
         "github.com/mbenkmann/golib/deque"
       )

// The initialization vector for the AES encryption of GOsa messages.
var InitializationVector = []byte(util.Md5sum("GONICUS GmbH")[0:aes.BlockSize])

// The keys used to address different gosa-si modules.
var ModuleKeys = []string{"dummy-key"}

// Maps a module name surrounded by brackets (such as "[ServerPackages]") to its key.
var ModuleKey = map[string]string{}

// The address to listen on. "127.0.0.1:<port>" listens only for connections from
// the local machine. ":<port>" allows connections from anywhere.
var ServerListenAddress = ":20081"

// If this is true, go-susi will offer server functionality in addition
// to client functionality. If false, only client functionality will
// be available.
var RunServer bool = true

// The address sent in the <source> element.
var ServerSourceAddress = "127.0.0.1:20081"

// Where to send log messages (in addition to stderr).
var LogFilePath = "/var/log/go-susi.log"

// Path of the server config file.
var ServerConfigPath = "/etc/gosa-si/server.conf"

// Path of the client config file.
var ClientConfigPath = "/etc/gosa-si/client.conf"

// Path to a file containing XML data to be added to here_i_am messages.
// If empty, no extra info will be added to here_i_am.
var ExtraInfoFilePath = ""

// Path(s) of the CA certificate(s) used to authenticate all other certificates.
var CACertPath = []string{"/etc/gosa-si/ca.cert"}

// The parsed CA certificate(s).
var CACert []*x509.Certificate

// Path of the certificate go-susi will present to the other party when
// connecting with TLS.
var CertPath = "/etc/gosa-si/si.cert"

// Path of the key corresponding to the certificate in CertPath.
var CertKeyPath = "/etc/gosa-si/si.key"

// Path to config file with additional DNs of OUs for finding servers.
var ServersOUConfigPath = "/etc/gosa/ou=servers.conf"

// Path to database of scheduled jobs.
var JobDBPath = "/var/lib/go-susi/jobdb.xml"

// Path to database of peer servers.
var ServerDBPath = "/var/lib/go-susi/serverdb.xml"

// Path to database of clients (foreign and our own).
var ClientDBPath = "/var/lib/go-susi/clientdb.xml"

// Directory where package-list-hook should store its cache.
var PackageCacheDir = "/var/lib/go-susi"

// Called by db.HooksExecute() to generate the kernel db.
var KernelListHookPath = "/usr/lib/go-susi/generate_kernel_list"

// Called by db.HooksExecute() to generate the packages db.
var PackageListHookPath = "/usr/lib/go-susi/generate_package_list"

// Called when a job_send_user_msg job is executed.
var UserMessageHookPath = "/usr/lib/go-susi/send_user_msg"

// Called whenever a new_foo_config message is received.
var NewConfigHookPath = "/usr/lib/go-susi/update_config_files"

// Called whenever a trigger_action_foo message is received.
var TriggerActionHookPath = "/usr/lib/go-susi/trigger_action"

// Called when a registered message is received that is part of a
// successful registration. The hook will not be called for
// spurious registered messages.
var RegisteredHookPath = "/usr/lib/go-susi/registered"

// Called when a set_activated_for_installation message is received.
var ActivatedHookPath = "/usr/lib/go-susi/activated"

// Called when a detect_hardware message is received.
// Writes to its standard output the system's hardware configuration
// in LDIF format.
var DetectHardwareHookPath = "/usr/lib/go-susi/detect_hardware"

// Path to a hook whose output will be read and converted to
// CLMSG_* messages.
var FAIProgressHookPath = "/usr/lib/go-susi/fai_progress"

// Path to a hook called when "TASKEND savelog" is seen in the output
// from the FAIProgressHookPath program. The output from the hook is
// sent to the server as CLMSG_save_fai_log message.
var FAISavelogHookPath = "/usr/lib/go-susi/fai_savelog"

// Path to a hook called when "TASKEND audit" is seen in the output
// from the FAIProgressHookPath program. The output from the hook is
// sent to the server as CLMSG_save_fai_log message.
var FAIAuditHookPath = "/usr/lib/go-susi/fai_audit"

// Path where log files from CLMSG_save_fai_log are stored.
// Within this directory go-susi creates sub-directories named
// after the clients' MAC addresses and symlinks named after the
// clients' plain names.
var FAILogPath = "/var/log/fai"

// Port for accepting FAI status updates sent via /usr/lib/fai/subroutines:sendmon()
var FAIMonPort = "disabled"

// UDP Port for receiving TFTP requests
var TFTPPort = "69"

// Potential ports for clients. This list is used for 2 purposes:
//   1) to distinguish between standard clients and test clients
//   2) to attempt contact with a client with known IP but unknown port 
// NOTE: The server port is appended to this list by ReadConfig().
var ClientPorts = []string{"20083"}

// TFTPRegexes and TFTPReplies are lists of equal length.
// They correspond to the request_re and reply arguments of
// tftp.ListenAndServe(). See there for a detailed explanation.
var TFTPRegexes = []*regexp.Regexp{}
var TFTPReplies = []string{}

// Temporary directory only accessible by the user running go-susi.
// Used e.g. for storing password files. Deleted in config.Shutdown().
var TempDir = ""

// host:port addresses of peer servers read from config file.
var PeerServers = []string{}

// IPs of all servers listed with a numeric IP address in
// [server]/ip or [ServerPackages]/address.
var ServerIPsFromConfigFile = []net.IP{}

// Names (without port) of all servers listed by name (i.e. not as
// a numeric IP address) in [server]/ip or [ServerPackages]/address.
var ServerNamesFromConfigFile = []string{}

// Names (without) of all servers listed in "gosa-si" SRV records.
var ServerNamesFromSRVRecords = []string{}

// The preferred server to register at when in client-only mode.
var PreferredServer = ""

// Default ruleset for detecting best network interface.
var InterfaceDetect = parseNetworkDetectRuleset("up~yes loopback~no srv~yes domain!~^$ ip!~^0.0.0.0$")

// The MAC address to send in the <macaddress> element.
// If [network]/my-mac does not contain '~' it is used directly as MAC.
// If it contains '~' it is a ruleset that is stored in MACDetect and
// used to fill in MAC in ReadNetwork().
// If the setting is not present or empty, the InterfaceDetect ruleset is used.
var MAC = ""
var MACDetect []NetDetRule

// IP address part of <source> element.
// If [network]/my-ip does not contain '~' it is used directly as IP.
// If it contains '~' it is a ruleset that is stored in IPDetect and
// used to fill in IP in ReadNetwork().
// If the setting is not present or empty, the MACDetect ruleset is used
// if it is not empty, or as last fallback the InterfaceDetect ruleset.
var IP = ""
var IPDetect []NetDetRule
var Broadcast = ""

// This machine's domain name.
// If [network]/my-domain does not contain '~' it is used directly as Domain.
// If it contains '~' it is a ruleset that is stored in DomainDetect and
// used to fill in Domain in ReadNetwork().
// If the setting is not present or empty, the first non-empty ruleset in
// following list is used: HostnameDetect, IPDetect, MACDetect, InterfaceDetect.
var Domain = ""
var DomainDetect []NetDetRule

// This machine's hostname.
// If [network]/my-hostname does not contain '~' it is used directly as Hostname.
// If it contains '~' it is a ruleset that is stored in HostnameDetect and
// used to fill in Hostname in ReadNetwork().
// If the setting is not present or empty, the first non-empty ruleset in
// following list is used: DomainDetect, IPDetect, MACDetect, InterfaceDetect.
var Hostname = ""
var HostnameDetect []NetDetRule

// Only log messages with level <= this number will be output.
// Note: The actual variable controlling the loglevel is util.LogLevel.
// This is just the value read from the config file.
var LogLevel int

// Maximum number of name+text bytes allowed in an individual here_i_am
// child element.
// Because hia info is stored in clientdb, we need to prevent badly behaved
// clients from overflowing us (which even if it does not fill the drive,
// may impact performance).
var HIAMaxElementSize = 1024

// Maximum number of all name+text bytes allowed in custom here_i_am elements.
// See HIAMaxElementSize.
var HIAMaxInfoSize = 4096

// Maximum number of active connections allowed. If this limit is reached,
// no further connections will be accepted on the socket.
var MaxConnections int32 = 512

// Maximum time permitted for a read or write transmission. If this time
// is exceeded, the transmission is aborted.
var Timeout = 5 * time.Minute

// Maximum time permitted for STARTTLS and TLS handshake.
var TimeoutTLS = 1 * time.Second

// Config used for TLS handshake when go-susi is acting as the
// server, i.e. the other party initiated the connection.
var TLSServerConfig *tls.Config

// Config used for TLS handshake when go-susi is acting as the
// client, i.e. it initiates the connection.
var TLSClientConfig *tls.Config

// This is set to false if the config file specifies at least one module key.
var TLSRequired = true

// Maximum time allowed for detect-hardware-hook. If the hook does not complete
// in this time, a standard detected_hardware message will be sent to the server.
var DetectHardwareTimeout = 30 * time.Second

// Maximum time to buffer and retry sending a message that informs the client
// of an action that is about to be performed (i.e. "trigger_action_*")
// This TTL is very short to avoid the situation where a client is off when
// the message is scheduled and then turns on due to a WOL and then gets
// the delayed message even though the client is already performing the
// operation that is being announced. See issue #169.
var ActionAnnouncementTTL = 15 * time.Second

// Maximum time to buffer and retry sending a "registered" message.
// This time is short because the client will not accept a registered
// message delayed more than 10s anyway.
var RegisteredMessageTTL = 10 * time.Second

// Maximum time to buffer and retry sending a client message without special
// properties (e.g. a "detect_hardware" message).
var NormalClientMessageTTL = 60 * time.Second

// If a peer is down for more than this time, its jobs are removed from the
// database and the connection to the peer is dropped. No reconnect will be
// attempted after this time, so unless the peer contacts us or go-susi is
// restarted (and the peer is listed in DNS or server.conf) there will be
// no further communication with this peer.
var MaxPeerDowntime = 7 * 24 * time.Hour

// When a request comes in from GOsa to modify or delete a foreign job,
// go-susi does not apply it directly to its own jobdb. Instead it
// forwards the request to the responsible siserver. Because of this,
// a gosa_query_jobdb done right after such a request will not reflect
// the changes. As that's exactly what GOsa does, the user experience
// is suboptimal because it will seem like the request had no effect.
// To compensate for this, gosa_query_jobdb delays for at most the
// duration specified in this variable. The delay only happens if
// a foreign job modification was forwarded shortly before the
// gosa_query_jobdb, so there is no delay during normal operation.
//
// GOsa note: GOsa has a very short timeout for gosa_query_jobdb
// requests (normally 5s). This duration needs to be shorter.
//
// Note: peer_connection:SyncIfNotGoSusi() bases its delay on
// this value to make sure that in the case a full sync is
// caused by a forwarded request, the full sync occurs before
// the delay of gosa_query_jobdb is finished. If this variable
// is changed peer_connection:SyncIfNotGoSusi() should be checked
// to make sure its derived wait time is still enough.
var GosaQueryJobdbMaxDelay = 4*time.Second

// The interval between calls to db.groomJobDB() to clean up stale jobs.
var JobDBGroomInterval = 1*time.Hour

// The maximum delay between a change to a database and the writing
// of the new data to disk. Longer delays improve performance and reduce
// memory usage.
var DBPersistDelay = 1*time.Second

// If true, then CNs created by go-susi based on DNS names will be full-qualified.
var FullQualifiedCN = false

// If the DN of a system has one of these strings as a suffix, go-susi will not
// change its CN, even if it otherwise would have (e.g. here_i_am updates the CN if
// it does not match the DNS name).
// If the system's CN starts with CNAutoPrefix and ends with CNAutoSuffix,
// the blacklist is not applied.
var CNRenameBlacklist = []string{",o=go-susi,c=de"}

// prefix for auto-generated CNs.
var CNAutoPrefix = "_"

// suffix for auto-generated CNs.
var CNAutoSuffix = "_"

// If true, existing data in /var/lib/go-susi will be discarded.
var FreshDatabase = false

// true => add peer servers from DNS to serverdb.
var DNSLookup = true

// List of domains, each starting with a dot, that will be
// appended in turn to short names that DNS can't resolve.
var LookupDomains = []string{}

// URI for connecting to LDAP server.
var LDAPURI = "ldap://localhost:389"

// LDAP searches will be restricted to the subtree rooted at this DN.
var LDAPBase = "c=de"

// List of DNs under which to search (1 level) for additional servers
// that can not be found under LDAPBase.
var LDAPServerOUs = []string{}

// DN of the admin user for writing to LDAP.
// If this is not a valid DN, RunServer will be false and only client
// services will be offered.
var LDAPAdmin = ""

// File containing the password of the admin user for writing to LDAP.
var LDAPAdminPasswordFile string

// DN of the user for reading from LDAP. Empty string means anonymous.
var LDAPUser = ""

// File containing the password of the user for reading from LDAP.
var LDAPUserPasswordFile string

// The unit tag for this server. If "", unit tags are not used.
var UnitTag = ""

// Filter that is ANDed with all LDAP queries. Must be enclosed in parentheses if non-empty.
var UnitTagFilter = ""

// only if UnitTag != "", this is the DN of the first object under LDAPBase that matches
// (&(objectClass=gosaAdministrativeUnit)(gosaUnitTag=...))
var AdminBase = ""

// only if UnitTag != "", this is the ou attribute of AdminBase
var Department = ""

// the dn of the ou=fai that contains all the FAI classes
var FAIBase = ""

// (R)DN of ou where new systems are put. If it ends with a "," then
// LDAPBase will be appended.
var IncomingOU = "ou=incoming,"

// db.FAIClasses() will not return entries older than this.
// See also FAIClassesCacheYoungAge
var FAIClassesMaxAge = 30 * time.Second

// go-susi tries to speed up FAIClasses() calls by holding the cache
// fresh. When certain messages are received that make it appear likely that
// db.FAIClasses() will be called in the near future, go-susi will
// automatically refresh the cache, but not if it is more recent than
// FAIClassesCacheYoungAge.
var FAIClassesCacheYoungAge = 15 * time.Second

// true if "--version" is passed on command line
var PrintVersion = false

// true if "--help" is passed on command line
var PrintHelp = false

// true if "--stats" is passed on the command line
var PrintStats = false

const re_1xx = "(1([0-9]?[0-9]?))"
const re_2xx = "(2([6-9]|([0-4][0-9]?)|(5[0-5]?))?)"
const re_xx  = "([3-9][0-9]?)"
const ip_part = "(0|"+re_1xx+"|"+re_2xx+"|"+re_xx+")"
var ipRegexp = regexp.MustCompile("^"+ip_part+"([.]"+ip_part+"){3}$")
var macAddressRegexp = regexp.MustCompile("^[0-9A-Fa-f]{2}(:[0-9A-Fa-f]{2}){5}$")
// valid as per RFC 1123. Originally, RFC 952 specified that hostname segments could not start with a digit.
var hostnameRegexp = regexp.MustCompile("^([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9-]*[A-Za-z0-9])$")
var domainRegexp = regexp.MustCompile("^((([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9-]*[a-zA-Z0-9])\\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9-]*[A-Za-z0-9]))$")

// Set up TempDir, LDAPAdminPasswordFile and LDAPUserPasswordFile.
// The TempDir is deleted when Shutdown() is called.
func Init() {
  tempdir, err := ioutil.TempDir("", "go-susi-")
  TempDir = tempdir
  if err != nil { panic(err) }
  err = os.Chmod(tempdir, 0700)
  if err != nil { panic(err) }

  LDAPAdminPasswordFile = TempDir + "/" + "ldapadminpw"
  err = ioutil.WriteFile(LDAPAdminPasswordFile, []byte{}, 0600)
  if err != nil { panic(err) }

  LDAPUserPasswordFile = TempDir + "/" + "ldapuserpw"
  err = ioutil.WriteFile(LDAPUserPasswordFile, []byte{}, 0600)
  if err != nil { panic(err) }
}

// Parses args and sets config variables accordingly.
func ReadArgs(args []string) {
  LogLevel = 0
  for i := 0; i < len(args); i++ {
    arg := args[i]
  
    if arg == "-v" || arg == "-vv" || arg == "-vvv" || arg == "-vvvv" || 
       arg == "-vvvvv" || arg == "-vvvvvv" || arg == "-vvvvvvv" {
    
      LogLevel = len(arg) - 1
    
    } else if arg == "-f" {
    
      FreshDatabase = true
    
    } else if strings.HasPrefix(arg, "--test=") {
    
      testdir := arg[7:]
      LogFilePath = testdir + "/go-susi.log"
      ServerConfigPath = testdir + "/server.conf"
      ServersOUConfigPath = testdir + "/ou=servers.conf"
      ClientConfigPath = testdir + "/client.conf"
      JobDBPath = testdir + "/jobdb.xml"
      ServerDBPath = testdir + "/serverdb.xml"
      ClientDBPath = testdir + "/clientdb.xml"
      CACertPath = []string{testdir + "/ca.cert"}
      CertPath = testdir + "/si.cert"
      CertKeyPath = testdir + "/si.key"

      PackageCacheDir = testdir
      FAILogPath = testdir
      
    } else if arg == "-c" {
      i++
      if i >= len(args) {
        util.Log(0, "ERROR! ReadArgs: missing argument to -c")
      } else {
        ServerConfigPath = args[i]
        ClientConfigPath = ""
      }
    } else if arg == "--help" {
    
      PrintHelp = true
      
    } else if arg == "--version" {      
      
      PrintVersion = true
      
    } else if arg == "--stats" {      
      
      PrintStats = true
    } else {
      util.Log(0, "ERROR! ReadArgs: Unknown command line switch: %v", arg)
    }
  }
}

// Parses the relevant configuration files and sets 
// the config variables accordingly.
func ReadConfig() {
  conf := map[string]map[string]string{"":map[string]string{}}
  
  var tftp_mappings deque.Deque
  
  // [general]/pxelinux-cfg-hook is deprecated and only supported for
  // backwards compatibility. It will be converted to patterns later.
  // That's why this is a local variable and not a global one.
  pxeLinuxCfgHookPath := "/usr/lib/go-susi/generate_pxelinux_cfg"
  
  for _, configfile := range []string{ClientConfigPath, ServerConfigPath} {
    if configfile == "" { continue }
    file, err := os.Open(configfile)
    if err != nil {
      if os.IsNotExist(err) { 
        /* File does not exist is not an error that needs to be reported */ 
      } else {
        util.Log(0, "ERROR! ReadConfig: %v", err)
      }
      continue
    }
    defer file.Close()
    input := bufio.NewReader(file)

    current_section := ""
    for {
      var line string
      line, err = input.ReadString('\n')
      if err != nil { break }
      
      if comment := strings.Index(line, "#"); comment >= 0 {
        line = line[0:comment]
      }
      
      line = strings.TrimSpace(line)
      if len(line) > 2 && line[0] == '[' && line[len(line)-1] == ']' {
        current_section = line
        if _, ok := conf[current_section]; !ok {
          conf[current_section] = map[string]string{}
        }
      }
      
      i := strings.Index(line, "=")
      if i >= 0 {
        key := strings.TrimSpace(line[0:i])
        value := strings.TrimSpace(line[i+1:])
        if key != "" {
          if current_section == "[tftp]" && key[0] == '/' && len(key) >= 2 {
            tftp_mappings.Push(key[1:])
            tftp_mappings.Push(value)
          } else {
            conf[current_section][key] = value
          }
        }
      }
    }

    if err != io.EOF {
      util.Log(0, "ERROR! ReadConfig: %v", err)
      // Do not return. Try working with whatever we got out of the file.
    }
  }
  
  for sectionname, section := range conf {
    if sectkey, ok := section["key"]; ok {
      ModuleKeys = append(ModuleKeys, sectkey)
      ModuleKey[sectionname] = sectkey
    }
  }
  
  TLSRequired = len(ModuleKey) == 0
  
  if general, ok := conf["[general]"]; ok {
    if logfile, ok := general["log-file"]; ok {
      LogFilePath = logfile
    }
    if failogdir, ok := general["fai-log-dir"]; ok {
      FAILogPath = failogdir
    }
    if kernel_list_hook, ok := general["kernel-list-hook"]; ok {
      KernelListHookPath = kernel_list_hook
    }
    if package_list_hook, ok := general["package-list-hook"]; ok {
      PackageListHookPath = package_list_hook
    }
    if user_msg_hook, ok := general["user-msg-hook"]; ok {
      UserMessageHookPath = user_msg_hook
    }
    if pxelinux_cfg_hook, ok := general["pxelinux-cfg-hook"]; ok {
      pxeLinuxCfgHookPath = pxelinux_cfg_hook
    }
    if new_config_hook, ok := general["new-config-hook"]; ok {
      NewConfigHookPath = new_config_hook
    }
    if trigger_action_hook, ok := general["trigger-action-hook"]; ok {
      TriggerActionHookPath = trigger_action_hook
    }
    if registered_hook, ok := general["registered-hook"]; ok {
      RegisteredHookPath = registered_hook
    }
    if activated_hook, ok := general["activated-hook"]; ok {
      ActivatedHookPath = activated_hook
    }
    if detect_hardware_hook, ok := general["detect-hardware-hook"]; ok {
      DetectHardwareHookPath = detect_hardware_hook
    }
    if fai_progress, ok := general["fai-progress-hook"]; ok {
      FAIProgressHookPath = fai_progress
    }
    if fai_savelog, ok := general["fai-savelog-hook"]; ok {
      FAISavelogHookPath = fai_savelog
    }
    if fai_audit, ok := general["fai-audit-hook"]; ok {
      FAIAuditHookPath = fai_audit
    }
  }
  
  if server, ok:= conf["[server]"]; ok {
    
    if dnslookup, ok := server["dns-lookup"]; ok {
      dnslookup = strings.TrimSpace(dnslookup)
      if dnslookup != "false" && dnslookup != "true" {
        util.Log(0, "ERROR! ReadConfig: [server]/dns-lookup must be \"true\" or \"false\", not \"%v\"", dnslookup)
      }
      DNSLookup = (dnslookup == "true")
    }
    
    if port,ok := server["port"]; ok {
      port = strings.TrimSpace(port)
      i := strings.Index(ServerSourceAddress,":")
      ServerSourceAddress = ServerSourceAddress[:i+1] + port
      i = strings.Index(ServerListenAddress,":")
      ServerListenAddress = ServerListenAddress[:i+1] + port
    }
    
    if ip,ok := server["ip"]; ok { PreferredServer = ip }
    if uri, ok := server["ldap-uri"]; ok { LDAPURI = uri }
    if base,ok := server["ldap-base"]; ok { LDAPBase = base }
    if newsysbase,ok := server["new-systems-base"]; ok { IncomingOU = newsysbase }
    if IncomingOU[len(IncomingOU)-1] == ',' { IncomingOU += LDAPBase }
    if admin,ok:= server["ldap-admin-dn"]; ok { LDAPAdmin = admin }
    if pw  ,ok := server["ldap-admin-password"]; ok { 
      err := ioutil.WriteFile(LDAPAdminPasswordFile, []byte(pw), 0600)
      if err != nil { util.Log(0, "ERROR! Could not write admin password to file: %v", err) }
    }
    if user,ok := server["ldap-user-dn"]; ok { LDAPUser = user }
    if pw  ,ok := server["ldap-user-password"]; ok { 
      err := ioutil.WriteFile(LDAPUserPasswordFile, []byte(pw), 0600)
      if err != nil { util.Log(0, "ERROR! Could not write user password to file: %v", err) } 
    }
  }
  
  if client, ok:= conf["[client]"]; ok {
    if port,ok := client["port"]; ok {
      ClientPorts = strings.Fields(strings.Replace(port,","," ",-1))
    }
    if einfo,ok := client["extra-info-file"]; ok {
      ExtraInfoFilePath = einfo
    }
  }
  
  if faimon, ok:= conf["[faimon]"]; ok {
    if port,ok := faimon["port"]; ok {
      FAIMonPort = port
    }
  }
  
  if tftp, ok:= conf["[tftp]"]; ok {
    if port,ok := tftp["port"]; ok {
      TFTPPort = port
    }
  }
  
  if tlsconf, ok:= conf["[tls]"]; ok {
    if cacert,ok := tlsconf["ca-certificate"]; ok {
      CACertPath = []string{cacert}
    }
    if cert,ok := tlsconf["certificate"]; ok {
      CertPath = cert
    }
    if certkey,ok := tlsconf["keyfile"]; ok {
      CertKeyPath = certkey
    }
  }
  
  // Backwards compatibility: Convert [general]/pxelinux-cfg-hook to patterns
  // as described in manual.
  if pxeLinuxCfgHookPath != "" {
    tftp_mappings.Insert("|"+pxeLinuxCfgHookPath)
    tftp_mappings.Insert("^pxelinux.cfg/01-(?P<macaddress>[0-9a-f]{2}(-[0-9a-f]{2}){5})$")
    tftp_mappings.Insert("")
    tftp_mappings.Insert("/^pxelinux.cfg/[0-9a-f]{8}(-[0-9a-f]{4}){3}-[0-9a-f]{12}$")
  }
  
  for !tftp_mappings.IsEmpty() {
    file := tftp_mappings.Pop().(string)
    pattern := tftp_mappings.Pop().(string)
    if pattern[0] != '^' { 
      pattern = "^" + regexp.QuoteMeta(pattern) + "$" 
      file = strings.Replace(file, "$", "$$", -1)
    }
    re, err := regexp.Compile(pattern)
    if err != nil {
      util.Log(0, "ERROR! ReadConfig: In section [tftp]: Error compiling regex \"%v\": %v", pattern, err)
    } else {
      TFTPRegexes = append(TFTPRegexes, re)
      TFTPReplies = append(TFTPReplies, file)
    }
  }
  
  // The [ServerPackages] section must be evaluated AFTER the [server]
  // section, because the manual says that [ServerPackages]/dns-lookup takes
  // precedence over [server]/dns-lookup.
  if serverpackages, ok := conf["[ServerPackages]"]; ok {
    if addresses, ok := serverpackages["address"]; ok && addresses != "" {
      PeerServers = append(PeerServers, strings.Fields(strings.Replace(addresses,","," ",-1))...)
    }
    if dnslookup, ok := serverpackages["dns-lookup"]; ok {
      dnslookup = strings.TrimSpace(dnslookup)
      if dnslookup != "false" && dnslookup != "true" {
        util.Log(0, "ERROR! ReadConfig: [ServerPackages]/dns-lookup must be \"true\" or \"false\", not \"%v\"", dnslookup)
      }
      DNSLookup = (dnslookup == "true")
    }
    if lookupdomains, ok := serverpackages["domains"]; ok {
      for _, dom := range strings.Fields(strings.Replace(lookupdomains,","," ",-1)) {
        if dom[0] != '.' { dom = "." + dom }
        LookupDomains = append(LookupDomains, dom)
      }
    }
  }
  
  if network, ok:= conf["[network]"]; ok {
    if mymac, ok := network["my-mac"]; ok {
      mymac = strings.ToLower(mymac)
      if !macAddressRegexp.MatchString(mymac) {
        MAC = ""
        MACDetect = parseNetworkDetectRuleset(mymac)
      } else {
        MAC = mymac
        MACDetect = nil
      }
    }
    if myip, ok := network["my-ip"]; ok {
      myip = strings.ToLower(myip)
      if !ipRegexp.MatchString(myip) {
        IP = ""
        IPDetect = parseNetworkDetectRuleset(myip)
      } else {
        IP = myip
        IPDetect = nil
      }
    }
    if myhostname, ok := network["my-hostname"]; ok {
      myhostname = strings.ToLower(myhostname)
      if !hostnameRegexp.MatchString(myhostname) {
        Hostname = ""
        HostnameDetect = parseNetworkDetectRuleset(myhostname)
      } else {
        Hostname = myhostname
        HostnameDetect = nil
      }
    }
    if mydomain, ok := network["my-domain"]; ok {
      mydomain = strings.ToLower(mydomain)
      if !domainRegexp.MatchString(mydomain) {
        Domain = ""
        DomainDetect = parseNetworkDetectRuleset(mydomain)
      } else {
        Domain = mydomain
        DomainDetect = nil
      }
    }
  }
  
  if MAC == "" && MACDetect == nil {
    MACDetect = InterfaceDetect
  }
  
  if IP == "" && IPDetect == nil {
    if MACDetect != nil {
      IPDetect = MACDetect
    } else {
      IPDetect = InterfaceDetect
    }
  }
  
  if Domain == "" && DomainDetect == nil {
    for _, s := range [][]NetDetRule{HostnameDetect, IPDetect, MACDetect, InterfaceDetect} {
      if s != nil {
        DomainDetect = s
        break
      }
    }
  }
  
  if Hostname == "" && HostnameDetect == nil {
    for _, s := range [][]NetDetRule{DomainDetect, IPDetect, MACDetect, InterfaceDetect} {
      if s != nil {
        HostnameDetect = s
        break
      }
    }
  }
  
  if LDAPAdmin == "" {
    RunServer = false
  }
  
  ClientPorts = append(ClientPorts, ServerListenAddress[strings.Index(ServerListenAddress,":")+1:])
  
  if PreferredServer != "" && strings.Index(PreferredServer,":") < 0 {
    PreferredServer += ServerListenAddress[strings.Index(ServerListenAddress,":"):]
  }
  
  serversInConfigFile := make([]string, len(PeerServers))
  copy(serversInConfigFile, PeerServers)
  if PreferredServer != "" {
    serversInConfigFile = append(serversInConfigFile, PreferredServer)
  }
  
  for _, srv := range serversInConfigFile {
    prefhost,_,err := net.SplitHostPort(srv)
    if err == nil {
      prefip := net.ParseIP(prefhost)
      if prefip != nil {
        ServerIPsFromConfigFile = append(ServerIPsFromConfigFile, prefip)
      } else {
        ServerNamesFromConfigFile = append(ServerNamesFromConfigFile, prefhost)
      }
    }
  }
  
  file, err := os.Open(ServersOUConfigPath)
  if err != nil {
    if os.IsNotExist(err) { 
      /* File does not exist is not an error that needs to be reported */ 
    } else {
      util.Log(0, "ERROR! ReadConfig: %v", err)
    }
  } else {
    defer file.Close()
    input := bufio.NewReader(file)
    for {
      var line string
      line, err = input.ReadString('\n')
      if err != nil { break }
      line = strings.TrimSpace(line)
      if line != "" {
        LDAPServerOUs = append(LDAPServerOUs, line)
      }
    }
  }
}

func ReadCertificates() {  
  have_something_valid := false
  certpool := x509.NewCertPool()
  tlscert, err := tls.LoadX509KeyPair(CertPath, CertKeyPath)
  if err != nil {
    util.Log(0, "ERROR! tls.LoadX509KeyPair: %v", err)
  } else {
    tlscert.Leaf, err = x509.ParseCertificate(tlscert.Certificate[0])
    if err != nil {
      util.Log(0, "ERROR! x509.ParseCertificate(%v): %v", CertPath, err)
    } else {
        
      for _, cacert_path := range CACertPath {
        root_ca, err := ioutil.ReadFile(cacert_path)
        if err != nil {
          util.Log(0, "ERROR! ReadFile: %v", err)
        } else {
          blk,_ := pem.Decode(root_ca)
          if blk == nil { blk = &pem.Block{} }
          cacert, err := x509.ParseCertificate(blk.Bytes)
          if err != nil {
            util.Log(0, "ERROR! x509.ParseCertificate(%v): %v", cacert_path, err)
          } else {
          
            CACert = append(CACert, cacert)
            
            if !certpool.AppendCertsFromPEM(root_ca) {
              util.Log(0, "ERROR! AppendCertsFromPEM: %v", err)
            } else {
              have_something_valid = true
            }
          }
        }
      }
    }
  }

  if have_something_valid {
    TLSClientConfig = &tls.Config{Certificates:[]tls.Certificate{tlscert},
                                  RootCAs:certpool,
                                  NextProtos:[]string{},
                                  ClientAuth:tls.RequireAnyClientCert,
                                  ClientCAs:certpool,
                                  ServerName:"",
                                  SessionTicketsDisabled:true,
                                  
                                  // We do our own verification in
                                  // security.ContextFor(). We can't use the
                                  // verification from the library because it
                                  // does not support all of the subjectAltName
                                  // variants we do (in particular registeredID
                                  // types).
                                  InsecureSkipVerify:true,
                                  }
    
    TLSServerConfig = &tls.Config{Certificates:[]tls.Certificate{tlscert},
                                  RootCAs:certpool,
                                  NextProtos:[]string{},
                                  ClientAuth:tls.RequireAnyClientCert,
                                  ClientCAs:certpool,
                                  ServerName:"",
                                  SessionTicketsDisabled:true,
                                  
                                  // We do our own verification in
                                  // security.ContextFor(). We can't use the
                                  // verification from the library because it
                                  // does not support all of the subjectAltName
                                  // variants we do (in particular registeredID
                                  // types).
                                  InsecureSkipVerify:true,
                                  }
  } else {
    util.Log(0, "WARNING! TLS is DISABLED!")
  }
}

var ndrRegexp = regexp.MustCompile("(?:([-+]?[1-9][0-9]*):)?([a-z]+)(!?~)(.*)")

type NetDetRule struct {
  Weight int
  Propname string
  Regex *regexp.Regexp
  InvertMatch bool
}

// Logs an error and returns nil if ruleset is invalid.
func parseNetworkDetectRuleset(ruleset string) []NetDetRule {
  var err error
  
  autoweight := []int{}
  
  f := strings.Fields(ruleset)
  result := make([]NetDetRule, len(f))
  for i := range f {
    sm := ndrRegexp.FindStringSubmatch(f[i])
    if sm == nil {
      util.Log(0, "ERROR! \"%v\" is not a proper net device matching rule", f[i])
      return nil
    }
    
    if sm[1] == "" { // no weight provided => power of 2 depending on position
      result[i].Weight = 1
      for k := range autoweight {
        result[autoweight[k]].Weight <<= 1
      }
      autoweight = append(autoweight, i)
    } else {
      result[i].Weight, err = strconv.Atoi(sm[1])
      if err != nil {
        util.Log(0, "ERROR! \"%v\" contains incorrect weight: %v", f[i], err)
        return nil
      }
    }
    
    result[i].Propname = sm[2]
    if result[i].Propname != "mac" &&
       result[i].Propname != "ifname" &&
       result[i].Propname != "ip" &&
       result[i].Propname != "hostname" &&
       result[i].Propname != "domain" &&
       result[i].Propname != "srv" &&
       result[i].Propname != "loopback" &&
       result[i].Propname != "up" {
    
       util.Log(0, "ERROR! \"%v\" refers to unknown property \"%v\"", f[i], sm[2])
        return nil
    }
    
    result[i].InvertMatch = (sm[3] == "!~")
    
    result[i].Regex, err = regexp.Compile(sm[4])
    if err != nil {
      util.Log(0, "ERROR! \"%v\" contains an invalid regex: %v", f[i], err)
      return nil
    }
  }
  
  return result
}

func bestInterface(logid string, ifaces []map[string]string, ruleset []NetDetRule) map[string]string {
  if len(ifaces) == 0 {
    return map[string]string{}
  }

  weights := []string{}
  
  bestweight := 0
  bestiface := 0
  
  for i := range ifaces {
    weight := 0
    for r := range ruleset {
      if ruleset[r].Regex.MatchString(ifaces[i][ruleset[r].Propname]) != ruleset[r].InvertMatch {
        weight += ruleset[r].Weight
      }
    }
    
    weights = append(weights, fmt.Sprintf("%v=%v", ifaces[i]["ifname"], weight))
    
    if weight > bestweight {
      bestweight = weight
      bestiface = i
    }
  }
  
  util.Log(1, "INFO! Weights for \"%v\": %v", logid, strings.Join(weights," "))
  
  return ifaces[bestiface]
}

func determineNetworkID(ifaces []map[string]string, propname string, hardwired string, ruleset []NetDetRule) string {
  if hardwired != "" { return hardwired }
  return bestInterface(propname, ifaces, ruleset)[propname]
}

// Reads network parameters.
func ReadNetwork() {
  var err error
  
  var ifaces []map[string]string
  // use os.Hostname as default in case we can't get a host name from an interface
  var hostname string
  hostname, err = os.Hostname()
  if err != nil {
    util.Log(0, "ERROR! os.Hostname(): %v", err)
    hostname = "localhost"
  }
  
  net_ifaces, err := net.Interfaces()
  if err != nil {
    util.Log(0, "ERROR! ReadNetwork => net.Interfaces(): %v", err)
  } else
  {
    // find non-loopback interfaces that are up
    for _, net_iface := range net_ifaces {
      iface := map[string]string{
        "mac":    net_iface.HardwareAddr.String(),
        "ifname": net_iface.Name,
        "ip": "0.0.0.0",
        "broadcast": "255.255.255.255",
        "hostname": hostname,
        "domain": "",
        "srv": "no",
        "loopback": "no",
        "up": "yes",
      }
      
      if net_iface.Flags & net.FlagLoopback != 0 { iface["loopback"] = "yes" }
      if net_iface.Flags & net.FlagUp == 0 { iface["up"] = "no" }
      
      var addrs []net.Addr
      addrs, err = net_iface.Addrs()
      if err == nil {

        // find the best IP address for that interface
        var ip net.IP
        var broadcast net.IP
        for _, addr := range addrs {
          ip2, ipnet, err2 := net.ParseCIDR(addr.String())
          if err2 == nil {
            if ip == nil || (ip.IsLoopback() && !ip2.IsLoopback()) ||
             (ip.To4() == nil && ip2.To4() != nil) {
              ip = ip2
              broadcast = MakeBroadcast(ip2, ipnet.Mask)
            }
          }
        }
        if ip != nil {
          iface["ip"] = ip.String()
          iface["broadcast"] = broadcast.String()
        } else {
          err = fmt.Errorf("Could not determine IP for interface %v/%v", iface["ifname"], iface["mac"])
        }
      }
      
      if err == nil {
        var names []string
        names, err = net.LookupAddr(iface["ip"])
        if err == nil {
          for _, name := range names {
            name = strings.Trim(name, ".")
            if name == "" { continue }
            
            // override default os.Hostname() with resolved name for this iface
            // if it contains a domain we chop it off later
            iface["hostname"] = name
            
            i := strings.Index(name, ".")
            if i > 0 {
              iface["hostname"] = name[0:i]
              iface["domain"] = name[i+1:]
              // we found a name that includes a domain => don't look further, this is as good as it gets
              goto DomainFound
            }
          }
          err = fmt.Errorf("Could not determine domain. Lookup of IP %v returned %v", iface["ip"], names)
        DomainFound:
        } 
      }
      
      if err != nil {
        util.Log(1, "INFO! %v: %v", iface["ifname"],err)
      }
      
      if iface["domain"] != "" {
        var addrs []*net.SRV
        _, addrs, err := net.LookupSRV("gosa-si", "tcp", iface["domain"])
        if err != nil {
          util.Log(1, "INFO! LookupSRV(\"gosa-si\",\"tcp\",\"%v\"): %v", iface["domain"], err) 
        } else 
        { 
          if len(addrs) > 0 {
            iface["srv"] = "yes"
          }

          for _, srv := range addrs {
            ServerNamesFromSRVRecords = append(ServerNamesFromSRVRecords, srv.Target)
          }
        }
      }
      
      ifaces = append(ifaces, iface)
    }
  }
  
  util.Log(1, "INFO! Found network interfaces: %v", ifaces )

  MAC = determineNetworkID(ifaces, "mac", MAC, MACDetect)
  Hostname = determineNetworkID(ifaces, "hostname", Hostname, HostnameDetect)
  Domain = determineNetworkID(ifaces, "domain", Domain, DomainDetect)
  IP = determineNetworkID(ifaces, "ip", IP, IPDetect)
  Broadcast = determineNetworkID(ifaces, "broadcast", "", IPDetect)
  
  ServerSourceAddress = IP + ServerListenAddress[strings.Index(ServerListenAddress,":"):]
  
  util.Log(1, "INFO! Hostname: %v  Domain: %v  MAC: %v  Listener: %v  Broadcast: %v", Hostname, Domain, MAC, ServerSourceAddress, Broadcast)
  
  if PreferredServer != "" {
    pref, err := util.Resolve(PreferredServer, IP)
    if err != nil {
      util.Log(0, "ERROR! Could not resolve [server]/ip value \"%v\": %v", PreferredServer, err)
      PreferredServer = ""
    } else {
      PreferredServer = pref
    }
  }
  
}

// Returns the broadcast address of ip with network mask.
func MakeBroadcast(ip net.IP, mask net.IPMask) net.IP {
  if len(ip) == 0 || len(mask) == 0 { panic("Can't...make...broadcast...Must...panic...") }
  if len(mask) == net.IPv6len && len(ip) == net.IPv4len {
    mask = mask[12:]
  }
  if len(mask) == net.IPv4len && len(ip) == net.IPv6len {
    ip = ip[12:]
  }
  n := len(ip)
  out := make(net.IP, n)
  if n != len(mask) {
    copy(out, ip)
    out[len(out)-1] = 255
    return out
  }
  for i := 0; i < n; i++ {
    out[i] = ip[i] | ^mask[i]
  }
  return out
}


// Returns the gosa-si servers listed in DNS.
func ServersFromDNS() []string {
  var cname string
  var addrs []*net.SRV
  cname, addrs, err := net.LookupSRV("gosa-si", "tcp", Domain)
  if err != nil {
    util.Log(1, "INFO! LookupSRV: %v", err) 
    return []string{}
  }
  
  servers := make([]string, len(addrs))
  if len(addrs) == 0 {
    util.Log(1, "INFO! No other go-susi or gosa-si servers listed in DNS for domain '%v'", Domain)
  } else {
    for i := range addrs {
      servers[i] = fmt.Sprintf("%v:%v", strings.TrimRight(addrs[i].Target,"."), addrs[i].Port)
    }
    util.Log(1, "INFO! DNS lists the following %v servers: %v", cname, strings.Join(servers,", "))
  }
  return servers
}

// Constructs a standard environment for hook execution and returns it.
func HookEnvironment() []string {
  env := []string{"MAC="+MAC, "IPADDRESS="+IP, "SERVERPORT="+ServerListenAddress[strings.Index(ServerListenAddress,":")+1:],
                  "HOSTNAME="+Hostname, "FQDN="+Hostname+"."+Domain }
  return env
}

func Shutdown() {
  util.Log(1, "INFO! Removing temporary directory %v", TempDir)
  err := os.RemoveAll(TempDir)
  if err != nil {
    util.Log(0, "ERROR! Could not remove temporary directory: %v", err)
  }
}
