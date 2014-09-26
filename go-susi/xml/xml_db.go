/* Copyright (C) 2012 Matthias S. Benkmann
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this file (originally named xml_db.go) and associated documentation files 
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


package xml

import (
         "os"
         "sync"
         "path"
         "time"
         "io/ioutil"
       )

// Stores a database somewhere. E.g. FileStorer stores the data in a file.
type Storer interface{
  // Stores the given Hash and returns an error if something went wrong.
  // ATTENTION! Must be goroutine-safe!
  // db must not be modified, because it is the actual db, not a copy.
  Store(db *Hash) error
}

// a database based on an xml.Hash, with optional backing store.
type DB struct {
  // the data
  data *Hash
  // if not nil, the data stored in the database is persisted by calling
  // persist.Store(xmlstr) with a string representation of the database.
  // ATTENTION! Must be goroutine-safe.
  persist Storer
  // to avoid persisting the database on every write access, a delay
  // can be set here. This is the maximum time between a write access to
  // the database and the attempt to persist the data.
  persistDelay time.Duration
  // for locking the database.
  mutex sync.RWMutex
  // true if the job for persisting the database has been scheduled.
  blockPersistJobs bool
}

// Creates a new database.
//  name: must be a valid tag name. It will be the outer-most tag in the XML string
//        representation of the database.
// 
//  persist: If nil, the database will exist in memory only. Otherwise this
//           object will be used to make changes to the database persistent.
//           ATTENTION! The object must be goroutine-safe. The DB does NOT
//           guarantee that only one job is running.
//
//  persistDelay: The maximum time that may pass between writing to
//                the in-memory db and the job to persist the database. The job
//                may be delayed further due to concurrent database accesses.
//                Increasing this improves performance if there are many
//                write accesses, but increases the chances of data loss.
func NewDB(name string, persist Storer, persistDelay time.Duration) (*DB) {
  db := &DB{data:NewHash(name), persist:persist, persistDelay:persistDelay, blockPersistJobs:false}

  if persist == nil {
    db.persistDelay = 0
    db.blockPersistJobs = true
  }
  
  return db
}

// Stores a database in a file. Every Store()
// will replace the file via a write to a temporary file followed by an
// atomic rename to make sure the data is not completely lost if something 
// goes wrong updating the file.
type FileStorer struct {
  Path string
}

// Stores data in the file specified by the FileStorer's Path.
func (f *FileStorer) Store(db *Hash) (err error) {
  dir := path.Dir(f.Path)
  prefix := path.Base(f.Path)
  var temp *os.File
  temp, err = ioutil.TempFile(dir, prefix)
  if err != nil { return err }
  
  // WE DON'T defer os.Remove(temp.Name()) because we want to rename
  // the file after writing
  
  // Write out the data to the temp file
  _, err = db.WriteTo(temp)
  temp.Close()
  if err == nil {
    // atomically replace the old with the new file
    err = os.Rename(temp.Name(), f.Path)
  }
  
  // WE DON'T os.Remove(temp.Name()) in case of error 
  // because the user may need it for manual data recovery.
  return err
}

// Replaces the database with the given hash.
// ATTENTION! The database uses the data pointer directly. You must not
// access it after passing it to Init() or you will bypass the database's lock.
//
// NOTE: Calling this method will NOT cause a persist job to be triggered nor
// will it cancel an already pending persist job.
func (db *DB) Init(data *Hash) {
  db.mutex.Lock()
  defer db.mutex.Unlock()
  db.data.Destroy()
  db.data = data
}

// Immediately persists the database and then blocks all further access.
// This call does not return until the database has been persisted.
// WARNING! After calling this function all function calls on the database
// will block forever.
func (db *DB) Shutdown() {
  db.mutex.Lock()
  db.persistWithLock()
  // DO NOT db.mutex.Unlock()! The database has been shut down!
}

// Adds deep-copy clones of all items into the database.
// Returns the database reference (for chaining).
//
// NOTE: Calling this method will trigger a persist job if none is pending.
func (db* DB) AddClone(items... *Hash) (*DB) {
  db.mutex.Lock()
  defer db.mutex.Unlock()
  
  for _, item := range items {
    db.data.AddClone(item)
  }
  
  db.persistJob()
  
  return db
}

// Returns a *Hash whose outer tag has the same name as that of the db and
// whose child elements are deep copies of the database items selected by filter.
func (db *DB) Query(filter HashFilter) *Hash {
  // This is just a READ lock!
  db.mutex.RLock()
  defer db.mutex.RUnlock()
  return db.data.Query(filter)
}

// Returns all text contents from all database items' <column> subelements.
func (db *DB) ColumnValues(column string) []string {
  // This is just a READ lock!
  db.mutex.RLock()
  defer db.mutex.RUnlock()
  result := make([]string, 0, 4)
  for child := db.data.FirstChild(); child != nil; child = child.Next() {
    result = append(result, child.Element().Get(column)...)
  }
  return result
}

// Removes the items selected by the filter from the database.
// Returns a *Hash whose outer tag has the same name as that of the db and
// whose child elements are the removed items.
//
// NOTE: Calling this method will trigger a persist job if none is pending.
func (db *DB) Remove(filter HashFilter) *Hash {
  db.mutex.Lock()
  defer db.mutex.Unlock()
  
  result := db.data.Remove(filter)
  
  db.persistJob()
  
  return result
}

// Performs Remove(filter) and AddClone(items) as one atomic operation.
// The returned value are the removed items as for Remove().
// If must_match is true, the AddClone(items) will only be performed if at
// least one item matches the filter.
//
// NOTE: Calling this method will trigger a persist job if none is pending.
func (db *DB) Replace(filter HashFilter, must_match bool, items... *Hash) *Hash {
  db.mutex.Lock()
  defer db.mutex.Unlock()
  
  result := db.data.Remove(filter)

  if must_match == false || result.FirstChild() != nil {
    for _, item := range items {
      db.data.AddClone(item)
    }
  }

  db.persistJob()
  
  return result
}

// Launches a persist job unless blocked. REQUIRES HOLDING THE DB LOCK!
func (db *DB) persistJob() {
  if !db.blockPersistJobs {
    db.blockPersistJobs = true
    go func() {
      time.Sleep(db.persistDelay)
      db.Persist()
    }()
  }
}

// If the DB has a persist object set, its Store() will be called 
// immediately (i.e. as soon as the write-lock can be obtained).
// If the persist object works synchronously, this function will not
// return until the data has actually been stored.
// Returns the error from the Store() operation if any.
//
// This function will unblock the scheduling of persist jobs as
// per the persistDelay specified on database creation. This means that
// a call to Persist() while such a job is already pending may result in
// the creation of another persist job even though the old one has not
// run. This will result in calls to Persist() with a smaller time delay
// than the persistDelay passed when the database was created.
func (db *DB) Persist() (err error) {
  if db.persist == nil { return }
  
  db.mutex.Lock()
  defer db.mutex.Unlock()

  // allow persist jobs to be scheduled again
  db.blockPersistJobs = false
  
  return db.persistWithLock()
}

// Actually persists the database using db.persist.Store().
// REQUIRES HOLDING THE DB LOCK!
func (db *DB) persistWithLock() (err error) {
  return db.persist.Store(db.data)
}
