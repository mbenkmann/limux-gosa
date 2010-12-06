<?php

class faiTemplate extends plugin
{
  /* attribute list for save action */
  var $ignore_account   = TRUE;

  /* Attributes for this Object */
  var $attributes       = array("cn","description");

  /* ObjectClasses for this Object*/
  var $objectclasses    = array("top","FAIclass","FAItemplate");

  /* Class name of the Ldap ObjectClass for the Sub Object */
  var $subClass         = "FAItemplateEntry";
  var $subClasses       = array("top","FAIclass","FAItemplateEntry");

  /* Class name of the php class which allows us to edit a Sub Object */
  var $subClassName     = "faiTemplateEntry";      

  /* Attributes to initialise for each subObject */
  var $subAttributes    = array("cn","description","FAItemplatePath","FAIowner","FAImode");
  var $sub_Load_Later   = array("FAItemplateFile"); 
  var $sub64coded       = array();
  var $subBinary        = array("FAItemplateFile");

  /* Specific attributes */
  var $cn               = "";       // The class name for this object
  var $description      = "";       // The description for this set of partitions
  var $is_dialog        = false;    // specifies which buttons will be shown to save or abort
  var $SubObjects       = array();  // All leafobjects of this object

  var $FAIstate         = "";
  var $ui;
  var $view_logged      = FALSE;

  function faiTemplate (&$config, $dn= NULL)
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

    // Prepare lists
    $this->templateList = new sortableListing();
    $this->templateList->setDeleteable(true);
    $this->templateList->setEditable(true);
    $this->templateList->setWidth("100%");
    $this->templateList->setHeight("220px");
    $this->templateList->setColspecs(array('*','18px'));
    $this->templateList->setHeader(array(_("Name"),""));
    $this->templateList->setDefaultSortColumn(0);

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

    /* New Listhandling
     */
    $this->templateList->save_object();
    $action = $this->templateList->getAction();
    if($action['action'] == 'delete'){
        $id = $this->templateList->getKey($action['targets'][0]);
        $obj  = $this->SubObjects[$id];
        $acl_dn = $this->acl_base_for_current_object($obj['dn']);
        $acl = $this->ui->get_permissions($acl_dn,"fai/faiTemplateEntry");
        if(preg_match("/d/",$acl)){
            $status = $this->SubObjects[$id]['status'];
            if($status == "edited" || $status == "FreshLoaded"){
                $this->SubObjects[$id]['status']= "delete";
            }else{
                unset($this->SubObjects[$id]);
            }
        }
    }
    if($action['action'] == 'edit'){
        $id = $this->templateList->getKey($action['targets'][0]);
        $obj  = $this->SubObjects[$id];
        $acl_dn = $this->acl_base_for_current_object($obj['dn']);
        $acl = $this->ui->get_permissions($acl_dn,"fai/faiTemplateEntry");
        if(preg_match("/r/",$acl)){
            if($obj['status'] == "FreshLoaded"){
                $obj  = $this->get_object_attributes($obj,$this->sub_Load_Later);
            }
            $this->dialog= new $this->subClassName($this->config,$this->dn,$obj);
            $this->dialog->set_acl_base($this->acl_base);
            $this->dialog->FAIstate = $this->FAIstate;
            $this->dialog->set_acl_category("fai");

            set_object_info($obj['dn']);
            $this->dialog->parent = &$this;
            $this->is_dialog=true;
        }
    }

    $once = true;
    foreach($_POST as $name => $value){
      if(preg_match("/^download_/",$name)&&($once) && !preg_match("/freeze/",$this->FAIstate)){
        $once = false;
        $entry = postDecode(preg_replace("/^download_/","",$name));
        $obj  = $this->SubObjects[$entry];
        $obj  = $this->get_object_attributes($obj,$this->sub_Load_Later);
        if(!isset($obj['cn'])){
          trigger_error("Something wen't wrong here!");
        }else{
          send_binary_content($obj['FAItemplateFile'],$obj['cn'].".FAItemplate");
        }
      } 
    }

    /* Edit entries via GET */
    if(isset($_GET['act']) && isset($_GET['id'])){
      if($_GET['act'] == "edit" && isset($this->SubObjects[$_GET['id']])){
        $obj = $this->SubObjects[$_GET['id']];
        if($obj['status'] == "FreshLoaded"){
          $obj  = $this->get_object_attributes($obj,$this->sub_Load_Later);
        }
        $this->dialog= new $this->subClassName($this->config,$this->dn,$obj);
        $this->dialog->FAIstate = $this->FAIstate;
        $this->dialog->acl = $this->acl;
        $this->dialog->set_acl_category("fai");
        set_object_info($obj['dn']);
        $this->dialog->parent = &$this;
        $this->is_dialog=true;
      }
    }

    /* Add new sub object */
    if(isset($_POST['AddSubObject']) && !preg_match("/freeze/",$this->FAIstate)){
      $acl_dn = $this->acl_base_for_current_object($this->dn);
      $acl    = $this->ui->get_permissions($acl_dn,"fai/faiTemplateEntry");

      if(preg_match("/c/",$acl)){
        $this->dialog= new $this->subClassName($this->config,"new");
        $this->dialog->set_acl_base($this->acl_base);
        $this->dialog->FAIstate = $this->FAIstate;
        $this->dialog->set_acl_category("fai");
        $this->dialog->parent = &$this;
        $this->is_dialog=true;
      }
    }

    if($this->dn != "new"){
      set_object_info($this->dn);
    }

    /* Save Dialog */
    if(isset($_POST['SaveSubObject']) && is_object($this->dialog)){
      $this->dialog->save_object();
      $msgs = $this->dialog->check();
      if(count($msgs)>0){
        foreach($msgs as $msg){
          msg_dialog::display(_("Error"), $msg, ERROR_DIALOG);
        }
      }else{
        /* Get return object */
        $obj = $this->dialog->save();
        if(isset($obj['remove'])){

          $old_stat = $this->SubObjects[$obj['remove']['from']]['status'];

          /* Depending on status, set new status */
          if($old_stat == "edited" || $old_stat == "FreshLoaded"){
            $this->SubObjects[$obj['remove']['from']]['status'] = "delete";
          }elseif($this->SubObjects[$obj['remove']['from']]['status']=="new"){
            unset($this->SubObjects[$obj['remove']['from']]);
          }
          $obj['status'] = "new";
          $this->SubObjects[$obj['remove']['to']] = $obj;
          unset($this->SubObjects[$obj['remove']['to']]['remove']);
        }else{
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

     /* list Containing FAItemplates */
    $dn  = $this->acl_base_for_current_object($this->dn);
    $acl = $this->ui->get_permissions($dn,"fai/faiTemplateEntry")  ;
    $this->templateList->setAcl($acl,preg_match('/^freeze/', $this->FAIstate));

    $tmp = $this->getList(true);
    $data = $lData = array();
    foreach($this->SubObjects as $key => $name){

      /* Skip removed entries */ 
      if($name['status'] == "delete") continue;

      $act = "";

      /* Check acls for download icon */
      $dn  = $this->acl_base_for_current_object($name['dn']);
      $s_acl = $this->ui->get_permissions($dn,"fai/faiTemplateEntry","FAItemplateFile")  ;
      if(($this->SubObjects[$key]['status'] == "new") || 
              ($this->SubObjects[$key]['dn'] == "new") || 
              !preg_match("/r/",$s_acl)){
        $down = "";
      }else{
        $down = image('images/save.png','download_'.postEncode($key), _("Download"));
      }
      $data[$key] = $key;
      $lData[$key] = array('data' => array($name['cn'], $down));
    }

    $this->templateList->setListData($data,$lData);
    $this->templateList->update();
    $smarty->assign("Entry_listing",$this->templateList->render());

     /* Magic quotes GPC, escapes every ' " \, to solve some security risks
     * If we post the escaped strings they will be escaped again
     */
    foreach($this->attributes as $attrs){
      $smarty->assign($attrs,set_post($this->$attrs));
    }

    $dn = $this->acl_base_for_current_object($this->dn);
    $smarty->assign("sub_object_is_addable", 
            preg_match("/c/",$this->ui->get_permissions($dn,"fai/faiTemplateEntry")) && 
            !preg_match("/freeze/",$this->FAIstate));

    foreach($this->attributes as $attr){
      $smarty->assign($attr."ACL",$this->getacl($attr, preg_match("/freeze/",$this->FAIstate)));
    }

    $display.= $smarty->fetch(get_template_path('faiTemplate.tpl', TRUE));
    return($display);
  }


  function acl_base_for_current_object($dn)
  {
    if($dn == "new" || $dn == ""){
      if($this->dn == "new"){
        $dn = $dn= $this->parent->parent->acl_base;
      }else{
        $dn = $this->dn;
      }
    }
    return($dn);
  }


  /* Generate listbox friendly SubObject list
  */
  function getList(){
    $a_return=array();
    foreach($this->SubObjects as $obj){
      if($obj['status'] != "delete"){
      
        if((isset($obj['description']))&&(!empty($obj['description']))){
          if(strlen($obj['description']) > 40){
            $obj['description'] = substr($obj['description'],0,40)."...";
          }
          $a_return[$obj['cn']]= $obj['cn']." [".$obj['description']."]";
        }else{
          $a_return[$obj['cn']]= $obj['cn'];
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
      $use_dn = preg_replace("/".preg_quote(FAI::get_release_dn($this->dn, '/'))."/i", $release, $this->dn);
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
    if((isset($_POST['FAItemplate_posted'])) && (!preg_match("/freeze/",$this->FAIstate))){
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
      $new_dn= 'cn='.$this->cn.",".get_ou("faiTemplate", "faiTemplateRDN").get_ou("faiManagement", "faiBaseRDN").$release;
      $res = faiManagement::check_class_name("FAItemplate",$this->cn,$new_dn);
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
    $tmp = $this->SubObjects;
    foreach($tmp as $name => $obj){
      if($obj['status'] == "FreshLoaded"){
        unset($tmp[$name]);
      }
    }

    foreach($tmp as $name => $obj){
      if($obj['status'] == "delete"){
        $Objects[$name] = $obj; 
      }
    }
    foreach($tmp as $name => $obj){
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
        if(!isset($obj[$attrs]) || $obj[$attrs] == ""){
          $obj[$attrs] = array();
        }
        $tmp[$attrs] =($obj[$attrs]);
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
  

  /* Return plugin informations for acl handling */ 
  static function plInfo()
  {
    return (array( 
          "plShortName" => _("Template"),
          "plDescription" => _("FAI template"),
          "plSelfModify"  => FALSE,
          "plDepends"     => array(),
          "plPriority"    => 24,
          "plSection"     => array("administration"),
          "plCategory"    => array("fai"),
          "plProperties" =>
          array(
              array(
                  "name"          => "faiTemplateRDN",
                  "type"          => "rdn",
                  "default"       => "ou=templates,",
                  "description"   => _("RDN for FAI template storage."),
                  "check"         => "gosaProperty::isRdn",
                  "migrate"       => "migrate_faiTemplateRDN",
                  "group"         => "plugin",
                  "mandatory"     => TRUE)),

          "plProvidedAcls" => array(
              "cn"                => _("Name")." ("._("Read-only").")",
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
}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
