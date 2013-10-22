<?php

class debconf
{
  var $package= "";
  var $language= "";
  var $loaded_template= FALSE;
  var $template= array();
  var $read_only = FALSE;

  function debconf($language= "", $read_only=FALSE)
  {
    $this->set_language($language);
    $this->read_only = $read_only;
  }

  function set_language($language)
  {
    $this->language= $language;
  }

  function load_from_string($str)
  {
    $lines                 = explode("\n",$str);
    $this->template        = array();
    $post_name             = 0;
    $langcode              = $this->language.".UTF-8";
    $in_description        = FALSE;
    $got_local_description = FALSE;

    foreach($lines as $line){

      /* Reset description flag */
      if ($in_description && !preg_match("/^ /", $line)){
        $in_description= FALSE;
      }

      /* Template header */
      if (preg_match("/^Template: /", $line)){
        $post_name ++; 
        $name= trim(preg_replace("/^Template: (.*)$/", "\\1", $line));
        $this->template[$post_name]['Name'] = $name;
        $this->template[$post_name]['Default'] ="";
        $this->template[$post_name]['Save'] = FALSE;

        $got_local_description= FALSE;
        continue;
      }

      /* Get type */
      if (preg_match("/^Type: /", $line)){
        $type= trim(preg_replace("/^Type: (.*)$/", "\\1", $line));
        $this->template[$post_name]['Type']= $type;
        continue;
      }

      /* Get default */
      if (preg_match("/^Default: /", $line)){
        $this->template[$post_name]['Default']= "";
        $default= trim(preg_replace("/^Default: (.*)$/", "\\1", $line));
        $this->template[$post_name]['Default']= $default;
        continue;
      }

      /* Get description */
      if (!$got_local_description && preg_match("/^Description: /i", $line)){
        $description= trim(preg_replace("/^Description: (.*)$/i", "\\1", $line));
        $this->template[$post_name]['Topic']= $description;
        $this->template[$post_name]['Description']= "";
        $in_description= TRUE;
        continue;
      }

      /* Fill description */
      if (!$got_local_description && $in_description){
        $description= preg_replace("/^ (.*)$/", "\\1", $line);
        if ($this->template[$post_name]['Description'] != "") {
          $this->template[$post_name]['Description'].= " ";
        }
        if ($description == ".") {
          $description = "<br/>";
        } else {
          $description = htmlspecialchars($description, ENT_QUOTES, 'UTF-8');
        }
        $this->template[$post_name]['Description'].= "$description";
        continue;
      }

      /* Get local description */
      if (preg_match("/^Description-$langcode: /i", $line)){
        $description= trim(preg_replace("/^Description-$langcode: (.*)$/i", "\\1", $line));
        $this->template[$post_name]['Topic']= $description;
        $in_description= TRUE;
        $got_local_description= TRUE;
        $this->template[$post_name]['Description']= "";
        continue;
      }

      /* Fill local description */
      if ($got_local_description && $in_description){
        $description= preg_replace("/^ (.*)$/", "\\1", $line);
        if ($this->template[$post_name]['Description'] != "") {
          $this->template[$post_name]['Description'].= " ";
        }
        if ($description == ".") {
          $description = "<br/>";
        } else {
          $description = htmlspecialchars($description, ENT_QUOTES, 'UTF-8');
        }
        $this->template[$post_name]['Description'].= "$description";
        continue;
      }

      /* Get native choices */
      if (preg_match("/^Choices: /", $line)){
        $type= trim(preg_replace("/^Choices: (.*)$/", "\\1", $line));
        $this->template[$post_name]['Choices']= $type;
      }

      /* Get local choices */
      if (preg_match("/^Choices-$langcode: /", $line)){
        $type= trim(preg_replace("/^Choices-$langcode: (.*)$/", "\\1", $line));
        $this->template[$post_name]['Choices-local']= $type;
      }

    }

    $this->loaded_template= TRUE;

    $tmp= array();
    foreach($this->template as $post_name => $template){
      $template['post_name'] = "post_".$post_name;
      $tmp[] = $template;
    }
    $this->template = $tmp;

    return (TRUE);
  }

  function has_template()
  {
    return(count($this->template) != FALSE);
  }


  /* Check if some fields are posted */
  function PostCheck()
  {
    /* Walk through all template variables */
    foreach($this->template as $post_name => $entry){

      /* Check if this var is set*/
      if(isset($_POST[$entry['post_name']])){
        $checkbox_key = sprintf("save_%s", $entry['post_name']);
        if (!isset($_POST[$checkbox_key])) {
          $this->template[$post_name]['Save'] = FALSE;
        }else {
          $this->template[$post_name]['Save'] = TRUE;
        }

        /* special handling for arrays */
        if(is_array($_POST[$entry['post_name']])){
          $str = "";
          foreach(get_post($entry['post_name']) as $val){
            $str.= $val.", ";
          }
          $str = preg_replace("/\,\ $/","",$str);
          $this->template[$post_name]['Default'] = $str;
        }else{
          $this->template[$post_name]['Default'] = get_post($entry['post_name']);
        }
      }
    }
    
    foreach($this->template as $post_name => $entry){
      if(isset($_POST["multi-".$entry['post_name']])){ 
        $this->template[$post_name]['Default']= "";
        foreach($_POST as $name => $value){
          if (get_magic_quotes_gpc()) {
              $value = stripcslashes($value);
          }
          if(preg_match("/".$entry['post_name']."-multi-/",$name)){
            $this->template[$post_name]['Default'] .= $value.", ";
          }
        } 
        $this->template[$post_name]['Default'] = preg_replace("/, $/","",$this->template[$post_name]['Default']);
      }
    }
  }


  /* This funtion sets the default value and the default save flag*/
  function SetDefault($var,$val, $save_flag)
  {
    if ($this->loaded_template) {
      foreach($this->template as $key => $tmp){
        if($tmp['Name'] == $var ){
          $this->template[$key]['Default'] = $val;
          $this->template[$key]['Save'] = $save_flag;
        }
      }
    }
  }


  /* Display all possible options in html*/
  function get_dialog()
  {
    if ($this->loaded_template && count($this->template)) {
      $result= "<table summary=''>";

      foreach ($this->template as $post_name => $entry){

        $types= array("boolean" => "", "multiselect" => "", "note" => "",
            "password" => "", "select" => "", "string" => "", "text" => "", "title" => "");

        /* Check if type is available */
        if ((isset($entry['Type']))&&(isset($types[$entry['Type']]))){

          /* Produce type specific output */
          $fn= "render_".$entry['Type'];
          $str = $this->$fn($entry);
          if(!empty($str)){
            $result.=$str."<tr><td colspan='3'><p class='seperator'>&nbsp;</p></td></tr>";
          }
        } else {
          //php_error(E_WARNING, "An unknown type has been specified in the debconf template. Please fix.");
        }
      }
      $result.="</table>";

    
      $result .= "<input type='hidden' post_name='check_post' value='1'>";
      return ($result);
    } else {
      return _("This package has no debconf options.");
    }
  }


  function render_checkbox($data) {
    $postname = $data['post_name'];
    if (isset($data['Save'])) {
      $save = $data['Save'];
    } else {
      $save = 0;
    }

    $checked="";
    if ($save) {
      $checked = "checked";
    }

    $disabled = ($this->read_only) ? "disabled" : ""; 
    $template = "<img src='images/save.png'><br><input type=checkbox name='save_%s' $disabled value='%s' %s>";
    return(sprintf($template, $postname, $save, $checked));
  }

  function render_boolean($data)
  {
    $post_name= $data['post_name'];
    $result = "";
    $disabled = ($this->read_only) ? "disabled" : "";
    $template = "<input type='radio' name=\"%s\" value=\"%s\" %s $disabled onChange='javascript:document.getElementsByName(\"save_%s\")[0].checked = true;'>%s";

    foreach(array("true","false") as $value){
      if($data['Default'] == $value){
        $result.= sprintf($template, $data['post_name'], $value, "checked",  $data['post_name'], _($value));
      }else{
        $result.= sprintf($template, $data['post_name'], $value, "", $data['post_name'], _($value));
      }
      $result.="<br>";
    }
    return ($this->render_entry($data, $result));
  }


  function render_multiselect($data)
  {
    $post_name= $data['post_name'];
    $input = "";
       
    if (preg_match('/\$\{/', $data['Choices'])){
        $data['Description'] .= '<br><br><b>' . _('This debconf question is dynamically generated during package installation and requires choosing between specific options which cannot be presented here. The entered text needs to be one of the valid choices in order to take effect.') . '</b>';
        return($this->render_string($data));
    } else {
        $choices= "";
        foreach (explode(", ", $data['Choices']) as $choice){
            $choices[]= $choice;
        }

        $disabled = ($this->read_only) ? "disabled" : "";
        $input="<input type='hidden' name='multi-".$post_name."' $disabled value='1'>";
        $template = "\n<input name='%s-multi-%s' type='checkbox' value=\"%s\" %s onChange='javascript:document.getElementsByName(\"save_%s\")[0].checked = true;'>%s";

        $defs = explode(", ",$data['Default']);
        foreach($choices as $value){
            if(in_array($value,$defs)){
                $input .= sprintf($template, $post_name, $value, htmlentities($value), "checked", $post_name, _($value));
            }else{
                $input .= sprintf($template, $post_name, $value, htmlentities($value), "", $post_name, _($value));
            }
        }

    }    
    return ($this->render_entry($data, $input));
}


  function render_note($data)
  {
    /* Ignore notes, they do not makes sense, since we don't get any
       chance to test entered values... */
    return ("");
  }


  function render_password($data)
  {
    $result =  "";

    $disabled = ($this->read_only) ? "disabled" : ""; 
    $template = "&nbsp;<input type='text' name='%s' value=\"%s\" $disabled onChange='javascript:document.getElementsByName(\"save_%s\")[0].checked = true;'><br><br>";
    $result .= sprintf($template, $data['post_name'], $data['Default'], $data['post_name']);
    return ($this->render_entry($data, $result));
  }


  function render_select($data)
  {
    $post_name= $data['post_name'];
    
    $result = "";
    if (preg_match('/\$\{/', $data['Choices'])){
        return($this->render_multiselect($data));
    } else {
        $choices= "";
        foreach (preg_split("/,\s?/", $data['Choices']) as $choice){
            $choices[]= $choice;
        }

        $disabled = ($this->read_only) ? "disabled" : "";
        $template = "\n<input type='radio' name='%s' value=\"%s\" %s $disabled onChange='javascript:document.getElementsByName(\"save_%s\")[0].checked = true;'>%s<br>";;
        foreach($choices as $value){
            if($data['Default'] == $value){
                $result.= sprintf($template, $post_name, htmlentities($value), "checked", $post_name, htmlentities($value));
            }else{
                $result.= sprintf($template, $post_name, htmlentities($value), "", $post_name, htmlentities($value));
            }
        }
    }

    return ($this->render_entry($data, $result));
  }


  function render_string($data)
  {
    $disabled = ($this->read_only) ? "disabled" : "";
    $template = "<input type='text' name='%s' $disabled value=\"%s\" style='width:300px' onChange='javascript:document.getElementsByName(\"save_%s\")[0].checked = true;'>";
    $result=  sprintf($template, $data['post_name'], $data['Default'], $data['post_name']);
    return ($this->render_entry($data, $result));
  }


  function render_text($data)
  {
    /* Ignore text messages, they are normally used for status hints. */
    return ("");
  }


  function render_title($data)
  {
    /* Ignore text messages, they are normally used for status hints. */
    return ("");
  }

  function render_entry($data, $input) {
    $smarty = get_smarty();
    $smarty->assign('chkbox', $this->render_checkbox($data));
    $smarty->assign('topic', $data['Topic']);
    $smarty->assign('description', $data['Description']);
    $smarty->assign('input', $input);
    return($smarty->fetch (get_template_path('debconfTemplate_entry.tpl', TRUE)));
  }
}


// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
