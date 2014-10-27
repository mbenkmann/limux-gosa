<?php
//
// This code is part of GOsa (http://www.gosa-project.org)
//
// Copyright (C) 2014 Landeshauptstadt München
// Author: Matthias S. Benkmann
//
// This program is free software;
// you can redistribute it and/or modify it under the terms of the
// GNU General Public License as published by the Free Software Foundation;
// either version 2 of the License, or (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See the GNU General Public License for more details.
//
//
// IMPORTANT: YOU NEED TO CONFIGURE YOUR /etc/ldap/ldap.conf PROPERLY,
// OR YOU WILL HAVE TO EDIT ldap2fai-pxelinuxcfg-common.php WITH YOUR
// LDAP CONFIGURATION.
//
// This script constructs a FAI config space from the data found in LDAP
// as written by GOsa. The config space is sent as .tar.gz file to the
// requesting client.
//
// If no MAC address is found in the request URI, the IP address of the
// requesting client will be used to find the LDAP object whose config
// space will be constructed.
//
// To request the config space of a particular machine this script can be
// called with a MAC-address in the request URI,
// either by appending a ?...<MACADDRESS> query string or by including the
// MAC address directly in the file name part of the URI, e.g.
// by aliasing the URL http://server/config.tar.gz/<MACADDRESS>
// to this script. See /etc/gosa/fai-helpers-apache.conf for an example.
// Everything works as long as the request URI ends in a
// MAC address using either ":" or "-" as separator.
//
// If no MAC address is found in the request URI, the IP address of the
// requesting client will be used to find the LDAP object.

/*
 * Close LDAP connection, log error and return 500 Internal Server Error.
 */

// xdebug_start_trace("/tmp/trace.log");
function ldapdie($ldap, $msg)
{
    // http_response_code(500);
    $protocol = (isset($_SERVER['SERVER_PROTOCOL']) ? $_SERVER['SERVER_PROTOCOL'] : 'HTTP/1.0');
    header("$protocol 500 Internal Server Error");

    if ($ldap) {
        $err = ldap_error($ldap);
        @ldap_close($ldap);
        if ($err != "Success")
            $msg = sprintf("%s: %s\n", $msg, $err);
    }
    die($msg);
}

// This is called if LDAP access works but the object is not found.
function no_ldap_entry_for_machine($ldap)
{
    ldapdie($ldap, "Could not find client in LDAP");
}

$required = array("faiclass" => TRUE,"cn" => TRUE,"faidebianmirror" => TRUE);
require (dirname(__FILE__) . '/ldap2fai-pxelinuxcfg-common.php');

// $config_space: maps relative file path to string contents of file
// $faiobject: A map as returned by ldap_get_entries(...)[0]. $faiobject['..'] returns the parent object.
// The function adds the information from $faiobject to $config_space.
function FAIpartitionEntry(&$config_space, &$faiobject)
{
    $classname = $faiobject['..']['..']['cn'][0];
    $diskname = $faiobject['..']['cn'][0];
    $data = isset($config_space["disk_config/$classname:$diskname"]) ? $config_space["disk_config/$classname:$diskname"] : "";
    $lines = explode("\n", $data);
    $idx = intval($faiobject['faipartitionnr'][0]);
    while (count($lines) <= $idx + 1) { // +1 because we want the file to end with a newline
        $lines[] = "";
    }
    $FAIPartitionType = aget($faiobject, 'faipartitiontype');
    $FAIMountPoint = aget($faiobject, 'faimountpoint');
    $FAIPartitionSize = aget($faiobject, 'faipartitionsize');
    $FAIfstype = aget($faiobject, 'faifstype');
    $FAIfsoptions = aget($faiobject, 'faifsoptions');
    if ($FAIfstype == "ext3")
        $FAIfsoptions = preg_replace('/-j\b/', '', $FAIfsoptions);
    $FAIMountOptions = aget($faiobject, 'faimountoptions');
    if (empty($FAIMountOptions))
        $FAIMountOptions = "rw";
    $FAIfsCreateOptions = aget($faiobject, 'faifscreateoptions');
    if (empty($FAIfsCreateOptions))
        $FAIfsCreateOptions = preg_replace('/\bboot\b/', '', $FAIfsoptions);
    $FAIfsCreateOptions = trim($FAIfsCreateOptions);
    if (! empty($FAIfsCreateOptions))
        $FAIfsCreateOptions = "createopts=\"$FAIfsCreateOptions\"";
    if ($FAIfstype == "preserve")
        $FAIfstype = "-";
    if ($FAIPartitionSize == "preserve")
        $FAIPartitionSize = "0-";
    if ($FAIfstype == "swap")
        $lines[$idx] = sprintf("%-7s %-12s %-12s %-5s %-10s", $FAIPartitionType, $FAIMountPoint, $FAIPartitionSize, $FAIMountPoint, $FAIMountOptions);
    else
        $lines[$idx] = sprintf("%-7s %-12s %-12s %-5s %-10s %s", $FAIPartitionType, $FAIMountPoint, $FAIPartitionSize, $FAIfstype, $FAIMountOptions, $FAIfsCreateOptions);
    if (preg_match('/\bboot\b/', $FAIfsoptions)) {
        $lines[0] .= " bootable:<$idx>";
    }
    if (preg_match('/\bpreserve\b/', aget($faiobject, 'faipartitionflags'))) {
        $lines[0] .= " preserve_always:<$idx>";
    }
    $data = implode("\n", $lines);
    $config_space["disk_config/$classname:$diskname"] = $data;
}

function FAIpartitionDisk(&$config_space, &$faiobject)
{
    $classname = $faiobject['..']['cn'][0];
    $diskname = $faiobject['cn'][0];
    $data = isset($config_space["disk_config/$classname:$diskname"]) ? $config_space["disk_config/$classname:$diskname"] : "";
    $lines = explode("\n", $data);
    $options = isset($faiobject['faidiskoption']) ? $faiobject['faidiskoption'] : array();
    unset($options["count"]);
    if (! preg_grep('/^fstabkey:/', $options)) {
        $options[] = "fstabkey:uuid";
    }
    if (! preg_grep('/^align-at:/', $options)) {
        $options[] = "align-at:1048576B";
    }
    $lines[0] = "disk_config $diskname " . implode(" ", $options) . $lines[0];
    $config_space["disk_config/$classname:$diskname"] = implode("\n", $lines);
}

function FAIpartitionTable(&$config_space, &$faiobject)
{}

function FAIpackageList(&$config_space, &$faiobject)
{
    $classname = $faiobject['cn'][0];
    $faiinstallmethod = aget($faiobject, "faiinstallmethod");
    unset($faiobject["faipackage"]["count"]);
    $packages = implode("\n", $faiobject["faipackage"]);
    $config_space["package_config/$classname"] = "PACKAGES $faiinstallmethod\n$packages\n";
    unset($faiobject["faidebiansection"]["count"]);
    $arr = $faiobject["faidebiansection"];
    $release = aget($faiobject, "faidebianrelease");
    array_unshift($arr, $release);
    $config_space["deb"][] = $arr;
}

function FAIdebconfInfo(&$config_space, &$faiobject)
{
    $classname = $faiobject['..']['cn'][0];
    $data = isset($config_space["debconf/$classname"]) ? $config_space["debconf/$classname"] : "";
    $FAIpackage = aget($faiobject, "faipackage");
    $FAIvariable = aget($faiobject, "faivariable");
    $FAIvariableType = aget($faiobject, "faivariabletype");
    $FAIvariableContent = aget($faiobject, "faivariablecontent");
    $config_space["debconf/$classname"] = $data . "$FAIpackage $FAIvariable $FAIvariableType $FAIvariableContent\n";
}

function FAIscriptEntry(&$config_space, &$faiobject)
{
    $classname = $faiobject['..']['cn'][0];
    $name = $faiobject['cn'][0];
    $prio = aget($faiobject, "faipriority", "0");
    while (strlen($prio) < 2)
        $prio = "0$prio";
    $code = aget($faiobject, "faiscript");
    $config_space["scripts/$classname/$prio-$name"] = $code;
}

function FAIscript(&$config_space, &$faiobject)
{}

function FAItemplateEntry(&$config_space, &$faiobject)
{
    $classname = $faiobject['..']['cn'][0];
    $path = aget($faiobject, "faitemplatepath");
    $contents = aget($faiobject, "faitemplatefile");
    if ($path[0] != '/')
        $path = "/$path";
    $config_space["files$path/$classname"] = $contents;
    $owner = str_replace(".", " ", aget($faiobject, "faiowner"));
    $mode = aget($faiobject, "faimode");
    $config_space["files$path/file-modes"] = (isset($config_space["files$path/file-modes"]) ? $config_space["files$path/file-modes"] : "") . "$owner $mode $classname\n";
}

function FAItemplate(&$config_space, &$faiobject)
{}

function FAIvariableEntry(&$config_space, &$faiobject)
{
    $classname = $faiobject['..']['cn'][0];
    $path = "class/$classname.var";
    $data = isset($config_space[$path]) ? $config_space[$path] : "";
    $FAIvariable = aget($faiobject, "cn");
    $FAIvariableContent = aget($faiobject, "faivariablecontent");
    $data .= "$FAIvariable='$FAIvariableContent'";
    $tmp = explode("\n", $data);
    sort($tmp);
    $data = implode("\n", $tmp) . "\n";
    $config_space[$path] = $data;
}

function FAIvariable(&$config_space, &$faiobject)
{}

function FAIhookEntry(&$config_space, &$faiobject)
{
    $classname = $faiobject['..']['cn'][0];
    $task = aget($faiobject, "faitask");
    if ($task == "prepareapt")
        $task = "repository"; // "prepareapt" is deprecated, use "repository" instead
    $code = aget($faiobject, "faiscript");
    $config_space["hooks/$task.$classname"] = $code;
}

function FAIhook(&$config_space, &$faiobject)
{}

// Determine the ou=fai for the machine. We do this by going through all ou=fai and taking the
// one whose dn starts with "ou=fai,ou=configs,ou=systems," and continues with a suffix of
// $machine_dn. All DNs of these ou=fai that match will be returned in an array that is sorted so that the
// longest DN is element 0 of the array.
function find_fai_ou($ldap, $ldap_base_top, $machine_dn)
{
    $results = ldap_search($ldap, $ldap_base_top, "(&(objectClass=organizationalUnit)(ou=fai))", array("dn"));
    $results or ldapdie($ldap, "ldap_search()");

    $count = ldap_count_entries($ldap, $results);

    $entry = ldap_first_entry($ldap, $results);
    $fai_dn = array();
    $got = 0;
    while ($entry) {
        $dn = ldap_get_dn($ldap, $entry);
        if (! $dn)
            break;

        for ($i = - 1;; $i -= 1) {
            $common = substr($machine_dn, $i);
            if ($dn == "ou=fai,ou=configs,ou=systems,$common")
                break;
            if ($common != substr($dn, $i))
                goto next_entry;
        }

        for ($i = 0; $i < count($fai_dn); $i ++) {
            if (strlen($dn) > strlen($fai_dn[$i]))
                break;
        }
        array_splice($fai_dn, $i, 0, array($dn));

        next_entry:
        $got ++;
        $entry = ldap_next_entry($ldap, $entry);
    }

    (count($fai_dn) > 0) or ldapdie($ldap, "Could not find ou=fai");

    ($got == $count) or ldapdie($ldap, "ldap_*_entry()");

    return $fai_dn;
}

/**
 * Takes a release like "tramp/5.0.0" and an array of ou=fai DNs like "ou=fai,ou=configs,ou=systems,..." and returns
 * the release hierarchy as an array like this
 * ["ou=5.0.0,ou=tramp,ou=fai,ou=configs,ou=systems,...", "ou=tramp,ou=fai,ou=configs,ou=systems,..." ].
 * If the $fai_dn array contains more than 1 entry, the returned array will first have the release hierarchy with
 * the first entry in $fai_dn as suffix and only after the complete release hierarchy (in the example tramp/5.0.0 followed
 * by tramp) has been listed with that suffix will be the release hierarchy with the next suffix from $fai_dn.
 */
function resolve_release($fai_dn, $release)
{
    $r = array();
    for ($i = count($fai_dn) - 1; $i >= 0; $i --) {
        $prev = $fai_dn[$i];
        foreach (explode("/", $release) as $component) {
            $prev = "ou=$component,$prev";
            array_unshift($r, $prev);
        }
    }
    return $r;
}

/**
 * Takes $release_hierarchy as returned by resolve_release() and a
 * prefix like "cn=MainProfile,ou=profiles" and returns the LDAP entry
 * for the FAI class with dn=prefix+","+release_dn that is effective
 * in the last release listed in release_hierarchy.
 * If no such FAI class
 * exists or the effective FAI class has a FAIstate including "delete",
 * then ldapdie() will be called if $mustexist, otherwise FALSE will be returned.
 * The returned LDAP object is an array with the following format:
 *
 * return_value["dn"] = DN of the FAI class
 * return_value["count"] = number of attributes
 * return_value[j] = NAME of the jth attribute
 * return_value["attribute"]["count"] = number of values for attribute
 * return_value["attribute"][k] = kth value of attribute
 */
function get_effective_faiclass($ldap, $release_hierarchy, $prefix, $mustexist = TRUE)
{
    foreach ($release_hierarchy as $release_dn) {
        @$result = ldap_read($ldap, "$prefix,$release_dn", "objectClass=*");
        if (! $result) {
            if (ldap_errno($ldap) == 0x20) // NO SUCH OBJECT
                continue;
            else
                ldapdie($ldap, "ldap_read()");
        }
        $entries = ldap_get_entries($ldap, $result);
        $entries or ldapdie($ldap, "ldap_get_entries()");
        if ($entries["count"] != 0) {
            if (array_key_exists("faistate", $entries[0]) and strpos($entries[0]["faistate"][0], "removed") !== /* important to use !== and not != here*/ FALSE) {
                if ($mustexist)
                    ldapdie($ldap, "$prefix,$release_dn does not exist or has 'removed' state.");
                else
                    return FALSE;
            }
            return $entries[0];
        }
    }
    if ($mustexist)
        ldapdie($ldap, "$prefix does not exist in any of [" . implode(", ", $release_hierarchy) . "]");
    else
        return FALSE;
}

function get_next_effective_faiclass($ldap, $release_hierarchy, $faiobject_dn)
{
    foreach ($release_hierarchy as $i => $release_dn) {
        $len = strlen($release_dn);
        if (substr($faiobject_dn, - $len) != $release_dn || $i + 1 >= count($release_hierarchy))
            continue;

        $prefix = substr($faiobject_dn, 0, strlen($faiobject_dn) - $len - 1);
        $release_dn = $release_hierarchy[$i + 1];

        @$result = ldap_read($ldap, "$prefix,$release_dn", "objectClass=*");
        if (! $result) {
            if (ldap_errno($ldap) == 0x20) // NO SUCH OBJECT
                continue;
            else
                ldapdie($ldap, "ldap_read()");
        }
        $entries = ldap_get_entries($ldap, $result);
        $entries or ldapdie($ldap, "ldap_get_entries()");
        if ($entries["count"] != 0) {
            if (array_key_exists("faistate", $entries[0]) and strpos($entries[0]["faistate"][0], "removed") !== /* important to use !== and not != here*/ FALSE)
                return FALSE;

            return $entries[0];
        }
    }
    return FALSE;
}

$machine_dn = $machine["dn"];
$hostname = strtolower(preg_replace('/([^.]+).*/', '\1', $machine['cn'][0]));

$repopaths = array();
$faiclasses = array();
$release = "";
unset($machine["faiclass"]["count"]);
foreach ($machine["faiclass"] as $faiclasslist) {
    foreach (explode(" ", $faiclasslist) as $faiclass)
        if ($faiclass != "")
            if ($faiclass[0] == ':') {
                $release = substr($faiclass, 1);
                $repopaths[] = $release;
            } elseif ($faiclass[0] == '+')
                $repopaths[] = substr($faiclass, 1);
            else
                $faiclasses[] = $faiclass;
}

$fai_dn = find_fai_ou($ldap, $ldap_base_top, $machine_dn);
$release_hierarchy = resolve_release($fai_dn, $release);

// Resolve profiles
$fc_new = array();
for ($i = count($faiclasses) - 1; $i >= 0; $i --) {
    $cls = $faiclasses[$i];
    $profile = get_effective_faiclass($ldap, $release_hierarchy, "cn=$cls,ou=profiles", FALSE);
    if ($profile) {
        $new_classes = explode(" ", $profile["faiclass"][0]);
        for ($k = count($new_classes) - 1; $k >= 0; $k --) {
            $faiclass = $new_classes[$k];
            if ($faiclass != "" && ! in_array($faiclass, $fc_new))
                array_unshift($fc_new, $faiclass);
        }
    } else {
        if (! in_array($cls, $fc_new))
            array_unshift($fc_new, $cls);
    }
}

// $faiclasses is now the list of FAI classes with profiles resolved to their classes
$faiclasses = $fc_new;

// Build the config space in memory. Maps file path to file contents.
$config_space = array(); // maps relative file path to string contents of file
$config_space["deb"] = array(); // special key that will be filled with arrays that start with the release followed by sections

$config_space["class/$hostname"] = implode(" ", $faiclasses);

array_unshift($faiclasses, "DEFAULT"); // always the first FAI class
$faiclasses[] = "$hostname"; // always the next-to-last FAI class
$faiclasses[] = "LAST"; // always the last FAI class

foreach ($faiclasses as $cls) {
    foreach (array("scripts","hooks","templates","variables","packages","disk") as $type) {
        $skiplist = array();
        $faiobject = get_effective_faiclass($ldap, $release_hierarchy, "cn=$cls,ou=$type", FALSE);
        $children_only = FALSE;
        while ($faiobject) {
            handle($ldap, $config_space, $faiobject, $skiplist, $children_only);
            $faiobject = get_next_effective_faiclass($ldap, $release_hierarchy, $faiobject["dn"]);
            $children_only = TRUE;
        }
    }
}

function handle($ldap, &$config_space, &$faiobject, &$skiplist, $children_only)
{
    $dn = $faiobject['dn'];
    if (! isset($faiobject['..'])) {
        $faiobject['..'] = array('dn' => substr($dn, strpos($dn, ',') + 1),'count' => 0); // dummy parent reference
    }

    if (! $children_only) {
        foreach (array("FAIpartitionEntry","FAIpartitionDisk","FAIpartitionTable","FAIpackageList","FAIdebconfInfo","FAIscriptEntry","FAIscript","FAItemplateEntry","FAItemplate","FAIvariableEntry","FAIvariable","FAIhookEntry","FAIhook") as $oc) {
            if (in_array($oc, $faiobject['objectclass'])) {
                call_user_func_array($oc, array(&$config_space,&$faiobject));
                break;
            }
        }
    }
    $result = ldap_list($ldap, $dn, 'objectClass=*');
    $result or ldapdie($ldap, "ldap_list()");
    $entries = ldap_get_entries($ldap, $result);
    $entries or ldapdie($ldap, "ldap_get_entries()");
    for ($i = 0; $i < $entries["count"]; $i ++) {
        $child = $entries[$i];
        $rdn = substr($child["dn"], 0, strpos($child["dn"], ","));
        if (isset($skiplist[$rdn]))
            continue;
        $skiplist[$rdn] = TRUE;
        if (array_key_exists("faistate", $child) and strpos($child["faistate"][0], "removed") !== /* important to use !== and not != here*/ FALSE)
            continue;
        $child['..'] = $faiobject;
        handle($ldap, $config_space, $child, $skiplist, FALSE);
    }
}

function delTree($dirPath)
{
    if (! is_dir($dirPath)) {
        throw new InvalidArgumentException("$dirPath must be a directory");
    }
    if (substr($dirPath, strlen($dirPath) - 1, 1) != '/') {
        $dirPath .= '/';
    }
    $files = glob($dirPath . '*', GLOB_MARK);
    foreach ($files as $file) {
        if (is_dir($file)) {
            delTree($file);
        } else {
            unlink($file);
        }
    }
    rmdir($dirPath);
}

// Generate sources.list from the data collected in FAIPackageList().
//
// Turns out this doesn't work because sections written into package lists as well as releases
// don't mean anything. GOsa would have to be fixed to maintain the respective attributes properly.
//
// $release2section2bool = array();
// foreach ($config_space["deb"] as $debline) {
// $section2bool = isset($release2section2bool[$debline[0]]) ? $release2section2bool[$debline[0]] : array();
// //$rel = $debline[0];
// $rel = $release; // the release extracted from the FAI class means nothing because it may be an inherited FAI class whose release is the parent release
// unset($debline[0]);
// foreach ($debline as $section) {
// $section2bool[$section] = TRUE;
// }
// $release2section2bool[$rel] = $section2bool;
// }

// $sourceslist = "files/etc/apt/sources.list/LAST";
// $config_space[$sourceslist] = "";
// $mirror = $machine["faidebianmirror"][0];
// if ($mirror == "auto") {
// // not implemented
// } else {
// foreach ($release2section2bool as $rel => $section2bool) {
// $wellknown = "";
// // add well known sections in canonical order
// foreach (array("main","contrib","non-free","restricted","universe","multiverse","lhm") as $wks) {
// if (isset($section2bool[$wks])) {
// unset($section2bool[$wks]);
// $wellknown = "$wellknown$wks ";
// }
// }
// $config_space[$sourceslist] .= "deb $mirror $rel $wellknown" . implode(" ", array_keys($section2bool)) . "\n";
// }
// }

unset($config_space["deb"]);

// Generate sources.list

// we specify multiple object classes to make it more likely we hit one that is indexed.
$repo_server = search($ldap, $ldap_base_top, "(&(objectClass=GOhard)(objectClass=goServer)(objectClass=FAIrepositoryServer)(fairepository=*))", array("fairepository"), 0);
$repopath2server2sections = array();
for ($i = $repo_server['count']; $i > 0; $i --) {
    $rs = $repo_server[$i - 1];
    unset($rs["fairepository"]["count"]);
    foreach ($rs["fairepository"] as $repoline) {
        $repoparts = explode("|", $repoline);
        if (count($repoparts) != 4)
            continue;
        $repopath = $repoparts[2];
        if (!isset($repopath2server2sections[$repopath]))
          $repopath2server2sections[$repopath] = array();
        $repopath2server2sections[$repopath][rtrim($repoparts[0], '/')] = explode(",", $repoparts[3]);
    }
}



$sourceslist = "files/etc/apt/sources.list/LAST";
$config_space[$sourceslist] = "";

foreach ($repopaths as $repopath) {
    if (!isset($repopath2server2sections[$repopath]))
        ldapdie($ldap, "No repository server in LDAP has a '$repopath' repository.");
    
    // First try to find a repository with $repopath in the machine's
    // FAIdebianMirror attributes
    for ($i = $machine["faidebianmirror"]['count']; $i > 0; $i--) {
      $mirror = rtrim($machine["faidebianmirror"][$i - 1], '/');
      if (isset($repopath2server2sections[$repopath][$mirror]))
        goto found_mirror;  
    }
    
    // None of the machine's FAIdebianMirrors has $repopath.
    // Look at all known repository servers and try to find a good one.
    
    $servers = array_keys($repopath2server2sections[$repopath]);
    shuffle($servers);
    foreach ($servers as $s) {
        // Find a server that is reachable and can send Release in less than 1.0s.
        $opts = array('http' => array('timeout' => 1.0));
        $context = stream_context_create($opts);
        if (@file_get_contents("$s/dists/$repopath/Release", FALSE, $context) !== FALSE) {
            $mirror = $s;
            goto found_mirror;
        }
    }
    
    // So we didn't find a good server. Just pick the first and pray it works.
    $mirror = $servers[0];
    
found_mirror:
    $config_space[$sourceslist] .= "deb $mirror $repopath " . implode(" ", $repopath2server2sections[$repopath][$mirror]) . "\n";
}

ldap_close($ldap);

$config_space["class/release.var"] = "LHMclientRelease='$release'\n";
$config_space["class/$hostname.var"] = "LHMclientRelease='$release'\n";

// Fix up disk_configs and sort debconf
$paths = array_keys($config_space);
foreach ($paths as $path) {
    if (strpos($path, "debconf/") === 0) {
        $tmp = explode("\n", rtrim($config_space[$path]));
        sort($tmp);
        $config_space[$path] = implode("\n", $tmp) . "\n";
    } elseif (strpos($path, "disk_config/") === 0) {

        $i = strpos($path, ":");
        $partmap = array();
        if ($i !== FALSE) {
            $lines = explode("\n", $config_space[$path]);
            $contents = "";
            $primary = 0;
            $logical = 4;
            $idx = 0;
            foreach ($lines as $line) {
                if (strpos($line, "logical") === 0) {
                    $logical ++;
                    $idx ++;
                    $partmap[$idx] = $logical;
                }
                if (strpos($line, "primary") === 0) {
                    $primary ++;
                    $idx ++;
                    $partmap[$idx] = $primary;
                }

                if ($line != "")
                    $contents .= $line . "\n";
            }

            foreach ($partmap as $before => $after) {
                $contents = str_replace("<$before>", $after, $contents);
            }

            // add back an empty line at the end to make our output identical to that of the old Perl script
            $contents .= "\n";

            $path2 = substr($path, 0, $i);
            unset($config_space[$path]);
            $path = $path2;
        }
        // insert an empty line if we're appending to an existing file
        $config_space[$path] = (isset($config_space[$path]) ? $config_space[$path] . "\n$contents" : "$contents");
    }
}

// Create temporary directory for building config space
$tmp = sprintf("/tmp/ldap2fai-%d-%d", getmypid(), rand());
while (mkdir($tmp, 0700) === FALSE) {
    $tmp = sprintf("/tmp/ldap2fai-%d-%d", getmypid(), rand());
}

$basedir = $tmp;

if (! isset($_SERVER['REMOTE_ADDR'])) {
    // script execution for testing, use fixed directory name

    $basedir = "/tmp/ldap2fai";
    @mkdir($basedir); // because deltree complains if it doesn't exist
    deltree($basedir);
    mkdir($basedir);
}

// The following is pure PHP tarball creation code using PharData.
// It has turned out to be terribly slow, taking seconds to build
// the archive whereas calling the external tar program takes only
// tenths of seconds.
//
// $p = new PharData("$tmp/config.tar");
// foreach ($config_space as $path => $contents) {
// $p->addFromString($path, $contents);
// if (strpos($path, "scripts/") === 0) {
// $p["$path"]->chmod(0755);
// }
// }

// $p->compress(Phar::GZ, ".tar.gz");
// $tarball = "$tmp/config.tar.gz";
// unset($p);

// Write config space to $basedir
umask(0022);
mkdir("$basedir/class", 0755, TRUE);
mkdir("$basedir/debconf", 0755, TRUE);
mkdir("$basedir/disk_config", 0755, TRUE);
mkdir("$basedir/files", 0755, TRUE);
mkdir("$basedir/hooks", 0755, TRUE);
mkdir("$basedir/package_config", 0755, TRUE);
mkdir("$basedir/scripts", 0755, TRUE);
foreach ($config_space as $path => $contents) {
    @mkdir(dirname("$basedir/$path"), 0755, TRUE);
    $fh = fopen("$basedir/$path", "a");
    fwrite($fh, $contents);
    fclose($fh);
    if (strpos($path, "scripts/") === 0) {
        chmod("$basedir/$path", 0755);
    }
}

if (isset($_SERVER['REMOTE_ADDR'])) {
    header('Content-Description: File Transfer');
    header('Content-Type: application/octet-stream');
    header('Content-Disposition: attachment; filename=config.tar.gz');
    header('Expires: 0');
    header('Cache-Control: must-revalidate');
    header('Pragma: public');
    passthru("cd $basedir && tar --owner=0 --group=0 -czf - .");
}

// delete temporary directory
delTree($tmp);
?>
