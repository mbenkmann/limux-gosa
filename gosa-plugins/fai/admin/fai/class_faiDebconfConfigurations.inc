<?php

class faiDebconfConfigurations extends plugin
{
    /* attribute list for save action */
    var $ignore_account = TRUE;
    var $attributes     = array();
    var $objectclasses  = array();
    var $FAIstate = "";
    var $DebconfConfigurations = array();
    var $DebconfDefaults = array();
    var $DebconfAll = array();
    var $to_remove = array();
    var $removed = array();
    var $package_list = FALSE;
    var $parent = FALSE;
    var $packages = array();

    /* Some settings */
    var $query_gosa_si = TRUE;
    var $merge_defaults_into_view = TRUE;
    var $bool_mapping = array(
                            "0" => 'false',
                            "1" => 'true',
                            "true" => 'true',
                            "false" => 'false'
                          );

    /*! \brief  Configure all debconf configurations of a given package list
        @param  Object  $config GOsa configuration object;
        @param  String  The DN of the current package list.
     */
    function faiDebconfConfigurations (&$config, $dn= NULL, &$parent)
    {
      plugin::plugin ($config, $dn);

      /* Store some information we need from the parent object */
      $this->FAIstate = $parent->FAIstate;
      $this->package_list = $parent->cn;
      $this->parent = $parent;

      /* Get the list of configurations from the parent object */
      $this->DebconfConfigurations = $this->parent->ConfiguredPackages;

      /* Load packages */
      foreach(array_keys($this->parent->usedPackages) as $package) {
          $this->packages[$package] = $package;
      }
      ksort($this->packages);


      /* Determine and assemble release name */
      $release = $this->parent->parent->parent->fai_release;
      $tmp= preg_replace('/[,]*'.preg_quote(get_ou("faiManagement", 'faiBaseRDN'), '/').'.*$/', '', $release);
      $tmp= preg_replace('/ou=/', '', $tmp);
      $rev= array_reverse(explode(',', $tmp));
      $this->release= "/";
      foreach ($rev as $part){
        $this->release.= "/$part";
      }
      $this->release= preg_replace('#^[/]*#', '', $this->release);
   
      /* Get the list of debconf configuration for the given package list */
      $ldap = $config->get_ldap_link();

      /* Get the defaults from gosa-si */
      if ($this->query_gosa_si == TRUE) {
        $q = new gosaSupportDaemon();
        $ret = $q->FAI_get_packages($this->release,array("package","template"),$this->packages);
        if($q->is_error()){
            msg_dialog::display(_("Service infrastructure"),msgPool::siError($q->get_error()),ERROR_DIALOG);
        }
        else {
          foreach($ret as $res) {
            $package = $res['PACKAGE'];
            if (isset($res['TEMPLATE'])) {
              $template = base64_decode($res['TEMPLATE']);
              $debconf_handle = new debconf(preg_replace('/_.*$/', '', get_browser_language()));
              $debconf_handle->load_from_string($template);
              $templates = $debconf_handle->template;
              unset($debconf_handle);
              foreach($templates as $template) {
                $template['Save'] = FALSE;
                if (!isset($this->DebconfDefaults[$package][$template['Name']])) {
                  $this->DebconfDefaults[$package][$template['Name']] = $template;
                }
              } 
            }
          }
        }
      }

      // Prepare lists
      $this->ConfigurationsList = new sortableListing();
      $this->ConfigurationsList->setHeight(400);
      $this->ConfigurationsList->setDeleteable(false);
      $this->ConfigurationsList->setEditable(false);
      $this->ConfigurationsList->setWidth("100%");
      $this->ConfigurationsList->setColspecs(array('120px','*','100px','*', '50px'));
      $this->ConfigurationsList->setHeader(array(_("Package"), _("Variable"), _("Type"), _("Value"), ""));

    }


    /*! \brief Return a save toggle */
    function getSaveToggle ($identifier) {
      list($package, $name) = $this->getIdentifierComponents(base64_decode($identifier));

      /* We do not need a save toggle for manually entered parameters */
      if (isset($this->DebconfDefaults[$package][$name])) {
        $template = "
          <input type='hidden' name='save_%s' value='%s'>
          <input type='image' src='%s' width='18' name='toggle_%s_%s' value='%s' title='%s'>
          ";
        if (isset($this->DebconfConfigurations[$package][$name])) {
          if ($this->DebconfConfigurations[$package][$name]['Save'] == TRUE) {
            return(sprintf($template, $identifier, '0', 'images/save.png', 'nosave', $identifier, '0', _("Currently saved")));
          }
          else {
            return(sprintf($template, $identifier, '1', 'images/empty.png', 'save', $identifier, '1', _("Currently not saved")));
          };
        } else {
            /* Defaults w/o corresponding configuration entry are not saved by default
            * so give the user the chance to change that */
            return(sprintf($template, $identifier, '1', 'images/empty.png', 'save', $identifier, '1', _("Currently not saved")));
        }
      }
    } 
    /*! \brief  Returns a html interface, that allow to modify 
                 the package attributes.
        @return String  HTML content/ui for this plugin
     */
    function execute()
    {
      /* Call parent execute */
      plugin::execute();

      ksort($this->DebconfConfigurations);

      if(is_object($this->dialog)) {
        $this->dialog->save_object();
      $display = $this->dialog->execute();
      return($display);
    }

    /* Define some vars we will need in the following loop */
    $str_input = "<input type=text length=100 value=\"%s\" name='val_%s' %s>";

    /* Merge defaults into display */
    if ($this->merge_defaults_into_view) {
      $defaults = array();
      foreach($this->DebconfDefaults as $package => $package_data){
        foreach($package_data as $debconf_info) {
            $name = $debconf_info['Name'];
            $value = $debconf_info['Default'];
            $type = $debconf_info['Type'];
            if(!in_array($type,array("boolean","multiselect","password","select","string","text"))){
              continue;
            }
            $defaults[$package][$name] = array( 
                                                 'Name' => $name,
                                                 'Value' => $value, 
                                                 'Type' => $type,
                                                 'Save' => FALSE
                                         );
        }
      }
      $this->DebconfAll = $this->DebconfConfigurations;
      
      foreach($this->packages as $package) {
        if (isset($defaults[$package])) {
          if (isset($this->DebconfConfigurations[$package])) {
            $this->DebconfAll[$package] = array_merge($defaults[$package], $this->DebconfConfigurations[$package]);
          }else {
            $this->DebconfAll[$package] = $defaults[$package];
          }

        }

      }
    } else {
      $this->DebconfAll = $this->DebconfConfigurations;
    }
    ksort($this->DebconfAll);

    /* Build package list */
    $lData = $data = array();

    $defaults_count = count($this->DebconfDefaults);
    foreach($this->DebconfAll as $package => $package_data) {
      foreach($package_data as $debconf_info) {
        /* Do not show removed entries */
        if (isset($debconf_info['Status']) && $debconf_info['Status'] == 'removed') {
          continue;
        }

        /* Base64-encoded identifier for every package/debconf_data combination
         * is in the form of packge/Name e.g. locales/locales/locales_to_generate */
        $name = sprintf("%s/%s", $package, $debconf_info['Name']);
        $identifier = base64_encode($name);

        /* Define actions for this entry */ 
        $actions = "";
        if(!preg_match('/^freeze/', $this->FAIstate)) {
            /* Don't enable actions if no gosa-si defaults available */
            $show_reset = FALSE;
            $show_edit = FALSE;
            $show_blank = FALSE;
            if ($defaults_count != 0) {
                $name = $debconf_info['Name'];

                if (isset($this->DebconfDefaults[$package][$name])) {
                  $show_edit = TRUE;
                }
                else {
                  $show_delete = TRUE;
                }

                if (isset($this->DebconfConfigurations[$package][$name]) && isset($this->DebconfDefaults[$package][$name])) {
                  $val = $this->DebconfConfigurations[$package][$name]['Value'];
                  $default_val = $this->DebconfDefaults[$package][$name]['Default'];
                  /* Make values comparable */
                  if (is_array($val)) {
                    $val = "";
                  }
                  if ($val != $default_val) {
                    $show_reset = TRUE;
                  }
                  else {
                    $show_blank = TRUE;
                  }
                }elseif (!isset($this->DebconfConfigurations[$package][$name]) && isset($this->DebconfDefaults[$package][$name])) {
                  $show_blank = TRUE;
                }
            }

            if ($show_reset) {
                $actions .= image('images/lists/reload.png', "reset_".postEncode($name), _("Reset"));
            } elseif ($show_blank) {
                $actions .= $this->getSaveToggle($identifier);
            } else {
                $actions .= image('images/lists/trash.png', "reset_".postEncode($name), _("Delete"));
            }
            if ($show_edit) {
                $actions .= image('images/lists/edit.png', "edit_".postEncode($name), _("Edit"));
            }
        }

        /* Do not allow editing if FAIstate is freeze */
        $input_disabled = "";
        if (preg_match('/^freeze/', $this->FAIstate)) {
          $input_disabled = "disabled";
        }

        /* Pre-Fill input field with the defined value */
        $input_value = "";
        if (!empty($debconf_info['Value'])) {
          $input_value = htmlentities($debconf_info['Value']);
        }
        $input_field = sprintf("$str_input", $input_value, $identifier, $input_disabled);

        # Add list entry
        $data[$identifier] = $identifier;
        $lData[$identifier] = array( 'data' => array($package, $debconf_info['Name'], $debconf_info['Type'], $input_field, $actions) );
      }
	

    }
    $this->ConfigurationsList->setAcl($this->getacl("FAIpackage",preg_match('/^freeze/', $this->FAIstate)));
    $this->ConfigurationsList->setListData($data,$lData); 
    $this->ConfigurationsList->update();
	
    /* Fill templating stuff */
    $smarty     = get_smarty();
    $smarty->assign("divlist", $this->ConfigurationsList->render());
    $smarty->assign("headline", sprintf(_("Debconf information for package list '%s'"), $this->package_list));
    $smarty->assign("writable", $this->FAIstate != 'freeze');
    return($smarty->fetch(get_template_path('faiDebconfConfigurations.tpl', TRUE)));
  }


  function normalize_bool($bool_value, $check_mode=FALSE) {
    $bool_value = strtolower($bool_value);
    if(isset($this->bool_mapping[$bool_value])) {
      return $this->bool_mapping[$bool_value];
    } else {
      return ($check_mode) ? FALSE : $bool_value;
    }
  }

  /*! \brief Return the components of an identifier as an array of two elements */
  function getIdentifierComponents($identifier) {
    $identifier_components = explode('/', $identifier);
    $package = $identifier_components[0];
    $name = preg_replace("/^".$package."\//", "", $identifier, 1);
    return(array($package, $name));
  }

  function DeleteEntry($identifier) {
    list($package, $name) = $this->getIdentifierComponents($identifier);
    if (isset($this->DebconfConfigurations[$package][$name])) {
      $this->DebconfConfigurations[$package][$name]['Save'] = FALSE;
      if (!isset($this->DebconfDefaults[$package][$name])) {
        $this->DebconfConfigurations[$package][$name]['Status'] = "removed";
      } else {
        $this->DebconfConfigurations[$package][$name]['Value'] = $this->DebconfDefaults[$package][$name]['Default'];
      }
    }

    /* Check if the package entry is still justified */
    if (is_array($this->DebconfConfigurations[$package])) {
      if (!count($this->DebconfConfigurations[$package]) > 0) {
        unset($this->DebconfConfigurations[$package]);
      }
    }
  }

  function UpdateEntry($identifier, $newval="") {
    list($package, $name) = $this->getIdentifierComponents($identifier);
    if (!isset($this->DebconfConfigurations[$package][$name])) {
      $this->DebconfConfigurations[$package][$name] = $this->DebconfDefaults[$package][$name];
    }

    /* Normalize boolean values */
    if ($this->DebconfConfigurations[$package][$name]['Type'] == 'boolean') {
      $newval = $this->normalize_bool($newval);
    }

    /* If the value changed, toggle the save flag */
    $previous_value = "";
    if (isset($this->DebconfConfigurations[$package][$name]['Value'])) {
      $previous_value = $this->DebconfConfigurations[$package][$name]['Value'];
    }
    else {
      $previous_value = $this->DebconfConfigurations[$package][$name]['Default'];
    }
    if (is_array($previous_value)) {
      $previous_value = "";
    }

    if ($previous_value != $newval) {
      $this->DebconfConfigurations[$package][$name]['Save'] = TRUE;
    }

    $this->DebconfConfigurations[$package][$name]['Value'] = $newval;


    /* Set empty values to an array */
    if (isset($this->DebconfConfigurations[$package][$name]) && !$this->DebconfConfigurations[$package][$name]['Value']) {
        $this->DebconfConfigurations[$package][$name]['Value'] = array();
    }
  }

  /*!\brief Toggle the save status of a given identifier */
  function ToggleSave($identifier, $value) {
    list($package, $name) = $this->getIdentifierComponents($identifier);

    $save = ($value == 1) ? TRUE : FALSE;

    if (!isset($this->DebconfConfigurations[$package][$name])) {
      $this->DebconfConfigurations[$package][$name] = $this->DebconfDefaults[$package];
    }

    $this->DebconfConfigurations[$package][$name]['Save'] = $save;
  }

  /* Save data to object */
  function save_object()
  {
     /* Process values */
    if (isset($_POST['update_values'])) {
      foreach($this->DebconfAll as $package => $package_data) {
          foreach($package_data as $debconf_info) {
              $identifier = sprintf("%s/%s",$package, $debconf_info['Name']);
              $post_key = sprintf("val_%s", base64_encode($identifier));
              if (isset($_POST[$post_key])) {
                $this->UpdateEntry($identifier, get_post($post_key));
              }
              else {
                $this->UpdateEntry($identifier);
              }
          }
      }
    }

    foreach($_POST as $key => $value) {
      if (preg_match("/^toggle_save_/", $key)) {
        $identifier = base64_decode(str_replace("toggle_save_", "", $key));
        $this->ToggleSave($identifier, 1);
      }

      if(preg_match("/^toggle_nosave_/", $key)) {
        $identifier = base64_decode(str_replace("toggle_nosave_", "", $key));
        $this->ToggleSave($identifier, 0);
      }

      if (preg_match("/^reset_/", $key)) {
        $identifier = base64_decode(str_replace("reset_", "", $key));
        $this->DeleteEntry($identifier);
        break;
      }

      if (preg_match("/^edit_/", $key)) {
        $identifier = base64_decode(str_replace("edit_", "", $key));
        list($package, $name) = $this->getIdentifierComponents($identifier);
        $this->dialog = new faiPackageConfiguration($this->config, $this->parent->dn, $package, $this->release, $this->DebconfConfigurations[$package]);
        $this->dialog->used_by_faiDebconfConfigurations = TRUE;
        break;
      }
      if (preg_match("/^NewDebconfEntry$/", $key)) {
        $this->dialog = new NewDebconfConfiguration($this->config, $this->parent->dn, $this);
        $this->is_dialog = TRUE;
        break;
      }
    }
 
    /* Cancel add debconf dialog */
    if (isset($_POST['cancel_AddDeconf'])) {
      $this->dialog = FALSE;
    }

    /* Save add debconf dialog */
    if (isset($_POST['save_AddDebconf'])) {
      $this->dialog->save_object();
      $msgs = $this->dialog->check();
      if (count($msgs)>0) {
        foreach($msgs as $msg){
          msg_dialog::display(_("Error"), $msg, ERROR_DIALOG);
        }
      }else {
          $new_config = $this->dialog->DebconfConfiguration;
          $this->dialog = FALSE;
          $package = $new_config['Package'];
          $name = $new_config['FAIvariable'];
          $data = array(
                        'Name' => $name,
                        'Value' => $new_config['FAIvariableContent'],
                        'Type' => $new_config['FAIvariableType'],
                        'Save' => TRUE,
                        'Status' => 'manually_added',
                       );
          $this->DebconfConfigurations[$package][$name] = $data;
      }
    }

    /* Handle the classical debconf dialog */
    /* Cancel configuration */
    if(isset($_POST['CancelObjectConfig'])){
      $this->dialog = false;
      $this->is_dialog=false;
    }

    /* Save Configuration */
    if(isset($_POST['SaveObjectConfig'])){
      if(!preg_match("/^freeze/", $this->FAIstate)){
        $result = $this->dialog->save();
        foreach($this->packages as $package) {
          if (isset($result[$package])) {
            foreach($result[$package] as $attr => $data) {
                $identifier = sprintf("%s/%s", $package, $attr);
                $this->UpdateEntry($identifier, $data['Value']);
                $this->ToggleSave($identifier, $data['Save']);
            }
          }
        }
      }
      $this->dialog = false;
      $this->is_dialog=false;
    }

  }

  /* Check supplied data */
  function check()
  {
    /* Call common method to give check the hook */
    $message= plugin::check();
    $message = array();

    foreach($this->DebconfConfigurations as $package => $package_data) {
      foreach($package_data as $debconf_info) {
        /* Do not show errors for values which are not saved to LDAP */
        if (!$debconf_info['Save'] or (isset($debconf_info['Status']) && $debconf_info['Status']=='removed')) {
          continue;
        }
        if ($debconf_info['Type'] == 'boolean') {
          if (!empty($debconf_info['Value'])) {
            $value = strtolower($debconf_info['Value']);
          } else {
            $value = "";
          }

          if (!$this->normalize_bool($value, TRUE)) {
            $message[] = sprintf(_("The specified value '%s' for variable '%s' is not valid with the specified boolean variable type.<br><i>Allowed values: true, false, 1, 0.</i>"), $debconf_info['Value'], $debconf_info['Name']);
          }
      }
     }
    }

    return ($message);
  }

  function save()
  {
    $this->save_object();
    return($this->DebconfConfigurations);
  }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
