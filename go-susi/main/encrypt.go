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

// encrypt.go is a command line tool to encrypt messages with the
// encryption scheme used by GOsa and gosa-si.

package main

import ( 
         "fmt"
         "os"
         "io/ioutil"
         
         "../bytes"
         "../message"
       )

const USAGE = 
`encrypt <key> [ <message> ]

Encrypts <message> using AES and prints the result base64-encoded to stdout.
If <message> is not provided on the command line, it will be read from stdin.
<key> is a string that will be used as the basis for the encryption key.
      It is NOT the encryption key itself. The keys found in gosa-si.conf
      for the individual modules can be used to decrypt those modules'
      messages.
`

func main() {
  if len(os.Args) != 3 && len(os.Args) != 2 {
    fmt.Fprintf(os.Stderr, "USAGE: %v",USAGE);
    os.Exit(0);
  }
  
  var input bytes.Buffer
  defer input.Reset()
  if len(os.Args) == 3 {
    input.WriteString(os.Args[2])
  } else {
    buf, err := ioutil.ReadAll(os.Stdin)
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v", err);
      os.Exit(1);
    }
    input.Write(buf)
  }
  message.GosaEncryptBuffer(&input, os.Args[1])
  fmt.Fprintln(os.Stdout, input.String())
}
