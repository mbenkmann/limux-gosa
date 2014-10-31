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
         "time"
         "bufio"
         "regexp"
         "strings"
         "compress/gzip"
         "compress/bzip2"
         "encoding/base64"
         "path/filepath"
         "math/rand"
         "bytes"
         "runtime"
      )

// maximum size of templates file to be considered
const TEMPLATES_MAX_SIZE = 1000000

// Regex for parsing lines in Packages file like this:
// 0c8a5062dee022b56afc2fca683f0748           959037 main/binary-amd64/Packages
var parse_release_file = regexp.MustCompile("^[0-9a-f]+\\s+[0-9]+ (([a-z]+)/binary-([a-z0-9]+))/Packages([.bzg2]*)")

// Which architectures to scan
var Architectures = map[string]bool{"all":true, "i386":true, "amd64":true}

// "trusty/14.04" -> ["http://de.archive.ubuntu.com/ubuntu,trusty", "http://de.archive.ubuntu.com/ubuntu,trusty-updates"]
var Versioncode2RepoCommaRepopathSet = map[string]map[string]bool {}
var Versioncode2RepoCommaRepopathSet_new = map[string]map[string]bool {}

// "http://de.archive.ubuntu.com/ubuntu,trusty-updates" -> ["main,amd64,.bz2", "main,i386,.bz2",... ]
var RepoRepopath2CompoArchExtSet = map[string]map[string]bool {}
var RepoRepopath2CompoArchExtSet_new = map[string]map[string]bool {}

// "http://de.archive.ubuntu.com/ubuntu,trusty-updates,main,i386,.bz2" -> ["bash|4.2+dfsg-0.1+deb7u3" , ...]
var RepoRepopathCompoArchExt2PackagePipeVersionSet = map[string]map[string]bool {}
var RepoRepopathCompoArchExt2PackagePipeVersionSet_new = map[string]map[string]bool {}

// "http://de.archive.ubuntu.com/ubuntu,trusty-updates" -> "trusty"
var RepoRepopath2Versioncode = map[string]string{}

// Maps PackagePipeVersion to list of URLs where .deb can be found
// for debs that need to be scanned for templates files.
var DebsToScan = map[string][]string{}
  

type PData struct {
  Meta []string // List of versioncode+","+repo+","+repopath+","+compo+","+arch+","+ext strings
  TemplatesBase64 string
  Section string
  DescriptionBase64 string
}

var PackagePipeVersion2PData = map[string]*PData {}
var PackagePipeVersion2PData_new = map[string]*PData {}

var CacheName = "generate_package_list.cache"
var CacheDir = "/tmp"

//var FAIrepository = "http://de.archive.ubuntu.com/ubuntu/|ignored|trusty|main,restricted,universe,multiverse http://dk.archive.ubuntu.com/ubuntu/|ignored|trusty-updates|main,restricted,universe,multiverse http://nl.archive.ubuntu.com/ubuntu/|ignored|trusty|main,restricted,universe,multiverse"
//var FAIrepository = "http://de.archive.ubuntu.com/ubuntu/|ignored|trusty|main,restricted,universe,multiverse"
var FAIrepository = "http://de.archive.ubuntu.com/ubuntu/||trusty|main,restricted,universe,multiverse http://de.archive.ubuntu.com/ubuntu/||trusty-backports|main,restricted,universe,multiverse http://de.archive.ubuntu.com/ubuntu/||trusty-updates|main,restricted,universe,multiverse http://de.archive.ubuntu.com/ubuntu/||trusty-security|main,restricted,universe,multiverse http://ftp.debian.org/debian||jessie|main,contrib,non-free http://ftp.debian.org/debian||jessie-updates|main,contrib,non-free http://ftp.debian.org/debian||jessie-backports|main,contrib,non-free http://security.debian.org||jessie/updates|main,contrib,non-free"

// "cache" => only use templates data from cache
// "depends" => scan .deb file if it depends on something that includes the string "debconf"
// everything else (including "") => scan all .deb files unless templates data is in cache
var Debconf = "depends"

type ReleaseTodo struct {
  Repo string
  Repopath string
  Components map[string]bool
  ReleaseFile []string
}

type TaggedBlob struct {
  Id string
  Ext string
  Payload bytes.Buffer
}

// http.Client(s) to use for connections in order of preference
// If a proxy is available, the first entry in this list will use it.
// The last entry is always a plain connection without proxy. 
var Client []*http.Client
// Transport[i] is the http.Transport of Client[i]
var Transport []*http.Transport

// Output informative messages.
var Verbose = true

func main() {
  rand.Seed(316888245464693718)
  readenv()
  initclient()
  readcache()
  process_releases_files()
  process_packages_files()
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
  Client = append(Client, &http.Client{Transport: tr})
}

func readcache() {
  cache, err := os.Open(filepath.Join(CacheDir,CacheName))
  if err != nil{
    if os.IsNotExist(err.(*os.PathError).Err) { return }
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
  defer cache.Close()
  input := bufio.NewReader(cache)
  doing_meta := false
  var meta []string
  var metalist []string
  var repo, repopath, versioncode, compo, arch, ext string
  for {
    var line string
    line, err = input.ReadString('\n')
    if err == io.EOF { break }
    if err != nil {
      fmt.Fprintln(os.Stderr, err)
      os.Exit(1)
    }
    line = strings.TrimSpace(line)
    if line[0] == '!' {
       if !doing_meta {
         metalist = []string{}
       }
       meta = strings.Split(line[1:], ",")
       versioncode, repo, repopath, compo, arch, ext = meta[0],meta[1],meta[2],meta[3],meta[4],meta[5]
       metalist = append(metalist, repo+","+repopath+","+compo+","+arch+","+ext)
       if Versioncode2RepoCommaRepopathSet[versioncode] == nil {
         Versioncode2RepoCommaRepopathSet[versioncode] = map[string]bool{}
       }
       rrp := repo+","+repopath
       Versioncode2RepoCommaRepopathSet[versioncode][rrp] = true
       RepoRepopath2Versioncode[rrp] = versioncode
       if RepoRepopath2CompoArchExtSet[rrp] == nil {
         RepoRepopath2CompoArchExtSet[rrp] = map[string]bool{}
       }
       RepoRepopath2CompoArchExtSet[rrp][compo+","+arch+","+ext] = true
    } else {
      p := strings.Split(line,"|")
      pkg, version, section, desc64, temp64 := p[0],p[1],p[2],p[3],p[4]
      pv := pkg+"|"+version
      for _,m := range metalist {
        if RepoRepopathCompoArchExt2PackagePipeVersionSet[m] == nil {
          RepoRepopathCompoArchExt2PackagePipeVersionSet[m] = map[string]bool{}
        }
        RepoRepopathCompoArchExt2PackagePipeVersionSet[m][pv] = true
        PackagePipeVersion2PData[pv] = &PData{TemplatesBase64:temp64, DescriptionBase64:desc64, Section:section}
      }
    }
  }
}

func process_releases_files() {
  reporepopath2release_todo := map[string]*ReleaseTodo{}
  for _, fairepo := range strings.Fields(FAIrepository) {
    parts := strings.Split(fairepo, "|")
    repo := strings.TrimRight(strings.TrimSpace(parts[0]),"/")
    repopath := strings.TrimRight(strings.TrimSpace(parts[2]),"/")
    components := map[string]bool{}
    for _, com := range strings.Fields(strings.TrimSpace(strings.Replace(parts[3],","," ",-1))) {
      components[com] = true
    }

    reporepopath2release_todo[repo+","+repopath] = &ReleaseTodo{Repo:repo, Repopath:repopath, Components:components}
  }
  
  c := make(chan []string, len(reporepopath2release_todo))
  
  for rs, rt := range reporepopath2release_todo {
    rs2 := rs
    // sends to channel c a []string whose first string is rs2 and whose
    // following strings are the trimmed lines of the Release file for that
    // repo and repopath.
    uri := rt.Repo+"/dists/"+rt.Repopath+"/Release"
    go handle_uri(rs2, uri, c)
  }
  
  tim := time.NewTimer(30*time.Second)
  count := len(reporepopath2release_todo)
  if count == 0 { os.Exit(0) } // nothing to do
  loop:
  for {
    select {
      case release_lines := <- c:  
                       reporepopath := release_lines[0]
                       reporepopath2release_todo[reporepopath].ReleaseFile = release_lines[1:]
                       if count--; count == 0 { 
                         tim.Stop()
                         break loop 
                       }
                       
      case _ = <- tim.C:
                       fmt.Fprintln(os.Stderr, "Timeout while reading Release files => Some data will be filled in from cache!")
                       break loop
    }
  }
  
  for reporepopath, todo := range reporepopath2release_todo {
    versioncode := ""
    compoarch2ext := map[string]string{}
    debian_special_case := strings.Contains(reporepopath, "debian")
    if todo.ReleaseFile != nil {
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
              compoarch := match[2]+","+match[3]
              /* Disabled because it turns out that at least Go's compress/bzip2
              is very slow
              // Prefer "Packages.bz2" over "Packages.gz" over "Packages"
              if len(match[4]) > len(compoarch2ext[compoarch]) {
                compoarch2ext[compoarch] = match[4]
              }
              */
              compoarch2ext[compoarch] = "";
            }
          }
        }
      }
      
      versioncode = codename+"/"+version
      RepoRepopath2Versioncode[reporepopath] = versioncode
      
    } else { // if todo.ReleaseFile == nil
      for vc, rrp := range Versioncode2RepoCommaRepopathSet {
        if rrp[reporepopath] {
          versioncode = vc
          break
        }
      }
      
      if caes, ok := RepoRepopath2CompoArchExtSet[reporepopath]; ok {
        for cae := range caes {
          caesparts := strings.Split(cae, ",")
          if todo.Components[caesparts[0]] && Architectures[caesparts[1]] {
            compoarch2ext[caesparts[0]+","+caesparts[1]] = caesparts[2]
          }
        }
      }
    }
    
    if Versioncode2RepoCommaRepopathSet_new[versioncode] == nil {
      Versioncode2RepoCommaRepopathSet_new[versioncode] = map[string]bool{}
    }
    Versioncode2RepoCommaRepopathSet_new[versioncode][reporepopath] = true
    
    for compoarch, ext := range compoarch2ext {
      if RepoRepopath2CompoArchExtSet_new[reporepopath] == nil {
        RepoRepopath2CompoArchExtSet_new[reporepopath] = map[string]bool{}
      }
      RepoRepopath2CompoArchExtSet_new[reporepopath][compoarch+","+ext] = true
    }
  }
}

func process_packages_files() {
  rrpcae := []string{}
  for rrp, caes := range RepoRepopath2CompoArchExtSet_new {
    for cae := range caes {
      rrpcae = append(rrpcae, rrp+","+cae)
    }
  }
  
  c := make(chan *TaggedBlob, len(rrpcae))
  
  for i := range rrpcae {
    rrpcae_i := rrpcae[i]
    parts := strings.Split(rrpcae_i,",")
    repo, repopath, compo, arch, ext := parts[0], parts[1], parts[2], parts[3], parts[4]
    
    // sends to channel c a TaggedBlob whose Id string is rrpcae_i, Ext is
    // the actual autodetected extension and whose
    // Payload is the compressed Packages file.
    uri := repo+"/dists/"+repopath+"/"+compo+"/binary-"+arch+"/Packages"+ext
    go fetch_uri(rrpcae_i, uri, c)
  }
  
  rrpcae2taggedblob := map[string]*TaggedBlob{}
  tim := time.NewTimer(30*time.Second)
  count := len(rrpcae)
  if count == 0 { os.Exit(0) } // nothing to do
  loop:
  for {
    select {
      case taggedblob := <- c:
                       rrpcae2taggedblob[taggedblob.Id] = taggedblob
                       if count--; count == 0 {
                         tim.Stop()
                         break loop
                       }
                       
      case _ = <- tim.C:
                       fmt.Fprintln(os.Stderr, "Timeout while reading Packages files => Some data will be filled in from cache!")
                       break loop
    }
  }
  
  for i := range rrpcae {
    rrpcae_i := rrpcae[i]
    rrpcaeparts := strings.Split(rrpcae_i, ",")
    var packages_lines []string
    if taggedblob,ok := rrpcae2taggedblob[rrpcae_i]; ok {
      packages_lines = extract(taggedblob.Ext, &taggedblob.Payload)
    }
    
    // Make sure that RepoRepopathCompoArchExt2PackagePipeVersionSet_new[rrpcae_i]
    // exists and has a dummy entry to make sure that the corresponding
    // Release => Repository mapping exists in the output.
    RepoRepopathCompoArchExt2PackagePipeVersionSet_new[rrpcae_i] = map[string]bool{"|"+rrpcae_i:true}
    PackagePipeVersion2PData_new["|"+rrpcae_i] = &PData{}
    
    if packages_lines != nil {
      var p,v,s,d,f string
      var debconf bool
      for _, line := range packages_lines {
        if line == "" {
          if p != "" && v != "" {
            ppv := p+"|"+v
            RepoRepopathCompoArchExt2PackagePipeVersionSet_new[rrpcae_i][ppv] = true
            PackagePipeVersion2PData_new[ppv] = &PData{Section:s, DescriptionBase64:d}
            
            PackagePipeVersion2PData_new[ppv].TemplatesBase64 = "?"
            // if we have that version in the cache already, copy templates
            if pkg, ok := PackagePipeVersion2PData[ppv]; ok {
              PackagePipeVersion2PData_new[ppv].TemplatesBase64 = pkg.TemplatesBase64
            } 
            
            // queue the .deb for scanning if necessary
            if PackagePipeVersion2PData_new[ppv].TemplatesBase64 == "?" {
              if Debconf != "cache" {
                if Debconf != "depends" || debconf {
                  DebsToScan[ppv] = append(DebsToScan[ppv], rrpcaeparts[0]+"/"+f)
                }
              }
            }
          }
          p,v,s,d,f = "","","","",""
          debconf = false
        } else if strings.HasPrefix(line, "Package:") {
          p = strings.Fields(line)[1]
        } else if strings.HasPrefix(line, "Version:") {
          v = strings.Fields(line)[1]
        } else if strings.HasPrefix(line, "Section:") {
          s = strings.Fields(line)[1]
        }  else if strings.HasPrefix(line, "Description:") {
          d = base64.StdEncoding.EncodeToString([]byte(strings.SplitN(line,": ",2)[1]))
        }  else if strings.HasPrefix(line, "Filename:") {
          f = strings.Fields(line)[1]
        } else if strings.HasPrefix(line, "Depends:") || strings.HasPrefix(line, "Pre-Depends:") {
          debconf = strings.Contains(line, "debconf")
        }
      }
    } else { // need to fill in from cache if possible
      if ppv, ok := RepoRepopathCompoArchExt2PackagePipeVersionSet[rrpcae_i]; ok {
        RepoRepopathCompoArchExt2PackagePipeVersionSet_new[rrpcae_i] = ppv
      } else {
        fmt.Fprintf(os.Stderr, "Download failed and no cached data for %v\n", rrpcae_i)
      }
    }
  }
  
  
  // Now fill in PData.Meta field of PackagePipeVersion2PData_new
  for rrpcae, ppvs := range RepoRepopathCompoArchExt2PackagePipeVersionSet_new {
    parts := strings.Split(rrpcae, ",")
    versioncode := RepoRepopath2Versioncode[parts[0]+","+parts[1]]
    for ppv := range ppvs {
      // If we copied some data from the cache into
      // RepoRepopathCompoArchExt2PackagePipeVersionSet_new
      // we may not have the corresponding data in PackagePipeVersion2PData_new
      // In that case, copy it over from the cache, too
      if PackagePipeVersion2PData_new[ppv] == nil {
        PackagePipeVersion2PData_new[ppv] = PackagePipeVersion2PData[ppv]
      }
      PackagePipeVersion2PData_new[ppv].Meta = append(PackagePipeVersion2PData_new[ppv].Meta, versioncode+","+rrpcae)
    }
  }
}


type ByMeta []string
func (a ByMeta) Len() int { return len(a) }
func (a ByMeta) Swap(i,j int) { a[i], a[j] = a[j],a[i] }
func (a ByMeta) Less(i, j int) bool { 
  pdata1 := PackagePipeVersion2PData_new[a[i]]
  pdata2 := PackagePipeVersion2PData_new[a[j]]
  if len(pdata1.Meta) < len(pdata2.Meta) { return true }
  if len(pdata1.Meta) > len(pdata2.Meta) { return false }
  for k := range pdata1.Meta {
    if pdata1.Meta[k] > pdata2.Meta[k] { return false }
    if pdata1.Meta[k] < pdata2.Meta[k] { return true }
  }
  return a[i] < a[j];
}

func shuffle(a []string) {
  for i := range a {
    j := rand.Intn(i + 1)
    a[i], a[j] = a[j], a[i]
  }
}

func debconf_scan() {
  num_scanners := 32
  c := make(chan []string, num_scanners)
  done := make(chan bool)
  
  go func() {
    for ppv, urilist := range DebsToScan {
      shuffle(urilist)
      c <- append(urilist, ppv)
    }
    done <- true
  }()
  
  c2 := make(chan []string, num_scanners)
  for i := 0; i < num_scanners; i++ {
    go func() {
      for {
        urilist := <- c
        ppv := urilist[len(urilist)-1]
        urilist = urilist[0:len(urilist)-1]
        templates64 := extract_templates(urilist)
        if templates64 != "?" {
          c2 <- []string{ppv, templates64}
        }
      }
    }()
  }
  
  tim := time.NewTimer(1*time.Hour)
  loop:
  for {
    select {
      case x := <- c2:
                       PackagePipeVersion2PData_new[x[0]].TemplatesBase64 = x[1]
                       
      case _ = <- tim.C:
                       break loop
      
      case _ = <- done: 
                       // Give scanners some time to process their last package
                       tim.Reset(30*time.Second)
    }
  }
}

func writecache() {
  cache, err := os.Create(filepath.Join(CacheDir,CacheName))
  if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
  defer cache.Close()

  // sort all .Meta properties
  for _, pdata := range PackagePipeVersion2PData_new {
    sort.Strings(pdata.Meta)
  }
  
  // Now get a list of ppv keys sorted by the respective PData.Meta strings.
  // This is done to reduce the amount of output in the cache by grouping
  // package-pipe-version entries by their versioncode (which is at the beginning of .Meta).
  package_pipe_version_list := make([]string, 0, len(PackagePipeVersion2PData_new))
  for ppv := range PackagePipeVersion2PData_new {
    package_pipe_version_list = append(package_pipe_version_list, ppv)
  }
  sort.Sort(ByMeta(package_pipe_version_list))

  currentmeta := ""
  for _, ppv := range package_pipe_version_list {
    pdata := PackagePipeVersion2PData_new[ppv]
    m := strings.Join(pdata.Meta,"|")
    if m != currentmeta {
      currentmeta = m
      for _, meta := range pdata.Meta {
        fmt.Fprintf(cache, "!%s\n", meta)
      }
    }
    
    fmt.Fprintf(cache, "%s|%s|%s|%s\n", ppv, pdata.Section, pdata.DescriptionBase64, pdata.TemplatesBase64)
  }
}

func printldif() {
  for package_pipe_version, pdata := range PackagePipeVersion2PData_new {
    pipe := strings.Index(package_pipe_version, "|")
    pkg := package_pipe_version[0:pipe]
    version := package_pipe_version[pipe+1:]
    prev_release := ""
    for _, meta := range pdata.Meta {
      m := strings.Split(meta,",")
      versioncode,repopath,section := m[0],m[2],m[3]
      vc_slash := strings.Index(versioncode, "/")
      release := versioncode
      // If the repo path does not end in the release version, then
      // we assume the release name should not include the version.
      // E.g. "trusty/14.04" becomes "trusty" because the repo paths
      // for trusty packages are "trusty", "trusty-backports",... which
      // do not include version numbers.
      // For LiMux this turns "tramp/5.0" into "tramp" but keeps
      // "tramp/5.0.0beta7" as is.
      if !strings.HasSuffix(repopath, versioncode[vc_slash:]) {
        release = versioncode[0:vc_slash]
      }
      // Do not output 2 entries for the same package and the same release.
      // In theory this could lose go-susi some information about repo paths,
      // however there is at least one unique dummy entry for each repo path,
      // to make sure each repo path gets output at least once
      if release == prev_release { continue }
      prev_release = release
    
      fmt.Printf("\nRelease: %v\n", release)
      if pkg != "" { // do not output useless data for dummy entry
        fmt.Printf(`Package: %v
Version: %v
Section: %v
Description:: %v
`,  pkg, version, section, pdata.DescriptionBase64)
      }
      if repopath != release || pkg == "" {
        fmt.Printf("Repository: %v\n", repopath)
      }
      if len(pdata.TemplatesBase64) >= 4 {
        fmt.Printf("Templates:: %v\n", pdata.TemplatesBase64)
      }
    }
  }
}


func extract_templates(uris_to_try []string) string {
  var err error
  var resp *http.Response
  var uri string
  
  // Workaround for a condition I encountered during testing where
  // the number of goroutines would shoot up and sockets would not
  // be closed until the program crashed with too many open files.
  for runtime.NumGoroutine() > 300 {
    if Verbose {
      fmt.Fprintln(os.Stderr, "Waiting for goroutines to finish...")
    }
    Transport[0].CloseIdleConnections()
    runtime.GC()
    time.Sleep(5*time.Second)
  }
  
  ok := false
  for _, uri = range uris_to_try {
    resp, err = Client[0].Get(uri)
    if err != nil {
      continue
    }
 
    if resp.StatusCode == 200 {
      ok = true
      break
    }

    // When we get here the connection succeeded but we got an HTTP level error like 404

    resp.Body.Close()
  }
  
  if !ok { return "?" }
  
  cmd := exec.Command("dpkg", "--info","/dev/stdin","templates")
  //cmd := exec.Command("head","-c","200000")
  cmd.Stdin = resp.Body
  var outbuf bytes.Buffer
  cmd.Stdout = &outbuf
  defer outbuf.Reset()
  var errbuf bytes.Buffer
  cmd.Stderr = &errbuf
  defer errbuf.Reset()
  err = cmd.Run()
  templates64 := "?"
  if err != nil && 
    // broken pipe is normal because dpkg stops reading once it has
    // the data it needs
    !strings.Contains(err.Error(), "broken pipe") &&
    // exit status 2 just means that the deb package has no templates
    !strings.Contains(err.Error(), "exit status 2") {
    if strings.Contains(err.Error(), "too many open files") {
      time.Sleep(60*time.Second)
    }
     fmt.Fprintf(os.Stderr, "dpkg --info %v: %v (%v)\n", uri, err, errbuf.String())
  } else {
    if outbuf.Len() > TEMPLATES_MAX_SIZE {
      fmt.Fprintf(os.Stderr, "TOO LARGE %v\n", uri)
    } else {
      templates64 = base64.StdEncoding.EncodeToString(outbuf.Bytes())
      if Verbose && templates64 != "" {
        fmt.Fprintf(os.Stderr, "DEBCONF %v\n", uri)
      }
    }
  }
  
  resp.Body.Close()
  return templates64
}

func handle_uri(line1 string, uri string, c chan []string) {
  var err error
  var resp *http.Response
  
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
  lines := []string{line1}
  for {
    var line string
    line, err = input.ReadString('\n')
    if err != nil { 
      if err == io.EOF { break }
      fmt.Fprintf(os.Stderr, "%v: %v", uri, err)
      return
    }
    lines = append(lines, strings.TrimSpace(line))
  }
  
  c <- lines
  
  if Verbose {
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
*/
var extensions_to_try = []string{".gz", ".bz2", ""}
func fetch_uri(rrpcae string, uri string, c chan *TaggedBlob) {
  var err error
  var resp *http.Response
  
  errors := []string{}
  tb := &TaggedBlob{Id: rrpcae}
  
  for ext_i, extension := range extensions_to_try {
    resp, err = Client[0].Get(uri+extension)
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v: %v\n", uri, err)
      return
    }
 
    if resp.StatusCode == 200 {
      uri = uri+extension
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
      tb.Ext = extension
      defer resp.Body.Close()
      break
    }

    errors = append(errors, fmt.Sprintf("%v: %v\n", uri, resp.Status))
    resp.Body.Close()
    if ext_i+1 == len(extensions_to_try) {
      for _, e := range errors {
        fmt.Fprintln(os.Stderr, e)
      }
      return
    }
  }
  
  _, err = tb.Payload.ReadFrom(resp.Body)
  
  if err != nil { 
    fmt.Fprintf(os.Stderr, "%v: %v", uri, err)
    return
  }
  
  c <- tb
  
  if Verbose {
    fmt.Fprintf(os.Stderr, "OK %v\n", uri)
  }
}  

func extract(ext string, buf *bytes.Buffer) []string {
  var r io.Reader = buf
  var err error
  
  if ext == ".bz2" {
    r = bzip2.NewReader(r)
  } else if ext == ".gz" {
    r, err = gzip.NewReader(r)
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v\n", err)
      return nil
    }
  }
  
  input := bufio.NewReader(r)
  lines := []string{}
  for {
    var line string
    line, err = input.ReadString('\n')
    if err != nil { 
      if err == io.EOF { break }
      fmt.Fprintf(os.Stderr, "%v\n", err)
      return nil
    }
    lines = append(lines, strings.TrimSpace(line))
  }
  return lines
}
