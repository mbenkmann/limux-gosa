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
         "os"
         "os/exec"
         "fmt"
         "net/http"
         "net/url"
         "sort"
         "sync"
         "sync/atomic"
         "time"
         "bufio"
         "regexp"
         "strings"
         "compress/gzip"
         "compress/bzip2"
         "path/filepath"
         "math/rand"
         "runtime"
         "runtime/debug"
         
         "../bytes"
         "../util"
      )

// Output informative messages.
var Verbose = 0

// maximum size of templates file to be considered
const TEMPLATES_MAX_SIZE = 1000000

// Regex for parsing lines in Packages file like this:
// 0c8a5062dee022b56afc2fca683f0748           959037 main/binary-amd64/Packages
var parse_release_file = regexp.MustCompile("^[0-9a-f]+\\s+[0-9]+ (([a-z]+)/binary-([a-z0-9]+))/Packages([.bzg2]*)")

// Which architectures to scan
var Architectures = map[string]bool{"all":true, "i386":true, "amd64":true}

var CacheName = "generate_package_list.cache"
var CacheDir = "/tmp"

//var FAIrepository = "http://de.archive.ubuntu.com/ubuntu/|ignored|trusty|main,restricted,universe,multiverse http://dk.archive.ubuntu.com/ubuntu/|ignored|trusty-updates|main,restricted,universe,multiverse http://nl.archive.ubuntu.com/ubuntu/|ignored|trusty|main,restricted,universe,multiverse"
//var FAIrepository = "http://de.archive.ubuntu.com/ubuntu/|ignored|trusty|main,restricted,universe,multiverse"
var FAIrepository = "http://de.archive.ubuntu.com/ubuntu/||trusty|main,restricted,universe,multiverse http://de.archive.ubuntu.com/ubuntu/||trusty-backports|main,restricted,universe,multiverse http://de.archive.ubuntu.com/ubuntu/||trusty-updates|main,restricted,universe,multiverse http://de.archive.ubuntu.com/ubuntu/||trusty-security|main,restricted,universe,multiverse http://ftp.debian.org/debian||jessie|main,contrib,non-free http://ftp.debian.org/debian||jessie-updates|main,contrib,non-free http://ftp.debian.org/debian||jessie-backports|main,contrib,non-free http://security.debian.org||jessie/updates|main,contrib,non-free"

// derived from FAIrepository this contains each base URL such as
// "http://de.archive.ubuntu.com/ubuntu".
var RepoBaseURLs []string

// "cache" => only use templates data from cache
// "depends" => scan .deb file if it depends on something that includes the string "debconf"
// everything else (including "") => scan all .deb files unless templates data is in cache
var Debconf = "depends"

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

// List of URIs of Packages files (without the actual "/Packages[.gz|.bz2]" at the end)
var PackagesURIs = []string{}

// This is what we're going through all the trouble for.
var MasterPackageList *PackageList

type PackageList struct {
  /*
  Contains lines of the form 
    trusty|pool/main/e/empathy/account-plugin-aim_3.8.6-0ubuntu9.1_amd64.deb|gnome|<description-base64>|<templatesbase64>
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
  return compare(a.Data.Bytes()[a.LineOfs[i]:], a.Data.Bytes()[a.LineOfs[j]:]) < 0
}

func compare(sl1, sl2 []byte) int {
  countslash := 0
  for x := 0; x < len(sl1) && x < len(sl2); x++ {
    if sl1[x] < sl2[x] { return -1 }
    if sl2[x] < sl1[x] { return +1 }
    if sl1[x] == '|' {
      countslash++
      if countslash == 4 { // stop comparing when we reach template part
        return 0 
      }
    } else if sl1[x] == '\n' { break }
  }
  panic("buffer not formatted properly")
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
    for data[nextOfs] != '\n' { nextOfs++ }
    nextOfs++ // skip '\n'
  }

  return nil
}

func isempty(b []byte) bool {
  for i := range b {
    if b[i] > ' ' { return false }
  }
  return true
}

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
  release is the name of the release (e.g. "trusty")
  If aDn error occurs, some package data may still be appended.
*/
func (pkg *PackageList) AppendPackages(release []byte, r io.Reader) error {
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
        if section == nil { section = utils }
        if description64 == nil { description64 = nodesc }
        if path != nil {
          pkg.Append(release, path, section, description64, templates64)
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
  Lines that are identical except for the templates component are combined
  according to the following rules:
    * base64 data overrides everything
    * from 2 different base64 strings one is picked
    * "" overrides "D" and "?"
    * "D" overrides "?"
  If p2templatesonly is true, then p2 can not contribute new lines, only
  amend existing lines from p1 with templates data.
*/
func (pkg *PackageList) AppendMerge(p1, p2 *PackageList, p2templatesonly bool) {
  if p1 == pkg || p2 == pkg || p1 == p2 { panic("all 3 lists involved in AppendMerge must be distinct") }
  p1.Sort()
  p2.Sort()
  
  a := 0
  b := 0
  
  // merge until the end of one list is reached
  for a < p1.Count() && b < p2.Count() {
    comp := compare(p1.Data.Bytes()[p1.LineOfs[a]:], p2.Data.Bytes()[p2.LineOfs[b]:])
    if comp < 0 {
      if Verbose > 3 { fmt.Fprintf(os.Stderr, "< ") }
      release, path, section, description64, templates64 := p1.Get(a)
      pkg.Append(release, path, section, description64, templates64)
      a++
    } else if comp > 0 {
      if !p2templatesonly {
        if Verbose > 3 { fmt.Fprintf(os.Stderr, "> ") }
        release, path, section, description64, templates64 := p2.Get(b)
        pkg.Append(release, path, section, description64, templates64)
      }
      b++
    } else {
      release, path, section, description64, templates64 := p1.Get(a)
      _, _, _, _, templates64_2 := p2.Get(b)
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
      pkg.Append(release, path, section, description64, templates64)
      a++
      b++
    }
  }
  
  // copy any remaining entries from p1
  for ; a < p1.Count(); a++ {
    if Verbose > 3 { fmt.Fprintf(os.Stderr, "<<") }
    release, path, section, description64, templates64 := p1.Get(a)
    pkg.Append(release, path, section, description64, templates64)
  }
  
  // copy any remaining entries from p2
  if !p2templatesonly {
    for ; b < p2.Count(); b++ {
      if Verbose > 3 { fmt.Fprintf(os.Stderr, ">>") }
      release, path, section, description64, templates64 := p2.Get(b)
      pkg.Append(release, path, section, description64, templates64)
    }
  }
}

/**
  Appends a single line to this PackageList.
  description64 and templates64 are base64-encoded.
  templates64 may also be "", "D" or "?".
*/
func (pkg *PackageList) Append(release, path, section, description64, templates64 []byte) {
  pkg.Mutex.Lock()
  defer pkg.Mutex.Unlock()
  sz := len(release)+1+len(path)+1+len(section)+1+len(description64)+1+len(templates64)+1
  pkg.Data.Grow(sz)
  ofs := pkg.Data.Len()
  ofs2 := ofs
  pkg.LineOfs = append(pkg.LineOfs, ofs)
  pkg.Data.Write(release)
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
    fmt.Fprintf(os.Stderr, "%s", pkg.Data.Bytes()[ofs2:pkg.Data.Len()])
  }
}

// Returns the elements of entry i. 
func (pkg *PackageList) Get(i int) (release, path, section, description64, templates64 []byte) {
  pkg.Mutex.Lock()
  defer pkg.Mutex.Unlock()
  if i >= len(pkg.LineOfs) {
    return nil,nil,nil,nil,nil
  }
  
  data := pkg.Data.Bytes()
  nxt := pkg.LineOfs[i]
  var ofs int
  for ofs = nxt; data[nxt] != '|'; nxt++ {}
  release = data[ofs:nxt]
  nxt++ // skip '|'
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
    end := start
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
  readenv()
  initclient()
  noerrors := process_releases_files()
  noerrors = noerrors && process_packages_files()
  // If noerrors, we only use the template data from cache.
  // Otherwise we also use the cache to provide missing packages.
  readcache(noerrors)
  debconf_scan()
  writecache()
  printldif()
}

func readenv() {
  if cd := os.Getenv("PackageListCacheDir"); cd != "" {
    CacheDir = cd
  }
  if dc := os.Getenv("PackageListDebconf"); dc != "" {
    Debconf = dc
  }
  if fr := os.Getenv("PackageListFAIrepository"); fr != "" {
    FAIrepository = fr
  }
  ve := os.Getenv("Verbose")
  if ve == "1" { Verbose = 1 }
  if ve == "2" { Verbose = 2 }
  if ve == "3" { Verbose = 3 }
  if ve == "4" { Verbose = 4 }
}

func initclient() {
  tr := &http.Transport{
    //DisableKeepAlives: true,
    MaxIdleConnsPerHost: 8,
    // proxy function examines Request r and decides if
    // a proxy should be used. If the returned error is non-nil,
    // the request is aborted. If the returned URL is nil,
    // no proxy is used. Otherwise URL is the URL of the
    // proxy to use.
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
  
  for rb := range repobases {
    RepoBaseURLs = append(RepoBaseURLs, rb)
  }
  
  if Verbose > 0 {
    fmt.Fprintf(os.Stderr, "Repositories to scan: %v\n", RepoBaseURLs)
  }
  
  c := make(chan []string, len(reporepopath2release_todo))
  
  for rs, rt := range reporepopath2release_todo {
    rs2 := rs
    uri := rt.Repo+"/dists/"+rt.Repopath+"/Release"
    go read_lines_from_uri(rs2, uri, c)
  }
  
  count := len(reporepopath2release_todo)
  if count == 0 { return true } // nothing to do
  tim := time.NewTimer(time.Duration(count)*5*time.Second)
  loop:
  for {
    select {
      case release_lines := <- c:  
                       reporepopath := release_lines[0]
                       if len(release_lines) == 1 {
                         fmt.Fprintf(os.Stderr, "Error reading %v/dists/%v/Release => Some data will be filled in from cache!\n", reporepopath2release_todo[reporepopath].Repo, reporepopath2release_todo[reporepopath].Repopath)
                         ok = false
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
  
  have_uri := map[string]bool{}
  
  for reporepopath, todo := range reporepopath2release_todo {
    versioncode := ""
    debian_special_case := strings.Contains(reporepopath, "debian")
    codename := ""
    version := ""
    for _, line := range todo.ReleaseFile {
      if strings.HasPrefix(line, "Codename: ") {
        codename = line[10:]
        if debian_special_case {
          // trim well known suffixes so that codename reflects
          // the main distribution the repo is compatible with
          codename = strings.TrimSuffix(codename, "-backports")
          codename = strings.TrimSuffix(codename, "-updates")
          codename = strings.TrimSuffix(codename, "-proposed-updates")
        }
      } else if strings.HasPrefix(line, "Version: ") {
        version = line[9:]
      } else {
        match := parse_release_file.FindStringSubmatch(line)
        if match != nil {
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
    
    versioncode = codename+"/"+version
    var release string
    // If the repo path does not end in the release version, then
    // we assume the release name should not include the version.
    // E.g. "trusty/14.04" becomes "trusty" because the repo paths
    // for trusty packages are "trusty", "trusty-backports",... which
    // do not include version numbers.
    // For LiMux this turns "tramp/5.0" into "tramp" but keeps
    // "tramp/5.0.0beta7" as is.
    if version != "" && strings.HasSuffix(todo.Repopath, version) {
      release = versioncode
    } else {
      release = codename
    }
    Release2Repopaths[release] = append(Release2Repopaths[release], todo.Repopath)
    Repopath2Release[todo.Repopath] = release
  }
  
  if Verbose > 1 {
    fmt.Fprintf(os.Stderr, "repo path -> release id %v\n", Repopath2Release)
  }
  
  return ok
}

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
    release := Repopath2Release[taggedblob.Repopath]
    if release == "" {
      err = fmt.Errorf("internal error: could not convert repopath %v into release id", taggedblob.Repopath)
    } else {
      if Verbose > 1 {
        fmt.Fprintf(os.Stderr, "Parsing Packages file\n")
      }
      err = tempPkgList.AppendPackages([]byte(release), r)
      if Verbose > 1 {
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


func readcache(templatesonly bool) {
  if MasterPackageList == nil {
    MasterPackageList = &PackageList{}
  }
  
  pkg := &PackageList{}
  cache, err := os.Open(filepath.Join(CacheDir,CacheName))
  if err != nil{
    if !os.IsNotExist(err.(*os.PathError).Err) {
      fmt.Fprintln(os.Stderr, err)
    }
    return
  }
  defer cache.Close()
  
  err = pkg.AppendRaw(cache)
  if err != nil {
    fmt.Fprintln(os.Stderr, err)
    pkg.Clear()
  }
  
  newPkgList := &PackageList{}
  
  if Verbose > 1 {
    if templatesonly {
      fmt.Fprintf(os.Stderr, "Merging %v lines (%v bytes) with TEMPLATE DATA ONLY from cache %v lines (%v bytes)\n", MasterPackageList.Count(), MasterPackageList.Data.Len(), pkg.Count(), pkg.Data.Len())
    } else {
      fmt.Fprintf(os.Stderr, "Merging %v lines (%v bytes) with ALL DATA from cache %v lines (%v bytes)\n", MasterPackageList.Count(), MasterPackageList.Data.Len(), pkg.Count(), pkg.Data.Len())
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
  c := make(chan [5][]byte, num_scanners)
  for i := 0; i < num_scanners; i++ {
    go debconf_scan_worker(c, deadline)
  }

  for i := 0; i < pkg.Count(); i++ {
    release, path, section, description64, templates64 := pkg.Get(i)
    if len(templates64) == 1 && (templates64[0] == 'D' || Debconf != "depends") {
      c <- [5][]byte{release, path, section, description64, templates64}
    } else {
      MasterPackageList.Append(release, path, section, description64, templates64)
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

func debconf_scan_worker(c chan [5][]byte, deadline time.Time) {
  // Make my own copy of RepoBaseURLs for shuffling
  repobases := make([]string, len(RepoBaseURLs))
  copy(repobases, RepoBaseURLs)
  
  for {
    task := <- c
    release, path, section, description64, templates64 := task[0], task[1], task[2], task[3], task[4]
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
        fmt.Fprintf(os.Stderr, "SCANFAIL %s %s\n", release, path)
      }
    }
    MasterPackageList.Append(release, path, section, description64, templates64)
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

func writecache() {
  cache, err := os.Create(filepath.Join(CacheDir,CacheName))
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

func printldif() {
  for release, repopaths := range Release2Repopaths {
    for _, repopath := range repopaths {
      fmt.Printf(`
Release: %v
Repository: %v
`,  release, repopath)
    }
  }
  
  prevpkg := ""
  
  for i := 0; i < MasterPackageList.Count(); i++ {
    release, path, section, description64, templates64 := MasterPackageList.Get(i)
    
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
    
    // because we have one entry per architecture the same package
    // may be included multiple times. They should however be sorted
    // consecutively so that simply comparing with the previous package
    // should be enough to eliminate duplicates
    pkgstr := fmt.Sprintf("%s%s%s",pkg,version,release)
    if pkgstr == prevpkg { continue }
    prevpkg = pkgstr
    
    fmt.Printf(`
Release: %s
Package: %s
Version: %s
Section: %s
Description:: %s
`,  release, pkg, version, section, description64)
    if len(templates64) > 1 {
      fmt.Printf("Templates:: %s\n", templates64)
    }
  }
}

// Reads a text file from uri, splits it into lines, trims them and
// writes the resulting []string slice to channel c, with line1 inserted
// before the first line from the uri.
// If an error occurs, only []string{line1} is written to c.
func read_lines_from_uri(line1 string, uri string, c chan []string) {
  var err error
  var resp *http.Response
  lines := []string{line1}
  defer func(){ c<-lines }()
  
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
      if err == io.EOF { break }
      fmt.Fprintf(os.Stderr, "%v: %v\n", uri, err)
      return
    }
    lines = append(lines, strings.TrimSpace(line))
  }
  
  if Verbose > 0 {
    fmt.Fprintf(os.Stderr, "OK %v\n", uri)
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
var extensions_to_try = []string{".gz", ".bz2", ""}
func fetch_uri(uri string, c chan *TaggedBlob) {
  var err error
  var resp *http.Response
  
  errors := []string{}
  var tb *TaggedBlob = nil
  defer func() { c <- tb }()
  
  for _, extension := range extensions_to_try {
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
          fmt.Fprintf(os.Stderr, "%v: %v\n", uri+extension, err)
        }
      }
    }
  }
  
  for _, e := range errors {
    fmt.Fprintln(os.Stderr, e)
  }
}  
