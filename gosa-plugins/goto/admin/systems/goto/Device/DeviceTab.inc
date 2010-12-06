<?php

class DeviceTab extends tabs
{
    function save($ignore_account= FALSE)
    {
        // Prepare object DN
        $baseobject= $this->by_object['Device'];
        $uuid = $baseobject->deviceUUID;
        $orig_uuid = $baseobject->orig_deviceUUID;


        $rdn = get_ou('Device','DeviceRDN');
        $cn      = preg_replace('/,/', '\,', $baseobject->cn);
        $cn      = preg_replace('/"/', '\"', $cn);

        $dnCn = "cn=".$cn.",".$rdn.$baseobject->base;
        $dnUuid= "deviceUUID=".$uuid.",".$rdn.$baseobject->base;

        // Save object using the 'cn' in the dn and then move it to 'uuid';
        $moveToUUID = FALSE;
        if(!empty($uuid) && empty($orig_uuid) && $baseobject->orig_dn != "new"){
            $moveToUUID = TRUE;
        }

        // Detect target dn
        $this->dn = (empty($uuid)) ? $dnCn : $dnUuid;

        if($moveToUUID) $this->dn = $dnCn;

        // Populate values 
        foreach ($this->by_object as $key => $obj){
            $this->by_object[$key]->dn= $this->dn;
            $this->by_object[$key]->cn= $baseobject->cn;
        }

        // Object moved?
        if($this->dn != $baseobject->orig_dn && $baseobject->orig_dn != "new"){
            $baseobject->update_acls($baseobject->orig_dn,$this->dn);
            $baseobject->move($baseobject->orig_dn,$this->dn);
            $baseobject->orig_dn = $this->dn;
        }

        // Save now
        tabs::save();

        if($moveToUUID){
            $this->dn = (empty($uuid)) ? $dnCn : $dnUuid;
            $baseobject->update_acls($baseobject->orig_dn,$this->dn);
            $baseobject->move($baseobject->orig_dn,$this->dn);
        }

    }
}

?>
