<?php

class filterDEVICES {

    static function query($base, $scope, $filter, $attributes, $category, $objectStorage= "")
    {
        $attributes[] = 'gotoHotplugDevice';
        $entries = filterLDAP::query($base, $scope, $filter, $attributes, $category, $objectStorage);

        foreach($entries as $key => $entry){
            if ($entry['gotoHotplugDevice']) {
                $parts = explode('|', $entry['gotoHotplugDevice'][0]);
                if (!empty($parts[0])) {
                    $description = $parts[0];
                    $entries[$key][$entries[$key]['count']]= 'description';
                    $entries[$key]['description'] = $description;
                    $entries[$key]['count'] ++;
                }
            
            }
        }
        
        return $entries;
    }
}

?>
