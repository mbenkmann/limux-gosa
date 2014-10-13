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
         
         "../util"
       )

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
  Meta []string // List of repo+","+repopath+","+compo+","+arch+","+ext+","+versioncode strings
  TemplatesBase64 string
  Section string
  DescriptionBase64 string
}

var PackagePipeVersion2PData = map[string]*PData {}
var PackagePipeVersion2PData_new = map[string]*PData {}

const CachePath = "/home/msb/devel/go/susi/pkg.cache"

var FAIReposFromLDAP = []string{"http://de.archive.ubuntu.com/ubuntu/|ignored|trusty|main,restricted",
                    "http://de.archive.ubuntu.com/ubuntu|ignored|precise|main,universe",
                    }

type ReleaseTodo struct {
  Repo string
  Repopath string
  Components map[string]bool
  ReleaseFile []string
}
 
var Client *http.Client

func main() {
  initclient()
  readcache()
  process_releases_files()
  process_packages_files() 
  writecache()
}

func initclient() {
  tr := &http.Transport{
    // proxy function examines Request r and decides if
    // a proxy should be used. If the returned error is non-nil,
    // the request is aborted. If the returned URL is nil,
    // no proxy is used. Otherwise URL is the URL of the
    // proxy to use.
    Proxy: func(r *http.Request) (*url.URL, error) {
      return nil, nil
    },
  }
  
  // the same Client object can (and for efficiency reasons should)
  // be used in all goroutines according to net/http docs.
  Client = &http.Client{Transport: tr}
}

func readcache() {
  cache, err := os.Open(CachePath)
  if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
  defer cache.Close()
  input := bufio.NewReader(cache)
  doing_meta := false
  var meta []string
  var prevmeta []string 
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
       for i := range meta {
         if meta[i] == "^" { meta[i] = prevmeta[i] }
       }
       prevmeta = meta
       repo, repopath, compo, arch, ext, versioncode = meta[0],meta[1],meta[2],meta[3],meta[4],meta[5]
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
  for _, fairepo := range FAIReposFromLDAP {
    parts := strings.Split(fairepo, "|")
    repo := strings.TrimSpace(strings.TrimRight(parts[0],"/"))
    repopath := strings.TrimSpace(parts[2])
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
    if todo.ReleaseFile != nil {
      codename := ""
      version := ""
      for _, line := range todo.ReleaseFile {
        if strings.HasPrefix(line, "Codename: ") {
          codename = line[10:]
        } else if strings.HasPrefix(line, "Version: ") {
          version = line[9:]
        } else {
          match := parse_release_file.FindStringSubmatch(line)
          if match != nil {
            if todo.Components[match[2]] && Architectures[match[3]] {
              compoarch := match[2]+","+match[3]
              // prefer "Packages.bz2" over "Packages.gz" and "Packages.gz" over "Packages"
              if len(match[4]) > len(compoarch2ext[compoarch]) {
                compoarch2ext[compoarch] = match[4]
              }
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
  
  c := make(chan []string, len(rrpcae))
  
  for i := range rrpcae {
    rrpcae_i := rrpcae[i]
    parts := strings.Split(rrpcae_i,",")
    repo, repopath, compo, arch, ext := parts[0], parts[1], parts[2], parts[3], parts[4]
    
    // sends to channel c a []string whose first string is rrpcae_i and whose
    // following strings are the trimmed lines of the Packages file
    uri := repo+"/dists/"+repopath+"/"+compo+"/binary-"+arch+"/Packages"+ext
    go handle_uri(rrpcae_i, uri, c)
  }
  
  rrpcae2packages := map[string][]string{}
  tim := time.NewTimer(30*time.Second)
  count := len(rrpcae)
  if count == 0 { os.Exit(0) } // nothing to do
  loop:
  for {
    select {
      case packages_lines := <- c:
                       rrpcae := packages_lines[0]
                       rrpcae2packages[rrpcae] = packages_lines[1:]
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
    if packages_lines,ok := rrpcae2packages[rrpcae_i]; ok {
      var p,v,s,d,f string
      var debconf bool
      for _, line := range packages_lines {
        if line == "" {
          if p != "" && v != "" {
            ppv := p+"|"+v
            if RepoRepopathCompoArchExt2PackagePipeVersionSet_new[rrpcae_i] == nil {
              RepoRepopathCompoArchExt2PackagePipeVersionSet_new[rrpcae_i] = map[string]bool{}
            }
            RepoRepopathCompoArchExt2PackagePipeVersionSet_new[rrpcae_i][ppv] = true
            PackagePipeVersion2PData_new[ppv] = &PData{Section:s, DescriptionBase64:d}
            
            // if we have that version in the cache already, copy templates
            if pkg, ok := PackagePipeVersion2PData[ppv]; ok {
              PackagePipeVersion2PData_new[ppv].TemplatesBase64 = pkg.TemplatesBase64
            } else { // otherwise we (may) need to scan the .deb
              if debconf {
                DebsToScan[ppv] = append(DebsToScan[ppv], rrpcaeparts[0]+"/"+f)
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
          d = string(util.Base64EncodeString(strings.SplitN(line,": ",2)[1]))
        }  else if strings.HasPrefix(line, "Filename:") {
          f = strings.Fields(line)[1]
        } else if strings.HasPrefix(line, "Depends:") {
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
      PackagePipeVersion2PData_new[ppv].Meta = append(PackagePipeVersion2PData_new[ppv].Meta, rrpcae+","+versioncode)
    }
  }
}

func handle_uri(line1 string, uri string, c chan []string) {
  resp, err := Client.Get(uri)
  if err != nil {
    fmt.Fprintf(os.Stderr, "%v: %v\n", uri, err)
    return
  }
  defer resp.Body.Close()
  
  if resp.StatusCode != 200 {
    fmt.Fprintf(os.Stderr, "%v: %v\n", uri, resp.Status)
    return
  }
  
  var r io.Reader = resp.Body
  
  if strings.HasSuffix(uri, ".bz2") {
    r = bzip2.NewReader(r)
  } else if strings.HasSuffix(uri, ".gz") {
    r, err = gzip.NewReader(r)
    if err != nil {
      fmt.Fprintf(os.Stderr, "%v: %v\n", uri, err)
      return
    }
  }
  
  input := bufio.NewReader(r)
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
}  

func writecache() {
  cache, err := os.Create(CachePath+"2")
  if err != nil {
    fmt.Fprintln(os.Stderr, err)
    os.Exit(1)
  }
  defer cache.Close()

  currentmeta := ""
  for ppv, pdata := range PackagePipeVersion2PData_new {
    sort.Strings(pdata.Meta)
    m := strings.Join(pdata.Meta,"|")
    if m != currentmeta {
      currentmeta = m
      prevparts := []string{"","","","","",""}
      for _, meta := range pdata.Meta {
        parts := strings.Split(meta,",")
        for i := range parts {
          if parts[i] == prevparts[i] {
            parts[i] = "^"
          } else {
            prevparts[i] = parts[i]
          }
        }
        fmt.Fprintf(cache, "!%s\n", strings.Join(parts, ","))
      }
    }
    
    fmt.Fprintf(cache, "%s|%s|%s|%s\n", ppv, pdata.Section, pdata.DescriptionBase64, pdata.TemplatesBase64)
  }
}
