<?php

class faiVariable extends plugin
{
  /* attribute list for save action */
  var $ignore_account   = TRUE;

  /* Attributes for this Object */
  var $attributes       = array("cn","description");

  /* ObjectClasses for this Object*/
  var $objectclasses    = array("top","FAIclass","FAIvariable");

  /* Class name of the Ldap ObjectClass for the Sub Object */
  var $subClass         = "FAIvariableEntry";
  var $subClasses       = array("top","FAIclass","FAIvariableEntry");

  /* Class name of the php class which allows us to edit a Sub Object */
  var $subClassName     = "faiVariableEntry";      

  /* Attributes to initialise for each subObject */
  var $subAttributes    = array("cn","description","FAIvariableContent"); 
  var $sub64coded       = array();  
  var $subBinary        = array();

  /* Specific attributes */
  var $cn               = "";       // The class name for this object
  var $description      = "";       // The description for this set of partitions
  var $is_dialog        = false;    // specifies which buttons will be shown to save or abort
  var $SubObjects       = array();  // All leafobjects of this object

  var $FAIstate         = "";
  var $ui   ;
  var $view_logged      = FALSE;

  function faiVariable (&$config, $dn= NULL)
  {
    /* Load Attributes */
    plugin::plugin ($config, $dn);

    if($dn != "new"){
      $this->dn =$dn;

      /* Read all leaf objects of this object (For FAIscript this would be FAIscriptEntry)
       */
      $res = FAI::get_all_objects_for_given_object($this->dn,"(&(objectClass=FAIclass)(objectClass=".$this->subClass."))");
      foreach($res as $obj){
        $objects = array();
        $objects['description'] = "";
        $objects['status']      = "FreshLoaded";
        $objects['dn']          = $obj['dn'];
        $objects                = $this->get_object_attributes($objects,$this->subAttributes);
        $this->SubObjects[$objects['cn']] = $objects;
      }
    }
    $this->is_new = FALSE;
    if($this->dn == "new"){
      $this->is_new =TRUE;
    }
    $this->ui = get_userinfo();
  }


  function acl_base_for_current_object($dn)
  {
    if($dn == "new" || $dn == ""){
      if($this->dn == "new"){
        $dn= $this->parent->parent->acl_base;
      }else{
        $dn = $this->dn;
      }
    }
    return($dn);
  }


  function execute()
  {
    /* Call parent execute */
    plugin::execute();

    if($this->is_account && !$this->view_logged){
      $this->view_logged = TRUE;
      new log("view","fai/".get_class($this),$this->dn);
    }

    /* Fill templating stuff */
    $smarty= get_smarty();
    $display= "";

    /* Add new sub object */
    if(isset($_POST['AddSubObject']) && !preg_match("/freeze/i",$this->FAIstate)){
      $this->dialog= new $this->subClassName($this->config,"new");
      $this->dialog->set_acl_base($this->acl_base_for_current_object($this->dn));
      $this->dialog->set_acl_category("fai");
      $this->dialog->parent = &$this;
      $this->dialog->FAIstate = $this->FAIstate;
      $this->is_dialog=true;
    }

    if($this->dn != "new"){
      set_object_info($this->dn);
    }


    /* Edit selected Sub Object */
    if((isset($_POST['EditSubObject']))&&(isset($_POST['SubObject']))){

      $var = $_POST['SubObject'][0];
      $c_dn = $this->acl_base_for_current_object($this->SubObjects[$var]['dn']);
      $this->dialog= new $this->subClassName($this->config,$this->dn,$this->SubObjects[$var]);
      $this->dialog->set_acl_category("fai");
      $this->dialog->set_acl_base($c_dn);
      $this->dialog->FAIstate = $this->FAIstate;
      $this->dialog->parent = &$this;
      set_object_info($this->SubObjects[$var]['dn']);
      $this->is_dialog=true;
    }
    
    /* Remove Sub object */
    if((isset($_POST['DelSubObject']))&&(isset($_POST['SubObject'])) && !preg_match("/freeze/i",$this->FAIstate)){
      foreach($_POST['SubObject'] as $var){

        $c_dn = $this->acl_base_for_current_object($this->SubObjects[$var]['dn']);
        $acl = $this->ui->get_permissions($c_dn,"fai/faiVariable");
        if(preg_match("/d/",$acl)){
          if($this->SubObjects[$var]['status'] != "new"){
            $this->SubObjects[$var]['status']= "delete";
          }else{
            unset($this->SubObjects[$var]);
          }
        }
      }
    }

    /* Save Dialog */
    if(isset($_POST['SaveSubObject']) && !preg_match("/freeze/i",$this->FAIstate)){
      $this->dialog->save_object();
      $msgs = $this->dialog->check();
      if(count($msgs)>0){
        foreach($msgs as $msg){
          msg_dialog::display(_("Error"), $msg, ERROR_DIALOG);
        }
      }else{
        $obj = $this->dialog->save();
        if(isset($obj['remove'])){
          if($this->SubObjects[$obj['remove']['from']]['status'] != "new"){
            $this->SubObjects[$obj['remove']['from']]['status'] = "delete";
          }else{
            unset($this->SubObjects[$obj['remove']['from']]);
          }
          $obj['status'] = "new";
          $this->SubObjects[$obj['remove']['to']] = $obj;
          unset($this->SubObjects[$obj['remove']['to']]['remove']);
        }else{
          $obj['status'] = "edited";
          $this->SubObjects[$obj['cn']]=$obj;
        }
        $this->is_dialog=false;
        unset($this->dialog);
        $this->dialog=FALSE;
      }
    }

    /* Sort entries */
    $tmp = $keys = array();
    foreach($this->SubObjects as $key => $entry){
      $keys[$key]=$key;
    }
    natcasesort($keys);
    foreach($keys as $key){
      $tmp[$key]=$this->SubObjects[$key];
    } 
    $this->SubObjects = $tmp;

    /* Cancel Dialog */
    if(isset($_POST['CancelSubObject'])){
      $this->is_dialog=false; 
      unset($this->dialog);
      $this->dialog=FALSE;
    }

    /* Print dialog if $this->dialog is set */
    if(is_object($this->dialog)){
      $this->dialog->save_object();
      $display = $this->dialog->execute();
      return($display);
    }

    $ui = get_userinfo();
    $ret = $this->getList();
    $tmp = array();
    foreach($this->SubObjects as $key => $obj){
      $acl = $ui->get_permissions($obj['dn'],"fai/faiVariableEntry");
      if((preg_match("/r/",$acl) || $obj['dn'] == "new") && isset($ret[$key])){
        $tmp[$key] = $ret[$key];
      } 
    }
    $smarty->assign("SubObjects",$tmp);


    /* Magic quotes GPC, escapes every ' " \, to solve some security risks
     * If we post the escaped strings they will be escaped again
     */
    foreach($this->attributes as $attrs){
        $smarty->assign($attrs,set_post($this->$attrs));
    }

    $c_dn = $this->acl_base_for_current_object($this->dn);
    $smarty->assign("is_createable", preg_match("/c/",$this->ui->get_permissions($c_dn,"fai/faiVariableEntry")) && !preg_match("/freeze/", $this->FAIstate));
    $smarty->assign("is_removeable",  preg_match("/d/",$this->ui->get_permissions($c_dn,"fai/faiVariableEntry")) && !preg_match("/freeze/", $this->FAIstate));

    $tmp = $this->plInfo();
    foreach($tmp['plProvidedAcls'] as $name => $translation) {
      $smarty->assign($name."ACL",$this->getacl($name,preg_match("/freeze/i",$this->FAIstate)));
    }
    

    $display.= $smarty->fetch(get_template_path('faiVariable.tpl', TRUE));
    return($display);
  }

  /* Generate listbox friendly SubObject list
  */
  function getList(){
    $a_return=array();
    foreach($this->SubObjects as $obj){
      if($obj['status'] != "delete"){

        if((isset($obj['description']))&&(!empty($obj['description']))&&(!preg_match("/\[\*\]/",$obj['description']))){
          if (preg_match("/\[\*\]/", $obj['description'])){
            $a_return[$obj['cn']]= $obj['cn']." [".preg_replace("/\s*\[\*\]\s*/", "", $obj['description'])."]";
          } else {
            $a_return[$obj['cn']]= $obj['cn']."=".$obj['FAIvariableContent']." [".$obj['description']."]";
          }
        }else{
          if (preg_match("/\[\*\]/", $obj['description'])){
            $a_return[$obj['cn']]= $obj['cn'];
          } else {
            $a_return[$obj['cn']]= $obj['cn']."=".$obj['FAIvariableContent'];
          }
        }
      }
    }
    return($a_return);
  }

  /* Delete me, and all my subtrees
   */
  function remove_from_parent()
  {
    if($this->acl_is_removeable()){
      $ldap = $this->config->get_ldap_link();
      $ldap->cd ($this->dn);
      $release = $this->parent->parent->fai_release;
      $use_dn = preg_replace("/".preg_quote(FAI::get_release_dn($this->dn), '/')."/i", $release, $this->dn);
      new log("remove","fai/".get_class($this),$use_dn,$this->attributes);
      FAI::prepare_to_save_FAI_object($use_dn,array(),true);

      foreach($this->SubObjects as $name => $obj){
        $use_dn = preg_replace("/".preg_quote(FAI::get_release_dn($this->dn), '/')."/i", $release, $obj['dn']);
        FAI::prepare_to_save_FAI_object($use_dn,array(),true);
      }
      $this->handle_post_events("remove");
    }
  }


  /* Save data to object 
   */
  function save_object()
  {
    if((isset($_POST['FAIvariable_posted'])) && !preg_match("/freeze/", $this->FAIstate) ){
      plugin::save_object();
    }
  }


  /* Check supplied data */
  function check()
  {
    /* Call common method to give check the hook */
    $message= plugin::check();

    /* Ensure that we do not overwrite an allready existing entry 
     */
    if($this->is_new){
      $release = $this->parent->parent->fai_release;
      $new_dn= 'cn='.$this->cn.",".get_ou("faiVariable", "faiVariableRDN").get_ou("faiManagement", "faiBaseRDN").$release;
      $res = faiManagement::check_class_name("FAIvariable",$this->cn,$new_dn);
      if(isset($res[$this->cn])){
        $message[] = msgPool::duplicated(_("Name"));
      }
    }

    return ($message);
  }


  /* Save to LDAP */
  function save()
  {
    plugin::save();
 
    $ldap = $this->config->get_ldap_link();
    FAI::prepare_to_save_FAI_object($this->dn,$this->attrs);

    if($this->initially_was_account){
      new log("modify","fai/".get_class($this),$this->dn,$this->attributes);
    }else{
      new log("create","fai/".get_class($this),$this->dn,$this->attributes);
    }
 
    /* Prepare FAIscriptEntry to write it to ldap
     * First sort array.
     *  Because we must delete old entries first.
     * After deletion, we perform add and modify 
     */
    $Objects = $this->SubObjects;
    foreach($Objects as $name => $obj){

      foreach($this->sub64coded as $codeIt){
        $obj[$codeIt]=base64_encode($obj[$codeIt]);
      }

      $tmp = array();
      foreach($this->subAttributes as $attrs){
        if(!isset($obj[$attrs]) || $obj[$attrs] == ""){
          $obj[$attrs] = array();
        }
        $tmp[$attrs] = $obj[$attrs];
      }    
        
      $tmp['objectClass'] = $this->subClasses;

      $sub_dn = "cn=".$obj['cn'].",".$this->dn;

      if($obj['status']=="new"){
        $ldap->cat($sub_dn,array("objectClass"));
        if($ldap->count()){
          $obj['status']="edited";
        }
      }

      /* Tag object */
      $this->tag_attrs($tmp, $sub_dn, $this->gosaUnitTag);
      
      if($obj['status'] == "delete"){
        FAI::prepare_to_save_FAI_object($sub_dn,array(),true);
        $this->handle_post_events("remove");
      }elseif($obj['status'] == "edited"){
        FAI::prepare_to_save_FAI_object($sub_dn,$tmp);
        $this->handle_post_events("modify");
      }elseif($obj['status']=="new"){
        FAI::prepare_to_save_FAI_object($sub_dn,$tmp);
        $this->handle_post_events("add");
      }

    }
  }


  function PrepareForCopyPaste($source)
  {
    plugin::PrepareForCopyPaste($source);

    /* Read all leaf objects of this object (For FAIscript this would be FAIscriptEntry)
     */
    $res = FAI::get_all_objects_for_given_object($source['dn'],"(&(objectClass=FAIclass)(objectClass=".$this->subClass."))");
    foreach($res as $obj){

      $objects = array();
      $objects['status']      = "edited";
      $objects['dn']          = $obj['dn'];
      $objects                = $this->get_object_attributes($objects,$this->subAttributes);
      $this->SubObjects[$objects['cn']] = $objects;
    }
  } 

 
  /* Return plugin informations for acl handling */ 
  static function plInfo()
  {
    return (array( 
          "plShortName" => _("Variable"),
          "plDescription" => _("FAI variable"),
          "plSelfModify"  => FALSE,
          "plDepends"     => array(),
          "plPriority"    => 22,
          "plSection"     => array("administration"),
          "plCategory"    => array("fai"),

          "plProperties" =>
          array(
              array(
                  "name"          => "faiVariableRDN",
                  "type"          => "rdn",
                  "default"       => "ou=variables,",
                  "description"   => _("RDN for FAI variable storage."),
                  "check"         => "gosaProperty::isRdn",
                  "migrate"       => "migrate_faiVariableRDN",
                  "group"         => "plugin",
                  "mandatory"     => TRUE)),

          "plProvidedAcls" => array(
            "cn"                => _("Name")." ("._("Read only").")",
            "description"       => _("Description"))
          ));
  }


  /*! \brief  Used for copy & paste.
    Returns a HTML input mask, which allows to change the cn of this entry.
    @param  Array   Array containing current status && a HTML template.
   */
  function getCopyDialog()
  {
    $vars = array("cn");
    $smarty = get_smarty();
    $smarty->assign("cn", set_post($this->cn));
    $str = $smarty->fetch(get_template_path("paste_generic.tpl",TRUE));
    $ret = array();
    $ret['string'] = $str;
    $ret['status'] = "";
    return($ret);
  }


  /*! \brief  Used for copy & paste.
    Some entries must be renamed to avaoid duplicate entries.
   */
  function saveCopyDialog()
  {
    if(isset($_POST['cn'])){
      $this->cn = get_post('cn');
    }
  }

  /* Reload some attributes */
  function get_object_attributes($object,$attributes)
  {
    $ldap = $this->config->get_ldap_link();
    $ldap->cd($this->config->current['BASE']);
    $ldap->cat($object['dn'],$attributes);
    $tmp  = $ldap->fetch();

    foreach($attributes as $attrs){
      if(isset($tmp[$attrs][0])){
        $var = $tmp[$attrs][0];

        /* Check if we must decode some attributes */
        if(in_array_ics($attrs,$this->sub64coded)){
          $var = base64_decode($var);
        }

        /*  check if this is a binary entry */
        if(in_array_ics($attrs,$this->subBinary)){
          $var = $ldap->get_attribute($object['dn'], $attrs,$r_array=0);
        }

        $object[$attrs] = $var;
      }
    }
    return($object);
  }

}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
