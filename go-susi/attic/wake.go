/*
Copyright (c) 2012 Landeshauptstadt München
Author: Matthias S. Benkmann

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

package main

import ( 
         "fmt"
         "os"
         "net"
         "os/exec"
         "strings"
         
         "github.com/mbenkmann/golib/util"
         "../xml"
       )

const USAGE = 
`wake <MAC>...

Wakes up the machines listed on the command line.
`

func main() {
  if len(os.Args) < 2 {
    fmt.Fprintf(os.Stderr, "USAGE: %v",USAGE);
    os.Exit(0);
  }
  
  for i:=1 ; i < len(os.Args); i++ {
    hwaddr, err := net.ParseMAC(os.Args[i])
    if err != nil {
      util.Log(0, "ERROR! ParseMAC: %v", err)
      continue
    }
    name := PlainnameForMAC(os.Args[i])
    udpaddr,err := net.ResolveUDPAddr("udp4",name+":40000")
    if err != nil {
      util.Log(0, "ERROR! ResolveUDPAddr: %v", err)
      continue
    }
    udpaddr.IP[len(udpaddr.IP)-1] = 255
    udpconn,err := net.DialUDP("udp", nil, udpaddr)
    if err != nil {
      util.Log(0, "ERROR! DialUDP: %v", err)
      continue
    }
    
    payload := []byte{0xff,0xff,0xff,0xff,0xff,0xff}
    for i := 0; i < 16; i++ { payload = append(payload, hwaddr...) }
    _, err = udpconn.Write(payload)
    if err != nil {
      util.Log(0, "ERROR! Write: %v", err)
      continue
    }
  }    
}

func PlainnameForMAC(macaddress string) string {
  system, err := xml.LdifToHash("", true, LdapSearch(fmt.Sprintf("(&(objectClass=GOhard)(macAddress=%v)%v)",macaddress, ""),"cn"))
  name := system.Text("cn")
  if name == "" {
    util.Log(0, "ERROR! Error getting cn for MAC %v: %v", macaddress, err)
    return "none"
  }
  
  // return only the name without the domain
  return strings.SplitN(name, ".", 2)[0]
}  

func LdapSearch(query string, attr... string) *exec.Cmd {
  args := []string{"-x", "-LLL"}
  args = append(args, query)
  args = append(args, attr...)
  return exec.Command("ldapsearch", args...)
}
