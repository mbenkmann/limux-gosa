/*
Copyright (c) 2016 Landeshauptstadt München
Author: Matthias S. Benkmann

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.
*/


// Access controls, TLS, encryption, connection limits,...
package security

import (
         "net"
         "sync"
         "time"
         "strings"

         "github.com/mbenkmann/golib/util"
       )

/*
  Checks if a connection from addr is permitted by the connection
  limits. If yes, the new connection is registered and true is returned.
  In this is the caller MUST ensure that ConnectionLimitsDeregister() is
  called for the same addr when the connection is closed.
  If the connection is not permitted, this function returns false and
  the caller should close the connection immediately. In this case
  the ConnectionLimitsDeregister() MUST NOT be called.
  
  addr must be an IP address or this function will return false.
*/
func ConnectionLimitsRegister(addr net.Addr) bool {
  ip := net.ParseIP(strings.Split(addr.String(),":")[0])
  if ip == nil {
    util.Log(0, "ERROR! [SECURITY] ConnectionLimitsRegister() called with invalid address: %v", addr)
    return false
  }
  
  // We manage 256 bins that can be individually locked to
  // avoid creating a bottleneck. The bin is chosen based on the
  // least significant byte in the IP address.
  bin := int(ip[len(ip)-1])
  
  ipstr := string(ip.To16()) // To16 for normalization of IPv4 addresses
  now := time.Now()
  ago1h := now.Add(-1*time.Hour)
  ago2h := now.Add(-2*time.Hour)
  ago30min := now.Add(-30*time.Minute)
  
  limiters[bin].mutex.Lock()
  defer limiters[bin].mutex.Unlock()
  
  if limiters[bin].limits == nil {
    limiters[bin].limits = map[string]*limits{ipstr:&limits{first:now,last:now}}
  }
  
  lim := limiters[bin].limits[ipstr]
  if lim == nil {
    lim = &limits{first:now,last:now}
    limiters[bin].limits[ipstr] = lim
  }
  
  // if last connection attempt is more than 1h ago, reset time limits
  if lim.last.Before(ago1h) {
    lim.last = now
    lim.first = now
    lim.attempts = 0
  }
  
  if lim.first.After(ago2h) { // if first connection attempt less than 2h ago
    lim.last = now            // use normal computation attempts/(last-first)
    lim.attempts++
  } else { // if first connection attempt more than 2h ago => fix up times to avoid skewed result (imagine 1 year between first and last connection; connPerHour would always be 0)
    lim.attempts = int64((30*time.Minute*time.Duration(lim.attempts))/lim.last.Sub(lim.first)) // last-first is always > 0 because we can only get into this branch if last is less than 1h ago but first is more than 2h ago
    lim.first = ago30min
    lim.last = now
  }
  
  if lim.maxactive > 0 && lim.active >= lim.maxactive {
    return false
  }
  
  delta := lim.last.Sub(lim.first)
  if lim.maxPerHour > 0 && delta > 0 && (time.Duration(lim.attempts)*time.Hour)/delta > time.Duration(lim.maxPerHour) {
    return false
  }
  
  lim.active++
  return true
}

/*
  Call this function AFTER closing the connection to addr to
  decrement the counter of parallel connections for that address.
  This function MUST be called if ConnectionLimitsRegister(addr) has
  returned true and MUST NOT be called if it has returned false.
  
  addr must be an IP address.
*/
func ConnectionLimitsDeregister(addr net.Addr) {
  ip := net.ParseIP(strings.Split(addr.String(),":")[0])
  if ip == nil {
    util.Log(0, "ERROR! [SECURITY] ConnectionLimitsDeregister() called with invalid address: %v", addr)
    return
  }
  bin := int(ip[len(ip)-1])
  ipstr := string(ip.To16()) // To16 for normalization of IPv4 addresses
  
  limiters[bin].mutex.Lock()
  defer limiters[bin].mutex.Unlock()
  
  var lim *limits
  if limiters[bin].limits != nil {
    lim = limiters[bin].limits[ipstr]
  }
  
  if lim == nil {
    util.Log(0, "ERROR! [SECURITY] ConnectionLimitsDeregister() called for unknown address: %v", addr)
    return
  }
  
  lim.active--
}

/*
  This updates the number of parallel connections and connections
  per hour limits that are used by ConnectionLimitsRegister() to
  check if a connection from context.PeerID.IP is permitted.
*/
func ConnectionLimitsUpdate(context *Context) {
  ip := context.PeerID.IP
  bin := int(ip[len(ip)-1])
  ipstr := string(ip.To16()) // To16 for normalization of IPv4 addresses
  limiters[bin].mutex.Lock()
  defer limiters[bin].mutex.Unlock()
  
  var lim *limits
  if limiters[bin].limits != nil {
    lim = limiters[bin].limits[ipstr]
  }
  
  if lim == nil {
    util.Log(0, "ERROR! [SECURITY] ConnectionLimitsUpdate() called for unknown address: %v", ip)
    return
  }
  
  lim.maxactive = int64(context.Limits.ConnParallel)
  lim.maxPerHour = int64(context.Limits.ConnPerHour)
}

type limits struct {
  first time.Time
  last time.Time
  attempts int64
  active int64
  maxactive int64
  maxPerHour int64
}

type limiter struct {
  mutex sync.Mutex
  limits map[string]*limits
}

var limiters [256]limiter
