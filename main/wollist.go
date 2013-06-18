/*
Copyright (c) 2013 Landeshauptstadt München
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


package main

/*
#include <stdlib.h>
#include <pcap/pcap.h>
#cgo LDFLAGS: -lpcap
*/
import "C"

import "unsafe"
import "fmt"

var pcap_filter = C.CString("udp port 40000")

func main() {
  var errbuf = (*C.char)(C.malloc(C.PCAP_ERRBUF_SIZE))
  defer C.free(unsafe.Pointer(errbuf))
  var source = C.CString("any")
  defer C.free(unsafe.Pointer(source))
  
  pcap_handle := C.pcap_create(source, errbuf);
  if pcap_handle == nil { panic("pcap_handle") }
  C.pcap_set_buffer_size(pcap_handle, 2*1024*1024)
  C.pcap_set_promisc(pcap_handle, 1)
  C.pcap_set_snaplen(pcap_handle, 512) // more than enough to recognize a WOL packet
  C.pcap_setdirection(pcap_handle, C.PCAP_D_IN)
  if C.pcap_activate(pcap_handle) != 0 { panic(C.GoString(C.pcap_geterr(pcap_handle))) }
  
  var bpf_program C.struct_bpf_program
  if C.pcap_compile(pcap_handle, &bpf_program, pcap_filter, 0, 0) != 0 { panic(C.GoString(C.pcap_geterr(pcap_handle))) }
  if C.pcap_setfilter(pcap_handle, &bpf_program) != 0 { panic(C.GoString(C.pcap_geterr(pcap_handle))) }
  
  for {
    var pkt_header *C.struct_pcap_pkthdr
    var pkt_data *C.u_char
    if C.pcap_next_ex(pcap_handle, &pkt_header, &pkt_data) < 0 { panic(C.GoString(C.pcap_geterr(pcap_handle))) }
    if pkt_data == nil { continue }
    data := make([]byte, pkt_header.caplen)
    copy(data, (*(*[10000000]byte)(unsafe.Pointer(pkt_data)))[0:])
    from_mac,to_mac := checkwol(data)
    if from_mac != "" { fmt.Println(from_mac+" sends WOL to "+to_mac) }
  }
  
}

var hex = "0123456789abcdef"

func checkwol(data []byte) (from_mac string, to_mac string) {
  from_mac = ""
  to_mac = ""
  if len(data) < 16*6+6+6 { return }
  from := data[6:12]
  from_ip := data[28:32]
  to_ip:= data[32:36]
  data = data[len(data)-(16*6+6+6):]
  if data[0] != 0x9c { return } // high byte of port 40000
  if data[1] != 0x40 { return } // low byte of port 40000
  for i := 6; i < 12; i++ {
    if data[i] != 0xff { return } // WOL magic
  }
  for i := 1; i < 16; i++ {
    for k := 0; k < 6; k++ {
      if data[i*6+12+k] != data[12+k] { return } // 16 copies of MAC to wake up
    }
  }
  
  from_mac = fmt.Sprintf("%d.%d.%d.%d/",from_ip[0],from_ip[1],from_ip[2],from_ip[3])
  
  for i := 0; i < 6; i++ {
    x := from[i] >> 4
    from_mac += hex[x:x+1]
    x = from[i] & 0xf
    from_mac += hex[x:x+1]
    if i != 5 { from_mac += ":" }
  }
  
  to_mac = fmt.Sprintf("%d.%d.%d.%d/",to_ip[0],to_ip[1],to_ip[2],to_ip[3])
  
  for i := 0; i < 6; i++ {
    x := data[12+i] >> 4
    to_mac += hex[x:x+1]
    x = data[12+i] & 0xf
    to_mac += hex[x:x+1]
    if i != 5 { to_mac += ":" }
  }
  return
}

