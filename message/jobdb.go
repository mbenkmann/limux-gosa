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

package message

import (
         "os"
         "time"
         "../xml"
         "../config"
         "../util"
       )

// Stores jobs to be executed at some point in the future.
var JobDB *xml.DB

// Initializes JobDB with data from the file config.JobDBPath if it exists.
func InitJobDB() {
  jobdb_storer := &xml.FileStorer{config.JobDBPath}
  var delay time.Duration = 0
  JobDB = xml.NewDB("jobdb", jobdb_storer, delay)
  xml, err := xml.FileToHash(config.JobDBPath)
  if err != nil {
    if os.IsNotExist(err) { 
      /* File does not exist is not an error that needs to be reported */ 
    } else
    {
      util.Log(0, "ERROR! InitJobDB() reading '%v': %v", config.JobDBPath, err)
    }
  } else
  {
    JobDB.Init(xml)
  }
}
