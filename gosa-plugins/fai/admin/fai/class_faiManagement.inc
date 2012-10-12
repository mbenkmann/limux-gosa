<?php
/*
 * This code is part of GOsa (http://www.gosa-project.org)
 * Copyright (C) 2003-2008 GONICUS GmbH
 *
 * ID: $$Id: class_faiManagement.inc 14742 2009-11-04 13:18:33Z hickert $$
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software
 * Foundation, Inc., 59 Temple Place, Suite 330, Boston, MA  02111-1307  USA
 */

class faiManagement extends management
{
  var $plHeadline     = "Software deployment";
  var $plDescription  = "Manage software packages and deployment recipes for Windows and Linux systems";
  var $plIcon  = "plugins/fai/images/plugin.png";

  // Tab definition 
  protected $tabClass = "";
  protected $tabType = "";
  protected $aclCategory = "";
  protected $aclPlugin   = "";
  protected $objectName   = "FAI object";
  protected $releaseSelector = NULL;

  //  Attributes Managed by this plugin can be used in post events
  public $attributes   = array("lock_type","lock_name","lock_dn");  

  var $dispNewBranch=false;
  var $dispNewFreeze=false;

  var $fai_release = ""; // The currently selected release while in release management mode!
  var $fai_base = ""; 
  var $acl_base = ""; 

  var $lock_type = "";
  var $lock_dn = "";
  var $lock_name = "";

  var $opsi = NULL;


  function __construct($config,$ui)
  {
    $this->config = $config;

    /* Check if the opsi plugin is installed.
     */
    if(class_available("opsi")){
      $this->opsi = new opsi($this->config);;
    }

    $this->fai_base = get_ou("faiManagement", "faiBaseRDN").$this->config->current['BASE'];
    $cfg_rel = $this->config->get_cfg_value("faiManagement","defaultFaiRelease");

    if(!empty($cfg_rel)){
      $this->fai_release = $cfg_rel;
    }else{
      $this->fai_release = $this->fai_base;
    }

    $releases = $this->getReleaseList();
    if(!isset($releases[$this->fai_release])){
      $this->fai_release = $this->fai_base;
    }

    $this->acl_base = $this->config->current['BASE'];
    $this->ui = $ui;
    $this->storagePoints = array(
        get_ou("faiPartitionTable", "faiPartitionRDN"),
        get_ou("faiPackage", "faiPackageRDN"),
        get_ou("faiScript", "faiScriptRDN"),
        get_ou("faiVariable", "faiVariableRDN"),
        get_ou("faiHook", "faiHookRDN"),
        get_ou("faiProfile", "faiProfileRDN"),
        get_ou("faiTemplate", "faiTemplateRDN"));

    // Build filter
    if (session::global_is_set(get_class($this)."_filter")){
      $filter= session::global_get(get_class($this)."_filter");
    } else {
      $filter = new filter(get_template_path("fai-filter.xml", true));
      $filter->setObjectStorage($this->storagePoints);
    }
    $filter->elementValues['RELEASE'] = $this->fai_release;
    $this->setFilter($filter);

    // Build headpage
    $headpage = new listing(get_template_path("fai-list.xml", true));
    $headpage->setFilter($filter);
    $headpage->setBase($this->fai_release);
    $headpage->registerElementFilter("filterProperties", "faiManagement::filterProperties");
    $headpage->registerElementFilter("filterLockedItem", "faiManagement::filterLockedItem");
    // Add copy&paste and snapshot handler.
    if ($this->config->boolValueIsTrue("core", "copyPaste")){
      $this->cpHandler = new CopyPasteHandler($this->config);
    }

    $this->registerAction("remove_multiple","removeEntryRequested");
    $this->registerAction("new_profile","newEntry");
    $this->registerAction("new_template","newEntry");
    $this->registerAction("new_script","newEntry");
    $this->registerAction("new_hook","newEntry");
    $this->registerAction("new_variable","newEntry");
    $this->registerAction("new_package","newEntry");
    $this->registerAction("new_partition","newEntry");
    $this->registerAction("newClassNameSelected","newClassNameSelected");
    $this->registerAction("saveOpsiProperties","saveOpsiProperties");
    $this->registerAction("editByGroup","editByGroup");
    $this->registerAction("createBranch","createBranch");
    $this->registerAction("createFreeze","createFreeze");
    $this->registerAction("removeBranch","removeBranch");
    $this->registerAction("removeBranchConfirmed","removeBranchConfirmed");
    $this->registerAction("saveBranch","saveBranch");
    $this->registerAction("PerformBranch","PerformBranch");

    $fai_types = array(
      "FAIpartitionTable",
      "FAIpackageList",
      "FAIscript",
      "FAIvariable",
      "FAIhook",
      "FAIprofile",
      "FAItemplate",
    );

    foreach ($fai_types as $type) {
      $this->registerAction("edit".$type, "editSubEntry");
    }

    $releases = $this->getReleaseStates($this->config,$this->fai_base);
    $this->releaseSelector = new releaseSelector($releases, $this->fai_release, $this->fai_base);
    parent::__construct($config, $ui, "FAI object", $headpage);
  }


  /*! \brief  Act on copy & paste actions here.
   */
  function copyPasteHandler($action="",$target=array(),$all=array(),$altTabClass ="",$altTabType="",$altAclCategory="",$altAclPlugin="")
  {
    // Collect real dns, the listed objects are grouped by their cn
    $headpage = $this->getHeadpage();
    if($action == "copy"){

      if(count($target) == 1){

        // We just want to copy a single FAI object, let the user choose entries from the FAI-Group
        $entry = $headpage->getEntry($target[0]);
        if(count($entry['GROUPS']) == 1){
          $data = array_pop($entry['GROUPS']);
          $type = $this->get_type($data);
          $this->cpHandler->add_to_queue($data['dn'],"copy",$type[0],$type[2],'fai',$this);
          @DEBUG (DEBUG_TRACE, __LINE__, __FUNCTION__, __FILE__,$g['dn'],"Entry copied!");
        }else{
          $this->dialogObject = new faiGroupHandle($entry['GROUPS'],"copy");
        }
        
      }else{

        // More than one group was selected, expect that the user wants to copy the complete groups.
        foreach($target as $t){
          $entry = $headpage->getEntry($t);

          // Check for valid FAI objects
          if(in_array_strict('FAKE_OC_FAI', $entry['objectClass'])){
            foreach($entry['GROUPS'] as $g){
              $type = $this->get_type($g);
              $this->cpHandler->add_to_queue($g['dn'],"copy",$type[0],$type[2],'fai',$this);
              @DEBUG (DEBUG_TRACE, __LINE__, __FUNCTION__, __FILE__,$g['dn'],"Entry copied!");
            }
          }
        }
      }
    }

    // Initiate pasting
    if($action == "paste"){
      $this->cpPastingStarted = TRUE;
    }

    // Display any c&p dialogs, eg. object modifications required before pasting.
    if($this->cpPastingStarted && $this->cpHandler->entries_queued()){
      $this->cpHandler->SetVar("base",$headpage->getBase());
      $data = $this->cpHandler->execute();
      FAI::save_release_changes_now();
      if(!empty($data)){
        return($data);
      }
    }

    // Automatically disable pasting process since there is no entry left to paste.
    if(isset($this->cpHandler) && !$this->cpHandler->entries_queued()){
      $this->cpPastingStarted = FALSE;
    }
    return("");
  }


  /*! \brief  A new FAI object was requested, let the user specify a name theis object now. 
   */
  function newEntry($action="",$target=array(),$all=array(), $altTabClass ="", $altTabType = "", $altAclCategory="")
  {
    $types = array( 
        "new_partition"     =>  "FAIpartitionTable",
        "new_script"        =>  "FAIscript",
        "new_hook"          =>  "FAIhook",
        "new_variable"      =>  "FAIvariable",
        "new_template"      =>  "FAItemplate",
        "new_package"       =>  "FAIpackageList");
    $types_i18n = array( 
        "new_partition"     =>  _("partition table"),
        "new_script"        =>  _("script"),
        "new_hook"          =>  _("hook"),
        "new_variable"      =>  _("variable"),
        "new_template"      =>  _("template"),
        "new_package"       =>  _("package list"));

    if(isset($types[$action])){
      $type_acl_mapping = array(
          "FAIpartitionTable"  => "faiPartitionTable",
          "FAIpackageList"     => "faiPackage",
          "FAIscript"          => "faiScript",
          "FAIvariable"        => "faiVariable",
          "FAIhook"            => "faiHook",
          "FAIprofile"         => "faiProfile",
          "FAItemplate"        => "faiTemplate");

      $acl = $this->ui->get_permissions($this->acl_base,"fai/".$type_acl_mapping[$types[$action]]);
      if(preg_match("/c/",$acl)){
        $this->dialogObject = new askClassName($this->config,$this->dn,$this->ui,$types[$action]);
        $this->dialogObject->parent = &$this;
      }else{
        msg_dialog::display(_("Permission error"), 
            sprintf(_("You have no permission to create a new %s!"), $types_i18n[$action]),      ERROR_DIALOG);
      }
    }
    if($action == "new_profile"){
      $this->dn = "new" ;

      $acl = $this->ui->get_permissions($this->acl_base,"fai/faiProfile");
      if(preg_match("/c/",$acl)){
        $type= $this->get_type(array("objectClass"=>array("FAIprofile")));
        $str= management::newEntry('newEntry',array(),array(),$type[0],$type[2],'fai');
        if($str) return($str);
        $this->tabObject->set_acl_base($this->acl_base);
      }else{
        msg_dialog::display(_("Permission error"), sprintf(_("You have no permission to create a new %s!"), _("profile")), ERROR_DIALOG);
      }
    }
  }


  /*! \brief   A new FAI class was requested and the user had a specify a name for it.
   *           Here we check if this name is useable and then open the edit dialogs.
   */
  function newClassNameSelected()
  {
    $this->dialogObject->save_object();
    if(count($this->dialogObject->check())!=0){
      foreach($this->dialogObject->check() as $msg){
        msg_dialog::display(_("Error"), $msg, ERROR_DIALOG);
      }
    }elseif(isset($this->dialogObject->objectClass)){
      $this->dn = "new" ;
      $type= $this->get_type(array("objectClass"=>array($this->dialogObject->objectClass)));
      $name = $this->dialogObject->save();

      if(class_exists($type[0])){
        $this->closeDialogs();
        $str = management::newEntry('newEntry',array(),array(),$type[0],$type[2],"fai");
        if($str) return($str);
        $this->tabObject->set_acl_base($this->acl_base);
        $this->tabObject->by_object[$type[1]]->cn = $name;
      }
    }
  }

  /*! \brief Initialize attributes of a FAI tab, like FAIstate etc.
   *
   * @param Array Type array as returned by $this->get_type()
   * @param String The FAIstate of the object, which is editted, usually
   * taken from $data['FAIstate']
   * */
  function init_tab_attrs($type, $FAIstate) {
    $this->tabObject->by_object[$type[1]]->FAIstate = $FAIstate;
    $this->tabObject->FAIstate = $FAIstate;
    if(!$this->tabObject->read_only)
        $this->tabObject->read_only = preg_match("/freeze/i", $FAIstate);
    if(!$this->tabObject->by_object[$type[1]]->read_only)
        $this->tabObject->by_object[$type[1]]->read_only=preg_match("/freeze/i", $FAIstate);
  }


  function editSubEntry($action,$target, $all) {

    // Guess requested type from action name
    $class = str_replace("edit", "", $action);
    $type = $this->get_type(array ( "objectClass" =>  array($class) ) );
    $headpage = $this->getHeadpage();
    $entry = $headpage->getEntry($target[0]);

    if ($entry) {
      $data = $entry['GROUPS'][$class];
      $str = management::editEntry('editEntry', array($data['dn']), $all,
      $type[0], $type[2], "fai");

      if ($str) return $str;
      $this->init_tab_attrs($type, $data['FAIstate']);
    }
  }

  /*! \brief   Edit the selected entry.
   *           If there was a FAI group clicked, display a dialog with all members of the group.
   */
  function editEntry($action="",$target=array(),$all=array(), $altTabClass ="", $altTabType = "", $altAclCategory="")
  {
    $headpage = $this->getHeadpage();
    if(count($target) == 1){
      $entry = $headpage->getEntry($target[0]);
      if($entry){

        // Edit Opsi objects here 
        if(in_array_strict("opsi_local", $entry['TYPES']) || in_array_strict("opsi_netboot", $entry['TYPES'])){ 
          $name = $entry['cn'];
          $cfg = $this->opsi->get_product_properties($name);

          $str = management::editEntry('editEntry',array($name),array(),'tabs_opsiProdConfig','OPSIPRODCONFIG','opsi');
          if($str) return($str);
          if(isset($this->tabObject->by_object['opsiProperties'])){
            $this->tabObject->by_object['opsiProperties']->set_cfg($cfg);
            $this->tabObject->by_object['opsiProperties']->set_product($name);
            $this->skipFooter = TRUE;
          }else{
            trigger_error("Unknown tab, please check config.");
          }

        }else{

          // Edit FAI objects here 
          if(count($entry['GROUPS']) == 1){
            $data = array_pop($entry['GROUPS']);
            $type = $this->get_type($data);
            $str = management::editEntry('editEntry',array($data['dn']),array(),$type[0],$type[2],"fai");
            if($str) return($str);
            $this->init_tab_attrs($type, $data['FAIstate']);

          }else{
            $this->dialogObject = new faiGroupHandle($entry['GROUPS'],"edit");
          }
        }
      }
    }
  }


  /*! \brief   Save changes made in opsi dialogs.
   */
  function saveOpsiProperties()
  {
    if($this->tabObject instanceof tabs_opsiProdConfig && isset($_POST['save_properties'])){
      $this->tabObject->save_object();
      $op    = $this->tabObject->by_object['opsiProperties'];
      $name  = $op->get_product();
      $cfg   = $op->get_cfg();
      $this->opsi->set_product_properties($name,$cfg);
      if($this->opsi->is_error()){
        msg_dialog::display(_("Error"),msgPool::siError($this->opsi->get_error()),ERROR_DIALOG);
      }else{
        $this->remove_lock();
        $this->closeDialogs();
      }
    }
  }


  /*! \brief   Someone wants to remove some object(s)
   *            ask for a confirmation now.
   */
  function removeEntryRequested($action="",$target=array(),$all=array(), $altTabClass ="", $altTabType = "", $altAclCategory="")
  {
    $this->closeDialogs();
    if($action == "remove_multiple"){

      // Collect objects to delete
      $headpage = $this->getHeadpage();
      $to_delete = array();
      foreach($target as $id){
        $object = $headpage->getEntry($id);
        if(in_array_strict("FAKE_OC_FAI", $object['objectClass'])){
          foreach($object['GROUPS']  as $entry){
            array_push($to_delete, $entry);
          }
        }
      }
      return($this->removeFAIObjects($to_delete));
    }else{

      // Try to remove a single object, only FAI objects can be removed!
      $headpage = $this->getHeadpage();
      if(count($target) == 1){
        $entry = $headpage->getEntry($target[0]);
        if($entry && in_array_strict("FAKE_OC_FAI",$entry['objectClass'])){
          $this->dialogObject = new faiGroupHandle($entry['GROUPS'],"remove");
        }
      }
    }
  }

  /*! \brief Returns profiles the given class is in */
  private function getProfilesOfClass($class, $dn)
  {
    $ldap = $this->config->get_ldap_link();

    # Get COW bases => this strips the first RDN
    $cow_bases = get_parent_dns($dn, $this->fai_base, TRUE);

    # Drop FAI type base
    unset($cow_bases[0]);

    # Start at FAI root
    $cow_bases = array_reverse($cow_bases);

    # Look for profiles in all COW releases
    $profiles = array();
    foreach ($cow_bases as $base) {
      $ldap->ls('(objectClass=FAIprofile)', 'ou=profiles,' . $base, array('FAIclass', 'FAIstate', 'cn'), TRUE);

      while ($profile = $ldap->fetch()) {
        $name = $profile['cn'][0];

        # If FAI state is removed, drop profile from list.
        if (array_key_exists('FAIstate', $profile)
            && array_key_exists('removed', explode(',', $profile['FAIstate'][0]))) {
          if (array_key_exists($name, $profiles))
            unset($profiles[$name]);
        }
        else {
          $p_classes = $profile['FAIclass'][0];
          if (in_array_strict($class, explode(' ', $p_classes))
              && !array_key_exists($name, $profiles))
            $profiles[$name] = 1;
        }
      }
    }

    return array_keys($profiles);
  }


  /*! \brief   Someone wants to remove some object(s)
   *            ask for a confirmation now.
   */
  function removeFAIObjects($to_delete)
  {
    // Close dialogs and remove locks for currently handled dns
    $this->cancelEdit();

    // Do not allow to remove objects with state freeezed
    $errors = $disallowed = $used_in_profiles = array();
  
    foreach($to_delete as $obj){
      $type = $this->get_type($obj);
      $acl  = $this->ui->get_permissions($obj['dn'],"fai/".$type[1]);
      if(!preg_match("/d/",$acl)){
        $disallowed[] = $obj['dn'];
      }elseif(isset($obj['FAIstate']) && preg_match('/^freeze/', $obj['FAIstate'])){
        $errors[] = $obj['dn'];
      }else{
        $this->dns[] = $obj['dn'];
      }
      
      $profiles = $this->getProfilesOfClass($obj['cn'][0], $obj['dn']);
      foreach ($profiles as $profile) {
        $used_in_profiles[] = array($profile, $obj['cn'][0]);
      }
    }
    if(count($errors)){
      msg_dialog::display(_("Branch locked"),
          sprintf(_("The following entries are locked, you can't remove them %s."),msgPool::buildList($errors)),INFO_DIALOG);
    }
    if(count($disallowed)){ 
      msg_dialog::display(_("Permission error"), msgPool::permDelete($disallowed), ERROR_DIALOG);
    }
    
    if (count($used_in_profiles)) {
      $formatted_list = ' ';
    
      foreach ($used_in_profiles as $v) {
        list($profile, $class) = $v;
        $formatted_list .= '<li>' . sprintf(_("Class <strong>%s</strong> in profile <strong>%s</strong>"), $class, $profile) . '</li>';
      }
      
      msg_dialog::display(_("Classes used in profiles"),
          _("The following entries are used in profiles, you should't remove them:") . 
          " <ul>" . $formatted_list . "</ul>", WARNING_DIALOG);
    }

    // Check entry locking
    $smarty = get_smarty();
    if(count($this->dns)){
      if ($user= get_multiple_locks($this->dns)){
        return(gen_locked_message($user,$this->dns));
      }
      if(count($this->dns)){

        // Add locks
        $dns_names = array();
        $types = array();
        $h = $this->getHeadpage();

        // Build list of object -labels
        foreach($h->objectTypes as $type){
          $map[$type['objectClass']]= $type['label'];
        }

        foreach($this->dns as $dn){
          $tmp = $h->getType($dn);
          if(isset($map[$tmp])){
            $dns_names[] = '('._($map[$tmp]).')&nbsp;-&nbsp;'.LDAP::fix($dn);
          }else{
            $dns_names[] =LDAP::fix($dn);
          }
        }
        add_lock ($this->dns, $this->ui->dn);

        $smarty->assign("info",msgPool::deleteInfo($dns_names));
        return($smarty->fetch(get_template_path('removeEntries.tpl')));
      }
    }
  }


  /*! \brief   Entry removal is confirmed, now remove objects
   */
  function removeEntryConfirmed($action="",$target=array(),$all=array(),
      $altTabClass="",$altTabType="",$altAclCategory="", $altAclPlugin="")
  {
    $ldap = $this->config->get_ldap_link();
    $ldap->cd($this->config->current['BASE']); 

    $disallowed = array();
    foreach($this->dns as $key => $dn){
      $ldap->cat($dn);
      if($ldap->count()){
        $attrs = $ldap->fetch();
        $type= $this->get_type($attrs);
        $acl  = $this->ui->get_permissions($dn,"fai/".$type[1]);
        if(preg_match("/d/",$acl)){

          // Delete the object
          $this->dn = $dn;
          $tabClass = $type[0];
          $this->tabObject= new $tabClass($this->config,$this->config->data['TABS'][$type[2]], 
              $this->dn, "fai", true, true);
          $this->tabObject->set_acl_base($this->dn);
          $this->tabObject->parent = &$this;
          $this->tabObject->by_object[$type[1]]->remove_from_parent ();

          FAI::save_release_changes_now();
          $to_del = FAI::clean_up_releases($dn);
          /* Remove sub-objects (e.g. the variable key/value of a FAIvariable) from LDAP entirely */
          $children = FAI::get_child_objects($dn);
          if ($children) {
            $to_del += $children;
          }
          foreach($to_del as $dn){
            $ldap->rmdir_recursive($dn);
          }

        } else {
          $disallowed[] = $dn;
          new log("security","fai/".get_class($this),$dn,array(),"Tried to trick deletion.");
        }
      }
    }
   
    $this->remove_lock();
    $this->closeDialogs();

    /* Normally this shouldn't be reached, send some extra
       logs to notify the administrator */
    if(count($disallowed)){ 
      msg_dialog::display(_("Permission error"), msgPool::permDelete($disallowed), ERROR_DIALOG);
    }
  }


  /*! \brief   Someone clicked on edit/remove for a grouped FAI object.
   *           We are now going to display a dialog to let the user select the entry 
   *            he wants to perform the action on.
   */
  function editByGroup()  
  {
    if($this->dialogObject instanceOf faiGroupHandle){
     $mode = $this->dialogObject->get_mode();
     $this->dialogObject->save_object();
     $entry = $this->dialogObject->get_selected();
     
    } else {
     // Fallback to retreiving information from session, which is the
     // way to go if group entry selection dialog is followed by a locking dialog
     $mode = session::get('mode');
     $fai_group = session::get('FAI_group'); 
     $entry = array();
     foreach($fai_group as $key => $obj) {
       if($obj['selected']) {
         $entry[$key] = $obj;
       }
     }
    }

    if($mode && $mode == "edit"){
      $this->closeDialogs();
      $data = array_pop($entry);
      $type = $this->get_type($data);
      $str = management::editEntry('editEntry',array($data['dn']),array(),$type[0],$type[2],'fai');
      if($str) return($str);
      $this->init_tab_attrs($type, $data['FAIstate']);
    }elseif($mode && $mode == "remove"){
      $to_delete = $entry;
      if(count($to_delete)) $this->closeDialogs();
      return($this->removeFAIObjects($to_delete));
    }elseif($mode && $mode == "copy"){
      $entries = $entry;
      if(count($entries)){
        foreach($entries as $entry){
          $type = $this->get_type($entry);
          $this->cpHandler->add_to_queue($entry['dn'],"copy",$type[0],$type[2],'fai',$this);
          @DEBUG (DEBUG_TRACE, __LINE__, __FUNCTION__, __FILE__,$entry['dn'],"Entry copied!");
        }
        $this->closeDialogs();
      }
    }
  }


  protected function handle_release_cleanup($dn) {
    $to_del = FAI::clean_up_releases($dn, false);
    $ldap = $this->config->get_ldap_link();
    $ldap->cd($dn);
    foreach($to_del as $dn) {
      /* Only delete subobjects of the current FAI class */
      $cn = $ldap->getCn($dn);
      $ldap->cd($this->dn);
      $ldap->search("(|(objectClass=FAIclass)(objectClass=FAIdebconfInfo))", array('dn'));
      if ($ldap->count()){
        while($attrs = $ldap->fetch()){
          if ($attrs['dn'] == $dn) {
            $ldap->rmdir_recursive($dn);
          }
        }
      }
    }
  }

  /*! \brief   Save dialog/object modifications
   */
  protected function saveChanges()
  {
    $str = management::saveChanges();
    if($str) return($str);

    // Now save changes
    FAI::save_release_changes_now();

    // Cleanup
    $this->handle_release_cleanup($this->last_dn);
  }


  /*! \brief   Save dialog/object modifications but keep the dialogs opened
   */
  protected function applyChanges($re_init=FALSE)
  {
    // Call apply changes without re_init
    $str = management::applyChanges($re_init);
    if($str) return($str);

    // Now save changes
    FAI::save_release_changes_now();

    // Cleanup
    $this->handle_release_cleanup($this->dn);

    // Call re_init
    $this->tabObject->re_init();
  }


  /*! \brief   Initiates release removal
   */
  function removeBranch()
  {
    /* Check if we have a post remove method configured
     *  else skip this operation. (Skip:Button in the ui should be disabled in this case too)
     */
    if("" != $this->config->get_cfg_value("faiManagement","postremove")){
      /* Load permissions for selected 'dn' and check if
         we're allowed to remove this 'dn' */
      if(preg_match("/d/",$this->ui->get_permissions($this->acl_base,"fai/faiManagement"))){
        $smarty=get_smarty();
        $smarty->assign("release_hidden",base64_encode($this->fai_release));
        $smarty->assign("info", msgPool::deleteInfo(LDAP::fix($this->fai_release),_("FAI branch/freeze")));
        return($smarty->fetch(get_template_path('remove_branch.tpl',TRUE)));
      } else {
        msg_dialog::display(_("Permission error"), _("You have no permission to delete this release!"), ERROR_DIALOG);
      }
    }
  }


  /*! \brief   Remove a release after removal was confirmed
   */
  function removeBranchConfirmed()
  {
    /* Check if we have a post remove method configured
     *  else skip this operation. (Skip:Button in the ui should be disabled in this case too)
     */
    if("" != $this->config->get_cfg_value("faiManagement","postremove")){

      if(!isset($_POST['release_hidden']) || base64_decode($_POST['release_hidden']) != $this->fai_release){
        msg_dialog::display(_("Warning"),_("Release remove aborted because the release name check failed!"));
      }else{

        $bb =  $this->fai_release;
        $ldap = $this->config->get_ldap_link();

        $br = $this->getBranches();

        if(isset($br[$bb]) && preg_match("/d/",$this->ui->get_permissions($this->acl_base,"fai/faiManagement"))){
          $name = $br[$bb];

          $ldap->cd($bb);
          $ldap->recursive_remove();
          $ldap->cd(preg_replace('/,'.preg_quote(get_ou("faiManagement", "faiBaseRDN"), '/').'/i', ','.get_ou("application", "applicationRDN"), $bb));
          $ldap->recursive_remove();
          $ldap->cd(preg_replace('/,'.preg_quote(get_ou("faiManagement", "faiBaseRDN"), '/').'/i', ','.get_ou("mimetype", "mimetypeRDN"), $bb));
          $ldap->recursive_remove();

          /* Search for all groups with configured application menus.
             - First search all groups, to ensure that we only remove entries form whithin groups.
             - The search für menu configuration for the specified release and collect all those dns.
             - Remove entries
           */
          $release_ou = preg_replace("/".preg_quote(get_ou("faiManagement", "faiBaseRDN"), '/').".*$/i","",$bb);
          $ldap->cd($this->config->current['BASE']);
          $ldap->search("(objectClass=posixGroup)",array("dn"));

          /* Collect all group dns
           */
          $groups = array();
          while($attrs = $ldap->fetch()){
            $groups[] = $attrs['dn'];
          }


          /* Collect all group menu release dns that match the release we have removed
           */
          $dns = array();
          foreach($groups as $dn){
            $ldap->cd($dn);
            $ldap->search("(objectClass=FAIbranch)",array("dn"));
            while($attrs = $ldap->fetch()){
              if(preg_match("/^".preg_quote($release_ou, '/')."/",$attrs['dn'])){
                $dns[] = $attrs['dn'];
              }
            }
          }

          /* Finally remove collected release dns
           */
          foreach($dns as $dn){
            $ldap->cd($dn);
            $ldap->recursive_remove();
          }

          /* Post remove */
          $this->fai_release = preg_replace("/^[^,]*+,/","", $this->fai_release);

          $this->lock_name   = $name;
          $this->lock_dn     = $bb;
          $this->handle_post_events('remove');

          // Update list and release selector to point to a valid release again.
          $releases = $this->getReleaseStates($this->config,$this->fai_base);
          if(!isset($releases[$this->fai_release])){
            $this->fai_release = $this->fai_base;
          }

          $headpage = $this->getHeadpage();
          $headpage->setBase($this->fai_release);
          $headpage->update();

          $this->releaseSelector->setBases($releases);
          $this->releaseSelector->setBase($this->fai_release);

          new log("remove","fai/".get_class($this),$br[$bb],array(),"Release removed");
        }
      }
    }
  }


  /*! \brief   Initiates release creation 
   */
  function createBranch()
  { 
    if($this->config->get_cfg_value("faiManagement", "postcreate") == ""){ 
      msg_dialog::display(_("Configuration"), msgPool::cmdnotfound("POSTCREATE", get_class()), ERROR_DIALOG);
    }elseif(!preg_match("/c/",$this->ui->get_permissions($this->acl_base,"fai/faiManagement"))){
      msg_dialog::display(_("Permission error"), msgPool::permCreate(_("Branch")), ERROR_DIALOG);
    }else{
      $smarty = get_smarty();
      $this->dispNewBranch=true;
      $this->dispNewFreeze=false;
      $smarty->assign("iframe",false);
      if(isset($_POST['BranchName'])){
        $smarty->assign("BranchName", get_post('BranchName'));
      }else{
        $smarty->assign("BranchName","");
      }
      return($smarty->fetch(get_template_path('faiNewBranch.tpl', TRUE, dirname(__FILE__))));
    }
  }


  /*! \brief   Initiates release creation 
   */
  function createFreeze()
  {
    if($this->config->get_cfg_value("faiManagement", "postcreate") == ""){ 
      msg_dialog::display(_("Configuration"), msgPool::cmdnotfound("POSTCREATE", get_class()), ERROR_DIALOG);
    }elseif(!preg_match("/c/",$this->ui->get_permissions($this->acl_base,"fai/faiManagement"))){
      msg_dialog::display(_("Permission error"), msgPool::permCreate(_("Branch")), ERROR_DIALOG);
    }else{
      $smarty = get_smarty();
      $this->dispNewFreeze=true;
      $this->dispNewBranch=false;
      $smarty->assign("iframe",false);
      if(isset($_POST['BranchName'])){
        $smarty->assign("BranchName", get_post('BranchName'));
      }else{
        $smarty->assign("BranchName","");
      }
      return($smarty->fetch(get_template_path('faiNewBranch.tpl', TRUE, dirname(__FILE__))));
    }
  }


  /*! \brief   Creates a new branch
   */
  function PerformBranch()
  {
    if(!preg_match("/c/",$this->ui->get_permissions($this->acl_base,"fai/faiManagement"))){
      msg_dialog::display(_("Permission error"), msgPool::permCreate(_("Branch")), ERROR_DIALOG);
    }else{

      /* In order to see error messages we have to reset the error handler.
         Due to the exit();
       */
      restore_error_handler();

      $this->dispNewBranch = false;
      $this->dispNewFreeze = false;

      $LASTPOST = session::get('LASTPOST');
      $base = $LASTPOST['base'];
      $_POST  = session::get('LASTPOST');
      $name = $_POST['BranchName'];

      $type = $LASTPOST['type'];
      $ldap = $this->config->get_ldap_link();

      $baseToUse = $base;
      if($this->fai_release !=  $this->fai_base){
        $baseToUse = $this->fai_release;
      }

      /* Create new Release name to be able to set faidebianRelease for FAIpackageList */

      $CurrentReleases  = $this->getBranches();
      $NewReleaseName   = $name;
      if(isset($CurrentReleases[$this->fai_release])) {
        if($this->fai_release != $this->fai_base){
          $NewReleaseName = $CurrentReleases[$this->fai_release]."/".$name;
          $NewReleaseName = preg_replace("#\/#","/",$NewReleaseName);
        }else{
          $NewReleaseName   = $name;
        }
      }

      $targetReleaseDN = "ou=".$name.",".$baseToUse;

      $appsEnabled = class_available("application");
      $mimeEnabled = class_available("mimetype");
      if($appsEnabled){
          $appsrc = preg_replace("/".preg_quote(get_ou("faiManagement", "faiBaseRDN"), '/')."/i",
                  get_ou("application", "applicationRDN"),$baseToUse);
          $appdst = preg_replace("/".preg_quote(get_ou("faiManagement", "faiBaseRDN"), '/')."/i",
                  get_ou("application", "applicationRDN"),"ou=".$name.",".$baseToUse) ;
      }
      if($mimeEnabled){
          $mimesrc = preg_replace("/".preg_quote(get_ou("faiManagement", "faiBaseRDN"), '/')."/i",
                  get_ou("mimetype", "mimetypeRDN"),$baseToUse);
          $mimedst = preg_replace("/".preg_quote(get_ou("faiManagement", "faiBaseRDN"), '/')."/i",
                  get_ou("mimetype", "mimetypeRDN"),"ou=".$name.",".$baseToUse) ;
      }

      $useBases = array();
      $useBases[] = $baseToUse;
      if($appsEnabled) $useBases[] = $appsrc;
      if($mimeEnabled) $useBases[] = $mimesrc;

      /* Check if source depeartments exist */
      foreach($useBases as $dep){
        $ldap->cd($this->config->current['BASE']);
        $ldap->cat($dep);
        if(!$ldap->count()){
          $ldap->create_missing_trees($dep);
        }
      }

      /* Print header to have styles included */
      echo '  <!DOCTYPE HTML PUBLIC "-//W3C//DTD HTML 3.2//EN">
        <html>
        <head>
        <title></title>
        <style type="text/css">@import url("themes/default/style.css");</style>
        <script language="javascript" src="include/gosa.js" type="text/javascript"></script>
        </head>
        <body style="background: none;margin:3px;color:black">
        ';

      new log("create","fai/".get_class($this),$NewReleaseName,array(),"New $type created");

      /* Duplicate group application releases
       */
      FAI::copy_FAI_group_releases($CurrentReleases[$this->fai_release],$name,$type);

      /* Duplicate applications
       */
      if($appsEnabled){
          $ldap->cat($appsrc,array("dn")) ;
          if($ldap->count()){
              $ldap->cd ($appdst);
              $ldap->recursive_remove();
              FAI::copy_FAI_resource_recursive($appsrc,$appdst,$NewReleaseName,$type,true);
          }
      }

      /* Duplicate mime types
       */
      if($mimeEnabled){
          $ldap->cat($mimesrc,array("dn")) ;
          if($ldap->count()){
              $ldap->cd ($mimedst);
              $ldap->recursive_remove();
              FAI::copy_FAI_resource_recursive($mimesrc,$mimedst,$NewReleaseName,$type,true);
          }
      }

      $attr = array();
      $attr['objectClass'] = array("organizationalUnit","FAIbranch");
      $attr['ou']       = $name;
      $attr['FAIstate'] = $type;
      $ldap->cd($this->config->current['BASE']);
      $ldap->cd("ou=".$name.",".$baseToUse);
      $ldap->cat("ou=".$name.",".$baseToUse);
      if($ldap->count()){
        $ldap->modify($attr);
      }else{
        $ldap->add($attr);
      }

      /* Duplicate fai objects
       */
      //      $ldap->cd ("ou=".$name.",".$baseToUse);
      //      $ldap->recursive_remove();
      //      FAI::copy_FAI_resource_recursive($baseToUse,"ou=".$name.",".$baseToUse,$NewReleaseName,$type,true);

      echo "<div style='width:100%;text-align:right;'><form name='form' method='post' action='?plug=".$_GET['plug']."' target='_parent'>
        <br><input type='submit' name='CloseIFrame' value='"._("Continue")."'>
        <input type='hidden' name='php_c_check' value='1'>
        </form></div>";

      echo "<script language=\"javascript\" type=\"text/javascript\">scrollDown2();</script>" ;

      /* Print footer to have valid html */
      echo "</body></html>";

      $this->dispNewFreeze = false;

      /* Assign possible attributes */
      $this->lock_type  = $type;
      $this->lock_name  = $name;
      $this->lock_dn    = $baseToUse;
      $this->handle_post_events('add');

      /* Send daemon event to reload the fai release database
       */
      if(class_available("DaemonEvent") && class_available("gosaSupportDaemon")){
        $events = DaemonEvent::get_event_types(SYSTEM_EVENT | HIDDEN_EVENT);
        if(isset($events['TRIGGERED']['DaemonEvent_recreate_fai_release_db'])){
          $evt = $events['TRIGGERED']['DaemonEvent_recreate_fai_release_db'];
          $tmp = new $evt['CLASS_NAME']($this->config);
          $tmp->set_type(TRIGGERED_EVENT);
          $tmp->add_targets(array("GOSA"));
          $o_queue = new gosaSupportDaemon();
          if(!$o_queue->append($tmp)){
            msg_dialog::display(_("Service infrastructure"),msgPool::siError($o_queue->get_error()),ERROR_DIALOG);
          }
        }
      }else{
          if($this->config->boolValueIsTrue("core","developmentMode")){
              trigger_error("Unknown class DaemonEvent / gosaSupportDaemon");
              msg_dialog::display(_("Fatal error"),
                      "Daemon events called but classes where not accessible, DaemonEvent gosaSupportDaemon",
                      FATAL_ERROR_DIALOG);
          }
      }
    
      // Update list and release selector to point to the newly created release 
      $headpage = $this->getHeadpage();
      $this->fai_release = $targetReleaseDN;
      $headpage->setBase($this->fai_release);
      $headpage->update();
 
      $releases = $this->getReleaseStates($this->config,$this->fai_base);
      $this->releaseSelector->setBases($releases);
      $this->releaseSelector->setBase($this->fai_release);
      exit();
    }
  }


  /*! \brief   Creates a new branch, after a useable name was specified.
   */
  function saveBranch()
  {
    if($this->dispNewBranch){
      $type = "branch";
    }else{
      $type = "freeze";
    }

    /* Check branch name */
    $name = $_POST['BranchName'];
    $is_ok = true;
    $smarty = get_smarty();
    $smarty->assign("BranchName",$name);
    $base= $this->fai_base;

    /* Check used characters */
    if(!preg_match("/^[0-9a-z\.]*$/",$name)){
      msg_dialog::display(_("Error"), msgPool::invalid(_("Name"),$name,"/[0-9a-z\.]/"), ERROR_DIALOG);
      $is_ok = false;
    }

    // Check if this name is already in use 
    if(!$this->CheckNewBranchName($_POST['BranchName'],$this->fai_release)){
      msg_dialog::display(_("Error"), msgPool::duplicated(_("Name")), ERROR_DIALOG);
      $is_ok = false;
    }

    // Handle errors
    if(!$is_ok && $this->dispNewFreeze){
      return($this->createFreeze());
    }elseif(!$is_ok && $this->dispNewBranch){
      return($this->createBranch());
    }

    // Now create new release

    if(session::is_set('LASTPOST')){
      $LASTPOST = session::get('LASTPOST');
    }else{
      $LASTPOST = array();
    }
    $LASTPOST['base'] = $base;
    $LASTPOST['type'] = $type;
    $LASTPOST['BranchName'] = $name;
    session::set('LASTPOST',$LASTPOST);
    $smarty->assign("iframe", true);
    $smarty->assign("plugID", $_GET['plug']);
    $display  = $smarty->fetch(get_template_path('faiNewBranch.tpl', TRUE, dirname(__FILE__)));
    return($display);

  }



  /*! \brief   Returns a list of all releases for useable for drop down boxes.
   *                      ou=fai... /
   *              ou=siga,ou=fai... &nbsp; siga
   *         ou=1,ou=siga,ou=fai... &nbsp;&nbsp; 1
   */
  function getReleaseList($base = "", $prefix ="")
  {
    $list = array();
    if(empty($base)){
      $base = $this->fai_base;
      $list[$base] = "/";
    }

    $ldap = $this->config->get_ldap_link();
    $ldap->ls("(objectClass=FAIbranch)",$base,array("ou","FAIstate"));
    while($release = $ldap->fetch()){
      $list[$release['dn']] = $prefix.$release['ou'][0];
      $list = array_merge($list,$this->getReleaseList($release['dn'],$prefix."&nbsp; "));
    }
    return($list);
  }


  /*! \brief   Returns a list of all releases for the releaseSelector
   *                      ou=fai... 
   *              ou=siga,ou=fai... "branch"
   *         ou=1,ou=siga,ou=fai... "freeze"
   */
  static function getReleaseStates($config,$base = "", $fai_base="")
  {
    $list = array();
    if(empty($fai_base)){
      $fai_base = $base;
      $list[$base] = "";
    }

    $ldap = $config->get_ldap_link();
    $ldap->ls("(objectClass=FAIbranch)",$base,array("ou","FAIstate"));

    while($release = $ldap->fetch()){
      $list[$release['dn']] = _("Release");
      if(isset($release['FAIstate'][0]) && preg_match("/freeze/i",$release['FAIstate'][0])){
        $list[$release['dn']] = _("Freeze");
      }
      $list = array_merge($list,faiManagement::getReleaseStates($config,$release['dn'], $fai_base));
    }
    return($list);
  }


  /*! \brief   Returns a list of all releases with full release names
   *                      ou=fai... /
   *              ou=siga,ou=fai... siga
   *         ou=1,ou=siga,ou=fai... siga/1
   */
  function getBranches($base = false,$prefix = "")
  {
    $ret = array("/"=>$this->fai_base);
    $ldap = $this->config->get_ldap_link();
    if(!$base){
      $base = $this->fai_base;
    }
    $tmp = FAI::get_all_releases_from_base($base,true);
    foreach($tmp as $dn => $name){
      $ret[$name]=$dn;
    }
    ksort($ret);
    $ret = array_flip($ret);

    return ($ret);
  }


  /*! \brief   Detects object info like corresponding  tab,class,acl 
   *    e.g.    [0]   =  tabsPartition
   *            [1]   =  faiPartitionTable
   *            [2]   =  FAIPARTITIONTABS
   */
  function get_type($array)
  {
    if(!isset($array['objectClass'])) return(array());
    if(in_array_strict("FAIpartitionTable",$array['objectClass'])){
      return(array("tabsPartition","faiPartitionTable","FAIPARTITIONTABS"));
    }
    if(in_array_strict("FAIscript",$array['objectClass'])){
      return(array("tabsScript","faiScript","FAISCRIPTTABS"));
    }
    if(in_array_strict("FAItemplate",$array['objectClass'])){
      return(array("tabsTemplate","faiTemplate","FAITEMPLATETABS"));
    }
    if(in_array_strict("FAIhook",$array['objectClass'])){
      return(array("tabsHook","faiHook","FAIHOOKTABS"));
    }
    if(in_array_strict("FAIvariable",$array['objectClass'])){
      return(array("tabsVariable","faiVariable","FAIVARIABLETABS"));
    }
    if(in_array_strict("FAIprofile",$array['objectClass'])){
      return(array("tabsProfile","faiProfile","FAIPROFILETABS"));
    }
    if(in_array_strict("FAIpackageList",$array['objectClass'])){
      return(array("tabsPackage","faiPackage","FAIPACKAGETABS"));
    }
    return(array());
  }


  /*! \brief   Checks if the given string can be used as class name
   */
  static function check_class_name($oc,$name,$dn)
  {
    $base = FAI::get_release_dn($dn);

    if($oc == "FAIprofile"){
      $f = "";
      $ocs = array("FAIprofile","FAItemplate","FAIhook","FAIpartitionTable","FAIpackageList","FAIscript","FAIvariable");
      foreach($ocs as $oc){
        $f .= "(objectClass=".$oc.")";
      }
      $res  = FAI::get_all_objects_for_given_base($base,"(|".$f.")",TRUE);
    }else{
      $res  = FAI::get_all_objects_for_given_base($base,"(|(objectClass=FAIprofile)(objectClass=".$oc."))",TRUE);
    }
    $delete = array();
    $used   = array();
    foreach($res as $object){
      $used[$object['cn'][0]]= $object['cn'][0];
    }
    return($used);
  }


  /*! \brief   Checks if the given string can be used for a new release
   */
  function CheckNewBranchName($name,$base)
  {
    $f = $this->fai_release;
    if($name == ""){
      return(false);
    }elseif(in_array_strict($name,$this->getBranches($f))) {
      return(false);
    }elseif(tests::is_department_name_reserved($name,$base)){
      return(false);
    }
    return(true);
  }


  /* \brief This filter is used to display the locked image, depending
   *        on the FAIstate of the object
   */

  static function filterLockedItem($row, $classes)
  {
    if (!in_array_strict("FAKE_OC_IsBranch", $classes)) {
      return image('images/lists/locked.png');
    } else {
      return "";
    }
  }

  /*! \brief   This filter is used to display small icons for each listed object 
   *            instead of their typ names
   */
  static function filterProperties($row, $classes)
  {
    $objects = array(
        "FAIpartitionTable"  => array("IMG"=> "plugins/fai/images/fai_partitionTable.png",
          "NAME"=>_("Partition table"),"KZL"=> "PT", "VAR"=>"ShowPartitions"),
        "FAIpackageList"     => array("IMG"=> "plugins/fai/images/fai_packages.png",
          "NAME"=>_("Package list") ,  "KZL"=> "PL", "VAR"=>"ShowPackages"),
        "FAIscript"          => array("IMG"=> "plugins/fai/images/fai_script.png",
          "NAME"=>_("Scripts") ,       "KZL"=> "S",  "VAR"=>"ShowScripts"),
        "FAIvariable"        => array("IMG"=> "plugins/fai/images/fai_variable.png",
          "NAME"=>_("Variables") ,     "KZL"=> "V",  "VAR"=>"ShowVariables"),
        "FAIhook"            => array("IMG"=> "plugins/fai/images/fai_hook.png",
          "NAME"=>_("Hooks"),          "KZL"=> "H",  "VAR"=>"ShowHooks"),
        "FAIprofile"         => array("IMG"=> "plugins/fai/images/fai_profile.png",
          "NAME"=>_("Profile") ,       "KZL"=> "P",  "VAR"=>"ShowProfiles"),
        "FAItemplate"        => array("IMG"=> "plugins/fai/images/fai_template.png",
          "NAME"=>_("Templates") ,     "KZL"=> "T",  "VAR"=>"ShowTemplates"),
        "opsi_netboot"       => array("IMG"=> "plugins/opsi/images/netboot_package.png",
          "NAME"=>_("OPSI net boot product") ,     "KZL"=> "ON",  "VAR"=>"ShowOpsiNetboot"),
        "opsi_local"         => array("IMG"=> "plugins/opsi/images/local_package.png",
          "NAME"=>_("OPSI local boot product")   ,     "KZL"=> "OL",  "VAR"=>"ShowOpsiLocal"));

    $icon_list = "";
    foreach($objects as $type => $type_data){
      if(in_array_strict($type, $classes)){
        $icon_list .= image($type_data['IMG'], 'listing_edit'.$type.'_'.$row,$type_data['NAME']);
      }else{
        $icon_list .= image('images/empty.png');
      }
    }

    return $icon_list;
  }


  /*! \brief   Overridden render method of class mangement. 
   *            this allows us to add a release selection box.
   */
  function renderList()
  {
    $filter = $this->getFilter();
    $headpage = $this->getHeadpage();
    $this->releaseSelector->update();
    $this->fai_release = $this->releaseSelector->getBase();
    $headpage->setBase($this->fai_release);
    $headpage->update();
    $smarty = get_smarty();
    $r = $this->config->get_cfg_value("faiManagement", "postremove");
    $c = $this->config->get_cfg_value("faiManagement", "postcreate");
    $smarty->assign("allow_create", $c);
    $smarty->assign("allow_remove", $r);
    $smarty->assign("RELEASE", $this->releaseSelector->render());
    $display = $headpage->render();
    return($this->getHeader().$display);
  }


  /*! \brief   Convert POST and GET variables into actions.
   */
  function detectPostActions()
  {
    $action = management::detectPostActions();
    if(isset($_POST['remove_multiple'])) $action['action'] = "remove";
    if(isset($_POST['new_profile'])) $action['action'] = "new_profile";
    if(isset($_POST['new_template'])) $action['action'] = "new_template";
    if(isset($_POST['new_script'])) $action['action'] = "new_script";
    if(isset($_POST['new_hook'])) $action['action'] = "new_hook";
    if(isset($_POST['new_variable'])) $action['action'] = "new_variable";
    if(isset($_POST['new_package'])) $action['action'] = "new_package";
    if(isset($_POST['new_partition'])) $action['action'] = "new_partition";

    if(isset($_POST['save_properties'])) $action['action'] = "saveOpsiProperties";
    if(isset($_POST['cancel_properties'])) $action['action'] = "cancel";

    if(isset($_POST['edit_continue'])) $action['action'] = "newClassNameSelected";
    if(isset($_POST['edit_cancel'])) $action['action'] = "cancel";

    if(isset($_POST['faiGroupHandle_cancel'])) $action['action'] = "cancel";
    if(isset($_POST['CancelBranchName'])) $action['action'] = "cancel";
    if(isset($_POST['delete_branch_confirm'])) $action['action'] = "removeBranchConfirmed";
    if(isset($_GET['PerformBranch'])) $action['action'] = "PerformBranch";
    if(isset($_POST['UseBranchName'])) $action['action'] = "saveBranch";
    if(isset($_POST['faiGroupHandle_apply']))  $action['action'] = "editByGroup";
    if(isset($_GET['act']) && $_GET['act'] == "branch_branch")  $action['action'] = "createBranch";
    if(isset($_GET['act']) && $_GET['act'] == "freeze_branch")  $action['action'] = "createFreeze";
    if(isset($_GET['act']) && $_GET['act'] == "remove_branch")  $action['action'] = "removeBranch";
    return($action);
  }


  static function plInfo()
  {
      return (array(
                  "plShortName"   => _("FAI releases"),
                  "plDescription" => _("FAI release management"),
                  "plSelfModify"  => FALSE,
                  "plDepends"     => array(),
                  "plPriority"    => 0,
                  "plSection"     => array("administration"),
                  "plRequirements"=> array(
                      'onFailureDisablePlugin' => array(
                          'faiDiskEntry',
                          'faiHookEntry',
                          'faiHook',
                          'faiManagement',
                          'faiPackageConfiguration',
                          'faiPackage',
                          'faiPartition',
                          'faiPartitionTableEntry',
                          'faiPartitionTable',
                          'faiProfile',
                          'faiScriptEntry',
                          'faiScript',
                          'faiSummaryTab',
                          'faiTemplateEdit',
                          'faiTemplateEntry',
                          'faiTemplate',
                          'faiVariableEntry',
                          'faiVariable',
                          'servrepository'
                          ),
                      'ldapSchema' => array(
                          'FAIclass' => '>=2.7',
                          'FAIbranch' => '>=2.7',
                          'FAIdebconfInfo' => '>=2.7',
                          'FAIobject' => '>=2.7',
                          'FAIpackageList' => '>=2.7',
                          'FAItemplate' => '>=2.7',
                          'FAIscript' => '>=2.7',
                          'FAIvariable' => '>=2.7',
                          'FAIhook' => '>=2.7',
                          'FAItemplateEntry' => '>=2.7',
                          'FAIscriptEntry' => '>=2.7',
                          'FAIvariableEntry' => '>=2.7',
                          'FAIhookEntry' => '>=2.7',
                          'FAIpartitionTable' => '>=2.7',
                          'FAIpartitionDisk' => '>=2.7',
                          'FAIpartitionEntry' => '>=2.7',
                          'FAIprofile' => '>=2.7',
                          'FAIpartitionEntry' => '>=2.7',
                          'FAIrepository' => '>=2.7',
                          'FAIrepositoryServer' => '>=2.7'
                              )
                              ),
                  "plProperties" =>
                  array(
                      array(
                          "name"          => "faiBaseRDN",
                          "type"          => "rdn",
                          "default"       => "ou=fai,ou=configs,ou=systems,",
                          "description"   => _("RDN for FAI storage."),
                          "check"         => "gosaProperty::isRdn",
                          "migrate"       => "migrate_faiBaseRDN",
                          "group"         => "plugin",
                          "mandatory"     => TRUE
                          ),
                      array(
                          "name"          => "defaultFaiRelease",
                          "type"          => "dn",
                          "default"       => "",
                          "description"   => _("Name of the FAI default release."),
                          "check"         => "gosaProperty::isDn",
                          "migrate"       => "",
                          "group"         => "fai",
                          "mandatory"     => FALSE
                          )
                          ),

                          "plCategory"    => array("fai"=> array("description" => _("FAI"),"objectClass" => "FAIclass")),
                          "plProvidedAcls"=> array()));
  }
} 
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
