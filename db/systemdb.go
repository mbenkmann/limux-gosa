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

// API for the various databases used by go-susi.
package db

// Returns the common name for the system with the given MAC address.
// The name may or may not include a domain. In fact technically the name
// may be anything.
// Returns "none" if the name could not be determined. Since this is a valid
// system name, you should NOT special case this (e.g. use it to check if
// the system is known).
func SystemNameForMAC(macaddress string) string {
  switch macaddress {
    case "01:02:03:04:05:06": return "systest1"
    case "11:22:33:44:55:66": return "systest2"
    case "77:66:55:44:33:22": return "systest3"
  }
  return "none"
}  
