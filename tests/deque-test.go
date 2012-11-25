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

// Unit tests run by run-tests.go.
package tests

import (
         "fmt"
         "time"
         
         "../util/deque"
       )

// Unit tests for the package go-susi/util/deque.
func Deque_test() {
  fmt.Printf("\n==== deque ===\n\n")

  whatever := []uint{0,1,2,3,4,5,21,127,200}
  zero := []uint{0}
  one := []uint{1}
  check(Growth(deque.Double, zero, zero, whatever, one),"")
  check(Growth(deque.Double, zero, one, whatever, one),"")
  check(Growth(deque.Double, []uint{1,2,3,4,8,16,32,111}, one, []uint{11}, []uint{1,2,3,4,8,16,32,111}),"")
  check(Growth(deque.Double, []uint{1,2,3,4,8,16,32,111}, []uint{11}, []uint{11}, []uint{15,14,21,12,24,16,32,111}),"")
  
  deque.GrowthFactor = 0
  check(Growth(deque.Exponential, whatever, zero, zero, one),"")
  check(Growth(deque.Exponential, whatever, one, zero, one),"")
  check(Growth(deque.Exponential, whatever, one, one, []uint{2}),"")
  check(Growth(deque.Exponential, whatever, one, []uint{2}, []uint{4}),"")
  deque.GrowthFactor = 7
  check(Growth(deque.Exponential, []uint{666}, one, []uint{0,1,2,3,4}, []uint{7,14,28,56,112}),"")
  check(Growth(deque.Exponential, []uint{666}, []uint{30}, []uint{0,1,2,3,4}, []uint{56,56,56,56,112}),"")
  
  deque.GrowthFactor = 0
  var i uint
  for i = 0; i < 5; i++ {
    check(Growth(deque.Accelerated, whatever, []uint{i}, whatever, []uint{i}),"")
  }
  
  deque.GrowthFactor = 7
  check(Growth(deque.Accelerated, []uint{111}, one, []uint{0,1,2,3,4}, []uint{7,14,21,28,35}),"")

  check(Growth(deque.GrowBy(0), []uint{111}, []uint{0,1,2,3,4,5,6,7,8}, []uint{222}, []uint{0,1,2,3,4,5,6,7,8}),"")  
  check(Growth(deque.GrowBy(1), []uint{111}, []uint{0,1,2,3,4,5,6,7,8}, []uint{222}, []uint{1,1,2,3,4,5,6,7,8}),"")
  check(Growth(deque.GrowBy(2), []uint{111}, []uint{0,1,2,3,4,5,6,7,8}, []uint{222}, []uint{2,2,2,3,4,5,6,7,8}),"")
  check(Growth(deque.GrowBy(3), []uint{111}, []uint{0,1,2,3,4,5,6,7,8}, []uint{222}, []uint{3,3,3,3,4,5,6,7,8}),"")

  check(Growth(deque.BlockIfFull, whatever, whatever, whatever, zero),"")
  
  func() {
    defer func(){
      check(recover(),deque.Overflow)
    }()
    deque.PanicIfOverflow(1,2,3)
  }()
  
  d2 := deque.New(deque.GrowthFunc(deque.Exponential))
  d2.CheckInvariant()
  check(d2.Growth, deque.Exponential)
  
  check(deque.New(111).Capacity(),111)
  check(deque.New(22,[]interface{}{1,"foo",2,"bar"},111).Capacity(),111)
  check(deque.New([]interface{}{1,"foo",2,"bar"},2).Capacity(),4)
  d1 := deque.New([]interface{}{1,"foo",2,"bar"},2,deque.Accelerated,[]interface{}{3,"foobar",10,"xxx","yyy"})
  check(Deque(d1,1,"foo",2,"bar",3,"foobar",10,"xxx","yyy"),"")
  check(d1.Capacity(),9)
  check(d1.Growth, deque.Accelerated)
  d1.Init([]interface{}{"bla","argl","bart"},d1,[]interface{}{},[]interface{}{"mark","susi","karl","peter"},[]interface{}{},d1)
  check(Deque(d1,"bla","argl","bart",1,"foo",2,"bar",3,"foobar",10,"xxx","yyy","mark","susi","karl","peter",1,"foo",2,"bar",3,"foobar",10,"xxx","yyy"),"")
  check(d1.Capacity(),25)
  check(d1.Count(),25)
  check(d1.String(), "Deque[bla argl bart 1 foo 2 bar 3 foobar 10 xxx yyy mark susi karl peter 1 foo 2 bar 3 foobar 10 xxx yyy]")
  check(deque.New().String(),"Deque[]")
  check(deque.New([]interface{}{"foobar"}).String(),"Deque[foobar]")
  
  var stack deque.Deque
  check(Deque(&stack), "")
  check(stack.Capacity(), deque.CapacityDefault)
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
  check(stack.Pop().(string), "foo"); check(Deque(&stack),"")
  check(stack.Next(), "bar"); check(Deque(&stack),"")
  
  check(Remove(11), "")
  check(Insert(11), "")
  
  deck := deque.New([]interface{}{1})
  check(Wait(deck, (*deque.Deque).WaitForItem, true, 0),"")
  check(Wait(deque.New(), (*deque.Deque).WaitForItem, false, 3*time.Second), "")
  deck.Init()
  go func() {
    time.Sleep(1*time.Second)
    deck.Push(1)
  }()
  check(Wait(deck, (*deque.Deque).WaitForItem, true, 1*time.Second),"")
  deck.Init()
  go func() {
    time.Sleep(1*time.Second)
    deck.Init()
  }()
  check(Wait(deck, (*deque.Deque).WaitForItem, false, 3*time.Second),"")
  go func() {
    time.Sleep(1*time.Second)
    deck.Init([]interface{}{1,2})
  }()
  check(Wait(deck, (*deque.Deque).WaitForItem, true, 1*time.Second),"")
  
  deck = deque.New(4, []interface{}{1,2,3}, deque.BlockIfFull)
  check(Wait(deck, (*deque.Deque).WaitForSpace, true, 0),"")
  deck.Push(4)
  check(Wait(deck, (*deque.Deque).WaitForSpace, false, 3*time.Second), "")
  go func() {
    time.Sleep(1*time.Second)
    deck.Pop()
  }()
  check(Wait(deck, (*deque.Deque).WaitForSpace, true, 1*time.Second),"")
  deck.Push(1)
  go func() {
    time.Sleep(1*time.Second)
    deck.Init(3,[]interface{}{1,2,3})
  }()
  check(Wait(deck, (*deque.Deque).WaitForSpace, false, 3*time.Second),"")
  go func() {
    time.Sleep(1*time.Second)
    deck.Init(4,[]interface{}{1,2,3})
  }()
  check(Wait(deck, (*deque.Deque).WaitForSpace, true, 1*time.Second),"")
  
  check(Wait(deck, (*deque.Deque).WaitForEmpty, false, 3*time.Second), "")
  go func() {
    time.Sleep(1*time.Second)
    deck.Pop()
    deck.Pop()
    deck.Pop()
  }()
  check(Wait(deck, (*deque.Deque).WaitForEmpty, true, 1*time.Second),"")
  check(Wait(deck, (*deque.Deque).WaitForEmpty, true, 0),"")
  deck.Push(1)
  go func() {
    time.Sleep(1*time.Second)
    deck.Init(4,[]interface{}{1,2,3})
  }()
  check(Wait(deck, (*deque.Deque).WaitForEmpty, false, 3*time.Second),"")
  go func() {
    time.Sleep(1*time.Second)
    deck.Init(4)
  }()
  check(Wait(deck, (*deque.Deque).WaitForEmpty, true, 1*time.Second),"")

  for i := 0; i < 33; i++ {
    check(Overcapacity(i),"")
  }
  
  deck = deque.New(4)
  deck.Raw(2)
  check(deck.Overcapacity(0)==deck, true)
  check(Deque(deck),"")
  check(deck.Capacity(),0)
  
  deck = deque.New(4)
  deck.Raw(5)
  check(deck.Overcapacity(10)==deck, true)
  check(Deque(deck),"")
  check(deck.Capacity(),10)
  
  deck = deque.New()
  check(deck.Push(1)==deck,true); check(Deque(deck,1),"")
  check(deck.Push(2)==deck,true); check(Deque(deck,1,2),"")
  check(deck.PushAt(-2,"x"),nil); check(Deque(deck,1,2),"")
  check(deck.PushAt(-1,"x"),nil); check(Deque(deck,1,2),"")
  check(deck.PushAt(3,"x"),nil); check(Deque(deck,1,2),"")
  check(deck.PushAt(4,"x")==nil,true); check(Deque(deck,1,2),"")
  check(deck.PushAt(0,"foo")==deck,true); check(Deque(deck,1,2,"foo"),"")
  check(deck.PushAt(1,"bar")==deck,true); check(Deque(deck,1,2,"bar","foo"),"")
  check(deck.PushAt(4,"FOO")==deck,true); check(Deque(deck,"FOO",1,2,"bar","foo"),"")
  check(deck.PushAt(5,"BAR")==deck,true); check(Deque(deck,"BAR","FOO",1,2,"bar","foo"),"")
  check(deck.PushAt(3,1.5)==deck,true); check(Deque(deck,"BAR","FOO",1,1.5,2,"bar","foo"),"")
  check(deck.Pop(),"foo"); check(Deque(deck,"BAR","FOO",1,1.5,2,"bar"),"")
  check(deck.PopAt(0),"bar"); check(Deque(deck,"BAR","FOO",1,1.5,2),"")
  check(deck.PopAt(5),nil); check(Deque(deck,"BAR","FOO",1,1.5,2),"")
  check(deck.PopAt(4),"BAR"); check(Deque(deck,"FOO",1,1.5,2),"")
  check(deck.PopAt(1),1.5); check(Deque(deck,"FOO",1,2),"")
  deck = deque.New()
  go func() { time.Sleep(1*time.Second); deck.Push("Yeah!") }()
  for i:=-2; i<=2; i++ { 
    check(deck.PopAt(i),nil); check(Deque(deck),"")
  }
  check(deck.PopAt(0),nil)
  time.Sleep(2*time.Second)
  check(deck.PopAt(0),"Yeah!")
  for i:=0; i < 50 ; i++ {
    go func(){ 
      p:=deck.Pop().(string);
      if p != "MSB" { check(p,"MSB") } 
    }()
  }
  for i=0; i < 50; i++ {
    deck.Push("MSB")
    time.Sleep(100*time.Millisecond)
  }
  
  deck = deque.New()
  check(deck.Insert(1)==deck,true); check(DequeReversed(deck,1),"")
  check(deck.Insert(2)==deck,true); check(DequeReversed(deck,1,2),"")
  check(deck.InsertAt(-2,"x"),nil); check(DequeReversed(deck,1,2),"")
  check(deck.InsertAt(-1,"x"),nil); check(DequeReversed(deck,1,2),"")
  check(deck.InsertAt(3,"x"),nil); check(DequeReversed(deck,1,2),"")
  check(deck.InsertAt(4,"x")==nil,true); check(DequeReversed(deck,1,2),"")
  check(deck.InsertAt(0,"foo")==deck,true); check(DequeReversed(deck,1,2,"foo"),"")
  check(deck.InsertAt(1,"bar")==deck,true); check(DequeReversed(deck,1,2,"bar","foo"),"")
  check(deck.InsertAt(4,"FOO")==deck,true); check(DequeReversed(deck,"FOO",1,2,"bar","foo"),"")
  check(deck.InsertAt(5,"BAR")==deck,true); check(DequeReversed(deck,"BAR","FOO",1,2,"bar","foo"),"")
  check(deck.InsertAt(3,1.5)==deck,true); check(DequeReversed(deck,"BAR","FOO",1,1.5,2,"bar","foo"),"")
  check(deck.Next(),"foo"); check(DequeReversed(deck,"BAR","FOO",1,1.5,2,"bar"),"")
  check(deck.RemoveAt(0),"bar"); check(DequeReversed(deck,"BAR","FOO",1,1.5,2),"")
  check(deck.RemoveAt(5),nil); check(DequeReversed(deck,"BAR","FOO",1,1.5,2),"")
  check(deck.RemoveAt(4),"BAR"); check(DequeReversed(deck,"FOO",1,1.5,2),"")
  check(deck.RemoveAt(1),1.5); check(DequeReversed(deck,"FOO",1,2),"")
  deck = deque.New()
  go func() { time.Sleep(1*time.Second); deck.Insert("Yeah!") }()
  for i:=-2; i<=2; i++ { 
    check(deck.RemoveAt(i),nil); check(DequeReversed(deck),"")
  }
  check(deck.RemoveAt(0),nil)
  time.Sleep(2*time.Second)
  check(deck.RemoveAt(0),"Yeah!")
  for i:=0; i < 50 ; i++ {
    go func(){ 
      p:=deck.Next().(string);
      if p != "MSB" { check(p,"MSB") } 
    }()
  }
  for i=0; i < 50; i++ {
    deck.Insert("MSB")
    time.Sleep(100*time.Millisecond)
  }
  
  
}

func Overcapacity(i int) string {
  deck := deque.New(i)
  for j := 1; j <= i; j++ { deck.Push(j) }
  for j := 1; j <= i; j++ { deck.Pop() }
  // now the internal buffer is filled with the numbers 1,...,i
  raw,_ := deck.Raw(-1)
  if len(raw) != i { return fmt.Sprintf("len(%v) != %v", raw,i) }
  if cap(raw) != i { return fmt.Sprintf("cap(%v) != %v", raw,i) }
  if deck.Overcapacity(uint(i)) != deck { return fmt.Sprintf("Overcapacity(%v) does not return self",i) }
  deck.CheckInvariant()
  raw2,_ := deck.Raw(-1)
  if fmt.Sprintf("%v",raw2) != fmt.Sprintf("%v",raw) { return fmt.Sprintf("Overcapacity(i) with i==Capacity() must leave internal buffer untouched") }
  
  data := []interface{}{"bar",1,2,3,4,5,6,7,8,9,"foo"}
  for _,rotation := range []int{0,1,4,7,len(data)-1,len(data)-2} {
    deck = deque.New(data)
    deck.Raw(rotation)
    deck.CheckInvariant()
    for j := 0; j <= i ; j++ {
      if deck.Overcapacity(uint(j)) != deck { return fmt.Sprintf("Overcapacity(%v) does not return self",j) }
      res := Deque(deck, data...)
      if res != "" { return res }
      if deck.Capacity() != len(data) + j { return fmt.Sprintf("Overcapacity(%v) resulted in %v",j,deck.Capacity()-len(data)) }
    }
    for j := i; j >= 0 ; j-- {
      deck.Raw(i >> 1)
      if deck.Overcapacity(uint(j)) != deck { return fmt.Sprintf("Overcapacity(%v) does not return self",j) }
      res := Deque(deck, data...)
      if res != "" { return res }
      if deck.Capacity() != len(data) + j { return fmt.Sprintf("Overcapacity(%v) resulted in %v",j,deck.Capacity()-len(data)) }
    }
  }
  
  return ""
}

func Wait(deck *deque.Deque, fn func(*deque.Deque, time.Duration) bool, result bool, wait time.Duration) string {
  start := time.Now()
  res := fn(deck, 3*time.Second)
  if res != result { return "Incorrect return value" }
  dura := time.Since(start) - wait
  if dura < 0 { dura = -dura }
  if dura > 100*time.Millisecond { return "Incorrect wait time" }
  return ""
}

func Remove(maxcapacity int) string {
  for capa := 0; capa <= maxcapacity; capa++ {
    for sz := 0; sz <= capa; sz++ {
      testdata := []interface{}{}
      for n := 1; n <= sz; n++ { testdata = append(testdata, n) }
      for a := 0; a < capa; a++ { 
        for idx := -2; idx <= capa + 2; idx++ {
          testdeque := deque.New(capa, testdata)
          _,rawidx := testdeque.Raw(a)
          if rawidx != a { return fmt.Sprintf("Raw(%v) is broken",a) }
          var res interface{}
          if idx >= 0 && idx < len(testdata) {
            res = testdata[idx]
          }
          x := testdeque.RemoveAt(idx)
          if x != res { 
            return fmt.Sprintf("After Raw(%v) Deque(%v,%v).RemoveAt(%v) == %v but should be %v",a,capa,testdata,idx,x,res)
          }
          var testdata2 []interface{}
          if x != nil {
            testdata2 = make([]interface{},len(testdata)-1)
            copy(testdata2[0:idx],testdata[0:idx])
            copy(testdata2[idx:], testdata[idx+1:])
          } else {
            testdata2 = testdata
          }
          ck := Deque(testdeque, testdata2...)
          if ck != "" { return ck }
        }
      }
    }
  }
  return ""
}

func Insert(maxcapacity int) string {
  for capa := 0; capa <= maxcapacity; capa++ {
    for sz := 0; sz <= capa; sz++ {
      testdata := []interface{}{}
      for n := 1; n <= sz; n++ { testdata = append(testdata, n) }
      for a := 0; a < capa; a++ { 
        for idx := -2; idx <= capa + 2; idx++ {
          testdeque := deque.New(capa, testdata)
          _,rawidx := testdeque.Raw(a)
          if rawidx != a { return fmt.Sprintf("Raw(%v) is broken",a) }
          var res *deque.Deque
          if idx >= 0 && idx <= len(testdata) {
            res = testdeque
          }
          x := testdeque.InsertAt(idx,"foo")
          if x != res { 
            return fmt.Sprintf("After Raw(%v) Deque(%v,%v).InsertAt(%v) == %v but should be %v",a,capa,testdata,idx,x,res)
          }
          var testdata2 []interface{}
          if x != nil {
            testdata2 = make([]interface{},len(testdata)+1)
            copy(testdata2[0:idx],testdata[0:idx])
            testdata2[idx] = "foo"
            copy(testdata2[idx+1:], testdata[idx:])
          } else {
            testdata2 = testdata
          }
          ck := Deque(testdeque, testdata2...)
          if ck != "" { return ck }
        }
      }
    }
  }
  return ""
}


func Growth(growth deque.GrowthFunc, current, additional, growthcount, result []uint) string {
  i := 0
  for _,c := range current {
    for _,a := range additional {
      for _,g := range growthcount {
        if growth(c,a,g) != result[i] { return fmt.Sprintf("Growth(%v,%v,%v) == %v but should be %v",c,a,g,growth(c,a,g),result[i]) }
        if i+1 < len(result) { i++ }
      }
    }
  }
  return ""
}

func DequeReversed(deck *deque.Deque, data... interface{}) string {
  i:=0
  j:=len(data)-1
  for ; i<j ; { 
    data[i],data[j] = data[j],data[i] 
    i++
    j--
  }
  return Deque(deck, data...)
}

func Deque(deck *deque.Deque, data... interface{}) string {
  deck.CheckInvariant()
  if deck.Count() != len(data) { return fmt.Sprintf("Count() is %v but should be %v", deck.Count(), len(data)) }
  if deck.IsEmpty() != (len(data) == 0) { return fmt.Sprintf("IsEmpty() should be %v", (len(data)==0)) }
  if deck.IsFull() != (len(data) == deck.Capacity()) { return fmt.Sprintf("IsFull() should be %v", (len(data)==deck.Capacity())) }
  for i := range data {
    if deck.At(i) != data[i] { return fmt.Sprintf("At(%v) is %v but should be %v", i, deck.At(i), data[i]) }
    if deck.Peek(i) != data[len(data)-1-i] { return fmt.Sprintf("Peek(%v) is %v but should be %v", i, deck.Peek(i), data[len(data)-1-i]) }
    
    p := deck.Put(i,"x") 
    if p != data[i] { return fmt.Sprintf("Put(%v,x) is %v but should be %v", i, p, data[i]) }
    if deck.Put(i, p).(string) != "x" { return fmt.Sprintf("Put is broken") }
    p = deck.Poke(i,"x")
    if p != data[len(data)-1-i] { return fmt.Sprintf("Poke(%v,x) is %v but should be %v", i, p, data[len(data)-1-i]) }
    if deck.Poke(i, p).(string) != "x" { return fmt.Sprintf("Poke is broken") }
    
    if deck.At(i) != data[i] { return fmt.Sprintf("After Put/Poke At(%v) is %v but should be %v", i, deck.At(i), data[i]) }
    if deck.Peek(i) != data[len(data)-1-i] { return fmt.Sprintf("After Put/Poke Peek(%v) is %v but should be %v", i, deck.Peek(i), data[len(data)-1-i]) }
  }
  var zero interface{}
  if deck.Peek(-1) != zero { return fmt.Sprintf("Peek(-1) returns %v", deck.Peek(-1)) }
  if deck.Peek(-1) != nil { return fmt.Sprintf("Peek(-1) returns %v", deck.Peek(-1)) }  
  if deck.Poke(-1,"foo") != zero { return fmt.Sprintf("Poke(-1,foo) returns not nil") }
  if deck.Poke(-1,"foo") != nil { return fmt.Sprintf("Poke(-1,foo) returns not nil") }  
  
  if deck.At(-1) != zero { return fmt.Sprintf("At(-1) returns %v", deck.At(-1)) }
  if deck.At(-1) != nil { return fmt.Sprintf("At(-1) returns %v", deck.At(-1)) }    
  if deck.Put(-1,"foo") != zero { return fmt.Sprintf("Put(-1,foo) returns not nil") }
  if deck.Put(-1,"foo") != nil { return fmt.Sprintf("Put(-1,foo) returns not nil") }    
  
  if deck.Peek(len(data)) != zero { return fmt.Sprintf("Peek(len(data)) returns %v", deck.Peek(len(data))) }
  if deck.Peek(len(data)) != nil { return fmt.Sprintf("Peek(len(data)) returns %v", deck.Peek(len(data))) }  
  if deck.Poke(len(data),"foo") != zero { return fmt.Sprintf("Poke(len(data),foo) returns not nil") }
  if deck.Poke(len(data),"foo") != nil { return fmt.Sprintf("Poke(len(data),foo) returns not nil") }  
  
  if deck.At(len(data)) != zero { return fmt.Sprintf("At(len(data)) returns %v", deck.At(len(data))) }
  if deck.At(len(data)) != nil { return fmt.Sprintf("At(len(data)) returns %v", deck.At(len(data))) }  
  if deck.Put(len(data),"foo") != zero { return fmt.Sprintf("Put(len(data),foo) returns not nil") }
  if deck.Put(len(data),"foo") != nil { return fmt.Sprintf("Put(len(data),foo) returns not nil") }  
  
  return ""
}

