<?php

class filterPrimaryGroupLDAP extends filterGroupLDAP {

    static function query($base, $scope, $filter, $attributes, $category, $objectStorage= "")
    {
        $attributes[] = 'gidNumber';
        $entries = parent::query($base, $scope, $filter, $attributes, $category, $objectStorage);
        
        $accounts = filterLDAP::get_list($base, '(&(objectClass=posixAccount)(gidNumber=*))' , array('gidNumber'), 'users', '', ($scope == "sub")?GL_SUBSEARCH:0);
        $extract_gid = function($account) { return $account['gidNumber'][0]; };
        $account_gids = array_map($extract_gid, $accounts);
                
        if (empty($account_gids)) {
          // array_combine in PHP < 5.4 errors out on empty arrays
          // and we can skip all following code if there are no matching groups
          return array();
        }        
        
        $account_gids_set = array_combine($account_gids, $account_gids);
        
        $is_primary_group = function ($entry) use ($account_gids_set) {
            $gid = $entry['gidNumber'][0];
            return isset($account_gids_set[$gid]);
        };

        return array_filter($entries, $is_primary_group);
    }


}

?>
