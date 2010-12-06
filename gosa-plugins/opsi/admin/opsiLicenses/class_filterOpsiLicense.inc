<?php

class filterOpsiLicense {

    static function query($base, $scope, $filter, $attributes, $category, $objectStorage= "")
    {

        if(!class_available('opsi')) return(array());

        $config= session::global_get('config');
        $ldap= $config->get_ldap_link(TRUE);
        $flag= ($scope == "sub")?GL_SUBSEARCH:0;
        $result= filterOpsiLicense::get_list($base, $filter, $attributes, $category, $objectStorage, $flag);

        // Prepare filter and split it into attribute and value to search for
        $filter=preg_replace("/\*/","",$filter);
        $attr = $value = "";
        if(!empty($filter) && preg_match("/=/", $filter)){
            list($attr,$value) = preg_split("/=/", $filter);
        }

        // Simple filtering 
        if(!empty($attr)){
            foreach($result as $key => $entry){
                if(!preg_match("/".$value."/i", $entry[$attr][0])){
                    unset($result[$key]);
                }
            }
        }

        return(filterACL::unifyResult($result));
    }

    static function get_list($base, $filter, $attributes, $category, $objectStorage, $flags= GL_SUBSEARCH)
    {
        $config= session::global_get('config');
        $si = new opsiLicenceHandler($config);

        if(!$si->enabled()) return(array());

        $res = $si->listPools();
        $result = array();
        if($si->is_error() || !is_array($res)){
            $this->init_successfull = FALSE;
            msg_dialog::display(_("Error"),msgPool::siError($si->get_error()),ERROR_DIALOG);
            return;
        }else{

            // Reset the list of licenses
            foreach($res as $item){

                $item['objectClass'] = array('fake_opsiLicense');

                // Fake an ldap entry, this enables ACL checks.
                $entry = array();
                $entry['dn'] = "opsi:cn=".$item['cn'][0].",".$config->current['BASE'];
                foreach($item as $name => $value){
                    $entry[] = $name;
                    $entry[$name] = $value;
                }
                $entry['count'] = count($item);
                $result[] = $entry;
            }
        }
        return($result);
    }

    static function unifyResult($result)
    {
        $res=array();
        foreach($result as $entry){
            if(!isset($res[$entry['dn']])){
                $res[$entry['dn']]=$entry;
            }
        }
        return(array_values($res)); 
    }
}

?>
