<?php

class filterFAIClass {

  static function query($base, $scope, $filter, $attributes, $category, $objectStorage= "")
  {
    global $config;

    $base = session::get('filterFAIClass_base');
    $objectFilter = "(|(objectClass=FAIscript)(objectClass=FAItemplate)(objectClass=FAIhook)".
      "(objectClass=FAIvariable)(objectClass=FAIpartitionTable)(objectClass=FAIpackageList))";

    $FAI_objects = FAI::get_all_objects_for_given_base($base,$objectFilter,true);

    /* Create array with categories to be able to sort different types of objects */
    $categories = array("FAIscript"         => "faiScript",
        "FAItemplate"       => "faiTemplate",
        "FAIhook"           => "faiHook",
        "FAIvariable"       => "faiVariable",
        "FAIpartitionTable" => "faiPartitionTable",
        "FAIpackageList"    => "faiPackage");

    /* Read out all objects from fai tree */
    $ldap= $config->get_ldap_link();
    $ldap->cd($base);
    $ui = get_userinfo();
    $sort=array();
    $FAIAllclasses = array();

    foreach($FAI_objects as $obj){

      $ldap->cat($obj['dn']);
      $attrs = $ldap->fetch();

      /* Only use objects which have cn set */
      $name = preg_replace("/[\*\.]/","",$filter);
      if(empty($name)){
        $regex = '.*';
      }else{
        $regex = preg_quote($name,'/');
      }
    
      if(isset($attrs['cn'][0]) && preg_match("/".$regex."/i", $attrs['cn'][0])){
        foreach($categories as $cat => $acl){
          if(in_array($cat,$attrs['objectClass'])){
            $acl =  $ui->get_permissions($attrs['dn'],"fai/".$acl);
            if(!isset($FAIAllclasses[$attrs['cn'][0]])){
              $FAIAllclasses[$attrs['cn'][0]] = $attrs ;
            }else{
              $FAIAllclasses[$attrs['cn'][0]]['objectClass'] = 
                array_merge($attrs['objectClass'],$FAIAllclasses[$attrs['cn'][0]]['objectClass']);
            }
          }
        }
      }
    }
    return(filterFAIClass::filterByBlacklist(array_values($FAIAllclasses)));
  }

  static function filterByBlacklist($entries)
  {
    if(session::is_set('filterBlacklist')){
      $blist = session::get('filterBlacklist');
      foreach($blist as $attr_name => $attr_values){
        foreach($attr_values as $match){
          foreach($entries as $id => $entry){
            if(isset($entry[$attr_name])){
              $test = $entry[$attr_name];
              if(!is_array($test)) $test = array($test);
              if(in_array($match, $test)) unset($entries[$id]);
            }
          }
        }
      }
    }
    return(array_values($entries));
  }
}
?>
