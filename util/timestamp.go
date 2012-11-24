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

// Offset that has to be added to a UTC time so that after converting it to Local
// the time is numerically the same. This is a workaround for the problem that you
// can't pass a separate Location to time.Parse() so that if (as is the case with
// siserver timestamps) the string being parsed doesn't contain location information
// the result is always in UTC and using .Local() changes the time.
var local_offset time.Duration
func init() { 
  _, ofs := time.Now().Zone()
  local_offset = -time.Duration(ofs) * time.Second 
}

// Converts t into a timestamp appropriate for use in siserver messages.
// The timestamp is always in local time, even if t is not.
func MakeTimestamp(t time.Time) string {
  return t.Local().Format("20060102150405")
}

// Converts a timestamp as used in siserver messages into a time.Time.
// Returns time.Unix(0,0) if the timestamp is invalid.
func ParseTimestamp(ts string) time.Time {
  t, err := time.Parse("20060102150405", ts)
  if err != nil {
    Log(0, "ERROR! Illegal timestamp: %v", ts)
    return time.Unix(0,0)
  }
  return t.Add(local_offset).Local()
}
