<?php

class faiPackage extends plugin
{
  /* attribute list for save action */
  var $ignore_account   = TRUE;

  /* Attributes for this Object */
  var $attributes       = array("cn","description","FAIpackage","FAIdebianRelease","FAIdebianSection", "FAIinstallMethod");

  /* ObjectClasses for this Object*/
  var $objectclasses    = array("top","FAIclass","FAIpackageList","FAIrepository");

  /* Class name of the Ldap ObjectClass for the Sub Object */
  var $subClass         = "";
  var $subClasses       = array("top","FAIclass","FAIscriptEntry");

  /* Class name of the php class which allows us to edit a Sub Object */
  var $subClassName     = "";      

  /* Attributes to initialise for each subObject */
  var $subAttributes    = array("cn","description"); 
  var $sub64coded       = array();

  var $ConfiguredPackages = array();
  var $ConfigurationsToRemove = array();
  var $Removal = array();

  /* Specific attributes */
  var $cn               = "";       // The class name for this object
  var $description      = "";       // The description for this set of partitions
  var $is_dialog        = false;    // specifies which buttons will be shown to save or abort
  var $SubObjects       = array();  // All leafobjects of this object

  var $FAIdebianRelease          = ""; // The selected release
  var $FAIdebianSection          = array(); // selected section
  var $FAIinstallMethod          = "aptitude"; // hard coded

  var $sections         = array();  // All section types

  var $list             = NULL;

  var $usedPackages     = array();
  var $buffer           = NULL; 
  var $newDialogShown   = false;
  var $direct_packages_add = FALSE;

  var $FAIstate         = "";
  var $view_logged      = FALSE;
  var $base;
  var $FAIpackage ;

  var $packageSelect ;

  var $FAIinstallMethods  = array( "install", "ninstall", "remove", 
      "dselect-upgrade", "taskinst", "taskrm",
      "hold", "clean", "aptitude", "aptitude-r",
      "pending", "dpkgc" );

  var $configure_dialog_shown = FALSE;
  var $debconf_configurations = FALSE;

  function faiPackage (&$config, $dn= NULL)
  {
    /* Load Attributes */
    plugin::plugin ($config, $dn);

    /* If "dn==new" we try to create a new entry
     * Else we must read all objects from ldap which belong to this entry.
     * First read SubObjects from ldap ... and then the partition definitions for the SubObjects.
     */
    if($dn != "new"){
      $this->dn =$dn;

      /* Check if there are already some packages in this list 
       */
      $this->usedPackages = array();
      if(isset($this->attrs['FAIpackage'])){
        unset($this->attrs['FAIpackage']['count']);
        foreach($this->attrs['FAIpackage'] as $pkg){
          $name = preg_replace("/\-$/","",$pkg);
          $this->usedPackages[$name] = $pkg;
        }
        ksort($this->usedPackages);
      }  

      /* Fetch package configurations from LDAP */
      $this->ConfiguredPackages = $this->get_configured_packages($this->usedPackages, $this->dn);

      $this->FAIdebianSection = array();
      if(isset($this->attrs['FAIdebianSection'])){
        for($i = 0 ; $i < $this->attrs['FAIdebianSection']['count'] ; $i++ ){ 
          $sec = $this->attrs['FAIdebianSection'][$i];
          $this->FAIdebianSection[$sec]=$sec;
        }
      }
      $this->FAIpackage = array();

    } // ENDE  dn != new  

    $methods = array();
    foreach($this->FAIinstallMethods as $method){
      $methods[$method] = $method;
    }
    $this->FAIinstallMethods = $methods;

    /* Check if we exist already - no need to ask for revisions, etc. then */
    if ($this->dn != "new"){
      $this->newDialogShown= true;
    }
    $this->is_new = FALSE;
    if($this->dn == "new"){
      $this->is_new =TRUE;
    }

    /* Check weither its allowed to directly add packages */
    $direct_packages_add = $config->search("faiManagement","direct_packages_add",array("menu"));
    /* Force Release and Sections to the currently selected Defaults */
    $this->FAIdebianRelease = $this->getRelease();
    $this->FAIdebianSection = $this->getSections();

    if (preg_match("/^true$/i", $direct_packages_add) || preg_match("/yes/i", $direct_packages_add)) {
      $this->direct_packages_add = TRUE;
    }
    
    /* Generate package list */
    $this->list= $this->genPkgs(TRUE);

    // Prepare lists
    $this->packageList = new sortableListing();
    $this->packageList->setDeleteable(false);
    $this->packageList->setEditable(false);
    $this->packageList->setWidth("100%");
    $this->packageList->setHeight("200px");
    $this->packageList->setColspecs(array('20px','*','*','100px','60px'));
    $this->packageList->setHeader(array(_("Status"),_("Name"),_("Description"),_("Version")));
    $this->packageList->setDefaultSortColumn(1);
  }

  function get_configured_packages($packages, $base)
  {
    /* Fetch all package configurations from ldap 
     */
    $PackageFilter = "";
    foreach($packages as $name => $value){
      $PackageFilter .= "(FAIpackage=".$name.")";
    }
    /* Also fetch d-i */
    $PackageFilter .= "(FAIpackage=d-i)";

    $PackageFilter = "(&(objectClass=FAIdebconfInfo)(|".$PackageFilter."))";

    /* Search for configuration objects */ 
    $ldap = $this->config->get_ldap_link();
    $ldap->cd($base);
    $ldap->search($PackageFilter,array("FAIvariable","FAIvariableType",
          "FAIvariableContent","FAIpackage","FAIdebianSection","FAIstate"));

    $configured_packages = array();

    /* Walk through configurations and append them to our list of ConfiguredPackages 
     */
    while($attr = $ldap->fetch()){

      /* Skip objects, that are tagged as removed */
      if(isset($object['FAIstate'][0])){
        if(preg_match("/removed$/",$attr['FAIstate'][0])){
          continue;
        }
      }

      $tmp =array(); 
      $tmp['Name']  = $attr['FAIvariable'][0];
      $tmp['Type']  = $attr['FAIvariableType'][0];
      $tmp['Save']  = TRUE;

      if (isset($attr['FAIvariableContent'][0])){
        if(!in_array($attr['FAIvariableType'],array("multiselect"))){
          $tmp['Value'] = $attr['FAIvariableContent'][0];
        }else{
          $content = array();        
          unset($attr['FAIvariableContent']['count']);
          foreach($attr['FAIvariableContent'] as $attr){
            $tmp['Value'][] = $attr;
          }
        }
        $configured_packages[$attr['FAIpackage'][0]][$attr['FAIvariable'][0]]=$tmp;
      }
      else {
        $tmp['Value'] = "";
        $configured_packages[$attr['FAIpackage'][0]][$attr['FAIvariable'][0]]=$tmp;
      }
    }
  
    return $configured_packages;
  }


  function execute()
  {
    /* Call parent execute */
    plugin::execute();

    if($this->is_account && !$this->view_logged){
      $this->view_logged = TRUE;
      new log("view","fai/".get_class($this),$this->dn);
    }

    /* Fill templating stuff */
    $smarty= get_smarty();
    $display= "";

    /******
     * Initialize a new Package List with release and section name
     ******/
    
    if(!$this->is_account){

      $this->FAIdebianRelease = $this->getRelease();

      $this->FAIdebianSection = $this->getSections();

      /* Assign Repository settings*/ 
      $this->is_account     = true;
    }

    // Display warning if release is not available, but only when the dialog is first opened
    if (!isset($_POST['pluginModified']) && !empty($this->FAIdebianRelease)) {
      $this->genPkgs();
      if (empty($this->buffer)) {
        msg_dialog::display(sprintf(_("Release unavailable: %s"), $this->FAIdebianRelease), 
          sprintf(_("The packages in release %s and their details cannot be displayed because the release is unavailable on this server."), $this->FAIdebianRelease),
          WARNING_DIALOG);      
        }
    }
    

    /******
     * Add 
     ******/

    if(isset($_POST['AddManualpkg']) && 
        isset($_POST['addPpkgsText']) &&
        strlen($_POST['addPpkgsText']) && 
        $this->acl_is_writeable("FAIpackage") && 
        !preg_match("/freeze/",$this->FAIstate)){

      // Check all splitted packages for valid package names
      $add_packages = preg_split( "/[\s,]+/", get_post('addPpkgsText'), -1, PREG_SPLIT_NO_EMPTY );
      if( is_array($add_packages) ) {
        $valid_packages = array();
        $invalid_packages = array();
        foreach ($add_packages as $value) {
          if(!preg_match( "/^[a-z0-9][-0-9a-z+\.]+$/",$value)){
            $invalid_packages[] = trim($value);
          }else{
            $valid_packages[] = trim($value);
          }
        }
        if(count($invalid_packages)){
          $str = implode(", ",$invalid_packages);
          msg_dialog::display(_("Invalid package names"), 
              sprintf(_("The following package names don't match the Debian policy: %s"),$str),
              ERROR_DIALOG);
        }


        // If we have a complete list of valid packages, add them
        if(count($valid_packages)){

          foreach($valid_packages as $key =>  $value){
            if(array_key_exists($value,$this->usedPackages)) {
              unset($valid_packages[$key]);
            }
          }

          // Query SI-Deamon for additional package information
          $daemon       = new gosaSupportDaemon();
          $query_attrs  = array("distribution", "package","version", "section", "description", "timestamp");
          $do_si_query  = true;


          foreach ($valid_packages as $value) {
            if( $do_si_query == true ) {
              $res = $daemon->FAI_get_packages($this->FAIdebianRelease,$query_attrs,array($value),0,1);
              if( ! $daemon->is_error()){
                
                if(count($res)){

                  // We just use the last answer - there shouldn't be multiple
                  $res_attrs = array_pop( $res );
                  $this->list[$value] = $res_attrs;
                  $this->usedPackages[$value] = $res_attrs['PACKAGE'];

                }else{
                  $this->usedPackages[$value] = $value;
                }
              }else{
                msg_dialog::display(_("Service infrastructure"),
                    msgPool::siError($daemon->get_error()),
                    ERROR_DIALOG);
                $do_si_query = false;
              }
            }
          }

          ksort($this->usedPackages);

          /* Generate package list */
          $this->list= $this->genPkgs(TRUE);
        }
      }
    }
    
    if(isset($_POST['EditConfigurations']) &&
      $this->acl_is_writeable("FAIdebconfInfo")) {
        $this->dialog = new faiDebconfConfigurations($this->config, $this->dn, $this);
        $this->dialog->set_acl_base($this->acl_base);
        $this->is_dialog = true;
    }


    // Open the packageSelect dialog to allow adding packages out of a list.
    if(isset($_POST['Addpkg']) && $this->acl_is_writeable("FAIpackage") && !preg_match("/freeze/",$this->FAIstate)){
      $this->packageSelect = new packageSelect($this->config, get_userinfo());
      session::set('packageSelect_Release',$this->FAIdebianRelease);
      $this->dialog =true;
    }

    // Close packageSelect dialog.
    if(isset($_POST['packageSelect_cancel'])){
      $this->packageSelect = FALSE;
      $this->dialog =FALSE;
    }

    // Close packageSelect dialog and add the selected packages.
    if(isset($_POST['packageSelect_save']) && $this->packageSelect instanceOf packageSelect){
      if(!preg_match("/freeze/i", $this->FAIstate)){
        $use = $this->packageSelect->save();

        foreach($use as $pkg){
          $item = array();
          for($i =0; $i < $pkg['count']; $i++){
            $item[$pkg[$i]] = $pkg[$pkg[$i]][0];
          }
          $this->usedPackages[$item['PACKAGE']] = $item['PACKAGE']; 
        }

        ksort($this->usedPackages);

        /* Generate package list */
        $this->list= $this->genPkgs(TRUE);
      }
      $this->packageSelect = FALSE;
      $this->dialog =FALSE;
    }

    // Display package add dialog 
    if($this->packageSelect instanceOf packageSelect){

      session::set('filterBlacklist', array('PACKAGE' => $this->usedPackages));
      return $this->packageSelect->execute();
    }


    /* Check image Posts 
     */
    foreach($_POST as $name => $value){
      /******
       * Remove configuration
       *****/
      if(!preg_match('/freeze/i', $this->FAIstate) && preg_match('/^remove_configuration_/', $name)){
        $id = @postDecode(preg_replace("/^remove_configuration_(.*)$/","\\1",$name));
        if(isset($this->ConfiguredPackages[$id])) {
          $this->Removal[$id] = $this->ConfiguredPackages[$id];
          unset($this->ConfiguredPackages[$id]);
        }
      }

      /******
       * Mark as removed  
       ******/
      
      if(!preg_match('/freeze/i', $this->FAIstate) && preg_match("/^removal_package_/",$name)){
        $id = @postDecode(preg_replace("/^removal_package_(.*)$/","\\1",$name));
        if(isset($this->usedPackages[$id])){
          $pkg = $this->usedPackages[$id]; 
          if (preg_match('/\-$/', $pkg)){
            $pkg= preg_replace('/\-$/', '', $pkg);
          } else {
            $pkg= preg_replace('/$/', '-', $pkg);
          }
          $this->usedPackages[$id] = $pkg;
        }
        break;
      }

      /******
       * Delete Pkgs   
       ******/
      if(!preg_match('/freeze/i', $this->FAIstate) && preg_match("/^remove_package_/",$name)){
        $id = @postDecode(preg_replace("/^remove_package_(.*)$/","\\1",$name));
        if(isset($this->usedPackages[$id])){
          unset($this->usedPackages[$id]);
        }
        break;
      }

      /******
       * Configure Pkgs   
       ******/
      if(!preg_match('/freeze/i', $this->FAIstate) && preg_match("/^configure_package_/",$name)){
        $pkg = @postDecode(preg_replace("/^configure_package_(.*)$/","\\1",$name));
         
        if(isset($this->usedPackages[$pkg])){

          /* Configuration dialog open*/
          $pkg_config = array();
          if(isset($this->ConfiguredPackages[$pkg])){
            $pkg_config = $this->ConfiguredPackages[$pkg];
          }
          $this->configure_dialog_shown = TRUE;
          $this->dialog = new faiPackageConfiguration($this->config, $this->dn,$pkg, $this->FAIdebianRelease , $pkg_config);
          $this->dialog->FAIstate = $this->FAIstate;
          $this->is_dialog =true;
        }
        break;
      }
    }

    /* Abort package selection dialog */ 
    if(isset($_POST['CancelSubObject'])){
      $this->dialog = false;
      $this->is_dialog=false;
    }

    /* Save Configuration */
    if(isset($_POST['SaveObjectConfig']) && $this->configure_dialog_shown){
      if(!preg_match("/^freeze/", $this->FAIstate)){
        $this->ConfiguredPackages= array_merge($this->ConfiguredPackages,$this->dialog->save());
      }
      $this->dialog = false;
      $this->is_dialog=false;
      $this->configure_dialog_shown=false;
    }

    if (isset($_POST['SaveDebconfConfig'])){
      $this->dialog->save_object();
      $msgs = $this->dialog->check();
      if(count($msgs)>0) {
        foreach($msgs as $msg){
          msg_dialog::display(_("Error"), $msg, ERROR_DIALOG);
        }
      } else {
        if(!preg_match("/^freeze/", $this->FAIstate)){
          $this->ConfiguredPackages= $this->dialog->save();
          foreach($this->ConfiguredPackages as $package => $attrs) {
            foreach($attrs as $name => $data) {
              if (isset($data['Save']) && !($data['Save'])) {
                $dn = "FAIvariable=".$name.",".$this->dn;
                $ldap = $this->config->get_ldap_link();
                $ldap->cat($dn);
                if (!$ldap->count()) {
                  unset($this->ConfiguredPackages[$package][$name]);
                }
              }
            }
            if (is_array($this->ConfiguredPackages[$package])) {
              if (count($this->ConfiguredPackages[$package]) == 0) {
                unset($this->ConfiguredPackages[$package]);
              }
            }
          }
        }
        $this->dialog = false;
        $this->is_dialog = false;
      }
    }

    /* cancel configuration */     
    if(isset($_POST['CancelObjectConfig']) && $this->configure_dialog_shown){
      $this->dialog = false;
      $this->is_dialog=false;
      $this->configure_dialog_shown=false;
    }

    /* Display dialog */ 
    if($this->is_dialog){
      $this->dialog->save_object();
      return $this->dialog->execute();
    }


    /******
     * Display UI / HTML / smarty 
     ******/

    /* Create list to display a list of all currently used packages
     */
    $data = $lData = array();
    $this->packageList->setHeight(600);

    ksort($this->usedPackages);
    if(is_array($this->usedPackages)){
        foreach($this->usedPackages as $usedName => $name){
            /* Is this class frezed? */
            $freeze = preg_match('/^freeze/', $this->FAIstate);

            /* Append message if package is configured */
            $configured = image('images/empty.png');
            if(isset($this->ConfiguredPackages[$usedName])){
              if ($this->acl_is_writeable("FAIdebconfInfo") and !$freeze) {
                $configured = image('plugins/fai/images/package_configure.png', "remove_configuration_".base64_encode($usedName), _("Remove configuration"));
              } else {
                $configured = image('plugins/fai/images/package_configure-grey.png');
              }
            }
            if (!$freeze) {
              $removal_image = image('plugins/fai/images/removal_mark.png',
                'removal_package_'.postEncode($usedName),
                _("Mark package for removal"));
            } else {
              $removal_image = image('plugins/fai/images/removal_mark-grey.png');
            }
            if (!$freeze) {
              $trash_image = image('images/lists/trash.png',
                'remove_package_'.postEncode($usedName),
                ("Remove this package"));
            } else {
              $trash_image = image('images/lists/trash-grey.png');
            }
                        
            /* Adapt used name if we're marked for removal */
            $removal = image('images/empty.png');
            if (preg_match('/\-$/', $name)){
                $removal = image('plugins/fai/images/removal_mark.png','',_("Package marked for removal"));
            }

            /* Get Version */
            $version = "";
            if(isset($this->list[$usedName]['VERSION'])){
                $version = $this->list[$usedName]['VERSION'];
            }

            /* Get description */
            $description = "";
            if(isset($this->list[$usedName]['DESCRIPTION'])){
                $description = base64_decode($this->list[$usedName]['DESCRIPTION']);
            }

            $actions = $removal_image;

            if(isset($this->list[$usedName]['TEMPLATE']) && 
                    !preg_match('/^freeze/', $this->FAIstate) && $this->acl_is_writeable("FAIdebconfInfo")){
                $actions.= image('plugins/fai/images/package_configure.png',
                        'configure_package_'.postEncode($usedName),
                        _("Configure this package"));
            }
            if($this->acl_is_writeable("FAIpackage")){
              $actions.= $trash_image; 
            }
            $data[$usedName] = $usedName;
            $lData[$usedName] = array('data'=>
                    array(
                        $configured."&nbsp;".$removal,
                        $usedName ,set_post($description), $version, $actions));
        }
    }
    $this->packageList->setAcl($this->getacl("FAIpackage",preg_match('/^freeze/', $this->FAIstate)));
    $this->packageList->setListData($data,$lData);
    $this->packageList->update();

    /* Assign variables */
    foreach($this->attributes as $attrs){
      $smarty->assign($attrs,set_post($this->$attrs));
    }
    $smarty->assign( "FAIinstallMethods", $this->FAIinstallMethods );

    /* Assign section to smarty */
    $strsec = "";
    foreach($this->FAIdebianSection as $sec){
      $strsec .= $sec." ";
    }

    $tmp = $this->plInfo();
    foreach($tmp['plProvidedAcls'] as $name => $translated){
      $smarty->assign($name."ACL",$this->getacl($name,preg_match("/freeze/",$this->FAIstate)));
    }

    if ($this->acl_is_writeable("FAIdebconfInfo")) {
      $this->debconf_configurations = TRUE;
    }
 
    $smarty->assign("freeze", preg_match("/freeze/",$this->FAIstate));
    $smarty->assign("listing",$this->packageList->render());
    $smarty->assign("release" ,$this->FAIdebianRelease);
    $smarty->assign("sections",$this->sections);
    $smarty->assign("section" ,$strsec);
    $smarty->assign("direct_packages_add", $this->direct_packages_add);
    $smarty->assign("debconf_configurations", $this->debconf_configurations);
    $display.= $smarty->fetch(get_template_path('faiPackage.tpl', TRUE));
    return($display);
  }


  /*! \brief  Removes this packageList from the ldap database 
   */
  function remove_from_parent()
  {
    $ldap = $this->config->get_ldap_link();
    $ldap->cd ($this->dn);
    $release = $this->parent->parent->fai_release;
    $use_dn = preg_replace("/".preg_quote(FAI::get_release_dn($this->dn), '/')."/i", $release, $this->dn);
    FAI::prepare_to_save_FAI_object($use_dn,array(),true);
    new log("remove","fai/".get_class($this),$use_dn,$this->attributes);
    foreach($this->ConfiguredPackages as $pkgname => $attrs){
      foreach($attrs as $name => $attr){
        $pkgdn =  "FAIvariable=".$name.",".$this->dn;
        $use_dn = preg_replace("/".preg_quote(FAI::get_release_dn($this->dn), '/')."/i", $release, $pkgdn);
        FAI::prepare_to_save_FAI_object($use_dn,array(),true);
      }
    }
    $this->handle_post_events("remove");
  }


  /*! \brief  Collect all relevant POST vars for this plugin 
   */
  function save_object()
  {
    if(preg_match("/^freeze/", $this->FAIstate)) return;
    plugin::save_object();
  }


  /*! \brief  Check given inputs for this package list
      @return Array Containing all error messages, or an empty array if no error occured
   */
  function check()
  {
    /* Call common method to give check the hook */
    $message= plugin::check();

    if(count($this->usedPackages)==0){
      $message[]= _("Please select a least one package!");
    }

    if((empty($this->FAIdebianRelease))||(empty($this->FAIdebianSection))){
      $message[]=_("Please choose a valid release/section combination for your repository setup!");
    }

    /* Ensure that we do not overwrite an allready existing entry 
     */
    if($this->is_new){
      $release = $this->parent->parent->fai_release;
      $new_dn= 'cn='.$this->cn.",".get_ou("faiPackage", "faiPackageRDN").get_ou("faiManagement", "faiBaseRDN").$release;
      $res = faiManagement::check_class_name("FAIpackageList",$this->cn,$new_dn);
      if(isset($res[$this->cn])){
        $message[] = msgPool::duplicated(_("Name"));
      }
    }
    return ($message);
  }


  /*! \brief  Reload the list of cached packages.
      @return Returns the currently cached list of packages. 
   */
  function genPkgs($force = false)
  {
    if(empty($this->FAIdebianRelease)) return;

    if(!count($this->buffer) || $force){
      $q = new gosaSupportDaemon();
      $attrs = array("distribution", "package","version", "section", "description", "timestamp","template");

      $packages = array_keys($this->usedPackages);

      $ret = $q->FAI_get_packages($this->FAIdebianRelease,$attrs,$packages);
      if($q->is_error()){
        msg_dialog::display(_("Service infrastructure"),msgPool::siError($q->get_error()),ERROR_DIALOG);
      }else{
        foreach($ret as $attr){
          $this->buffer[$attr['PACKAGE']] = $attr;
        }
      }
    }
    return $this->buffer;
  }


  /*! \brief Save packages and their configuration to ldap 
   */
  function save()
  {

    /* Assemble release name */
    if($this->FAIdebianRelease == "ClearFromCopyPaste"){

      $current_release  = $this->parent->parent->fai_release;
      $tmp= preg_replace('/,'.preg_quote(get_ou("faiManagement", "faiBaseRDN"), '/').'.*$/i', '', $current_release);
      $tmp= preg_replace('/ou=/', '', $tmp);
      $rev= array_reverse(explode(',', $tmp));
      $this->FAIdebianRelease= "";
      foreach ($rev as $part){
        $this->FAIdebianRelease.= "/$part";
      }
      $this->FAIdebianRelease= preg_replace('#^/#', '', $this->FAIdebianRelease);
    }

    plugin::save();

    $ldap = $this->config->get_ldap_link();

    $this->attrs['FAIpackage'] = array();
    foreach($this->usedPackages as $pkg){
      $this->attrs['FAIpackage'][] = $pkg;
    } 

    $this->attrs['FAIdebianSection'] = array();
    foreach($this->FAIdebianSection as $sec){
      $this->attrs['FAIdebianSection'][] = $sec;
    }

    FAI::prepare_to_save_FAI_object($this->dn,$this->attrs);
    
    if($this->initially_was_account){
      new log("modify","fai/".get_class($this),$this->dn,$this->attributes);
    }else{
      new log("create","fai/".get_class($this),$this->dn,$this->attributes);
    }

    $ldap->cd($this->dn);

    /* Save Package configurations */
    foreach($this->Removal as $pkgname => $attrs){
      /* Skip entries which are back in the ConfiguredPackages array */
      if (isset($this->ConfiguredPackages[$pkgname])) {
        continue;
      }
      foreach($attrs as $name => $attr){
        $dn = "FAIvariable=".$name.",".$this->dn;
        $ldap = $this->config->get_ldap_link();
        $ldap->cat($dn);
        if ($ldap->count()) {
          FAI::prepare_to_save_FAI_object($dn,array(),true);
        }
      }
    }

    foreach($this->ConfiguredPackages as $pkgname => $attrs){
      foreach($attrs as $name => $attr){
        $pkgattrs = array();

        foreach($attr as $n=>$v){
          if(empty($v)) $v = array();
        }

        /* Set attributes */
        $pkgattrs['objectClass'][]        = "FAIdebconfInfo";

        $pkgattrs['FAIpackage']           = $pkgname;
        $pkgattrs['FAIvariable']          = $name;
        $pkgattrs['FAIvariableType']      = $attr['Type'];
        $pkgattrs['FAIvariableContent']   = $attr['Value'];
        $pkgdn =  "FAIvariable=".$name.",".$this->dn;

        if (!$attr['Save']) {
          $ldap = $this->config->get_ldap_link();
          $ldap->cat($pkgdn);
          if ($ldap->count()) {
            FAI::prepare_to_save_FAI_object($pkgdn,array(),true);
          }
          else {
            unset($this->ConfiguredPackages[$pkgname][$name]);
          }
          if (is_array($this->ConfiguredPackages[$pkgname]) && count($this->ConfiguredPackages[$pkgname]) == 0) {
            unset($this->ConfiguredPackages[$pkgname]);
          }


          continue;

        }

        if (empty($pkgattrs['FAIvariableContent'])) {
          $pkgattrs['FAIvariableContent'] = array();
        }

        if (is_array($pkgattrs['FAIvariableContent'])) {
          $ldap = $this->config->get_ldap_link();
          $ldap->cat($pkgdn);
          if (!$ldap->count()) {
            /* Do not try to set FAIvariableContent to empty array if this
             * debconfInfo object is new */
            unset($pkgattrs['FAIvariableContent']);
          }
        }

        /* Tag object */
       $ui= get_userinfo();
        $this->tag_attrs($pkgattrs, $pkgdn, $ui->gosaUnitTag);

        if(in_array_strict($pkgattrs['FAIvariableType'],array("boolean","multiselect","password","select","string","text"))){

          if($pkgattrs['FAIvariableType'] == "text" && $pkgattrs['FAIvariableContent'] == ""){
            gosa_log("Skipped saving FAIvariable '$name' empty string can't be saved.");
          }else{
            FAI::prepare_to_save_FAI_object($pkgdn,$pkgattrs);
          }
        }
      }
    }

  }


  /*! \brief  Return plugin informations for acl handling 
      @return Array ACL infos of this plugin.
   */ 
  static function plInfo()
  {
    return (array( 
          "plShortName" => _("Package"),
          "plDescription" => _("FAI Package list"),
          "plSelfModify"  => FALSE,
          "plDepends"     => array(),
          "plPriority"    => 28,
          "plSection"     => array("administration"),
          "plCategory"    => array("fai"),
          "plProperties" =>
          array(
              array(
                  "name"          => "faiPackageRDN",
                  "type"          => "rdn",
                  "default"       => "ou=packages,",
                  "description"   => _("RDN for FAI package storage."),
                  "check"         => "gosaProperty::isRdn",
                  "migrate"       => "migrate_faiPackageRDN",
                  "group"         => "plugin",
                  "mandatory"     => TRUE)),

          "plProvidedAcls" => array(
            "cn"                => _("Name"),
            "description"       => _("Description"),
            "FAIpackage"        => _("Packages"),
            "FAIdebianSection"  => _("Section")."&nbsp;("._("Read-only").")",
            "FAIinstallMethod"  => _("Install Method"),
            "FAIdebconfInfo"    => _("Package configuration"),
            "FAIdebianRelease"  => _("Release")."&nbsp;("._("Read-only").")")
          ));
  }


  /*! \brief prepares this plugin to be inserted after it was copied or cut.
      @param Array  All attributes from the source object. 
   */
  function PrepareForCopyPaste($source)
  {
    plugin::PrepareForCopyPaste($source);

    if(isset($source['FAIstate'][0])){
      $this->FAIstate = $source['FAIstate'][0];
    }

    $this->FAIdebianRelease = "ClearFromCopyPaste";

    if(isset($source['FAIpackage'])){
      unset($source['FAIpackage']['count']);
      foreach($source['FAIpackage'] as $pkg){
        $this->usedPackages[$pkg] = $pkg;
      }
      ksort($this->usedPackages);
    }else{
      $this->usedPackages = array();
    }

    if((isset($source['FAIdebianSection']))&&(is_array($source['FAIdebianSection']))){
      $this->FAIdebianSection = array();
      for($i = 0 ; $i < $source['FAIdebianSection']['count'] ; $i ++){
        $this->FAIdebianSection[$source['FAIdebianSection'][$i]]=$source['FAIdebianSection'][$i];
      }
    }
    $this->ConfiguredPackages = $this->get_configured_packages($this->usedPackages, $source['dn']);
  }


  /*! \brief  Returns a list of all configured servers with repositories.
      @return Array  All repository server 
   */
  function getServerInfos()
  {
    $ret = array();
    $ldap = $this->config->get_ldap_link();
    $ldap->cd($this->config->current['BASE']);
    $ldap->search("(objectClass=FAIrepositoryServer)",array("*"));
    while($attrs = $ldap->fetch()){
      if(isset($attrs['FAIrepository'])){
        for($i =0 ; $i < $attrs['FAIrepository']['count']; $i++){
          $obj = $attrs['FAIrepository'][$i];
          $tmp = explode("|",$obj);
          if(count($tmp)==4){
            foreach(explode(",",$tmp[3]) as $sec){
              if(!empty($sec)){
                $ret[$tmp[2]][] =  $sec;
              }
            }
          }
        }
      }
    }
    return($ret);
  }

  /*! \brief Assemble release name 
    Returns the currently used Debian Release.
   */ 
  function getRelease()
  {
    /* Assemble release name */
    if (isset($this->parent->parent->fai_release)) {
      $release = $this->parent->parent->fai_release;
    } else {
      $release = $this->dn;
      # Remove CN
      $tmp = preg_replace('/cn='.$this->cn.',/', '', $release);
      # Remove ou=packages
      $release = preg_replace('/'.preg_quote(get_ou("faiPackage", "faiPackageRDN"), '/').'/', '', $tmp);
    }
    $tmp= preg_replace('/[,]*'.preg_quote(get_ou("faiManagement", "faiBaseRDN"), '/').'.*$/', '', $release);
    $tmp= preg_replace('/ou=/', '', $tmp);
    $rev= array_reverse(explode(',', $tmp));
    $release = "/";
    foreach ($rev as $part){
      $release .= "/$part";
    }
    return(preg_replace('#^[/]*#', '', $release));
  }

  /*! \brief Assemble sections
    Returns the currently used Debian archive sections.
   */ 
  function getSections()
  {
    $ret = array();
    $repos= $this->getServerInfos();
    if(isset($repos[$this->FAIdebianRelease])){
      $ret = $repos[$this->FAIdebianRelease];
      $ret = array_unique($ret);
    }
    return $ret;
  }

  /*! \brief  Used for copy & paste.
    Returns a HTML input mask, which allows to change the cn of this entry.
    @param  Array   Array containing current status && a HTML template.
   */
  function getCopyDialog()
  {
    $vars = array("cn");
    $smarty = get_smarty();
    $smarty->assign("cn", set_post($this->cn));
    $str = $smarty->fetch(get_template_path("paste_generic.tpl",TRUE));
    $ret = array();
    $ret['string'] = $str;
    $ret['status'] = "";
    return($ret);
  }


  /*! \brief  Used for copy & paste.
    Some entries must be renamed to avaoid duplicate entries.
   */
  function saveCopyDialog()
  {
    if(isset($_POST['cn'])){
      $this->cn = get_post('cn');
    }
  }
}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
