<?php

class faiTemplateEdit extends plugin
{
  /* attribute list for save action */
  var $config = "";
  var $dn     = "";
  var $value  = "";

  // Encoding identification. Allows to warn the user.
  var $enc_before_edit = "";
  var $enc_after_edit = "";
  var $write_protect = FALSE;
  var $mb_extension = TRUE;

  function faiTemplateEdit (&$config, $dn, $value)
  {
    plugin::plugin ($config, $dn);
    if(!function_exists("mb_detect_encoding")){
      $this->mb_extension = FALSE;
    }else{
      $this->value = $value;
      $this->enc_before_edit = mb_detect_encoding($value);
      if($this->enc_before_edit != "ASCII"){
        $this->write_protect = TRUE;
      }
    }
  }

  function execute()
  {
    /* Call parent execute */
    plugin::execute();

    /* We now split cn/FAItemplatePath to make things more clear... */
    $smarty     = get_smarty();
    $smarty->assign("templateValue",set_post(($this->value)));
    $smarty->assign("write_protect",$this->write_protect);
    $smarty->assign("mb_extension",$this->mb_extension);
    return($smarty->fetch(get_template_path('faiTemplateEdit.tpl', TRUE)));
  }

  /* Save data to object */
  function save_object()
  { 
    if(!$this->mb_extension) return;
    if(isset($_POST['templateValue']) && !$this->write_protect){
      $this->value = get_post('templateValue');
      $this->enc_after_edit = mb_detect_encoding($this->value);
    }
    if(isset($_POST['editAnyway'])) $this->write_protect = FALSE;
  }


  /* Check supplied data */
  function check()
  {
    $message = array();
    if($this->mb_extension && !$this->write_protect && $this->enc_after_edit !== $this->enc_before_edit ){
      $msg = sprintf(_("The file encoding has changed from '%s' to '%s'. Do you really want to save?"),
        "<i>".$this->enc_before_edit."</i>","<i>".$this->enc_after_edit."</i>");
      $message[] = $msg;
      $this->enc_before_edit = $this->enc_after_edit;
    }
    return($message);
  }

  function save()
  {
    return($this->value);
  }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
