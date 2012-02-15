<?php
class filterSystemByIp  extends filterLDAP{

    static function query($base, $scope, $filter, $attributes, $category, $objectStorage= array(""))
    {

        // IP Range selection 
        $filterByIP = preg_match("/__ipAddr=([^_]+)__/", $filter, $matches);
        $ip_filter = str_replace('*', '', $matches[1]);

        // Remove pseudo filter from real LDAP filter
        $filter = preg_replace("/__ipAddr=([^_]+)__/", "", $filter);
        

        // Test if filtering by a range is requested
        if (strstr($ip_filter, "-") != FALSE) {
            list($ipstart, $ipstop) = explode('-', $ip_filter, 3);
            if (!tests::is_ip($ipstart)) {
                $ipstart = '0.0.0.0';
            }
            if (!tests::is_ip($ipstop)) {
                $ipstop = '255.255.255.255';
            }
        } else {
            $ip = $ip_filter;
        }
        
        // Get list of entries
        $entries = filterLDAP::query($base, $scope, $filter, $attributes, $category, $objectStorage);

        // Filter entries by IP-Address
        if($filterByIP && !empty($ip_filter)){
            foreach($entries as $key => $entry){ 
                if(!isset($entry['ipHostNumber'])){
                    unset($entries[$key]);
                    continue;
                } elseif (isset($ipstart) && isset($ipstop) && tests::is_in_ip_range($ipstart,$ipstop, $entry['ipHostNumber'][0])){
                    continue;
                } elseif (isset($ip) && strstr($entry['ipHostNumber'][0], $ip) != FALSE) {
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
