<?php

class filterFAI extends filterLDAP{

  static function query($base, $scope, $filter, $attributes, $category, $objectStorage= "")
  {
    /* Create a new list of FAI object
     * Generate List of Partitions,Hooks,Scripts,Templates,Profiles ...
     */
    $ObjectTypes = array(
        "FAIpartitionTable"  => array("OU"=> get_ou("faiPartitionTable", "faiPartitionRDN") , "CHKBOX"=>"ShowPartitions"  ,
          "ACL" => "faiPartitionTable"),
        "FAIpackageList"     => array("OU"=> get_ou("faiPackage", "faiPackageRDN")   , "CHKBOX"=>"ShowPackages"    ,
          "ACL" => "faiPackage"),
        "FAIscript"          => array("OU"=> get_ou("faiScript", "faiScriptRDN")    , "CHKBOX"=>"ShowScripts"     ,
          "ACL" => "faiScript"),
        "FAIvariable"        => array("OU"=> get_ou("faiVariable", "faiVariableRDN")  , "CHKBOX"=>"ShowVariables"   ,
          "ACL" => "faiVariable"),
        "FAIhook"            => array("OU"=> get_ou("faiHook", "faiHookRDN")      , "CHKBOX"=>"ShowHooks"       ,
          "ACL" => "faiHook"),
        "FAIprofile"         => array("OU"=> get_ou("faiProfile", "faiProfileRDN")   , "CHKBOX"=>"ShowProfiles"    ,
          "ACL" => "faiProfile"),
        "FAItemplate"        => array("OU"=> get_ou("faiTemplate", "faiTemplateRDN")  , "CHKBOX"=>"ShowTemplates"   ,
          "ACL" => "faiTemplate"));


    global $config;
    $tmp = FAI::get_all_objects_for_given_base($base,$filter);
    $ui = get_userinfo();
    $ldap = $config->get_ldap_link();
    $ldap->cd($config->current['BASE']);
   
    $result = array();
    foreach($tmp as $entry){

      /* Get some more informations about the object */
      $ldap->cat($entry['dn'], array("cn","description","objectClass","FAIclass","FAIstate","objectClass"));
      $object  = $ldap->fetch();

      /* Walk through possible types */
      foreach($ObjectTypes as $type => $rest){

        $acl = $ui->get_permissions($object['dn'],"fai/".$rest ['ACL']);

        if(in_array($type,$object['objectClass']) && preg_match("/r/",$acl)){

          /* Prepare object */
          unset($object['objectClass']['count']);
          if(!isset($object['description'][0])){
            $object['description'][0]="";
          }

          $dn = preg_replace("/^[^,]*+,/","",$object['dn']);

          /* Clean up object informations */
          $obj                  = array();
          $obj['cn']            = $object['cn'][0];
          $obj['dn']            = $object['cn'][0].",".get_ou("faiManagement", "faiBaseRDN").$config->current['BASE'];
          $obj['acl']           = $acl;
          $obj['class']         = $rest ['ACL'];
          $obj['FAIstate']      = $entry['FAIstate'];
          $obj['description']   = $object['description'][0];
          $obj['objectClass']   = $object['objectClass'];
          $obj['objectClass'][] = 'FAKE_OC_FAI';

          if(!preg_match("/freeze/i", $entry['FAIstate'])){
            $obj['objectClass'][] = "FAKE_OC_IsBranch";
          }
          $object['FAIstate'] = $entry['FAIstate'];

          $obj['TYPES'][]       = $type;
          $obj['GROUPS'][$type]      = $object;

          if(!isset($result[$obj['cn']])){
            $result[$obj['cn']] = $obj;
          }else{
            $result[$obj['cn']]['TYPES'][]=$type;
            $result[$obj['cn']]['GROUPS'][$type]=$object;
          }
        }
      }
    }

    // Make results useable for the listing class.
    $entries = array();
    foreach($result as $entry){
      $item=array('count' => count($entry) -1);
      foreach($entry as $name => $value){
        if($name != "GROUPS"){
          $item[] = $name;

          $item[$name] = $value ;       
        }
      }
      $item['GROUPS'] = $entry['GROUPS'];
      $entries[] = $item;
    }


    // Append opsi stuff ... 
    if(class_available('opsi') && $base == get_ou("faiManagement", "faiBaseRDN").$config->current['BASE']){
      $opsi = new opsi($config);
      $objects = array();
      if($opsi instanceof opsi && $opsi->enabled()){
        $opsi_acl = $ui->get_permissions($base,"opsi/opsiProperties");
        if(preg_match("/r/",$opsi_acl)){
          $err = FALSE;

          if(preg_match("/FAIstate=__NETBOOT__/", $filter)){
            $n_pro = $opsi->get_netboot_products();
            $err |= $opsi->is_error();
            foreach($n_pro as $name => $data){
              $entry = array(
                  "cn" => $name,
                  "description" => $data['DESC'],
                  "type" => "opsi_netboot");
              $objects[$name]['opsi_netboot'] = $entry;
            }
          }
          if(preg_match("/FAIstate=__LOCALBOOT__/", $filter)){
            $l_pro = $opsi->get_local_products();
            $err |= $opsi->is_error();
            foreach($l_pro as $name => $data){
              $entry = array("cn" => $name,
                  "description" => $data['DESC'],
                  "type" => "opsi_local");
              $objects[$name]["opsi_local"] = $entry;
            }
          }
          if($err){
            msg_dialog::display(_("Error"),msgPool::siError($opsi->get_error()),ERROR_DIALOG);
          }
        }
      }

      $cn_filter = "";
      if(preg_match("/^.*cn=([^)]*).*/",$filter)){
        $cn_filter = trim(preg_replace("/^.*cn=([^)]*).*/","\\1",$filter),"*");
      }

      foreach($objects as $name => $data)
      {
        $data = array_pop($data);
        if(!empty($cn_filter) && !preg_match("/".preg_quote($cn_filter,'/')."/",  $data['cn'])){
          continue;
        }
        $item =array();

        $item['objectClass']  = array('count' => 2, $data['type'],"FAKE_OC_OPSI");
        $item[] = 'objectClass';
        $item['cn']  = $data['cn'];
        $item[] = 'cn';
        $item['description']  = $data['description'];
        $item[] = 'description';
        $item['dn']  = $data['cn'];
        $item[] = 'dn';
        $item['TYPES']  = array($data['type']);
        $item[] = 'TYPES';
        $item['count'] = '5';
        $entries[] = $item;  

      }
    }

    return($entries);
  }
}
?>
