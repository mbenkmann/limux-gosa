<?php

class askObjectGroup extends plugin
{
  /* attribute list for save action */
  var $ignore_account     = TRUE;
  var $attributes         = array();
  var $objectclasses      = array("whatever");

  var $objectClass        = false;
  var $ObjectGroup        = "";  
  var $ObjectGroups       = array();
  var $sw = "";
  var $parent;

  function askObjectGroup (&$config,$dn,$ui, $ObjectClass, $sw)
  {
    $this->ui = $ui;
    plugin::plugin ($config, $dn);
    $this->ObjectClass = $ObjectClass;
    $this->sw = $sw;

    /* Get a list of object groups */
    $ldap = $this->config->get_ldap_link();
    $ldap->cd($this->config->current['BASE']);
    $ldap->search('(objectClass=gosaGroupOfNames)', array("cn"));
    $tmp = array();
    while($attrs = $ldap->fetch()) {
      $tmp[$attrs['dn']] = $attrs['cn'][0];
    }
    asort($tmp, SORT_LOCALE_STRING);
    $this->ObjectGroups = array_merge(array("" => _("None")), $tmp);
  }

  function execute()
  {
    /* Call parent execute */
    plugin::execute();

    /* Fill templating stuff */
    $smarty = get_smarty();
    $display= "";

    /* Generate headline */
    $str = "";
    switch($this->ObjectClass) {
      case "worktabs":  $str =_("Create workstation"); break;
      default:  $str =_("Create");break;
    }
    $smarty->assign("headline", $str);

    $smarty->assign("ogroups", $this->ObjectGroups);
    $display.= $smarty->fetch(get_template_path('askObjectGroup.tpl', TRUE));
    return($display);
  }

  /* Get posts and set class name 
   */ 
  function save_object()
  {
    if (isset($_POST['SelectedOgroup']) && isset($_POST['edit_continue'])) {
      if ($this->ObjectGroup != _("None")) {
          $this->ObjectGroup = $_POST['SelectedOgroup'];
      }
    }
  }

  /* Return the class name */
  function save()
  {
    return($this->ObjectGroup);
  }

}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
