<?php
class logoffManagementDialog extends plugin
{
  var $pathTitle = "Logoff scripts";

  /* attribute list for save action */
  var $ignore_account       = TRUE;
  var $attributes           = array("LogoffName","LogoffPriority","LogoffLast","LogoffOverload","LogoffData","LogoffDescription");
  var $objectclasses        = array("whatever");
  var $use_existing         = false;  

  var $LogoffName        ="";  // Name for the LogoffScript
  var $LogoffPriority    ="";  // Priority defines the order in which the scripts will be processed 
  var $LogoffLast        ="";  // Is this script marked as the last, all following scripts will be skipped
  var $LogoffOverload    ="";  // If Overload is activated this script is overlaodable by a group script with same prio
  var $LogoffData        ="";  // The script itself
  var $LogoffDescription ="";
    
  var $nameIsEditable = true;

  var $real_LogoffName = "";

  function logoffManagementDialog (&$config, $dn= NULL,$data=false)
  {
    plugin::plugin ($config, $dn);
    
    if($data){
      $this->LogoffName        = $data['LogoffName'];
      $this->LogoffPriority    = $data['LogoffPriority'];
      $this->LogoffOverload    = $data['LogoffOverload'];
      $this->LogoffLast        = $data['LogoffLast'];
      $this->LogoffData        = $data['LogoffData'];
      $this->LogoffDescription = $data['LogoffDescription'];
      $this->nameIsEditable   = false;
      $this->real_LogoffName   = $data['LogoffName'];
    }
  }

  function execute()
  {
	/* Call parent execute */
	plugin::execute();

    $smarty= get_smarty();
    $display= "";

    if((isset($_POST['StartImport']))&&(isset($_FILES['importFile']))){
      $filename = gosa_file_name($_FILES['importFile']['tmp_name']);
      $this->LogoffData = file_get_contents($filename);
      @unlink($filename);
    }

    if(isset($_GET['getLogoffData'])){
      send_binary_content($this->LogoffData, $this->real_LogoffName);
    }

    /* Create download button*/
    if($this->dn != "new" && $this->LogoffData != ""){
      $smarty->assign("DownMe","<a href='?plug=".$_GET['plug']."&getLogoffData'>
          <img src='images/save.png' alt='"._("Download")."' title='"._("Download")."' border=0 class='center'>
        </a>");
    }else{
      $smarty->assign("DownMe","");
    }

    foreach($this->attributes as $attr){
      $smarty->assign($attr,$this->$attr);
      if($this->$attr){
        $smarty->assign($attr."CHK"," checked ");
      }else{
        $smarty->assign($attr."CHK","");
      }
    }
    $prios=array(1,2,3,4,5,6,7,8,9,10);
    $smarty->assign("LogoffPrioritys",$prios);
    $smarty->assign("LogoffPriorityKeys",$prios);

    if(!$this->nameIsEditable){
      $smarty->assign("LogoffNameACL"," disabled ");
    }else{
      $smarty->assign("LogoffNameACL","");
    }
  

    $display.= $smarty->fetch(get_template_path('logoffManagement.tpl', TRUE,dirname(__FILE__)));
    return($display);
  }

  function save_object()
  {
    if(isset($_POST['dialogissubmitted'])){
      foreach($this->attributes as $attr){

        if(!$this->nameIsEditable && $attr == 'LogoffName'){
          continue;
        }
        if(isset($_POST[$attr])){
          $this->$attr = stripslashes($_POST[$attr]); 
        }
      }

      foreach(array("LogoffLast","LogoffOverload") as $checkBoxes){
        if(isset($_POST[$checkBoxes])){
          $this->$checkBoxes = stripslashes($_POST[$checkBoxes]);
        }else{
          $this->$checkBoxes = "";
        }

      }
      
      if ($this->LogoffData) {
        $this->LogoffData = preg_replace('/\\r\\n/', "\n", $this->LogoffData);
      }
      
    }
  }

  function check()
  {
    /* Call common method to give check the hook */
    $message= plugin::check();
  
    if(preg_match("/[^a-zA-Z]/",$this->LogoffName)){
      $message[] = msgPool::invalid(_("Name"),$this->LogoffName,"/[a-zA-Z]/");
    }

    if(empty($this->LogoffName)){
      $message[] = msgPool::required(_("Script name"));
    }

    if(preg_match("/[^a-z0-9,\.-;:_\(\)!\? ]/i",$this->LogoffDescription)){
      $message[] = msgPool::required(_("Description"),$this->LogoffDescription,"/[a-z0-9,\.-;:_\(\)!\? ]/i");
    }

    return $message;
  }


  function save()
  {
    $a_return= array();
    foreach($this->attributes as $attr){
      $a_return[$attr]=$this->$attr;
    }  

    if(!$this->nameIsEditable){
      $a_return['LogoffName']=$this->real_LogoffName;
    }

    return($a_return); 
  }

}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
