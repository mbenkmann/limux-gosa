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

// The handling code for the many messages passed around between 
// GOsa, gosa-si and go-susi.
package message

import ( 
         "fmt"
         "net"
         "bytes"
         "strings"
         "crypto/cipher"
         "crypto/aes"
         "encoding/base64"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

// Returns an XML string to return to GOsa if there was an error processing
// a message from GOsa. msg is an error message (will be formatted by Sprintf())
func ErrorReply(msg interface{}) string {
  // Use an XML hash so that msg will be properly escaped if it contains e.g. "<"
  x := xml.NewHash("foo")
  x.SetText("%v", msg)
  return fmt.Sprintf("<xml><header>answer</header><source>%v</source><target>GOSA</target><answer1>1</answer1><error_string>%v</error_string></xml>", config.ServerSourceAddress, x.Text())
}

// Takes a possibly encrypted message and processes it, returning a reply
// or the empty string if none is necessary/possible.
// tcpAddr is the address of the message's sender.
// Returns: 
//  reply to return
//  disconnect == true if connection should be terminated due to error
func ProcessEncryptedMessage(msg string, tcpAddr *net.TCPAddr) (reply string, disconnect bool) {
  util.Log(2, "DEBUG! Processing message: %v", msg)
  
  for attempt := 0 ; attempt < 3; attempt++ {
    var keys_to_try []string
    
    switch attempt {
      case 0: keys_to_try = config.ModuleKeys
      case 1: host, _, err := net.SplitHostPort(tcpAddr.String())
              if err != nil {
                util.Log(0, "ERROR! SplitHostPort: %v")
                keys_to_try = []string{}
              } else {
                keys_to_try = db.ServerKeys(host)
              }
      case 2: util.Log(1, "INFO! Last resort attempt to decrypt message from %v with all server keys", tcpAddr)
              keys_to_try = db.ServerKeysForAllServers()
    }
    
    for _, key := range keys_to_try {
      if decrypted := GosaDecrypt(msg, key); decrypted != "" {
        util.Log(2, "DEBUG! Decrypted message from %v with key %v: %v", tcpAddr, key, decrypted)
        xml, err := xml.StringToHash(decrypted)
        if err != nil {
          util.Log(0,"ERROR! %v", err)
          return ErrorReply(err), true
        } 
        
        // At this point we have successfully decrypted and parsed the message
        return ProcessXMLMessage(msg, xml, tcpAddr, key)
      }
    }
  }
  
  // This part is only reached if none of the keys opened the message
  util.Log(0, "ERROR! Could not decrypt message from %v: %v", tcpAddr, msg)
  return ErrorReply("Could not decrypt message"), true
}

// Arguments
//   encrypted: the original encrypted message
//   xml: the message
//   tcpAddr: the sender
//   key: the key that successfully decrypted the message
// Returns:
//   reply: reply to return
//   disconnect: true if connection should be terminated due to error
func ProcessXMLMessage(encrypted string, xml *xml.Hash, tcpAddr *net.TCPAddr, key string) (reply string, disconnect bool) {
  if key == "dummy-key" && xml.Text("header") != "gosa_ping" {
    util.Log(0, "ERROR! Rejecting non-ping message encrypted with dummy-key or not at all")
    return ErrorReply("ERROR! Rejecting non-ping message encrypted with dummy-key or not at all"),true
  }
  
  reply = ""
  disconnect = false
  switch xml.Text("header") {
    case "gosa_query_jobdb":    reply = gosa_query_jobdb(xml)
    case "new_server":          new_server(xml)
    case "confirm_new_server":  confirm_new_server(xml)
    case "foreign_job_updates": foreign_job_updates(xml)
    case "new_foreign_client":  new_foreign_client(xml)
    case "here_i_am":           here_i_am(xml)
    case "gosa_trigger_action_lock",      // "Sperre"
         "gosa_trigger_action_halt",      // "Anhalten"
         "gosa_trigger_action_localboot", // "Erzwinge lokalen Start"
         "gosa_trigger_action_reboot",    // "Neustarten"
         "gosa_trigger_action_activate",  // "Sperre aufheben"
         "gosa_trigger_action_update",    // "Aktualisieren"
         "gosa_trigger_action_reinstall", // "Neuinstallation"
         "gosa_trigger_action_wake":      // "Aufwecken"
                                reply = gosa_trigger_action(xml)
    case "job_trigger_action_lock",      // "Sperre"
         "job_trigger_action_halt",      // "Anhalten"
         "job_trigger_action_localboot", // "Erzwinge lokalen Start"
         "job_trigger_action_reboot",    // "Neustarten"
         "job_trigger_action_activate",  // "Sperre aufheben"
         "job_trigger_action_update",    // "Aktualisieren"
         "job_trigger_action_reinstall", // "Neuinstallation"
         "job_trigger_action_wake":      // "Aufwecken"
                                reply = job_trigger_action(xml)
    case "trigger_wake":
                                trigger_wake(xml)
    
    case "gosa_delete_jobdb_entry":
                                reply = gosa_delete_jobdb_entry(xml)
    case "gosa_update_status_jobdb_entry":
                                reply = gosa_update_status_jobdb_entry(xml)
    case "sistats":             reply = sistats()
    case "panic":               go func(){panic("Panic by user request")}()
  default:
        util.Log(0, "ERROR! ProcessXMLMessage: Unknown message type '%v'", xml.Text("header"))
        reply = ErrorReply("Unknown message type")
  }
  
  disconnect = strings.Contains(reply, "<error_string>")
  reply = GosaEncrypt(reply, key)
  return
}

// Returns a byte slice that has the input string's bytes preceded
// by up to (aes.BlockSize-1) 0-bytes so that the slice's length is 
// a multiple of aes.BlockSize.
func paddedMessage(msg string) []byte {
  padding := (aes.BlockSize - len(msg) % aes.BlockSize) &^ aes.BlockSize
  buf := make([]byte, len(msg) + padding)
  copy(buf[padding:], msg)
  return buf
}

// Returns the base64 representation of the message after encryption with
// the given key. The key is a word as used in gosa-si.conf whose md5sum will
// be used as the actual AES key.
// If msg == "", "" will be returned.
func GosaEncrypt(msg string, key string) string {
  if msg == "" { return "" }
  aes,_ := aes.NewCipher([]byte(util.Md5sum(key)))
  crypter := cipher.NewCBCEncrypter(aes, config.InitializationVector)
  cyphertext := paddedMessage(msg)
  crypter.CryptBlocks(cyphertext, cyphertext)
  return base64.StdEncoding.EncodeToString(cyphertext)
}

// Tries to decrypt msg with the given key and returns the decrypted message or
// the empty string if decryption failed. Decryption will be considered successful
// if the decrypted message starts with "<xml>" (after trimming whitespace).
//
// Whitespace will be trimmed at the start and end of msg before decryption.
//
// msg can be one of the following:
//
// * an unencrypted message starting (after trimming) with "<xml>". It will be
// returned trimmed but otherwise unchanged.
//
// * a base64 string as returned by GosaEncrypt when used with the same key.
// The unencrypted message will be returned.
// 
// The key is a word as used in gosa-si.conf whose md5sum will
// be used as the actual AES key.
func GosaDecrypt(msg string, key string) string {
  trimmed := strings.TrimSpace(msg)
  
  if strings.HasPrefix(trimmed, "<xml>") { 
    return trimmed 
  }
  
  // Fixes the following:
  // * gosa-si bug in the following line:
  //     if( $client_answer =~ s/session_id=(\d+)$// ) {
  //   This leaves the "." before "session_id" which breaks base64
  // * new gosa-si protocol has ";IP:PORT" appended to message 
  //   which also breaks base64
  semicolon_period := strings.IndexAny(trimmed, ";.")
  if semicolon_period >= 0 { trimmed = trimmed[:semicolon_period] }
  
  cyphertext, err := base64.StdEncoding.DecodeString(trimmed)
  if err != nil { return "" }
  
  if len(cyphertext) % aes.BlockSize != 0 { return "" }
    
  aes,_ := aes.NewCipher([]byte(util.Md5sum(key)))
  crypter := cipher.NewCBCDecrypter(aes, config.InitializationVector)
  crypter.CryptBlocks(cyphertext, cyphertext)
  
  cyphertext = bytes.Trim(cyphertext, "\u0000")
  trimmed = strings.TrimSpace(string(cyphertext))
  if strings.HasPrefix(trimmed, "<xml>") { 
    return trimmed 
  }
  
  return ""
}


