<?php

class faiGroupHandle{


  private $FAI_group;
  private $mode     = "edit";  
  private $open     = TRUE;
  private $canceled = FALSE;

  public function __construct($FAI_group, $Action)
  {
    if(in_array($Action,array("edit","remove","copy","cut"))){
      $this->mode = $Action;
    }else{
      trigger_error("Wrong action type '".$Action."'.");
      exit();
    }

    /* Preset required values */
    $select_set = FALSE;
    foreach($FAI_group as $type => $data){
      $FAI_group[$type]['selected'] = FALSE;
      $FAI_group[$type]['freezed'] = isset($data['FAIstate']) && preg_match("/freeze/",$data['FAIstate']);
      if(!$select_set && ($this->mode == "cut" || $this->mode == "edit") && !$FAI_group[$type]['freezed']){
        $FAI_group[$type]['selected'] = TRUE;
        $select_set =TRUE;
      }
    }
    $this->FAI_group = $FAI_group;
  }

  function execute()
  {
    $smarty = get_smarty();
    $smarty->assign("FAI_group",$this->FAI_group);
    $smarty->assign("types",$this->get_types());
    $smarty->assign("mode",$this->mode);
    return($smarty->fetch(get_template_path("faiGroupHandle.tpl",TRUE,dirname(__FILE__))));
  }

  function save_object()
  {
    if(isset($_POST['faiGroupHandle'])){
      if(in_array($this->mode,array("copy","cut","remove"))){
        foreach($this->FAI_group as $key => $group){
          if(isset($_POST[$this->mode."_".$key])){
            $this->FAI_group[$key]['selected'] = TRUE;
          }else{
            $this->FAI_group[$key]['selected'] = FALSE;
          }
        } 
      }else{
        if(isset($_POST['edit_selected']) && isset($this->FAI_group[get_post('edit_selected')])){
          foreach($this->FAI_group as $key => $obj){
            $this->FAI_group[$key]['selected'] = FALSE;
          }
          $this->FAI_group[get_post('edit_selected')]['selected'] = TRUE;
        }
      }
      if(isset($_POST['faiGroupHandle_apply']) || isset($_POST['faiGroupHandle_cancel'])){
        $this->open = FALSE;
      }
      if(isset($_POST['faiGroupHandle_cancel'])){
        $this->canceled = TRUE;
      }
    }
  }

  function is_open()
  {
    return($this->open);
  }

  function is_canceled()
  {
    return($this->canceled);
  }
  
  function get_mode()
  {
    return($this->mode);
  }
  
  function get_selected()
  {
    $ret= array();
    foreach($this->FAI_group as $key => $obj){
      if($obj['selected']){
        $ret[$key] = $obj;
      }
    }
    return($ret);
  }

  function get_types()
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
    return($objects);
  }
}






// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
