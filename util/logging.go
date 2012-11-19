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

package util

import (
         "io"
         "os"
         "log"
         "fmt"
         "time"
       )

// The *log.Logger used by util.Log(). By default it simply prints logged
// messages to Stderr without adding any kind of prefix, time, etc.
var Logger = log.New(os.Stderr, "", 0)

// Only messages with a level <= this number will be printed.
var LogLevel = 0

// Outputs a message to util.Logger formatted as with fmt.Printf().
// The level parameter assigns an importance to the message, where 0
// is the most important (such as fatal errors) and increasing numbers
// mark messages of lesser importance. The idea is that a message of 
// level N should only be printed when the program was started with
// N -v (i.e. verbose) switches. So level 0 marks messages that should
// always be logged. Level 1 are informative messages the user may
// not care about. Level 2 and above are typically used for debug
// messages, where level 2 are debug messages that may help the user
// pinpoint a problem and level 3 are debug messages only useful to
// developers. There is usually no need for higher levels.
func Log(level int, format string, args ...interface{}) {
  if (level > LogLevel) { return }
  message := fmt.Sprintf(format, args...)
  t := time.Now()
  output := fmt.Sprintf("%d-%02d-%02d %02d:%02d:%02d %v",
      t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), message)
  Logger.Println(output)
}

// Returns a io.WriteCloser that appends to the file fpath, checking on each
// write if fpath still refers to the same file and if it doesn't re-opens or
// re-creates the file fpath
// to append to. This behaviour is compatible with log-rotation without
// incurring the overhead of re-opening the file on every write.
//
// NOTE: While closing the returned object will close the underlying file
// if it is open, it will not invalidate the object. The next write will
// open the file again (creating it if necessary) and will append to it.
func LogFile(fpath string) io.WriteCloser {
  return &logFile{path:fpath}
}

type logFile struct {
  path string
  file *os.File
  fi os.FileInfo
}

func (f *logFile) Close() error {
  if f.file == nil { return nil }
  err := f.file.Close()
  f.file = nil
  f.fi = nil
  return err
} 

func (f *logFile) Write(p []byte) (n int, err error) {
  if f.file != nil { // if we have an open file
    fi2, err := os.Stat(f.path)
    if err != nil || !os.SameFile(f.fi,fi2) { // if statting the path failed or file has changed => close old and re-open/create
      f.file.Close()
      f.file, err = os.OpenFile(f.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
      if err != nil { return 0,err }
      f.fi, err = f.file.Stat()
      if err != nil { return 0,err }
    }
  } else { // if we don't have an open file => create a new one
    f.file, err = os.OpenFile(f.path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
    if err != nil { return 0,err }
    f.fi, err = f.file.Stat()
    if err != nil { return 0,err }
  }
  
  return f.file.Write(p)
}
