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

// Unit tests run by run_tests.go.
package tests

import (
         "fmt"
         "../xml"
       )

// Unit tests for the package susi/xml.
func Xml_test() {
  
  fmt.Printf("\n=== xml ===\n\n")
  
  x := xml.NewHash("foo")
  check(x, "<foo></foo>")
  
  x.SetText("Dıes ist ein >>>Test<<<")
  check(x, "<foo>Dıes ist ein &gt;&gt;&gt;Test&lt;&lt;&lt;</foo>")
  
  x.SetText("Dies ist %v %vter Test","ein",2)
  check(x, "<foo>Dies ist ein 2ter Test</foo>")
  
  bar := x.Add("bar")
  check(bar, "<bar></bar>")
  
  bar.Add("server", "srv1")
  check(x, "<foo>Dies ist ein 2ter Test<bar><server>srv1</server></bar></foo>")
  
  x_clone := x.Clone()
  x_str := x.String()
  
  srv4 := bar.Add("server", "srv2", "srv3", "srv4")
  check(x, "<foo>Dies ist ein 2ter Test<bar><server>srv1</server><server>srv2</server><server>srv3</server><server>srv4</server></bar></foo>")
  
  srv4.Add("alias", "foxtrott", "alpha")
  check(x, "<foo>Dies ist ein 2ter Test<bar><server>srv1</server><server>srv2</server><server>srv3</server><server>srv4<alias>foxtrott</alias><alias>alpha</alias></server></bar></foo>")
  
  srv4.Add("alias", "bravo")
  check(x, "<foo>Dies ist ein 2ter Test<bar><server>srv1</server><server>srv2</server><server>srv3</server><server>srv4<alias>foxtrott</alias><alias>alpha</alias><alias>bravo</alias></server></bar></foo>")
  
  srv4.Add("alias", "delta")
  check(x, "<foo>Dies ist ein 2ter Test<bar><server>srv1</server><server>srv2</server><server>srv3</server><server>srv4<alias>foxtrott</alias><alias>alpha</alias><alias>bravo</alias><alias>delta</alias></server></bar></foo>")
  
  lst := x.Get("foo") // x is a <foo> but contains no <foo> !!
  check(lst, []string{})
  
  lst = x.Get()
  check(lst, []string{})
  
  lst = x.Get("bar")
  check(lst, []string{bar.Text()})
  
  lst = bar.Get("server")
  check(lst, []string{"srv1","srv2","srv3",srv4.Text()})
  
  txt := srv4.Text("alias")
  check(txt, "foxtrott\u241ealpha\u241ebravo\u241edelta")
  
  txt = srv4.Text("alias","doesnotexist")
  check(txt, "foxtrott\u241ealpha\u241ebravo\u241edelta")
  
  x.Add("max", "X")
  x.Add("moritz", "X")
  txt = x.Text("max", "moritz")
  check(txt, "X\u241eX")
  
  y, xmlerr := xml.StringToHash("<foo/><bar/>")
  check(y, "<bar></bar>")  // last one wins
  check(xmlerr, "StringToHash(): Multiple top-level elements")
  
  y, xmlerr = xml.StringToHash("Nix gut")
  check(y, "<xml></xml>")  // dummy returned if nothing properly parsed
  check(xmlerr, "StringToHash(): Stray text outside of tag: Nix gut")
  
  y, xmlerr = xml.StringToHash("<?xml version=\"1.0\"?><foo>We<bar>Hallo</bar>lt</foo>")
  check(y, "<foo>Welt<bar>Hallo</bar></foo>")
  check(xmlerr, "StringToHash(): Unsupported XML token: {xml [118 101 114 115 105 111 110 61 34 49 46 48 34]}")
  
  _, xmlerr = xml.StringToHash("<-foo></-foo>")
  check(xmlerr, "StringToHash(): XML syntax error on line 1: invalid XML name: -foo")
  
  check(x_clone, x_str)
  check(x_clone.Verify(), nil)
  check(x.Verify(), nil)
  //TODO: The preceding check must fail because /last-sibling is currently not
  // handled properly
  
  // TODO: Name(), AddClone(), AddWithOwnership (including panic), RemoveFirst(),
  // RemoveNext(), Subtags()
  
  // TODO: FileToHash, ReaderToHash
}

