/* Copyright (c) 2013 Matthias S. Benkmann
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this file (originally named buffer.go) and associated documentation files 
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


// Alternative to the Go standard lib's bytes package that avoids the GC problems.
package bytes

/*
#include <stdlib.h>
*/
import "C"
import "unsafe"
import "runtime"
import "errors"

const mAX_GROW_SIZE = 1024*1024
const maxInt = int(^uint(0) >> 1) 

// ErrTooLarge is passed to panic if memory cannot be allocated to store data in a buffer.
var ErrTooLarge = errors.New("bytes.Buffer: too large")

type Buffer struct {
  ptr unsafe.Pointer
  sz int
  capa int
}

// Returns nil if no space is currently allocated for the Buffer, 
// otherwise a raw memory pointer to the buffer space.
func (b *Buffer) Pointer() unsafe.Pointer {
  return b.ptr
}

// Returns the total buffer space (used + unused).
func (b *Buffer) Capacity() int { return b.capa }

// Returns the number of meaningful bytes in the buffer (as opposed to Capacity()).
func (b *Buffer) Len() int { return b.sz }

// grow grows the buffer to guarantee space for n more bytes.
// It returns the index where bytes should be written.
// If the buffer can't grow it will panic with ErrTooLarge.
func (b *Buffer) Grow(n int) int {
  if n > 0 {
    rest := b.capa - b.sz
    if rest < n { 

      rest = n - rest  // minimum number of bytes we need to grow by
        
      // if we're allocating memory for the first time, set a finalizer
      if b.capa == 0 { runtime.SetFinalizer(b, (*Buffer).Reset) }
      growth := b.capa
      if growth > mAX_GROW_SIZE { growth = mAX_GROW_SIZE }
      if rest > growth { growth = rest }
      b.capa += growth
      p := C.realloc(b.ptr, C.size_t(b.capa))
      if p == nil { panic(ErrTooLarge) }
      b.ptr = p
    }
  } else if n < 0 { panic(ErrTooLarge) } // not really too large, but this case is just a precaution.
  
  return b.sz
}

// WriteByte appends the byte c to the buffer. 
// The returned error is always nil, but is included to match bufio.Writer's 
// WriteByte. If the buffer becomes too large, WriteByte will panic with ErrTooLarge.
func (b *Buffer) WriteByte(c byte) error {
  b.Grow(1)
  ((*[maxInt]byte)(b.ptr))[b.sz] = c
  b.sz++
  return nil
}

// Appends the bytes from p to the buffer. Always returns len(p), nil.
// If out of memory occurs trying to grow the buffer, the function will
// panic with ErrTooLarge.
func (b *Buffer) Write(p []byte) (n int, err error) {
  if len(p) == 0 { return 0, nil }
  b.Grow(len(p))
  b.sz += copy(((*[maxInt]byte)(b.ptr))[b.sz:b.capa], p)
  return len(p),nil
}

// Appends the bytes from s to the buffer. Always returns len(s), nil.
// If out of memory occurs trying to grow the buffer, the function will
// panic with ErrTooLarge.
func (b *Buffer) WriteString(s string) (n int, err error) {
  if len(s) == 0 { return 0, nil }
  b.Grow(len(s))
  b.sz += copy(((*[maxInt]byte)(b.ptr))[b.sz:b.capa], s)
  return len(s),nil
}

// Returns a copy of the Buffer contents as string.
func (b *Buffer) String() string {
  return C.GoStringN((*C.char)(b.ptr), C.int(b.sz))
}

// Returns a byte slice that directly accesses the buffer's data.
// WARNING! Appending anything to the buffer invalidates any
// slices obtained via this function. They may end up pointing at
// invalid memory locations.
//
// NOTE: The return value is always a valid slice, even if the
// buffer is empty. The function never returns nil.
func (b *Buffer) Bytes() []byte {
  if b.ptr == nil { return []byte{} }
  return ((*[maxInt]byte)(b.ptr))[0:b.sz]
}

// Return true if the buffer contains the string s. Returns true if s == "".
func (b *Buffer) Contains(s string) bool {
  if s == "" { return true }
  if b.sz == 0 { return false }
  data := ((*[maxInt]byte)(b.ptr))[0:b.sz]
  for i := 0; i <= b.sz - len(s); i++ {
    if data[i] == s[0] {
      k := 0
      for ; k < len(s); k++ {
        if s[k] != data[i+k] { break }
      }
      if k == len(s) { return true }
    }
  }
  return false
}

// Split slices the buffer into all substrings separated by sep and returns
// a slice of the substrings between those separators. The buffer itself is
// unchanged and the strings are copies (of course).
// If sep is empty, this function panics.
func (b *Buffer) Split(sep string) []string {
  if sep == "" { panic("UTF-8 splitting like strings.Split() has is not implemented") }
  if b.sz == 0 { return []string{} }
  
  result := make([]string,0,2)
  buf := ((*[maxInt]byte)(b.ptr))[0:b.sz]
  last_idx := 0
  idx := 0
  ch := sep[0]
  last_possible_idx := len(buf)-len(sep)
  for idx <= last_possible_idx { 
    if buf[idx] == ch {
      for k := 1; k < len(sep); k++ {
        if buf[idx+k] != sep[k] { goto notfound }
      }
      result = append(result, string(buf[last_idx:idx]))
      last_idx = idx + len(sep)
      idx = last_idx - 1
    }
  notfound:
    idx++
  }
  result = append(result, string(buf[last_idx:]))
  return result
}

// Frees the memory held by the Buffer. The Buffer remains valid and is ready
// to take new data.
func (b *Buffer) Reset() {
  C.free(b.ptr)
  b.ptr = nil
  b.sz = 0
  b.capa = 0
  runtime.SetFinalizer(b, nil)
}
