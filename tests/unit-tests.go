/*
Copyright (c) 2012 Matthias S. Benkmann

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

package tests

import (
         "os"
         "os/exec"
         "syscall"
         "path"
         "time"
         
         "../util"
         "../config"
       )

// run unit tests
func UnitTests() {
  conffile, confdir := createConfigFile("unit-tests-","")
  defer os.RemoveAll(confdir)
  //defer fmt.Printf("\nLog file directory: %v\n", confdir)
  config.ReadArgs([]string{"-c", conffile, "--test="+confdir })
  config.Init()
  defer config.Shutdown()
  config.ReadConfig()
  util.LogLevel = config.LogLevel
  config.Timeout = 5*time.Second
  
  os.MkdirAll(path.Dir(config.JobDBPath), 0750)
  
  config.ReadNetwork() // after config.ReadConfig()
  
  // launch the test ldap server
  cmd := exec.Command("/usr/sbin/slapd","-f","./slapd.conf","-h","ldap://127.0.0.1:20088","-d","0")
  cmd.Dir = "./testdata"
  err := cmd.Start()
  if err != nil { panic(err) }
  // give slapd time to start up to prevent tests failing because LDAP port isn't listening
  time.Sleep(2*time.Second)
  
  defer func() { 
    cmd.Process.Signal(syscall.SIGTERM)
    // give slapd time to terminate, so that we don't get a conflict when
    // the system test starts its slapd instance
    time.Sleep(2*time.Second)
  }()
  
  Deque_test()
  Bytes_test()
  Util_test()
  Xml_test()
  DB_test() // Must run before Message_test()
  Message_test() // DB_test() must run before this to init db.*
}


