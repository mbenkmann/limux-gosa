<?php

class faiPartitionTable extends plugin
{
  /* attribute list for save action */
  var $ignore_account   = TRUE;
  var $attributes       = array("cn","description", "FAIpartitionMethod");
  var $objectclasses    = array("top","FAIclass","FAIpartitionTable");

  var $subAttributes      = array("cn","description");
  var $subPartAttributes  = array("cn","FAIpartitionNr","FAIpartitionSize","FAImountPoint","FAIfsType","FAIpartitionType","FAImountOptions","FAIfsOptions","FAIpartitionFlags","description","FAIfsCreateOptions","FAIfsTuneOptions","FAIfsOptions","FAIpartitionFlags","FAIlvmDevice");

  var $sub64coded = array();
  var $subBinary = array();

  /* Specific attributes */
  var $cn                 = "";       // The class name for this object
  var $description        = "";       // The description for this set of partitions
  var $FAIpartitionMethod = "";       // "setup-storage" or not assigned
  var $disks              = array();  // All defined Disks 
  var $is_dialog          = false;    // specifies which buttons will be shown to save or abort

  var $FAIstate           = "";
  var $ui;

  var $view_logged      = FALSE;

  function faiPartitionTable (&$config, $dn= NULL)
  {
    /* Load Attributes */
    plugin::plugin ($config, $dn);

    /* If "dn==new" we try to create a new entry
     * Else we must read all objects from ldap which belong to this entry.
     */
    $this->ui = get_userinfo();
    if($dn != "new"){
      $this->dn =$dn;

      /* Read all leaf objects of this object (For FAIscript this would be FAIscriptEntry)
       */
      $res = FAI::get_all_objects_for_given_base($this->dn,"(&(objectClass=FAIclass)(objectClass=FAIpartitionDisk))");
      foreach($res as $obj){

        /* Skip not relevant objects */
        if(!preg_match("/".preg_quote($this->dn, '/')."$/i",$obj['dn'])) continue;
        $objects = array();
        $objects['description']  = "";

        // Transform disk type into image later...
        if (!isset($obj['FAIdiskType'])){
          $objects['FAIdiskType']        = "old";
        } else {
          $objects['FAIdiskType']        = $obj['FAIdiskType'];
        }

        // Get disk options, without 'count' index. 
        $objects['FAIdiskOption'] = array();
        if (isset($obj['FAIdiskOption'])){
          for($i=0;$i<$obj['FAIdiskOption']['count'];$i++){ 
            $objects['FAIdiskOption'][] = $obj['FAIdiskOption'][$i];
          }
        }

        // Transform potential lvm information
        if (isset($obj['FAIlvmDevice'])){
          for($i=0;$i<$obj['FAIlvmDevice']['count'];$i++){
            $name = $obj['FAIlvmDevice'][$i]; 
            $objects['FAIlvmDevice'][$name] = $name;
          }
        }

        $objects['dn']          = $obj['dn'];
        $objects                = $this->get_object_attributes($objects,$this->subAttributes);
        $this->disks[$objects['cn']] = $objects;
        $this->disks[$objects['cn']]['partitions'] = array();
      }

      /* read all partitions for each disk 
       */
      foreach($this->disks as $name => $disk){
        $res = FAI::get_all_objects_for_given_base($disk['dn'],"(&(objectClass=FAIclass)(objectClass=FAIpartitionEntry))");
        foreach($res as $obj){

          /* Skip not relevant objects */
          if(!preg_match("/".preg_quote($disk['dn'], '/')."$/i",$obj['dn'])) continue;

          $objects = array();
          $objects['dn']          = $obj['dn'];
          $objects                = $this->get_object_attributes($objects,$this->subPartAttributes);
          unset($objects['dn']);;
          $this->disks[$name]['partitions'][$objects['FAIpartitionNr']] = $objects;
        }
      }
    }

    /* Keep track of disk changes, by comparing the initial and resulting 
     *  disk setup.
     */
    $this->initial_disks = $this->disks;

    $this->is_new = FALSE;
    if($this->dn == "new"){
      $this->is_new =TRUE;
    }
    ksort($this->disks);

    // Prepare lists
    $this->diskList = new sortableListing();
    $this->diskList->setDeleteable(true);
    $this->diskList->setEditable(true);
    $this->diskList->setWidth("100%");
    $this->diskList->setHeight("400px");
    $this->diskList->setColspecs(array('20px','*','*','40px'));
    $this->diskList->setHeader(array(("?"),_("Name"),_("Description"),"#"));
    $this->diskList->setDefaultSortColumn(1);

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
 
    /* Add Disk to this Partitionset
     * This code adds a new HDD to the disks 
     * A new Dialog will be opened 
     */
    if((isset($_POST['AddDisk']) || isset($_POST['AddRaid']) || isset($_POST['AddVolgroup'])) && 
        !preg_match("/freeze/i",$this->FAIstate)){
      $usedDiskNames =array();
      foreach($this->disks as $key=>$disk){
        $usedDiskNames[]= $key;
      }
      if ($this->FAIpartitionMethod == "setup-storage") {
        if(isset($_POST['AddDisk'])) $type = "disk";
        if(isset($_POST['AddRaid'])) $type = "raid";
        if(isset($_POST['AddVolgroup'])) $type = "lvm";
        $this->dialog = new faiDiskEntry($this->config,$this->dn,$this, array(),$type); 
      } else {
        $this->dialog = new faiPartitionTableEntry($this->config,$this->dn,$this); 
      }

      $this->dialog->set_acl_base($this->acl_base_for_current_object($this->dn));
      $this->dialog->set_acl_category("fai");
      $this->dialog->FAIstate = $this->FAIstate;


      $this->is_dialog = true;
    }

    /* Edit disk.
     * Open dialog which allows us to edit the selected entry 
     */    

    if($this->dn != "new"){
      set_object_info($this->dn);
    }

    /* Edit entries via GET */
    $s_action = "";
    $s_entry = "";
    if(isset($_GET['act']) && isset($_GET['id'])){
      if($_GET['act'] == "edit" && isset($this->disks[$_GET['id']])){
        $s_entry= $_GET['id'];
        $s_action= "edit";
      }
    }

    $this->diskList->save_object();
    $action = $this->diskList->getAction();
    if($action['action'] == 'edit'){
        $s_entry = $this->diskList->getKey($action['targets'][0]);
        $s_action ="edit";
    }
    if($action['action'] == 'delete'){
        $s_entry = $this->diskList->getKey($action['targets'][0]);
        $s_action ="remove";
    }

    /* Disk remove was requested. 
     * Now check if the disk is still in use, in this case 
     *  display a warning message and abort the removal. 
     * If the disk is not used anymore, then remove it. 
     */
    if($s_action == "remove"){
      if(isset($this->disks[$s_entry])){

        /* Create a list of all partitions that are used in 
         *  lvm or raid compilations. 
         */
        $list = array();
        foreach($this->disks as $dname => $disk){
          if($disk['FAIdiskType'] != "disk" && $dname != $s_entry){
            if($disk['FAIdiskType'] == "lvm"){
              foreach($disk['FAIlvmDevice'] as $partname){
                $list[preg_replace("/:.*$/","",$partname)][] = $disk;
              }
            }
            foreach($disk['partitions'] as $partkey => $part){
              if($disk['FAIdiskType'] == "raid"){
                foreach(explode(",",$part['FAIpartitionSize']) as $partname){
                  $list[preg_replace("/:.*$/","",$partname)][] = $disk;
                }
              }
            }  
          }
        }

        /* Now that we've a list of all partition references, lets check if
         *  one of the partitions we are going to remove is still in use.
         */
        $used = array();
        foreach($this->disks[$s_entry]['partitions'] as $part){
          if(isset($list[$part['cn']])){    
            foreach($list[$part['cn']] as $disk){
              $used[$disk['cn']] = $disk['cn'];
            }
          }
        }

        /* Skip removal while disk is in use. 
         */
        if(count($used)){
          $used = implode(",",$used);
          msg_dialog::display(_("Error"), 
              sprintf(_("The disk cannot be deleted while it is used in the '%s' disk definition!"), 
                $used), ERROR_DIALOG);
        }else{

          /* Everything is ok, we can remove the disk now.
           */
          unset($this->disks[$s_entry]);
        } 
      } 
    }

    

    if($s_action == "edit"){

      /* Set object info string, which will be displayed in plugin info line */ 
      if(isset($this->disks[$s_entry]['dn'])){
        set_object_info($this->disks[$s_entry]['dn']);
        $dn = $this->disks[$s_entry]['dn'];
      }else{
        set_object_info("");
        $dn = "new";
      }

      $type ="old"; 
      if(isset($this->disks[$s_entry]['FAIdiskType'])){
        $type = $this->disks[$s_entry]['FAIdiskType'];
      }
      if(in_array($type,array('raid','lvm','disk'))){
        $this->dialog = new faiDiskEntry(
            $this->config,$this->dn,$this,$this->disks[$s_entry], 
            $this->disks[$s_entry]['FAIdiskType']); 

      }else{
        $this->dialog = new faiPartitionTableEntry(
            $this->config,$this->dn,$this,$this->disks[$s_entry]); 
      }
      if($this->dialog){
        $this->dialog->set_acl_base($this->acl_base_for_current_object($dn));
        $this->dialog->set_acl_category("fai");
        $this->dialog->FAIstate = $this->FAIstate;
        $this->is_dialog = true;
      }
    }

    /* Edit aborted, close dialog, without saving anything
     */
    if(isset($_POST['CancelDisk'])){
      unset($this->dialog);
      $this->dialog = FALSE;
      $this->is_dialog=false;
    }

    /* Dialog saved
     * Save given data from Dialog, if no error is occurred
     */
    if(isset($_POST['SaveDisk'])){

      if (!preg_match("/freeze/i", $this->FAIstate)){
        $this->dialog->save_object();
        if(count($this->dialog->check())){
          foreach($this->dialog->check() as $msg){
            msg_dialog::display(_("Error"), $msg, ERROR_DIALOG);
          }
        }else{
          $disk = $this->dialog->save();
          if(isset($disk['rename'])){
            unset($this->disks[$disk['rename']['from']]);
            $disk['cn']= $disk['rename']['to'];
          }

          $this->disks[$disk['cn']]=$disk; 
          unset($this->dialog);
          $this->dialog = FALSE;
          $this->is_dialog=false;
          ksort($this->disks);
        }
      }else{
        $this->dialog = FALSE;
        $this->is_dialog=false;
      }
    }

    /* Display dialog if one is defined
     */
    if(is_object($this->dialog)){
      $this->dialog->save_object();
      return($this->dialog->execute());
    }

    /* Assign all attributes to smarty engine
     */
    foreach($this->attributes as $attrs){
      $smarty->assign($attrs,set_post($this->$attrs));
      if($this->$attrs){
        $smarty->assign($attrs."CHK"," ");
      }else{
        $smarty->assign($attrs."CHK"," disabled ");
      }
    }
   
    $dn = $this->acl_base_for_current_object($this->dn);
    $smarty->assign("sub_object_is_addable",
        preg_match("/c/",$this->ui->get_permissions($dn,"fai/faiPartitionTableEntry")) &&
        !preg_match("/freeze/",$this->FAIstate));
    $smarty->assign("sub_object_is_removeable",
        preg_match("/d/",$this->ui->get_permissions($dn,"fai/faiPartitionTableEntry")) &&
        !preg_match("/freeze/",$this->FAIstate));

    $tmp = $this->plInfo();
    foreach($tmp['plProvidedAcls'] as $name => $translated){
      $smarty->assign($name."ACL",$this->getacl($name,preg_match("/freeze/i",$this->FAIstate)));
    }

    /* Assign mode */
    if ($this->FAIpartitionMethod == ""){
      $smarty->assign("mode", "");
      $smarty->assign("storage_mode", "disabled");
    } else {
      $smarty->assign("mode", "checked");
      $smarty->assign("storage_mode", "");
    }
    if (!count($this->disks)) {
      $smarty->assign("lockmode", "");
    } else {
      $smarty->assign("lockmode", "disabled");
    }
    if (isset($this->disks['raid'])){
      $smarty->assign("addraid", "disabled");
    } else {
      $smarty->assign("addraid", "");
    }

    $dn = $this->acl_base_for_current_object($this->dn);
    $acl = $this->ui->get_permissions($dn,"fai/faiPartitionTableEntry");
    $this->diskList->setAcl($acl);
    $data = $lData = array();
    foreach($this->disks as $key => $disk){
      $act = "";

      $dn = "new";
      if(isset($obj['dn'])){
        $dn = $obj['dn'];
      }
      $dn = $this->acl_base_for_current_object($dn);
      $acl = $this->ui->get_permissions($dn,"fai/faiPartitionTableEntry");
      if(preg_match("/(r|w)/",$acl)) {
        $cnt= count($disk['partitions']);
        $types= array(
            "old"  => "plugins/fai/images/fai_partitionTable.png", 
            "disk" => "plugins/fai/images/fai_partitionTable.png",
            "raid" => "plugins/fai/images/raid.png", "lvm" => "plugins/ogroups/images/list_ogroup.png");
        $type = isset($disk['FAIdiskType'])?$types[$disk['FAIdiskType']]:$types['old'];
        $data[$key] = $key;
        $lData[$key] = array('data' => array(image($type),$key,$disk['description'],$cnt));
      }
    }
    $this->diskList->setListData($data,$lData);
    $this->diskList->update();
    $smarty->assign("Entry_listing",$this->diskList->render());

    $display.= $smarty->fetch(get_template_path('faiPartitionTable.tpl', TRUE));
    return($display);
  }


  function getUsedDiskNames()
  {
    $ret = array();
    foreach($this->disks as $disk){
      $ret[] = $disk['cn'];
    }
    return($ret);  
  }


  /* Delete me, and all my subtrees
   */
  function remove_from_parent()
  {
    $ldap = $this->config->get_ldap_link();
    $ldap->cd ($this->dn);

    $release = $this->parent->parent->fai_release;
    $use_dn = preg_replace("/".preg_quote(FAI::get_release_dn($this->dn), '/')."/i", $release, $this->dn);

    FAI::prepare_to_save_FAI_object($use_dn,array(),true);
    new log("remove","fai/".get_class($this),$use_dn,$this->attributes);   
    foreach($this->initial_disks as $disk){
      $disk_dn = "cn=".$disk['cn'].",".$this->dn;
      $use_dn = preg_replace("/".preg_quote(FAI::get_release_dn($this->dn), '/')."/i",$release, $disk_dn);
      FAI::prepare_to_save_FAI_object($use_dn,array(),true);
      foreach($disk['partitions'] as $key => $partition){    
        $partition_dn= "FAIpartitionNr=".$partition['FAIpartitionNr'].",".$disk_dn;      
        $use_dn = preg_replace("/".preg_quote(FAI::get_release_dn($this->dn), '/')."/i", $release, $partition_dn);
        FAI::prepare_to_save_FAI_object($use_dn,array(),true);
      }
    }
  }


  /* Save data to object 
   */
  function save_object()
  {
    if(isset($_POST['FAIpartitionTablePosted'])){
      if (preg_match("/freeze/", $this->FAIstate)) return;
      plugin::save_object();
      if(!count($this->disks)){
        if(isset($_POST['mode'])){
          $this->FAIpartitionMethod = "setup-storage";
        }else{
          $this->FAIpartitionMethod = "";
        }
      }
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
      $new_dn= 'cn='.$this->cn.",".get_ou("faiPartitionTable", "faiPartitionRDN").get_ou("faiManagement", "faiBaseRDN").$release;
      $res = faiManagement::check_class_name("FAIpartitionTable",$this->cn,$new_dn);
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

    /* Save current settings.
     * 1 : We must save the partition table, with its description and cn 
     * 2 : Append Disk with cn and  description.
     * 3 : Save partitions for each disk
     */  

    $ldap = $this->config->get_ldap_link();

    FAI::prepare_to_save_FAI_object($this->dn,$this->attrs);

    if($this->initially_was_account){
      new log("modify","fai/".get_class($this),$this->dn,$this->attributes);
      @DEBUG (DEBUG_FAI, __LINE__, __FUNCTION__, __FILE__,$this->dn , "Saving disk: ");
    }else{
      new log("create","fai/".get_class($this),$this->dn,$this->attributes);
      @DEBUG (DEBUG_FAI, __LINE__, __FUNCTION__, __FILE__,$this->dn , "Adding disk: ");
    }
 
    // Prepare disks to be saved - The 'status' attribute is added here.
    $this->prepareDiskToBeSave(); 
 
    /* Sort entries, because we must delete entries with status="delete" first */
    $order = array();
    foreach($this->disks as $key => $disk){
      if($disk['status'] == "delete"){
        $order[$key] = $disk;
      }
    }
    foreach($this->disks as $key => $disk){
      if($disk['status'] != "delete"){
        $order[$key] = $disk;
      }
    }


    /* Append all disks to ldap */
    foreach($order as $cn=>$disk){

      $disk_dn                    = "cn=".$disk['cn'].",".$this->dn;
      $short_dn                   = "cn=".$disk['cn'].",...";
      $disk_attrs['cn']           =  $disk['cn'];
      $disk_attrs['objectClass']  =  array("top","FAIclass","FAIpartitionDisk");
  

      if($disk['status']=="new"){
        $ldap->cat($disk_dn,array("objectClass"));
        if($ldap->count()){
          $disk['status']="edited";
        }
      }

      foreach(array("description","FAIdiskType","FAIdiskOption","FAIlvmDevice") as $attr){
        if($disk['status'] == "new"){
          if(isset($disk_attrs[$attr])) unset($disk_attrs[$attr]);
          if(isset($disk[$attr]) && !empty($disk[$attr])){
            if(is_array($disk[$attr])){
              $disk_attrs[$attr] = array_values($disk[$attr]);
            }else{
              $disk_attrs[$attr] = $disk[$attr];
            }
          }
        }else{
          if(isset($disk[$attr]) && !empty($disk[$attr])){
            if(is_array($disk[$attr])){
              $disk_attrs[$attr] = array_values($disk[$attr]);
            }else{
              $disk_attrs[$attr] = $disk[$attr];
            }
          }else{
            $disk_attrs[$attr] = array();
          }
        }
      }


      /* Tag object */
      $this->tag_attrs($disk_attrs, $disk_dn, $this->gosaUnitTag);

      if($disk['status'] == "delete"){
        @DEBUG (DEBUG_FAI, __LINE__, __FUNCTION__, __FILE__,$short_dn , "Removing disk: ");
        FAI::prepare_to_save_FAI_object($disk_dn,array(),true);
        $this->handle_post_events("remove");
        unset($this->disks[$cn]);
      }elseif($disk['status'] == "edited"){
        @DEBUG (DEBUG_FAI, __LINE__, __FUNCTION__, __FILE__,$short_dn , "Updating disk: ");
        FAI::prepare_to_save_FAI_object($disk_dn,$disk_attrs);
        $this->handle_post_events("modify");
      }elseif($disk['status']=="new"){
        @DEBUG (DEBUG_FAI, __LINE__, __FUNCTION__, __FILE__,$short_dn , "Adding disk: ");
        FAI::prepare_to_save_FAI_object($disk_dn,$disk_attrs);
        $this->handle_post_events("add");
      }

      if($disk['status']!="delete")

      /* Add all partitions */
      foreach($disk['partitions'] as $pkey => $partition){
        $partition_attrs = array();

        foreach($partition as $key => $value){
          if(!empty($value)){
            $partition_attrs[$key]=$value;        
          }else{
            unset($partition_attrs[$key]);        
          }
        }

        $partition_dn= "FAIpartitionNr=".$partition_attrs['FAIpartitionNr'].",".$disk_dn;     
        $short_dn= "FAIpartitionNr=".$partition_attrs['FAIpartitionNr'].",...";
     
        $partition_attrs['objectClass']= array("top","FAIclass","FAIpartitionEntry");
        
        unset($partition_attrs['status']);
        unset($partition_attrs['old_cn']);

        if($partition['status']=="new"){
          $ldap->cat($partition_dn,array("objectClass"));
          if($ldap->count()){
            $partition['status']="edited";
          }
        }

        if((!isset($partition['FAImountPoint']))||(empty($partition['FAImountPoint']))){
          $partition_attrs['FAImountPoint']="swap";
        }

        /* Tag object */
        $this->tag_attrs($partition_attrs, $partition_dn, $this->gosaUnitTag);

        
        if($partition['status'] == "delete"){
          @DEBUG (DEBUG_FAI, __LINE__, __FUNCTION__, __FILE__,$short_dn , "Removing partition: ");
          FAI::prepare_to_save_FAI_object($partition_dn,array(),true);
          $this->handle_post_events("remove");
          unset($this->disks[$cn]['partitions'][$pkey]);
        }elseif($partition['status'] == "edited"){
          @DEBUG (DEBUG_FAI, __LINE__, __FUNCTION__, __FILE__,$short_dn , "Updating partition: ");
          FAI::prepare_to_save_FAI_object($partition_dn,$partition_attrs);
          $this->handle_post_events("modify");
        }elseif($partition['status']=="new"){
          @DEBUG (DEBUG_FAI, __LINE__, __FUNCTION__, __FILE__,$short_dn , "Adding partition: ");
          FAI::prepare_to_save_FAI_object($partition_dn,$partition_attrs);
          $this->handle_post_events("add");
        }
        
        // We do not need the status flag any longer 
        if(isset($this->disks[$cn]['partitions'][$pkey]['status'])){
          unset($this->disks[$cn]['partitions'][$pkey]['status']);
        }
      }
      // We do not need the status flag any longer 
      if(isset($this->disks[$cn]['status'])){
        unset($this->disks[$cn]['status']);
      }
    }
    $this->initial_disks = $this->disks;
    $this->handle_post_events("add");
  }


  function prepareDiskToBeSave()
  {
    foreach($this->disks as $id => $disk){
      
      /* Correct FAIpartitionNr.
       * If we've only primary partition then set the partition numbers from  
       *  1 to 4, else set the primary from 1 to 3 and logical >= 5 
       * 
       */
      if(!isset($disk['partitions'])){
        $disk['partitions'] = array();
      }
      $newSetup = array();

      if($disk['FAIdiskType'] == "disk"){
        $primary = $logical = array();
        foreach($disk['partitions'] as $partid => $part){
          if($part['FAIpartitionType'] == "primary"){
            $primary[$partid] = $part;
          }elseif($part['FAIpartitionType'] == "logical"){
            $logical[$partid] = $part;
          }else{
            trigger_error("Fatal: unknown disk type? ".$part['FAIpartitionType']); 
          }
        }
        $cnt = 1;
        foreach($primary as $part){
          $part['FAIpartitionNr'] = $cnt;
          $part['cn'] = $disk['cn'].$cnt;
          $newSetup[$cnt] = $part;
          $cnt ++;
        } 
        $cnt = 5;
        foreach($logical as $part){
          $part['FAIpartitionNr'] = $cnt;
          $part['cn'] = $disk['cn'].$cnt;
          $newSetup[$cnt] = $part;
          $cnt ++;
        }
        $this->disks[$disk['cn']]['partitions'] = $newSetup; 
      }
    }

 
    # FAIpartitionNr have to be used as index for this->disks
    #  else the next operation will fail. 

 
    /* Check if there are disks to be removed, edited or added.
     * We compare the initial disk setup with the current setup and
     *  and add a status flag, which will then be used to perform the 
     *  correct action - add, edited, remove.
     */
    foreach($this->disks as $key => $disk){

      // - A complete NEW disk
      if(!isset($this->initial_disks[$disk['cn']])){
        $this->disks[$key]['status'] = "new";
        foreach($disk['partitions'] as $pkey => $part){
          $this->disks[$disk['cn']]['partitions'][$pkey]['status'] = "new";
        }
      }else{
    
        // - Disk was "EDITED" 
        $this->disks[$key]['status'] = "edited";
        foreach($disk['partitions'] as $pkey => $part){

          // - Check whether partition is "NEW" or "EDITED" 
          if(!isset($this->initial_disks[$key]['partitions'][$pkey])){
            $this->disks[$key]['partitions'][$pkey]['status'] = "new";
          }else{
            $this->disks[$key]['partitions'][$pkey]['status'] = "edited";
          }
        }
      }
    }
     
    /* Check which partitions havbe to be removed. 
     * (They intially existed, but are now gone.)
     */ 
    foreach($this->initial_disks as $ikey => $idisk){
      
      // - Complete disk was REMOVED.
      if(!isset($this->disks[$idisk['cn']])){
        $this->disks[$idisk['cn']] = $idisk;
        $this->disks[$idisk['cn']]['status'] = "delete";
        foreach($idisk['partitions'] as $pkey=>$part){
          $this->disks[$idisk['cn']]['partitions'][$pkey] = $part;
          $this->disks[$idisk['cn']]['partitions'][$pkey]["status"] = "delete";
        }
      }else{
        foreach($idisk['partitions'] as $pkey=>$part){
          if(!isset($this->disks[$idisk['cn']]['partitions'][$pkey])){
            $this->disks[$idisk['cn']]['partitions'][$pkey] = $part;
            $this->disks[$idisk['cn']]['partitions'][$pkey]["status"] = "delete";
          }
        }
      }
    }
  }
  

  function PrepareForCopyPaste($source)
  {
    plugin::PrepareForCopyPaste($source);

    /* Read all leaf objects of this object (For FAIscript this would be FAIscriptEntry)
     */

    /* To be sure to copy disks and partitions correctly, just create a 
     *  new PartitionTable object and use the 'disks' attribute 
     *  from this it. This is much easier and less code.
     */
    $obj = new faiPartitionTable($this->config, $source['dn']);
    $this->disks = $obj->disks;
  }


  /* Return plugin informations for acl handling */ 
  static function plInfo()
  {
    return (array( 
          "plShortName" => _("Partition table"),
          "plDescription" => _("FAI partition table"),
          "plSelfModify"  => FALSE,
          "plDepends"     => array(),
          "plPriority"    => 26,
          "plSection"     => array("administration"),
          "plCategory"    => array("fai"),
          "plProperties" =>
          array(
              array(
                  "name"          => "faiPartitionRDN",
                  "type"          => "rdn",
                  "default"       => "ou=disk,",
                  "description"   => _("RDN for FAI partition storage."),
                  "check"         => "gosaProperty::isRdn",
                  "migrate"       => "migrate_faiPartitionRDN",
                  "group"         => "plugin",
                  "mandatory"     => TRUE)),

          "plProvidedAcls" => array(
            "cn"                => _("Name")."&nbsp;("._("Read only").")",
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
