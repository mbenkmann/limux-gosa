<?php

class filterMenuGroupLDAP extends filterGroupLDAP {

    static function query($base, $scope, $filter, $attributes, $category, $objectStorage= "")
    {
        $entries = parent::query($base, $scope, $filter, $attributes, $category, $objectStorage);

        $is_menu_group = function ($entry) {
            return in_array_strict('gotoMenuGroup', $entry['objectClass']);
        };

        return array_filter($entries, $is_menu_group);
    }


}

?>
