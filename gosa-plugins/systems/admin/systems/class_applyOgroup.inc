<?php

class applyObjectGroup extends SelectDeviceType
{
  
  var $is_incoming = FALSE;      // If set to a TRUE value device type can be
                                 // chosen
  var $reinstall_allowed = TRUE; // If set to TRUE a checkbox will be shown to
                                 // opt-in for system reinstallation after save
  var $always_inherit = FALSE;   // If set to TRUE no inheritance checkbox will
                                 // be shown
  var $template_action = "none";
  var $PreviousType = NULL;

  function execute()
  {
    /* Call parent execute */
    plugin::execute();

    /* Fill templating stuff */
    $smarty = $this->prepare_execute();
    get_smarty();
    $display= "";

    if ($this->ObjectGroup == "none" || ($this->PreviousType != $this->SystemType)) {
      $smarty->assign("apply_disabled", TRUE);
    } else {
      $smarty->assign("apply_disabled", FALSE);
    }

    $SystemTypeName = $this->SystemTypes[$this->SystemType];
    $smarty->assign("SystemTypeName", $SystemTypeName);
    $smarty->assign("is_incoming", $this->is_incoming);
    $smarty->assign("reinstall_allowed", $this->reinstall_allowed);
    $smarty->assign("always_inherit", $this->always_inherit);
    $smarty->assign("devicetype_selection_tpl", get_template_path("devicetype_selection.tpl",TRUE,dirname(__FILE__)));
    $smarty->assign("ogroup_selection_tpl", get_template_path("ogroup_selection.tpl",TRUE,dirname(__FILE__)));
    $smarty->assign("template_actions", array(
      "none" => _("None"),
      "trigger_reinstall" => _("Trigger Reinstall"),
      "plan_reinstall" => _("Schedule Reinstall")));

    $smarty->assign("template_action", set_post($this->template_action));

    $display.= $smarty->fetch(get_template_path('ApplyObjectGroup.tpl', TRUE));
    return($display);
  }

  function save_object()
  {
    if (isset($_POST['SystemType'])) {
      $this->PreviousType = $this->SystemType;
    }
    if (isset($_POST['template_action'])) {
      $this->template_action = get_post('template_action');
    }
    SelectDeviceType::save_object(); 
  }

  /* Return the class name */
  function save()
  {
    return($this->ObjectGroup);
  }

}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
