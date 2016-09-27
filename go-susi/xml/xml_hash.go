/* Copyright (C) 2012 Matthias S. Benkmann
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this file (originally named xml_hash.go) and associated documentation files 
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

// XML fragments and primitive database based on them.
package xml

import ( 
         "os"
         "os/exec"
         "io"
         "io/ioutil"
         "fmt"
         "sort"
         "strings"
         "unsafe"
         
         "github.com/mbenkmann/golib/bytes"
         "github.com/mbenkmann/golib/util"
       )

import encxml "encoding/xml"

// To prevent memory leaks caused by the garbage collector, the Hash will
// attempt to split up text content of elements into fragments no longer than
// this many bytes. Except where otherwise noted this is transparent to the
// user of the xml.Hash's API.
var MaxFragmentLength = 16384

// An xml.Hash is a representation of simple XML data that follows certain
// restrictions:
//  * no attributes (they can be parsed but will be converted to child elements)
//  * all text children of an element are treated as a single text, even if
//    they are interspersed with tags.
//  * the order of siblings with the same tag name is preserved, but the order
//    of siblings with different tag names is not guaranteed to be.
//    E.g.  "<xml><foo>1</foo><bar>A</bar><foo>2</foo><bar>B</bar></xml>" and
//          "<xml><bar>A</bar><foo>1</foo><foo>2</foo><bar>B</bar></xml>" 
//          are not guaranteed to be distinguishable from each other.
//
// In simpler terms: a Hash maps each element name to an ordered list of all
// child elements with that name.
// 
type Hash struct {
  // Each Hash object can be of 3 types: start tag, end tag and text.
  // For a start tag, data is the tag name without "<" and ">".
  // For an end tag, data is the empty string (and unused).
  // For a text element, data is the text in unescaped form (i.e. "<" instead of "&lt;").
  data string
  // An XML fragment is represented as a doubly linked list of Hash objects.
  // The succ_ pointer points to the successor in the list. Only an end tag may have
  // a nil successor. In that case the matching start tag will have a nil predecessor.
  // This rule implies that each XML fragment has an outermost tag surrounding
  // everything, with no stray text outside.
  succ_ *Hash
  // pred_ has 2 independent functions. Bits 0 and 1 determine the type of Hash:
  // 01 end tag  (returned as -1 by nest())
  // 10 text     (returned as 0 by nest())
  // 11 start tag (returned as 1 by nest())
  // The other bits are the binary negation of the predecessor *Hash pointer which is
  // the counterpart of succ_ in the linked list structure.
  // The reason for this convoluted encoding is to make the pred_ pointer not be
  // apparent as a pointer to the GC, so that as far as the GC is concerned the
  // structure looks like a singly-linked list. Hopefully this will make memory leaks
  // by the GC less likely.
  pred_ uintptr
  // For a start tag this is the binary negation of a *Hash pointer to the corresponding
  // end tag. For an end tag it is the binary negation of a *Hash pointer to the
  // corresponding start tag. For a text element this is 0 and unused.
  // The reason for this convoluted encoding is the same as for pred_.
  link_ uintptr
}

// For a start tag this returns the corresponding end tag. For an end tag this
// returns the corresponding start tag. This function must not be called for a
// text element.
func (self *Hash) link() *Hash { return (*Hash)(unsafe.Pointer(^self.link_)) }
// See Hash.succ_.
func (self *Hash) succ() *Hash { return self.succ_ }
// See Hash.pred_. This functions returns the extracted and decoded pointer.
func (self *Hash) pred() *Hash { return (*Hash)(unsafe.Pointer(^self.pred_ &^ 3)) }
// Returns the relative nesting change by the element. +1 => start tag, 0 => text,
// -1 => end tag.
func (self *Hash) nest() int { return int(self.pred_ & 3)-2 }
// Sets the relative nesting. Only for initializing a new Hash. Never changes over
// the lifetime of a Hash.
func (self *Hash) setNest(n int) { self.pred_ = (self.pred_ &^ 3) | uintptr((n+2)&3) }
// Sets the pred() pointer.
func (self *Hash) setPred(p *Hash) { self.pred_ = (self.pred_ & 3) | (^uintptr(unsafe.Pointer(p)) &^ 3) }
// Sets the succ() pointer.
func (self *Hash) setSucc(s *Hash) { self.succ_ = s }
// Sets the link() pointer. Only for initializing a new Hash. Never changes over
// the lifetime of a Hash.
func (self *Hash) setLink(l *Hash) { self.link_ = ^uintptr(unsafe.Pointer(l)) }

// Returns a new *Hash with outer-most element <name>.
// If N contents strings are passed, the effect will be
// as if 
// hash.Add(contents[0]).Add(contents[1])...Add(contents[N-1]).SetText(contents[N])
// is called (but the element returned is always the outermost).
func NewHash(name string, contents ...string) *Hash {
  hash := &Hash{data:name, succ_:&Hash{pred_:1}, pred_:3}
  hash.setPred(nil)
  hash.setLink(hash.succ())
  hash.succ().setPred(hash)
  hash.succ().setLink(hash)
  sub := hash
  for i:= 0; i < len(contents)-1; i++ {
    sub = sub.Add(contents[i])
  }
  if len(contents) > 0 {
    sub.SetText(contents[len(contents)-1])
  }
  return hash
}

// Returns the name of the Hash's start and end tags.
func (self *Hash) Name() string { return self.data }

// Changes the name of the Hash's start and end tags.
func (self *Hash) Rename(name string) {
  self.data = name
}

// Returns a deep copy of this xml.Hash that is completely independent.
// The clone will not have any siblings, even if the original had some.
func (self *Hash) Clone() *Hash {
  hash := NewHash(self.data)
  end := self.link()
  for h := self.succ(); h != end; {
    switch h.nest() {
      case 0: hash.AppendString(h.data)
              h = h.succ()
        // this is always case 1 (i.e. start tag)
        // we can never see an end tag here because we skip them
        // in the line below  h = h.link().succ()
      default: hash.AddClone(h)
               h = h.link().succ()
    }
  }
  return hash
}

// Appends s to this Hash's text content.
func (self *Hash) AppendString(s string) {
  if len(s) == 0 { return }
  end := self.link()
  hash := &Hash{data:s, pred_:2, succ_:end}
  hash.setPred(end.pred())
  end.setPred(hash)
  hash.pred().setSucc(hash)
}

// Replaces the current text content of the receiver.
// If called without any args, the receiver's text content will be deleted.
// If called with a single arg that is a string, []string, []byte or [][]byte, the 
// text will be replaced with the literal bytes from that argument.
// If called with a single arg that is of a different type, the text
// will be replaced with the result of fmt.Sprintf("%v",arg).
// If called with 2 or more args, the first one must be a format string F
// and the text will be replaced with the result from fmt.Sprintf(F,args[1:])
func (self *Hash) SetText(args ...interface{}) {
  // delete all old text children
  end := self.link()
  for h := self.succ(); h != end; {
    switch h.nest() {
      case 0: pred := h.pred()
              succ := h.succ()
              h.succ_ = nil
              h.pred_ = 0
              h.data = ""
              pred.setSucc(succ)
              succ.setPred(pred)
              h = succ
        // this is always case 1 (i.e. start tag)
        // we can never see an end tag here because we skip them
        // in the line below  h = h.link().succ()
      default: h = h.link().succ()
    }
  }
  
  // now add the new text (if any)
  if len(args) == 0 {
    return
  } else if len(args) == 1 {
    switch arg := args[0].(type) {
      case string: self.AppendString(arg)
      case []string: for _,st := range arg { self.AppendString(st) }
      case []byte: self.AppendString(string(arg))
      case [][]byte: for _,by := range arg { self.AppendString(string(by)) }
      default: self.AppendString(fmt.Sprintf("%v", arg))
    }
  } else {
    self.AppendString(fmt.Sprintf(args[0].(string), args[1:]...))
  }
}

// Adds a new child element X named subtag and (if any setText parameters are
// supplied) executes X.SetText(setText...).
// Returns the new child X.
func (self *Hash) Add(subtag string, setText... interface{}) *Hash {
  new_hash := NewHash(subtag)
  new_hash.SetText(setText...)
  self.AddWithOwnership(new_hash)
  return new_hash
}

// Adds a clone of xml to this Hash as a child (at the end of the list
// of children with the same element name) and returns the clone.
func (self *Hash) AddClone(xml *Hash) *Hash {
  clone := xml.Clone()
  self.AddWithOwnership(clone)
  return clone
}

// Takes the xml object (not a copy) and integrates it into this Hash
// as a child (at the end of the list of children with the same element name).
// ATTENTION! xml must not be child of another Hash (which implies that it
// must not have any siblings).
func (self* Hash) AddWithOwnership(xml *Hash) {
  if xml == nil || xml == self || xml.pred() != nil || xml.link().succ() != nil {
    panic("AddWithOwnership: Sanity check failed!")
  }
  xmlend := xml.link()
  end := self.link()
  xml.setPred(end.pred())
  xmlend.setSucc(end)
  end.setPred(xmlend)
  xml.pred().setSucc(xml)
}

// Removes the first subtag child from this Hash and returns it (or nil if this
// Hash has no subtag child)
func (self *Hash) RemoveFirst(subtag string) *Hash {
  first := self.First(subtag)
  if first != nil { first.remove() }
  return first
}

// Removes this Hash (including all its children) from its parent.
// After this operation the Hash is an independent top-level Hash.
// ATTENTION! Must not be called on a Hash that has no parent.
func (self *Hash) remove() {
  pred := self.pred()
  end := self.link()
  succ := end.succ()
  
  pred.setSucc(succ)
  succ.setPred(pred)
  
  self.setPred(nil)
  end.setSucc(nil)
}

// Destroys the internal structure of this Hash, nulling out all pointers, to
// help the garbage collector. After calling this function, the Hash and all
// parts of it (children, iterators,...) become invalid and must not be accessed
// anymore.
func (self* Hash) Destroy() {
  h := self
  for {
    nxt := h.succ()
    h.data = ""
    h.link_ = 0
    h.succ_ = nil
    h.pred_ = 0
    if nxt == nil { break }
    h = nxt
  }
  return
}

// Returns the first child element with the tag name subtag or nil if none exists.
// See also FirstOrAdd().
func (self *Hash) First(subtag string) *Hash {
  end := self.link()
  for h := self.succ(); h != end; {
    switch h.nest() {
      case 0: h = h.succ()
        // this is always case 1 (i.e. start tag)
        // we can never see an end tag here because we skip them
        // in the line below  h = h.link().succ()
      default: if h.Name() == subtag { return h }
               h = h.link().succ()
    }
  }
  return nil
}

// Returns the first child named subtag if one exists; otherwise performs
// Add(subtag) and then returns the new child added.
// See also First().
func (self *Hash) FirstOrAdd(subtag string) *Hash {
  ele := self.First(subtag)
  if ele == nil { ele = self.Add(subtag) }
  return ele
}


// Returns the next sibling with the same tag name or nil if none exists.
func (self *Hash) Next() *Hash {
  name := self.Name()
  for h := self.link().succ(); h != nil; {
    switch h.nest() {
      case 0: h = h.succ()
      case 1: if h.Name() == name { return h }
              h = h.link().succ()
      case -1: return nil
    }
  }
  
  return nil
}

// Effectively a special kind of *Hash that allows more flexibility.
// In particular you can call Remove() on an Iterator and its Next() element
// is still available. Furthermore Iterators offer selections and
// orderings of elements other than the one defined by each *Hash's Next().
//
//   NOTE:
//   This Iterator's usage differs from that of typical iterators. The Next()
//   function does not advance the Iterator itself. Instead it returns a new
//   Iterator to the next element in the iteration.
//   This has been done for consistency with *Hash as demonstrated by the following
//   examples:
//
//             // *Hash
//   for child := x.First("foo"); child != nil; child = child.Next() { ... }
//
//            // Iterator
//   for child := x.FirstChild(); child != nil; child = child.Next() { ... }
type Iterator interface {
  // Returns the current element in the iteration, or nil if 
  // the element has been Remove()d.
  //
  // NOTE: A non-nil Iterator's Element is always non-nil unless it has been Remove()d.
  Element() *Hash
  
  // Returns an Iterator to the next element in the iteration or nil if the iteration
  // has reached the end.
  Next() Iterator
  
  // Removes Element() from its parent Hash, so that it is independent and
  // has neither parent nor siblings. Returns the removed element or nil if
  // Element() was already nil.
  Remove() *Hash
}

type iterateAllChildren struct {
  // see Iterator.Element(). After Remove() this is nil and nxt is set.
  cur *Hash
  // Only set after a call to Remove() to store a pointer to the Next() element.
  nxt *Hash
}

func (iter *iterateAllChildren) Element() *Hash {
  return iter.cur
}

func  (iter *iterateAllChildren) Remove() *Hash {
  if iter.cur == nil { return nil }
  nxt := iter.Next()
  if nxt != nil { iter.nxt = nxt.Element() }
  item := iter.cur
  item.remove()
  iter.cur = nil
  return item
}

func (iter *iterateAllChildren) Next() Iterator {
  if iter.cur == nil {
    if iter.nxt == nil { return nil }
    return &iterateAllChildren{cur:iter.nxt, nxt:nil}
  }
  
  nxt := iter.cur.link().succ()
  // skip over text elements
  for nxt.nest() == 0 { nxt = nxt.succ() }
  
  // if we encounter an end element => end of iteration
  if nxt.nest() < 0 { return nil }
  
  return &iterateAllChildren{cur:nxt, nxt:nil}
}


// Returns nil if this Hash has no child elements; otherwise returns an Iterator
// to the first child element within an iteration over all immediate child elements
// (i.e. not including grandchildren) in some unspecified order.
func (self *Hash) FirstChild() Iterator {
  nxt := self.succ()
  // skip over text elements
  for nxt.nest() == 0 { nxt = nxt.succ() }
  
  // if we encounter an end element (self's end element that is) => no children
  if nxt.nest() < 0 { return nil }
  
  return &iterateAllChildren{cur:nxt, nxt:nil}
}

// Returns the names of all subtags of this Hash (unsorted!).
// If the Hash has multiple subelements with the same tag, the tag is
// only listed once. I.e. all strings in the result list are always different.
func (self *Hash) Subtags() []string {
  names := map[string]bool{}
  for child := self.FirstChild(); child != nil; child = child.Next() {
    names[child.Element().Name()] = true
  }
  result := make([]string, len(names))
  i := 0
  for name := range names {
    result[i] = name
    i++
  }
  return result
}

// Returns the Text() contents of all <subtag>...</subtag> children for
// all of the provided subtag names.
// The slice will be empty if there are no subtags of that name or if the
// subtag... list is empty.
// The order in the slice follows the order in the argument list, i.e. first
// all values for the first subtag name, then all values for the 2nd subtag 
// name,...
func (self *Hash) Get(subtag ...string) []string {
  result := make([]string,0,1)
  for _,sub := range subtag {
    for child := self.First(sub); child != nil; child = child.Next() {
      result = append(result, child.Text())
    }
  }
  return result
}

// Returns the text contained in this element (just text, not child elements)
// split up in some optimized manner to avoid creating a large string block that
// might leak because of the GC.
func (self *Hash) TextFragments() []string {
  result := make([]string, 0, 1)
  end := self.link()
  for h := self.succ(); h != end; {
    switch h.nest() {
      case 0: result = append(result, h.data)
              h = h.succ()
        // this is always case 1 (i.e. start tag)
        // we can never see an end tag here because we skip them
        // in the line below  h = h.link().succ()
      default: h = h.link().succ()
    }
  }
  return result
}

func write_via_buffer(w io.Writer, buf []byte, s string, n *int64, err *error) bool {
  i := 0
  for i < len(s) {
    cnt := copy(buf,s[i:])
    i += cnt
    nn,ee := util.WriteAll(w, buf[0:cnt])
    *n += int64(nn)
    if ee != nil { *err = ee; return false }
  }
  return true
}

// Writes an XML-representation of this Hash to w until there's no more 
// data to write or an error occurs. The return value n is the number
// of bytes written. Any error encountered during the write is also returned.
//
// NOTE: The representation written is not the same as String(). It's an
// unsorted representation. This function is the most memory-efficient way to
// serialize a Hash.
func (self *Hash) WriteTo(w io.Writer) (n int64, err error) {
  writebuf := make([]byte, 1024)
  
  h := self
  end := self.link()
  for {
    switch h.nest() {
      case 0: sl := escape(h.data)
              for _, s := range sl { 
                if !write_via_buffer(w, writebuf, s, &n, &err) { return }
              }
              
      case 1: if !write_via_buffer(w, writebuf, "<" , &n, &err) { return }
              if !write_via_buffer(w, writebuf, h.data , &n, &err) { return }
              if !write_via_buffer(w, writebuf, ">" , &n, &err) { return }
              
      case -1: if !write_via_buffer(w, writebuf, "</" , &n, &err) { return }
              if !write_via_buffer(w, writebuf, h.link().data , &n, &err) { return }
              if !write_via_buffer(w, writebuf, ">" , &n, &err) { return }
    }
    if h == end { break }
    h = h.succ()
  }
  return
}

// With no arguments Text() returns the single text child of the receiver element;
// with arguments Text(s1, s2,...) returns the concatenation of Get(s1, s2,...)
// separated by '␞' (\u241e, i.e. symbol for record separator).
//
// NOTE: The returned string does not use entity-encoding, i.e. "<" is NOT "&lt;"
func (self *Hash) Text(subtag ...string) string {
  if len(subtag) == 0 {
    return strings.Join(self.TextFragments(), "")
  }
  return strings.Join(self.Get(subtag...), "\u241e")
}

// Returns a textual representation of the XML-tree rooted at the receiver 
// (with proper entity-encoding such as "&lt;" instead of "<").
// Subtags are listed in alphabetical order preceded by the element's text (if any).
func (self *Hash) String() string {
  parts := self.xmlParts()
  return strings.Join(parts,"")
}

func (self *Hash) xmlParts() []string {
  var paths []sortPath = self.innerXML()
  sort.Sort(xmlOrder(paths))
  parts := make([]string,0,6+len(paths)*3) // *3 to have enough room for "<" and ">"
  parts = append(parts, "<", self.Name(), ">")
  for _,path := range paths {
    ele := path[len(path)-1]
    switch ele.n & 3 {
      case 1: parts = append(parts, "</", ele.s, ">")
      case 2: parts = append(parts, "<", ele.s, ">")
      default: parts = append(parts, ele.s)
    }
  }
  parts = append(parts, "</", self.Name(), ">")
  return parts
}

// Runs this Hash's text child through a base64 decoder and replaces it with
// the result. Garbage characters (such as whitespace) are ignored during the
// decoding.
func (self *Hash) DecodeBase64() {
  text := self.TextFragments()
  self.SetText()
  var carry int
  for i := range text {
    self.AppendString(string(util.Base64DecodeString(text[i], &carry)))
  }
  flush := string(util.Base64DecodeString("=", &carry))
  self.AppendString(flush)
}

// Runs this Hash's text child through a base64 encoder and replaces it with
// the result. 
func (self *Hash) EncodeBase64() {
  text := self.TextFragments()
  self.SetText()
  carry := make([]byte,0,2)
  for i := range text {
    l := len(text[i]) + len(carry)
    var rem int
    var buf []byte
    if i < len(text)-1 { // if we have a next string that we can carry over into
      rem = l % 3        // round down l to multiple of 3 and carry over the remainder
      l -= rem
      if l == 0 { // nothing remaining? carry over everything
        carry = append(carry, text[i]...)
        continue
      }
      buf = make([]byte, (l / 3) <<2)
    } else { // last string, can't carry over
      buf = make([]byte, ((l+2) / 3) <<2)
    }
    idx := len(buf) - l
    copy(buf[idx:], carry)
    copy(buf[idx+len(carry):], text[i])
    carry = append(carry[0:0], text[i][len(text[i])-rem:]...)
    self.AppendString(string(util.Base64EncodeInPlace(buf, idx)))
  }
}

// Returns a *Hash whose outer tag has the same name as that of self and
// whose child elements are deep copies of the children selected by filter.
func (self *Hash) Query(filter HashFilter) *Hash {
  result := NewHash(self.Name())
  for item := self.FirstChild() ; item != nil; item = item.Next() {
    if filter.Accepts(item.Element()) {
      result.AddClone(item.Element())
    }
  }
  return result
}

// Removes the children selected by the filter from the hash.
// Returns a *Hash whose outer tag has the same name as self and
// whose child elements are the removed items.
func (self *Hash) Remove(filter HashFilter) *Hash {
  result := NewHash(self.Name())
  for child := self.FirstChild(); child != nil; child = child.Next() {
    if filter.Accepts(child.Element()) {
      result.AddWithOwnership(child.Remove())
    }
  }
  return result
}

// Parses an XML string that must have a single tag surrounding everything,
// with no text outside.
// The conversion will not abort for errors, so even if xmlerr != nil,
// you will get a valid result (although it may not have much to do with 
// the input).
// NOTE: Attributes are treated as if they were child elements.
// E.g. <foo bar="bla"/> is equivalent to <foo><bar>bla</bar></foo>
func StringToHash(xmlstr string) (xml *Hash, xmlerr error) {
  parser := encxml.NewDecoder(strings.NewReader(xmlstr))
  depth := -1
  path := make(map[int]*Hash,4)
  var err error // declare outside because I need it after the loop
  var tok encxml.Token // declare outside because I think := creates a new "err", too.
  for tok, err = parser.Token(); err == nil ; tok, err = parser.Token() {
    switch token := tok.(type) {
      case encxml.StartElement:
              if depth < 0 {
                if len(path) != 0 {
                  xmlerr = fmt.Errorf("StringToHash(): Multiple top-level elements")
                }
                
                xml = NewHash(token.Name.Local)
                path[0] = xml
                depth = 0
              } else {
                path[depth + 1] = path[depth].Add(token.Name.Local)
                depth++
              }
              
              for _, attr := range token.Attr {
                path[depth].Add(attr.Name.Local, attr.Value)
              }
              
      case encxml.EndElement:
              depth--
      case encxml.CharData:
              if (depth < 0) {
                xmlerr = fmt.Errorf("StringToHash(): Stray text outside of tag: %v", string(token))
              } else {
                path[depth].SetText(path[depth].Text() + string(token))
              }
      default: 
              xmlerr = fmt.Errorf("StringToHash(): Unsupported XML token: %v", token)
    }
  }
  
  if err != io.EOF {
    xmlerr = fmt.Errorf("StringToHash(): %v", err)
  }
  
  if xml == nil {
    xml = NewHash("xml")
  }
      
  return
}

// Reads a string from path and uses StringToHash() to parse it to a Hash.
// This function will read until EOF (or another error).
// If an error occurs the returned Hash may contain partial data but it
// is never nil.
func FileToHash(path string) (xml *Hash, err error) {
  file, err := os.Open(path)
  if err != nil {
    return NewHash("xml"), err
  }
  defer file.Close()
  return ReaderToHash(file)
}

// Reads a string from r and uses StringToHash() to parse it to a Hash.
// This function will read until EOF (or another error).
// If an error occurs the returned Hash may contain partial data but it
// is never nil.
func ReaderToHash(r io.Reader) (xml *Hash, err error) {
  xmldata, err := ioutil.ReadAll(r)
  if err != nil {
    return NewHash("xml"), err
  } 
  
  // Ignore whitespace at end of input
  i := len(xmldata)
  for i > 0 && xmldata[i-1] <= ' ' { i-- }
  
  return StringToHash(string(xmldata[0:i]))
}

// returns true if string(b[ofs:ofs+len(pre)]) == pre.
func match(b []byte, ofs int, pre string) bool {
  i := 0
  for ; i < len(b) && i < len(pre); i++ {
    if b[ofs+i] != pre[i] { return false }
  }
  return i == len(pre)
}

// If casefold == false, returns true iff string(b) == s;
// if casefold == true, returns true iff tolower(string(b)) == s
func equals(casefold bool, b []byte, s string) bool {
  if len(b) != len(s) { return false }
  if casefold {
    for i := range b {
      b1 := b[i]
      if 'A' <= b1 && b1 <= 'Z' { b1 += 'a'-'A' }
      b2 := s[i]
      if b1 != b2 { return false }
    }
  } else {
    for i := range b {
      b1 := b[i]
      b2 := s[i]
      if b1 != b2 { return false }
    }
  }
  
  return true
}

// Specifies how to convert an LDIF attribute to a Hash element.
type ElementInfo struct{
  // Name of the LDIF attribute this applies to.
  LDIFAttributeName string
  // Name the corresponding Hash element should have.
  ElementName string
  // If true, the Hash element's text content should be base64-encoded.
  // If the LDIF attribute's value is not already encoded, it will be encoded.
  Base64 bool
}

// Converts LDIF data into a Hash. The outermost tag will always be "xml".
// If an error occurs the returned Hash may contain partial data but it
// is never nil.
//
//  itemtag: If non-empty, each object in the LDIF will be inside an element
//           whose outermosttag is itemtag. If itemtag == "", all objects in the
//           LDIF are merged, i.e. all their combined attributes are directly
//           inside the surrounding "xml" tag.
//  casefold: If true, all attribute names are converted to lowercase.
//            If false, they are left exactly as found in the LDIF.
//  ldif: A []byte, string, io.Reader or *exec.Cmd that provides the LDIF data.
//        Understands all ldapsearch formats with an arbitrary number of "-L" switches.
//  elementInfo: If one or more ElementInfo structs are passed, only attributes
//        matching one of them will be accepted and the first match in the
//        elementInfo list determines how the attribute in the LDIF will be
//        converted to an element in the result Hash.
//        If casefold==true, matching is done case-insensitive. This requires that
//        the LDIFAttributeName fields are all lowercase.
func LdifToHash(itemtag string, casefold bool, ldif interface{}, elementInfo... *ElementInfo) (xml *Hash, err error) {
  x := NewHash("xml")
  
  var xmldata []byte
  switch ld := ldif.(type) {
    case []byte: xmldata = ld
    case string: xmldata = []byte(ld)
    case io.Reader:
      xmldata, err = ioutil.ReadAll(ld)
      if err != nil {
        return x, err
      }
    case *exec.Cmd:
      var outbuf bytes.Buffer
      defer outbuf.Reset()
      var errbuf bytes.Buffer
      defer errbuf.Reset()
      oldout := ld.Stdout
      olderr := ld.Stderr
      ld.Stdout = &outbuf
      ld.Stderr = &errbuf
      err := ld.Run()
      ld.Stdout = oldout
      ld.Stderr = olderr
      errstr := errbuf.String()
      if errstr != "" {
        err = fmt.Errorf(errstr)
      }
      if err != nil {
        return x, err
      }
      
      xmldata = outbuf.Bytes()
    default:
      return x, fmt.Errorf("ldif argument has unsupported type")
  }
  
  item := x
  var attr *Hash
  new_item := true
  end := len(xmldata)
  b64 := false
  var info *ElementInfo = nil
  skip := false
  
  i := 0
  start := 0
  if !match(xmldata, i, "version:") { goto wait_for_item }

///////////////////////////////////////////////////////////////////////  
skip_line:
///////////////////////////////////////////////////////////////////////
  for {  
    if i == end { goto end_of_input }
    if xmldata[i] == '\n' { break }
    i++
  }
  // Even comments can have line continuations in LDIF, so we need to
  // continue skipping if there is a continuation.
  i++
  if i < end && (xmldata[i] == ' ' || xmldata[i] == '\t') { goto skip_line }

///////////////////////////////////////////////////////////////////////
wait_for_item:
///////////////////////////////////////////////////////////////////////
  new_item = true
  for { 
    if i == end { goto end_of_input }
    if ch := xmldata[i]; ch > ' ' {
      if match(xmldata, i, "# search result") { goto end_of_input }
      if ch < 'A' { goto skip_line } // skip garbage (typically comments)
      break
    }
    i++
  }

///////////////////////////////////////////////////////////////////////
scan_attribute_name:  
///////////////////////////////////////////////////////////////////////    
  start = i
  b64 = false
  info = nil
  skip = false
  for {  
    if i == end { goto end_of_input }
    
    if xmldata[i] == '#' { goto skip_broken_attribute }
    
    if xmldata[i] == ':' {
      colon := i
      if colon == start { goto skip_broken_attribute } // line that starts with ":" => Skip
      i++
      if i < end && xmldata[i] == ':' { b64 = true; i++ }

      // See if we have an ElementInfo.
      if len(elementInfo) > 0 {
        for _, inf := range elementInfo {
          if equals(casefold, xmldata[start:colon], inf.LDIFAttributeName) {
            info = inf
            break
          }
        }
        skip = (info == nil)
      }
      
      // if separate items are requested, create <itemtag></itemtag> element as new item
      // otherwise item == x stays as it is
      if new_item && itemtag != "" && !skip { 
        item = x.Add(itemtag) 
        new_item = false
      }
      
      // If no elementInfo arguments have been provided, we create a new element
      // directly from the LDIF attribute's name.
      if len(elementInfo) == 0 {
        attrname := string(xmldata[start:colon])
        if casefold { attrname = strings.ToLower(attrname) }
        attr = item.Add(attrname)
      } else { // We create the new element from the ElementInfo
        if !skip { attr = item.Add(info.ElementName) }
      }
      
      if i == end { goto end_of_input }
      
       // skip 1 space or tab after colon
      if xmldata[i] == ' ' || xmldata[i] == '\t' { i++ } 
      
      break
    }
    i++
  }

///////////////////////////////////////////////////////////////////////
//scan_value_fragment:
///////////////////////////////////////////////////////////////////////
  start = i
  for {
    if i - start > MaxFragmentLength {
      if !skip { attr.AppendString(string(xmldata[start:i])) }
      start = i
    }
    
    if i == end || xmldata[i] == '\n' {
      if !skip { attr.AppendString(string(xmldata[start:i])) }
      i++
        // 1 tab or space is a line continuation, everything else ends the value
      if i >= end || (xmldata[i] != ' ' && xmldata[i] != '\t') { 
        break 
      }
      
      start = i+1 // start next fragment after line continuation marker
    }
    i++
  }

///////////////////////////////////////////////////////////////////////
//attribute_value_scanned:
///////////////////////////////////////////////////////////////////////
  if !skip {
    if info != nil {
      if b64 {
        if !info.Base64 { attr.DecodeBase64() }
      } else {
        if info.Base64 { attr.EncodeBase64() }
      }
    } else {
      if b64 { attr.DecodeBase64() }
    }
  }
  if i >= end { goto end_of_input }
  if xmldata[i] == '\n' { goto wait_for_item } // empty line => next item
  goto scan_attribute_name


///////////////////////////////////////////////////////////////////////  
skip_broken_attribute:
///////////////////////////////////////////////////////////////////////
  for {  
    if i == end { goto end_of_input }
    if xmldata[i] == '\n' { break }
    i++
  }
  i++
  if i >= end { goto end_of_input }
  if xmldata[i] == '\n' { goto wait_for_item } // empty line => next item
  
  // Even comments can have line continuations in LDIF, so we need to
  // continue skipping if there is a continuation.
  if xmldata[i] == ' ' || xmldata[i] == '\t' { goto skip_broken_attribute }
  
  goto scan_attribute_name


///////////////////////////////////////////////////////////////////////
end_of_input:
///////////////////////////////////////////////////////////////////////
    // NOTE: 
    //   Don't assume anything about i here.
    //   i > len(xmldata) is possible, as well as i < len(xmldata)
  
  return x,nil
}

// Returns a sequence of strings that, when joined, yield the original string
// with entity references instead of characters that need replacing in XML
// text and attributes.
func escape(s string) []string {
  sl := []string{s}
  replace := ""
  for i := len(s)-1; i >= 0; i-- {
    switch s[i] {
      case '"':  replace = "&quot;" 
      case '\'': replace = "&apos;" 
      case '&':  replace = "&amp;"
      case '<':  replace = "&lt;"
      case '>':  replace = "&gt;"
      default: continue
    }
    sl[len(sl)-1] = s[i+1:]
    s = s[0:i]
    sl = append(sl, replace, s)
  }
  for i:=0; i < len(sl)-1-i; i++ { sl[i], sl[len(sl)-1-i] = sl[len(sl)-1-i], sl[i] }
  return sl
}

type sortElement struct {
  s string
  n int
}
type sortPath []sortElement
type xmlOrder []sortPath
func (sp xmlOrder) Len() int { return len(sp) }
func (sp xmlOrder) Less(i, j int) bool {
  var path1 sortPath = sp[i]
  var path2 sortPath = sp[j]
  i = 0
  for ; i < len(path1)-1 && i < len(path2)-1; i++ {
    if path1[i].s < path2[i].s { return true }
    if path1[i].s > path2[i].s { return false }
    if path1[i].n < path2[i].n { return true }
    if path1[i].n > path2[i].n { return false }
  }
  // i = min(len(path1)-1, len(path2)-1)
  
  // text nodes are sorted before non-text nodes
  if len(path1) < len(path2) && (path1[i].n & 3 == 0) { return true }
  if len(path1) > len(path2) && (path2[i].n & 3 == 0) { return false }  
  
  return path1[i].n < path2[i].n
}
func (sp xmlOrder) Swap(i, j int) { sp[i], sp[j] = sp[j], sp[i]  }

func (self *Hash) innerXML() []sortPath {
  result := []sortPath{}
  path := sortPath{sortElement{"",0}}
  end := self.link()
  for h := self.succ(); h != end; h = h.succ() {
  
    switch h.nest() {
      case 0: 
              sl := escape(h.data)
              for i := range sl {
                path[len(path)-1].n += 4
                path[len(path)-1].s = sl[i]
                sp := make([]sortElement, len(path))
                copy(sp, path)
                result = append(result, sp)
              }
              
      case 1: 
              path[len(path)-1].n += 6
              path[len(path)-1].s = h.data
              path = append(path, sortElement{path[len(path)-1].s, path[len(path)-1].n})
              sp := make([]sortElement, len(path))
              copy(sp, path)
              result = append(result, sp)
              path[len(path)-1].n -= 2
      
      case -1: 
              path[len(path)-1].n += 5
              path[len(path)-1].s = h.link().data
              sp := make([]sortElement, len(path))
              copy(sp, path)
              result = append(result, sp)
              path = path[0:len(path)-1]
              path[len(path)-1].n -= 2
    }
  }
  
  return result
}

// Verifies that the Hash's structure is intact and returns an error if not.
func (self *Hash) Verify() (err error) {
  defer func() {
    if x := recover(); x != nil {
      err = fmt.Errorf("%v",x)
    }
  }()
  
  h := self
  for h.pred() != nil { h = h.pred() }
  if err = verifyStartTag(h); err != nil { return }
  end := h.link()
  if err = verifyEndTag(end); err != nil { return }
  if end.succ() != nil { return fmt.Errorf("%v has a brother %v but no parent", h.data, end.succ().data) }
  
  path := []*Hash{}
  h = h.succ()
  
  for h != end {
    switch h.nest() {
      case 0: if err = verifyText(h); err != nil { return }
      case 1: if err = verifyStartTag(h); err != nil { return }
              path = append(path, h)
      case -1:
              if h.link() != path[len(path)-1] { return fmt.Errorf("Incorrect end tag for %v", path[len(path)-1].data) }
              if err = verifyEndTag(h); err != nil { return }
              path = path[0:len(path)-1]
      
      default: { return fmt.Errorf("Illegal nest() value %v encountered", h.nest()) }
    }
    h = h.succ()
  }
  
  if len(path) > 0 { return fmt.Errorf("Start tag %v has no matching end tag", path[0].data) }
  
  return
}

func verifyStartTag(h *Hash) error {
  if h == nil { return fmt.Errorf("Unexpected nil pointer where start tag expected") }
  if h.nest() != 1 { return fmt.Errorf("Start tag %v has nest()==%v", h.data, h.nest()) }
  if h.link() == nil { return fmt.Errorf("Start tag %v has nil link", h.data) }
  if h.link().link() != h { return fmt.Errorf("Start tag %v has link() that doesn't point back to it", h.data) }
  if h.succ() == nil { return fmt.Errorf("Start tag %v has succ()==nil", h.data) }
  if h.pred() != nil && h.pred().succ() != h { return fmt.Errorf("Start tag %v has pred() that doesn't point back to it", h.data) }
  if h.succ() != nil && h.succ().pred() != h { return fmt.Errorf("Start tag %v has succ() that doesn't point back to it", h.data) }
  if h.data == "" { return fmt.Errorf("Start tag without name encountered") }
  return nil
}

func verifyEndTag(h *Hash) error {
  if h == nil { return fmt.Errorf("Unexpected nil pointer where end tag expected") }
  if h.data != "" { return fmt.Errorf("End tag has non-empty data %v", h.data) }
  if h.link() == nil { return fmt.Errorf("End tag has nil link") }
  if h.link().link() != h { return fmt.Errorf("End tag has link() that doesn't point back to it") }
  if h.pred() == nil { return fmt.Errorf("End tag has pred()==nil") }
  if h.nest() != -1 { return fmt.Errorf("End tag %v has nest()==%v", h.link().data, h.nest()) }
  if h.pred() != nil && h.pred().succ() != h { return fmt.Errorf("End tag %v has pred() that doesn't point back to it", h.link().data) }
  if h.succ() != nil && h.succ().pred() != h { return fmt.Errorf("End tag %v has succ() that doesn't point back to it", h.link().data) }
  return nil
}

func verifyText(h *Hash) error {
  if h == nil { return fmt.Errorf("Unexpected nil pointer where text expected") }
  if h.link_ != 0 { return fmt.Errorf("Text %v has non-0 link_", h.data) }
  if h.nest() != 0 { return fmt.Errorf("Text %v has nest()==%v", h.data, h.nest()) }
  if h.pred() == nil { return fmt.Errorf("Text %v has pred()==nil", h.data) }
  if h.succ() == nil { return fmt.Errorf("Text %v has succ()==nil", h.data) }
  if h.pred().succ() != h { return fmt.Errorf("Text %v has pred() that doesn't point back to it", h.data) }
  if h.succ().pred() != h { return fmt.Errorf("Text %v has succ() that doesn't point back to it", h.data) }
  return nil
}
