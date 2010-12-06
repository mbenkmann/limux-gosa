<?php

class askClassName extends plugin
{
  /* attribute list for save action */
  var $ignore_account     = TRUE;
  var $attributes         = array();
  var $objectclasses      = array("whatever");

  var $objectClass        = false;
  var $forbidden          = array();
  var $ClassName          = "";  
  var $ClassAlreadyExists = false;
  var $parent;

  function askClassName (&$config,$dn,$ui,$objectClass)
  {
    $this->ui = $ui;
    $this->objectClass = $objectClass;
    plugin::plugin ($config, $dn);
  }

  function execute()
  {
    /* Call parent execute */
    plugin::execute();

    /* Fill templating stuff */
    $smarty = get_smarty();
    $display= "";

    /* First search for every ou, in given fai base
     * With a second search detect all object that belong to the different ous. 
     */

    $base = $this->parent->fai_release;
    $ldap = $this->config->get_ldap_link();

    $type_acl_mapping = array(
        "FAIpartitionTable"  => "faiPartitionTable",
        "FAIpackageList"     => "faiPackage",
        "FAIscript"          => "faiScript",
        "FAIvariable"        => "faiVariable",
        "FAIhook"            => "faiHook",
        "FAIprofile"         => "faiProfile",
        "FAItemplate"        => "faiTemplate");

    $filter = "";
    foreach($type_acl_mapping as $map){
      $filter .= "(objectClass=".$map.")";
    }

    $res = FAI::get_all_objects_for_given_base($base,"(|".$filter.")",TRUE);

    $delete = array();
    $used   = array();
    foreach($res as $object){

      /* skip class names with this name */ 
      if(in_array($this->objectClass,$object['objectClass']) || in_array("FAIprofile",$object['objectClass'])){
        if(isset($object['FAIstate'][0]) && preg_match("/removed$/",$object['FAIstate'][0])){
          continue;
        }
        $delete[] = $object['cn'][0];
      }

      /* Skip profiles */
      if(!in_array("FAIprofile",$object['objectClass'])){
        if(isset($object['cn'])){
          $used[$object['cn'][0]]= $object['cn'][0];
        }
      }
    }

    /* Create headline
     * Depending on the object type we want to create, a different headline will be shown
     */ 
    switch($this->objectClass) {
      case "FAIpartitionTable":  $str =_("Create new FAI object")." - "._("Partition table");break;
      case "FAIpackageList"   :  $str =_("Create new FAI object")." - "._("Package bundle");break;
      case "FAIscript"        :  $str =_("Create new FAI object")." - "._("Script bundle");break;
      case "FAIvariable"      :  $str =_("Create new FAI object")." - "._("Variable bundle");break;
      case "FAIhook"          :  $str =_("Create new FAI object")." - "._("Hook bundle");break;
      case "FAIprofile"       :  $str =_("Create new FAI object")." - "._("Profile");break;
      case "FAItemplate"      :  $str =_("Create new FAI object")." - "._("Template bundle");break;
      default                 :  $str =_("Create new FAI object");break;
    }
    $smarty->assign("headline",$str);
    
    /* Save forbidden class names
     */
    $this->forbidden = $delete;

    /* Delete all class names which already have this type of object 
     */
    foreach($delete as $del){
      unset($used[$del]);
    }
   
    /* if there is no class name which is missing for this type 
     *  of objectClass, we only can create a new one, disable selectBox 
     */
    if(count ($used)==0){
      $smarty->assign("ClassNamesAvailable", " disabled ");
      $smarty->assign("grey", 'style="color:#C0C0C0"');
    }else{
      $smarty->assign("ClassNamesAvailable", "");
      $smarty->assign("grey", "");
    }
    ksort($used);
    $smarty->assign("ClassNames", $used);
    $smarty->assign("ClassName",  $this->ClassName);
    $display.= $smarty->fetch(get_template_path('askClassName.tpl', TRUE));
    return($display);
  }

  /* Get posts and set class name 
   */ 
  function save_object()
  {
    if(isset($_POST['classSelector']) && $_POST['classSelector'] == 1 
       && isset($_POST['edit_continue'])){
      $this->ClassName          = get_post('UseTextInputName');
      $this->ClassAlreadyExists = true;
    }
  
    if(isset($_POST['classSelector']) && $_POST['classSelector'] == 2 
       && isset($_POST['edit_continue'])){
      $this->ClassAlreadyExists = false;
      $this->ClassName          = get_post('SelectedClass');
    }
  }

  /* Check given class name 
   */
  function check()
  {
    /* Call common method to give check the hook */
    $message= plugin::check();

    if($this->objectClass == "FAIscript" && preg_match("/^-/", $this->ClassName)) { 
      $message[] = msgPool::invalid(_("Name"),preg_replace("/ /","_",$this->ClassName),"/[^_]/");
    }   

    if($this->ClassName != preg_replace("/ /","",trim($this->ClassName))){
      $message[] = msgPool::invalid(_("Name"),preg_replace("/ /","_",$this->ClassName),"/[^_]/");
    }

    if($this->ClassName == ""){
      $message[]= msgPool::required(_("Name"));
    }

    if(in_array($this->ClassName,$this->forbidden)){
      $message[]= msgPool::duplicated(_("Name"));
    }

    return ($message);
  }


  /* Return the class name */
  function save()
  {
    return($this->ClassName);
  }

}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
