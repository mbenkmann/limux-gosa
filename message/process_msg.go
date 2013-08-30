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
         "net"
         "time"
         "math/rand"
         "strings"
         "crypto/cipher"
         "crypto/aes"
         
         "../db"
         "../xml"
         "../util"
         "../bytes"
         "../config"
       )

// Returns an XML string to return to GOsa if there was an error processing
// a message from GOsa. msg is an error message (will be formatted by Sprintf())
func ErrorReplyXML(msg interface{}) *xml.Hash {
  // Use an XML hash so that msg will be properly escaped if it contains e.g. "<"
  x := xml.NewHash("xml","header","answer")
  x.Add("error_string",msg)
  x.Add("source", config.ServerSourceAddress)
  x.Add("target", "GOSA")
  x.Add("answer1","1")
  return x
}

// Returns ErrorReplyXML(msg).String()
func ErrorReply(msg interface{}) string {
  return ErrorReplyXML(msg).String()
}

// Returns ErrorReply(msg) within a bytes.Buffer THAT NEEDS TO BE FREED BY THE CALLER!!
func ErrorReplyBuffer(msg interface{}) *bytes.Buffer {
  var b bytes.Buffer
  ErrorReplyXML(msg).WriteTo(&b)
  return &b
}

func handleServerMessage() bool {
  if !config.RunServer {
    util.Log(0, "WARNING! Ignoring server message because operating in client-only mode")
  }
  return config.RunServer
}

// Takes a possibly encrypted message in buf and processes it, returning a reply.
// tcpAddr is the address of the message's sender.
// Returns: 
//  buffer containing the reply to return (MUST BE FREED BY CALLER VIA Reset()!)
//  disconnect == true if connection should be terminated due to error
//
// NOTE: buf IS NOT FREED BY THIS FUNCTION BUT ITS CONTENTS ARE CHANGED!
func ProcessEncryptedMessage(buf *bytes.Buffer, tcpAddr *net.TCPAddr) (reply *bytes.Buffer, disconnect bool) {
  if buf.Len() > 4096 {
    util.Log(2, "DEBUG! Processing LONG message: (truncated)%v\n.\n.\n.\n%v", string(buf.Bytes()[0:2048]), string(buf.Bytes()[buf.Len()-2048:]))
  } else {
    util.Log(2, "DEBUG! Processing message: %v", buf.String())
  }
  
  for attempt := 0 ; attempt < 4; attempt++ {
    var keys_to_try []string
    
    switch attempt {
      case 0: keys_to_try = config.ModuleKeys
      case 1: host, _, err := net.SplitHostPort(tcpAddr.String())
              if err != nil {
                util.Log(0, "ERROR! SplitHostPort: %v")
                keys_to_try = []string{}
              } else {
                keys_to_try = append(db.ServerKeys(host), db.ClientKeys(host)...)
                if host == "127.0.0.1" { // make sure we find the key even if registered under our external IP address
                  keys_to_try = append(db.ServerKeys(config.IP), db.ClientKeys(config.IP)...)
                }
              }
      case 2: util.Log(1, "INFO! Last resort attempt to decrypt message from %v with all server keys", tcpAddr)
              keys_to_try = db.ServerKeysForAllServers()
      case 3: util.Log(1, "INFO! Last resort attempt to decrypt message from %v with all client keys", tcpAddr)
              keys_to_try = db.ClientKeysForAllClients()
    }
    
    for _, key := range keys_to_try {
      if GosaDecryptBuffer(buf, key) {
        if buf.Len() > 4096 {
          util.Log(2, "DEBUG! Decrypted LONG message from %v with key %v: (truncated)%v\n.\n.\n.\n%v", tcpAddr, key, string(buf.Bytes()[0:2048]), string(buf.Bytes()[buf.Len()-2048:]))
        } else {
          util.Log(2, "DEBUG! Decrypted message from %v with key %v: %v", tcpAddr, key, buf.String())
        }
        
        // special case for CLMSG_save_fai_log because this kind of message
        // is so large and parsing it to XML doesn't really gain us anything.
        if buf.Contains("<CLMSG_save_fai_log>") {
          if handleServerMessage() {
            clmsg_save_fai_log(buf)
          }
          return &bytes.Buffer{}, false
        }
        
        xml, err := xml.StringToHash(buf.String())
        if err != nil {
          util.Log(0,"ERROR! %v", err)
          return ErrorReplyBuffer(err), true
        } 
        
        // At this point we have successfully decrypted and parsed the message
        return ProcessXMLMessage(xml, tcpAddr, key)
      }
    }
  }
  
  // This part is only reached if none of the keys opened the message
  util.Log(0, "ERROR! Could not decrypt message from %v", tcpAddr)
  
  // Maybe we got out of sync with the sender's encryption key 
  // (e.g. by missing a new_key message). Try to re-establish communcation.
  ip := tcpAddr.IP.To4()
  if ip == nil {
    util.Log(0, "ERROR! Cannot convert sender address to IPv4 address: %v", tcpAddr)
  } else {
    go tryToReestablishCommunicationWith(ip.String())
  }
  
  return ErrorReplyBuffer("Could not decrypt message"), true
}

// Tries to re-establish communication with a client/server at the given IP,
// by 
//   1) sending here_i_am to the server where we are registered. We do this
//      even if config.RunServer (i.e. we are registered at ourselves) because
//      this will trigger new_foreign_client messages sent to peers so that other
//      servers that may believe they own us correct their data.
//   2) sending (if config.RunServer) new_server messages to all known servers
//      we find for the IP in our servers database.
//   3) if config.RunServer and in 2) we did not find a server at that IP,
//      maybe it's a client that thinks we are its server. Send "Müll" to
//      all ClientPorts in that case to cause re-registration.
func tryToReestablishCommunicationWith(ip string) {
  // Wait a little to limit the rate of spam wars between
  // 2 machines that can't re-establish communication (e.g. because of changed
  // keys in server.conf).
  delay := time.Duration(rand.Intn(60))*time.Second

  util.Log(0, "WARNING! Will try to re-establish communication with %v after waiting %v", ip, delay)
  time.Sleep(delay)
  ConfirmRegistration() // 1)
  
  if config.RunServer { // 2)
    sendmuell := true
    for _, server := range db.ServerAddresses() {
      if strings.HasPrefix(server, ip) {
        sendmuell = false
        srv := server
        go util.WithPanicHandler(func(){ Send_new_server("new_server", srv) })
      }
    }
    
    if sendmuell {
      for _, port := range config.ClientPorts {
        addr := ip + ":" + port
        go util.SendLnTo(addr, "Müll", config.Timeout)
      }
    }
  }
}

// Arguments
//   xml: the message
//   tcpAddr: the sender
//   key: the key that successfully decrypted the message
// Returns:
//   reply: buffer containing the reply to return
//   disconnect: true if connection should be terminated due to error
func ProcessXMLMessage(xml *xml.Hash, tcpAddr *net.TCPAddr, key string) (reply *bytes.Buffer, disconnect bool) {
  if key == "dummy-key" && xml.Text("header") != "gosa_ping" {
    util.Log(0, "ERROR! Rejecting non-ping message encrypted with dummy-key or not at all")
    return ErrorReplyBuffer("ERROR! Rejecting non-ping message encrypted with dummy-key or not at all"),true
  }
  
  reply = &bytes.Buffer{}
  disconnect = false
  
  is_client_message := true
  switch xml.Text("header") {
    case "new_ldap_config":     new_ldap_config(xml)
    case "new_ntp_config":      new_foo_config(xml)
    case "registered":          registered(xml)
    case "usr_msg":             usr_msg(xml)
    case "set_activated_for_installation": set_activated_for_installation(xml)
    case "detect_hardware":     detect_hardware(xml)
    case "sistats":             sistats().WriteTo(reply)
    case "panic":               go func(){panic("Panic by user request")}()
    case "trigger_action_halt",      // "Anhalten"
         "trigger_action_localboot", // "Erzwinge lokalen Start"
         "trigger_action_reboot",    // "Neustarten"
         "trigger_action_faireboot", // "Job abbrechen"
         "trigger_action_update",    // "Aktualisieren"
         "trigger_action_reinstall", // "Neuinstallation"
         "trigger_action_instant_update": trigger_action_foo(xml)
  default:                      
    is_client_message = false
  }
  
  is_server_message := !is_client_message
  if is_server_message {
    switch xml.Text("header") {
      case "gosa_ping":                if handleServerMessage() {
                                         reply.WriteString(gosa_ping(xml))
                                         disconnect = true
                                       }
      case "gosa_query_jobdb":         if handleServerMessage() { gosa_query_jobdb(xml).WriteTo(reply) }
      case "gosa_query_fai_server":    if handleServerMessage() { gosa_query_fai_server(xml).WriteTo(reply) }
      case "gosa_query_fai_release":   if handleServerMessage() { gosa_query_fai_release(xml).WriteTo(reply) }
      case "gosa_query_packages_list": if handleServerMessage() { 
                                         pkg := gosa_query_packages_list(xml)
                                         pkg.WriteTo(reply)
                                         pkg.Destroy()
                                       }
      case "gosa_show_log_by_mac":     if handleServerMessage() { gosa_show_log_by_mac(xml).WriteTo(reply) }
      case "gosa_show_log_files_by_date_and_mac": 
                                       if handleServerMessage() { 
                                         gosa_show_log_files_by_date_and_mac(xml).WriteTo(reply)
                                       }
      case "gosa_get_log_file_by_date_and_mac":   
                                       if handleServerMessage() { 
                                         gosa_get_log_file_by_date_and_mac(xml).WriteTo(reply)
                                       }
      case "gosa_get_available_kernel":   
                                       if handleServerMessage() {
                                         gosa_get_available_kernel(xml).WriteTo(reply)
                                       }
      case "new_server":          if handleServerMessage() { new_server(xml) }
      case "confirm_new_server":  if handleServerMessage() { confirm_new_server(xml) }
      case "foreign_job_updates": if handleServerMessage() { foreign_job_updates(xml) }
      case "new_foreign_client":  if handleServerMessage() { new_foreign_client(xml) }
      case "information_sharing": if handleServerMessage() { information_sharing(xml) }
      case "here_i_am":           if handleServerMessage() { here_i_am(xml) }
      case "new_key":             if handleServerMessage() { new_key(xml) }
      case "detected_hardware":   if handleServerMessage() { detected_hardware(xml) }
      case "CLMSG_CURRENTLY_LOGGED_IN": if handleServerMessage() { clmsg_currently_logged_in(xml) }
      case "CLMSG_LOGIN":         if handleServerMessage() { clmsg_login(xml) }
      case "CLMSG_LOGOUT":        if handleServerMessage() {clmsg_logout(xml) }
      case "CLMSG_PROGRESS":      if handleServerMessage() {clmsg_progress(xml) }
      case "CLMSG_TASKDIE":       if handleServerMessage() {clmsg_taskdie(xml) }
      case "CLMSG_GOTOACTIVATION":if handleServerMessage() {clmsg_gotoactivation(xml) }
      case "job_set_activated_for_installation",
           "gosa_set_activated_for_installation":
                                  if handleServerMessage() { gosa_set_activated_for_installation(xml) }
      case "gosa_trigger_action_lock",      // "Sperre"
           "gosa_trigger_action_halt",      // "Anhalten"
           "gosa_trigger_action_localboot", // "Erzwinge lokalen Start"
           "gosa_trigger_action_reboot",    // "Neustarten"
           "gosa_trigger_action_faireboot", // "Job abbrechen"
           "gosa_trigger_action_activate",  // "Sperre aufheben"
           "gosa_trigger_action_update",    // "Aktualisieren"
           "gosa_trigger_action_reinstall", // "Neuinstallation"
           "gosa_trigger_action_wake":      // "Aufwecken"
                                  if handleServerMessage() {
                                    gosa_trigger_action(xml).WriteTo(reply)
                                  }
      case "job_trigger_action_lock",      // "Sperre"
           "job_trigger_action_halt",      // "Anhalten"
           "job_trigger_action_localboot", // "Erzwinge lokalen Start"
           "job_trigger_action_reboot",    // "Neustarten"
           "job_trigger_action_faireboot", // "Job abbrechen"
           "job_trigger_action_activate",  // "Sperre aufheben"
           "job_trigger_action_update",    // "Aktualisieren"
           "job_trigger_action_reinstall", // "Neuinstallation"
           "job_trigger_action_wake":      // "Aufwecken"
                                  if handleServerMessage() {
                                    job_trigger_action(xml).WriteTo(reply)
                                  }
      case "gosa_trigger_activate_new",
           "job_trigger_activate_new":
                                  if handleServerMessage() {
                                    job_trigger_activate_new(xml).WriteTo(reply)
                                  }
      case "gosa_send_user_msg",
           "job_send_user_msg":   if handleServerMessage() { job_send_user_msg(xml).WriteTo(reply) }
      case "trigger_wake":        if handleServerMessage() {
                                    trigger_wake(xml)
                                  }
      
      case "gosa_delete_jobdb_entry":
                                  if handleServerMessage() { gosa_delete_jobdb_entry(xml).WriteTo(reply) }
      case "gosa_update_status_jobdb_entry":
                                  if handleServerMessage() { gosa_update_status_jobdb_entry(xml).WriteTo(reply) }
    default:
          is_server_message = false
    }
  }
  
  if !is_server_message && !is_client_message {
    xml.SetText() // clean up excess top-level whitespace before logging
    util.Log(0, "ERROR! ProcessXMLMessage: Unknown message type '%v'\n=======start message=======\n%v\n=======end message=======", xml.Text("header"), xml.String())
    ErrorReplyXML("Unknown message type").WriteTo(reply)
  }
  
  disconnect = disconnect || reply.Contains("<error_string>")
  GosaEncryptBuffer(reply, key)
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
  return string(util.Base64EncodeString(string(cyphertext)))
}

// Replaces the contents of buf with the base64 representation of the data
// after encryption with the given key. 
// The key is a word as used in gosa-si.conf whose md5sum will be used as 
// the actual AES key. buf is empty, it won't be changed.
func GosaEncryptBuffer(buf *bytes.Buffer, key string) {
  datalen := buf.Len()
  if datalen == 0 { return }
  ciph,_ := aes.NewCipher([]byte(util.Md5sum(key)))
  crypter := cipher.NewCBCEncrypter(ciph, config.InitializationVector)
  cryptpad := (aes.BlockSize - datalen % aes.BlockSize) &^ aes.BlockSize
  cryptlen := cryptpad + datalen
  b64len := ((cryptlen+2)/3)<<2
  for i := datalen; i < b64len; i++ { buf.WriteByte(0) }
  data := buf.Bytes()
  copy(data[b64len-datalen:], data) // move data back
  idx := b64len - cryptlen
  copy(data[idx:], make([]byte, cryptpad)) // insert 0s in front
  crypter.CryptBlocks(data[idx:], data[idx:])
  util.Base64EncodeInPlace(data, idx)
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
  
  cyphertext := util.Base64DecodeString(trimmed, nil)
  
  if len(cyphertext) % aes.BlockSize != 0 { return "" }
    
  aes,_ := aes.NewCipher([]byte(util.Md5sum(key)))
  crypter := cipher.NewCBCDecrypter(aes, config.InitializationVector)
  crypter.CryptBlocks(cyphertext, cyphertext)
  
  for len(cyphertext) > 0 && cyphertext[0] == 0 { 
    cyphertext = cyphertext[1:]
  }
  for len(cyphertext) > 0 && cyphertext[len(cyphertext)-1] == 0 { 
    cyphertext = cyphertext[:len(cyphertext)-1]
  }
  trimmed = strings.TrimSpace(string(cyphertext))
  if strings.HasPrefix(trimmed, "<xml>") { 
    return trimmed 
  }
  
  return ""
}

// Like GosaDecrypt() but operates in-place on buf.
// Returns true if decryption successful and false if not.
// If false is returned, the buffer contents may be destroyed, but only
// if further decryption attempts with other keys would be pointless anyway,
// because of some fatal condition (such as the data not being a multiple of
// the cipher's block size).
func GosaDecryptBuffer(buf *bytes.Buffer, key string) bool {
  buf.TrimSpace()
  
  if buf.Len() < 11 { return false } // minimum length of unencrypted <xml></xml>
  
  data := buf.Bytes()
  if string(data[0:5]) == "<xml>" { return true }
  
  // Fixes the following:
  // * gosa-si bug in the following line:
  //     if( $client_answer =~ s/session_id=(\d+)$// ) {
  //   This leaves the "." before "session_id" which breaks base64
  // * new gosa-si protocol has ";IP:PORT" appended to message 
  //   which also breaks base64
  for semicolon_period := 0; semicolon_period < len(data); semicolon_period++ {
    if data[semicolon_period] == ';' || data[semicolon_period] == '.' { 
      buf.Trim(0, semicolon_period)
      data = buf.Bytes()
      break
    }
  }
  
  aescipher,_ := aes.NewCipher([]byte(util.Md5sum(key)))
  crypter := cipher.NewCBCDecrypter(aescipher, config.InitializationVector)
  
  cryptotest := make([]byte, (((3*aes.BlockSize)+2)/3)<<2)
  n := copy(cryptotest, data)
  cryptotest = cryptotest[0:n]
  cryptotest = util.Base64DecodeInPlace(cryptotest)
  n = (len(cryptotest)/aes.BlockSize) * aes.BlockSize
  cryptotest = cryptotest[0:n]
  crypter.CryptBlocks(cryptotest, cryptotest)
  if !strings.Contains(string(cryptotest),"<xml>") { return false }
  
  data = util.Base64DecodeInPlace(data)
  buf.Trim(0, len(data))
  data = buf.Bytes()
  
  if buf.Len() % aes.BlockSize != 0 { 
    // this condition is fatal => further decryption attempts are pointless
    buf.Reset()
    return false
  }
  
  crypter = cipher.NewCBCDecrypter(aescipher, config.InitializationVector)  
  crypter.CryptBlocks(data, data)
  
  buf.TrimSpace() // removes 0 padding, too
  
  return true
}



