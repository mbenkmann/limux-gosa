<?php

class macrotabs extends tabs
{

    function macrotabs($config, $data, $dn,$category)
    {
        tabs::tabs($config, $data, $dn,$category);

        /* Add references/acls/snapshots */
        $this->addSpecialTabs();
    }

    function save($ignore_account= FALSE)
    {
        $baseobject= $this->by_object['macro'];

        /* Check for new 'dn', in order to propagate the
           'dn' to all plugins */
        $cn      = preg_replace('/,/', '\,', $baseobject->cn);
        $cn      = preg_replace('/"/', '\"', $cn);
        $new_dn= @LDAP::convert("cn=".$cn.",".get_ou("macro", "phoneMacroRDN").$baseobject->base);

        /* Move group? */
        if ($this->dn != $new_dn){

            /* Write entry on new 'dn' */
            if ($this->dn != "new"){
                $baseobject->move($this->dn, $new_dn);
                $this->by_object['macro']= $baseobject;
            }

            /* Happen to use the new one */
            $this->dn= $new_dn;
        }

        tabs::save();
    }

}

?>
