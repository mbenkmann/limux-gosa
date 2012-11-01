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
         "time"
         
         "../util/deque"
       )

// Unit tests for the package go-susi/util/deque.
func Deque_test() {
  fmt.Printf("\n==== deque ===\n\n")

  var stack deque.Deque
  check(Deque(&stack), "")
  stack.Push(1)
  stack.Push(2)
  stack.Push(3)
  stack.Push(4)
  stack.Push(5)
  check(Deque(&stack, 1,2,3,4,5),"")
  check(stack.Next(), 1); check(Deque(&stack, 2,3,4,5),"")
  check(stack.Next(), 2); check(Deque(&stack, 3,4,5),"")
  check(stack.Pop(), 5);  check(Deque(&stack, 3,4),"")
  check(stack.Next(), 3); check(Deque(&stack, 4),"")
  check(stack.Pop().(int), 4);  check(Deque(&stack),"")
  go func(){
    time.Sleep(1*time.Second)
    stack.Insert("foo")
    time.Sleep(1*time.Second)
    stack.Insert("bar")
  }()
  check(stack.Pop().(string), "foo")
  check(stack.Next(), "bar")
/*
  fifo := deque.New(4, deque.Blocking)
  
  lst := deque.New([]interface{}{1,2,3,4,5})
  
  empty := deque.New()
  
  one := deque.New(deque.Blocking)
  
  vector := deque.New(deque.Exponential, 10)*/
}

func Deque(deck *deque.Deque, data... interface{}) string {
  if deck.Count() != len(data) { return fmt.Sprintf("Count() is %v but should be %v", deck.Count(), len(data)) }
  if deck.IsEmpty() != (len(data) == 0) { return fmt.Sprintf("IsEmpty() should be %v", (len(data)==0)) }
  for i := range data {
    if deck.At(i) != data[i] { return fmt.Sprintf("At(%v) is %v but should be %v", i, deck.At(i), data[i]) }
    if deck.Peek(i) != data[len(data)-1-i] { return fmt.Sprintf("Peek(%v) is %v but should be %v", i, deck.Peek(i), data[len(data)-1-i]) }
  }
  var zero interface{}
  if deck.Peek(-1) != zero { return fmt.Sprintf("Peek(-1) returns %v", deck.Peek(-1)) }
  if deck.Peek(-1) != nil { return fmt.Sprintf("Peek(-1) returns %v", deck.Peek(-1)) }  
  
  if deck.At(-1) != zero { return fmt.Sprintf("At(-1) returns %v", deck.At(-1)) }
  if deck.At(-1) != nil { return fmt.Sprintf("At(-1) returns %v", deck.At(-1)) }    
  
  if deck.Peek(len(data)) != zero { return fmt.Sprintf("Peek(len(data)) returns %v", deck.Peek(len(data))) }
  if deck.Peek(len(data)) != nil { return fmt.Sprintf("Peek(len(data)) returns %v", deck.Peek(len(data))) }  
  
  if deck.At(len(data)) != zero { return fmt.Sprintf("At(len(data)) returns %v", deck.At(len(data))) }
  if deck.At(len(data)) != nil { return fmt.Sprintf("At(len(data)) returns %v", deck.At(len(data))) }  
  
  return ""
}

