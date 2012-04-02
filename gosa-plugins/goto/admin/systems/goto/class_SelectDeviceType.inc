<?php

class SelectDeviceType extends plugin
{
    /* attribute list for save action */
    var $ignore_account= TRUE;
    var $attributes= array("ObjectGroup","SystemType");
    var $objectclasses= array("whatever");

    var $ObjectGroups   = array();
    var $SystemTypes       = array();

    var $ObjectGroup    = "none";
    var $SystemType        = "gotoWorkstation";

    var $dn;

    function SelectDeviceType  (&$config, $dn= NULL)
    {
        if(!is_array($dn)){
            $this->dns = array($dn);
        }else{
            $this->dns = $dn;
        }
        plugin::plugin ($config, NULL);

        $this->SystemTypes =array("gotoWorkstation"=>_("Workstation"), "gotoTerminal"=>_("Terminal"), "goServer"=>_("Server"));
        if(class_available("opsi")){
            $this->SystemTypes["FAKE_OC_OpsiHost"]= _("Windows workstation");
        }
    }

    function get_object_groups($type) {
      $map = array("gotoWorkstation" => "W","gotoTerminal" => "T","goServer" => "S", "FAKE_OC_OpsiHost" => "O");

      $ldap = $this->config->get_ldap_link();
      $ldap->cd ($this->config->current['BASE']);
      $ldap->search("(&(objectClass=gosaGroupOfNames)(cn=*))",array("gosaGroupObjects","cn"));
      $tmp= array("W" => array(),"T" => array(), "S" => array(),"O" => array());
      while($attrs = $ldap->fetch()){
        $tmp[preg_replace("/[\[\] ]/","",$attrs['gosaGroupObjects'][0])][$attrs['dn']] = $attrs['cn'][0];
      }

      if (isset($tmp[$map[$type]])) {
        return $tmp[$map[$type]];
      } else {
        return array();
      }
    }

    /* Prepare smarty object for this class */
    function prepare_execute() {
      $smarty = get_smarty();
      $smarty->assign("dns_cnt" ,        set_post(count($this->dns)));
      $ogroups = $this->get_object_groups($this->SystemType);
      $smarty->assign("ogroups",         set_post($ogroups));
      $smarty->assign("OgroupKeys",      set_post(array_flip($ogroups)));
      $smarty->assign("ObjectGroup",      set_post($this->ObjectGroup));
      $smarty->assign("SystemTypes"     ,set_post($this->SystemTypes));
      $smarty->assign("SystemTypeKeys"  ,set_post(array_flip($this->SystemTypes))); 
      $smarty->assign("ObjectGroup",     set_post($this->ObjectGroup));
      $smarty->assign("SystemType",      set_post($this->SystemType));

      return $smarty;
    }
      


    function execute()
    {
        /* Call parent execute */
        plugin::execute();


        /* Get ogroups */
        $smarty= $this->prepare_execute();
        $display= $smarty->fetch(get_template_path('SelectDeviceType.tpl', TRUE,dirname(__FILE__)));
        return($display);
    }

    /* Save data to object */
    function save_object()
    {
        plugin::save_object();
        foreach($this->attributes as $attr){
            if(isset($_POST[$attr])){
                $this->$attr = get_post($attr);
            }
        }
    }

    /* Check supplied data */
    function check()
    {
        /* Call common method to give check the hook */
        $message= plugin::check();

        return ($message);
    }

    /* Save to LDAP */
    function save()
    {
    }

    function acl_is_writeable($attribute,$skip_write = FALSE)
    {
        if($this->read_only) return(FALSE);
        $ui= get_userinfo();
        return preg_match('/w/', $ui->get_permissions($this->acl_base, $this->acl_category."systemManagement", "", $skip_write));
    }

}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
