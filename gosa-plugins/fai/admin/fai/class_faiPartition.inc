<?php

class faiPartition extends plugin
{
  var $attributes = array("cn","description","FAIfsCreateOptions","FAIfsOptions","FAIfsTuneOptions",
      "FAIfsType","FAImountOptions","FAImountPoint","FAIpartitionNr","FAIpartitionFlags","FAIdiskType",
      "FAIpartitionSize","FAIpartitionType","FAIstate", "bootable","resize", "preserve", "preserveType",
      "encrypted");

  var $cn  = "";
  var $description  = "";
  var $FAIfsCreateOptions = "";
  var $FAIfsOptions = "";
  var $FAIfsTuneOptions = "";
  var $FAIfsType = "";
  var $FAImountOptions = "";
  var $FAImountPoint = "";
  var $FAIpartitionNr = "undefined"; // Initial value for new partitions
  var $FAIpartitionSize = "";
  var $FAIpartitionType = "";
  var $FAIstate = "";
  var $FAIpartitionFlags = "";
  
  var $FAIdiskType = "disk";

  // Size options.
  var $FAIpartitionSizeType = "fixed";
  var $sizeStart = 0;
  var $sizeStop  = 0;
  var $sizeStart_Type = "MB";
  var $sizeStop_Type  = "MB";
  
  //  Flags 
  var $bootable = false; 
  var $resize = false;
  var $encrypted = false;
  var $preserve = false;
  var $preserveType = "always";

  var $raidDevices = array();

  // Once we've exceeded the primary partition limit,
  //  hide the the 'primary' option from the select box.
  var $disablePrimary = FALSE; 

  function __construct($config, $object, $parent,$type)
  {

      $this->initTime = microtime(TRUE);
      $this->parent = $parent;
    $this->FAIdiskType = $type;

    // Check if we should be able to add primary partitions.
    if(!$object || $object['FAIpartitionType'] == "logical"){
      if($this->FAIdiskType == "disk"){
        $types = array('logical' => array(), 'primary' => array());
        foreach($this->parent->partitions as $key => $part){
          $types[$part['FAIpartitionType']][$part['FAIpartitionNr']] = 1;
        }
        if(count($types['logical']) && count($types['primary']) >= 3){
          $this->disablePrimary = TRUE;
        }elseif(count($types['logical']) >= 4){
          $this->disablePrimary = TRUE;
        }
      } 
    } 

    // Load attributes from existing partition 
    if($object){
      foreach($this->attributes as $attr){
        if(isset($object[$attr])){
          $this->$attr = $object[$attr];
        }
      }

      if($type == "disk" || $type =="lvm"){

        /* Prepare size attribute 
         * This attribute may represent a range, a fixed value 
         *  or a percentage.
         * fixed is just a number   * 500MB
         * range                    * 500MB-1TB
         * remaining                * -
         */
        // Fixed
        if(preg_match("/^[0-9]{1,}(KB|MB|GB|TB|PB|%|)$/",$this->FAIpartitionSize)){
          $this->sizeStart = preg_replace("/^([0-9]*)(KB|MB|GB|TB|PB|%)$/","\\1",$this->FAIpartitionSize);
          $this->sizeStart_Type = preg_replace("/^([0-9]*)(KB|MB|GB|TB|PB|%)$/","\\2",$this->FAIpartitionSize);
          $this->FAIpartitionSizeType = "fixed";
        }else

        // Dynamic range
        if(preg_match("/^[0-9]{1,}(KB|MB|GB|TB|PB|%|)-[0-9]{1,}(KB|MB|GB|TB|PB|%|)$/",$this->FAIpartitionSize)){
          $this->sizeStart = preg_replace("/^([0-9]*)(KB|MB|GB|TB|PB|%).*$/","\\1",$this->FAIpartitionSize);
          $this->sizeStart_Type = preg_replace("/^([0-9]*)(KB|MB|GB|TB|PB|%).*$/","\\2",$this->FAIpartitionSize);
          $this->sizeStop = preg_replace("/^[^\-]*\-([0-9]*)(KB|MB|GB|TB|PB|%)$/","\\1",$this->FAIpartitionSize);
          $this->sizeStop_Type = preg_replace("/^[^\-]*\-([0-9]*)(KB|MB|GB|TB|PB|%)$/","\\2",$this->FAIpartitionSize);
          $this->FAIpartitionSizeType = "dynamic";
        }else

        // Dynamic range
        if(preg_match("/^(0|)\-$/",$this->FAIpartitionSize)){
          $this->FAIpartitionSizeType = "remaining";
        }

        // Ensure that we've a valid value type selected.
        if(!preg_match("/(KB|MB|GB|TB|PB|%)/",$this->sizeStart_Type)){
          $this->sizeStart_Type = "MB";
        }
        if(!preg_match("/(KB|MB|GB|TB|PB|%)/",$this->sizeStop_Type)){
          $this->sizeStop_Type = "MB";
        }

        /* Check for encrypted partitions
         */
        if(preg_match("/:encrypt$/",$this->FAImountPoint)){
          $this->FAImountPoint = preg_replace("/:encrypt/","",$this->FAImountPoint);
          $this->encrypted = TRUE;
        }
    
      }elseif($type == "raid"){
   
        // Extract raid devices out of the partition size attribute. 
        $usedDisks = explode(",",$this->FAIpartitionSize);
        foreach($usedDisks as $disk){
          $name = preg_replace("/:.*$/","",$disk);
          $spare = preg_match("/:spare/",$disk);
          $missing = preg_match("/:missing/",$disk);
          if(!empty($name)){
            $this->raidDevices[$name] = 
              array(
                  "name" => $name, 
                  "spare" => $spare, 
                  "missing" => $missing);
          }
        }
      }
    }

    // Create statistic table entry
    stats::log('plugin', $class = get_class($this), $category = array($this->acl_category),  $action = 'open',
            $amount = 1, $duration = (microtime(TRUE) - $this->initTime));

  }

  function execute()
  {
    plugin::execute();
    $smarty = get_smarty();

    // Remove partition
    if(isset($_POST['addPhysicalPartition']) && isset($_POST['physicalPartitionAdd'])){
      $name =get_post('physicalPartitionAdd');
      $this->raidDevices[$name] = array("name" => $name,"spare"=>false,"missing"=>false);     
    }

    // Create a list of selectable partition types
    if($this->FAIdiskType == "disk"){

      if($this->disablePrimary){
        $types  = array(
            "logical" => _("Logical"));
      }else{
        $types  = array(
            "primary" => _("Primary"),
            "logical" => _("Logical"));
      }

    }elseif($this->FAIdiskType == "raid"){
      $types  = array(
          "raid0" => _("RAID 0"),
          "raid1" => _("RAID 1"),
          "raid5" => _("RAID 5"),
          "raid6" => _("RAID 6"));
    }else{
      $types = "";
    }
     
    // Create a list of all size options
    $partitionSizeTypes  = array(
        "fixed"     => _("fixed"),
        "dynamic"     => _("dynamic"),
        "remaining" => _("remaining space")
        );

    // Create a list of all size options
    $sizeTypes  = array(
        "KB"      => _("KB"),
        "MB"      => _("MB"),
        "GB"      => _("GB"),
        "TB"      => _("TB"),
        "PB"      => _("PB"),
        "%"      => _("%")
        );

    // Preserve types 
    $preserveTypes = array(
        "always" => _("always"),
        "reinstall" => _("reinstall"));

    // File system types.  
    $FAIfsTypes = array(
        "swap" => _("swap space"),
        "ext2" => "ext2",
        "ext3" => "ext3",
        "ext4" => "ext4",
        "reiserfs" => "reiser fs",
        "xfs" => "xfs",
        "btrfs" => "btrfs",
        "-" => "-");

    $smarty->assign("partitionTypes", $types);
    $smarty->assign("partitionSizeTypes", $partitionSizeTypes);
    $smarty->assign("FAIpartitionSizeType", $this->FAIpartitionSizeType);
    $smarty->assign("sizeTypes", $sizeTypes);

    $smarty->assign("sizeStart_Type", $this->sizeStart_Type);
    $smarty->assign("sizeStop_Type", $this->sizeStop_Type);
    $smarty->assign("sizeStart", set_post($this->sizeStart));
    $smarty->assign("sizeStop", set_post($this->sizeStop));

    $smarty->assign("preserveTypes", $preserveTypes);
    $smarty->assign("preserveType", set_post($this->preserveType));

    $smarty->assign("FAIfsTypes", $FAIfsTypes);
    $smarty->assign("cn", $this->cn);
    $smarty->assign("freeze", preg_match("/freeze/i",$this->parent->FAIstate));

    $smarty->assign("plist",$this->getRaidlist());
    $smarty->assign("physicalPartitionList",$this->getPartitionlist());
    $smarty->assign("disablePrimary", $this->disablePrimary);

    foreach($this->attributes as $attr){
      $smarty->assign($attr,set_post($this->$attr));
    }
    return($smarty->fetch(get_template_path("faiPartition.tpl", TRUE, dirname(__FILE__))));
  }

  
  /* Returns a list of all partitions that are useable 
   *  for raid arrays.
   */
  function getPartitionList()
  {
    $may = $used = array();  
    foreach($this->parent->parent->disks as $disk){

      // Skip ourselves 
      if($disk['cn'] == $this->parent->DISKcn) continue;

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

        // Collect all available disks 
        if($disk['FAIdiskType'] == "disk"){
          $name = $part['cn'];
          if(!isset($this->raidDevices[$name])){
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


  /* Creates a human readable list of all used partitions 
   *  of a raid device.
   */
  function getRaidList()
  {
    $disks = $this->parent->parent->disks;
    $objs = array();
    foreach($disks as $disk){
      if($disk['FAIdiskType'] != "raid"){
        foreach($disk['partitions'] as $id => $part){
          $objs[$part['cn']] = $part;
        }
      }
    }
  
    $list = array();
    foreach($this->raidDevices as $device){
      $str = $name = $device['name'];
      $str = str_pad($str,25," ");
      if(isset($objs[$device['name']]['FAIpartitionSize'])){
        $str .= _("Size").":&nbsp;";
        $str .= $objs[$device['name']]['FAIpartitionSize'];
      }
      $str = str_pad($str,60," ");
      $opt = "";
      if($device['spare']){
        $opt.= "&nbsp;"._("spare")."&nbsp;";
      }
      if($device['missing']){
        $opt.= "&nbsp;"._("missing")."&nbsp;";
      }
      if($opt){
        $str .= " &nbsp; "._("Options").": ".$opt;
      }
      $str = str_pad($str,80," ");
      $list[$name] = preg_replace("/ /","&nbsp;",$str);
    }
    return($list);
  }

  function save_object()
  {
    if(isset($_POST['faiPartition'])){
      foreach($this->attributes as $attr){
        if(isset($_POST[$attr])){
          $this->$attr = get_post($attr);
        }
      }
      foreach(array("FAIpartitionSizeType","sizeStart","sizeStop","sizeStart_Type","sizeStop_Type") as $attr){
        if(isset($_POST[$attr])){
          $this->$attr = get_post($attr);
        }
      }
      foreach(array("bootable","preserve","resize","encrypted") as $attr){
        if(isset($_POST[$attr])){
          $this->$attr = TRUE;
        }else{
          $this->$attr = FALSE;
        }
      }

      // Allow user defined partition names for lvm disks.
      if($this->FAIdiskType == "lvm" && isset($_POST['cn'])){
        $this->cn = get_post('cn');
      }

      // Remove partition
      if(isset($_POST['delPhysicalPartition']) && isset($_POST['physicalPartition'])){
        foreach($_POST['physicalPartition'] as $key){
          if(isset($this->raidDevices[$key])){
            unset($this->raidDevices[$key]);
          }
        }
      }

      // Toggle spare flag for partition entries
      if(isset($_POST['toggleSpare']) && isset($_POST['physicalPartition'])){
        $this->raidDevices[$_POST['physicalPartition'][0]]['spare'] = 
            !$this->raidDevices[$_POST['physicalPartition'][0]]['spare'];
      }

      // Toggle missing flag for partition entries
      if(isset($_POST['toggleMissing']) && isset($_POST['physicalPartition'])){
        $this->raidDevices[$_POST['physicalPartition'][0]]['missing'] = 
            !$this->raidDevices[$_POST['physicalPartition'][0]]['missing'];
      }
    } 
  }


  function check()
  {
    $msgs = plugin::check();
    
    // Check the given partition size.
    if($this->FAIdiskType == "disk" || $this->FAIdiskType == "lvm"){
      if($this->FAIpartitionSizeType == "fixed" || $this->FAIpartitionSizeType == "dynamic"){ 
        if(!is_numeric($this->sizeStart)){
          $msgs[] = msgPool::invalid(_("Partition size"),$this->sizeStart,"/[0-9]/i");
        }
      }
      if($this->FAIpartitionSizeType == "dynamic"){ 
        if(!is_numeric($this->sizeStop)){
          $msgs[] = msgPool::invalid(_("Partition size"),$this->sizeStop,"/[0-9]/i");
        }

        $mp = array(
                    "%"  => 1,
                    "KB" => pow(1024,0),
                    "MB" => pow(1024,1),
                    "GB" => pow(1024,2),
                    "TB" => pow(1024,3),
                    "PB" => pow(1024,4));
        $res1 = $this->sizeStart * $mp[$this->sizeStart_Type];
        $res2 = $this->sizeStop * $mp[$this->sizeStop_Type];
        if($res1 > $res2){
          $msgs[] = msgPool::toobig(_("Minimum partition size"), "'"._("Maximum partition size")."'");
        }
      }
    }

    // Add raid checks
    if($this->FAIdiskType == "raid"){
      if(count($this->raidDevices) < 2){
        $msgs[] = _("Raid arrays must contain at least two partitions!");
      }else if($this->FAIpartitionType == "raid0" && count($this->raidDevices) != 2){
        $msgs[] = _("Raid 0 arrays can only be realized with a combination of two partitions!");
      }
    }

    // check mount point 
    if($this->FAIfsType != "swap" && $this->FAIfsType != "-"){
      if(!preg_match("#^/#",$this->FAImountPoint)){
        $msgs[] = msgPool::invalid(_("Mount point"));
      }
    }

    return($msgs);
  }


  function save()
  {
    $ret = array();
    foreach($this->attributes as $attr){
      $ret[$attr] = $this->$attr;
    }

    // Save partition size
    if($this->FAIdiskType == "disk" || $this->FAIdiskType == "lvm"){ 
      switch($this->FAIpartitionSizeType){
        case 'fixed' : 
          $ret['FAIpartitionSize'] = $this->sizeStart.$this->sizeStart_Type;break; 
        case 'dynamic' : 
          $ret['FAIpartitionSize'] = $this->sizeStart.$this->sizeStart_Type."-".  
            $this->sizeStop.$this->sizeStop_Type;break; 
        case 'remaining' : 
          $ret['FAIpartitionSize'] = "0-";break; 
        default: trigger_error("Unknown partition size!");
      }

      // Add encryption flag to partition mount point
      if($this->encrypted){
        $ret['FAImountPoint'] .= ":encrypt";
      }

    }elseif($this->FAIdiskType == "raid"){

      // Save selected raid partitions in FAIpartitionSize
      $ret['FAIpartitionSize'] = "";
      foreach($this->raidDevices as $device){
        $ret['FAIpartitionSize'] .= $device['name'];
        if($device['spare']){
          $ret['FAIpartitionSize'] .= ":spare";
        }
        if($device['missing']){
          $ret['FAIpartitionSize'] .= ":missing";
        }
        $ret['FAIpartitionSize'] .= ",";
      }
      $ret['FAIpartitionSize'] = trim($ret['FAIpartitionSize'],",");
    }

    if($this->FAIfsType == "swap"){
      $ret['FAImountPoint'] = "swap";
    }
 
    return($ret);
  }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
