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


package xml

import (
         "sync"
       )

// a database based on an xml.Hash, optionally backed by a file on disk.
type DB struct {
  // the data
  data *Hash
  // if not "", the data stored in the database is persisted in this file.
  persistPath string
  // to avoid writing the database on every write access, a delay in ms
  // can be set here. This is the maximum time between a write access to
  // the database and the attempt to persist the data on disk.
  persistDelay int
  // for locking the database.
  mutex sync.RWMutex
  // true if the job for persisting the database has been scheduled.
  persistJobScheduled bool
}

// Creates a new database.
//  name: must be a valid tag name. It will be the outer-most tag in the XML string
//        representation of the database
// 
//  persistPath: If "" the database will exist in memory only. Otherwise this
//               is the path to a file whose contents will be used to
//               initialize the database and to which database changes
//               will be written back. (The writing happens via write-rename to
//               make sure the data is never lost)
//  persistDelay: The maximum time in ms that may pass between writing to
//                the in-memory db and the job to persist the database. The job
//                may be delayed further due to concurrent database accesses.
//                Increasing this improves performance if there are many
//                write accesses, but increases the chances of data loss.
//  Returns:
//   An error is only possible if a persistPath is provided and something
//   went wrong reading the file. If that happens persisting will be disabled and
//   the database will start out empty.
func NewDB(name string, persistPath string, persistDelay int) (*DB, error) {
  db := &DB{data:NewHash(name), persistPath:persistPath, persistDelay:persistDelay, persistJobScheduled:false}
  return db, nil
}
