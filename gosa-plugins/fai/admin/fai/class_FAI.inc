<?php

define("DEBUG_FAI_FUNC",FALSE);


class FAI
{

  /* TEST PHASE .... */

  static function get_all_objects_for_given_object($dn,$filter,$detailed = false)
  {
    $res  = FAI::get_all_objects_for_given_base($dn,$filter,$detailed);
    $name = preg_replace("/,ou=.*$/","",$dn);
    $entries = array();
    foreach($res as $entry_dn => $data){
      if(!preg_match("/,".$name.",/",$entry_dn)) continue;
      $entries[$entry_dn] = $data; 
    }
    return($entries);
  }




  /* Returns all object for the given release.
     This function resolves the releases  
     from base up to the given dn.
   */
  static function get_all_objects_for_given_base($Current_DN,$filter,$detailed = false)
  {
    global $config;
    $ldap = $config->get_ldap_link();
    $ldap->cd($config->current['BASE']);
    $res = array();
    $tmp = array();

    if(!FAI::is_release_department($Current_DN)) {
#      return($res);
    }

    /* Collect some basic informations and initialize some variables */ 
    $base_release       = FAI::get_release_dn($Current_DN);
    $previous_releases  = array_reverse(FAI::get_previous_releases_of_this_release($base_release,true));

    $ldap->cat($base_release);
    $attrs = $ldap->fetch();
    $FAIstate = "branch";
    if(isset($attrs['FAIstate'][0])){
      $FAIstate = $attrs['FAIstate'][0];
    }

    /* We must also include the given release dn */
    $previous_releases[] = $base_release;

    /* Walk through all releases */
    foreach($previous_releases as $release){

      /* Get fai departments */
      $deps_to_search = FAI::get_FAI_departments($release); 

      /* For every single department  (ou=hoos,ou ..) */
      foreach($deps_to_search as $fai_base){

        /* Ldap search for fai classes specified in this release */
        $attributes  = array("dn","objectClass","FAIstate","cn","FAIdiskType","FAIlvmDevice","FAIdiskOption");
        $res_tmp = get_list($filter,"fai",$fai_base,$attributes,GL_SUBSEARCH);

        /* check the returned objects, and add/replace them in our return variable */
        foreach($res_tmp as $attr){

          $buffer = array();
          $name = preg_replace("/".preg_quote($release, '/')."/i","",$attr['dn']);

          if(isset($attr['FAIstate'][0])){
            if(preg_match("/removed$/",$attr['FAIstate'][0])){
              if(isset($res[$name])){
                unset($res[$name]);
              }
              continue;
            }
          }

          /* Seems to be an inherited class, apply current FAIstate to this classes 
           */
          if(!preg_match("/".preg_quote($base_release, '/')."$/i",$attr['dn'])){
            $buffer['FAIstate'] = $FAIstate; 
          }else{

            /* Seems to be created within this release department.
               This indicates - it can't be of state "freeze"
             */             
            if(isset($attr['FAIstate'])){
              $buffer['FAIstate'] = $attr['FAIstate'][0];
            }else{
              $buffer['FAIstate'] = "branch"; 
            }
          }

          if(isset($attr['FAIdiskType'])){
            $buffer['FAIdiskType'] = $attr['FAIdiskType'][0];
          } else {
            $buffer['FAIdiskType'] = "old";
          }

          if(isset($attr['FAIdiskOption'])){
            $buffer['FAIdiskOption'] = $attr['FAIdiskOption'];
          } else {
            $buffer['FAIdiskOption'] = null;
          }
          if(isset($attr['FAIlvmDevice'])){
            $buffer['FAIlvmDevice'] = $attr['FAIlvmDevice'];
          }

          /* In detailed mode are some additonal informations visible */
          if($detailed){

            /* Create list of parents */
            if(isset($res[$name])){
              $buffer = $res[$name];
              $buffer['parents'][] = $res[$name]['dn'];
            }else{
              $buffer['parents'] = array();
            }

            /* Append objectClass to resulsts */
            foreach($attributes as $val){
              if(isset($attr[$val])){
                $buffer[$val] = $attr[$val];
              }
            }
            unset($buffer['objectClass']['count']);
          }

          /* Add this object to our list */
          $buffer['dn']           = $attr['dn'];
          $res[$name] = $buffer;
        }
      }
    }
    return($res);
  }


  /* Return all relevant FAI departments */
  static function get_FAI_departments($suffix = "")
  {
    global $config;
    $arr = array(
        get_ou('faiScript', 'faiScriptRDN'),
        get_ou('faiHook', 'faiHookRDN'),
        get_ou('faiTemplate', 'faiTemplateRDN'),
        get_ou('faiVariable', 'faiVariableRDN'),
        get_ou('faiProfile', 'faiProfileRDN'),
        get_ou('faiPartitionTable', 'faiPartitionRDN'),
        get_ou('faiPackage', 'faiPackageRDN')
        );

    $tmp = array();
    if(preg_match("/^,/",$suffix)){
      $suffix = preg_replace("/^,/","",$suffix);
    }
    foreach($arr as $name){
      if(empty($suffix)){
        $tmp[$name] = $name;
      }else{
        $tmp[$name] = $name.$suffix;
      }
    }
    return($tmp);
  }


  /* Return all releases within the given base */
  static function get_all_releases_from_base($dn,$appendedName=false)
  {
    global $config;

    if(!preg_match("/".preg_quote(get_ou("faiManagement", "faiBaseRDN"), '/')."/i",$dn)){
      $base = get_ou("faiManagement", "faiBaseRDN").$dn;
    }else{
      $base = $dn;
    }
    $res = array();  

    $ldap = $config->get_ldap_link();
    $ldap->cd($base);
    $ldap->search("(objectClass=FAIbranch)",array("ou","dn"));
    while($attrs = $ldap->fetch()){
      if($appendedName){
        $res[$attrs['dn']] = convert_department_dn(preg_replace("/,".preg_quote(get_ou("faiManagement", "faiBaseRDN"), '/').".*$/i","",$attrs['dn']));
      }else{
        $res[$attrs['dn']] = $attrs['ou'][0];
      }
    }
    return($res);
  }


  /* Add this object to list of objects, that must be checked for release saving */
  static function prepare_to_save_FAI_object($Current_DN,$objectAttrs,$removed = false)
  {
    /* Get ldap object */  
    global $config;
    $addObj['Current_DN'] = $Current_DN;
    $addObj['objectAttrs']= $objectAttrs;
    $addObj['removed']    = $removed;
    $addObj['diff']       = TRUE;

    if(!$removed){
      $ldap = $config->get_ldap_link();
      $ldap->cd($config->current['BASE']);

      /* Get some basic informations */
      $parent_obj   = FAI::get_parent_release_object($Current_DN);
      if(!empty($parent_obj)){
        $ldap->cat($parent_obj,array("*"));
        $attrs = FAI::                           prepare_ldap_fetch_to_be_saved($ldap->fetch());

        if(!FAI::array_diff_FAI( $attrs,$objectAttrs)){
          $addObj['diff'] = FALSE;
        }
      } 
    }else{

      /* If this is the last CLASS of a specific name (e.g. DEPOTSERVER)
          we have to remove this name from all profiles in this release.
      */
      $ldap = $config->get_ldap_link();
      $ldap->cd($config->current['BASE']);
      $obj_dn = FAI::get_parent_release_object($Current_DN,TRUE);

      /* Dont't try to modify non FAIclasses  
       */
      if(!preg_match("/[^,]+,".preg_quote(get_ou("faiManagement", "faiBaseRDN"), '/')."/i",$obj_dn)){
        trigger_error("PLEASE check fai class handling in ".__LINE__." -> ".__FILE__);        
        echo "<br>-->".$Current_DN."<br>";
        echo "<br>-->".$obj_dn."<br>";
      }else{

        /* Get source object and check if it is a base FAIclass
         */
        $ldap->cat($obj_dn);
        $attrs = $ldap->fetch();
        $classes = array("FAIprofile","FAIscript","FAIpackageList","FAIpartitionTable","FAIHook","FAIvariable","FAItemplate");
        if(count(array_intersect($classes,$attrs['objectClass']))){
          $cn    = $attrs['cn'][0];

          /* Check if this is the last with this name in the current release.
              In this case we have to remove the package name 
              from all profiles in this release.
           */
          $classes = FAI::get_all_objects_for_given_base(FAI::get_release_dn($Current_DN),
              "(&(objectClass=FAIclass)(cn=".$cn."))"); 

          /* Check if this is the last class with this name */
          if(count($classes) == 1){

            /* Get all FAI Profiles 
             */
            $profiles = FAI::get_all_objects_for_given_base(FAI::get_release_dn($Current_DN),
                "(&(objectClass=FAIprofile)(FAIclass=*))"); 

            /* Walk though all profiles and remove the source class name
             */
            foreach($profiles as $dn){
              $ldap->cat($dn['dn']);
              $attrs = $ldap->fetch();

              $attrs = array('FAIclass' => $attrs['FAIclass'][0]);

              /* Check if this Profile uses the source class ($cn)
               */
              $classlist = split(" ", $attrs['FAIclass']);
              $new_classlist = "";
              foreach($classlist as $class){
                  if($class != $cn){
                      $new_classlist = $new_classlist." ".$class;
                  }
              } 
              $attrs['FAIclass'] = $new_classlist;
              if(empty($attrs['FAIclass'])){
                  $attrs['FAIclass'] = array();
              }
              $ldap->cd($dn['dn']);
              $ldap->modify($attrs);
              if (!$ldap->success()){
                  msg_dialog::display(_("LDAP error"), msgPool::ldaperror($ldap->get_error(), $this->dn, LDAP_MOD, get_class()));
              }
            }
          }
        }
      }
    }



    $FAI_objects_to_save = session::get('FAI_objects_to_save') ;
    $FAI_objects_to_save[$Current_DN] =  $addObj;
    session::set('FAI_objects_to_save',$FAI_objects_to_save);
  }


  /* Detect differences in attribute arrays  */
  static function array_diff_FAI($ar1,$ar2)
  {

    if((!isset($ar1['description'])) || (isset($ar1['description']) && (count($ar1['description']) == 0))){
      $ar1['description'] = "";
    }
    if((!isset($ar2['description'])) || (isset($ar2['description']) && (count($ar2['description']) == 0))){
      $ar2['description'] = "";
    }

    if(count($ar1) != count($ar2)) {
      return (true);
    }

    foreach($ar1 as $key1 => $val1){

      if((is_array($val1)) && (count($val1)==1)){
        $ar1[$key1] = $val1[0];
      }

      if(isset($ar2[$key1])&&  (is_array($ar2[$key1])) && (count($ar2[$key1])==1)){
        $val1 = $val1[0];
        $ar2[$key1] = $ar2[$key1][0];
      }
    }
    ksort($ar1);
    ksort($ar2);
    if(count( array_diff($ar1,$ar2)) || FAI::arr_diff($ar1,$ar2)){
      return(true);
    }else{
      return(false);
    }
  }


  static function arr_diff($ar1,$ar2)
  {
    foreach($ar1 as $ak1 => $av1){
      if(!isset($ar2[$ak1]) || (!($av1 === $ar2[$ak1]))){
        return(TRUE);    
      }elseif(is_array($av1)){
        $ret = (FAI::arr_diff($av1,$ar2[$ak1]));
        if($ret) {
          return(TRUE);
        }
      }
    }
    return(FALSE);
  }




  /* check which objects must be saved, and save them */
  static function save_release_changes_now()
  {
    global $config;
    /* Variable init*/
    $to_save = array();
    
    $reload_fai_classes = FALSE;

    /* check which objects must be saved */
    if(!session::is_set('FAI_objects_to_save')){
      return;
    }
    $FAI_objects_to_save = session::get('FAI_objects_to_save');
    if(!is_array($FAI_objects_to_save)) {
      print_a(array(session::get('FAI_objects_to_save')));
      trigger_error("Can't save FAI objects, no array given.");
      return;
    }
  
    foreach($FAI_objects_to_save as $Current_DN => $object){
      if($object['diff']){
        $sub_name = $Current_DN;
        while(isset($FAI_objects_to_save[$sub_name])){
          $to_save[strlen($sub_name)][$sub_name] = $FAI_objects_to_save[$sub_name]; 
          unset($FAI_objects_to_save[$sub_name]);
          $sub_name = preg_replace('/^[^,]+,/', '', $sub_name);
        }
      }
    }
    session::set('FAI_objects_to_save',$FAI_objects_to_save);

    /* Sort list of objects that must be saved, and ensure that 
       container   objects are safed, before their childs are saved */
    ksort($to_save);
    $tmp = array();
    foreach($to_save as $SubObjects){
      foreach($SubObjects as $object){
        $tmp[] = $object;
      }
    }
    $to_save = $tmp;


    /* Save objects and manage the correct release behavior*/
    foreach($to_save as $save){

      $Current_DN = $save['Current_DN'];
      $removed    = $save['removed'];
      $objectAttrs= $save['objectAttrs'];

      /* Get ldap object */ 
      $ldap = $config->get_ldap_link();
      $ldap->cd($config->current['BASE']);

      /* Get some basic informations */
      $base_release       = FAI::get_release_dn($Current_DN);
      $sub_releases       = FAI::get_sub_releases_of_this_release($base_release,true);
      $parent_obj         = FAI::get_parent_release_object($Current_DN);
      $following_releases = $sub_releases;

      /* Check if given dn exists or if is a new entry */
      $ldap->cat($Current_DN);
      if(!$ldap->count()){
        $is_new = true;
      }else{
        $is_new = false;
      }

      /* if parameter removed is true, we have to add FAIstate to the current attrs 
         FAIstate should end with ...|removed after this operation */  
      if($removed ){
        $ldap->cat($Current_DN);

        /* Get current object, because we must add the FAIstate ...|removed */
        if((!$ldap->count()) && !empty($parent_obj)){
          $ldap->cat($parent_obj);
        }

        /* Check if we have found a suiteable object */ 
        if(!$ldap->count()){
          echo "Error can't remove this object ".$Current_DN;
          return;
        }else{

          /* Set FAIstate to current objectAttrs */
          $objectAttrs = FAI::                           prepare_ldap_fetch_to_be_saved($ldap->fetch());
          if(isset($objectAttrs['FAIstate'][0])){
            if(!preg_match("/removed$/",$objectAttrs['FAIstate'][0])){
              $objectAttrs['FAIstate'][0] .= "|removed";
            }
          }else{
            $objectAttrs['FAIstate'][0] = "|removed";
          }

          /* Force reload of FAI classes */
          $classes = array("FAIprofile","FAIscript","FAIpackageList","FAIpartitionTable","FAIHook","FAIvariable","FAItemplate");
          if(count(array_intersect($classes,$objectAttrs['objectClass']))){
            $reload_fai_classes = TRUE;
          }
        }
      }

      /* Check if this a leaf release or not */ 
      if(count($following_releases) == 0 ){

        /* This is a leaf object. It isn't inherited by any other object */    
        if(DEBUG_FAI_FUNC) { 
          echo "<b>Saving directly, is a leaf object</b><br> ".$Current_DN;
          print_a($objectAttrs);
        }
        FAI::save_FAI_object($Current_DN,$objectAttrs);

        /* Force reload of FAI classes */
        $classes = array("FAIprofile","FAIscript","FAIpackageList","FAIpartitionTable","FAIHook","FAIvariable","FAItemplate");
        if(count(array_intersect($classes,$objectAttrs['objectClass']))){
          $reload_fai_classes = TRUE;
        }

      }else{

        /* This object is inherited by some sub releases */  

        /* Get all releases, that inherit this object */ 
        $r = FAI::get_following_releases_that_inherit_this_object($Current_DN);

        /* Get parent object */
        $ldap->cat($parent_obj);
        $parent_attrs = FAI::prepare_ldap_fetch_to_be_saved($ldap->fetch());

        /* New objects require special handling */
        if($is_new){

          /* Force reload of FAI classes */
          $classes = array("FAIprofile","FAIscript","FAIpackageList","FAIpartitionTable","FAIHook","FAIvariable","FAItemplate");
          if(count(array_intersect($classes,$objectAttrs['objectClass']))){
            $reload_fai_classes = TRUE;
          }

          /* check if there is already an entry named like this,
             in one of our parent releases */
          if(!empty($parent_obj)){
            if(DEBUG_FAI_FUNC) { 
              echo "There is already an entry named like this.</b><br>";

              echo "<b>Saving main object</b>".$Current_DN;
              print_a($objectAttrs);
            }    
            FAI::save_FAI_object($Current_DN,$objectAttrs);

            foreach($r as $key){
              if(DEBUG_FAI_FUNC) { 
                echo "<b>Saving parent to following release</b> ".$key;
                print_a($parent_attrs);
              }
              FAI::save_FAI_object($key,$parent_attrs);
            }
          }else{

            if(DEBUG_FAI_FUNC) { 
              echo "<b>Saving main object</b>".$Current_DN;
              print_a($objectAttrs);
            }
            FAI::save_FAI_object($Current_DN,$objectAttrs);

            if(isset($objectAttrs['FAIstate'])){
              $objectAttrs['FAIstate'] .= "|removed"; 
            }else{
              $objectAttrs['FAIstate'] = "|removed";
            }

            foreach($r as $key ){
              if(DEBUG_FAI_FUNC) { 
                echo "<b>Create an empty placeholder in follwing release</b> ".$key; 
                print_a($objectAttrs);
              }
              FAI::save_FAI_object($key,$objectAttrs);
            }
          }
        }else{

          /* check if we must patch the follwing release */
          if(!empty($r)){

            foreach($r as $key ){
              if(DEBUG_FAI_FUNC) { 
                echo "<b>Copy current objects original attributes to next release</b> ".$key;
                print_a($parent_attrs);
              }
             
              /* Append FAIstate tag to ensure that freezed objects stay freezed
               */ 
              $rTag = FAI::get_release_tag(FAI::get_release_dn($key));
              $parent_attrs['FAIstate'] = $rTag;

              /* FAItemplateFile can be binary, therefore it needs to be fetched with
               * $ldap->get_attribute */
              if (isset($parent_attrs['FAItemplateFile'])) {
                $parent_attrs['FAItemplateFile'] = $ldap->get_attribute($parent_obj, 'FAItemplateFile');
              }

              FAI::save_FAI_object($key,$parent_attrs);
            }
          }

          if(DEBUG_FAI_FUNC) { 
            echo "<b>Saving current object</b>".$parent_obj;
            print_a($objectAttrs);
          }
          FAI::save_FAI_object($parent_obj,$objectAttrs);

          if(($parent_obj != $Current_DN)){
            msg_dialog::display(_("Error"), sprintf(_("Error, following objects should be equal '%s' and '%s'"),$parent_obj,$Current_DN), ERROR_DIALOG);
          }
        }
      }
    }

    /* Reload GOsa si FAI DB/cache
     */
    if($reload_fai_classes){
      if( class_available("DaemonEvent") && class_available("gosaSupportDaemon")){
        $events = DaemonEvent::get_event_types(SYSTEM_EVENT | HIDDEN_EVENT);        
        if(isset($events['TRIGGERED']['DaemonEvent_recreate_fai_release_db'])){
          $evt = $events['TRIGGERED']['DaemonEvent_recreate_fai_release_db']; 
          $tmp = new $evt['CLASS_NAME']($config);
          $tmp->set_type(TRIGGERED_EVENT);
          $tmp->add_targets(array("GOSA"));
          $o_queue = new gosaSupportDaemon();
          if(!$o_queue->append($tmp)){
            msg_dialog::display(_("Service infrastructure"),msgPool::siError($o_queue->get_error()),ERROR_DIALOG);
          }
        }
      }
    }

    session::set('FAI_objects_to_save',array());
  }


  /* this function will remove all unused (deleted) objects,
     that have no parent object */
  static function clean_up_releases($Current_DN)
  {
    global $config;
    $ldap = $config->get_ldap_link();
    $ldap->cd($config->current['BASE']);

    /* Collect some basic informations and initialize some variables */ 
    $base_release       = FAI::get_release_dn($Current_DN);
    $previous_releases  = array_reverse(FAI::             get_previous_releases_of_this_release($base_release,true));
    $Kill = array();
    $Skip = array();

    /* We must also include the given release dn */
    $previous_releases[] = $base_release;

    /* Walk through all releases */
    foreach($previous_releases as $release){

      /* Get fai departments */
      $deps_to_search = FAI::get_FAI_departments($release); 

      /* For every single department  (ou=hoos,ou ..) */
      foreach($deps_to_search as $fai_base){

        /* Ldap search for fai classes specified in this release */
        $ldap->cd($fai_base);
        $ldap->search("(objectClass=FAIclass)",array("dn","objectClass","FAIstate"));

        /* check the returned objects, and add/replace them in our return variable */
        while($attr = $ldap->fetch()){

          $buffer = array();
#        $name = str_ireplace($release,"",$attr['dn']);
          $name = preg_replace("/".preg_quote($release, '/')."/i","",$attr['dn']);

          if(isset($attr['FAIstate'][0])&&(preg_match("/removed$/",$attr['FAIstate'][0]))){

            /* Check if this object is required somehow */    
            if(!isset($Skip[$name])){
              $Kill[$attr['dn']] = $attr['dn'];
            }
          }else{

            /* This object is required (not removed), so do not 
               delete any following sub releases of this object */
            $Skip[$name] = $attr['dn'];
          }
        }
      }
    }
    return($Kill);
  }


  /* Remove numeric index and 'count' from ldap->fetch result */
  static function prepare_ldap_fetch_to_be_saved($attrs)
  {
    foreach($attrs as $key => $value){
      if(is_numeric($key) || ($key == "count") || ($key == "dn")){
        unset($attrs[$key]);
      }
      if(is_array($value) && isset($value['count'])){
        unset($attrs[$key]['count']);
      }
    }
    return($attrs);
  }


  /* Save given attrs to specified dn*/
  static function save_FAI_object($dn,$attrs)
  {
    global $config;
    $ldap = $config->get_ldap_link();
    $ldap->cd($config->current['BASE']);
    $ldap->create_missing_trees(preg_replace('/^[^,]+,/', '', $dn));
    $ldap->cd($dn);

    $ldap->cat($dn,array("dn"));
    if($ldap->count()){

      /* Remove FAIstate*/
      if(!isset($attrs['FAIstate'])){
        $attrs['FAIstate'] = array();
      }

      $ldap->modify($attrs);
    }else{

      /* Unset description if empty  */
      if(empty($attrs['description'])){
        unset($attrs['description']);
      }    

      $ldap->add($attrs);
    }
    if (!$ldap->success()){
      msg_dialog::display(_("LDAP error"), msgPool::ldaperror($ldap->get_error(), $dn, 0, get_class()));
    }
  }


  /* Return FAIstate freeze branch or "" for specified release department */
  static function get_release_tag($dn)
  {
    global $config;
    $ldap = $config->get_ldap_link();
    $ldap->cd($dn);
    $ldap->cat($dn,array("FAIstate"));

    if($ldap->count()){

      $attr = $ldap->fetch();
      if(isset($attr['FAIstate'][0])){
        if(preg_match("/freeze/",$attr['FAIstate'][0])){
          return("freeze");
        }elseif(preg_match("/branch/",$attr['FAIstate'][0])){
          return("branch");
        }
      }
    }
    return("");
  }


  static function get_following_releases_that_inherit_this_object($dn)
  {
    global $config;
    $ldap = $config->get_ldap_link();
    $ldap->cd($config->current['BASE']);

    $ret = array();

    /* Get base release */
    $base_release = FAI::get_release_dn($dn);

    /* Get previous release dns */
    $sub_releases = FAI::                       get_sub_releases_of_this_release($base_release);

    /* Get dn suffix. Example  "FAIvairableEntry=keksdose,FAIvariable=Keksregal," */
#  $dn_suffix = str_ireplace($base_release,"",$dn);
    $dn_suffix = preg_replace("/".preg_quote($base_release, '/')."/i","",$dn);

    /* Check if given object also exists whitin one of these releases */
    foreach($sub_releases as $p_release => $name){

      $check_dn = $dn_suffix.$p_release;

      $ldap->cat($check_dn,array("dn","objectClass"));

      if($ldap->count()){
        //return($ret);
      }else{
        $ret[$check_dn]=$check_dn;
      }
    }
    return($ret);
  }


  /* Get previous version of the object dn */
  static function get_parent_release_object($dn,$include_myself=true)
  {
    global $config;
    $ldap = $config->get_ldap_link();
    $ldap->cd($config->current['BASE']);
    $previous_releases= array();

    /* Get base release */
    $base_release = FAI::get_release_dn($dn);
    if($include_myself){
      $previous_releases[] = $base_release;  
    }

    /* Get previous release dns */
    $tmp = FAI::             get_previous_releases_of_this_release($base_release,true);
    foreach($tmp as $release){
      $previous_releases[] = $release;
    }

    /* Get dn suffix. Example  "FAIvairableEntry=keksdose,FAIvariable=Keksregal," */
#  $dn_suffix = str_ireplace($base_release,"",$dn);
    $dn_suffix = preg_replace("/".preg_quote($base_release, '/')."/i","",$dn);

    /* Check if given object also exists whitin one of these releases */
    foreach($previous_releases as $p_release){
      $check_dn = $dn_suffix.$p_release;
      $ldap->cat($check_dn,array("dn","objectClass"));

      if($ldap->count()){
        return($check_dn);
      }
    }
    return("");
  }


  /* return release names of all parent releases */
  static function get_previous_releases_of_this_release($dn,$flat)
  {
    global $config;
    $ldap = $config->get_ldap_link();
    $ldap->cd($config->current['BASE']);
    $ret = array();

    /* Explode dns into pieces, to be able to build parent dns */
    $dns_to_check = gosa_ldap_explode_dn(preg_replace("/".preg_quote(",".$config->current['BASE'], '/')."/i","",$dn));

    if(!is_array($dns_to_check)){
      return;  
    }

    /* Unset first entry which represents the given dn */
    unset($dns_to_check['count']); 
    unset($dns_to_check[key($dns_to_check)]);

    /* Create dns addresses and check if this dn is a release dn */
    $id = 0;
    while(count($dns_to_check)){

      /* build parent dn */
      $new_dn = "";
      foreach($dns_to_check as $part){
        $new_dn .= $part.",";
      }
      $new_dn .= $config->current['BASE'];

      /* check if this dn is a release */
      if(FAI::is_release_department($new_dn)){
        if($flat){
          $ret[$id] = $new_dn; 
        }else{
          $ret = array($new_dn=>$ret); 
        }
        $id ++;
      }else{
        return($ret);
      }
      reset($dns_to_check);
      unset($dns_to_check[key($dns_to_check)]);
    }
    return($ret);
  } 


  /* This function returns all sub release names, recursivly  */
  static function get_sub_releases_of_this_release($dn,$flat = false)
  {
    global $config;
    $res  = array();
    $ldap = $config->get_ldap_link();
    $ldap->cd($config->current['BASE']);
    $ldap->ls("(objectClass=FAIbranch)",$dn,array("objectClass","dn","ou"));
    while($attr = $ldap->fetch()){

      /* Append department name */
      if($flat){
        $res[$attr['dn']] = $attr['ou'][0];
      }else{
        $res[$attr['dn']] = array();
      }

      /* Get sub release departments of this department */
      if(in_array("FAIbranch",$attr['objectClass'])) {
        if($flat){
          $tmp = FAI::                       get_sub_releases_of_this_release($attr['dn'],$flat);
          foreach($tmp as $dn => $value){
            $res[$dn]=$value;
          }
        }else{
          $res[$attr['dn']] = FAI::                       get_sub_releases_of_this_release($attr['dn']);
        }
      }
    }
    return($res);
  }


  /* Check if the given department is a release department */
  static function is_release_department($dn)
  {
    global $config;
    $ldap = $config->get_ldap_link();
    $ldap->cd($config->current['BASE']);
    $ldap->cat($dn,array("objectClass","ou"));

    /* Check objectClasses and name to check if this is a release department */
    if($ldap->count()){
      $attrs = $ldap->fetch();

      $ou = "";
      if(isset($attrs['ou'][0])){
        $ou = $attrs['ou'][0];	
      }

      if((in_array("FAIbranch",$attrs['objectClass'])) || ($ou == "fai")){
        return($attrs['dn']);
      }
    }
    return(false);
  }


  static function copy_FAI_group_releases($source_release , $destination_name, $type ="" )
  {
    global $config;
    $start = microtime(TRUE);
    $source_release = trim($source_release,"/");

    echo "<h3>".sprintf(_("Creating group application release for %s"),$destination_name)."</h3>";

    $sub_releases = array();
    $source_dn = "";
    
    $tmp = explode("/",$source_release);
    foreach($tmp as $part){
      if(empty($part)){
        continue;
      }
      $source_dn            = "ou=".$part.",".$source_dn;
      $sub_releases[$part]  = $source_dn;
    }

    /* Get all groups */
    $ldap =$config->get_ldap_link();
    $ldap->cd($config->current['BASE']);
    $ldap->search("(objectClass=posixGroup)",array("dn"));
    $groups = array();
    while($attrs = $ldap->fetch()){
      $groups[$attrs['dn']] = $attrs;
    }

    /* Get all FAI releases, to be able to create missing group application releases 
        with the correct type of release (FAIstate=freeze/branch).
     */
    $f_releases = array();
    $ldap->cd ($config->current['BASE']);
    $ldap->search("(objectClass=FAIbranch)",array("ou","FAIstate"));
    while($attrs = $ldap->fetch()){
      foreach($sub_releases as $sub_rel){
        if(preg_match("/^".preg_quote($sub_rel.get_ou("faiManagement", "faiBaseRDN"), '/')."/i",$attrs['dn'])){
          $f_releases[$sub_rel.get_ou("faiManagement", "faiBaseRDN")] = $attrs;
        }
      }
    }

    /* Get all group releases */
    $g_releases = array();
    foreach($groups as $dn => $data){
      $ldap->cd($dn);
      $ldap->search("(objectClass=FAIbranch)",array("ou","FAIstate"));
      while($attrs = $ldap->fetch()){
        $g_releases[$attrs['dn']] = $attrs;
      }
    }

    /* Check if base releases exists.
       If they do not exist, create them and adapt FAIstate attribute from FAI releases. 
     */
    foreach($sub_releases as $name => $sub_rel){

      $FAIstate = "";
      if(isset($f_releases[$sub_rel.get_ou("faiManagement", "faiBaseRDN")]) && isset($f_releases[$sub_rel.get_ou("faiManagement", "faiBaseRDN")]['FAIstate'])){
        $FAIstate = $f_releases[$sub_rel.get_ou("faiManagement", "faiBaseRDN")]['FAIstate'][0];
      }

      foreach($groups as $dn => $data){
        if(!isset($g_releases[$sub_rel.$dn])){
          $ldap->cd($dn);
          $r_data = array();
          $r_data['ou'] = $name;
          $r_data['objectClass'] = array("top","organizationalUnit","FAIbranch");
          if(!empty($FAIstate)) {
            $r_data['FAIstate'] = $FAIstate;
          }
 
          $ldap->cd($sub_rel.$dn) ;
          $ldap->add($r_data);
          echo "&nbsp;<b>"._("Object").":</b> ";
          echo sprintf(_("Adding missing group application release container %s."),substr(LDAP::fix($sub_rel.$dn),0,96))."<br>";
          flush();
        }
      }
    } 
 
    /* Create new release container in each group.
     */
    $n_data = array();
    $n_data = array();
    $n_data['ou'] = $destination_name;
    $n_data['objectClass'] = array("top","organizationalUnit","FAIbranch");
    if(!empty($type)){
      $n_data['FAIstate'] = $type."/cow";
    }

    foreach($groups as $dn => $att){
      $n_dn = "ou=".$destination_name.",".$source_dn.$dn;
      if(!isset($g_releases[$n_dn])){
        $ldap->cd ($n_dn);
        $ldap->add($n_data);
        echo "&nbsp;<b>"._("Object").":</b> ";
        echo sprintf(_("Adding group application release container %s."),substr(LDAP::fix($n_dn),0,96))."<br>";
        flush();
      }
    }

    /* If the source release is empty, then create a new release by copying 
        all group application menus into a new ou=$destination_name release container.
      
       If the source release is not empty. 
         We detect all releases which match the source release dn and copy the contents.
     */
    if(empty($source_release)){
      $source_dns = $groups;
    }else{
      $source_dns = array();
      foreach($g_releases as $dn => $data){
        if(preg_match("/^".preg_quote($source_dn, '/')."/",$dn)){
          $source_dns[$dn] = $data; 
        }
      }
    }

    /* Detect all menu object we have to copy 
     */
    $to_copy = array();
    foreach($source_dns as $dn => $attrs){
      $ldap->cd($dn);
      $ldap->ls("(|(objectClass=gotoSubmenuEntry)(objectClass=gotoMenuEntry))",$dn,array("dn"));
      while($attrs = $ldap->fetch()){
        $destination = preg_replace("/".preg_quote($dn, '/')."$/","ou=".$destination_name.",".$dn,$attrs['dn']);
        $to_copy[$attrs['dn']] = $destination;
      }
    }
   
    /* At least create the menu objects object */
    $plug = new plugin($config);
    foreach($to_copy as $source => $destination){
      $ldap->cat($destination);
      if($ldap->count()){
        echo "&nbsp;<b>"._("Object").":</b> ";
        echo sprintf(_("Could not create menu entry %s. (Already exists)."),substr(LDAP::fix($destination),0,96))."<br>";
        flush();
      }else{
        $plug->copy($source,$destination);
        echo "&nbsp;<b>"._("Object").":</b> ";
        echo sprintf(_("Created group application menu entry for %s."),substr(LDAP::fix($destination),0,96))."<br>";
        flush();
      }
    }
  }


  /*! \brief Create a new FAI branch.
   *  @param $sourcedn          String  The source release dn
   *  @param $destinationdn     String  The destination dn
   *  @param $destinationName   String  The name of the new release
   *  @param $type              String  The release type (freeze/branch)
   *  @param $is_first          Boolean Use to identify the first func. call when recursivly called.
   *  @param $depth             Integer Current depth of recursion.
   */
  static function copy_FAI_resource_recursive($sourcedn,$destinationdn,$destinationName,$type="branch",$is_first = true,$depth=0)
  {
    global $config;
    error_reporting(E_ALL | E_STRICT);
    $ldap     = $config->get_ldap_link();
    $basedn   = $config->current['BASE'];
    $delarray = array();

    /* The following code will output a status string
     *  for each handled object, in a seperate iframe.
     */


    /* Display current action information.
     */
    if($is_first){
      echo "<h3>".sprintf(_("Creating copy of %s"),"<i>".LDAP::fix($sourcedn)."</i>")."</h3>";
    }else{
      if(preg_match("/^ou=/",$sourcedn)){
        echo "<h3>"._("Processing")." <i>".LDAP::fix($destinationdn)."</i></h3>";
      }else{
        $tmp = explode(",",$sourcedn);
        echo "&nbsp;<b>"._("Object").":</b> ";
        $deststr = LDAP::fix($destinationdn);
        if(strlen($deststr) > 96){
          $deststr = substr($deststr,0,96)."...";
        }
        echo $deststr."<br>";
      }
    }
    /* .. immediately display infos */
    flush();

    /* Check if destination entry already exists
     */
    $ldap->cat($destinationdn);
    if($ldap->count()){
      echo _("Could not create new release, the destination DN is already in use.");
      return;
    }else{

      $ldap->clearResult();

      /* Get source entry
       *  if it does not exist, abort here.
       */
      $ldap->cd($basedn);
      $ldap->cat($sourcedn);
      $attr = $ldap->fetch();
      if((!$attr) || (count($attr)) ==0) {
        echo _("Error while fetching source DN - aborted!");
        return;
      }

      /* The current object we want to create is an department.
       * Create the department and add the FAIbranch tag.
       */
      if(in_array("organizationalUnit",$attr['objectClass'])){
        $attr['dn'] = LDAP::convert($destinationdn);
        $ldap->cd($basedn);
        $ldap->create_missing_trees($destinationdn);
        $ldap->cd($destinationdn);

        /* If is first entry, append FAIbranch to department entry */
        if($is_first){
          $ldap->cat($destinationdn);
          $attr= $ldap->fetch();
          /* Filter unneeded informations */
          foreach($attr as $key => $value){
            if(is_numeric($key)) unset($attr[$key]);
            if(isset($attr[$key]['count'])){
              if(is_array($attr[$key])){
                unset($attr[$key]['count']);
              }
            }
          }

          unset($attr['count']);
          unset($attr['dn']);

          /* Add marking attribute */
          $attr['objectClass'][] = "FAIbranch";
          $attr['FAIstate'] = $type;

          /* Add this entry */
          $ldap->modify($attr);
        }
      }else{

        /* Replicate all relevant FAI objects here.
         * FAI objects, Apps and Mimetypes.
         * Get all attributes as binary value, to ensure that Icon, File template aso
         *  are created correctly.
         */
        foreach($attr as $key => $value){

          if(in_array($key ,array("gotoLogonScript", "gosaApplicationIcon","gotoMimeIcon"))){
            $sr= ldap_read($ldap->cid, LDAP::fix($sourcedn), "$key=*", array($key));
            $ei= ldap_first_entry($ldap->cid, $sr);
            if ($tmp= @ldap_get_values_len($ldap->cid, $ei,$key)){
              $attr[$key] = $tmp;
            }
          }

          if(is_numeric($key)) unset($attr[$key]);
          if(isset($attr[$key]['count'])){
            if(is_array($attr[$key])){
              unset($attr[$key]['count']);
            }
          }
        }
        unset($attr['count']);
        unset($attr['dn']);
        if(!in_array("FAIobject",$attr['objectClass'])){
          $attr['objectClass'][] = "FAIobject";
        }
        $attr['FAIstate'] = $type;

        /* Add entry
         */
        $ldap->cd($destinationdn);
        $ldap->cat($destinationdn);

        $a = $ldap->fetch();
        if(!count($a)){
          $ldap->add($attr);
        }

        if(!$ldap->success()){

          /* Some error occurred */
          msg_dialog::display(_("Fatal error"),
              sprintf(_("Release creation failed due to LDAP errors: %s"),
                $ldap->get_error()."<br>".$sourcedn."<br>".$destinationdn."<br>"),FATAL_ERROR_DIALOG);
          exit();
        }
      }
    }

    echo "<script language=\"javascript\" type=\"text/javascript\">scrollDown2();</script>" ;

    /* Prepare for recursive copy.
     * Get all object within the source dn and
     *  call the recursive copy for each.
     */
    $ldap->ls ("(objectClass=*)",$sourcedn);
    while ($ldap->fetch()){
      $deldn= $ldap->getDN();
      $delarray[$deldn]= strlen($deldn);
    }
    asort ($delarray);
    reset ($delarray);
    $depth ++;
    foreach($delarray as $dn => $bla){
      if($dn != $destinationdn){
        $ldap->cd($basedn);
        $item = $ldap->fetch($ldap->cat($dn));
        if(!in_array("FAIbranch",$item['objectClass'])){
          FAI::copy_FAI_resource_recursive($dn,str_replace($sourcedn,$destinationdn,$dn),$destinationName,$type,false,$depth);
        }
      }
    }
    if($is_first){
      echo "<hr>";
    }
  }



  /* This function returns the dn of the object release */
  static function get_release_dn($Current_DN)
  {
    global $config;
    $ldap = $config->get_ldap_link();
    $ldap->cd($config->current['BASE']);

    /* Split dn into pices */ 
    $dns_to_check = gosa_ldap_explode_dn(preg_replace("/".preg_quote(",".$config->current['BASE'], '/')."/i","",$Current_DN));

    if(!is_array($dns_to_check)){
      return;  
    }

    /* Use dn pieces, to create sub dns like 
       ou=test,ou=1,ou=0...
       ou=1,ou=0...
       ou=0... 
       To check which dn is our release container.
     */
    unset($dns_to_check['count']); 
    while(count($dns_to_check)){

      /* Create dn */
      $new_dn = "";
      foreach($dns_to_check as $part){
        $new_dn .= $part.",";
      }
      $new_dn .= $config->current['BASE'];

      /* Check if this dn is a release dn */
      if(FAI::is_release_department($new_dn)){
        return($new_dn);
      }

      /* Remove first element of dn pieces */
      reset($dns_to_check);
      unset($dns_to_check[key($dns_to_check)]);
    }
    return("");
  }
}




// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
