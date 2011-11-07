<?php

class NewDebconfConfiguration extends plugin
{
  /* attribute list for save action */
  var $ignore_account     = TRUE;
  var $attributes         = array();
  var $objectclasses      = array("whatever");

  var $objectClass        = false;
  var $DebconfConfiguration = array();
  var $packages = array();
  var $variable_types = array(
                            "boolean" => "boolean",
                            "multiselect" => "multiselect",
                            "password" => "password",
                            "select" => "select",
                            "string" => "string",
                          );
  var $parent;

  function NewDebconfConfiguration (&$config, $dn, &$parent)
  {
    plugin::plugin ($config, $dn);

    /* Get list of packages */
    $this->packages = $parent->packages;
    /* Add d-i as special exception */
    $this->packages['d-i'] = 'd-i';
    ksort($this->packages);

    $this->parent = $parent;
  }

  function execute()
  {
    /* Call parent execute */
    plugin::execute();

    /* Fill templating stuff */
    $smarty = get_smarty();
    $display= "";

    $ldap = $this->config->get_ldap_link();
    /* Get package names */

    /* Pre-Fill input fields (needed for save_object if input fields are missing) */
    $variable = "";
    $variable_type = "";
    $content = "";
    $package = "";
    if (isset($this->DebconfConfiguration['Package'])) {
      $package = $this->DebconfConfiguration['Package'];
    }
    if (isset($this->DebconfConfiguration['FAIvariable'])) {
      $variable = $this->DebconfConfiguration['FAIvariable'];
    }
    if (isset($this->DebconfConfiguration['FAIvariableType'])) {
      $variable_type = $this->DebconfConfiguration['FAIvariableType'];
    }

    if (isset($this->DebconfConfiguration['FAIvariableContent'])){
      $content = $this->DebconfConfiguration['FAIvariableContent'];
    }
    $smarty->assign("package", $package);
    $smarty->assign("variable", $variable);
    $smarty->assign("variable_type", $variable_type);
    $smarty->assign("content", $content);
    

    /* Assign packages and variable types */
    $smarty->assign("variable_types", $this->variable_types);
    $smarty->assign("packages", $this->packages);

    /* Fetch template and display */
    $display.= $smarty->fetch(get_template_path('NewDebconfConfiguration.tpl', TRUE));
    return($display);
  }

  /* Get posts and set class name 
   */ 
  function save_object()
  {
    if (isset($_POST['save_AddDebconf'])) {
      foreach (array("FAIvariableType", "FAIvariableContent", "FAIvariable", "Package") as $attr) 
      {
        if (isset($_POST[$attr])) {
          $this->DebconfConfiguration[$attr] = get_post($attr);
        }
      }

      /* Normalize boolean values */
      if($this->DebconfConfiguration['FAIvariableType'] == 'boolean') {
        $this->DebconfConfiguration['FAIvariableContent'] = $this->parent->normalize_bool($this->DebconfConfiguration['FAIvariableContent']);
      }
    }
  }

  /* Check given class name 
   */
  function check()
  {
    /* Call common method to give check the hook */
    $message= plugin::check();

    $package = $this->DebconfConfiguration['Package'];
    if (!isset($this->DebconfConfiguration['FAIvariable']) || empty($this->DebconfConfiguration['FAIvariable'])) {
      $message[] = msgPool::required("Variable");
    }

    if ($this->DebconfConfiguration['FAIvariableType'] == 'boolean') {
      if (!empty($this->DebconfConfiguration['FAIvariableContent'])) {
        $value = strtolower($this->DebconfConfiguration['FAIvariableContent']);
      }
      else {
        $value = "";
      }
      if (!$this->parent->normalize_bool($value, TRUE)) {
        $message[] = sprintf(_("The specified value '%s' for the variable type boolean is invalid.<br><i>Allowed values: true, false, 1, 0.</i>"), $this->DebconfConfiguration['FAIvariableContent']);
      }
        
    }
    
    if (isset($this->DebconfConfiguration['FAIvariable'])){
        $name = $this->DebconfConfiguration['FAIvariable'];
        if (isset($this->parent->DebconfConfigurations[$package][$name])) {
            $message[] = msgPool::duplicated('Variable');
        }

        if (isset($this->parent->DebconfDefaults[$package][$name])) {
            $message[] = msgPool::duplicated('Variable');
        }
    }


    return ($message);
  }


  /* Return the class name */
  function save()
  {
    return($this->DebconfConfiguration);
  }

}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
