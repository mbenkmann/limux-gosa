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

// NULL => use default (from ldap.conf)
$ldap_server = NULL;
$ldap_port = NULL;
$ldap_base_top = NULL;

function extract_mac($str) {
  $matches = array();
  if (preg_match('/^(01-)?(([0-9a-fA-F][0-9a-fA-F])[:-]([0-9a-fA-F][0-9a-fA-F])[:-]([0-9a-fA-F][0-9a-fA-F])[:-]([0-9a-fA-F][0-9a-fA-F])[:-]([0-9a-fA-F][0-9a-fA-F])[:-]([0-9a-fA-F][0-9a-fA-F]))([.][.a-zA-Z0-9_]+)?$/', $str, $matches)) {
    return str_replace("-",":",$matches[2]);
  }
  return "";
}

$macaddress = "";

if (isset($_SERVER['REQUEST_URI'])) {
  $str = $_SERVER['REQUEST_URI'];
  $i = strrpos($str, "=");
  $j = strrpos($str, "/");
  $k = strrpos($str, "?");
  if ($i === FALSE || ($j !== FALSE && $i < $j)) { $i = $j; }
  if ($i === FALSE || ($k !== FALSE && $i < $k)) { $i = $k; }
  if ($i !== FALSE) {
    $macaddress = extract_mac(substr($str, $i+1));
  }
}

if ($macaddress == "" && isset($argv[1])) {
  $macaddress = extract_mac($argv[1]);
}

if ($macaddress == "" && getenv('macaddress') !== FALSE) {
  $macaddress = extract_mac(getenv('macaddress'));
}

if ($macaddress != "") {
  $findclient = "(&(objectClass=GOhard)(macAddress=$macaddress))";
} else {
  ldapdie(FALSE, "Could not extract MAC address from request");
}

function aget(&$obj, $attname, $default = "")
{
    return isset($obj[$attname]) ? $obj[$attname][0] : $default;
}

function search($ldap, $base, $filter, $attrs, $minresults = 1, $maxresults = 999999)
{
    $results = ldap_search($ldap, $base, $filter, $attrs);
    $results or ldapdie($ldap, "ldap_search()");
    $entries = ldap_get_entries($ldap, $results);
    $entries or ldapdie($ldap, "ldap_get_entries()");
    $entries["count"] <= $maxresults or ldapdie($ldap, "More than $maxresults object(s) match \"$filter\"");
    $entries["count"] >= $minresults or ldapdie($ldap, "Could not find $minresults object(s) matching \"$filter\"");
    return $entries;
}

function searchonelevel($ldap, $base, $filter, $attrs, $minresults = 1, $maxresults = 999999)
{
    $results = ldap_list($ldap, $base, $filter, $attrs);
    $results or ldapdie($ldap, "ldap_list()");
    $entries = ldap_get_entries($ldap, $results);
    $entries or ldapdie($ldap, "ldap_get_entries()");
    $entries["count"] <= $maxresults or ldapdie($ldap, "More than $maxresults object(s) match \"$filter\"");
    $entries["count"] >= $minresults or ldapdie($ldap, "Could not find $minresults object(s) matching \"$filter\"");
    return $entries;
}

/**
 * function ldap_escape (http://stackoverflow.com/questions/8560874/php-ldap-add-function-to-escape-ldap-special-characters-in-dn-syntax)
 *
 * @author Chris Wright
 * @version 2.0
 * @param string $subject
 *            The subject string
 * @param bool $dn
 *            Escape for use in a DN if TRUE; escape for use in a filter if FALSE or omitted.
 * @param string|array $ignore
 *            Set of characters to leave untouched (useful to pass '*' on to a filter)
 * @return string The escaped string
 */
function ldap_escape($subject, $dn = FALSE, $ignore = NULL)
{

    // The base array of characters to escape
    // Flip to keys for easy use of unset()
    $search = array_flip($dn ? array('\\',',','=','+','<','>',';','"','#') : array('\\','*','(',')',"\x00"));

    // Process characters to ignore
    if (is_array($ignore)) {
        $ignore = array_values($ignore);
    }
    for ($char = 0; isset($ignore[$char]); $char ++) {
        unset($search[$ignore[$char]]);
    }

    // Flip $search back to values and build $replace array
    $search = array_keys($search);
    $replace = array();
    foreach ($search as $char) {
        $replace[] = sprintf('\\%02x', ord($char));
    }

    // Do the main replacement
    $result = str_replace($search, $replace, $subject);

    // Encode leading/trailing spaces in DN values
    if ($dn) {
        if ($result[0] == ' ') {
            $result = '\\20' . substr($result, 1);
        }
        if ($result[strlen($result) - 1] == ' ') {
            $result = substr($result, 0, - 1) . '\\20';
        }
    }

    return $result;
}

if (isset($ldap_server)) {
    $ldap = ldap_connect($ldap_server, $ldap_port);
} else { // use defaults from ldap.conf
    $ldap = ldap_connect();
}
$ldap or ldapdie(FALSE, "ldap_connect() failed\n");

ldap_set_option($ldap, LDAP_OPT_PROTOCOL_VERSION, 3);
ldap_set_option($ldap, LDAP_OPT_REFERRALS, 1);
ldap_bind($ldap) or ldapdie($ldap, "ldap_bind()");

// Find the machine's LDAP entry. There has to be exactly 1 entry, or we error out.
$machine = search($ldap, $ldap_base_top, $findclient, array_keys($required), 0, 1);
if ($machine['count'] == 0) {
    no_ldap_entry_for_machine($ldap);
    $machine = array();
} else {
    $machine = $machine[0];
    $machine_dn = $machine['dn'];

    // Check attributes and get missing ones from object groups
    $ogroups = NULL;
    foreach ($required as $attrname => $must) {
        if (! isset($machine[$attrname])) {
            if (! isset($ogroups)) {
                $member = ldap_escape($machine_dn);
                $ogroups = search($ldap, $ldap_base_top, "(&(objectClass=gosaGroupOfNames)(member=$member))", array_keys($required), 0);
            }
            for ($i = 0; $i < $ogroups['count']; $i ++) {
                if (isset($ogroups[$i][$attrname])) {
                    $machine[$attrname] = $ogroups[$i][$attrname];
                    break; // If there are more object groups that provide the same attribute => Who cares. That's an error.
                }
            }
            if ($must && ! isset($machine[$attrname])) {
                ldapdie($ldap, "$machine_dn has no attribute $attrname in its object or object groups it is a member of");
            }
        }
    }
}
?>
