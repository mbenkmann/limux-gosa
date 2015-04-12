/*
Copyright (c) 2014 Landeshauptstadt München
Author: Matthias S. Benkmann

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.
*/

package main

import (
         "io"
         "io/ioutil"
         "os"
         "os/exec"
         "fmt"
         "net"
         "net/http"
         "net/url"
         "sort"
         "sync"
         "sync/atomic"
         "syscall"
         "time"
         "bufio"
         "regexp"
         "strings"
         "strconv"
         "compress/gzip"
         "compress/bzip2"
         "math/rand"
         "runtime"
         "runtime/debug"
         
         "../bytes"
         "../util"
      )

var USAGE = `debdb [-v|-vv|-vvv|-vvvv] kernels|packages <cachefile>
debdb [-v|-vv|-vvv|-vvvv] update --debconf=all|depends|cache <cachefile> <FAIrepository>...

`

// set by readargs()
type ModeT int
var Mode ModeT

const (
  GenerateKernelList ModeT = iota  // output available kernels from db
  GeneratePackageList             // output available packages from db
  UpdateDB                        // update the database
  Download                        // download a .deb package
)
                                
// If this is true, the cache will only have 1 listing per package version even if
// the same version exists for i386 and amd64.
// The exception to this are packages for which kernel_name() returns non-"". This
// means that the kernel packages in the output will never be merged.
// NOTE: The LDIF output always merges i386 and amd64 because there is no architecture
// information in the LDIF. The exception is GenerateKernelList output.
// This switch saves a lot of memory!
var MergeI386andAMD64 = true

// Every line in the cache and the PackageList objects starts with this many
// bytes that together form a "repository paths bitmap" (LITTLE-ENDIAN).
// If bit N is set, then
// the respective package is found in the repo path RP with Repopath2Index[RP]==N.
const RPBytes = 8

// Output informative messages.
var Verbose = 0

// maximum size of templates file to be considered
const TEMPLATES_MAX_SIZE = 1000000

// Regex for parsing lines in Packages file like this:
// 0c8a5062dee022b56afc2fca683f0748           959037 main/binary-amd64/Packages
var parse_release_file = regexp.MustCompile("^[0-9a-f]+\\s+[0-9]+ (([a-z]+)/binary-([a-z0-9]+))/Packages([.bzg2]*)")

// A repository path (i.e. the path relative to dists/ of a directory containing
// a Release file) is converted to a release id
// by removing all substrings that start with a "-" or a "/" followed by a letter
// optionally followed by more letters and/or digits.
// E.g. "trusty-backports" => "trusty"
//      "jessie/updates"   => "jessie"
//      "tramp/5.0.0rc1"   => "tramp/5.0.0rc1"
var Repopath2ReleaseRegexp = regexp.MustCompile("[/-][a-zA-Z][0-9a-zA-Z]*")

// In GenerateKernelList mode, the following list of regex/replacement pairs
// is applied in sequence to all package names with the result of the previous
// replacement being the input into the next one. If the replacement is "",
// the regular expression must match or the package name is not accepted as
// a kernel package name. If the replacement is "!" the regular expression
// must NOT match.
// The end result is considered the name of that particular kernel and all
// packages whose names map to the same name after massaging are considered
// to be just different versions of the same kernel.
var KernelVersionMassage = []string{ "^linux-image-",  "",
                                     "^linux-image-extra", "!",
                                     "-dbg$"        , "!",
                                     "-virtual$"    , "!",
                                     "-goldfish$"    , "!",
                                     "-\\d+\\.\\d+\\.\\d+-\\d+-", "",
                                     "-(\\d+\\.\\d+)\\.\\d+-\\d+", "-$1",
                                     "^linux-image-(.)", "$1",
}

// split out KernelVersionMassage into 2 arrays and parse regexps
var KernelVersionMassage_match = []*regexp.Regexp{}
var KernelVersionMassage_repl  = []string{}
func init() {
  for i := 0; i < len(KernelVersionMassage); i+=2 {
    KernelVersionMassage_match = append(KernelVersionMassage_match, regexp.MustCompile(KernelVersionMassage[i]))
    KernelVersionMassage_repl  = append(KernelVersionMassage_repl, KernelVersionMassage[i+1])
  }
}

// Which architectures to scan
var Architectures = map[string]bool{"all":true, "i386":true, "amd64":true}

var CachePath = "/tmp/generate_package_list.cache"
var CacheMetaPath = "/tmp/generate_package_list.meta"

//var FAIrepository = "http://de.archive.ubuntu.com/ubuntu/|ignored|trusty|main,restricted,universe,multiverse http://dk.archive.ubuntu.com/ubuntu/|ignored|trusty-updates|main,restricted,universe,multiverse http://nl.archive.ubuntu.com/ubuntu/|ignored|trusty|main,restricted,universe,multiverse"
//var FAIrepository = "http://de.archive.ubuntu.com/ubuntu/|ignored|trusty|main,restricted,universe,multiverse"
var FAIrepository = "http://de.archive.ubuntu.com/ubuntu/||trusty|main,restricted,universe,multiverse http://de.archive.ubuntu.com/ubuntu/||trusty-backports|main,restricted,universe,multiverse http://de.archive.ubuntu.com/ubuntu/||trusty-updates|main,restricted,universe,multiverse http://de.archive.ubuntu.com/ubuntu/||trusty-security|main,restricted,universe,multiverse http://ftp.debian.org/debian||jessie|main,contrib,non-free http://ftp.debian.org/debian||jessie-updates|main,contrib,non-free http://ftp.debian.org/debian||jessie-backports|main,contrib,non-free http://security.debian.org||jessie/updates|main,contrib,non-free"

// derived from FAIrepository this contains each base URL such as
// "http://de.archive.ubuntu.com/ubuntu".
var RepoBaseURLs []string

// "cache" => only use templates data from cache
// "depends" => scan .deb file if it depends on something that includes the string "debconf"
// everything else (including "") => scan all .deb files unless templates data is in cache
var Debconf = "all"

// http.Client(s) to use for connections in order of preference
// If a proxy is available, the first entry in this list will use it.
// The last entry is always a plain connection without proxy. 
var Client []*http.Client
// Transport[i] is the http.Transport of Client[i]
var Transport []*http.Transport

// Maps a release id to a list of all repository paths compatible with that release.
// The Release id is derived from Codename and Version using several heuristics.
var Release2Repopaths = map[string][]string{}

// Maps a repository path to the release id. Compare Release2Repopaths above.
var Repopath2Release = map[string]string{}

// Maps each repopath to a unique index >= 0. This map only contains mappings
// for repopaths encountered during this program run, not those from the cache.
var Repopath2Index = map[string]int{}

// Maps each repopath to a unique index >= 0. Contains all mappings from
// Repopath2Index plus any additional mappings for repopaths from the cache
// that are not part of the current program run.
var Repopath2IndexWithCache = map[string]int{}

// Maps repo+","+repopath to true for every repo/repopath combination whose packages
// are contained in the cache file to be read by readcache().
var HaveCache = map[string]bool{}

// Like HaveCache but for the cache file that will be written by writecache().
var WillHaveCache = map[string]bool{}

// List of URIs of Packages files (without the actual "/Packages[.gz|.bz2]" at the end)
var PackagesURIs = []string{}

// This is what we're going through all the trouble for.
var MasterPackageList *PackageList

type MergeSource interface {
  Bytes(int) []byte
  Sort()
  Count() int
  Get(i int) (rpbitmap uint64, path, section, description64, templates64 []byte)
  Clear()
}

type MMapMergeSource struct {
  cache *os.File
  mmap []byte
  data []byte
  LineOfs []int
}

func NewMMapMergeSource(cache *os.File, mmap []byte, size int) *MMapMergeSource {
  m := &MMapMergeSource{cache:cache, mmap:mmap, data:mmap[0:size]}
  
  // make sure the new last line is terminated by '\n'
  if m.data[len(m.data)-1] != '\n' {
    fmt.Fprintln(os.Stderr, "Cache is corrupt; does not end with newline")
    m.data = nil
    return m
  }

  nextOfs := 0
  for nextOfs < len(m.data) {
    m.LineOfs = append(m.LineOfs, nextOfs)
    nextOfs += RPBytes // do not look inside rpbitmap for \n
    for m.data[nextOfs] != '\n' { nextOfs++ }
    nextOfs += 1 // skip '\n'
  }
  
  return m
}

func (pkg *MMapMergeSource) Bytes(i int) []byte {
  return pkg.data[pkg.LineOfs[i]:]
}

func (pkg *MMapMergeSource) Get(i int) (rpbitmap uint64, path, section, description64, templates64 []byte) {
  if i >= len(pkg.LineOfs) {
    return 0,nil,nil,nil,nil
  }
  return get(pkg, i)
}

type MMapSorter MMapMergeSource

func (a *MMapSorter) Len() int { return len(a.LineOfs) }
func (a *MMapSorter) Swap(i,j int) { a.LineOfs[i], a.LineOfs[j] = a.LineOfs[j], a.LineOfs[i] }
func (a *MMapSorter) Less(i, j int) bool { 
  return compare(a.data[a.LineOfs[i]+RPBytes:], a.data[a.LineOfs[j]+RPBytes:]) < 0 
}

func (pkg *MMapMergeSource) Sort() {
  sort.Sort((*MMapSorter)(pkg))
}

func (pkg *MMapMergeSource) Count() int {
  return len(pkg.LineOfs)
}

func (pkg *MMapMergeSource) Clear() {
  if err := syscall.Munmap(pkg.mmap); err != nil {
    fmt.Fprintln(os.Stderr, err)
  }
  pkg.cache.Close()
  pkg.mmap = nil
  pkg.data = nil
  pkg.cache = nil
}

type PackageList struct {
  /*
  Contains lines of the form 
    <rpbitmap>|pool/main/e/empathy/account-plugin-aim_3.8.6-0ubuntu9.1_amd64.deb|gnome|<description-base64>|<templatesbase64>
  See RPBytes constant further above for explanation of <rpbitmap>
  If <templatesbase64> is "", there are not templates.
  If <templatesbase64> is "D", the package's dependencies contain "debconf" but
     no actual scan for templates has yet been performed.
  If <templatesbase64> is "?", the package's dependencies do not contain "debconf"
     and no scan for templates has yet been performed.
  */
  Data bytes.Buffer
  
  /*
  LineOfs[i] is the offset in Data.Bytes() of the i-th line.
  When the lines are sorted, only this array is reordered and not the actual Data.
  */
  LineOfs []int
  
  // Protects this object from concurrent access.
  sync.Mutex
}

type PackageListSorter PackageList

func (a *PackageListSorter) Len() int { return len(a.LineOfs) }
func (a *PackageListSorter) Swap(i,j int) { a.LineOfs[i], a.LineOfs[j] = a.LineOfs[j], a.LineOfs[i] }
func (a *PackageListSorter) Less(i, j int) bool { 
  return compare(a.Data.Bytes()[a.LineOfs[i]+RPBytes:], a.Data.Bytes()[a.LineOfs[j]+RPBytes:]) < 0
}

// Compares the byte slices sl1 and sl2 lexicographically up to the 3rd pipe, i.e.
// it does not compare the <description64> and <templates64> fields.
// It returns <0, ==0 and >0 depending on whether sl1 is smaller, equal or greater than
// sl2.
// The path field is compared starting with the package name, so that entries
// with the same package name are sorted next to each other even if they have
// differing paths in the pool.
// If MergeI386andAMD64, then "_i386.deb" and "_amd64.deb" are considered equal
// unless kernel_name(path) returns non-""
func compare(sl1, sl2 []byte) int {
  countpipe := 0
  x := 0
  y := 0
  for x < len(sl1) && y < len(sl2) {
    if sl1[x] < sl2[y] { 
      if MergeI386andAMD64 && x>0 && sl1[x-1] == '_' && sl1[x] == 'a' && sl2[y] == 'i' && sl1[x+1] == 'm' && sl2[y+1] == '3' && sl1[x+2] == 'd' && sl2[y+2] == '8' && sl1[x+3] == '6' && sl2[y+3] == '6' && sl1[x+4] == '4' {
        slash := x
        for slash > 0 && sl1[slash] != '/' { slash-- }
        if kernel_name(sl1[slash:x+5]) != "" {
          return -1
        }
        x += 5
        y += 4
        continue
      } else {
        return -1
      }
    }
    if sl2[y] < sl1[x] { 
      if MergeI386andAMD64 && y>0 && sl2[y-1] == '_' && sl2[y] == 'a' && sl1[x] == 'i' && sl2[y+1] == 'm' && sl1[x+1] == '3' && sl2[y+2] == 'd' && sl1[x+2] == '8' && sl2[y+3] == '6' && sl1[x+3] == '6' && sl2[y+4] == '4' {
        slash := y
        for slash > 0 && sl2[slash] != '/' { slash-- }
        if kernel_name(sl2[slash:y+5]) != "" {
          return +1
        }
        y += 5
        x += 4
        continue
      } else {
        return +1
      }
    }
    if sl1[x] == '|' {
      countpipe++
      
      // After the 1st pipe follows the path field. We want to compare the
      // package names first, so we seek forward to the next pipe and then
      // backward to the 1st slash. Then we compare until we reach the 2nd pipe.
      if countpipe == 1 {
        for x+1 < len(sl1) && sl1[x+1] != '|' { x++ }
        for y+1 < len(sl2) && sl2[y+1] != '|' { y++ }
        for x>0 && sl1[x] != '/' { x-- }
        for y>0 && sl2[y] != '/' { y-- }
        continue
      }
      
      // If we reach the 2nd pipe, the package names are equal. Now
      // we seek back to the 1st pipe to compare the pool paths.
      // If these are equal, too, the 2nd pipe will be reached again,
      // but counted as the 3rd pipe!
      if countpipe == 2 {
        for x > 1 && sl1[x-1] != '|' { x-- }
        for y > 1 && sl2[y-1] != '|' { y-- }
        continue
      }
      
      // stop comparing when we reach the description part because
      // descriptions may differ for different architectures even for the
      // same package version. I guess we could condition the test on
      // MergeI386andAMD64 and use countpipe == 5 if !MergeI386andAMD64, but...
      if countpipe == 4 /* actually means the 3rd pipe! See countpipe==2 case comment */ {
        return 0 
      }
    }
    x++
    y++
  }
  panic("buffer not formatted properly")
}

// Returns a raw view of the list's data starting at the first byte of entry.
// Be careful to lock the object if necessary!
func (pkg *PackageList) Bytes(entry int) []byte {
  return pkg.Data.Bytes()[pkg.LineOfs[entry]:]
}

// Sorts the lines.
func (pkg *PackageList) Sort() {
  pkg.Mutex.Lock()
  defer pkg.Mutex.Unlock()
  sort.Sort((*PackageListSorter)(pkg))
}

/*
  Appends to the PackageList from the data read from r which has to be
  in raw PackageList formaŧ. If an error occurs, the data is unchanged.
*/
func (pkg *PackageList) AppendRaw(r io.Reader) error {
  pkg.Mutex.Lock()
  defer pkg.Mutex.Unlock()
  nextOfs := pkg.Data.Len()
  _, err := pkg.Data.ReadFrom(r)
  if err != nil {
    pkg.Data.Trim(0, nextOfs)
    return err
  }
  
  if pkg.Data.Len() == nextOfs { return nil } // nothing new was read
  
  // make sure the new last line is terminated by '\n'
  if pkg.Data.Bytes()[pkg.Data.Len()-1] != '\n' {
    pkg.Data.WriteByte('\n')
  }

  data := pkg.Data.Bytes()
  for nextOfs < len(data) {
    pkg.LineOfs = append(pkg.LineOfs, nextOfs)
    nextOfs += RPBytes // do not look inside rpbitmap for \n
    for data[nextOfs] != '\n' { nextOfs++ }
    nextOfs++ // skip '\n'
  }

  return nil
}

// Returns true iff the byte slice b only contains bytes <= ' ' (space)
func isempty(b []byte) bool {
  for i := range b {
    if b[i] > ' ' { return false }
  }
  return true
}

// Contains true iff the byte slice b contains the string s somewhere.
func has(b []byte, s string) bool {
  if len(s) == 0 { return true }
  for i := 0; i <= len(b)-len(s); i++ {
    k := 0
    for ; k < len(s); k++ {
      if b[i+k] != s[k] { break }
    }
    if k == len(s) { return true }
  }
  return false
}

// If the byte slice b starts with the string id (seen as a sequence of bytes)
// this function returns the subslice of b that starts with the first byte that
// follows id. If b does not have id as a prefix, returns nil.
// This function is used to extract the field values from a debian (P)ackages (L)ist,
// e.g. it's called with id=="Section: "
func plextract(b []byte, id string) []byte {
  if len(b) < len(id) { return nil }
  for i := range id {
    if b[i] != id[i] { return nil }
  }
  return b[len(id):]
}

/**
  Appends to the PackageList from the data read from r which has to be
  in the format of a Debian repository's "Packages" file.
  rpbitmap is the bitmap of release paths (see RPBytes constant further above)
  If an error occurs, some package data may still be appended.
*/
func (pkg *PackageList) AppendPackages(rpbitmap uint64, r io.Reader) error {
  // DO NOT LOCK MUTEX BECAUSE WE CALL Append() WHICH LOCKS!
 
  var path []byte
  var section []byte
  var description64 []byte
  templates_unknown := []byte{'?'}
  templates_debconf := []byte{'D'}
  var templates64 []byte = templates_unknown
  utils := []byte("utils")
  nodesc := util.Base64EncodeString("description missing")
  
  d64 := make([]byte, 4096) // buffer for base64 encoding description
  buffy := make([]byte, 65536)
  buffy_end := 0
  buffy_start := 0
  
  for {
    if buffy_end == len(buffy) {
      return fmt.Errorf("Line too long in Packages file: %s [...]", buffy[0:256])
    }
    
    n, err := r.Read(buffy[buffy_end:])
    if err != nil && err != io.EOF {
      return err
    }
    if err == io.EOF && n == 0 { break }
    
    eol := buffy_end
    buffy_end += n
    
    for {
      for eol < buffy_end && buffy[eol] != '\n' { eol++ }
      
      if eol == buffy_end { break }
      
      line := buffy[buffy_start:eol]
      
      if isempty(line) {
        if len(section) == 0 { section = utils }
        if len(description64) == 0 { description64 = nodesc }
        slash := 0
        for slash < len(path) && path[slash] != '/' { slash++ }
        if slash < len(path)-1 { // a slash as last character is no good => -1
          pkg.Append(rpbitmap, path, section, description64, templates64)
        } else {
          return fmt.Errorf("Malformed Filename in Packages file: %s", path)
        }
        path = nil
        section = nil
        description64 = nil
        templates64 = templates_unknown
        buffy_end = copy(buffy, buffy[eol+1:buffy_end])
        buffy_start = 0
        eol = 0
      } else if s := plextract(line, "Section: "); s != nil {
        section = s
        eol++
        buffy_start = eol
      } else if p := plextract(line, "Filename: "); p != nil {
        path = p
        eol++
        buffy_start = eol
      } else if d := plextract(line, "Description: "); d != nil {
        idx := (((len(d)+2)/3)<<2)-len(d)
        if idx + len(d) > len(d64) {
          if len(d) > 256 { d = d[0:256] }
          return fmt.Errorf("Description too long in Packages file: %s [...]", d)
        }
        copy(d64[idx:], d)
        description64 = util.Base64EncodeInPlace(d64[:idx+len(d)], idx)
        eol++
        buffy_start = eol
      } else {
        // also catches Pre-Depends!
        if has(line, "Depends:") && has(line,"debconf") { 
          templates64 = templates_debconf 
        }
        buffy_end = buffy_start + copy(buffy[buffy_start:], buffy[eol+1:buffy_end])
        eol = buffy_start
      }
    }
  }

  return nil
}

/**
  Merges p1 and p2 and appends the result to this PackageList.
  p1 and p2 will be sorted first.
  Lines that are identical except for the rpbitmap and templates are combined
  according to the following rules:
    * rpbitmaps are ORed
    * base64 data overrides everything
    * from 2 different base64 strings one is picked
    * "" overrides "D" and "?"
    * "D" overrides "?"
  If p2templatesonly is true, then p2 can not contribute new lines, only
  amend existing lines from p1 with templates data.
*/
func (pkg *PackageList) AppendMerge(p1, p2 MergeSource, p2templatesonly bool) {
  if p1 == pkg || p2 == pkg || p1 == p2 { panic("all 3 lists involved in AppendMerge must be distinct") }
  p1.Sort()
  p2.Sort()
  
  a := 0
  b := 0
  
  // merge until the end of one list is reached
  for a < p1.Count() && b < p2.Count() {
    comp := compare(p1.Bytes(a)[RPBytes:], p2.Bytes(b)[RPBytes:])
    if comp < 0 {
      if Verbose > 3 { fmt.Fprintf(os.Stderr, "< ") }
      rpbitmap, path, section, description64, templates64 := p1.Get(a)
      pkg.Append(rpbitmap, path, section, description64, templates64)
      a++
    } else if comp > 0 {
      if !p2templatesonly {
        if Verbose > 3 { fmt.Fprintf(os.Stderr, "> ") }
        rpbitmap, path, section, description64, templates64 := p2.Get(b)
        pkg.Append(rpbitmap, path, section, description64, templates64)
      }
      b++
    } else {
      rpbitmap, path, section, description64, templates64 := p1.Get(a)
      rpbitmap_2, path_2, _, _, templates64_2 := p2.Get(b)
      // if path ends in "i386.deb" we use path_2. This means we prefer
      // "amd64.deb" when both are present.
      if MergeI386andAMD64 && len(path) > 8 && has(path[len(path)-8:],"i386") {
        path = path_2
      }
      if len(templates64) == 1 && templates64[0] == '?' {
        // everything overrides ?
        templates64 = templates64_2
      }
      if len(templates64) == 1 && len(templates64_2) == 0 {
        // "" overrides ? and D
        templates64 = templates64_2
      }
      if len(templates64) < 2 && len(templates64_2) > 2 {
        // base64 string overrides ?, D and ""
        templates64 = templates64_2
      }
      if Verbose > 3 { fmt.Fprintf(os.Stderr, "= ") }
      pkg.Append(rpbitmap|rpbitmap_2, path, section, description64, templates64)
      a++
      b++
    }
  }
  
  // copy any remaining entries from p1
  for ; a < p1.Count(); a++ {
    if Verbose > 3 { fmt.Fprintf(os.Stderr, "<<") }
    rpbitmap, path, section, description64, templates64 := p1.Get(a)
    pkg.Append(rpbitmap, path, section, description64, templates64)
  }
  
  // copy any remaining entries from p2
  if !p2templatesonly {
    for ; b < p2.Count(); b++ {
      if Verbose > 3 { fmt.Fprintf(os.Stderr, ">>") }
      rpbitmap, path, section, description64, templates64 := p2.Get(b)
      pkg.Append(rpbitmap, path, section, description64, templates64)
    }
  }
}

/**
  Appends a single line to this PackageList.
  description64 and templates64 are base64-encoded.
  templates64 may also be "", "D" or "?".
*/
func (pkg *PackageList) Append(rpbitmap uint64, path, section, description64, templates64 []byte) {
  pkg.Mutex.Lock()
  defer pkg.Mutex.Unlock()
  rpbytes := make([]byte, RPBytes)
  rpb := rpbitmap
  for i := range rpbytes {
    rpbytes[i] = byte(rpb & 255)
    rpb >>= 8
  }
  sz := len(rpbytes)+1+len(path)+1+len(section)+1+len(description64)+1+len(templates64)+1
  pkg.Data.Grow(sz)
  ofs := pkg.Data.Len()
  ofs2 := ofs
  pkg.LineOfs = append(pkg.LineOfs, ofs)
  pkg.Data.Write(rpbytes)
  pkg.Data.WriteByte('|')
  pkg.Data.Write(path)
  pkg.Data.WriteByte('|')
  pkg.Data.Write(section)
  pkg.Data.WriteByte('|')
  pkg.Data.Write(description64)
  pkg.Data.WriteByte('|')
  pkg.Data.Write(templates64)
  pkg.Data.WriteByte('\n')
  if Verbose > 3 {
    fmt.Fprintf(os.Stderr, "%x%s", rpbitmap, pkg.Data.Bytes()[ofs2+len(rpbytes):pkg.Data.Len()])
  }
}

// Returns the elements of entry i. 
func (pkg *PackageList) Get(i int) (rpbitmap uint64, path, section, description64, templates64 []byte) {
  pkg.Mutex.Lock()
  defer pkg.Mutex.Unlock()
  if i >= len(pkg.LineOfs) {
    return 0,nil,nil,nil,nil
  }
  return get(pkg, i)
}

func get(pkg MergeSource, i int) (rpbitmap uint64, path, section, description64, templates64 []byte) {
  data := pkg.Bytes(i)
  if len(data) < RPBytes+1 || data[RPBytes] != '|' {
    fmt.Fprintf(os.Stderr, "Malformed MergeSource\n")
    return 0,nil,nil,nil,nil
  }
  rpbytes := data[0:RPBytes]
  for i := range rpbytes {
    rpbitmap |= uint64(rpbytes[i]) << (uint(i)*8)
  }
  var ofs int
  nxt := RPBytes+1 // past the first '|'
  for ofs = nxt; data[nxt] != '|'; nxt++ {}
  path = data[ofs:nxt]
  nxt++ // skip '|'
  for ofs = nxt; data[nxt] != '|'; nxt++ {}
  section = data[ofs:nxt]
  nxt++ // skip '|'
  for ofs = nxt; data[nxt] != '|'; nxt++ {}
  description64 = data[ofs:nxt]
  nxt++ // skip '|'
  for ofs = nxt; data[nxt] != '\n'; nxt++ {}
  templates64 = data[ofs:nxt]
  return
}

/** 
  WriteTo writes data to w until there's no more data to write or when an
  error occurs. The return value n is the number of bytes written.
  Any error encountered during the write is also returned.
*/
func (pkg *PackageList) WriteTo(w io.Writer) (n int, err error) {
  pkg.Mutex.Lock()
  defer pkg.Mutex.Unlock()
  // write in correct order of lines
  n = 0
  data := pkg.Data.Bytes()
  for i := range pkg.LineOfs {
    start := pkg.LineOfs[i]
    end := start + RPBytes // +RPBytes so that we don't look inside rpbitmap for \n
    for data[end] != '\n' { end++ }
    end++ // include \n in output
    n2, err := util.WriteAll(w, data[start:end])
    n += n2
    if err != nil { return n, err }
  }
  return n, nil
}

/**
  Removes all contents from the PackageList and frees associated memory.
*/
func (pkg *PackageList) Clear() {
  pkg.Mutex.Lock()
  defer pkg.Mutex.Unlock()
  pkg.Data.Reset()
  pkg.LineOfs = nil
}

// Returns the number of entries in the list.
func (pkg *PackageList) Count() int {
  pkg.Mutex.Lock()
  defer pkg.Mutex.Unlock()
  return len(pkg.LineOfs)
}

func main() {
  rand.Seed(316888245464693718)
  readargs()
  initclient()
  readmeta()
  switch (Mode) {
    case GenerateKernelList:  generate_kernel_list() 
    case GeneratePackageList: generate_package_list()
    case UpdateDB:            updatedb()
    //case Download:            download()
  }
}

// Takes the pool path of a package and applies KernelVersionMassage to it.
// If the package refers to a linux kernel, this function returns the massaged name
// of that kernel. Otherwise "" is returned.
// It is permitted to pass just the package name as path.
func kernel_name(path []byte) string {
  slash := len(path)-1
  for slash > 0 && path[slash] != '/' { slash-- }
  under := slash
  for under < len(path) && path[under] != '_' { under++ }
  name := string(path[slash+1:under])
  
  for k := range KernelVersionMassage_match {
    if KernelVersionMassage_repl[k] == "" {
      if !KernelVersionMassage_match[k].MatchString(name) {
        return ""
      }
    } else if KernelVersionMassage_repl[k] == "!" {
      if KernelVersionMassage_match[k].MatchString(name) {
        return ""
      }
    } else {
      name = KernelVersionMassage_match[k].ReplaceAllString(name, KernelVersionMassage_repl[k])
    }
  }
  return name
}

func generate_kernel_list() {
  readcache(false) // false => read packages, too, not just templates data
  release2name2bool := map[string]map[string]bool{}
  name2version := map[string]string{}
  name2path := map[string]string{}
  for i := MasterPackageList.Count()-1; i >= 0; i-- {
    rpbitmap, path, _, _, _ := MasterPackageList.Get(i)

    name := kernel_name(path)
    
    if name != "" {
      // find last slash in path
      slash := len(path)-1
      for slash > 0 && path[slash] != '/' { slash-- }
      // find 1st underscore after last slash
      under := slash
      for under < len(path) && path[under] != '_' { under++ }
      // find 2nd underscore
      under2 := under+1
      for under2 < len(path) && path[under2] != '_' { under2++ }
      version := string(path[under+1:under2])
      // find "."
      dot := under2+1
      for dot < len(path) && path[dot] != '.' { dot++ }
      arch := string(path[under2+1:dot])
      // prefix name with architecture
      name = arch + "/" + name
      
      for rp, idx := range Repopath2IndexWithCache {
        if rpbitmap & (uint64(1) << uint(idx)) != 0 {
          release := Repopath2ReleaseRegexp.ReplaceAllString(rp,"")
          if release2name2bool[release] == nil {
            release2name2bool[release] = map[string]bool{}
          }
          release2name2bool[release][name] = true
          if debian_greater(version, name2version[name]) {
            name2version[name] = version
            name2path[name] = string(path)
          }
        }
      }
    }
  }

  for release, name2bool := range release2name2bool {
    fmt.Printf("release: %v\ncn: default\n\n", release)
    for name, _ := range name2bool {
      fmt.Printf("release: %v\ncn: %v\nversion: %v\nfile: %v\n\n", release, name, name2version[name], name2path[name])
    }
  }
}

func updatedb() {
  noerrors := process_releases_files()
  noerrors = noerrors && process_packages_files()
  // If noerrors, we only use the template data from cache.
  // Otherwise we also use the cache to provide missing packages.
  readcache(noerrors)
  
  debconf_scan()
  writemeta()
  writecache()
}

func generate_package_list() {
  readcache(false) // false => read packages, too, not just templates data
  printldif()
}

func readargs() {
  words := []string{}
  for _, arg := range os.Args[1:] {
    if strings.HasPrefix(arg, "-") {
      if strings.HasPrefix(arg, "--debconf=cache") {
        Debconf = "cache"
      } else if strings.HasPrefix(arg, "--debconf=depend") {
        Debconf = "depends"
      } else if strings.HasPrefix(arg, "-v") {
        Verbose += len(arg) - 1
      } else if strings.HasPrefix(arg, "--help") {
        fmt.Fprintf(os.Stdout, USAGE)
        os.Exit(0)
      } else {
        fmt.Fprintf(os.Stderr, "Unknown command line switch: %v\n", arg)
        os.Exit(1)
      }
    } else {
      words = append(words, arg)
    }
  }

  if len(words) < 2 {
    fmt.Fprintf(os.Stdout, USAGE)
    os.Exit(0)
  }
  
  if words[0] == "update" { 
    Mode = UpdateDB
    if len(words) < 3 {
      fmt.Fprintf(os.Stderr, "Missing FAIrepository argument\n")
      os.Exit(1)
    }
    FAIrepository = words[2]
    for _, w := range words[3:] { FAIrepository = FAIrepository + " " + w }
  } else if words[0] == "kernels" { 
    Mode = GenerateKernelList
  } else if words[0] == "packages" { 
    Mode = GeneratePackageList
  } else {
    fmt.Fprintf(os.Stderr, "Unknown command: %v\n", words[0])
    os.Exit(1)
  }

  CachePath = words[1]
  dot := strings.Index(CachePath, ".")
  if dot < 0 { dot = len(CachePath) }
  CacheMetaPath = CachePath[0:dot] + ".meta"
}

func initclient() {
  tr := &http.Transport{
    Dial: (&net.Dialer{
        Timeout:   5 * time.Second,
        KeepAlive: 30 * time.Second,
    }).Dial,
    TLSHandshakeTimeout: 10 * time.Second,
    //DisableKeepAlives: true,
    MaxIdleConnsPerHost: 8,
    // proxy function examines Request r and decides if
    // a proxy should be used. If the returned error is non-nil,
    // the request is aborted. If the returned URL is nil,
    // no proxy is used. Otherwise URL is the URL of the
    // proxy to use.
    // NOTE NOTE NOTE
    // net/http contains helpers to get a proxy function
    // easily: See net/http/ProxyFromEnvironment()
    Proxy: func(r *http.Request) (*url.URL, error) {
      return nil, nil
    },
  }
  
  Transport = append(Transport, tr)
  
  // the same Client object can (and for efficiency reasons should)
  // be used in all goroutines according to net/http docs.
  Client = append(Client, &http.Client{Transport: tr, Timeout: 2*time.Minute})
}

// Used by process_releases_files to manage the todo-list of Releases files to process.
type ReleaseTodo struct {
  //First component of FAIRepository entry with trailing "/" trimmed away,
  // e.g. "http://de.archive.ubuntu.com/ubuntu"
  Repo string
  // Second component of FAIRepository entry with trailing "/" trimmed away,
  // e.g. "jessie/updates"
  Repopath string
  // Third component of FAIRepository entry translated into a map, e.g.
  // {"main":true, "restricted":true, "universe":true, "multiverse":true}
  Components map[string]bool
  // The trimmed lines of the actual Release file.
  ReleaseFile []string
}

/**
  Processes the Releases files for all repositories listed in FAIRepository.
  Returns true if no error occurred and false if some error occurred. In the
  latter case the cache file should be used to fill in data ŧhat may be missing.
*/
func process_releases_files() (ok bool) {
  ok = true
  
  ///////// Parse FAIrepository and compose a TODO list reporepopath2release_todo ///////
  repobases := map[string]bool{}
  reporepopath2release_todo := map[string]*ReleaseTodo{}
  for _, fairepo := range strings.Fields(FAIrepository) {
    parts := strings.Split(fairepo, "|")
    repo := strings.TrimRight(strings.TrimSpace(parts[0]),"/")
    repobases[repo] = true
    repopath := strings.TrimRight(strings.TrimSpace(parts[2]),"/")
    components := map[string]bool{}
    for _, com := range strings.Fields(strings.TrimSpace(strings.Replace(parts[3],","," ",-1))) {
      components[com] = true
    }

    reporepopath2release_todo[repo+","+repopath] = &ReleaseTodo{Repo:repo, Repopath:repopath, Components:components}
  }
  
  /* 
    Collect repository base URLs. We'll later use them during debconf scan.
    For each .deb we want to download to be scanned we'll shuffle this list
    and try to append the pool-path of the .deb to these base URLs until we
    find a URL that works. This distributes the load among the known mirrors.
  */
  for rb := range repobases {
    RepoBaseURLs = append(RepoBaseURLs, rb)
  }
  
  if Verbose > 0 {
    fmt.Fprintf(os.Stderr, "Repositories to scan: %v\n", RepoBaseURLs)
  }
  
  /*
    Now we start one goroutine for each entry in our TODO list and
    have it send the downloaded Release file to a channel.
  */
  
  c := make(chan []string, len(reporepopath2release_todo))
  
  for rs, rt := range reporepopath2release_todo {
    rs2 := rs
    uri := rt.Repo+"/dists/"+rt.Repopath+"/Release"
    go read_lines_from_uri(rs2, uri, c)
  }
  
  /*
    The following select block collects the Release file data sent
    over the channel. When all goroutines have reported back it
    stops. To prevent a hang a timeout is used.
  */
  
  count := len(reporepopath2release_todo)
  if count == 0 { return true } // nothing to do
  tim := time.NewTimer(time.Duration(count)*5*time.Second)
  loop:
  for {
    select {
      case release_lines := <- c:  
                       reporepopath := release_lines[0]
                       if len(release_lines) == 1 {
                         fmt.Fprintf(os.Stderr, "Error reading %v/dists/%v/Release", reporepopath2release_todo[reporepopath].Repo, reporepopath2release_todo[reporepopath].Repopath)
                         if HaveCache[reporepopath] {
                           fmt.Fprintf(os.Stderr, " => Some data will be filled in from cache!")
                           // We only set ok=false if have_cache[...]. Otherwise a
                           // repository entry in LDAP that doesn't work anymore
                           // would permanently prevent old package data from
                           // being purged fromt the cache.
                           // Another way to put it: If it hasn't worked last time
                           // then it's ok if it doesn't work this time.
                           ok = false
                         }
                         fmt.Fprintln(os.Stderr)
                       } else {
                         WillHaveCache[reporepopath] = true
                       }
                       reporepopath2release_todo[reporepopath].ReleaseFile = release_lines[1:]
                       if count--; count == 0 { 
                         tim.Stop()
                         break loop 
                       }
                       
      case _ = <- tim.C:
                       fmt.Fprintln(os.Stderr, "Timeout while reading Release files => Some data will be filled in from cache!")
                       ok = false
                       break loop
    }
  }
  
  // Release files contain multiple lines for the same Packages file, so
  // we use this map for duplicate elimination.
  have_uri := map[string]bool{}
  
  /*
    Now we evaluate all of the collected Release files and extract the
    URLs of the Packages files referenced therein.
    At the same time we build up several maps between
    repository path (that's the path of the directory containing the
    Packages file relative to dists/) and other things.
  */
  
  for _, todo := range reporepopath2release_todo {
    if len(todo.ReleaseFile) == 1 && todo.ReleaseFile[0] == "empty" {
      continue
    }
    
    codename := ""
    release := Repopath2ReleaseRegexp.ReplaceAllString(todo.Repopath,"")
    
    for _, line := range todo.ReleaseFile {
      if strings.HasPrefix(line, "Codename: ") {
        codename = line[10:]
      } else {
        match := parse_release_file.FindStringSubmatch(line)
        if match != nil {
          if codename == "" {
            fmt.Fprintf(os.Stderr, "SKIPPED %v/dists/%v/Release because it contains no Codename\n", todo.Repo, todo.Repopath)
            break
          }
          if todo.Components[match[2]] && Architectures[match[3]] {
            uri := todo.Repo+"/dists/"+todo.Repopath+"/"+match[1]
            if !have_uri[uri] {
              PackagesURIs = append(PackagesURIs, uri)
              have_uri[uri] = true
            }
          }
        }
      }
    }
    
    if codename == "" { continue }
    
    Release2Repopaths[release] = append(Release2Repopaths[release], todo.Repopath)
    Repopath2Release[todo.Repopath] = release
    if index, have := Repopath2IndexWithCache[todo.Repopath]; have {
      // use same index as in cache if possible
      Repopath2Index[todo.Repopath] = index
    } else {
      // otherwise find an index that's not used (including not in the cache!!!)
      indexes := map[int]bool{}
      for _, i := range Repopath2IndexWithCache { indexes[i] = true }
      index := 0
      for indexes[index] { index++ }
      Repopath2Index[todo.Repopath] = index
      Repopath2IndexWithCache[todo.Repopath] = index
    }
  }
  
  if Verbose > 1 {
    fmt.Fprintf(os.Stderr, "repo path -> release id %v\n", Repopath2Release)
    for r, i := range Repopath2IndexWithCache {
      star := " "
      if Repopath2Index[r] == i { star = "*" }
      fmt.Fprintf(os.Stderr, "%v%v -> index %v\n", star, r, i)
    }
  }
  
  return ok
}

// Used to store downloaded Packages files in COMPRESSED form (i.e. as they
// are on the server).
type TaggedBlob struct {
  Payload bytes.Buffer
  Compression string // "", "gz" or "bz2"
  Repopath string
}

// Returns false if an error occurred (and consequently cache data
// should be used to fill in missing packages).
// Sets MasterPackageList UNLESS there is nothing to do.
func process_packages_files() (ok bool) {
  if Verbose > 1 {
    fmt.Fprintf(os.Stderr, "Will read Packages files from the following paths:\n%v\n", strings.Join(PackagesURIs, "\n"))
  }

/*
  This function follows the same principle as process_release_files.
  We start a goroutine for each Packages file to download it and send it
  over a channel. We collect the downloaded data from the channel and then
  process it sequentially.
*/

  
  ok = true
  count := len(PackagesURIs)
  if count == 0 { return true } // nothing to do
  
  c := make(chan *TaggedBlob, count)
  
  for _, uri := range PackagesURIs {
    pkguri := uri+"/Packages" // PackageURIs does not include the "/Packages"
    // fetch_uri tries to fetch the pkguri with .gz, .bz2 and no extension
    // before giving up.
    go fetch_uri(pkguri, c)
  }
  
  var taggedblobs []*TaggedBlob
  
  tim := time.NewTimer(time.Duration(count)*5*time.Second)
  loop:
  for {
    select {
      case taggedblob := <- c:
                       if taggedblob == nil {
                         fmt.Fprintln(os.Stderr, "Error while reading Packages files => Some data will be filled in from cache!")
                         ok = false
                       } else {
                         taggedblobs = append(taggedblobs, taggedblob)
                       }
                       if count--; count == 0 {
                         tim.Stop()
                         break loop
                       }
                       
      case _ = <- tim.C:
                       fmt.Fprintln(os.Stderr, "Timeout while reading Packages files => Some data will be filled in from cache!")
                       ok = false
                       break loop
    }
  }
  
  pkgList := &PackageList{}
  
  for _, taggedblob := range taggedblobs {
    if taggedblob.Payload.Len() < 100 { continue } // empty, except for compression overhead
    var r io.Reader = &taggedblob.Payload
    var err error
  
    if taggedblob.Compression == "bz2" {
      r = bzip2.NewReader(r)
    } else if taggedblob.Compression == "gz" {
      r, err = gzip.NewReader(r)
      if err != nil {
        fmt.Fprintf(os.Stderr, "%v\n", err)
        taggedblob.Payload.Reset()
        ok = false
        continue
      }
    }

    tempPkgList := &PackageList{}
    rpindex, haverp := Repopath2Index[taggedblob.Repopath]
    if !haverp {
      err = fmt.Errorf("internal error: could not convert repopath %v to index", taggedblob.Repopath)
    } else {
      if Verbose > 1 {
        fmt.Fprintf(os.Stderr, "Parsing Packages file for repo path %v\n", taggedblob.Repopath)
      }
      err = tempPkgList.AppendPackages(uint64(1) << uint(rpindex), r)
      if err == nil && Verbose > 1 {
        fmt.Fprintf(os.Stderr, "Resulting list has %v lines (%v bytes)\n", tempPkgList.Count(), tempPkgList.Data.Len())
      }
    }
    taggedblob.Payload.Reset()
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v\n", err)
      tempPkgList.Clear()
      ok = false
      continue
    }
    
    pkgListNew := &PackageList{}
    if Verbose > 1 {
      fmt.Fprintf(os.Stderr, "Merging %v lines (%v bytes) and %v lines (%v bytes) from %v \n", pkgList.Count(), pkgList.Data.Len(), tempPkgList.Count(), tempPkgList.Data.Len(), taggedblob.Repopath)
    }
    pkgListNew.AppendMerge(pkgList, tempPkgList, false)
    pkgList.Clear()
    tempPkgList.Clear()
    pkgList = pkgListNew
    if Verbose > 1 {
      fmt.Fprintf(os.Stderr, "Resulting list has %v lines (%v bytes)\n", pkgList.Count(), pkgList.Data.Len())
    }
  }
  
  MasterPackageList = pkgList
  return ok
}


func readmeta() {
  metapath := CacheMetaPath
  meta, err := os.Open(metapath)
  if err != nil{
    if os.IsNotExist(err.(*os.PathError).Err) {
      // Make sure we don't read a cache without the metadata
      os.Remove(CachePath)
    } else {
      fmt.Fprintln(os.Stderr, err)
    }
    return
  }
  defer meta.Close()
  
  metadata, err := ioutil.ReadAll(meta)
  for _, line := range strings.Split(string(metadata),"\n") {
    line = strings.TrimSpace(line)
    if line == "" { continue }
    f := strings.Fields(line)
    if len(f) == 1 {
      HaveCache[line] = true
    } else {
      i, err := strconv.Atoi(f[0])
      if err != nil {
        fmt.Fprintf(os.Stderr, "%v is corrupt: %v", metapath, err)
        os.Exit(1) // this is fatal because we can't continue with crap data
      }
      Repopath2IndexWithCache[f[1]] = i
    }
  }
  
  for repopath := range Repopath2IndexWithCache {
    release := Repopath2ReleaseRegexp.ReplaceAllString(repopath,"")
    Release2Repopaths[release] = append(Release2Repopaths[release], repopath)
    Repopath2Release[repopath] = release
  }
}



func readcache(templatesonly bool) {
  if MasterPackageList == nil {
    MasterPackageList = &PackageList{}
  }

  if !templatesonly {
    for reporepopath := range HaveCache {
      WillHaveCache[reporepopath] = true
    }
    Repopath2Index = Repopath2IndexWithCache
  }
  
  cachepath := CachePath
  cache, err := os.Open(cachepath)
  if err != nil{
    if !os.IsNotExist(err.(*os.PathError).Err) {
      fmt.Fprintln(os.Stderr, err)
    }
    return
  }
  
  
  var pkg MergeSource
  
  fi, err := os.Stat(cachepath)
  if err == nil {
    sz := int(fi.Size()) + os.Getpagesize()-1
    sz -= sz % os.Getpagesize()
    fd := cache.Fd()
    var mmap []byte
    if sz == 0 { // empty files can't be mmapped. And we don't need to anyway.
      cache.Close()
      return
    }
    mmap, err = syscall.Mmap(int(fd), 0, sz, syscall.PROT_READ, syscall.MAP_PRIVATE)
    if err == nil {
      pkg = NewMMapMergeSource(cache, mmap, int(fi.Size()))
    }
  }

  if err != nil {
    defer cache.Close()
    fmt.Fprintf(os.Stderr, "Could not mmap %v (%v) ==> Falling back to normal read\n", cachepath, err)
    pkgfile := &PackageList{}
    err = pkgfile.AppendRaw(cache)
    if err != nil {
      fmt.Fprintln(os.Stderr, err)
      pkgfile.Clear()
    }
    pkg = pkgfile
  }
  
  newPkgList := &PackageList{}
  
  if Verbose > 1 {
    if templatesonly {
      fmt.Fprintf(os.Stderr, "Merging %v lines (%v bytes) with TEMPLATE DATA ONLY from cache %v lines (%v bytes)\n", MasterPackageList.Count(), MasterPackageList.Data.Len(), pkg.Count(), len(pkg.Bytes(0)))
    } else {
      fmt.Fprintf(os.Stderr, "Merging %v lines (%v bytes) with ALL DATA from cache %v lines (%v bytes)\n", MasterPackageList.Count(), MasterPackageList.Data.Len(), pkg.Count(), len(pkg.Bytes(0)))
    }
  }
  newPkgList.AppendMerge(MasterPackageList, pkg, templatesonly)
  MasterPackageList.Clear()
  pkg.Clear()
  MasterPackageList = newPkgList
  if Verbose > 1 {
    fmt.Fprintf(os.Stderr, "Resulting list has %v lines (%v bytes)\n", MasterPackageList.Count(), MasterPackageList.Data.Len())
  }
  
}

type LineData struct {
  rpbitmap uint64
  path []byte
  section []byte
  description64 []byte
  templates64 []byte
}

var DebconfScanned int32
var DebconfExtracted int32
func debconf_scan() {
  if Debconf == "cache" { 
    if Verbose > 0 {
      fmt.Fprintf(os.Stderr, "Skipping debconf-scan because PackageListDebconf mode is \"cache\"\n")
    }
    return 
  }
  
  if Verbose > 0 {
    if Debconf == "depends" {
      fmt.Fprintf(os.Stderr, "Scanning packages with debconf-dependency for templates\n")
    } else {
      fmt.Fprintf(os.Stderr, "Scanning ALL packages for templates\n")
    }
  }

  pkg := MasterPackageList
  defer pkg.Clear()
  MasterPackageList = &PackageList{}

  deadline := time.Now().Add(60*time.Minute)

  num_scanners := 16
  c := make(chan *LineData, num_scanners)
  for i := 0; i < num_scanners; i++ {
    go debconf_scan_worker(c, deadline)
  }

  for i := 0; i < pkg.Count(); i++ {
    rpbitmap, path, section, description64, templates64 := pkg.Get(i)
    if len(templates64) == 1 && (templates64[0] == 'D' || Debconf != "depends") {
      c <- &LineData{rpbitmap, path, section, description64, templates64}
    } else {
      MasterPackageList.Append(rpbitmap, path, section, description64, templates64)
    }
  }

  have_printed_message := false
  for MasterPackageList.Count() < pkg.Count() {
    time.Sleep(5*time.Second)
    // When workers notice that the deadline has passed, they will switch to
    // a mode where they simply push through data without scanning.
    if !have_printed_message && time.Now().After(deadline) {
      fmt.Fprintln(os.Stderr, "Deadline reached. Workers will switch into fast-forward mode!")
      have_printed_message = true
    }
    // We wait a couple minutes after the deadline to let all workers time out
    // on their pending connections and go into fast-forward mode before
    // we consider the program to be in a broken state.
    if time.Now().Sub(deadline) > 5*time.Minute {
      fmt.Fprintln(os.Stderr, "Deadline exceeded. Probably some workers have crashed!")
      os.Exit(1)
    }
  }
  
  if Verbose > 0 {
    fmt.Fprintf(os.Stderr, "#packages: %v, #scanned: %v, #templates extracted: %v\n", MasterPackageList.Count(), DebconfScanned, DebconfExtracted)
  }
}

func shuffle(a []string) {
  for i := range a {
    j := rand.Intn(i + 1)
    a[i], a[j] = a[j], a[i]
  }
}

func debconf_scan_worker(c chan *LineData, deadline time.Time) {
  // Make my own copy of RepoBaseURLs for shuffling
  repobases := make([]string, len(RepoBaseURLs))
  copy(repobases, RepoBaseURLs)
  
  for {
    task := <- c
    rpbitmap, path, section, description64, templates64 := task.rpbitmap, task.path, task.section, task.description64, task.templates64
    // If we are past the deadline, we don't scan and simply forward the data as is.
    if time.Now().Before(deadline) {
      shuffle(repobases)
      ok := false
      for _, b := range repobases {
        if temp64 := extract_templates(b+"/"+string(path)); temp64 != nil {
          templates64 = temp64
          ok = true
          break
        }
      }
      if !ok {
        fmt.Fprintf(os.Stderr, "SCANFAIL %s\n", path)
      }
    }
    MasterPackageList.Append(rpbitmap, path, section, description64, templates64)
  }
}

func extract_templates(uri string) []byte {
  
  // Workaround for a condition I encountered during testing where
  // the number of goroutines would shoot up and sockets would not
  // be closed until the program crashed with too many open files.
  if runtime.NumGoroutine() > 300 {
    if Verbose > 0 {
      fmt.Fprintln(os.Stderr, "Waiting for goroutines to finish...")
    }
    Transport[0].CloseIdleConnections()
    debug.FreeOSMemory()
    for runtime.NumGoroutine() > 200 {
      time.Sleep(5*time.Second)
    }
  }
  
  resp, err := Client[0].Get(uri)
  if err != nil {
    return nil
  }
  defer resp.Body.Close()

  if resp.StatusCode != 200 {
    return nil
  }

  cmd := exec.Command("dpkg", "--info","/dev/stdin","templates")
  cmd.Stdin = resp.Body
  var outbuf bytes.Buffer
  cmd.Stdout = &outbuf
  defer outbuf.Reset()
  var errbuf bytes.Buffer
  cmd.Stderr = &errbuf
  defer errbuf.Reset()
  err = cmd.Run()
  var templates64 []byte
  if err != nil && 
    // broken pipe is normal because dpkg stops reading once it has
    // the data it needs
    !strings.Contains(err.Error(), "broken pipe") &&
    // exit status 2 just means that the deb package has no templates
    !strings.Contains(err.Error(), "exit status 2") {
     fmt.Fprintf(os.Stderr, "dpkg --info %v: %v (%v)\n", uri, err, errbuf.String())
  } else {
    atomic.AddInt32(&DebconfScanned, 1)
    if outbuf.Len() > TEMPLATES_MAX_SIZE {
      fmt.Fprintf(os.Stderr, "TOO LARGE %v\n", uri)
      templates64 = []byte{} // pretend that templates are empty to prevent rescan
    } else {
      if outbuf.Len() == 0 {
        templates64 = []byte{}
      } else {
        atomic.AddInt32(&DebconfExtracted, 1)
        templates64 = make([]byte, ((outbuf.Len()+2)/3)<<2)
        idx := len(templates64) - outbuf.Len()
        copy(templates64[idx:], outbuf.Bytes())
        templates64 = util.Base64EncodeInPlace(templates64, idx)
        if Verbose > 2 {
          fmt.Fprintf(os.Stderr, "DEBCONF %v\n", uri)
        }
      }
    }
  }

  return templates64
}

func writemeta() {
  meta, err := os.Create(CacheMetaPath)
  if err != nil{
    fmt.Fprintln(os.Stderr, err)
    return
  }
  defer meta.Close()
  
  for line := range WillHaveCache {
    fmt.Fprintln(meta, line)
  }
  
  for r,i := range Repopath2Index {
    fmt.Fprintf(meta, "%d %s\n", i, r)
  }
}

func writecache() {
  cache, err := os.Create(CachePath)
  if err != nil {
    fmt.Fprintln(os.Stderr, err)
    return
  }
  defer cache.Close()
  
  MasterPackageList.Sort() // to make diff'ing easier for debug purposes
  _, err = MasterPackageList.WriteTo(cache)
  if err != nil {
    fmt.Fprintln(os.Stderr, err)
  }
}

type LDIFObject []string

func (l *LDIFObject) Append(key string, value []byte) {
  val := string(value)
  for i := len(*l)-2; i >= 0; i-=2 {
    if (*l)[i] == key {
      if (*l)[i+1] == val {
        copy((*l)[i:], (*l)[i+2:])
        *l = (*l)[0:len(*l)-2]
      }
      break
    }
  }
  *l = append(*l, key, val)
}

func (l *LDIFObject) ToString() string {
  sl := make([]string,0,len(*l)*2)
  for i := range (*l) {
    sl = append(sl, (*l)[i])
    if i & 1 == 0 {
      sl = append(sl, ": ")
    } else {
      sl = append(sl, "\n")
    }
  }
  return strings.Join(sl,"")
}

func printldif() {
  for release, repopaths := range Release2Repopaths {
    for _, repopath := range repopaths {
      fmt.Printf(`
Release: %v
Repository: %v
`,  release, repopath)
    }
  }
  
  index2repopath := map[int]string{}
  for r, i := range Repopath2Index { index2repopath[i] = r }
  
  prevpkg := ""
  release_printed := map[string]bool{}
  
  outobj := make(LDIFObject,0)
  
  for i := 0; i < MasterPackageList.Count(); i++ {
    rpbitmap, path, section, description64, templates64 := MasterPackageList.Get(i)
    
    pkg := []byte{}
    for last_slash := len(path)-1; last_slash >= 0; last_slash-- {
      if path[last_slash] == '/' { 
        pkg = path[last_slash+1:]
        break 
      }
    }
    // pkg is something like "account-plugin-aim_3.8.6-0ubuntu9.1_amd64.deb"
    version := []byte{}
    for first_underscore := 0; first_underscore < len(pkg); first_underscore++ {
      if pkg[first_underscore] == '_' {
        version = pkg[first_underscore+1:]
        pkg = pkg[:first_underscore]
        break
      }
    }
    // pkg is now "account-plugin-aim" and version "3.8.6-0ubuntu9.1_amd64.deb"
    for second_underscore := 0; second_underscore < len(version); second_underscore++ {
      if version[second_underscore] == '_' {
        version = version[:second_underscore]
        break
      }
    }
    // pkg and version are now properly package name and version
    
    // Lines are sorted, so entries with the same package name but different
    // versions or sections should be subsequent. If we encounter a line that
    // has the same package as the previous line, instead of starting a new
    // object in the LDIF, append additional attributes to the previous object
    pkgstr := fmt.Sprintf("%s",pkg)
    if pkgstr != prevpkg { // start a new object
      fmt.Printf("\n%s", outobj.ToString()) // flush current object
      outobj = make(LDIFObject,0)
      outobj.Append("Package", pkg)
      release_printed = map[string]bool{}
    }
    prevpkg = pkgstr
    
    for i := 0; i < RPBytes*8; i++ {
      if (rpbitmap & (1<<uint(i))) != 0 {
        release := Repopath2Release[index2repopath[i]]
        if release_printed[release] { continue }
        outobj.Append("Release", []byte(release))
        release_printed[release] = true
      }
    }
    
    outobj.Append("Version", version)
    outobj.Append("Section", section)
    outobj.Append("Description:"/*yes, the ':' is correct*/, description64)
    if len(templates64) > 1 {
      outobj.Append("Templates:"/*':' here, too*/,  templates64)
    }
  }
  
  fmt.Printf("\n%s", outobj.ToString()) // flush current object
}

// Reads a text file from uri, splits it into lines, trims them and
// writes the resulting []string slice to channel c, with line1 inserted
// before the first line from the uri.
// If an error occurs, only []string{line1} is written to c.
// If the file is empty, []string{line1,"empty"} is written to c.
func read_lines_from_uri(line1 string, uri string, c chan []string) {
  var err error
  var resp *http.Response
  lines := []string{line1}
  defer func(){ c<-lines }()
  
  tries := 2
  for {
    resp, err = Client[0].Get(uri)
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v: %v\n", uri, err)
      return
    }
    
    defer resp.Body.Close()
   
    if resp.StatusCode != 200 {
      fmt.Fprintf(os.Stderr, "%v: %v\n", uri, resp.Status)
      return
    }
    
    input := bufio.NewReader(resp.Body)
    for {
      var line string
      line, err = input.ReadString('\n')
      if err != nil { 
        if err == io.EOF { goto done }
        if tries--; tries == 0 {
          fmt.Fprintf(os.Stderr, "%v: %v\n", uri, err)
          return
        }
        lines = lines[0:0]
        resp.Body.Close()
        break
      }
      lines = append(lines, strings.TrimSpace(line))
    }
  }

done:  
  if Verbose > 0 {
    if len(lines) == 1 {
      fmt.Fprintf(os.Stderr, "EMPTY %v\n", uri)
    } else {
      fmt.Fprintf(os.Stderr, "OK %v\n", uri)
    }
  }
  
  if len(lines) == 1 { 
    lines = append(lines, "empty")
  }
  
}  

/*
We want the fastest speed and low memory usage. 
There are 2 factors that have to be considered:
- download time
- decompression time
- compressed size
Obviously bz2 gives better download time and lower memory usage (because the
downloaded file is stored in memory). However Go's bzip2
decompressor is very slow. Therefore ".gz" is probably the best compromise.
We put "" at the end, even though uncompressed files would seem to offer the 
2nd best compromise. However current Debian repositories do not offer uncompressed
Packages files anymore and Ubuntu at least seems to have started redirecting
requests to "Packages" to "Packages.bz2".

If an error occurs, nil is sent to the channel.
*/
var extensions_to_try = []string{".gz", /*retry=>*/ ".gz", ".bz2", ".bz2", "", ""} 
func fetch_uri(uri string, c chan *TaggedBlob) {
  var err error
  var resp *http.Response
  
  errors := []string{}
  var tb *TaggedBlob = nil
  defer func() { c <- tb }()
  
  for ext_i, extension := range extensions_to_try {
    resp, err = Client[0].Get(uri+extension)
    
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v: %v\n", uri+extension, err)
      
    } else {
      if resp.StatusCode != 200 {
        // HTTP level errors like 404 are only reported if we don't manage
        // to get anything. So at this point we just store the error.
        errors = append(errors, fmt.Sprintf("%v: %v\n", uri+extension, resp.Status))
        resp.Body.Close()
        
      } else {
        if extension == "" {
           if contenttype,ok := resp.Header["Content-Type"]; ok {
             for _, ct := range contenttype {
               if strings.Contains(ct, "gzip") { 
                 extension = ".gz" 
               } else if strings.Contains(ct, "bzip2") { 
                 extension = ".bz2" 
               }
             }
          }
        }
        
        rpstart := strings.LastIndex(uri, "/dists/")+7
        rpend := strings.LastIndex(uri, "/binary")
        if rpend > 0 { rpend-- }
        for rpend > 0 && uri[rpend] != '/' { rpend-- }
        repopath := ""
        if rpstart < rpend {
          repopath = uri[rpstart:rpend]
        }
        tb = &TaggedBlob{Repopath:repopath}
        if extension != "" { tb.Compression = extension[1:] }
        _, err = tb.Payload.ReadFrom(resp.Body)
        resp.Body.Close()
        
        if err == nil {
          if Verbose > 0 {
            if tb.Payload.Len() < 100 {
              fmt.Fprintf(os.Stderr, "EMPTY %v\n", uri+extension)
            } else {
              fmt.Fprintf(os.Stderr, "OK %v\n", uri+extension)
            }
          }
          return
          
        } else {
          tb.Payload.Reset()
          tb = nil
          // We only print out the error if we reached the final retry of the
          // current extension to try
          if ext_i == len(extensions_to_try)-1 || extensions_to_try[ext_i] != extensions_to_try[ext_i+1] {
            fmt.Fprintf(os.Stderr, "%v: %v\n", uri+extension, err)
          }
        }
      }
    }
  }
  
  for _, e := range errors {
    fmt.Fprintln(os.Stderr, e)
  }
}  

// Returns true if smaller == "" or dpkg --compare-versions $greater gt $smaller.
func debian_greater(greater, smaller string) bool {
  if smaller == "" { return true }
  return nil == exec.Command("dpkg", "--compare-versions", greater, "gt", smaller).Run()
}
