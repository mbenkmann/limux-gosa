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
// YOU ALSO NEED TO CREATE A CONFIG FILE /etc/gosa/pxelinux.conf (see below)
//
// This script constructs a pxelinux.cfg from the data found in LDAP
// as written by GOsa. The config is based on the FAIstate and
// (in case FAIstate=install) the OS to be installed.
//
// If no MAC address is found in the request URI, the IP address of the
// requesting client will be used to find the LDAP object whose pxelinux.cfg
// will be constructed.
//
// To request the pxelinux.cfg of a particular machine this script can be
// called with a MAC-address in the request URI,
// either by appending a ?...<MACADDRESS> query string or by including the
// MAC address directly in the file name part of the URI, e.g.
// by aliasing the URL http://server/pxelinux.cfg/01-<MACADDRESS>
// to this script. See /etc/gosa/fai-helpers-apache.conf for an example.
// Everything works as long as the request URI ends in a
// MAC address using either ":" or "-" as separator.
//
// If no MAC address is found in the request URI, the IP address of the
// requesting client will be used to find the LDAP object.
//
// The format of /etc/gosa/pxelinux.cfg is demonstrated by the following
// example:
//
// # NOTE:
// # kernel and initrd will automatically be changed if gotoBootKernel is present and not "default".
// # gotoKernelParameters are automatically appended to the append line.
// # Default for installations when there is no OS-specific config
// [install]
// kernel vmlinuz-install
// initrd initrd.img-install
// append nfsroot=/nfsroot,nfs4,union FAI_ACTION=install FAI_FLAGS=syslogd,verbose,sshd,poweroff,skipusb ip=dhcp devfs=nomount root=/dev/nfs
// ipappend 2
//
// # Config if OS to be installed is "lucid"
// [lucid]
// kernel vmlinuz-install-lucid
// initrd initrd.img-install-lucid
// append nfsroot=/nfsroot,nfs4,union FAI_ACTION=install FAI_FLAGS=syslogd,verbose,sshd,poweroff,skipusb ip=dhcp devfs=nomount root=/dev/nfs
// ipappend 2
//
// # Configuration used if FAIstate=error:details...  pxelinux.php automatically appends FAI_ERROR=details as base64...
// # to the append line.
// [error]
// kernel vmlinuz-install
// initrd initrd.img-install
// append nfsroot=/nfsroot,nfs4,union FAI_ACTION=sysinfo FAI_FLAGS=syslogd,verbose,sshd,poweroff,skipusb ip=dhcp devfs=nomount root=/dev/nfs
// ipappend 2
//
// # Default if nothing else matches.
// [default]
// localboot 0



/*
 * Close LDAP connection, log error and return 500 Internal Server Error.
 */
function ldapdie($ldap, $msg)
{
    //http_response_code(500);
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
    // No error. We send the default config for a client install.
}

$required = array("macaddress" => TRUE,"faistate" => FALSE,"cn" => TRUE,"gotobootkernel" => FALSE,"faiclass" => FALSE,"gotokernelparameters" => FALSE);
require (dirname(__FILE__) . '/ldap2fai-pxelinuxcfg-common.php');

if (isset($machine["faistate"])) { // if we have a faistate in LDAP, use it
    $faistate = $machine["faistate"][0];
} else {
    if (isset($machine["dn"])) { // if we have an LDAP object
        if (strpos($machine["dn"], ",ou=incoming,") !== FALSE) { // if it's in incoming
            $faistate = "install";
        } else { // if it's not in incoming, it's not safe to install
            $faistate = "localboot";
        }
    } else { // if there is no LDAP object
        $faistate = "install";
    }
}

$gotobootkernel = "";
if (isset($machine["gotobootkernel"])) {
    $gotobootkernel = $machine["gotobootkernel"][0];
    if ($gotobootkernel == "default")
        $gotobootkernel = "";
}
$gotokernelparameters = "";
if (isset($machine["gotokernelparameters"])) {
    $gotokernelparameters = $machine["gotokernelparameters"][0];
}

$conf = file("/etc/gosa/pxelinux.conf", FILE_IGNORE_NEW_LINES | FILE_SKIP_EMPTY_LINES);
if ($conf === FALSE) {
    $conf = array();
}

for ($i = 0; $i < count($conf); $i ++) {
    $conf[$i] = trim($conf[$i]);
}

$conf[] = "[internal-default]";
$conf[] = "# No appropriate [section] found in /etc/gosa/pxelinux.conf => localboot";
$conf[] = "localboot 0";

if (strpos($faistate, "error:") === 0) {
    $sections = array("error","default","internal-default");
    $error = substr($faistate, 6);
} else {
    $error = "";
    if ($faistate == "install") {
        $release = "";
        if (isset($machine["faiclass"])) {
            $faiclass = $machine["faiclass"][0];
            $colon = strpos($faiclass, ":");
            if ($colon !== FALSE) {
                $release = trim(substr($faiclass, $colon + 1));
            }
        }
        if (! empty($release)) {
            $sections = array($release,"install","default","internal-default");
        } else {
            $sections = array("install","default","internal-default");
        }
    } else {
        $sections = array("default","internal-default");
    }
}

header('Content-type: text/plain');
print("default fai-generated\n\nlabel fai-generated\n");

foreach ($sections as $section) {
    for ($i = 0; $i < count($conf); $i ++) {
        if ($conf[$i] == "[$section]") {
            for ($i ++; $i < count($conf) && strpos($conf[$i], "[") !== 0; $i ++) {
                $line = $conf[$i];
                if (! empty($error)) {
                    if (strpos($line, "append") === 0) {
                        $line .= " FAI_ERROR=" . base64_encode($error);
                    }
                }
                if (! empty($gotobootkernel)) {
                    if (strpos($line, "kernel") === 0) {
                        $line = "kernel $gotobootkernel";
                    }
                    if (strpos($line, "initrd") === 0) {
                        $line = "initrd " . str_replace("vmlinuz", "initrd.img", $gotobootkernel);
                    }
                }
                if (! empty($gotokernelparameters)) {
                    if (strpos($line, "append") === 0) {
                        $line .= " $gotokernelparameters";
                    }
                }

                print($line);
                print("\n");
            }
            break 2;
        }
    }
}
?>
