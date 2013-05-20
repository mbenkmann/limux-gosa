/* Copyright (C) 2012 Matthias S. Benkmann
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this file (originally named deque.go) and associated documentation files 
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

// A goroutine-safe universal buffer/container/queue/stack/vector/list.
//
// Deque can serve
// as any of the basic bread-and-butter containers, is suitable for
// inter-goroutine communications (both blocking and non-blocking), can be
// either fixed size or expand dynamically (with configurable tradeoff
// between memory usage and speed). Except for indexed insertions/removals 
// (which are O(min(index,Count()-index)) all operations are (amortized) O(1).
// It's easy to use, with pleasing method names that fit the various use cases.
//
// Example 1: Use as a stack
//   var stack deque.Deque
//   stack.Push(...)
//   ...
//   // print the whole stack beginning with the top
//   for i:=0; i < stack.Count(); i++ { 
//     fmt.Println(stack.Peek(i))
//   }
//   ...
//   a := stack.Pop()
//
// Example 2: Use as FIFO work queue with fixed size 32 in a
// producer-consumer scenario. If the consumer is too slow and the 
// queue gets filled, the producer will block until a slot is free.
//
//   queue := deque.New(32, deque.BlockIfFull)
//
//   go func() {
//     for i:=0; i < 100; i++ {
//       a := produce()
//       queue.Push(a)
//     }
//
//     queue.Push(nil)  // signal consumer to terminate
//     queue.WaitForEmpty(0)
//     fmt.Println("All done!")
//
//   }()
//
//   go func() {
//     for {
//       a := queue.Next()
//       if a == nil { break }
//       consume(a)
//     }
//   }()
//
// Example 3: Random access
//
//   intvec := deque.New([]int{1,2,3,4})
//
//   intvec.Push(5)
//   intvec.Push(6)
//   intvec.Insert(0)
//   
//   // This loop prints 0,1,2,3,4,5,6
//   for i:=0; i < intvec.Count(); i++ {
//     fmt.Println(intvec.At(i))
//   }
//
//   // Remove all even numbers
//   for i:=0; i < intvec.Count(); {
//     if intvec.At(i).(int) % 2 == 0 {
//       intvec.RemoveAt(i)
//     } else {
//       i++
//     }
//   }
//
// Example 4: Sort a list
//
//   lst := deque.New([]int{3,1,10,4,0})
//   lst.Sort(func(a,b interface{})int{return a.(int)-b.(int)})
//
// Example 5: Keep track of the 16 most recently used items
//
//   mru := deque.New(16, deque.DropFarEndIfOverflow)
//   ...
//     // make item the most recent
//     // we remove an old entry first to avoid duplicates
//   mru.Remove(item)
//   mru.Push(item)
//   ...
//     // get the most recent item (it remains in mru)
//   item := mru.Peek(0)
//
// Notes about design decisions:
//  At() is not called Get()
//       because Get() could be a function that removes the element it returns,
//       whereas At() is less likely to be misinterpreted in this manner.
//       Also queue.Get(3) could be misinterpreted as getting the next 3 elements.
//  Next() is not called Remove() (which would fit with Insert()) because
//       Next() is shorter and in the typical use case of a FIFO buffer,
//       where a consumer processes items in a loop, Next() sounds more
//       natural. Furthermore deck.Remove() could be misconstrued as removing
//       the deque (e.g. freeing memory). Furthermore since Go does not allow
//       overloaded functions, Remove() could not be used as Remove(item).
//  Set() is not used, because intvec.Set(1,2) could be read as initializing the
//       whole list with [1,2].
//  Put() is not called PutAt() because Put() is shorter and the fact that it
//       takes 2 argument makes it hard to misinterpret.
//  "Front" and "Back" pairs such as PushFront()/PushBack() are not used because
//       front and back would be arbitrary designations when used with a
//       symmetric data structure and there is not typical use case where
//       you would use both ends as stack simultaneously. So it's much more
//       desirable to have the shorter and nicer sounding Push()/Pop() and
//       Push()/Next() pairs.
//  "First" and "Last" are not used for similar reasons as "Front" and "Back".
//  Same goes for "Top", "Bottom", "Start" and "End".
//  Enqueue() and Dequeue() are not used because they are cumbersome words and
//       one of them would either have to be a redundant alias for Push() or Pop()
//       or would have to be left out, which would surprise users.
//  Append() and Prepend() may be desirable to have for some use cases where they
//       are natural words, but they would be redundant which is undesirable and
//       the other names they compete with seem more desirable to have. There's
//       also the problem that they have no natural companions for popping the
//       respective elements.
//  Len() and Size() are not used because they could refer to the size/length of
//       the allocated buffer rather than the number of elements contained.
//  Overcapacity() was chosen instead of SetCapacity()/Reserve() because it 
//       has clear and useful semantics over the whole range of its argument
//       and because it makes TrimToSize() unnecessary.
//  Cat() is not called Append() because Append() could refer to appending a
//       single item, so that it's not clear if Append(stack) appends the
//       contents of stack or the stack object itself.
//       The same argument counts against Add(). AddAll() would be clear but
//       is more cumbersome than Cat().
//  The naming "inconsistency" between BlockIfFull() and FooIfOverflow() is
//       deliberate, because "overflow" is a condition where some data is lost
//       but BlockIfFull() does not lose data because eventually
//       the item will be written to the queue.
package deque

import (
          "fmt"
          "sync"
          "time"
       )

/*********************************************************************************

                   STRUCTURE DEFINITION

**********************************************************************************/

// It is sufficient to declare a variable of type Deque. The Deque will be 
// initialized with default values when one of its methods is called for the first
// time. For more control over the Deque's behaviour you can use the New()
// constructor or the Init() function.
type Deque struct {
  // See the documentation for the type GrowthFunc.
  Growth GrowthFunc
  // Counts the number of times Growth() has been called.
  GrowthCount uint
  // The Mutex that protects this Deque against concurrent access. By locking
  // this mutex you can block all of the Deque's methods. You should lock this
  // mutex before changing GrowthFunc or GrowthCount unless you can otherwise
  // guarantee that no goroutine will access the Deque concurrently.
  Mutex sync.Mutex
  // The following 3 slices are used for waiting for the respective conditions.
  // A waiter will create a buffered channel and append it to the respective list,
  // then wait for a signal on that channel.
  hasItem []chan bool
  isEmpty []chan bool
  hasSpace []chan bool
  data []interface{}
  // Current item count. Not to be confused with capacity (which is len(data)).
  count int
  // Index of the element At(0). The next Insert() call will write to
  // (a-1+capacity) % capacity .
  // a==b occurs if the Deque is either empty or full. count distinguishes these cases.
  a int
  // Index that the next Push() will write to (if the Deque is not full).
  // Peek(0) returns the item at index (b-1+capacity) % capacity.
  //  If a<b, then the slice data[a:b] contains all items in order.
  //  If a>b, then data[a:] is the 1st and data[:b] the 2nd part.
  //  If a==b and count == len(data), then data[a:] is the 1st and data[:b] the 2nd part.
  //  If a==b and count == 0, then data[a:b] (an empty slice) contains all items.
  // Combined that is:
  //  If a<b || count == 0, all items are found in data[a:b]
  //  Otherwise all items are found in data[a:] followed by data[:b]
  b int
}


/*********************************************************************************

                   GROWTH FUNCTIONS

**********************************************************************************/

// When new items are to be added to the deque and there is not enough
// capacity, the deque's Growth() function is called.
//  current: The deque's current capacity.
//  additional: Additional capacity needed to complete the current operation.
//              This is usually 1.
//  growthcount: The deque's GrowthCount field. It starts out at 0 and is
//               incremented after every call to the Growth() function.
// Returns: 
//  How much to increase the capacity. If this is less than the requested
//  additional capacity, the operation will partially complete and then block
//  until other goroutines remove items to make more space.
//
// Note: If Growth() panics, the deque remains unchanged and valid.
type GrowthFunc func (current, additional, growthcount uint) uint

// The default Growth() function to use if no other is specified. 
var GrowthDefault GrowthFunc = Double

// Some GrowthFuncs use this to control growth.
var GrowthFactor uint = 16

// If Growth() returns this, the behaviour is as DropFarEndIfOverflow().
const DROP_FAR_END = ^uint(0)
// If Growth() returns this, the behaviour is as DropNearEndIfOverflow().
const DROP_NEAR_END = DROP_FAR_END - 1
// If Growth() returns this, the behaviour is as DiscardIfOverflow().
const DISCARD = DROP_FAR_END - 2

// The GrowthFunc Double() doubles the capacity every time it is called (multiple
// times if necessary to satisfy the requested additional capacity). 
// This is the familiar behaviour of "vector" classes in other languages that
// results in amortized constant time insertion performance.
// Double is a good choice if your application typically creates Deques without
// knowing how many elements they will need to hold eventually. If you have a
// large number of long-lived Deques and need to reduce memory usage, you can
// call Overcapacity(0) when you're done inserting to free wasted memory.
func Double(current, additional, growthcount uint) uint {
  capa := current*2
  if capa == 0 { capa++ }
  for ; capa-current < additional ; capa*=2 {}
  return capa-current
}

// Exponential() grows the capacity by GrowthFactor<<(growthcount+x) where
// x is the minimum necessary (typically 0) to satisfy the requested
// additional capacity. The performance characteristics of Exponential() are
// the same as for Double(). The difference lies in the fact that Exponential()
// can be tuned via GrowthFactor and is independent of the Deque's current
// capacity. Exponential() can be used to have a Deque start out small but
// grow in large increments right away or have it start out large but start growing
// in small increments. Most of the time you should just use Double().
//
// Note: A GrowthFactor of 0 is treated as 1.
func Exponential(current, additional, growthcount uint) uint {
  fac := GrowthFactor
  if fac == 0 { fac = 1 }
  for ; (fac << growthcount) < additional; growthcount++ {}
  return fac << growthcount
}

// Accelerated() grows the capacity by (growthcount+1)*GrowthFactor, unless
// that's less than the requested additional capacity in which case it will grow
// by exactly the requested additional size.
// If you have many long-lived Deques and worry about Double()'s
// exponential space wastage but can't be bothered to gather hard data with
// a memory profiler, Accelerated() is a good way to get those worries off your
// back. It's a feel-good choice that's unlikely to cause problems no matter
// how you use Deques.
func Accelerated(current, additional, growthcount uint) uint {
  growth := (growthcount+1) * GrowthFactor
  if growth < additional { growth = additional }
  return growth
}

// GrowBy(n) grows the capacity by the fixed number n or, if n is too small, by
// exactly the requested additional capacity. This means that GrowBy(0) and GrowBy(1)
// cause the Deque to grow exactly as much as needed without wasted space.
// Don't forget that a Deque's capacity never shrinks automatically 
// (use Overcapacity() to shrink manually), so even with GrowBy(1) a Deque may waste
// some space if elements are removed after it has grown.
// GrowBy(n) causes insertion performance to degrade to O(Count()) whenever
// the Deque needs to grow. Therefore GrowBy(n) is a bad choice with respect to
// speed if your application's typical usage is to create Deques without specifying
// an initial capacity, then to add items and once all items have been added to
// perform only random access on the Deque (i.e. no deletions and insertions).
// If, on the other hand, most of your Deques are continuously used for insertions
// and deletions over the lifetime of your application (e.g. when they're used
// as buffers between producers and consumers), the initial costs for
// growing the Deques may not matter because once a Deque is large enough, insertions
// are O(1). In use cases like this GrowBy(n) can be a good choice because it avoids
// wasted memory. This applies even more to use cases where the initial capacity
// (either specified when creation the Deque or by CapacityDefault) is sufficient
// most of the time and Deques rarely need to grow.
func GrowBy(n uint) GrowthFunc {
  f := func (current,additional,growthcount uint) uint {
    if additional > n { return additional }
    return n
  }
  return f
}

// A Growth() function that never grows the capacity which causes insertions
// to block if the Deque has no free space, until some other goroutine
// removes items from the Deque.
// A Deque with this Growth() function is somewhat similar to a buffered Go channel.
func BlockIfFull(uint, uint, uint) uint { return 0 }

// PanicIfOverflow panics with this error.
var Overflow = fmt.Errorf("Deque overflow")

// Panics with the error deque.Overflow. A Deque with this Growth() function is
// a fixed size buffer that treats the attempt to add items to a full a buffer
// as an error rather than blocking. The operation that attempted to add more
// elements will have no effect and the Deque will remain valid.
func PanicIfOverflow(uint, uint, uint) uint { panic(Overflow) }

// New items added to a Deque with no empty space will cause old items to be
// "pushed out at the far end". This means that the item that is dropped will be
// the item whose index is the farthest away from the insertion point of the
// new item. E.g. a Push() on a full Deque will cause the item At(0) to be dropped,
// whereas an Insert() will cause the item Peek(0) to be dropped.
// If the insertion point is exactly in the middle (e.g. InsertAt(1,item) on a 
// 2-slot Deque), it is unspecified at which end the drop will occur.
//
// ATTENTION! If an item is dropped to make room, the index of the item
// after the insertion may be different from the index you passed to the
// insertion function. E.g. after InsertAt(2,item) on a 3-element Deque,
// the inserted item will be At(1). Conceptually the insertion occurs before
// the old item is pushed out, i.e. the new item is inserted At(2) and then
// RemoveAt(0) removes the item at the far end, which causes the new item's
// index to decrease. While this may sound complicated, most of the time
// you won't use DropFarEndIfOverflow in combination with indexed access.
// When used as in Example 5, none of this confusion occurs.
//
// NOTE: If the Deque's capacity is 0, the new item is discarded.
func DropFarEndIfOverflow(uint, uint, uint) uint { return DROP_FAR_END }

// The counterpart to DropFarEndIfOverflow that drops items at the near end.
// See DropFarEndIfOverflow() for more information.
//
// ATTENTION! As explained at DropFarEndIfOverflow(), conceptually the insertion
// happens before dropping an element. This means that with functions
// that append to either end such as Push() and Insert() DropNearEndIfOverflow
// is equivalent to DiscardIfOverflow.
func DropNearEndIfOverflow(uint, uint, uint) uint { 
  panic("TODO: Add DROP_NEAR_END case to insertAt()")
  return DROP_NEAR_END
}

// When new items are added to a Deque with no empty space, the new items
// are discarded and the Deque remains unchanged. 
// Like PanicIfOverflow() without the panic :-)
// One use case of this function is to temporarily disable a consumer without
// having to notify/disrupt/block producers and without having to worry about
// unbounded buffer growth.
func DiscardIfOverflow(uint, uint, uint) uint { return DISCARD }


/*********************************************************************************

                   CREATING AND INITIALIZING DEQUES

**********************************************************************************/

// The default capacity for a Deque if none is specified on creation.
var CapacityDefault uint = 16

// Creates a new Deque, calls Init(args) on it and returns a pointer to it. 
func New(args... interface{}) *Deque {
  var d Deque
  return d.Init(args...)
}

// Releases the buffer currently used by the Deque and starts a fresh buffer
// with CapacityDefault unless args override the start capacity. 
//
// The following arguments may be used (in any order)
//
//  initial capacity (int): How many items may be added before the 1st Growth().
//                          If multiple capacity arguments are passed, the last one
//                          wins.
//  
//  items ([]interface{},*Deque): 
//                         All []interface{} slices and *Deques passed to Init()
//                         are concatenated to form the initial contents of the Deque.
//                         It is permissible to pass the same slice or Deque multiple
//                         times and it is permissible to pass the Deque on which
//                         Init() is being called itself. All involved Deques will
//                         be locked before concatenation begins and will not be
//                         unlocked until the complete list has been assembled.
//                         The 1st element of the first slice in args will be
//                         At(0) and the last element of the last slice will be
//                         Peek(0) (i.e. the stack top). This corresponds to
//                         Push()ing all elements onto the Deque in order.
//                         If a capacity argument is passed anywhere in args that
//                         is smaller than the concatenated items list, the
//                         capacity argument will be ignored.
//
//  Growth (GrowthFunc): The Growth() function to use.
//
// The GrowthCount is reset to 0. If Init() is called on an uninitialized Deque,
// the Growth function will be set to GrowthDefault (unless overridden by args),
// but if Deque has already been initialized, Growth will remain unchanged (unless
// overridden by args).
//
// Waiters currently blocked on the queue will be woken as appropriate.
// E.g. if the Deque is currently full and uses Growth=BlockIfFull and a goroutine
// is blocked on Push(), then that goroutine will be awoken if Init() causes
// the Deque to have free space.
//
// It is unspecified whether waiting operations
// will immediately call Growth() again if not enough space is available
// for the operation or if they will block instead.
//
// It is unspecified whether changing the Growth() function from/to
// BlockIfFull or PanicIfOverflow affects waiting operations that were initiated
// while the previous Growth() function was in effect.
func (self *Deque) Init(args... interface{}) *Deque {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  return self.init(args...)
}

// Calls Init() with no arguments. Exists for situations where the name Clear()
// is better at conveying the intention than Init().
func (self *Deque) Clear() *Deque {
  return self.Init()
}


/*********************************************************************************

                   SIZE AND CAPACITY

**********************************************************************************/

// Returns the number of items in the Deque, not to be confused with Capacity().
func (self *Deque) Count() int { 
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  if self.data == nil { self.init() }
  return self.count
}

// Returns the total number of items that can be stored in the Deque without calling
// Growth(). The REMAINING capacity, i.e. the number of Push() calls that can be
// performed before Growth() has to be called is Capacity()-Count().
// You can use Overcapacity() to free the Capacity()-Count() of "wasted" memory.
func (self *Deque) Capacity() int { 
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  if self.data == nil { self.init() }
  return len(self.data)
}

// Changes the internal buffer to have the requested remaining capacity (i.e.
// the number of Push() calls that can be executed before Growth() has to be called).
// When you know that a buffer has reached the maximum number of elements it
// will ever hold, you can use Overcapacity(0) to free wasted memory.
// Overcapacity() can be used with a non-0 number to reserve memory ahead of
// adding a known number of items to avoid expensive calls to Growth().
//
// Optimization note: Overcapacity() may create a new buffer of the desired
// length and copy the data into it. It is therefore an expensive operation.
// However if remaining is already the current remaining capacity, this is avoided.
// So there's no point checking for this yourself.
func (self *Deque) Overcapacity(remaining uint) *Deque { 
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  if self.data == nil { self.init() }
  r := len(self.data) - self.count
  if uint(r) != remaining {
    new_data := make([]interface{},0,self.count + int(remaining))
    if self.a < self.b || self.count == 0 {
      new_data = append(new_data, self.data[self.a:self.b]...)
    } else {
      new_data = append(new_data, self.data[self.a:]...)
      new_data = append(new_data, self.data[:self.b]...)
    }
    self.data = new_data[0:cap(new_data)]
    self.a = 0
    self.b = self.count
    if self.b == len(self.data) { self.b = 0 }
  }
  return self
}

// Returns true iff no items are in the Deque. Be wary of race conditions in
// concurrent programs. By the time one goroutine evaluates the return value from
// IsEmpty(), another may have already added an item.
// Use WaitForEmpty() or WaitForItem() instead of busy waiting on IsEmpty().
// Note that for a 0-capacity Deque IsFull() and IsEmpty() are both true.
func (self *Deque) IsEmpty() bool { return self.Count()==0 }

// Returns true iff adding an item without removing one first
// will call the Growth() function.
// Be wary of race conditions in
// concurrent programs. By the time one goroutine evaluates the return value from
// IsFull(), another may have already removed an item.
// Note that for a 0-capacity Deque IsFull() and IsEmpty() are both true.
func (self *Deque) IsFull() bool { 
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  if self.data == nil { self.init() }
  return len(self.data) == self.count
}

// Blocks until either timeout has elapsed or at least one item is in the Deque.
// Returns true if the function returned because of an item and false if it
// returned because of the timeout. 0 means wait as long as necessary.
// Be wary of race conditions in concurrent programs. That this function returns
// true does not mean that a subsequent Pop() won't block, because a concurrent
// goroutine may have emptied the Deque again.
func (self *Deque) WaitForItem(timeout time.Duration) bool {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  if self.data == nil { self.init() }
  if self.count > 0 { return true }
  return self.waitFor(&self.hasItem, timeout)
}

// Blocks until either timeout has elapsed or at least one free slot is available
// for a new item.
// Returns true if the function returned because of a free slot and false if it
// returned because of the timeout. 0 means wait as long as necessary.
// Be wary of race conditions in concurrent programs. That this function returns
// true does not mean that a subsequent Push() won't block, because a concurrent
// goroutine may have filled up the Deque again.
//
// Note that this function will not attempt to Grow() the Deque, so if the Deque
// IsFull() this function will block, even if Grow() could add more space.
func (self *Deque) WaitForSpace(timeout time.Duration) bool { 
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  if self.data == nil { self.init() }
  if self.count < len(self.data) { return true }
  return self.waitFor(&self.hasSpace, timeout)
}

// Blocks until either timeout has elapsed or no items remain in the Deque.
// Returns true if the function returned because of an empty Deque and false if it
// returned because of the timeout. 0 means wait as long as necessary.
// Be wary of race conditions in concurrent programs. That this function returns
// true does not mean that the Deque is actually empty by the time the caller
// gets the return value. A concurrent goroutine may have added an item in the
// meantime.
func (self *Deque) WaitForEmpty(timeout time.Duration) bool {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  if self.data == nil { self.init() }
  if self.count == 0 { return true }
  return self.waitFor(&self.isEmpty, timeout)
}



/*********************************************************************************

                   STACK METHODS

**********************************************************************************/


// Makes item the new stack top. After this, Peek(0), Pop(), PopAt(0) and 
// At(Count()-1) will return item. 
// If the Deque is full, Growth() will be called beforehand.
// Returns false iff the item was discarded due to Growth().
func (self *Deque) Push(item interface{}) bool {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
restart:
  res := self.insertAt(self.count, item)
  if res < 0 { return false }
  if res == 0  { 
    self.waitFor(&self.hasSpace, 0)
    goto restart
  }
  return true
}

// Inserts item at index idx counting from the stack top. PushAt(0, item) is
// equivalent to Push(item).
// After calling this function, Peek(idx) and PopAt(idx) will return item.
//
// If the Deque is full, Growth() will be call beforehand.
//
// Returns false if idx is out of range (idx < 0 or idx > Count()) or
// if the item was discarded due to Growth().
//
// Note that Count() is a valid index for this function whereas for most other
// SomethingAt() functions Count()-1 is the maximum permissible index.
//
// Optimization note: PushAt() and InsertAt() have the exact same performance
// and automatically minimize the size of the memory block that needs to be moved.
// You can not optimize anything by choosing one or the other based on the index.
func (self *Deque) PushAt(idx int, item interface{}) bool { 
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
restart:
  res := self.insertAt(self.count-idx, item)
  if res < 0 { return false }
  if res == 0  { 
    self.waitFor(&self.hasSpace, 0)
    goto restart
  }
  return true
}

// Blocks until there is at least 1 item in the Deque, then removes and returns
// the stack top (i.e. the item returned by Peek(0) and At(Count()-1)).
//
// If you need a non-blocking Pop(), use PopAt(0).
func (self *Deque) Pop() interface{} {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  if self.data == nil { self.init() }
  for ; self.count == 0 ; {
    self.waitFor(&self.hasItem, 0)
  }
  return self.removeAt(self.count-1)
}

// If the Deque contains at least idx+1 items, this call removes and returns
// the idx-th item counting from the stack top (which is index 0).
// If idx is out of range (idx < 0 or idx >= Count()), nil is returned.
// PopAt(0) is the non-blocking version of Pop() (which waits for an item).
//
// Optimization note: PopAt() and RemoveAt() have the exact same performance
// and automatically minimize the size of the memory block that needs to be moved.
// You can not optimize anything by choosing one or the other based on the index.
func (self *Deque) PopAt(idx int) interface{} {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  return self.removeAt(self.count-1-idx)
}

// If the Deque contains at least idx+1 items, this call returns
// the idx-th item counting from the stack top (which is index 0).
// Unlike PopAt() Peek() does not remove the item from the stack.
// If idx is out of range (idx < 0 or idx >= Count()), nil is returned.
func (self *Deque) Peek(idx int) interface{} {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  return self.at(self.count-1-idx)
}

// If the Deque contains at least idx+1 items, this call replaces
// the idx-th item counting from the stack top (which is index 0) with
// the new item and returns the old item.
// If idx is out of range (idx < 0 or idx >= Count()), nil is returned.
func (self *Deque) Poke(idx int, item interface{}) interface{} {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  return self.put(self.count-1-idx, item)
}



/*********************************************************************************

                   VECTOR/LIST/QUEUE METHODS

**********************************************************************************/

// Makes item the new first element. After this, At(0), Next(), RemoveAt(0) and 
// PopAt(Count()-1) will return item. 
// If the Deque is full, Growth() will be called beforehand.
// Returns false iff the item was discarded due to Growth().
func (self *Deque) Insert(item interface{}) bool {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
restart:
  res := self.insertAt(0, item)
  if res < 0 { return false }
  if res == 0  { 
    self.waitFor(&self.hasSpace, 0)
    goto restart
  }
  return true
}

// Makes item the new idx-th element (counting from the bottom). 
// InsertAt(0, item) is equivalent to Insert(item).
// After calling this function, At(idx) and RemoveAt(idx) will return item.
//
// If the Deque is full, Growth() will be call beforehand.
//
// Returns false if idx is out of range (idx < 0 or idx > Count()) or
// if the item was discarded due to Growth().
//
// Note that Count() is a valid index for this function whereas for most other
// SomethingAt() functions Count()-1 is the maximum permissible index.
//
// Optimization note: PushAt() and InsertAt() have the exact same performance
// and automatically minimize the size of the memory block that needs to be moved.
// You can not optimize anything by choosing one or the other based on the index.
func (self *Deque) InsertAt(idx int, item interface{}) bool {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
restart:
  res := self.insertAt(idx, item)
  if res < 0 { return false }
  if res == 0  { 
    self.waitFor(&self.hasSpace, 0)
    goto restart
  }
  return true
}

// Blocks until there is at least 1 item in the Deque, then removes and returns
// the 1st element (i.e. the item returned by At(0) and Peek(Count()-1)).
//
// If you need a non-blocking Next(), use RemoveAt(0).
func (self *Deque) Next() interface{} {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  if self.data == nil { self.init() }
  for ; self.count == 0 ; {
    self.waitFor(&self.hasItem, 0)
  }
  return self.removeAt(0)
}

// If the Deque contains at least idx+1 items, this call removes and returns
// the idx-th item (i.e. the item returned by At(idx)).
// If idx is out of range (idx < 0 or idx >= Count()), nil is returned.
// RemoveAt(0) is the non-blocking version of Next() (which waits for an item).
//
// Optimization note: PopAt() and RemoveAt() have the exact same performance
// and automatically minimize the size of the memory block that needs to be moved.
// You can not optimize anything by choosing one or the other based on the index.
func (self *Deque) RemoveAt(idx int) interface{} {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  return self.removeAt(idx)
}

// If the Deque contains at least idx+1 items, this call returns
// the idx-th item.
// At() does not remove the item from the stack. Use RemoveAt() for that.
// If idx is out of range (idx < 0 or idx >= Count()), nil is returned.
func (self *Deque) At(idx int) interface{} {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  return self.at(idx)
}

// If the Deque contains at least idx+1 items, this call replaces
// the idx-th item (i.e. the item returned by At(idx)) with the new item
// and returns the old item.
// If idx is out of range (idx < 0 or idx >= Count()), nil is returned.
func (self *Deque) Put(idx int, item interface{}) interface{} {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  return self.put(idx, item)   
}




/*********************************************************************************

                   STRUCTURAL METHODS

**********************************************************************************/

// Takes any combination of []interface{} slices and *Deques and appends 
// all their elements in order to the Deque's current list of elements.
// It is permissible to
// pass the Deque itself as an argument and it is permissible to pass the same
// Deque or slice multiple times. All Deques involed will be locked
// for the call, so if the same Deque is listed multiple times as argument, it
// is not possible that the resulting list contains different states of the
// same Deque even in the presence of concurrent modifications.
// If the target Deque uses BlockIfFull, the elements that won't fit will be
// copied to a temporary buffer and the locks will be released. Then Cat() will
// block until space is available in the Deque and append more elements, repeating
// this as often as necessary.
// Cat() returns the Deque for chaining.
func (self *Deque) Cat(lst... interface{}) *Deque { 
  panic("TODO: Implement Cat()")
  return self 
}

// Swaps the items At(i) and At(j). Returns nil if either index is
// out of range, otherwise the Deque is returned.
func (self *Deque) Swap(i, j int) *Deque { 
  panic("TODO: Implement Swap()")
  return self
}

// Reverses the order of all elements, i.e. swapping At(i)<->Peek(i) for all i.
// Returns the Deque.
func (self *Deque) Reverse() *Deque { 
  panic("TODO: Implement Reverse()")
  return self
}

// Performs a stable sort of the elements of the Deque in ascending order,
// if cmp is a function
// that returns a negative value if its first argument is less than the second,
// a positive value if it is greater and 0 if the arguments are equal.
// Returns the Deque.
func (self *Deque) Sort(cmp func(interface{},interface{}) int) *Deque { 
  panic("TODO: Implement Sort()")
  return self 
}

// Removes all instances that are equal to item, either per operator == or by
// the notion of equality of a comparison function (see Sort()) passed as 
// optional second argument.
// Returns the number of items removed.
func (self *Deque) Remove(item interface{}, cmp... func(interface{},interface{}) int) int {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  if self.data == nil { self.init() }
  
  count := 0
  
  switch len(cmp) {
    case 0:
      for i:=self.count-1; i >= 0 ; i-- {
        idx := self.a + i
        if idx >= len(self.data) { idx -= len(self.data) }
        if self.data[idx] == item { self.removeAt(i); count++ } 
      }
    case 1:
      for i:=self.count-1; i >= 0 ; i-- {
        idx := self.a + i
        if idx >= len(self.data) { idx -= len(self.data) }
        if cmp[0](self.data[idx], item) == 0 { self.removeAt(i); count++ } 
      }
    default: panic("Remove() takes 1 or 2 parameters")
  }

  return count
}

// Returns the index of the first element that compares equal
// to the given item according to operator == or the notion of equality of
// a comparison function (see Sort()) passed as optional second argument.
// Returns -1 if no such element exists.
func (self *Deque) IndexOf(item interface{}, cmp... func(interface{},interface{}) int) int {
  panic("TODO: Implement IndexOf()")
  return -1 
}

// When called on a Deque that has been sorted by Sort() with the same cmp() function
// as passed to Search(), the latter will perform a binary search for the given item
// and return the smallest index idx so that InsertAt(idx, item) keeps the Deque sorted.
func (self *Deque) Search(item interface{}, cmp func(interface{},interface{}) int) int { 
  panic("TODO: Implement Search()")
  return self.Count() 
}

// Inserts item into the deque so that if it was sorted according to Sort(cmp) it
// remains sorted. Returns idx so that At(idx) is the inserted item. If the item
// was discarded due to the Growth function (e.g. DiscardIfOverflow) InsertSorted()
// returns -1.
//
//  NOTE: This call is preferable to calling Search() and InsertAt() separately 
//  because this call is atomic.
//
//  NOTE: Like Insert() this function may block if the deque's GrowthFunction is
//  BlockIfFull.
func (self *Deque) InsertSorted(item interface{}, cmp func(interface{},interface{}) int) int { 
  panic("TODO: Implement InsertSorted()")
  return -1
}

// Returns true if the Deque contains an item that compares equal to the given
// item according to operator == or a comparison function (see Sort()) passed
// as optional 2nd argument.
func (self *Deque) Contains(item interface{}, cmp... func(interface{},interface{}) int) bool { 
  return self.IndexOf(item, cmp...)>=0 
}

// Panics if the Deque is broken. As this could only happen if there were
// a bug in the code, this function will never do that. Well, actually it
// also panics if YOU have set Growth to nil (which is a bug in your code).
// So how about this: If this function panics, there's a bug in
// your application ;-)
func (self *Deque) CheckInvariant() {
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  if self.count < 0 || self.count > len(self.data) { panic("invariant broken") }
  if self.a < 0 || self.b < 0 { panic("invariant broken") }
  if len(self.data) != 0 && (self.a >= len(self.data) || self.b >= len(self.data)) { panic("invariant broken") }
  if len(self.data) == 0 && (self.a != 0 || self.b != 0) { panic("invariant broken") }
  if self.a == self.b && self.count != 0 && self.count != len(self.data) { panic("invariant broken") }
  if (self.count == 0 || self.count == len(self.data)) && self.a != self.b { panic("invariant broken") }
  if self.a < self.b && self.count != self.b-self.a { panic("invariant broken") }
  if self.b < self.a && self.count != (self.b + len(self.data)-self.a) { panic("invariant broken") }
  if self.Growth == nil && self.data != nil { panic("invariant broken") }
  for _, ar := range []*[]chan bool{&self.hasItem,&self.isEmpty,&self.hasSpace} {
    for _, c := range *ar {
      if len(c) != 0 || cap(c) != 2 { panic("invariant broken") }
    }
  }
  if len(self.data) != cap(self.data) { panic("invariant broken") }
  if self.count == 0 && len(self.isEmpty) != 0 { panic("invariant broken") }
  if self.count != 0 && len(self.hasItem) != 0 { panic("invariant broken") }
  if self.count < len(self.data) && len(self.hasSpace) != 0 { panic("invariant broken") }
}

/*********************************************************************************

                   TYPE CONVERSIONS

**********************************************************************************/

// Returns a string representation of the Deque.
func (self *Deque) String() string { 
  self.Mutex.Lock()
  defer self.Mutex.Unlock()
  if self.data == nil { self.init() }
  buf := make([]string, self.count)
  for i:=0; i < self.count; i++ {
    j := self.a+i
    if j >= len(self.data) { j -= len(self.data) }
    buf[i] = fmt.Sprintf("%v", self.data[j])
  }
  return fmt.Sprintf("Deque%v",buf)
}

// Returns the internal ring buffer, rotated so that ring[idx0] is element At(0).
// If the argument index0 >= 0, then idx0 will be (index0 % Capacity()).
// If index0 < 0, the internal ring buffer is returned as is without any
// rotation being performed.
// The returned slice will have length Capacity(). If Count() < Capacity(), then
// parts of the slice are currently unused.
// WARNING! This function does not initialize the Deque, and doesn't lock the Deque
// at all, even while performing the
// rotation. To use this function safely in a situation where concurrent goroutines
// may access the Deque in any way, requires that you lock deque.Mutex before
// calling Raw() and unlock it when you are finished working with the data.
// It is possible (and in fact one of the intended use cases) to modify the elements
// of the returned slice (e.g. sorting them), but adding or removing elements 
// will not work, because you cannot modify the element count.
func (self *Deque) Raw(index0 int) (ring []interface{}, idx0 int) { 
  if len(self.data) == 0 { return self.data, 0 } // Avoid division by 0
  if index0 >= 0 {
    index0 = index0 % len(self.data)
    if index0 != self.a {
      new_buf := make([]interface{}, len(self.data))
      if index0 > self.a {
        // |-------A---0-----|
        copy(new_buf[index0:], self.data[self.a:])
        copy(new_buf[0:], self.data[self.a+len(self.data)-index0:])
        copy(new_buf[index0-self.a:], self.data[0:self.a])
      } else {
        // |-------0---A-----|
        copy(new_buf[index0:], self.data[self.a:])
        copy(new_buf[index0+len(self.data)-self.a:], self.data[0:])
        copy(new_buf[0:], self.data[self.a-index0:self.a])
      }
      self.data = new_buf
      self.a = index0
      self.b = self.a + self.count
      if self.b >= len(self.data) { self.b -= len(self.data) }
    }
  }
  return self.data, self.a
}



/*********************************************************************************

                   ITERATION

**********************************************************************************/

func (self* Deque) Iterator() Iterator {
  panic("TODO: Implement Iterator()")
  return nil
}

type Iterator interface{
}


/*********************************************************************************

                   IMPLEMENTATION (PRIVATE FUNCTIONS)

**********************************************************************************/
// like Init() but the caller is responsible for locking self.
func (self *Deque) init(args... interface{}) *Deque {
  locklist := map[*Deque]bool{self:true}
  
  // evaluate arguments
  requested_capacity := -1
  cat_capacity := -1
  var new_growth GrowthFunc
  for i, x := range args {
    switch arg := x.(type) {
      case *Deque: 
             if !locklist[arg] { 
               locklist[arg] = true
               arg.Mutex.Lock()
               defer arg.Mutex.Unlock()
             }
             if arg.data != nil { // protect against uninitialized Deques
               if cat_capacity < 0 { 
                 cat_capacity = arg.count 
               } else {
                 cat_capacity += arg.count
               }
             }
      case []interface{}: 
             if cat_capacity < 0 { 
               cat_capacity = len(arg)
             } else {
               cat_capacity += len(arg)
             }
      case int:  requested_capacity = arg
      case uint: requested_capacity = int(arg)
      case int64: requested_capacity = int(arg)
      case uint64: requested_capacity = int(arg)
      
      case GrowthFunc: new_growth = arg
      case func(uint, uint, uint) uint: new_growth = arg
      default: panic(fmt.Errorf("Argument #%d is unsupported by deque.Init()",i+1))
    }
  }
  
  if cat_capacity > requested_capacity { requested_capacity = cat_capacity }
  // Current implemention is to only use CapacityDefault if neither an explicit
  // capacity is requested nor initial values are provided. It would also be
  // a reasonable implementation to have the following line before the
  // previous line which would have the effect that if fewer initial values
  // are provided than CapacityDefault, the deque would be created with some
  // empty slots.
  if requested_capacity < 0 { requested_capacity = int(CapacityDefault) }
  
  new_data := make([]interface{}, 0, requested_capacity)
  
  // concatenate all initial items into new_data
  for _, x := range args {
    switch arg := x.(type) {
      case *Deque: 
             if arg.data != nil { // protect against uninitialized Deques
               if arg.a < arg.b || arg.count == 0 {
                 new_data = append(new_data, arg.data[arg.a:arg.b]...)
               } else {
                 new_data = append(new_data, arg.data[arg.a:]...)
                 new_data = append(new_data, arg.data[:arg.b]...)
               }
             }
      case []interface{}: new_data = append(new_data, arg...)
    }
  }
  
  self.count = len(new_data)
  self.a = 0
  self.b = self.count
  
  // grow new_data slice to full capacity
  new_data = new_data[0:cap(new_data)]
  // wrap around b if beyond end
  if self.b == len(new_data) { self.b = 0 }
  
  self.data = new_data
  
  if self.Growth == nil && new_growth == nil { new_growth = GrowthDefault }
  if new_growth != nil { self.Growth = new_growth }
  
  self.GrowthCount = 0
  
  // notify waiters based on new state
  if self.count > 0 { 
    for _,c := range self.hasItem { c <-true } 
    self.hasItem = self.hasItem[0:0]
  }
  if self.count < len(self.data) { 
    for _,c := range self.hasSpace { c <- true } 
    self.hasSpace = self.hasSpace[0:0]
  }
  if self.count == 0 { 
    for _,c := range self.isEmpty { c <- true } 
    self.isEmpty = self.isEmpty[0:0]
  }

  return self
}


func (self *Deque) waitFor(what *[]chan bool, timeout time.Duration) bool {
  c := make(chan bool, 2)
  *what = append(*what, c)
  self.Mutex.Unlock()
  defer self.Mutex.Lock()
  if timeout > 0 {
    go func(){
      time.Sleep(timeout)
      c <- false
    }()
  }
  return <-c // wait for signal or timeout
}



func (self *Deque) at(idx int) interface{} { 
  if self.data == nil { self.init() }
  if idx < 0 || idx >= self.count { return nil }
  idx += self.a
  if idx >= len(self.data) { idx -= len(self.data) }
  return self.data[idx]
}

func (self *Deque) put(idx int, item interface{}) interface{} { 
  if self.data == nil { self.init() }
  if idx < 0 || idx >= self.count { return nil }
  idx += self.a
  if idx >= len(self.data) { idx -= len(self.data) }
  old := self.data[idx]
  self.data[idx] = item
  return old
}


//*************************** removeAt() ******************************/
func (self *Deque) removeAt(idx int) interface{} {
  if self.data == nil { self.init() }
  if idx < 0 || idx >= self.count { return nil }
  idx += self.a
  if idx >= len(self.data) { idx -= len(self.data) }
  
  old := self.data[idx]
  
  if self.a < self.b { //simple case: |...A----idx----B...|
    if idx-self.a < self.b-idx { // idx closer to A than to B
      copy(self.data[self.a+1:],self.data[self.a:idx])
      self.a++
    } else { // idx closer to B than to A
      copy(self.data[idx:],self.data[idx+1:self.b])
      self.b--
    }
  } else { //harder case:   |--idx?--B.........A---idx?--|
    da := idx - self.a
    if da < 0 { da += len(self.data) }
    db := self.b - idx
    if db < 0 { db += len(self.data) }
    if da <= db { // |--idx?----------------B.........A--idx?-|
                  // including cases where B == A
                  // including the special case B == A == idx
      if idx < self.b { 
        copy(self.data[1:], self.data[0:idx])
        idx = len(self.data)-1
        self.data[0] = self.data[idx]
      }
      copy(self.data[self.a+1:], self.data[self.a:idx])
      self.a++
      if self.a == len(self.data) { self.a = 0 }
    } else { // |--idx?--B.........A-----------------idx?--|
             // including cases where B == A
      if idx >= self.a {
        copy(self.data[idx:],self.data[idx+1:])
        self.data[len(self.data)-1] = self.data[0]
        idx = 0
      }
      // self.b+1 is used instead of self.b, because
      // it is possible that self.b==0 and in that case
      // idx+1 > self.b and the Go specs say about slices that
      // low <= high must be satisfied.
      // The +1 causes one additional slot to be copied but
      // that doesn't matter because the slot it overwrites (self.b-1)
      // is definitely unused once we do self.b--.
      copy(self.data[idx:],self.data[idx+1:self.b+1])
      self.b--
      if self.b < 0 { self.b += len(self.data) }
    }
  }
  
  if self.count == len(self.data) { // we went from full to 1 available slot
    for _,c := range self.hasSpace { c <- true } 
    self.hasSpace = self.hasSpace[0:0]
  }
  self.count--

  if self.count == 0 { // deque is now empty
    for _,c := range self.isEmpty { c <- true } 
    self.isEmpty = self.isEmpty[0:0]
  }

  return old
}


//*************************** insertAt() ******************************/
// Returns <0  => item discarded or index out of range
//         >0 => item added successfully
//          0 => wait for free space, then try again
func (self *Deque) insertAt(idx int, item interface{}) int {
  if self.data == nil { self.init() }
  if idx < 0 || idx > self.count { // Note: self.count IS a valid idx for insertAt()!
    return -1
  }
  
  // If buffer is full, grow it or wait for an empty slot.
  if self.count == len(self.data) {
    growth := self.Growth(uint(len(self.data)), 1, self.GrowthCount)
    self.GrowthCount++
    switch growth {
      case 0: { // no growth => block until there's space
        return 0
      }
      
      case DROP_FAR_END: { // drop far end
        if len(self.data) == 0 { return -1 }
        if idx+idx > self.count { // A end is far end
          self.a++
          if self.a == len(self.data) { self.a = 0 }
          idx--
        } else { // B end is far end
          self.b--
          if self.b < 0 { self.b += len(self.data) }
        }
        self.count--
      }
      
      case DISCARD: { // discard the new item
        return -1
      }        
      
      default: { // grow buffer
        new_buf := make([]interface{}, len(self.data) + int(growth))
        // Note: self.a == self.b because buffer is full
        copy(new_buf, self.data[self.a:len(self.data)])
        copy(new_buf[len(self.data)-self.a:], self.data[0:self.a])
        self.a = 0
        self.b = self.count
        self.data = new_buf
        
        if growth > 1 { // if we grew more than necessary, signal waiters for space
          for _,c := range self.hasSpace { c <- true } 
          self.hasSpace = self.hasSpace[0:0]
        }
      }
    }
  }
  
  // At this point we have an empty slot. Let's insert!
  
  idx += self.a
  if idx >= len(self.data) { idx -= len(self.data) }
  
  if self.a <= self.b { //simple case: |...A----idx----B...|
                        //including the case A == B == idx (empty Deque)
    if idx-self.a < self.b-idx { // idx closer to A than to B
      a := self.a
      self.a--
      if self.a < 0 { self.a += len(self.data) }
      // copy 1st element separately because of possible wrap-around
      self.data[self.a] = self.data[a]
      
      // We use idx+1 because it is possible that idx==a and
      // the Go specs say that low <= high must be satisfied.
      // This may cause data[idx] to be copied to data[idx-1]
      // but this unnecessary copy doesn't hurt.
      copy(self.data[a:], self.data[a+1:idx+1])
      idx-- // insert before the item
      if idx < 0 { idx += len(self.data) }
    } else { // idx closer to B than to A 
             //   or 
             // A == B == idx (empty Deque)
      // because self.data[self.b] is an empty slot, the following
      // copy cannot wrap around unlike the "closer to A" case
      copy(self.data[idx+1:len(self.data)], self.data[idx:self.b])
      self.b++
      if self.b == len(self.data) { self.b = 0 }
    }
  } else { //harder case:   |--idx?--B.........A---idx?--|
           //Note that A != B because the Deque cannot be full and 
           //the case of an empty Deque is handled above.
    da := idx - self.a
    if da < 0 { da += len(self.data) }
    db := self.b - idx
    if db < 0 { db += len(self.data) }
    if da < db { // |--idx?----------------B.........A--idx?-|
      idx-- // insert before the item
      if idx < 0 { idx += len(self.data) }
      self.a-- // cannot wrap around
      if idx >= self.a { // |----------------------B.........A--idx--|
        copy(self.data[self.a:], self.data[self.a+1:idx+1])
      } else 
      { // |--idx-----------------B.........A-------|
        copy(self.data[self.a:], self.data[self.a+1:len(self.data)])
          // copy 1st element separately because of wrap-around
        self.data[len(self.data)-1] = self.data[0]
        copy(self.data[0:], self.data[1:idx+1])
      } 
    } else { // |--idx?--B.........A-----------------idx?--|
      if idx <= self.b { // |--idx--B.........A-----------------------|
        copy(self.data[idx+1:], self.data[idx:self.b])
        self.b++ // cannot wrap around
      } else
      { // |------B.........A-----------------idx--|
        copy(self.data[1:], self.data[0:self.b])
        self.data[0] = self.data[len(self.data)-1]
        copy(self.data[idx+1:len(self.data)], self.data[idx:len(self.data)-1])
        self.b++ // cannot wrap around
      }
    }
  }

  self.data[idx] = item

  if self.count == 0 { // we inserted into an empty deque => signal waiters for item
    for _,c := range self.hasItem { c <- true } 
    self.hasItem = self.hasItem[0:0]
  }
  self.count++
  
  return 1
}
