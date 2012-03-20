<?php

class filterINCOMING {

    static function query($base, $scope, $filter, $attributes, $category, $objectStorage= "")
    {
        $ui = get_userinfo();
        global $config;

        // Override base and scope to search for incoming objects
        $base = (get_ou("ArpNewDevice", "systemIncomingRDN") . $config->current['BASE']);
        $scope = "sub";
        $entries = filterSYSTEMS::query($base, $scope, $filter, $attributes,
           $category, $objectStorage);

        return($entries);
    }
}

?>
