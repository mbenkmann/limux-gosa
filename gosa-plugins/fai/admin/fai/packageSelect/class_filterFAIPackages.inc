<?php

class filterFAIPackages {

  static function query($base, $scope, $filter, $attributes, $category, $objectStorage= "")
  {
    $filter= preg_replace('/\*/', '', $filter);
    $pkgs = array();
    $q          = new gosaSupportDaemon();
    $filter     = array("%$filter%");
    $attrs      = array("distribution", "package","version", "section", "description", "timestamp");
    $release = session::get('packageSelect_Release');
    $ret        = $q->FAI_get_packages($release,$attrs,$filter,0,200);
    if($q->is_error()){
      msg_dialog::display(_("Service infrastructure"),msgPool::siError($q->get_error()),ERROR_DIALOG);
    }else{
      foreach($ret as $attr){
        
        $attr['objectClass'] = "FAKE_OC__FaiPackage";
        $attr['DESCRIPTION'] = base64_decode($attr['DESCRIPTION']);

        $item = array();
        $item['dn']   = "dn=".$attr['PACKAGE'].",".$attr['DISTRIBUTION'].",".$base;
        $item['count']= 0;
        foreach($attr as $key => $value){
          $item['count']++;
          $item[] = $key;
          $item[$key] = array('count' => 1, $value);
        }
        $pkgs[] = $item;
      }
    }

    return(filterLDAPBlacklist::filterByBlacklist($pkgs));
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

  function save()
  {
    $act = $this->detectPostActions();
    $headpage = $this->getHeadpage();
    if(!isset($act['targets'])) return(array());
    $ret = array();
    foreach($act['targets'] as $dn){
      $ret[] = $headpage->getEntry($dn);
    }
    return($ret);
  }

}

?>
