<?php

class filterSYSTEMS {

    static function query($base, $scope, $filter, $attributes, $category, $objectStorage= "")
    {
        $ui = get_userinfo();
        global $config;

        // Check whether the arp handling active or not
        $arp_handling_active = $config->pluginEnabled("ArpNewDevice");

        // Get object group membership and FAIclass inheritance info
        $array_ogroups = array();
        $ldap = $config->get_ldap_link();
        $ldap->cd($config->current['BASE']);
        $ldap->search("(&(objectClass=gosaGroupOfNames)(FAIclass=*)(member=*))",array('member','cn', 'FAIclass'));
        while ($attrs = $ldap->fetch()) {
            unset($attrs['member']['count']);
            foreach($attrs['member'] as $member) {
                $memberID = LDAP::convert($member);
                $array_ogroups[$memberID]['ogroup'] = $attrs['cn'][0];
                $array_ogroups[$memberID]['FAIclass'] = $attrs['FAIclass'][0];
            }
        }

        $entries = filterLDAP::query($base, $scope, $filter, $attributes, $category, $objectStorage);


        foreach($entries as $key => $entry){
            if(class_available("ArpNewDevice") &&
                preg_match("/".preg_quote(get_ou("ArpNewDevice", "systemIncomingRDN"),'/')."/i", $entry['dn'])){

                // Detect incoming types
                if(in_array_strict('gotoWorkstation', $entry['objectClass'])){
                    $entries[$key]['objectClass'][] = 'FAKE_OC_NewWorkstation';
                    $entries[$key]['cn'][0].= " <i>("._("New workstation").")</i>";
                }elseif(in_array_strict('gotoTerminal', $entry['objectClass'])){
                    $entries[$key]['objectClass'][] = 'FAKE_OC_NewTerminal';
                    $entries[$key]['cn'][0].= " <i>("._("New terminal").")</i>";
                }elseif($arp_handling_active && 
                        in_array_strict('GOhard', $entry['objectClass']) && 
                        !isset($entry['gotoMode'][0])){ 
                    $entries[$key]['objectClass'][] = 'FAKE_OC_ArpNewDevice';
                    $entries[$key]['cn'][0].= " <i>("._("New unknown device").")</i>";
                }elseif(in_array_strict('GOhard', $entry['objectClass'])){
                    $entries[$key]['objectClass'][] = 'FAKE_OC_NewDevice';
                    $entries[$key]['cn'][0].= " <i>("._("New device").")</i>";
                }

                // Detect SAMBA machines
            }elseif(in_array_strict('sambaSamAccount', $entry['objectClass']) && preg_match("/\$$/", $entry['cn'][0])){
                $entries[$key]['objectClass'][] = 'FAKE_OC_winstation';

                // Detect Workstation & templates
            }elseif(in_array_strict('gotoWorkstation', $entry['objectClass'])){
                $pwd_acl = $ui->get_permissions($entry['dn'],'workstation/workgeneric','userPassword');
                if(preg_match("/w/",$pwd_acl)){
                    $entries[$key]['objectClass'][] = 'FAKE_OC_PWD_changeAble';
                }

                // Detect Terminal & templates
            }elseif(in_array_strict('gotoTerminal', $entry['objectClass'])){
                $pwd_acl = $ui->get_permissions($entry['dn'],'terminal/termgeneric','userPassword');
                if(preg_match("/w/",$pwd_acl)){
                    $entries[$key]['objectClass'][] = 'FAKE_OC_PWD_changeAble';
                }

                // Detect Server & templates
            }elseif(in_array_strict('goServer', $entry['objectClass'])){
                $pwd_acl = $ui->get_permissions($entry['dn'],'server/servgeneric','userPassword');
                if(preg_match("/w/",$pwd_acl)){
                    $entries[$key]['objectClass'][] = 'FAKE_OC_PWD_changeAble';
                }

                // Detect ieee802Device
            }elseif(in_array_strict('ieee802Device', $entry['objectClass'])){
                $pwd_acl = $ui->get_permissions($entry['dn'],'component/componentGeneric','userPassword');
                if(preg_match("/w/",$pwd_acl)){
                    $entries[$key]['objectClass'][] = 'FAKE_OC_PWD_changeAble';
                }
            }

            // Detect object states
            $state = 'none';
            foreach(array('gotoWorkstation','gotoTerminal','goServer') as $type){
                if(in_array_strict($type, $entry['objectClass'])){

                    if(isset($entry['FAIstate'][0])){
                        switch (preg_replace('/:.*$/', '', $entry['FAIstate'][0])) {
                            case 'error':
                                $state= 'error';
                                break;
                            case 'installing':
                            case 'install':
                            case 'sysinfo':
                            case 'softupdate':
                            case 'scheduledupdate':
                                $state= 'busy';
                                break;
                        }
                    }
                    if(isset($entry['gotoMode'][0]) && preg_match("/locked/i", $entry['gotoMode'][0]) && $state != 'error') {
                        $state = "locked";
                    }
                }
            }

            if(isset($entries[$key])){    
                $entries[$key][$entries[$key]['count']]= 'state';
                $entries[$key]['state']= $state;
                $entries[$key]['count'] ++;
            }

            // Get object group membership and FAIclass inheritance info
            $dn = $entries[$key]['dn'];
            $ogroup = "";
            $class = "";

            if(array_key_exists($dn,$array_ogroups)){
                if (isset($array_ogroups[$dn]['FAIclass'])) {
                    $class = $array_ogroups[$dn]['FAIclass'];
                }
                if (isset($array_ogroups[$dn]['ogroup'])) {
                    $ogroup = $array_ogroups[$dn]['ogroup'];
                }
            }

            // Add ogroup and ogroup_FAIclass in any case, even if its empty
            // to avoid problems with filter functions which could otherwise
            // have more then one empty parameter
            $entries[$key][$entries[$key]['count']]= 'ogroup';
            $entries[$key]['ogroup'] = $ogroup;
            $entries[$key]['count'] ++;

            $entries[$key][$entries[$key]['count']]= 'ogroup_FAIclass';
            $entries[$key]['ogroup_FAIclass'] = $class;
            $entries[$key]['count'] ++;

        }

        return($entries);
    }
}

?>
