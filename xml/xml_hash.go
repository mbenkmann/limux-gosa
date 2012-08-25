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
         "io"
         "fmt"
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

// Returns the first subtag with the given name or nil if none exists.
func (self *Hash) First(subtag string) *Hash {
  return self.refs[subtag]
}

// Returns the next sibling with the same tag name or nil if none exists.
func (self *Hash) Next() *Hash {
  return self.refs["/next"]
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

// Returns a textual representation of the XML-tree rooted at the receiver.
func (self *Hash) String() string {
  var buffy bytes.Buffer
  fmt.Fprintf(&buffy, "<%s>%s</%s>", self.name, self.InnerXML(), self.name)
  return buffy.String()
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

// Like String() without the surrounding tags for the receiver's element itself.
func (self *Hash) InnerXML() string {
  var buffy bytes.Buffer
  encxml.Escape(&buffy, []byte(self.text))
  for name := range self.refs {
    if name != "" && name[0] != '/' {
      for child := self.refs[name]; child != nil; child = child.Next() {
        buffy.Write([]byte(child.String()))
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
