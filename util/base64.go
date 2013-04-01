/* Copyright (C) 2013 Matthias S. Benkmann
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this file (originally named base64.go) and associated documentation files 
 * (the "Software"), to deal
 * in the Software without restriction, including without limitation the rights
 * to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
 * copies of the Software, and to permit persons to whom the Software is furnished
 * to do so, subject to the following conditions:
 * The above copyright notice and this permission notice shall be included in all
 * copies or substantial portions of the Software.
 * 
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
 * SOFTWARE. 
 */

package util

const translate = "|||||||||||||||||||||||||||||||||||||||||||>|>|?456789:;<=|||~|||\x00\x01\x02\x03\x04\x05\x06\a\b\t\n\v\f\r\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19||||?|\x1a\x1b\x1c\x1d\x1e\x1f !\"#$%&'()*+,-./0123|||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||||"

// Decodes the base64-encoded string b64 and returns the result.
// If the input is incomplete, *carry will be used to store partial data of
// the last 4-char block. By passing in this *carry to a future call, the
// decoding can be resumed.
//
// If you pass carry==nil, the bytes from the last block will be output even
// if it is shorter than 4 chars. IOW the function will behave as if there were
// an appropriate number of "=".
//
// Multiple base64-strings can be concatenated and passed together in one call.
// The decoded result will be the concatenation of the 2 decodings. Make sure
// that "=" terminators are used if necessary (see more info below).
//
// After each 4-char block or "=" terminator the
// *carry will be 0 again, so that the next string can be fed into the decoder
// without having to explicitly reset the carry variable.
//
// Non-alphabet characters are ignored. In particular the input string may contain
// arbitrary amounts of whitespace.
//
// Due to the characteristics of base64, the output slice's length is 
// always <=3/4 of the input length. The underlying array reserved by this
// function always has exactly (len(b64)>>2)*3 +3 bytes capacity, but the 
// returned slice has the correct length.
//
// The function can decode both standard encoding (using "+" and "/") and
// URL encoding (which uses "-" and "_").
//
// The function doesn't care about the number of "=" characters. Each
// sequence of one or more "=" characters serves to flush the bytes from
// a partially decoded block. So if you want to concatenate multiple
// base64 strings you can always insert a "=" character between them to
// make sure that the first string doesn't influence the second, even if
// it is not properly padded (or corrupt). You can also feed the string "=" 
// into this function to flush the carry.
func Base64DecodeString(b64 string, carry *int) (decoded []byte) {
  decoded = make([]byte,(len(b64)>>2)*3+3)
  
  var i int
  var o int
  var b int
  var shift int
  if carry != nil {
    shift = *carry
    *carry = 0
    count := shift >> 24
    shift &= 0x00ffffff
  
    switch count {
      case 3: goto char4
      case 2: goto char3
      case 1: goto char2
    }
  }

char1:  
  if i >= len(b64) { goto end_carry0 }
  shift = int(translate[b64[i]])
  i++
  if shift & 0x40 != 0 { goto char1 } // garbage character => skip

char2:  
  if i >= len(b64) { goto end_carry1 }
  b = int(translate[b64[i]])
  i++
  if b & 0x40 != 0 { goto char2 } // garbage character => skip
  shift = (shift << 6) | b

char3:  
  if i >= len(b64) { goto end_carry2 }
  b = int(translate[b64[i]])
  i++
  if b & 0x40 != 0 { goto garbage_or_pad_char3 }
  shift = (shift << 6) | b

char4:  
  if i >= len(b64) { goto end_carry3 }
  b = int(translate[b64[i]])
  i++
  if b & 0x40 != 0 { goto garbage_or_pad_char4 }
  shift = (shift << 6) | b
  
  o += 3
  decoded[o-1] = byte(shift)
  shift >>= 8
  decoded[o-2] = byte(shift)
  shift >>= 8
  decoded[o-3] = byte(shift)
 
  goto char1

garbage_or_pad_char3:
  if b == '|' { goto char3 } // garbage character => skip
  decoded[o] = byte(shift >> 4)
  o++
  goto char1
  
garbage_or_pad_char4:
  if b == '|' { goto char4 } // garbage character => skip
  o += 2
  shift >>= 2
  decoded[o-1] = byte(shift)
  shift >>= 8
  decoded[o-2] = byte(shift)
  goto char1


end_carry3:
  if carry == nil { b = 0; goto garbage_or_pad_char4 }
  shift += 0x01000000
end_carry2:
  if carry == nil { b = 0; goto garbage_or_pad_char3 }
  shift += 0x01000000
end_carry1:
  shift += 0x01000000
  if carry != nil { *carry = shift }
end_carry0:
  return decoded[0:o]
}

// In-place decoder for base64 data. The decoded slice is always
// a sub-slice b64[0:n] of the input. See Base64DecodeString() for details
// about the decoder.
func Base64DecodeInPlace(b64 []byte) (decoded []byte) {
  decoded = b64
  var i int
  var o int
  var b int
  var shift int

char1:  
  if i >= len(b64) { goto end_carry0 }
  shift = int(translate[b64[i]])
  i++
  if shift & 0x40 != 0 { goto char1 } // garbage character => skip

char2:  
  if i >= len(b64) { goto end_carry1 }
  b = int(translate[b64[i]])
  i++
  if b & 0x40 != 0 { goto char2 } // garbage character => skip
  shift = (shift << 6) | b

char3:  
  if i >= len(b64) { goto end_carry2 }
  b = int(translate[b64[i]])
  i++
  if b & 0x40 != 0 { goto garbage_or_pad_char3 }
  shift = (shift << 6) | b

char4:  
  if i >= len(b64) { goto end_carry3 }
  b = int(translate[b64[i]])
  i++
  if b & 0x40 != 0 { goto garbage_or_pad_char4 }
  shift = (shift << 6) | b
  
  o += 3
  decoded[o-1] = byte(shift)
  shift >>= 8
  decoded[o-2] = byte(shift)
  shift >>= 8
  decoded[o-3] = byte(shift)
 
  goto char1

garbage_or_pad_char3:
  if b == '|' { goto char3 } // garbage character => skip
end_carry2:
  decoded[o] = byte(shift >> 4)
  o++
  goto char1
  
garbage_or_pad_char4:
  if b == '|' { goto char4 } // garbage character => skip
end_carry3:
  o += 2
  shift >>= 2
  decoded[o-1] = byte(shift)
  shift >>= 8
  decoded[o-2] = byte(shift)
  goto char1

end_carry1:
end_carry0:
  return decoded[0:o]
}
