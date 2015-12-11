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
         "net"
         "sync/atomic"
         "time"
         "runtime"
         
         "../db"
         "../xml"
         "../config"
       )

/*
#include <malloc.h>
*/
import "C"


var startTime = time.Now() 

// Sum of the nanoseconds the last 100 requests took.
var RequestProcessingTime int64

type ClientStats struct {
  KnownClients int
  KnownTLSClients int
  MyClients int
  MyClientsUp int32
}

func (s *ClientStats) Accepts(x *xml.Hash) bool {
  if x == nil { return false }
  s.KnownClients++
  keys := x.Get("key")
  if len(keys) > 0 && keys[0] == "" {
    s.KnownTLSClients++
  }
  if x.Text("source") == config.ServerSourceAddress {
    s.MyClients++
    go func(a string, i *int32){
      c, err := net.Dial("tcp", a)
      if err == nil { c.Close(); atomic.AddInt32(i, 1) }
    }(x.Text("client"), &s.MyClientsUp)
  }
  return false
}

// Handles the message "sistats".
// Returns:
//  unencrypted reply
func sistats() *xml.Hash {
  stats := xml.NewHash("xml","header","answer")
  stats.Add("source", config.ServerSourceAddress)
  stats.Add("target", "GOSA")
  answer := stats.Add("answer1")
  
  answer.Add("Version", config.Version)
  answer.Add("Revision", config.Revision)
  answer.Add("Uptime", time.Since(startTime))
  answer.Add("Compiler", runtime.Compiler)
  answer.Add("Go-Version", runtime.Version())
  answer.Add("Architecture", runtime.GOARCH)
  answer.Add("OS", runtime.GOOS)
  answer.Add("NumCPU", runtime.NumCPU())
  answer.Add("NumGoroutine", runtime.NumGoroutine())
  answer.Add("AvgRequestTime", time.Duration((atomic.LoadInt64(&RequestProcessingTime)+50)/100))
  susipeers := 0
  susipeersdown := 0
  nonsusipeers := 0
  nonsusipeersdown := 0
  for _, addr := range db.ServerAddresses() {
    if Peer(addr).Downtime() == 0 {  
      if Peer(addr).IsGoSusi() { susipeers++ } else { nonsusipeers++ }
    } else {
      if Peer(addr).IsGoSusi() { susipeersdown++ } else { nonsusipeersdown++ }
    }
  }
  answer.Add("SusiPeersUp", susipeers)
  answer.Add("SusiPeersDown", susipeersdown)
  answer.Add("NonSusiPeersUp", nonsusipeers)
  answer.Add("NonSusiPeersDown", nonsusipeersdown)
  var clistats ClientStats
  db.ClientsQuery(&clistats)
  time.Sleep(2*time.Second) // give Up checks time to succeed
  answer.Add("KnownClients", clistats.KnownClients)
  answer.Add("KnownTLSClients", clistats.KnownTLSClients)
  up := atomic.LoadInt32(&clistats.MyClientsUp)
  answer.Add("MyClientsUp", up)
  answer.Add("MyClientsDown", clistats.MyClients-int(up))
  
  answer.Add("TotalRegistrations", atomic.LoadInt32(&TotalRegistrations))
  answer.Add("MissedRegistrations", atomic.LoadInt32(&MissedRegistrations))
  
  var m runtime.MemStats
  runtime.ReadMemStats(&m)
  answer.Add("Alloc",m.Alloc)
  answer.Add("TotalAlloc",m.TotalAlloc)
  answer.Add("Sys",m.Sys)
  answer.Add("Lookups",m.Lookups)
  answer.Add("Mallocs",m.Mallocs)
  answer.Add("Frees",m.Frees)
  answer.Add("HeapAlloc",m.HeapAlloc)
  answer.Add("HeapSys",m.HeapSys)
  answer.Add("HeapIdle",m.HeapIdle)
  answer.Add("HeapInuse",m.HeapInuse)
  answer.Add("HeapReleased",m.HeapReleased)
  answer.Add("HeapObjects",m.HeapObjects)
  answer.Add("StackInuse",m.StackInuse)
  answer.Add("StackSys",m.StackSys)
  answer.Add("MSpanInuse",m.MSpanInuse)
  answer.Add("MSpanSys",m.MSpanSys)
  answer.Add("MCacheInuse",m.MCacheInuse)
  answer.Add("MCacheSys",m.MCacheSys)
  answer.Add("BuckHashSys",m.BuckHashSys)
  answer.Add("NextGC",m.NextGC)
  answer.Add("LastGC",m.LastGC)
  answer.Add("PauseTotalNs",m.PauseTotalNs)
  answer.Add("NumGC",m.NumGC)
  answer.Add("EnableGC",m.EnableGC)
  answer.Add("DebugGC",m.DebugGC)
  
  mallinfo := C.mallinfo()
  answer.Add("mallinfo_arena",mallinfo.arena)
  answer.Add("mallinfo_ordblks",mallinfo.ordblks)
  answer.Add("mallinfo_hblks",mallinfo.hblks)
  answer.Add("mallinfo_hblkhd",mallinfo.hblkhd)
  answer.Add("mallinfo_uordblks",mallinfo.uordblks)
  answer.Add("mallinfo_fordblks",mallinfo.fordblks)
  answer.Add("mallinfo_keepcost",mallinfo.keepcost)
  
  return stats
}
