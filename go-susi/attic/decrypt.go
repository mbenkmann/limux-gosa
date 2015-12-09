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

// decrypt.go is a command line tool to decrypt messages with the
// encryption scheme used by GOsa and gosa-si.

package main

import ( 
         "fmt"
         "os"
         "../message"
       )

const USAGE = 
`decrypt <key> <message>

Attempts to decrypt <message> using AES with the provided key. 
If <message> is unencrypted and starts with "<xml>" it will be left as is
(except that whitespace will be trimmed).
<key> is a string that will be used as the basis for the encryption key.
      It is NOT the encryption key itself. The keys found in gosa-si.conf
      for the individual modules can be used to decrypt those modules'
      messages.
`

func main() {
  if len(os.Args) != 3 {
    fmt.Fprintf(os.Stderr, "USAGE: %v", USAGE);
    os.Exit(0);
  }
  
  msg := security.GosaDecrypt(os.Args[2], os.Args[1])
  if msg == "" { 
    fmt.Fprintln(os.Stderr, "Cannot decrypt message with the provided key")
    os.Exit(1)
  }
  
  fmt.Fprintln(os.Stdout, msg)
}
