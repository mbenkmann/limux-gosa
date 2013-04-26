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

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, 
MA  02110-1301, USA.
*/

package message

import (
         "io"
         "os"
         "fmt"
         "path"
         "strings"
         
         "../xml"
         "../util"
         "../bytes"
         "../config"
       )

// Handles the message "gosa_get_log_file_by_date_and_mac".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_get_log_file_by_date_and_mac(xmlmsg *xml.Hash) *xml.Hash {
  macaddress := xmlmsg.Text("mac")
  lmac := strings.ToLower(macaddress)
  subdir := xmlmsg.Text("date")
  log_file := xmlmsg.Text("log_file")
  
  if !macAddressRegexp.MatchString(macaddress) {
    emsg := fmt.Sprintf("Illegal or missing <mac> element in message: %v", xmlmsg)
    util.Log(0, "ERROR! %v", emsg)
    return ErrorReplyXML(emsg)
  }
  
  // As a precaution, make sure subdir and log_file contain no slashes.
  subdir = strings.Replace(subdir,"/","_",-1)
  log_file = strings.Replace(log_file,"/","_",-1)
  
  if subdir == "" {
    emsg := fmt.Sprintf("Missing or empty <date> element in message: %v", xmlmsg)
    util.Log(0, "ERROR! %v", emsg)
    return ErrorReplyXML(emsg)
  }
  
  if log_file == "" {
    emsg := fmt.Sprintf("Missing or empty <log_file> element in message: %v", xmlmsg)
    util.Log(0, "ERROR! %v", emsg)
    return ErrorReplyXML(emsg)
  }
  
  f, err := os.Open(path.Join(config.FAILogPath, lmac, subdir, log_file)) 
  if err != nil {
    emsg := fmt.Sprintf("gosa_get_log_file_by_date_and_mac: %v", err)
    util.Log(0, "ERROR! %v", emsg)
    return ErrorReplyXML(emsg)
  }
  defer f.Close()
  
  var b bytes.Buffer
  defer b.Reset()
  buffy := make([]byte,65536)
  for {
    n, err := f.Read(buffy)
    b.Write(buffy[0:n])
    if err == io.EOF { break }
    if err != nil {
      emsg := fmt.Sprintf("gosa_get_log_file_by_date_and_mac: %v", err)
      util.Log(0, "ERROR! %v", emsg)
      return ErrorReplyXML(emsg)
    }
    if n == 0 {
      util.Log(0, "WARNING! Read returned 0 bytes but no error. Assuming EOF")
      break
    }
  }
  
  idx := (((b.Len()+2)/3)<<2)-b.Len()
  b.Write0(idx)
  data := b.Bytes()
  copy(data[idx:], data)
  data = util.Base64EncodeInPlace(data, idx)
 
  header := "get_log_file_by_date_and_mac"
  x := xml.NewHash("xml","header", header)
  x.Add(header)
  data_element := x.Add(log_file)
  
  // To reduce memory leak potential, we append in pieces rather than as one large string
  end := xml.MaxFragmentLength
  for ; end < len(data); end += xml.MaxFragmentLength {
    data_element.AppendString(string(data[end-xml.MaxFragmentLength:end]))
  }
  data_element.AppendString(string(data[end-xml.MaxFragmentLength:]))
  
  x.Add("source", config.ServerSourceAddress)
  x.Add("target", "GOSA")
  x.Add("session_id","1")
  return x
}
