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
         "strings"
         "crypto/aes"
         
         "../util"
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

// IP address part of <source> element.
var IP = "127.0.0.1"

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

// Path to database of scheduled jobs.
var JobDBPath = "/var/lib/go-susi/jobdb.xml"

// Path to database of peer servers.
var ServerDBPath = "/var/lib/go-susi/serverdb.xml"

// Path to database of clients (foreign and our own).
var ClientDBPath = "/var/lib/go-susi/clientdb.xml"

// Called by db.HooksExecute() to generate the kernel db.
var KernelListHookPath = "/usr/lib/go-susi/generate_kernel_list"

// Called by db.HooksExecute() to generate the packages db.
var PackageListHookPath = "/usr/lib/go-susi/generate_package_list"

// Called when a job_send_user_msg job is executed.
var UserMessageHookPath = "/usr/lib/go-susi/send_user_msg"

// Called to generate a pxelinux.cfg file for a system.
var PXELinuxCfgHookPath = "/usr/lib/go-susi/generate_pxelinux_cfg"

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

// Path where log files from CLMSG_save_fai_log are stored.
// Within this directory go-susi creates sub-directories named
// after the clients' MAC addresses and symlinks named after the
// clients' plain names.
var FAILogPath = "/var/log/fai"

// Port for accepting FAI status updates send via /usr/lib/fai/subroutines:sendmon()
var FAIMonPort = "4711"

// UDP Port for receiving TFTP requests
var TFTPPort = "69"

// Potential ports for clients. This list is used for 2 purposes:
//   1) to distinguish between standard clients and test clients
//   2) to attempt contact with a client with known IP but unknown port 
// NOTE: The server port is appended to this list by ReadConfig().
var ClientPorts = []string{"20083"}

// Maps a file name as contained in a TFTP request to the actual path on
// the local filesystem for satisfying that request.
// Only files contained in this map will be served, in addition to
// files matching "pxelinux.cfg/<MAC>".
var TFTPFiles = map[string]string{}

// Temporary directory only accessible by the user running go-susi.
// Used e.g. for storing password files. Deleted in config.Shutdown().
var TempDir = ""
func init() {
  tempdir, err := ioutil.TempDir("", "go-susi-")
  TempDir = tempdir
  if err != nil { panic(err) }
  err = os.Chmod(tempdir, 0700)
  if err != nil { panic(err) }
}

// host:port addresses of peer servers read from config file.
var PeerServers = []string{}

// The preferred server to register at when in client-only mode.
var PreferredServer = ""

// This machine's hostname.
var Hostname = "localhost"

// This machine's domain name.
var Domain = "localdomain"

// The MAC address to send in the <macaddress> element.
var MAC = "01:02:03:04:05:06"

type InterfaceInfo struct {
  IP string   // IP address of this machine for this interface. If this string begins with "ERROR!", then it could not be determined
  Hostname string // hostname (without domain) of the above IP. If it begins with "ERROR!" it could not be determined
  Domain string   // domain of the above IP. If it begins with "ERROR!" it could not be determined
  Interface net.Interface // low level interface data
  HasPeers bool // true if DNS has SRV records for tcp/gosa-si (if Domain could not be determined, this will always be false)
}

// Information about each non-loopback interface that is UP.
var Interfaces = []InterfaceInfo{}

// index in Interfaces of the most appropriate interface to use. -1 if none could be determined
var BestInterface = -1

// Only log messages with level <= this number will be output.
// Note: The actual variable controlling the loglevel is util.LogLevel.
// This is just the value read from the config file.
var LogLevel int

// Maximum number of active connections allowed. If this limit is reached,
// no further connections will be accepted on the socket.
var MaxConnections int32 = 512

// Maximum time permitted for a read or write transmission. If this time
// is exceeded, the transmission is aborted.
var Timeout = 5 * time.Minute

// Maximum time allowed for detect-hardware-hook. If the hook does not complete
// in this time, a standard detected_hardware message will be sent to the server.
var DetectHardwareTimeout = 30 * time.Second

// Maximum time to buffer and retry sending a message that is of interest only
// to a local client, such as new_ldap_config. This TTL is very short.
var LocalClientMessageTTL = 0 * time.Second

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

// DN of the admin user for writing to LDAP.
// If this is not a valid DN, RunServer will be false and only client
// services will be offered.
var LDAPAdmin = ""

// File containing the password of the admin user for writing to LDAP.
var LDAPAdminPasswordFile string
func init() {
  // The assignment must be inside init() instead of (as would be nicer)
  // as part of the var declaration, because TempDir is initialized in init().
  LDAPAdminPasswordFile = TempDir + "/" + "ldapadminpw"
  err := ioutil.WriteFile(LDAPAdminPasswordFile, []byte{}, 0600)
  if err != nil { panic(err) }
}

// DN of the user for reading from LDAP. Empty string means anonymous.
var LDAPUser = ""

// File containing the password of the user for reading from LDAP.
var LDAPUserPasswordFile string
func init() {
  // The assignment must be inside init() instead of (as would be nicer)
  // as part of the var declaration, because TempDir is initialized in init().
  LDAPUserPasswordFile = TempDir + "/" + "ldapuserpw"
  err := ioutil.WriteFile(LDAPUserPasswordFile, []byte{}, 0600)
  if err != nil { panic(err) }
}

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
      ClientConfigPath = testdir + "/client.conf"
      JobDBPath = testdir + "/jobdb.xml"
      ServerDBPath = testdir + "/serverdb.xml"
      ClientDBPath = testdir + "/clientdb.xml"
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
          conf[current_section][key] = value
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
      PXELinuxCfgHookPath = pxelinux_cfg_hook
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
    
    if ip,ok := server["ip"]; ok {
      pref, err := util.Resolve(ip)
      if err != nil {
        util.Log(0, "ERROR! Could not resolve [server]/ip value \"%v\": %v",ip, err)
      } else {
        PreferredServer = pref
      }
    }
    
    if uri, ok := server["ldap-uri"]; ok { LDAPURI = uri }
    if base,ok := server["ldap-base"]; ok { LDAPBase = base }
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
    
    for tftp_path, real_path := range tftp {
      if len(tftp_path) > 1 && tftp_path[0] == '/' {
        TFTPFiles[tftp_path[1:]] = real_path
      }
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
  
  if LDAPAdmin == "" {
    RunServer = false
  }
  
  ClientPorts = append(ClientPorts, ServerListenAddress[strings.Index(ServerListenAddress,":")+1:])
  
  if strings.Index(PreferredServer,":") < 0 {
    PreferredServer += ServerListenAddress[strings.Index(ServerListenAddress,":"):]
  }
}

// Reads network parameters.
func ReadNetwork() {
  var err error
  
  var ifaces []net.Interface
  ifaces, err = net.Interfaces()
  if err != nil {
    util.Log(0, "ERROR! ReadNetwork: %v", err)
  } else
  {
    best_interface_weight := -1
    
    // find non-loopback interfaces that are up
    for _, iface := range ifaces {
      if iface.Flags & net.FlagLoopback != 0 { continue }
      if iface.Flags & net.FlagUp == 0 { continue }
      
      ifaceInfo := InterfaceInfo{}
      ifaceInfo.Interface = iface
      
      var addrs []net.Addr
      addrs, err = iface.Addrs()
      if err == nil {
        
        // find the first IP address for that interface
        for _, addr := range addrs {
          ip, _, err2 := net.ParseCIDR(addr.String())
          if err2 == nil && !ip.IsLoopback() {
            ifaceInfo.IP = ip.String()
            goto FoundIP
          }
        }
        err = fmt.Errorf("Could not determine IP for interface %v", iface.HardwareAddr.String())
      FoundIP:
      }
      
      if err != nil { 
        ifaceInfo.IP = fmt.Sprintf("ERROR! %v", err)
        ifaceInfo.Hostname = ifaceInfo.IP
        ifaceInfo.Domain = ifaceInfo.IP
      } else
      {
        var names []string
        names, err = net.LookupAddr(ifaceInfo.IP)
        //util.Log(2, "DEBUG! Names for %v: %v", ifaceInfo.IP, names)
        if err == nil {
          for _, name := range names {
            name = strings.Trim(name, ".")
            if name == "" { continue }
            
            // if we have no hostname yet, use the name from the address
            // if this includes a "." we'll chop off the domain in the if below
            if ifaceInfo.Hostname == "" { ifaceInfo.Hostname = name }
            
            i := strings.Index(name, ".")
            if i > 0 {
              ifaceInfo.Hostname = name[0:i]
              ifaceInfo.Domain = name[i+1:]
              goto DomainFound
            }
          }
          err = fmt.Errorf("Could not determine domain. Lookup of IP %v returned %v", ifaceInfo.IP, names)
        DomainFound:
        } 
        
        if err != nil {
          if ifaceInfo.Hostname == "" { ifaceInfo.Hostname = fmt.Sprintf("ERROR! %v", err) }
          ifaceInfo.Domain = fmt.Sprintf("ERROR! %v", err)
        }
      }
      
      if !strings.HasPrefix(ifaceInfo.Domain, "ERROR!") {
        var addrs []*net.SRV
        _, addrs, err := net.LookupSRV("gosa-si", "tcp", ifaceInfo.Domain)
        if err != nil {
          util.Log(0, "ERROR! LookupSRV(\"gosa-si\",\"tcp\",\"%v\"): %v", ifaceInfo.Domain, err) 
        } else 
        { 
          ifaceInfo.HasPeers = (len(addrs) > 0)
        }
      }
      
      Interfaces = append(Interfaces, ifaceInfo)
      
      weight := 0
      if !strings.HasPrefix(ifaceInfo.IP, "ERROR!") { weight += 1 }
      if !strings.HasPrefix(ifaceInfo.Hostname, "ERROR!") { weight += 2 }
      if !strings.HasPrefix(ifaceInfo.Domain, "ERROR!") { weight += 4 }
      if ifaceInfo.HasPeers { weight += 8 }
      
      if BestInterface < 0 || weight > best_interface_weight { 
        BestInterface = len(Interfaces) - 1 
        best_interface_weight = weight
      }
    }
  }
  
  // use os.Hostname as default in case we can't get a host name from an interface
  var hostname string
  hostname, err = os.Hostname()
  if err == nil { Hostname = hostname }

  if BestInterface >= 0 {
    MAC = Interfaces[BestInterface].Interface.HardwareAddr.String()
    if !strings.HasPrefix(Interfaces[BestInterface].Hostname, "ERROR!") {
      Hostname = Interfaces[BestInterface].Hostname
    }
    if !strings.HasPrefix(Interfaces[BestInterface].Domain, "ERROR!") {
      Domain = Interfaces[BestInterface].Domain
    }
    if !strings.HasPrefix(Interfaces[BestInterface].IP, "ERROR!") {
      IP = Interfaces[BestInterface].IP
    }
    ServerSourceAddress = IP + ServerListenAddress[strings.Index(ServerListenAddress,":"):]
  }
  
  util.Log(1, "INFO! Hostname: %v  Domain: %v  MAC: %v  Server: %v", Hostname, Domain, MAC, ServerSourceAddress)
}

// Returns the gosa-si servers listed in DNS.
func ServersFromDNS() []string {
  var cname string
  var addrs []*net.SRV
  cname, addrs, err := net.LookupSRV("gosa-si", "tcp", Domain)
  if err != nil {
    util.Log(0, "ERROR! LookupSRV: %v", err) 
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
