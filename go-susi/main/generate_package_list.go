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
         "bufio"
         "net/url"
         "net/http"
         "strings"
         "time"
         "regexp"
         "compress/bzip2"
         "compress/gzip"
         
         "../util"
         "../bytes"
         "../util/deque"
)

type Package struct {
  Release string
  Package string
  Version string
  Section string
  Description string
  Templates string
  Url string
}

// Which architectures to scan
var architectures = map[string]bool{"i386":true, "amd64":true}

var parse_release_file = regexp.MustCompile("^ [0-9a-f]+\\s+[0-9]+ (([a-z]+)/binary-([a-z0-9]+))/(Packages[.bzg2]*)")

var package_queue deque.Deque
var templates_queue deque.Deque

var MAX_WAIT = 30*time.Second
var NUM_TEMPLATES_SCANNERS = 32

// "skip" : never scan for templates
// "smart": scan for templates if package depends on debconf
// "scan" : always scan for templates
var templates_mode = "scan"

func main() {

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
  client := &http.Client{Transport: tr}
  
  repos := []string{"http://de.archive.ubuntu.com/ubuntu/|ignored|trusty|main,restricted",
                    "http://de.archive.ubuntu.com/ubuntu|ignored|precise|main,universe",
                    }
  
  for i := range repos {
    repo := repos[i] // don't move this into the loop above, or you'll get closure problems
    go util.WithPanicHandler(func(){handle_repo(client, repo)})
  }
  
  for i := 0; i < NUM_TEMPLATES_SCANNERS; i++ {
    go util.WithPanicHandler(func(){handle_templates(client)})
  }
  
  have_seen := map[string]bool{}
  
  for {
     if !package_queue.WaitForItem(MAX_WAIT) { os.Exit(0) }
     pkg := package_queue.Next().(*Package)
     
     // Eliminate duplicates which are possible because we scan
     // multiple architectures
     pkgid := pkg.Package+pkg.Version+pkg.Release
     if have_seen[pkgid] && (pkg.Templates == "skip" || pkg.Templates == "scan" || pkg.Templates == "smart") {
       continue
     }
     have_seen[pkgid] = true
     
     if pkg.Templates == "scan" {
       templates_queue.Push(pkg)
       continue
     }
     
     fmt.Printf(`Release: %v
Package: %v
Version: %v
Section: %v
Description: %v
`, pkg.Release, pkg.Package, pkg.Version, pkg.Section, pkg.Description)
     
     if pkg.Templates != "" && pkg.Templates != "skip" && pkg.Templates != "smart" {
       fmt.Printf("Templates:: %s\n", util.Base64EncodeString(pkg.Templates))
     }
     fmt.Printf("\n")
  }

}  

func handle_templates(client *http.Client) {
  var outbuf bytes.Buffer
  defer outbuf.Reset()
  var errbuf bytes.Buffer
  defer errbuf.Reset()
  for {
    pkg := templates_queue.Next().(*Package)
    pkg.Templates = "" // default is empty templates
    resp, err := client.Get(pkg.Url)
    if err != nil && err != io.EOF {
      util.Log(0, "ERROR! handle_templates Get(\"%v\"): %v", pkg.Url, err)
    } else {
      if resp.StatusCode != 200 {
        util.Log(0, "ERROR! handle_templates Get(\"%v\"): %v", pkg.Url, resp.Status)
        resp.Body.Close()
      } else {
        cmd := exec.Command("dpkg", "--info","/dev/stdin","templates")
        cmd.Stdin = resp.Body
        outbuf.Reset()
        cmd.Stdout = &outbuf
        errbuf.Reset()
        cmd.Stderr = &errbuf
        err = cmd.Run()
        resp.Body.Close()
        if err != nil && 
          // broken pipe is normal because dpkg stops reading once it has
          // the data it needs
          !strings.Contains(err.Error(), "broken pipe") &&
          // exit status 2 just means that the deb package has no templates
          !strings.Contains(err.Error(), "exit status 2") {
           util.Log(0, "ERROR! dpkg --info %v: %v (%v)", pkg.Url, err, errbuf.String())
        }
        // always output result even if error (best effort)
        pkg.Templates = outbuf.String()
      }
    }
    package_queue.Push(pkg)
  }
}


func handle_repo(client *http.Client, repo string) {
  parts := strings.Split(repo, "|")
  base := strings.TrimSpace(strings.TrimRight(parts[0],"/"))
  release := strings.TrimSpace(parts[2])
  components := map[string]bool{}
  for _, com := range strings.Fields(strings.TrimSpace(strings.Replace(parts[3],","," ",-1))) {
    components[com] = true
  }
  
  uri := base+"/dists/"+release+"/Release"
  resp, err := client.Get(uri)
  if err != nil {
    util.Log(0, "ERROR! handle_repo Get(\"%v\"): %v", uri, err)
    return
  }
  defer resp.Body.Close()
  
  if resp.StatusCode != 200 {
    util.Log(0, "ERROR! handle_repo Get(\"%v\"): %v", uri, resp.Status)
    return
  }
  
  // maps a path like "main/binary-amd64" to the name of
  // the best compressed Packages file within that directory, e.g. "Packages.bz2"
  packages_files := map[string]string{}
  
  input := bufio.NewReader(resp.Body)
  for {
    var line string
    line, err = input.ReadString('\n')
    if err != nil { 
      if err != io.EOF {
        util.Log(0, "ERROR! ReadString(\"%v\"): %v", uri, err)
        // don't return; keep going with what we have so far
      }
      break 
    }
    
    match := parse_release_file.FindStringSubmatch(line)
    if match != nil {
      if components[match[2]] && architectures[match[3]] {
        if len(match[4]) > len(packages_files[match[1]]) {
          packages_files[match[1]] = match[4]
        }
      }
    }
  }
  
  for k,v := range packages_files {
    packages_uri := base + "/dists/"+release+"/"+k+"/"+v
    go util.WithPanicHandler(func(){handle_packages_file(client, base, release, packages_uri)})
  }
}

func handle_packages_file(client *http.Client, base, release, packages_uri string) {
  resp, err := client.Get(packages_uri)
  if err != nil {
    util.Log(0, "ERROR! handle_packages_file Get(\"%v\"): %v", packages_uri, err)
    return
  }
  defer resp.Body.Close()
  
  if resp.StatusCode != 200 {
    util.Log(0, "ERROR! handle_packages_file Get(\"%v\"): %v", packages_uri, resp.Status)
    return
  }
  
  var r io.Reader = resp.Body
  
  if strings.HasSuffix(packages_uri, ".bz2") {
    r = bzip2.NewReader(r)
  } else if strings.HasSuffix(packages_uri, ".gz") {
    r, err = gzip.NewReader(r)
    if err != nil {
      util.Log(0, "ERROR! Reading from %v: %v", packages_uri, err)
      return
    }
  }
  
  input := bufio.NewReader(r)
  pkg := &Package{Release:release, Templates:templates_mode}
  for {
    var line string
    line, err = input.ReadString('\n')
    if err != nil { 
      if err == io.EOF { break }
      util.Log(0, "ERROR! ReadString(\"%v\"): %v", packages_uri, err)
      return
    }
    line = strings.TrimSpace(line)
    if line == "" {
      //send pkg over the line to the next process stage
      package_queue.Push(pkg)
      // start a new pkg
      pkg = &Package{Release:release, Templates:templates_mode}
    } else if strings.HasPrefix(line, "Package:") {
      pkg.Package = strings.Fields(line)[1]
    } else if strings.HasPrefix(line, "Version:") {
      pkg.Version = strings.Fields(line)[1]
    } else if strings.HasPrefix(line, "Section:") {
      pkg.Section = strings.Fields(line)[1]
    }  else if strings.HasPrefix(line, "Description:") {
      pkg.Description = strings.SplitN(line,": ",2)[1]
    }  else if strings.HasPrefix(line, "Filename:") {
      pkg.Url = base+"/"+strings.Fields(line)[1]
    } else if strings.HasPrefix(line, "Depends:") {
      if pkg.Templates != "skip" && strings.Contains(line, "debconf") {
        pkg.Templates = "scan"
      }
    }
  }
  
}
