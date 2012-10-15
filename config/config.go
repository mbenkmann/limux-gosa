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

// The address sent in the <source> element.
var ServerSourceAddress = "127.0.0.1:20081"

// Where to send log messages (in addition to stderr).
var LogFilePath = "/var/log/go-susi.log"

// Path of the server config file.
var ServerConfigPath = "/etc/gosa-si/server.conf"

// Path to database of scheduled jobs.
var JobDBPath = "/var/lib/go-susi/jobdb.xml"

// Path to database of peer servers.
var ServerDBPath = "/var/lib/go-susi/serverdb.xml"

// host:port addresses of peer servers read from config file.
var PeerServers = []string{}

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

// Maximum time permitted for a read or write transmission. If this time
// is exceeded, the transmission is aborted.
var Timeout = 5 * time.Minute

// If true, existing data in /var/lib/go-susi will be discarded.
var FreshDatabase = false

// Parses os.Args and sets config variables accordingly.
func ReadArgs() {
  LogLevel = 0
  for i := 1; i < len(os.Args); i++ {
    arg := os.Args[i]
  
    if arg == "-v" || arg == "-vv" || arg == "-vvv" || arg == "-vvvv" || 
       arg == "-vvvvv" || arg == "-vvvvvv" || arg == "-vvvvvvv" {
    
      LogLevel = len(arg) - 1
    
    } else if arg == "-f" {
    
      FreshDatabase = true
    
    } else if strings.HasPrefix(arg, "--test=") {
    
      testdir := arg[7:]
      LogFilePath = testdir + "/go-susi.log"
      ServerConfigPath = testdir + "/server.conf"
      JobDBPath = testdir + "/jobdb.xml"
      ServerDBPath = testdir + "/serverdb.xml"
      
    } else if arg == "-c" {
      i++
      if i >= len(os.Args) {
        util.Log(0, "ERROR! ReadArgs: missing argument to -c")
      } else {
        ServerConfigPath = os.Args[i]
      }
    } else {
      util.Log(0, "ERROR! ReadArgs: Unknown command line switch: %v", arg)
    }
  }
}

// Parses the relevant configuration files and sets 
// the config variables accordingly.
func ReadConfig() {
  file, err := os.Open(ServerConfigPath)
  if err != nil {
    util.Log(0, "ERROR! ReadConfig: %v", err)
    return
  }
  defer file.Close()
  input := bufio.NewReader(file)
  
  conf := map[string]map[string]string{"":map[string]string{}}
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
    util.Log(0, "ERROR! ReadString: %v", err)
    // Do not return. Try working with whatever we got out of the file.
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
  }
  
  if serverpackages, ok := conf["[ServerPackages]"]; ok {
    if addresses, ok := serverpackages["address"]; ok && addresses != "" {
      for _,address := range strings.Split(addresses, ",") {
        PeerServers = append(PeerServers, strings.TrimSpace(address))
      }
    }
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

