<?php
class filterSystemByMac  extends filterLDAP{

    static function query($base, $scope, $filter, $attributes, $category, $objectStorage= array(""))
    {

        $filterByMAC = preg_match("/__macAddr=([^_]+)__/", $filter, $matches);
        $mac = str_replace('*', '', $matches[1]);

        // Remove pseudo filter from real LDAP filter
        $filter = preg_replace("/__macAddr=([^_]+)__/", "", $filter);
        
        // Get list of entries
        $entries = filterLDAP::query($base, $scope, $filter, $attributes, $category, $objectStorage);

        // Filter entries by MAC
        if($filterByMAC and (!empty($mac))){
            foreach($entries as $key => $entry){ 
                if(!isset($entry['macAddress'])){
                    unset($entries[$key]);
                    continue;
                } elseif (isset($mac) && stristr($entry['macAddress'][0], $mac) != FALSE) {
                    continue;
                } else {
                    unset($entries[$key]);
                    continue;
                }
            }
        }
        
        return(array_values($entries));
    }
}
?>
