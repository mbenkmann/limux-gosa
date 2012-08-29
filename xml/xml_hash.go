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

// A simple map-based XML storage.
package xml

import ( 
         "os"
         "io"
         "fmt"
         "sort"
         "bufio"
         "bytes"
         "strings"
       )

import encxml "encoding/xml"

// An xml.Hash is a representation of simple XML data that follows certain
// restrictions:
//  * no attributes
//  * all text children of an element are treated as a single text, even if
//    they are interspersed with tags.
//  * the order of siblings with the same tag name is preserved, but the order
//    of siblings with different tag names is not. 
//    E.g.  "<xml><foo>1</foo><bar>A</bar><foo>2</foo><bar>B</bar></xml>"
//    will result in a Hash that preserves the order of "1" and "2" as well as
//    the order of "A" and "B", but does not preserve the relative order of <foo>
//    and <bar> elements.
//
// In simpler terms: a Hash is a hashmap of lists where each list stores
// Hashes for a single type of tag. 
// 
type Hash struct {
  // "foo" for a tag <foo>
  name string
  
  // text content
  text string
  
  // A first child has refs["/last-sibling"] if it has siblings.
  // An element that has a following sibling has refs["/next"]
  refs map[string]*Hash
}

// Returns a new *Hash with outer-most element <name>.
func NewHash(name string) *Hash {
  return &Hash{name:name, refs:map[string]*Hash{}}
}

// Returns the name of the outer tag of the Hash.
func (self *Hash) Name() string { return self.name }

// Returns a deep copy of this xml.Hash that is completely independent.
// The clone will not have any siblings, even if the original had some.
func (self *Hash) Clone() *Hash {
  hash := &Hash{name:self.name, text:self.text, refs:map[string]*Hash{}}
  for k, v := range self.refs {
    if k != "/last-sibling" && k != "/next" {
      hash.refs[k] = v.cloneWithSiblings()
    }
  }
  return hash
}

// Returns a deep copy of this Hash including (if it has "/next") a clone
// of all of its siblings. "/last-sibling" will be set on the clone if it has
// at least 1 sibling, even if the original Hash did not have "/last-sibling" set.
// This means that cloneWithSiblings() called on a 2nd child will nevertheless
// return a correctly linked Hash.
func (self *Hash) cloneWithSiblings() *Hash {
  clone := self.Clone()
  prev  := clone
  for next := self.refs["/next"]; next != nil; next = next.refs["/next"] {
    nextClone := next.Clone()
    prev.refs["/next"] = nextClone
    prev = nextClone
  }
  if prev != clone {
    clone.refs["/last-sibling"] = prev
  }
  return clone
}

// Verifies that the Hash's structure is intact and returns an error if not.
func (self *Hash) Verify() error {
  have_seen := map[*Hash]bool{}
  return self.verify(have_seen, false)
}

// Verifies that this Hash's structure is intact and returns an error if not.
//  have_seen - if either this Hash or any of its descendants or siblings is
//              in this map, an error will be reported. All the Hash's siblings
//              and descendants will be added to this map during the check. This
//              detects cycles in the data structure.
//  must_be_first_sibling - set to true if this Hash is a first (or only) child.
//                          Used to check if "/last-sibling" is properly set.
func (self *Hash) verify(have_seen map[*Hash]bool, must_be_first_sibling bool) error {
  if have_seen[self] {
    return fmt.Errorf("Loop/backreference at %v", self.name)
  }
  
  have_seen[self] = true
  
  if self.refs["/last-sibling"] != nil {
    if self.refs["/next"] == nil {
      return fmt.Errorf("%v has /last-sibling but no /next", self.name)
    }
    if self.refs["/last-sibling"] == self {
      return fmt.Errorf("%v is its own /last-sibling", self.name)
    }
    if self.refs["/next"] == self {
      return fmt.Errorf("%v is its own /next sibling", self.name)
    }
    
    last := self.refs["/next"]
    for next := last ; next != nil; next = next.refs["/next"] {
      if have_seen[next] {
        return fmt.Errorf("Loop/backreference at %v", next.name)
      }
      if next.refs["/last-sibling"] != nil {
        return fmt.Errorf("%v has a sibling %v that has /last-sibling set", self.name, next.name)
      }
      last = next
      have_seen[next] = true
    }
    
    if last != self.refs["/last-sibling"] {
      return fmt.Errorf("%v's /last-sibling is not actually its last sibling", self.name)
    }
  } else {  // if !self.refs["/last-sibling"]
    if self.refs["/next"] != nil {
      if must_be_first_sibling {
        return fmt.Errorf("%v is first child and has siblings but has no /last-sibling", self.name)
      }
      
      for next := self.refs["/next"] ; next != nil; next = next.refs["/next"] {
        if have_seen[next] {
          return fmt.Errorf("Loop/backreference at %v", next.name)
        }
        if next.refs["/last-sibling"] != nil {
          return fmt.Errorf("%v has a sibling %v that has /last-sibling set", self.name, next.name)
        }
        have_seen[next] = true
      }
    }
  }
  
  err := self.verifyChildren(have_seen)
  if err != nil { return err }
  
  for next := self.refs["/next"] ; next != nil; next = next.refs["/next"] {
    if next.Name() != self.Name() {
      return fmt.Errorf("Element with name %v has sibling with name %v", self.Name(), next.Name())
    }
    err = next.verifyChildren(have_seen)
    if err != nil { return err }
  }
  
  return nil
}

// Recursively verifies the correctness of the subtrees rooted at this Hash's
// children. See Verify().
func (self *Hash) verifyChildren(have_seen map[*Hash]bool) error {
  for k, v := range self.refs {
    if k != "/next" && k != "/last-sibling" {
      if v.Name() != k {
        return fmt.Errorf("Element with name %v in list of key %v", v.Name(), k)
      }
      err := v.verify(have_seen, true)
      if err != nil { return err }
    }
  }
  
  return nil
}

// Replaces the current text content of the receiver with the new text.
// If no args are provided, the format string is used directly, otherwise
// formatting will be done by Sprintf().
// Returns the new text content.
func (self *Hash) SetText(format string, args ...interface{}) string {
  if len(args) == 0 {
    self.text = format
  } else {
    self.text = fmt.Sprintf(format, args...)
  }
  return self.text
}

// Adds <subtag>text</subtag> children (an empty <subtag></subtag> is added
// even if no text is provided) and returns the last subtag added.
func (self *Hash) Add(subtag string, text ...string) *Hash {
  if len(text) == 0 {
    text = []string{""}
  }
  
  new_hash := make([]*Hash, len(text))
  for i := range text {
    new_hash[i] = &Hash{name:subtag, text:text[i], refs:map[string]*Hash{}}
    if i > 0 {
      new_hash[i-1].refs["/next"] = new_hash[i]
    }
  }
  
  first := self.First(subtag)
  if first == nil {
    first = new_hash[0]
    self.refs[subtag] = first
    if len(new_hash) > 1 {
      first.refs["/last-sibling"] = new_hash[len(new_hash)-1]
    }
  } else {
    last := first.refs["/last-sibling"]
    first.refs["/last-sibling"] = new_hash[len(new_hash)-1]
    if last == nil {
      first.refs["/next"] = new_hash[0]
    } else {
      last.refs["/next"] = new_hash[0]
    }
  }
  
  return new_hash[len(new_hash)-1]
}

// Adds a clone of xml to this Hash as a child (at the end of the list
// of children with the same element name) and returns the clone.
func (self *Hash) AddClone(xml *Hash) *Hash {
  clone := xml.Clone()
  self.AddWithOwnership(clone)
  return clone
}

// Takes the xml object (not a copy) and integrates it into this Hash
// as a child.
// ATTENTION! xml must not be child of another Hash (which implies that it
// must not have any siblings).
func (self* Hash) AddWithOwnership(xml *Hash) {
  if xml == nil || xml == self || xml.refs["/next"] != nil {
    panic("AddWithOwnership: Sanity check failed!")
  }
  subtag := xml.Name()
  first  := self.First(subtag)
  if first == nil {
    self.refs[subtag] = xml
  } else {
    last := first.refs["/last-sibling"]
    first.refs["/last-sibling"] = xml
    if last == nil {
      first.refs["/next"] = xml
    } else {
      last.refs["/next"] = xml
    }
  }
}

// Removes the first subtag child from this Hash and returns it (or nil if this
// Hash has no subtag child)
func (self *Hash) RemoveFirst(subtag string) *Hash {
  first := self.First(subtag)
  if first == nil { return nil }
  
  next := first.refs["/next"]
  if next == nil {
    delete(self.refs, subtag)
    return first
  }
  
  last := first.refs["/last-sibling"]
  self.refs[subtag] = next
  if last != next {
    next.refs["/last-sibling"] = last
    delete(first.refs, "/last-sibling")
  }
  
  delete(first.refs, "/next")
  return first
}

// Removes the next sibling of this Hash from parent's child list
// (which both must be members of)
// and returns it (or nil if this Hash has no siblings).
func (self *Hash) RemoveNext(parent *Hash) *Hash {
  next := self.refs["/next"]
  if next == nil { return nil }
  
  nextnext := next.refs["/next"]
  delete(next.refs, "/next")
  
  if nextnext != nil {
    self.refs["/next"] = nextnext
    return next
  }
  
  // self is now the last child in the list
  
  first := parent.First(self.Name())
  if first == self {
    delete(self.refs, "/last-sibling")
  } else {
    first.refs["/last-sibling"] = self
  }
  
  delete(self.refs, "/next")
  return next
}

// Returns the first subtag with the given name or nil if none exists.
func (self *Hash) First(subtag string) *Hash {
  return self.refs[subtag]
}

// Returns the next sibling with the same tag name or nil if none exists.
func (self *Hash) Next() *Hash {
  return self.refs["/next"]
}

// Returns the names of all subtags of this Hash (unsorted!).
// If the Hash has multiple subelements with the same tag, the tag is
// only listed once. I.e. all strings in the result list are always different.
func (self *Hash) Subtags() []string {
  result := make([]string, 0, len(self.refs))
  for k, _ := range self.refs {
    if !strings.HasPrefix(k, "/") {
      result = append(result, k)
    }
  }
  return result
}

// Returns the Text() contents of all <subtag>...</subtag> children for
// all of the provided subtag names.
// The slice will be empty if there are no subtags of that name.
// The order in the slice follows the order in the argument list, i.e. first
// all values for the first subtag name, then all values for the 2nd subtag 
// name,...
func (self *Hash) Get(subtag ...string) []string {
  result := []string{}
  for _,sub := range subtag {
    for child := self.First(sub); child != nil; child = child.Next() {
      result = append(result, child.Text())
    }
  }
  return result
}

// With no arguments Text() returns the single text child of the receiver element;
// with arguments Text(s1, s2,...) returns the concatenation of Get(s1, s2,...)
// separated by \u241e (symbol for record separator).
func (self *Hash) Text(subtag ...string) string {
  if len(subtag) == 0 {
    var buffy bytes.Buffer
    encxml.Escape(&buffy, []byte(self.text))
    return buffy.String()
  }
  return strings.Join(self.Get(subtag...), "\u241e")
}

// Returns a textual representation of the XML-tree rooted at the receiver.
// Subtags are listed in alphabetical order preceded by the element's text (if any).
// Use SortedString() if you want more control over the tag order.
func (self *Hash) String() string {
  return self.SortedString()
}

// Like String() but you can pass a list of tags that should be sorted before 
// their siblings. These subelements will be listed in the order they appear
// in the sortorder arguments list.
// All other subelements will appear after the listed ones in alphabetical order.
// An element's text always precedes subelements.
//
// NOTE:
//  If sortorder contains the same name more than once, the earliest position in
//  the list will win. Subelements will not be duplicated in the output.
func (self *Hash) SortedString(sortorder ...string) string {
  var buffy bytes.Buffer
  buffy.WriteByte('<')
  buffy.WriteString(self.Name())
  buffy.WriteByte('>')
  buffy.WriteString(self.InnerXML(sortorder...))
  buffy.WriteString("</")
  buffy.WriteString(self.Name())
  buffy.WriteByte('>')
  return buffy.String()
}

// Like SortedString() without the surrounding tags for the receiver's element itself.
// You can pass a list of tags that should be sorted before their siblings. These
// subelements will be listed in the order they appear in the sortorder arguments 
// list.
// All other subelements will appear after the listed ones in alphabetical order.
// An element's text always precedes subelements.
//
// NOTE:
//  If sortorder contains the same name more than once, the earliest position in
//  the list will win. Subelements will not be duplicated in the output.
func (self *Hash) InnerXML(sortorder ...string) string {
  var buffy bytes.Buffer
  encxml.Escape(&buffy, []byte(self.text))
  keys := make([]string, len(self.refs))
  i := 0
  for key := range self.refs {
    if key != "" && key[0] != '/' {
      keys[i] = key
      i++
    }
  }
  keys = keys[0:i]
  sort.Strings(keys)
  
  var name string
  for _, name = range sortorder {
    // WARNING! Do not use sort.SearchStrings, here. The binary search breaks
    // after replacing a key with "" as done below.
    for i = 0; i < len(keys) ; i++ {
      if keys[i] == name {
        keys[i] = ""  // remove key from the remaining keys list
        for child := self.First(name); child != nil; child = child.Next() {
          childstr := child.SortedString(sortorder...)
          buffy.WriteString(childstr)
        }
      }
    }
  }
  
  for _, name = range keys {
    if name != "" {
      for child := self.First(name); child != nil; child = child.Next() {
        childstr := child.SortedString(sortorder...)
        buffy.WriteString(childstr)
      }
    }
  }
  return buffy.String()
}


// Parses an XML string that must have a single tag surrounding everything,
// with no text outside.
// The conversion will not abort for errors, so even if xmlerr != nil,
// you will get a valid result (although it may not have much to do with 
// the input).
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

// Reads a string from r and uses StringToHash() to parse it to a Hash.
// This function will read until EOF (or another error).
// If an error occurs the returned Hash may contain partial data.
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
// If an error occurs the returned Hash may contain partial data.
func ReaderToHash(r io.Reader) (xml *Hash, err error) {
  bread := bufio.NewReader(r)
  xmlstr, err := bread.ReadString(0)
  if err != nil && err != io.EOF {
    return NewHash("xml"), err
  } 
  
  return StringToHash(xmlstr)
}

