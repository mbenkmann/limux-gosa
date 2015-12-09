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

*/

package security

import ( 
         "strings"
         "crypto/cipher"
         "crypto/aes"
         
         "github.com/mbenkmann/golib/util"
         "github.com/mbenkmann/golib/bytes"
         "../config"
       )

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
