<?php
/*
   This code is part of GOsa (https://gosa.gonicus.de)
   Copyright (C) Fabian Hickert 

   This program is free software; you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation; either version 2 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program; if not, write to the Free Software
   Foundation, Inc., 59 Temple Place, Suite 330, Boston, MA  02111-1307  USA
 */

class faiSummaryTab extends plugin{

  var $Classes        = array();
  var $Release        = "";
  var $Releases       = array();
  var $Result         = array();
  var $objs	          = array();
  var $ObjectList     = "";
  var $is_dialog      = false;
  var $InitCalled     = false;
  var $usedClasses    = array();
  var $base           = ""; 
  var $parent         = NULL; 
  var $ui             = NULL;

  function faiSummaryTab(&$config,$dn)//,$parent)
  {
    plugin::plugin($config,$dn);//,$parent);

    $this->ui = get_userinfo();

    /* Base object configuration */
    $this->objs = array(	
        "FAIscript" 		    => array(	"Image"	=>"plugins/fai/images/fai_script.png",        "Name"=>_("Scripts")      ,"Tree"	=>get_ou("faiScript", "faiScriptRDN")) ,
        "FAIhook" 			    => array(	"Image"	=>"plugins/fai/images/fai_hook.png" ,         "Name"=>_("Hooks")        ,"Tree"	=>get_ou("faiHook", "faiHookRDN")),
        "FAIvariable" 		  => array(	"Image"	=>"plugins/fai/images/fai_variable.png",      "Name"=>_("Variables")    ,"Tree"	=>get_ou("faiVariable", "faiVariableRDN")) ,
        "FAItemplate" 		  => array(	"Image"	=>"plugins/fai/images/fai_template.png",      "Name"=>_("Templates")    ,"Tree"	=>get_ou("faiTemplate", "faiTemplateRDN")) ,
        "FAIpartitionTable" => array(	"Image"	=>"plugins/fai/images/fai_partitionTable.png","Name"=>_("Partition table")  ,"Tree"	=>get_ou("faiPartitionTable", "faiPartitionRDN")) ,
        "FAIpackageList" 	  => array(	"Image" =>"plugins/fai/images/fai_packages.png",      "Name"=>_("Package list") ,"Tree"	=>get_ou("faiPackage", "faiPackageRDN")) ,
        "FAIprofile" 		    => array(	"Image"	=>"plugins/fai/images/fai_profile.png",       "Name"=>_("Profiles")     ,"Tree"	=>get_ou("faiProfile", "faiProfileRDN")));
  }


  /* Init the fai tree with objects.
     If this method is called, all assigned classes and the release name 
      will be read from parent object. */
  function init()
  {
    $this->InitCalled = true;
    $this->Result = array();
  
    /* Get classes & release name 
       There are 2 different tabs which use the summary tab
       faiProfile / Workstations */
   
    /* Check if given parent object is fai profile tabs */
    if(isset($this->parent->by_name['faiProfile'])){
      
      /* Get Classes and release */
      $this->Classes = $this->parent->by_object['faiProfile']->FAIclasses;
      $this->base    = $this->parent->by_object['faiProfile']->parent->parent->fai_release;
      $str = preg_replace("/^.*".preg_quote(get_ou("faiProfile", "faiProfileRDN"), '/')."/i","",$this->dn);
      $tmp  = $this->parent->by_object['faiProfile']->parent->parent->getBranches();
      $this->Release = $tmp[$this->base];
  
    /* Check if parent tab is work tab */
    }elseif(isset($this->parent->by_name['workstartup'])){

      if($this->parent->by_object['workstartup']->FAIdebianMirror == "inherited"){
        $this->Classes = $this->parent->by_object['workstartup']->InheritedFAIclass;
        $this->Release = $this->parent->by_object['workstartup']->InheritedFAIrelease;
      }else{
        $this->Classes = $this->parent->by_object['workstartup']->FAIclass;
        $this->Release = $this->parent->by_object['workstartup']->FAIrelease;
      }

      /* Check if this is a workstation or an object group */
      if(isset($this->parent->by_object['workgeneric'])){
        $this->base    = $this->parent->by_object['workgeneric']->base;
      }elseif(isset($this->parent->by_object['ogroup'])){
        $this->base    = $this->parent->by_object['ogroup']->base;
      }elseif(isset($this->parent->by_object['servgeneric'])){
        $this->base    = $this->parent->by_object['servgeneric']->base;
      }else{
        msg_dialog::display(_("Error"), _("Unknown FAI information source!"), ERROR_DIALOG);
      }
      
      /* Append workstation class && LAST */
      if(isset($this->parent->by_object['workgeneric']->cn)){
        $this->Classes[] = $this->parent->by_object['workgeneric']->cn;
      }
    }

    // All FAI stuff is located on the ldap base, isn't it?
    $this->base = $this->config->current['BASE'];

    $tmp = array_flip($this->getBranches());

    $this->Classes[] = "LAST";

    /* You need full access to complete collection of fai acls to view this summary */
    $acl = $this->ui->has_complete_category_acls($this->base,"fai");
    if(!preg_match("/r/",$acl)){
      $this->Classes = array();
    }

    /* Set all available releases */
    $this->Releases = $tmp;
    $this->usedClasses = array();


    /* Check the configuration */
    if(count($this->Classes) && (!empty($this->Release))){
      foreach($this->Classes as $class){
        $this->resolveObject($class);
      }
    }
  }


  /* Capture posts, and call opnen close tag */
  function save_object()
  {
    $once = true;

    if(isset($_GET['expand']) && isset($_GET['id'])){
      if(isset($this->Result[$_GET['id']])){
        $this->Result[$_GET['id']]['Open'] =true  ;
      }
    }

    if(isset($_GET['compress']) && isset($_GET['id'])){
      if(isset($this->Result[$_GET['id']])){
        unset($this->Result[$_GET['id']]['Open']);
      }
    }

    foreach($_POST as $name => $value ){

      /* check for open tag request */
      if(preg_match("/^open_/",$name) && $once){
        $once  = false;
        $value = preg_replace("/^open_/","",$name);
        $value = preg_replace("/_.*$/","",$value); 
        $this->Result[$value]['Open'] =true  ;
      }
      /* Check for close tag request */
      if(preg_match("/^close_/",$name) && $once){
        $once  = false;
        $value = preg_replace("/^close_/","",$name);
        $value = preg_replace("/_.*$/","",$value); 
        unset($this->Result[$value]['Open']);
      }
    }
    if(isset($_POST['reloadList'])){
      $this->InitCalled = false;
    }
  }


  /* returns an array with all possibly release types */
  function getBranches($base = false,$prefix = "")
  {
    if(!$base){
      $base = $this->base;
    }
    return(FAI::get_all_releases_from_base($base,true));
  }


  /* This function calls all nescessary functions to generate the fai class summary tree */
  function execute() 
  {
    plugin::execute();

    /* Skip this if fai is deactivated */
    if(!$this->config->pluginEnabled("faiManagement")){
      $str = "<h3>"._("You can't use this plug-in until FAI is activated.")."</h3>";
      return $str;
    }    

    /* Check if we must (re)init the this tab */
    if(!$this->InitCalled){
      $this->init();
    }

    if(isset($_GET['show'])){
      $ldap = $this->config->get_ldap_link();
      $ldap->cat(postDecode($_GET['id']), array('FAIscript', 'FAIhook', 'FAItemplateFile'));
      $attrs = $ldap->fetch();
      if($_GET['type'] == "FAIscript"){
        $str = $attrs['FAIscript'][0];
      }elseif($_GET['type'] == "FAIhook"){
        $str = $attrs['FAIscript'][0];
      }elseif($_GET['type'] == "FAItemplate"){
        $str = $attrs['FAItemplateFile'][0];
      }
      echo "<pre>".$str."</pre>";
      exit();
    }

    /* Get smarty class & assign created summary results */
    $smarty = get_smarty();
    $acl = $this->ui->has_complete_category_acls($this->base,"fai");
    $smarty->assign("readable", preg_match("/r/",$acl));

    $this->ObjectList = $this->createSummary($this->Result);
    $smarty->assign("objectList",$this->ObjectList);
    return($smarty->fetch (get_template_path('faiSummary.tpl', TRUE, dirname(__FILE__))));
  }


  /* Create output of from resolved fai objects */
  function createSummary($data)
  {
    /* Don't generate any output, if there is no result */ 
    if(!count($this->Result)){
      return("<h3>"._("This object has no FAI classes assigned.")."</h3>");
    }

    $str ="<table style='width:100%;' summary='"._("FAI summary")."'> ";
    /* walk through all classes */	
    foreach($data as $key => $entry){
      
      $image = image($this->objs[$key]['Image']);
      if(!isset($entry['Open'])){
        $str .= "<tr>
          <td colspan=2>".
          image('images/lists/expand.png').$image."&nbsp;".
          "<a href='?plug=".$_GET['plug']."&amp;expand&amp;id=".$key."'>".
          "<b>".$this->objs[$key]['Name']."</b></a></td></tr>";
      }else{
        $str .= "<tr><td colspan=2>".
          image('images/lists/sort-down.png').$image."&nbsp;".
          "<a href='?plug=".$_GET['plug']."&amp;compress&amp;id=".$key."'>".
          "<b>".$this->objs[$key]['Name']."</b></a></td></tr>";

        /* Display FAItemplate FAIhook FAIscript entries */ 
        if(in_array($key,array("FAItemplate","FAIhook","FAIscript"))){

          $nums = array();          
          $tmp  = array();
          if($key == "FAIscript"){
            if(is_array($entry['Entries'])){
              foreach($entry['Entries'] as $scripts){
                foreach($scripts as $script){

                  if(!isset($nums[$script['cn'][0]])){
                    $nums[$script['cn'][0]]= 0;
                  } 
                  $nums[$script['cn'][0]] ++;

                  $tmp[$script['FAIpriority'][0].$script['cn'][0].$script['CLASS']] = $script;
                }
              }
            }
            krsort($tmp);;
            
            $entry['Entries'] = $tmp;
          }else{
            $tmp = array();
            foreach($entry['Entries'] as $script){
              $tmp[$script['cn'][0].$script['CLASS']] = $script;
              if(!isset($nums[$script['cn'][0]])){
                $nums[$script['cn'][0]]= 0;
              } 
              $nums[$script['cn'][0]] ++;
            }
             ksort($tmp);
            $entry['Entries'] = $tmp;
          }

          foreach($entry['Entries'] as $cn => $data){

            if(isset($nums[$data['cn'][0]]) && ($nums[$data['cn'][0]] > 1)){
              $str .="<tr style='background: #EE3434;'><td style='padding-left:20px;'>";
            }else{
              $str .="<tr><td style='padding-left:20px;'>";
            }

            $str .= $image."&nbsp;";
            $str .= "<a target='_blank' href='?plug=".$_GET['plug']."&amp;show&amp;type=".$key."&amp;id=".postEncode($data['dn'])."'>";
            if(isset($data['FAIpriority'][0])){
              $str .= "(".$data['FAIpriority'][0].")";
            }

            $str .= "&nbsp;".$data['cn'][0];
            if(isset($data['description'][0])){
              $str .= " [".$data['description'][0]."]";
            }
            $str .= "</a>";
            $str .= "</td><td><i>".$data['CLASS']."</i>";
            $str .= "</td></tr>";
          }
        }

        /* Create variable entries */
        if(in_array($key,array("FAIvariable"))) {
          foreach($entry['Entries'] as $cn => $data){
            $str .="<tr><td style='padding-left:20px;'>";
            $str .= $image."&nbsp;".$data['cn'][0];
            if(isset($data['description'][0])){
              $str .= " [".$data['description'][0]."] ";
            }
            /* Only display FAIvariableContent if description dosn't contain [*] */ 
            if(isset($data['description'][0]) && !preg_match("#\[\*\]#",$data['description'][0])){
              $str .=" = '".$data['FAIvariableContent'][0]."'";
            }
            $str .= "</td><td><i>".$data['CLASS']."</i>";
            $str .="</td></tr>";
          }
        }

        /* Create packagelist entries */ 
        if(in_array($key,array("FAIpackageList"))) {
          ksort($entry['Entries']);
          foreach($entry['Entries'] as $data){
            $str .="<tr><td style='padding-left:20px;'>";
            $str .= $image."&nbsp;".$data['cn'][0];
            $str .= "</td><td><i>".$data['CLASS']."</i>";
            $str .= "</td></tr>";;
          }
        }

        /* Create partition table entries */
        if(in_array($key,array("FAIpartitionTable"))) {
          foreach($entry['Entries'] as $cn => $data){
            $str .= "<tr><td style='padding-left:20px;'>";
            $str .= $image."&nbsp;".$data['name'];
            $str .= "</td><td><i>".$data['CLASS']."</i>";
            $str .= "</td></tr>";
            ksort($data['partitions']);

            $str .= "<tr><td colspan=2  style='padding-left:20px;'>";
            $str .= "<table \""._("Summary of fai classes")."\"
                        cellspacing=0 cellpadding=2 style='background:#FFFFFF;border:1px solid #B0B0B0'>
                      <tr style='background-color:#DBDBDB;'>
                       <td class='tbhead'>"._("No.")."</td>
                       <td class='tbhead'>"._("Name")."</td>
                       <td class='tbhead'>"._("File system options")."</td>
                       <td class='tbhead'>"._("Mount options")."</td>
                       <td class='tbhead'>"._("Size in MB")."</td>
                       <td class='tbhead'>"._("Mount point")."</td>
                       <td class='tbrhead'>"._("Type")."</td>
                      </tr>";

            $i = 0;
            foreach($data['partitions'] as $key => $part){
              $i ++;

              if($i%2 == 1){
                $c = " class='rowxp1'";
                $d = " class='list1'";
              }else{
                $c = " class='rowxp0'";
                $d = " class='list0'";
              }

              $str.="<tr $c style='height:10px;'>
                      <td $d>".$i."</td> 
                      <td $d>".$part['cn'][0]."</td> 
                      <td $d>".$part['FAIfsType'][0]."</td> 
                      <td $d>".$part['FAIfsOptions'][0]."</td> 
                      <td $d>".$part['FAIpartitionSize'][0]."</td> 
                      <td $d>".$part['FAImountPoint'][0]."</td> 
                      <td $d style='border:none'>".$part['FAIpartitionType'][0]."</td> 
                    </tr>";
            }
            $str .="</table>";
          }
        }
      }
    }
    $str .="</table>";	
    return($str);
  }


  /* resolve specified object to append it to our object tree */
  function resolveObject($class)
  {
    $ldap 	          = $this->config->get_ldap_link();
    if(isset($this->Releases[$this->Release])){
      $dn 	            = $this->Releases[$this->Release];
    }else{
      return(FALSE);
    }
  
    $resolvedClasses  = FAI::get_all_objects_for_given_base($dn,"(&(objectClass=FAIclass)(cn=".$class."))");

    /* Try to fetch all types of fai objects with the given cn  */
    foreach($resolvedClasses as $obj){

      $dn = $obj['dn'];
      $ldap->cat($dn);

      while($attrs = $ldap->fetch()){

        foreach($this-> objs as $key => $rest){
          if(in_array($key,$attrs['objectClass'])){

            if(!isset($this->usedClasses[$key][$class])){
              $this->usedClasses[$key][$class] = true;
              switch($key){
                case "FAIprofile": 		        $this->prepare_FAIprofile($attrs);break;
                case "FAIscript": 		        $this->prepare_FAIscript($attrs);break;
                case "FAIhook": 		          $this->prepare_FAIhook($attrs);break;
                case "FAIvariable":		        $this->prepare_FAIvariable($attrs);break;
                case "FAItemplate":		        $this->prepare_FAItemplate($attrs);break;
                case "FAIpackageList":	      $this->prepare_FAIpackageList($attrs);break;
                case "FAIpartitionTable":	    $this->prepare_FAIpartitionTable($attrs);break;
              }
            }

          }
        }
      }
    }
  }

  /* Prepare fai script */
  function prepare_FAIscript($data)
  {
    if(isset($this->Result['FAIscript']['Entries'])){
      $current = $this->Result['FAIscript']['Entries'];
    }else{
      $current = array();
    }
    $ldap = $this->config->get_ldap_link();
    $ldap->cd($data['dn']);
    $ldap->search("(objectClass=FAIscriptEntry)",array("cn","description","FAIpriority"));
    while($attrs = $ldap->fetch()){
      $attrs['CLASS'] = $data['cn'][0];
      $current[$attrs['FAIpriority'][0]][] = $attrs;
    }
    $this->Result['FAIscript']['Entries'] = $current ;
  }


  /* Prepare fai script */
  function prepare_FAIpartitionTable($data)
  {
    if(isset($this->Result['FAIpartitionTable']['Entries'])){
      $current = $this->Result['FAIpartitionTable']['Entries'];
    }else{
      $current = array();
    }

    /* get subentries */
    $ldap = $this->config->get_ldap_link();
    $ldap->cd($data['dn']);
    $ldap->search("(objectClass=FAIpartitionDisk)",array("cn","description"));
    $disks = array();

    /* Create an array which is sortable by priority */
    while($attrs = $ldap->fetch()){
      $attrs['CLASS'] = $data['cn'][0];
      $disks[$attrs['cn'][0]]= $attrs;
      if(isset($attrs['description'][0])){
        $disks[$attrs['cn'][0]]['name'] = $attrs['cn'][0]." [".$attrs['description'][0]."]";
      }else{
        $disks[$attrs['cn'][0]]['name'] = $attrs['cn'][0];
      }
      $disks[$attrs['cn'][0]]['dn'] = $attrs['dn'];
      $disks[$attrs['cn'][0]]['partitions'] = array();
    }

    /* Sort by priority */
    foreach($disks as $key => $disk){
      $ldap->cd($disk['dn']);
      $ldap->search("(objectClass=FAIpartitionEntry)",array("*"));
      while($attrs = $ldap->fetch()){
        if(!isset($attrs['FAIfsOptions'][0])){
          $attrs['FAIfsOptions'][0] = "";
        }
        $attrs['CLASS'] = $data['cn'][0];
        $disks[$key]['partitions'][$attrs['cn'][0]] = $attrs;
      }
    }
    $this->Result['FAIpartitionTable']['Entries'] = $disks;
  }


  /* Create template entry */	
  function prepare_FAItemplate($data)
  {
    $current = array();
    if(isset($this->Result['FAItemplate']['Entries'])){
      $current = $this->Result['FAItemplate']['Entries'];
    }

    $ldap = $this->config->get_ldap_link();
    $ldap->cd($data['dn']);
    $ldap->search("(objectClass=FAItemplateEntry)",array("cn","description"));
    while($attrs = $ldap->fetch()){
      $attrs['CLASS'] = $data['cn'][0];
      $current[] = $attrs;
    }
    $this->Result['FAItemplate']['Entries'] = $current ;
  }


  /* Create a package list  */	
  function prepare_FAIpackageList($data)
  {
    $current = array();
    if(isset($this->Result['FAIpackageList']['Entries'])){
      $current = $this->Result['FAIpackageList']['Entries'];
    }
    if(isset($data['FAIpackage'])){
      unset($data['FAIpackage']['count']);
      foreach($data['FAIpackage'] as $pkg){
        $attrs['CLASS'] = $data['cn'][0];
        $attrs['cn'][0] = $pkg;
        $current[$pkg] = $attrs;
      }
    }
    $this->Result['FAIpackageList']['Entries'] = $current ;
  }


  /* Create a variable entry */	
  function prepare_FAIvariable($data)
  {
    $current = array();
    if(isset($this->Result['FAIvariable']['Entries'])){
      $current = $this->Result['FAIvariable']['Entries'];
    }
    $ldap = $this->config->get_ldap_link();
    $ldap->cd($data['dn']);
    $ldap->search("(objectClass=FAIvariableEntry)",array("cn","description","FAIvariableContent"));
    while($attrs = $ldap->fetch()){
      $attrs['CLASS'] = $data['cn'][0];
      $current[] = $attrs;
    }
    $this->Result['FAIvariable']['Entries'] = $current ;
  }

  /* Create a hook entry */	
  function prepare_FAIhook($data)
  {
    $current = array();
    if(isset($this->Result['FAIhook']['Entries'])){
      $current = $this->Result['FAIhook']['Entries'];
    }
    $ldap = $this->config->get_ldap_link();
    $ldap->cd($data['dn']);
    $ldap->search("(objectClass=FAIhookEntry)",array("cn","description"));
    while($attrs = $ldap->fetch()){
      $attrs['CLASS'] = $data['cn'][0];
      $current[$attrs['cn'][0]] = $attrs;
    }
    $this->Result['FAIhook']['Entries'] = $current ;
  }


  /* Create a new Profile entry */
  function prepare_FAIprofile($data)
  {
    $classes = explode(" ",$data['FAIclass'][0]);
    foreach($classes as $class){
      $class = trim($class);
      $this->resolveObject($class);
    }
  }

  /* Return plugin informations for acl handling * /
  static function plInfo()
  {
    return (array(
          "plShortName"   => _("Summary"),
          "plDescription" => _("FAI summary"),
          "plSelfModify"  => FALSE,
          "plDepends"     => array(),
          "plPriority"    => 15,
          "plSection"     => array("administration"),
          "plCategory"    => array("workstation","server","fai","ogroups") ,
          "plProvidedAcls"=> array(
            "readable" => _("Viewable")),
          ));
  }
  */
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
