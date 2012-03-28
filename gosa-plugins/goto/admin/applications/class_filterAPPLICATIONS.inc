<?php

class filterAPPLICATIONS extends filterLDAP{

  static function query($base, $scope, $filter, $attributes, $category, $objectStorage= "")
  { 
    $flag = GL_FILTER_INCLUDE_EMPTY_TAG | (($scope == "sub")?GL_SUBSEARCH:0);
    $res = filterLDAP::get_list($base, $filter, $attributes, $category, $objectStorage, $flag);

    foreach($res as $id => $entry){
      if(isset($entry['FAIstate']) && preg_match("/freeze/i", $entry['FAIstate'][0])){
        $res[$id]['objectClass'][] = 'FAKE_OC_freeze';
      }else{
        $res[$id]['objectClass'][] = 'FAKE_OC_branch';
      }
    }
    return($res);
  }
}

?>
