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
         
         "../xml"
         "../util"
         "../config"
       )


// Takes a possibly encrypted message and processes it, returning a reply
// or the empty string if none is necessary/possible.
// tcpAddr is the address of the message's sender.
func ProcessEncryptedMessage(msg string, tcpAddr *net.TCPAddr) (reply string) {
  util.Log(2, "DEBUG! Processing message: %v", msg)
  for _, key := range config.ModuleKeys {
    if decrypted := GosaDecrypt(msg, key); decrypted != "" {
      util.Log(2, "DEBUG! Decrypted message from %v with key %v: %v", tcpAddr, key, decrypted)
      xml, err := xml.StringToHash(decrypted)
      
      if err != nil {
        // something went wrong parsing the XML
      
        util.Log(0,"ERROR! %v", err)
        reply = fallback(msg)
        logFallbackReply(reply, key)
        return reply
      } 
      
      // At this point we have successfully decrypted and parsed the message
      return ProcessXMLMessage(msg, xml, tcpAddr, key)
      
    }
  }
  
  // This part is only reached if none of the keys opened the message
  util.Log(0, "ERROR! Could not decrypt message from %v: %v", tcpAddr, msg)
  reply = fallback(msg)
  logFallbackReply(reply, "")
  return reply
}

// encrypted: the original encrypted message
// xml: the message
// tcpAddr: the sender
// key: the key that successfully decrypted the message
func ProcessXMLMessage(encrypted string, xml *xml.Hash, tcpAddr *net.TCPAddr, key string) string {
  var reply string
  switch xml.Text("header") {
    case "gosa_query_jobdb": reply = gosa_query_jobdb(encrypted, xml)
  default:
          //encrypted := GosaEncrypt(xml.String(), key)  // for testing that we can properly encrypt the message
          reply = fallback(encrypted)
          logFallbackReply(reply, key)
          return reply
  }
  
  return GosaEncrypt(reply, key)
}

// util.Logs the message, decrypted if possible.
// 
// key - a key that might open the message. If it doesn't, the other keys are
// attempted as well.
func logFallbackReply(encrypted string, key string) {
  util.Log(2, "DEBUG! Reply from fallback server: %v", encrypted)
  if decrypted := GosaDecrypt(encrypted, key); decrypted != "" {
    util.Log(2, "DEBUG! Decrypted fallback reply with key %v: %v", key, decrypted)
    return
  }
  
  for _, key = range config.ModuleKeys {
    if decrypted := GosaDecrypt(encrypted, key); decrypted != "" {
      util.Log(2, "DEBUG! Decrypted fallback reply with key %v: %v", key, decrypted)
      return      
    }
  }
  
  // This part is only reached if none of the keys opened the message
  util.Log(2, "DEBUG! Could not decrypt fallback reply: %v", encrypted)
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
func GosaEncrypt(msg string, key string) string {
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
  
  // Workaround for gosa-si bug in the following line:
  // if( $client_answer =~ s/session_id=(\d+)$// ) {
  // This leaves the "." before "session_id" which breaks base64
  trimmed = strings.TrimRight(trimmed, ".")
  
  cyphertext, err := base64.StdEncoding.DecodeString(trimmed)
  if err != nil { fmt.Println(err);return "" }
  
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


