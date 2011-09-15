<?php

class faiScript extends plugin
{
  /* attribute list for save action */
  var $ignore_account   = TRUE;

  /* Attributes for this Object */
  var $attributes       = array("cn","description");

  /* ObjectClasses for this Object*/
  var $objectclasses    = array("top","FAIclass","FAIscript");

  /* Class name of the Ldap ObjectClass for the Sub Object */
  var $subClass         = "FAIscriptEntry";
  var $subClasses       = array("top","FAIclass","FAIscriptEntry");

  /* Class name of the php class which allows us to edit a Sub Object */
  var $subClassName     = "faiScriptEntry";      

  /* Attributes to initialise for each subObject */
  var $subAttributes    = array("cn","description","FAIpriority"); 
  var $sub_Load_Later   = array("FAIscript");
  var $sub64coded       = array();
  var $subBinary        = array("FAIscript");

  /* Specific attributes */
  var $cn               = "";       // The class name for this object
  var $description      = "";       // The description for this set of partitions
  var $is_dialog        = false;    // specifies which buttons will be shown to save or abort
  var $SubObjects       = array();  // All leafobjects of this object

  var $FAIstate         = "branch";
  var $sort_by          = "name";
  var $sort_order       = "up";

  var $view_logged = FALSE;
  var $ui;

  function faiScript (&$config, $dn= NULL)
  {
    /* Load Attributes */
    plugin::plugin ($config, $dn);

    /* If "dn==new" we try to create a new entry
     * Else we must read all objects from ldap which belong to this entry.
     * First read SubObjects from ldap ... and then the partition definitions for the SubObjects.
     */
    if($dn != "new"){
      $this->dn =$dn;

      /* Read all leaf objects of this object (For FAIscript this would be FAIscriptEntry)
       */
      $res = FAI::get_all_objects_for_given_base($this->dn,"(&(objectClass=FAIclass)(objectClass=".$this->subClass."))");
      foreach($res as $obj){

        /* Skip not relevant objects */
        if(!preg_match("/".preg_quote($this->dn, '/')."$/i",$obj['dn'])) continue;

        $objects = array();
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

    $this->scriptListWidget= new sortableListing($this->convertList(TRUE), $this->convertList());
    $this->scriptListWidget->setDeleteable(true);
    $this->scriptListWidget->setInstantDelete(false);
    $this->scriptListWidget->setEditable(true);
    $this->scriptListWidget->setWidth("100%");
    $this->scriptListWidget->setHeight("140px");
    $this->scriptListWidget->setHeader(array(_("Name"),_("Description")));
  }


  function convertList($type = FALSE)
  {
    $data = array();
    $s_acl = $this->ui->get_permissions($this->dn,"fai/faiScriptEntry","FAIscript");
    foreach($this->SubObjects as $cn => $entry){
      if($entry['status'] == "delete") continue;
      if($type){
        $data[$cn] = $entry;
      }else{
        if(!isset($entry['description'])) $entry['description']="";
        $data[$cn] = array('data' => array($entry['cn'], $entry['description']));
      }
    }
    return($data);
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

  
  /* Return a valid dn to fetch acls. Because 'new' will not work. */
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
      $this->dialog->FAIstate = $this->FAIstate;
      $this->dialog->set_acl_base($this->acl_base);
      $this->dialog->set_acl_category("fai");
      $this->dialog->parent = &$this;
      $this->is_dialog=true;
    }

    if($this->dn != "new"){
      set_object_info($this->dn);
    }

    $this->scriptListWidget->setAcl($this->getacl(""));
    $this->scriptListWidget->save_object();
    $action = $this->scriptListWidget->getAction();
    if($action['action'] =="edit"){
      $s_entry = $this->scriptListWidget->getKey($action['targets'][0]);
      if(isset($this->SubObjects[$s_entry])){ 

        $obj  = $this->SubObjects[$s_entry];
        if($obj['status'] == "FreshLoaded"){
          $obj  = $this->get_object_attributes($obj,$this->sub_Load_Later);
        }

        /* Create new dialog and set acl attributes  */
        $this->dialog= new $this->subClassName($this->config,$this->dn,$obj);
        $this->dialog->FAIstate = $this->FAIstate;
        $this->dialog->set_acl_base($this->acl_base_for_current_object($obj['dn']));
        $this->dialog->set_acl_category("fai");

        /* Assign some additional dialog informations like headline and parent  */
        set_object_info($obj['dn']);
        $this->dialog->parent = &$this;
        $this->is_dialog=true;
      }
    }

    /* Check acls, are we allowed to delete an entry */
    if($action['action'] =="delete"){
      $s_entry = $this->scriptListWidget->getKey($action['targets'][0]);
      if(isset($this->SubObjects[$s_entry])){ 
        $entry = $this->SubObjects[$s_entry];  
        $acl = $this->ui->get_permissions($this->acl_base_for_current_object($entry['dn']),"fai/faiScriptEntry")  ;
        if(preg_match("/d/",$acl)){
          $status = $entry['status'];
          if($status == "edited" || $status == "FreshLoaded"){
            $this->SubObjects[$s_entry]['status']= "delete";
          }else{
            unset($this->SubObjects[$s_entry]);
          }
        }
      }
    }


    /* Save the edited entry */
    if(isset($_POST['SaveSubObject'])){

      /* Check if there are still errors remaining that must be fixed before saving */
      $this->dialog->save_object();
      $msgs = $this->dialog->check();
      if(count($msgs)>0){
        foreach($msgs as $msg){
          msg_dialog::display(_("Error"), $msg, ERROR_DIALOG);
        }
      }else{

        /* Get return object */
        $obj = $this->dialog->save();

        /* If we have renamed the script entry, we must remove the old entry */
        if(isset($obj['remove'])){

          /* Get old entry values */
          $old_stat = $this->SubObjects[$obj['remove']['from']]['status'];

          /* Depending on status, set new status */
          if($old_stat == "edited" || $old_stat == "FreshLoaded"){
            $this->SubObjects[$obj['remove']['from']]['status'] = "delete";
          }elseif($this->SubObjects[$obj['remove']['from']]['status']=="new"){
            unset($this->SubObjects[$obj['remove']['from']]);
          }

          /* Append the new entry */
          $obj['status'] = "new";
          $this->SubObjects[$obj['remove']['to']] = $obj;
          unset($this->SubObjects[$obj['remove']['to']]['remove']);
        }else{
  
          /* Set new status and append the entry */
          if($obj['status'] == "FreshLoaded"){
            $obj['status'] = "edited";
          }
          $this->SubObjects[$obj['cn']]=$obj;
        }
        $this->is_dialog=false;
        unset($this->dialog);
        $this->dialog=FALSE;

      }
    }

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

    $this->scriptListWidget->setListData($this->convertList(TRUE), $this->convertList());
    $this->scriptListWidget->update();
    $smarty->assign("Entry_listing",$this->scriptListWidget->render());

    /* Magic quotes GPC, escapes every ' " \, to solve some security risks
     * If we post the escaped strings they will be escaped again
     */
    foreach($this->attributes as $attrs){
      $smarty->assign($attrs,set_post($this->$attrs));
    }

    $dn = $this->acl_base_for_current_object($this->dn);
    $smarty->assign("sub_object_is_addable",  
        preg_match("/c/",$this->ui->get_permissions($dn,"fai/faiScriptEntry")) && 
        !preg_match("/freeze/",$this->FAIstate));

    $tmp = $this->plInfo();
    foreach($tmp['plProvidedAcls'] as $name => $translated){
      $smarty->assign($name."ACL",$this->getacl($name));
    }

    $display.= $smarty->fetch(get_template_path('faiScript.tpl', TRUE));
    return($display);
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
    if((isset($_POST['FAIscript_posted'])) && !preg_match("/freeze/", $this->FAIstate)){
      plugin::save_object();
    }
    
    /* Get sort order */
    if(isset($_GET['sort']) && in_array_strict($_GET['sort'],array("name","priority"))){
      if($this->sort_by == $_GET['sort']){
        if($this->sort_order == "up"){
          $this->sort_order = "down";
        }elseif($this->sort_order == "down"){
          $this->sort_order = "up";
        }
      }
      $this->sort_by = $_GET['sort'];
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
      $new_dn= 'cn='.$this->cn.",".get_ou("faiScript", "faiScriptRDN").get_ou("faiManagement", "faiBaseRDN").$release;
      $res = faiManagement::check_class_name("FAIscript",$this->cn,$new_dn);
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
    $Objects = array();

    /* We do not need to save untouched objects */
    foreach($this->SubObjects as $name => $obj){
      if($obj['status'] == "FreshLoaded"){
        unset($this->SubObjects[$name]);
      }
    }

    foreach($this->SubObjects as $name => $obj){
      if($obj['status'] == "delete"){
        $Objects[$name] = $obj; 
      }
    }
    foreach($this->SubObjects as $name => $obj){
      if($obj['status'] != "delete"){
        $Objects[$name] = $obj; 
      }
    }

    foreach($Objects as $name => $obj){

      foreach($this->sub64coded as $codeIt){
        $obj[$codeIt]=base64_encode($obj[$codeIt]);
      }

      $tmp = array();
      $attributes = array_merge($this->sub_Load_Later,$this->subAttributes);
      foreach($attributes as $attrs){
        if(!isset($obj[$attrs])) continue; 
        if($obj[$attrs] == ""){
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

      if(empty($tmp['FAIpriority'])){
        $tmp['FAIpriority']  ="0";
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
    $res = FAI::get_all_objects_for_given_base($source['dn'],"(&(objectClass=FAIclass)(objectClass=".$this->subClass."))");
    foreach($res as $obj){

      /* Skip not relevant objects */
      if(!preg_match("/".preg_quote($source['dn'], '/')."$/i",$obj['dn'])) continue;

      $objects = array();
      $objects['status']      = "edited";
      $objects['dn']          = $obj['dn'];
      $objects                = $this->get_object_attributes($objects,$this->subAttributes);
      $objects                = $this->get_object_attributes($objects,$this->sub_Load_Later);
      $this->SubObjects[$objects['cn']] = $objects;
    }
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
  

  /* Return plugin informations for acl handling */ 
  static function plInfo()
  {
    return (array( 
          "plShortName" => _("Script"),
          "plDescription" => _("FAI script"),
          "plSelfModify"  => FALSE,
          "plDepends"     => array(),
          "plPriority"    => 18,
          "plSection"     => array("administration"),
          "plCategory"    => array("fai"),
          "plProperties" =>
          array(
              array(
                  "name"          => "faiScriptRDN",
                  "type"          => "rdn",
                  "default"       => "ou=scripts,",
                  "description"   => _("RDN for FAI script storage."),
                  "check"         => "gosaProperty::isRdn",
                  "migrate"       => "migrate_faiScriptRDN",
                  "group"         => "plugin",
                  "mandatory"     => TRUE
                  )
              ),


          "plProvidedAcls" => array(
            "cn"                => _("Name")." ("._("Read-only").")",
            "description"       => _("Description"))
          ));
  }
}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
