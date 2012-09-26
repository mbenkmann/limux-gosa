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


// This machine's hostname.
var Hostname = "localhost"

// This machine's domain name.
var Domain = "localdomain"

// The MAC address to send in the <macaddress> element.
var MAC = "01:02:03:04:05:06"


// Only log messages with level <= this number will be output.
// Note: The actual variable controlling the loglevel is util.LogLevel.
// This is just the value read from the config file.
var LogLevel int

// Parses os.Args and sets config variables accordingly.
func ReadArgs() {
  LogLevel = 0
  for _, arg := range os.Args[1:] {
  
    if strings.HasPrefix(arg, "-v") {
    
      LogLevel = len(arg) - 1
      
    } else if strings.HasPrefix(arg, "--test=") {
    
      testdir := arg[7:]
      LogFilePath = testdir + "/go-susi.log"
      ServerConfigPath = testdir + "/server.conf"
      JobDBPath = testdir + "/jobdb.xml"
      ServerDBPath = testdir + "/serverdb.xml"
      
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
  
}

// Reads network parameters.
func ReadNetwork() {
  var err error
  
  var ifaces []net.Interface
  ifaces, err = net.Interfaces()
  if err == nil {
    
    // find the first non-loopback interface that is up
    for _, iface := range ifaces {
      if iface.Flags & net.FlagLoopback != 0 { continue }
      if iface.Flags & net.FlagUp == 0 { continue }
      
      var addrs []net.Addr
      addrs, err = iface.Addrs()
      if err == nil {
        MAC = iface.HardwareAddr.String()
        
        // find the first IP address for that interface
        for _, addr := range addrs {
          ip, _, err2 := net.ParseCIDR(addr.String())
          if err2 == nil && !ip.IsLoopback() {
            IP = ip.String()
            ServerSourceAddress = IP + ServerListenAddress[strings.Index(ServerListenAddress,":"):]
            goto FoundIP
          }
        }
        err = fmt.Errorf("Could not determine IP for interface %v", MAC)
        FoundIP:
      }
    }
  }
  
  if err != nil {
    util.Log(0, "ERROR! ReadNetwork: %v", err)
  }
  
  var hostname string
  hostname, err = os.Hostname()
  if err == nil {
    Hostname = hostname
    var names []string
    names, err = net.LookupAddr(IP)
    if err == nil {
      for _, name := range names {
        if strings.HasPrefix(name, hostname + ".") {
          Domain = name[len(hostname)+1:]
          goto DomainFound
        }
      }
      err = fmt.Errorf("Could not determine domain for hostname '%v'. Lookup of IP %v returned %v", Hostname, IP, names)
    DomainFound:
    }
  }
  
  if err != nil {
    util.Log(0, "ERROR! ReadNetwork: %v", err)
  }
  
  util.Log(1, "INFO! Hostname: %v  Domain: %v  MAC: %v  Server: %v", Hostname, Domain, MAC, ServerSourceAddress)
}

