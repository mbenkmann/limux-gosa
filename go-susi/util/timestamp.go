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

// Various re-usable utility functions.
package util

import "time"

// Converts t into a timestamp appropriate for use in siserver messages.
// The timestamp loses the time zone information of t. No time zone
// conversion will be performed. IOW "12:00 UTC" and "12:00 PDT" will
// both result in a timestamp that says "yyyymmdd1200ss".
func MakeTimestamp(t time.Time) string {
  return t.Format("20060102150405")
}

// Converts a timestamp as used in siserver messages into a time.Time.
// The returned time will be the time at which the server clock's current
// time converted with MakeTimestamp() is ts. The computation is based on
// the assumption that the server's time zone does not change EXCEPT for
// daylight savings time. IOW on a server running on local time in Berlin
// ParseTimestamp("20140101120000") gives 12:00 CET (winter time) and
// ParseTimestamp("20140601120000") gives 12:00 CEST (summer time).
//
// ParseTimestamp() returns time.Unix(0,0) if the timestamp is invalid.
func ParseTimestamp(ts string) time.Time {
  t, err := time.Parse("20060102150405", ts)
  if err != nil {
    Log(0, "ERROR! Illegal timestamp: %v", ts)
    return time.Unix(0,0)
  }
  
  // The timestamp doesn't contain information about the time zone it
  // is in, so time.Parse() has interpreted it as UTC. In order to
  // properly get a time in local time, we need to first determine
  // the code of the time zone active at the requested time. Then
  // we reparse with that time zone code appended.
  zone,_ := t.Local().Zone()
  ts += zone
  t, err = time.Parse("20060102150405MST", ts)
  if err != nil {
    Log(0, "ERROR! Could not parse with time zone: %v", ts)
    return time.Unix(0,0)
  }
  return t
}
