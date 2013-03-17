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
         "time"
         "os/exec"
         
         "../xml"
         "../util"
         "../config"
       )

// Run KernelListHook() and PackageListHook() to update the respective databases.
// This happens in the background. This function does not wait for them to complete.
func HooksExecute() {
  go util.WithPanicHandler(KernelListHook)
  go util.WithPanicHandler(PackageListHook)
}

// Reads the output from the program config.KernelListHookPath (LDIF) and
// uses it to replace kerneldb.
func KernelListHook() {
  util.Log(1, "INFO! Running kernel-list-hook %v", config.KernelListHookPath)
  klist, err := xml.LdifToHash("kernel", true, exec.Command(config.KernelListHookPath))
  if err != nil {
    util.Log(0, "ERROR! kernel-list-hook %v: %v", config.KernelListHookPath, err)
    return
  }
  if klist.First("kernel") == nil {
    util.Log(0, "ERROR! kernel-list-hook %v returned no data", config.KernelListHookPath)
    return
  }
  util.Log(1, "INFO! Finished kernel-list-hook %v ", config.KernelListHookPath)
  
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


// Reads the output from the program config.PackageListHookPath (LDIF) and
// uses it to replace packagedb.
func PackageListHook() {
  timestamp := util.MakeTimestamp(time.Now())

  util.Log(1, "INFO! Running package-list-hook %v", config.PackageListHookPath)
  plist, err := xml.LdifToHash("pkg", true, exec.Command(config.PackageListHookPath))
  if err != nil {
    util.Log(0, "ERROR! package-list-hook %v: %v", config.PackageListHookPath, err)
    return
  }
  if plist.First("pkg") == nil {
    util.Log(0, "ERROR! package-list-hook %v returned no data", config.PackageListHookPath)
    return
  }
  util.Log(1, "INFO! Finished package-list-hook %v ", config.PackageListHookPath)
  
  pkgdata := xml.NewHash("packagedb")
  
  accepted := 0
  total := 0
  
  for pkg := plist.First("pkg"); pkg != nil; pkg = pkg.Next() {
    total++
    pkgname := pkg.Get("package")
    if len(pkgname) == 0 {
      util.Log(0, "ERROR! kernel-list-hook %v returned entry without \"Package\": %v", config.PackageListHookPath, pkg)
      continue
    }
    if len(pkgname) > 1 {
      util.Log(0, "ERROR! kernel-list-hook %v returned entry with multiple \"Package\" values: %v", config.PackageListHookPath, pkg)
      continue
    }
    
    release := pkg.Get("release")
    if len(release) == 0 {
      util.Log(0, "ERROR! package-list-hook %v returned entry without \"Release\": %v", config.PackageListHookPath, pkg)
      continue
    }
    if len(release) > 1 {
      util.Log(0, "ERROR! package-list-hook %v returned entry with multiple \"Release\" values: %v", config.PackageListHookPath, pkg)
      continue
    }
    
    version := pkg.Text("version")
    if version == "" {
      util.Log(0, "WARNING! package-list-hook %v returned entry for \"%v\" without \"Version\". Assuming \"1.0\"", config.PackageListHookPath, pkgname[0])
      version = "1.0"
    }
    
    section := pkg.Text("section")
    if section == "" {
      util.Log(0, "WARNING! package-list-hook %v returned entry for \"%v\" without \"Section\". Assuming \"main\"", config.PackageListHookPath, pkgname[0])
      section = "main"
    }
    
    description := pkg.Text("description")
    if description == "" { description = pkgname[0] }
    
    p := xml.NewHash("pkg","distribution",release[0])
    p.Add("package", pkgname[0])
    p.Add("timestamp",timestamp)
    p.Add("version",version)
    p.Add("section",section)
    p.Add("description", description)
      // accept "template" and "templates" (with and without "s")
    p.Add("template",pkg.Text("template")+pkg.Text("templates"))

    pkgdata.AddWithOwnership(p)
    accepted++
  }
  
  if pkgdata.First("pkg") == nil {
    util.Log(0, "ERROR! package-list-hook %v returned no valid entries", config.PackageListHookPath)
  } else {
    util.Log(1, "INFO! package-list-hook: %v/%v entries accepted into database", accepted,total)
    packagedb.Init(pkgdata)
  }
}

