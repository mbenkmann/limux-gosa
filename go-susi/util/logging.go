/*
Copyright (c) 2013 Matthias S. Benkmann

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
         "fmt"
         "time"
         "sync/atomic"
         
         "../util/deque"
         "../bytes"
       )

// The loggers used by util.Log(). 
var loggers deque.Deque
func init() { LoggerAdd(os.Stderr) }

// When the length of the backlog is N times the Backlog factor, 
// the LogLevel is reduced by N.
var BacklogFactor = 100

// ATOMIC counter for messages suppressed due to BacklogFactor
var missingMessages int32

// logEntry objects are appended via Push() and the worker goroutine processes entries
// starting At(0). If a log entry is suppressed due to the automatic log rate limitting,
// a nil is queued, so that it is at least recorded that there was supposed to be an
// entry.
var backlog deque.Deque

type logEntry struct {
  Timestamp time.Time
  Format string
  Args []interface{}
}

type Flushable interface {
  Flush() error
}

type Syncable interface {
  Sync() error
}

// Only messages with a level <= this number will be printed.
var LogLevel = 0

// Adds w to the beginning of the list of loggers. Note that any logger that blocks
// during Write() will prevent loggers later in the list from receiving data.
// No checking is done to see if w is already in the list.
// If w == nil, nothing happens.
//
// The most efficient loggers are those that buffer data and support a Flush() or Sync()
// operation (e.g. os.File or bufio.Writer). The background task that writes to
// the loggers will call Flush()/Sync() whenever there is no backlog, so
// even if the logger has a large buffer, data will only be delayed if there is
// a backlog of messages.
func LoggerAdd(w io.Writer) {
  if w != nil { loggers.Insert(w) }
}

// Removes all loggers from the queue that are == to w (if any).
// If w == nil, nothing happens.
func LoggerRemove(w io.Writer) {
  if w != nil { loggers.Remove(w) }
}

// Returns the number of currently active loggers (not counting those
// suspended by LoggersSuspend())
func LoggersCount() int {
  count := 0
  for ; loggers.At(count) != nil; count++ {}
  return count
}

// Disables all loggers currently in the list of loggers until
// LoggersRestore() is called. Loggers added later via LoggerAdd() are
// unaffected, so this call can be used to temporarily switch to
// a different set of loggers.
// Multiple LoggersSuspend()/LoggersRestore() pairs may be nested.
func LoggersSuspend() {
  loggers.Insert(nil)
}

// Restores the loggers list at the most recent LoggersSuspend() call.
// Loggers that were deactivated by LoggersSuspend() are reactivated and
// all loggers added after that call are removed.
//
// ATTENTION! If this function is called without LoggersSuspend() having
// been called first, all loggers will be removed.
func LoggersRestore() {
  for loggers.RemoveAt(0) != nil {}
}

// Does not return until all messages that have accrued up to this point
// have been processed and all loggers that are
// Flushable or Syncable have been flushed/synched.
// Messages that are logged while LoggersFlush() is executing are not
// guaranteed to be logged.
//
// If you pass maxwait != 0, this function will return after at most
// this duration, even if the logs have not been flushed completely
// up to this point.
func LoggersFlush(maxwait time.Duration) {
  // We push TWO dummy entries. The first one already causes a flush,
  // but we need the second to make sure that WaitForEmpty() does not
  // return until the flush is complete.
  backlog.Push(logEntry{})
  backlog.Push(logEntry{})
  backlog.WaitForEmpty(maxwait)
}

// Outputs a message to all loggers added by LoggerAdd() formatted as
// by fmt.Printf().
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
  level_reduce := backlog.Count()/BacklogFactor
  
  if level > (LogLevel - level_reduce) { 
    atomic.AddInt32(&missingMessages, 1)
    return 
  }
  
  entry := logEntry{Timestamp:time.Now(), Format:format, Args:make([]interface{},len(args))}
  
  for i := range args {
    switch arg := args[i].(type) {
      case string, // for known pass-by-value types, store them directly
           int,uint,uintptr,int8, uint8, int16, uint16, int32, uint32, int64, uint64,
           float32, float64, complex64, complex128,
           time.Time, time.Duration:
           // WARNING! DO NOT ADD []byte or other slices to this case, because
           // the actual logging is done in the background so that the data
           // in the array underlying the slice may have changed when the slice
           // is eventually logged.
        entry.Args[i] = arg
      case io.WriterTo: // special case for *xml.Hash, because it's more efficient to use WriteTo()
        buf := new(bytes.Buffer)
        _, err := arg.WriteTo(buf)
        if err != nil {
          buf.Reset()
          entry.Args[i] = fmt.Sprintf("%v", arg)
        } else {
          entry.Args[i] = buf
        }
      default: // for unknown types, transform them to a string with %v format
        entry.Args[i] = fmt.Sprintf("%v", arg)
    }
  }
  
  backlog.Push(entry)
}

// infinite loop that processes backlog and writes it to all loggers.
func writeLogsLoop() {
  for {
    if backlog.IsEmpty() { 
      m := atomic.LoadInt32(&missingMessages)
      if m > 0 {
        writeLogEntry(logEntry{Timestamp:time.Now(), Format:"%d %s", Args:[]interface{}{m,"missing message(s)"}})
        atomic.AddInt32(&missingMessages, -m)
      }
      flushLogs() 
    }
    
    entry := backlog.Next().(logEntry)
    if entry.Args == nil { flushLogs() } else { writeLogEntry(entry) }
  } 
}
func init() { go writeLogsLoop() }

// Writes entry to all loggers.
func writeLogEntry(entry logEntry) {
  buf := new(bytes.Buffer)
  defer buf.Reset()
  
  t := entry.Timestamp
  fmt.Fprintf(buf, "%d-%02d-%02d %02d:%02d:%02d ",
      t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
  
  fmt.Fprintf(buf, entry.Format, entry.Args...)
  
  // free all buffers created by Log()
  for i := range entry.Args {
    if b, isbuf := entry.Args[i].(*bytes.Buffer); isbuf {
      b.Reset()
    }
  }
  
  buf.WriteByte('\n')
  writeToAllLogs(buf.Bytes())
}

// Writes data to all elements of loggers up to the first nil entry (which is
// a mark inserted by LoggersSuspend().
func writeToAllLogs(data []byte) {
  for i:=0; i < loggers.Count(); i++ {
    logger := loggers.At(i)
    if logger == nil { break }
    WriteAll(logger.(io.Writer), data)
  }
}


// Calls Flush() for all loggers that are Flushable and Sync() for all loggers
// that are Syncable (unless they are also Flushable).
// The loggers list is processed up to the first nil entry
// (see writeToAlllogs).
func flushLogs() {
  for i:=0; i < loggers.Count(); i++ {
    logger := loggers.At(i)
    if logger == nil { break }
    if flush, flushable := logger.(Flushable); flushable {
      flush.Flush()
    } else if syn, syncable := logger.(Syncable); syncable {
      syn.Sync()
    }
  }
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
