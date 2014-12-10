/*
Copyright (c) 2013 Matthias S. Benkmann

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

// API for the various databases used by go-susi.
package db

import (
         "fmt"
         "sync"
         "time"
         "os"
         "os/exec"
         "runtime"
         "strings"
         
         "../xml"
         "../util"
         "../config"
         "../bytes"
       )

// used to prevent the hooks from being started while they are still running,
// e.g. because someone sends SIGUSR2 twice in quick succession.
var hookMutex sync.Mutex

// Run KernelListHook() and PackageListHook() to update the respective databases.
// This happens in the background. This function does not wait for them to complete.
// startup == true => This is the initial call right after go-susi starts.
func HooksExecute(startup bool) {
  go util.WithPanicHandler(func(){runHooks(startup)})
  go util.WithPanicHandler(FAIReleasesListUpdate)
}

// startup == true => This is the initial call right after go-susi starts.
func runHooks(startup bool) {
  hookMutex.Lock()
  defer hookMutex.Unlock()
  
  // The hooks process large LDIFs and convert them to xml.Hashes.
  // This drives memory usage up. Execute the hooks in sequence and
  // call garbage collection in hopes of keeping memory usage in check.
  
  runtime.GC()
  if startup {
    PackageListHook("cache")
    runtime.GC()
    KernelListHook()
    runtime.GC()
    PackageListHook("depends")
    runtime.GC()
  } else {
    PackageListHook("all")
    runtime.GC()
    KernelListHook()
    runtime.GC()
  }
}

// Reads the output from the program config.KernelListHookPath (LDIF) and
// uses it to replace kerneldb.
func KernelListHook() {
  start := time.Now()
  util.Log(1, "INFO! Running kernel-list-hook %v", config.KernelListHookPath)
  cmd := exec.Command(config.KernelListHookPath)
  cmd.Env = append(config.HookEnvironment(), os.Environ()...)
  cmd.Env = append(cmd.Env, "PackageListCacheDir="+config.PackageCacheDir)
  klist, err := xml.LdifToHash("kernel", true, cmd)
  if err != nil {
    util.Log(0, "ERROR! kernel-list-hook %v: %v", config.KernelListHookPath, err)
    return
  }
  if klist.First("kernel") == nil {
    util.Log(0, "ERROR! kernel-list-hook %v returned no data", config.KernelListHookPath)
    return
  }
  util.Log(1, "INFO! Finished kernel-list-hook. Running time: %v", time.Since(start))
  
  kerneldata := xml.NewHash("kerneldb")
  
  accepted := 0
  total := 0
  
  for kernel := klist.First("kernel"); kernel != nil; kernel = kernel.Next() {
    total++
    cn := kernel.Get("cn")
    if len(cn) == 0 {
      util.Log(0, "ERROR! kernel-list-hook %v returned entry without cn: %v", config.KernelListHookPath, kernel)
      continue
    }
    if len(cn) > 1 {
      util.Log(0, "ERROR! kernel-list-hook %v returned entry with multiple cn values: %v", config.KernelListHookPath, kernel)
      continue
    }
    
    release := kernel.Get("release")
    if len(release) == 0 {
      util.Log(0, "ERROR! kernel-list-hook %v returned entry without release: %v", config.KernelListHookPath, kernel)
      continue
    }
    if len(release) > 1 {
      util.Log(0, "ERROR! kernel-list-hook %v returned entry with multiple release values: %v", config.KernelListHookPath, kernel)
      continue
    }
    
    k := xml.NewHash("kernel","fai_release",release[0])
    k.Add("cn", cn[0])
    kerneldata.AddWithOwnership(k)
    accepted++
  }
  
  if kerneldata.First("kernel") == nil {
    util.Log(0, "ERROR! kernel-list-hook %v returned no valid entries", config.KernelListHookPath)
  } else {
    util.Log(1, "INFO! kernel-list-hook: %v/%v entries accepted into database", accepted,total)
    kerneldb.Init(kerneldata)
  }
}

var packageListFormat = []*xml.ElementInfo{
  &xml.ElementInfo{"package","package",false},
  &xml.ElementInfo{"release","distribution",false},
  &xml.ElementInfo{"repository","repository",false},
  &xml.ElementInfo{"version","version",false},
  &xml.ElementInfo{"section","section",false},
  &xml.ElementInfo{"description","description",true},
  &xml.ElementInfo{"template","template",true},
  &xml.ElementInfo{"templates","template",true},
}

// Reads the output from the program config.PackageListHookPath (LDIF) and
// uses it to replace packagedb.
// debconf is passed as PackageListDebconf environment var to the hook.
// See manual section on package-list-hook for more info.
func PackageListHook(debconf string) {
  start := time.Now()
  timestamp := util.MakeTimestamp(start)

  cmd := exec.Command(config.PackageListHookPath)
  cmd.Env = append(config.HookEnvironment(), os.Environ()...)
  
  fairepos := []string{}
  for repo := FAIServers().First("repository"); repo != nil; repo = repo.Next() {
    fairepos = append(fairepos, fmt.Sprintf("%v||%v|%v", repo.Text("server"), repo.Text("repopath"), repo.Text("sections")))
  }
  
  package_list_params := []string{"PackageListDebconf="+debconf, "PackageListCacheDir="+config.PackageCacheDir, "PackageListFAIrepository="+strings.Join(fairepos," ")}
  cmd.Env = append(cmd.Env, package_list_params...)
  util.Log(1, "INFO! Running package-list-hook: %v %v", strings.Join(package_list_params, " "), config.PackageListHookPath)

  var outbuf bytes.Buffer
  defer outbuf.Reset()
  var errbuf bytes.Buffer
  defer errbuf.Reset()
  cmd.Stdout = &outbuf
  cmd.Stderr = &errbuf
  err := cmd.Run()
  
  if err != nil {
    util.Log(0, "ERROR! package-list-hook %v: %v (%v)", config.PackageListHookPath, err, errbuf.String())
    return
  } else if errbuf.Len() != 0 {
    // if the command prints to stderr but does not return non-0 exit status (which
    // would result in err != nil), we just log a WARNING, but use the stdout data
    // anyway.
    util.Log(0, "WARNING! package-list-hook %v: %v", config.PackageListHookPath, errbuf.String())
  }
      
  plist, err := xml.LdifToHash("pkg", true, outbuf.Bytes(), packageListFormat...)
  if err != nil {
    util.Log(0, "ERROR! package-list-hook %v: %v", config.PackageListHookPath, err)
    return
  }
  if plist.First("pkg") == nil {
    util.Log(0, "ERROR! package-list-hook %v returned no data", config.PackageListHookPath)
    return
  }
  util.Log(1, "INFO! Finished package-list-hook. Running time: %v", time.Since(start))
  
  start = time.Now()
  
  plist.Rename("packagedb")
  
  new_mapRepoPath2FAIrelease := map[string]string{}
  
  accepted := 0
  total := 0
  
  for pkg := plist.FirstChild(); pkg != nil; pkg = pkg.Next() {
    total++
    p := pkg.Element()
    
    release := p.First("distribution") // packageListFormat translates "release" => "distribution"
    if release == nil {
      util.Log(0, "ERROR! package-list-hook %v returned entry without \"Release\": %v", config.PackageListHookPath, p)
      pkg.Remove()
      continue
    }
    
    for repopath := p.First("repository"); repopath != nil; repopath = repopath.Next() {
      new_mapRepoPath2FAIrelease[repopath.Text()] = release.Text()
    }
    
    pkgname := p.Get("package")
    if len(pkgname) == 0 {
      if p.First("repository") == nil { // Release/Repository groups without Package are okay, so only log error if there is no Repository
        util.Log(0, "ERROR! package-list-hook %v returned entry without \"Package\": %v", config.PackageListHookPath, p)
      }
      pkg.Remove()
      continue
    }
    
    if len(pkgname) > 1 {
      util.Log(0, "ERROR! package-list-hook %v returned entry with multiple \"Package\" values: %v", config.PackageListHookPath, p)
      pkg.Remove()
      continue
    }

    version := p.First("version")
    if version == nil {
      util.Log(0, "WARNING! package-list-hook %v returned entry for \"%v\" without \"Version\". Assuming \"1.0\"", config.PackageListHookPath, pkgname[0])
      p.Add("version", "1.0")
    }
    
    section := p.First("section")
    if section == nil {
      util.Log(0, "WARNING! package-list-hook %v returned entry for \"%v\" without \"Section\". Assuming \"main\"", config.PackageListHookPath, pkgname[0])
      p.Add("section", "main")
    }
    
    p.FirstOrAdd("timestamp").SetText(timestamp)
    
    description := p.First("description")
    if description == nil {
      description = p.Add("description", pkgname[0])
      description.EncodeBase64()
    }
    
    // add empty <template></template> if there is no <template> element.
    if p.First("template") == nil { p.Add("template") }

    accepted++
  }
  
  if accepted == 0 {
    util.Log(0, "ERROR! package-list-hook %v returned no valid entries", config.PackageListHookPath)
  } else {
    util.Log(1, "INFO! package-list-hook: %v/%v entries accepted into database. Processing time: %v", accepted,total, time.Since(start))
    packagedb.Init(plist)
    mapRepoPath2FAIrelease_mutex.Lock()
    defer mapRepoPath2FAIrelease_mutex.Unlock()
    mapRepoPath2FAIrelease = new_mapRepoPath2FAIrelease
    util.Log(1, "INFO! Repository path => FAI release %v", mapRepoPath2FAIrelease)
  }
}

