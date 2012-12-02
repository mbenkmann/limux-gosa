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
         "path"
         
         "../util"
         "../config"
         "../db"
         "../action"
       )

// run unit tests
func UnitTests() {
  conffile, confdir := createConfigFile("unit-tests-","")
  defer os.RemoveAll(confdir)
  //defer fmt.Printf("\nLog file directory: %v\n", confdir)
  config.ReadArgs([]string{"-c", conffile, "--test="+confdir })
  config.ReadConfig()
  util.LogLevel = config.LogLevel
  
  os.MkdirAll(path.Dir(config.JobDBPath), 0750)
  
  config.ReadNetwork() // after config.ReadConfig()
  db.ServersInit() // after config.ReadNetwork()
  db.JobsInit() // after config.ReadConfig()
  action.Init()
  
  Deque_test()
  Util_test()
  Xml_test()
  DB_test()
}


