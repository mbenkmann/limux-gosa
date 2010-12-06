<?php

class faiDiskEntry extends plugin
{
  /* attribute list for save action */
  var $ignore_account= TRUE;
  var $attributes   = array("DISKcn","DISKdescription");
  var $UsedAttrs            = array("cn","description","FAIpartitionType",
      "FAIpartitionNr","FAIfsType","FAImountPoint","FAIpartitionSize",
      "FAIfsTuneOptions", "FAIfsCreateOptions", "FAImountOptions",
      "FAIfsOptions","FAIpartitionFlags","FAIdiskOption");

  var $objectclasses= array();

  var $DISKcn              = "";
  var $DISKdescription     = "";
  var $DISKFAIdiskOptions  = "";
  
  var $partitions          = array();
  var $is_edit             = false;
  var $old_cn              = "";
  var $fstabkey            = "device";
  var $disklabel           = "msdos";
  var $FAIstate            = "";
  var $FAIdiskType         = "disk";
  
  var $lvmDevices          = array();

  function faiDiskEntry (&$config, $dn= NULL,$parent,$disk,$type)
  {
    plugin::plugin ($config, $dn);
  
    // Set default attributes 
    $this->parent = $parent;
    $this->FAIdiskType = $type;

    // If disk is not empty, then we are going to edit 
    //  an existing disk, load disk info now. 
    if($disk){

      // Get devices used in volume base disks.
      if(isset($disk['FAIlvmDevice'])){
        $this->lvmDevices = $disk['FAIlvmDevice'];
      }

      // Load default attributes 
      $this->DISKcn          = $disk['cn'];
      $this->DISKdescription = $disk['description'];
      $this->is_edit         = true;
      $this->old_cn          = $disk['cn'];

      // Load partition info 
      foreach($disk['partitions'] as $values){

        $name = $values['FAIpartitionNr'];

        // Load partition attributes  
        $disk['partitions'][$name]['old_cn']= $disk['partitions'][$name]['cn'];
        $disk['partitions'][$name]['FAIdiskType']= $this->FAIdiskType;
        foreach($this->UsedAttrs as $attr){
          if(!isset($values[$attr])){
            $disk['partitions'][$name][$attr]="";  
          }
        }

        if (preg_match('/^_/', $disk['partitions'][$name]['FAIfsType'])){
          $disk['partitions'][$name]['FAIfsType']= 
            preg_replace('/^_/', '', $disk['partitions'][$name]['FAIfsType']);
        }
      }

      $this->partitions      = $disk['partitions'];

      /* Load FAIdiskOptions.
       * Some options are disk related and others are used for partitions.
       *  - fstabkey    -> disk
       *  - disklabel   -> disk
       *  - bootable    -> partition
       *  - preserve    -> partition
       *  - resize      -> partition
       */
      if (isset($disk['FAIdiskOption'])){
        foreach($disk['FAIdiskOption'] as $option) {

          // Get fstab key  
          if (preg_match("/fstabkey:(device|label|uuid)/", $option)){
            $this->fstabkey= preg_replace("/^.*fstabkey:(device|label|uuid).*$/", "$1", $option);
            continue;
          }

          // Get disk label
          if (preg_match("/disklabel:(msdos|gpt)/", $option)){
            $this->disklabel= preg_replace("/^.*disklabel:(msdos|gpt).*$/", "$1", $option);
            continue;
          }

          // Load bootable flag for partitions 
          if (preg_match("/^bootable:/", $option)){
            $bootable = explode(",", trim(preg_replace("/^bootable:/","",$option),","));
            foreach($bootable as $bootflag){
              if(isset($this->partitions[$bootflag])){
                $this->partitions[$bootflag]['bootable'] = TRUE;  
              }
            }
            continue;
          }

          // Load resize flag for partitions 
          if (preg_match("/^resize:/", $option)){
            $resize = explode(",", trim(preg_replace("/^resize:/","",$option),","));
            foreach($resize as $id){
              if(isset($this->partitions[$id])){
                $this->partitions[$id]['resize'] = TRUE;  
              }
            }
            continue;
          }

          // Load preserve_always flag for partitions 
          if (preg_match("/^preserve_always:/", $option)){
            $preserve = explode(",", trim(preg_replace("/^preserve_always:/","",$option),","));
            foreach($preserve as $presflag){
              if(isset($this->partitions[$presflag])){
                $this->partitions[$presflag]['preserve'] = TRUE;  
                $this->partitions[$presflag]['preserveType'] = 'always';  
              }
            }
            continue;
          }

          // Load preserve_reinstall flag for partitions 
          if (preg_match("/^preserve_reinstall:/", $option)){
            $preserve = explode(",", trim(preg_replace("/^preserve_reinstall:/","",$option),","));
            foreach($preserve as $presflag){
              if(isset($this->partitions[$bootflag])){
                $this->partitions[$presflag]['preserve'] = TRUE;  
                $this->partitions[$presflag]['preserveType'] = 'reinstall'; 
              } 
            }
            continue;
          }
        }
      } else {
        $this->fstabkey= "device";
      }
    }

    // Prepare lists
    $this->diskList = new sortableListing();
    $this->diskList->setDeleteable(true);
    $this->diskList->setEditable(true);
    $this->diskList->setWidth("100%");
    $this->diskList->setHeight("400px");
    $this->diskList->setColspecs(array('200px','*'));
    $this->diskList->setHeader(array("Name",_("Description"),_("Type"),_("Mount point"),_("Options")));
    $this->diskList->setDefaultSortColumn(1);
    $this->diskList->setAcl('rwcdm'); // All ACLs, we filter on our own here.
  }


  function execute()
  {
    /* Call parent execute */
    plugin::execute();

    // Fill templating stuff
    $smarty     = get_smarty();
    $s_action   = "";
    $s_entry    = "";
    $display    = "";

    // Add partition to lvm compilation.
    if(isset($_POST['addLvmPartition']) && isset($_POST['lvmPartitionAdd'])){
      $name = get_post('lvmPartitionAdd');
      $this->lvmDevices[$name] = $name;
    }

    // Remove partition from lvm compilation.
    if(isset($_POST['delLvmPartition']) && isset($_POST['physicalPartition'])){
      $names = get_post('physicalPartition');
      foreach($names as $name){
        if(isset($this->lvmDevices[$name])){
          unset($this->lvmDevices[$name]);
        }
      }
    }

    // Get list actions
    $this->diskList->save_object();
    $action = $this->diskList->getAction();
    if($action['action'] == 'edit'){
        $id = $this->diskList->getKey($action['targets'][0]);
        if(isset($this->partitions[$id])){
            $this->dialog = new faiPartition($this->config,$this->partitions[$id], $this,$this->FAIdiskType);
        }
    }
    if($action['action'] == 'delete'){
        $id = $this->diskList->getKey($action['targets'][0]);
        if(isset($this->partitions[$id])){
            $this->removePartition($id);
        }
    }

    /* Create a new partition for this disk.
     */
    if(isset($_POST['AddPartition']) && !preg_match("/freeze/i",$this->FAIstate)){
      $this->dialog = new faiPartition($this->config, array(), $this,$this->FAIdiskType);
    }

    /* Handle partition dialogs.
     */
    if($this->dialog instanceOf plugin && isset($_POST['PartitionCancel'])){
      $this->dialog = null;
    }
    if($this->dialog instanceOf plugin && isset($_POST['PartitionSave'])){
      $this->dialog->save_object();

      // Validate new partition
      $new_partition = $this->dialog->save(); 
      $msgs = $this->dialog->check();
      $msgs = array_merge($msgs,$this->check_disks($new_partition));

      if(!count($msgs)){
        $this->updatePartition($new_partition);
        $this->dialog = null;
      }else{
        msg_dialog::displayChecks($msgs);
      }
    }
    if($this->dialog instanceOf plugin){
      $this->dialog->save_object();
      return($this->dialog->execute());
    }

    // Assign checkbox related values.
    foreach($this->attributes as $attrs){
      $smarty->assign($attrs,set_post($this->$attrs));
      if($this->$attrs){
        $smarty->assign($attrs."CHK"," ");
      }else{
        $smarty->assign($attrs."CHK"," disabled ");
      }
    }

    // Assign disk info to smarty.
    $smarty->assign("setup", $this->generateParts());
    $smarty->assign("sub_object_is_createable",$this->acl_is_createable());
    $smarty->assign("freeze",preg_match("/freeze/i",$this->FAIstate));
    $smarty->assign("fstabkeys", array("device" => _("Device"), "label" => _("Label"), "uuid" => _("UUID")));
    $smarty->assign("disklabels", array("msdos" => "MSDOS", "gpt" => "GPT"));
    $smarty->assign("fstabkey", $this->fstabkey);
    $smarty->assign("disklabel", set_post($this->disklabel));
    $smarty->assign("FAIdiskType", $this->FAIdiskType);
    $smarty->assign("plist", $this->getPartitionList());
    $smarty->assign("physicalPartitionList", $this->getAvailablePartitions());

    // Assign partitions
    $tmp = $this->plInfo();
    $sacl = "";
    foreach($tmp['plProvidedAcls'] as $name => $translated){
      $acl = $this->getacl($name, preg_match("/freeze/i",$this->FAIstate));
      $smarty->assign($name."ACL",$acl);
    }
   
    $display.= $smarty->fetch(get_template_path('faiDiskEntry.tpl', TRUE));
    return($display);
  }


  /* Creates a human readable list, that contains all physical 
   *  devices that are used by the volume group.
   * This list will then be displayed in a html select box.
   * (lvm)
   */
  function getPartitionList()
  {
    /* Create a list of all available disks and partitions. 
     * This list will then be used to display detailed info.
     */
    $disks = $this->parent->disks;
    foreach($disks as $dname => $disk){

      // Skip currently edited disk 
      if($disk['cn'] == $this->old_cn) continue;
   
      // Add disk 
      $objs[$dname] = $disk;

      // Add disk partitions
      foreach($disk['partitions'] as $id => $part){
        $part['parentDisk'] = $disk;
        $objs[$part['cn']] = $part;
      }
    }

    // Attach current disk setup to the details list.
    $data = $this->save();
    $objs[$data['cn']] = $data;
    foreach($data['partitions'] as $part){
      $part['parentDisk'] = $data;
      $objs[$part['cn']] = $part;
    }

    // Walk through physical partition combinations and build up 
    //  user friendly list with partition details.
    $list = array();
    foreach($this->lvmDevices as $device){

      // We've a html select box here, add spaces for better readability
      $str = $device;
      $str = preg_replace("/ /","&nbsp;",str_pad($str,20," "));

      // Add disk/partition details.
      if(isset($objs[$device])){
        if(isset($objs[$device]['FAIpartitionSize'])){
          if($objs[$device]['parentDisk']['FAIdiskType'] == "raid"){
            $str .= _("Disks").":&nbsp;";
            $str .= preg_replace("/(:spare|:missing)/i","",$objs[$device]['FAIpartitionSize']);
          }else{
            $str .= _("Size").":&nbsp;";
            $str .= $objs[$device]['FAIpartitionSize'];
          } 
        }
      }
      $list[$device] = $str;
    }
    return($list);
  }

 
  /* Returns a list of available partitions that are useable in 
   *  lvm disk setups.
   */ 
  function getAvailablePartitions()
  {
    $may = $used = array();
    foreach($this->parent->disks as $disk){

      // Skip ourselves
      if($disk['cn'] == $this->DISKcn) continue;

      // Add partition from lvm combinations
      if($disk['FAIdiskType'] == "lvm"){
        $used = array_merge($used,$disk['FAIlvmDevice']);
      }

      foreach($disk['partitions'] as $key => $part){

        // Add disks of raid arrays, to the used list.
        if($disk['FAIdiskType'] == "raid"){
          foreach(explode(",",$part['FAIpartitionSize']) as $rDevice){
            $used[] = preg_replace("/:.*$/i","",$rDevice);
          }
        }

        // Collect all available partitions
        if($disk['FAIdiskType'] == "disk" || $disk['FAIdiskType'] == "raid"){
          $name = $part['cn'];
          if(!isset($this->lvmDevices[$name])){
            $may[] = $name;
          }
        }
      }
    }

    // Check which of the available disks are unused.
    $ret = array();
    foreach($may as $val){
      if(!in_array($val,$used)){
        $ret[$val] = $val;
      }
    }
    return($ret);
  }


  /* Remove the selected partition and shift the following partitions 
   *  to fill the gap.
   * Additionally update the partition numbers correspondingly.
   *  (Checks if the partition is in use, too)
   */
  function removePartition($id)
  {
    $start = false;

    /* Create a list of all partitions that are used in
     *  lvm or raid compilations.
     */
    $list = array();
    foreach($this->parent->disks as $dname => $disk){
      if($disk['FAIdiskType'] != "disk"){
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
    if(isset($list[$this->partitions[$id]['cn']])){
      $used = array();
      foreach($list[$this->partitions[$id]['cn']] as $disk){
        $used[$disk['cn']] = $disk['cn'];
      }
      $used = implode(",",$used);
      msg_dialog::display(_("Error"),
          sprintf(_("The disk cannot be deleted while it is used in the '%s' disk definition!"),
            $used), ERROR_DIALOG);
    }else{
      unset($this->partitions[$id]);
    }
  }

  
  function get_free_partition_number()
  {
    $used = array();
    foreach($this->partitions as $key => $part){
      $used[$key] = $part['FAIpartitionNr'];
    }
    $id = 1;
    while(in_array($id,$used) && $id < 16 ){
      $id++;
    }
    return($id);
  }



  /* Add or update a partition 
   */
  function updatePartition($part)
  {
    if(!isset($part['FAIpartitionNr']) || $part['FAIpartitionNr'] == "undefined"){
      $part['FAIpartitionNr'] = $this->get_free_partition_number();
    }

    /* Update the disk cn -       
     * Do NOT touch the partition 'cn' in case of lvm or raid devices. 
     */
    if($this->FAIdiskType == "disk"){
      $part['cn'] = $this->DISKcn.$part['FAIpartitionNr'];
    }

    /* Check if we've to update partition names of lvm compilations.
     */ 
    if($this->FAIdiskType == "lvm"){
      if(isset($this->partitions[$part['FAIpartitionNr']])){
        $old_cn = $this->partitions[$part['FAIpartitionNr']]['cn'];
        $new_cn = $part['cn'];
        if(isset($this->lvmDevices[$old_cn])){
          unset($this->lvmDevices[$old_cn]);
          $this->lvmDevices[$new_cn] = $new_cn;
        }
      }
    }

    /* Set raid names to md#
     */ 
    if($this->FAIdiskType == "raid"){
      $part['cn'] = 'md'.$part['FAIpartitionNr'];
    }

    $this->partitions[$part['FAIpartitionNr']] = $part;
  }


  /* This method generates the partition listing with all necessary info,
   *  depending on the disk type.
   */
  function generateParts()
  {
    $data = $lData = array();
    foreach($this->partitions as $key => $part){
    
      $cn       = $part['cn'];
      $desc     = $part['description'];
      $number   = $part['FAIpartitionNr'];
      $size     = $part['FAIpartitionSize'];
      $type     = $part['FAIpartitionType'];

      // Remove encryption info from the mount point.
      $mnt = $part['FAImountPoint'];
      if(preg_match("/:encrypt/", $mnt)){
        $mnt = preg_replace("/:encrypt/","",$mnt);
      }

      // create human readable strings out of the flags.
      $opt = "";
      if(isset($part['encrypted']) && $part['encrypted']){
        $opt.= "&nbsp;"._("encrypted").", ";
      }      
      if(isset($part['bootable']) && $part['bootable']){
        $opt.= "&nbsp;"._("boot able").", ";
      }      
      if(isset($part['preserve']) && $part['preserve']){
        $opt.= "&nbsp;"._("preserve").":&nbsp;".$part['preserveType'].", ";
      }     

      // Combine options to a single table col. 
      $opt = "<i>".preg_replace('/, $/',"",$opt)."&nbsp;</i>";

      // Build up info table, depending on the disk type. 
      $data[$key]=$key;
      if($this->FAIdiskType == "lvm"){
        $lData[$key]=array('data' => array($cn,$desc,"",$mnt,$opt,$size));
      }elseif($this->FAIdiskType == "raid"){
        $raid = $part['FAIpartitionType']." (".$part['FAIpartitionSize'].")";
        $lData[$key]=array('data' => array($cn,$desc,$raid,$mnt,$opt));
      }else{
        $lData[$key]=array('data' => array("",$desc,$type,$mnt,$opt,$size));
      }
    }
    $this->diskList->setListData($data,$lData);
    $acl = "rwcdm";
    if(preg_match("/freeze/", $this->FAIstate)) $acl = "r";
    $this->diskList->setAcl($acl);;
    $this->diskList->update();
    return($this->diskList->render());
  }


  function save()
  {
    $tmp = array();
    $tmp['cn']          = $this->DISKcn;

    /* Attach partitions. 
     * And prepare names and numbers.
     */
    foreach($this->partitions as $key=>$val) {
      $this->partitions[$key]['FAIpartitionNr']=$key;
      if($this->FAIdiskType == "disk"){
        $this->partitions[$key]['cn'] = $this->DISKcn.$key;
      }elseif($this->FAIdiskType == "lvm"){
        $this->partitions[$key]['FAIpartitionType'] = 'lvm';
      }
    }

    $tmp['description'] = $this->DISKdescription;
    $tmp['partitions']  = $this->partitions;
    $tmp['FAIdiskType'] = $this->FAIdiskType;

    // Add lvm devices if available.
    $tmp['FAIlvmDevice'] = array();
    foreach($this->lvmDevices as $dev){
      $tmp['FAIlvmDevice'][] = $dev;
    } 

    /* Assemble flags */
    $tmp['FAIdiskOption'] = array("fstabkey:".$this->fstabkey, "disklabel:".$this->disklabel);

    /* If hdd name has changed, tell partitionTable to rename it */
    if(($this->is_edit)&&($this->old_cn != $this->DISKcn)){
      $tmp['rename']['from']  = $this->old_cn;
      $tmp['rename']['to']    = $this->DISKcn;
    }

    // Build up disk options 
    $bootable = "";
    $resize = "";
    $preserve_always = "";
    $preserve_reinstall = "";

    /* Assemble boot, resize and preserve flags 
     */
    foreach($tmp['partitions'] as $id => $part){
      if(isset($part['bootable']) && $part['bootable']){
        $bootable .= $id.",";
      }
      if(isset($part['resize']) && $part['resize']){
        $resize .= $id.",";
      }
      if(isset($part['preserve']) && $part['preserve']){
        if($part['preserveType'] == "always"){
          $preserve_always .= $id.",";
        }else{
          $preserve_reinstall .= $id.",";
        }
      }

      // Unset non valid attributes 
      foreach(array("bootable","encrypted","preserve","preserveType","resize","FAIdiskType") as $attr){
        if(isset($tmp['partitions'][$id][$attr])){
          unset($tmp['partitions'][$id][$attr]);
        }
      }
    }    

    /* Assembe disk flags
     */
    if(!empty($bootable)){
      $tmp['FAIdiskOption'][] = "bootable:".trim($bootable,",");
    }
    if(!empty($resize)){
      $tmp['FAIdiskOption'][] = "resize:".trim($resize,",");
    }
    if(!empty($preserve_always)){
      $tmp['FAIdiskOption'][] = "preserve_always:".trim($preserve_always,",");
    }
    if(!empty($preserve_reinstall)){
      $tmp['FAIdiskOption'][] = "preserve_reinstall:".trim($preserve_reinstall,",");
    }

    return($tmp);
  }


  /* Save data to object */
  function save_object()
  {
    if((isset($_POST['TableEntryFrameSubmitted'])) && !preg_match("/freeze/", $this->FAIstate) ){
      plugin::save_object();

      // Save posted disk label and fstab key
      if (isset($_POST['disklabel']) && preg_match("/^(msdos|gpt)$/", $_POST['disklabel'])){
        $this->disklabel= get_post('disklabel');
      }
      if (isset($_POST['fstabkey']) && preg_match("/^(device|label|uuid)$/", $_POST['fstabkey'])){
        $this->fstabkey= get_post('fstabkey');
      }
    }
  }


  /* Check supplied data */
  function check()
  {
    /* Call common method to give check the hook */
    $message= plugin::check();
  
    /* Check for an empty disk name */
    $d = trim($this->DISKcn);
    if($d == "" ){
      $message[] = msgPool::required(_("Name"));
    }
    if(preg_match("/[^a-z0-9_\-]/i",$d)){
      $message[] = msgPool::invalid(_("Name"),$d,"/[a-z0-9_\-]/i");
    }

    return ($message);
  }


  /* Checks the disk combinations.
   * 
   */  
  function check_disks($disk_to_add = array())
  {
    $msgs = array();

    /* Check 'disk' combinations. 
     *  - There can be four primary partitions.
     *  - If there is at least one 'logical' partition, then there can be only 
     *     three 'primary' partitions.
     */    
    if($this->FAIdiskType == "disk"){
     
      $types = array('logical' => array(), 'primary' => array());
      $types[$disk_to_add['FAIpartitionType']][$disk_to_add['FAIpartitionNr']] = 1;
      foreach($this->partitions as $key => $part){
        $types[$part['FAIpartitionType']][$part['FAIpartitionNr']] = 1;
      }

      // There can only be four primary partitions per disk - without any logical partition.
      if(count($types['logical']) == 0){
        if(count($types['primary']) > 4){
          $msgs[] = _("You have more than four primary partition table entries in your configuration, please check your configuration twice.");
        }
      }else{
        if(count($types['primary']) > 3){
          $msgs[] = _("You cannot have more than three primary partition while using logical partitions, please check your configuration twice.");
        }
      }
    }

    return($msgs);
  }  


  /* Return plugin informations for acl handling */
  static function plInfo()
  {
    return (array(
          "plShortName" => _("Partition table entry"),
          "plDescription" => _("FAI partition table entry"),
          "plSelfModify"  => FALSE,
          "plDepends"     => array(),
          "plPriority"    => 27,
          "plSection"     => array("administration"),
          "plCategory"    => array("fai"),
          "plProvidedAcls" => array(
            "DISKcn"           => _("Name"),
            "DISKdescription"  => _("Description"),
            "DISKFAIdiskOption"  => _("Disk options"),
            "FAIpartitionType"  => _("Partition type"),
            "FAIpartitionNr"    => _("Partition no."),
            "FAIfsType"         => _("File system type"),
            "FAImountPoint"     => _("Mount point"),
            "FAIpartitionSize"  => _("Partition size"),
            "FAImountOptions"   => _("Mount options"),
            "FAIfsOptions"      => _("File system options"),
            "FAIpartitionFlags" => _("Partition flags"))
          ));
  }
 
}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
