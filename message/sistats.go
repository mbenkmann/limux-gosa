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
         "time"
         "runtime"
         
         "../xml"
         "../config"
       )

/*
#include <malloc.h>
*/
import "C"


var startTime = time.Now() 

// Handles the message "sistats".
// Returns:
//  unencrypted reply
func sistats() string {
  stats := xml.NewHash("xml","header","answer")
  stats.Add("source", config.ServerSourceAddress)
  stats.Add("target", "GOSA")
  answer := stats.Add("answer1")
  
  answer.Add("Version", config.Version)
  answer.Add("Revision", config.Revision)
  answer.Add("Uptime").SetText("%v", time.Since(startTime))
  answer.Add("Compiler", runtime.Compiler)
  answer.Add("Go-Version", runtime.Version())
  answer.Add("Architecture", runtime.GOARCH)
  answer.Add("OS", runtime.GOOS)
  answer.Add("NumCPU").SetText("%v",runtime.NumCPU())
  answer.Add("NumGoroutine").SetText("%v",runtime.NumGoroutine())
  var m runtime.MemStats
  runtime.ReadMemStats(&m)
  answer.Add("Alloc").SetText("%v",m.Alloc)
  answer.Add("TotalAlloc").SetText("%v",m.TotalAlloc)
  answer.Add("Sys").SetText("%v",m.Sys)
  answer.Add("Lookups").SetText("%v",m.Lookups)
  answer.Add("Mallocs").SetText("%v",m.Mallocs)
  answer.Add("Frees").SetText("%v",m.Frees)
  answer.Add("HeapAlloc").SetText("%v",m.HeapAlloc)
  answer.Add("HeapSys").SetText("%v",m.HeapSys)
  answer.Add("HeapIdle").SetText("%v",m.HeapIdle)
  answer.Add("HeapInuse").SetText("%v",m.HeapInuse)
  answer.Add("HeapReleased").SetText("%v",m.HeapReleased)
  answer.Add("HeapObjects").SetText("%v",m.HeapObjects)
  answer.Add("StackInuse").SetText("%v",m.StackInuse)
  answer.Add("StackSys").SetText("%v",m.StackSys)
  answer.Add("MSpanInuse").SetText("%v",m.MSpanInuse)
  answer.Add("MSpanSys").SetText("%v",m.MSpanSys)
  answer.Add("MCacheInuse").SetText("%v",m.MCacheInuse)
  answer.Add("MCacheSys").SetText("%v",m.MCacheSys)
  answer.Add("BuckHashSys").SetText("%v",m.BuckHashSys)
  answer.Add("NextGC").SetText("%v",m.NextGC)
  answer.Add("LastGC").SetText("%v",m.LastGC)
  answer.Add("PauseTotalNs").SetText("%v",m.PauseTotalNs)
  answer.Add("NumGC").SetText("%v",m.NumGC)
  answer.Add("EnableGC").SetText("%v",m.EnableGC)
  answer.Add("DebugGC").SetText("%v",m.DebugGC)
  
  mallinfo := C.mallinfo()
  answer.Add("mallinfo_arena").SetText("%v",mallinfo.arena)
  answer.Add("mallinfo_ordblks").SetText("%v",mallinfo.ordblks)
  answer.Add("mallinfo_hblks").SetText("%v",mallinfo.hblks)
  answer.Add("mallinfo_hblkhd").SetText("%v",mallinfo.hblkhd)
  answer.Add("mallinfo_uordblks").SetText("%v",mallinfo.uordblks)
  answer.Add("mallinfo_fordblks").SetText("%v",mallinfo.fordblks)
  answer.Add("mallinfo_keepcost").SetText("%v",mallinfo.keepcost)
  
  return stats.String()
}
