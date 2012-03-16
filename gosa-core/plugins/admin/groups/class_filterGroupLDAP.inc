<?php

class filterGroupLDAP {

    static function query($base, $scope, $filter, $attributes, $category, $objectStorage= "")
    {
        $config= session::global_get('config');
        $flag= ($scope == "sub")?GL_SUBSEARCH:0;
        $entries= filterGroupLDAP::get_list($base, $filter, $attributes, $category, $objectStorage, $flag | GL_SIZELIMIT);

        foreach ($entries as $id => $entry) {
            $ldap = $config->get_ldap_link();
            $ldap->cd($entry['dn']);
            $ldap->search("(|(objectClass=gotoMenuEntry)(objectClass=gotoSubmenuEntry))");
            if ($ldap->count()) {
                $entries[$id]["objectClass"][] = "gotoMenuGroup";
            }

        }

        return $entries;
    }


    static function get_list($base, $filter, $attributes, $category, $objectStorage, $flags= GL_SUBSEARCH)
    {
        return filterLDAP::get_list($base, $filter, $attributes, $category, $objectStorage, $flags);
    }


}

?>
