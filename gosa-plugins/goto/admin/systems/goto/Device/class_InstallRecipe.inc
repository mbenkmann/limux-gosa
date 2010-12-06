<?php

class InstallRecipe extends plugin
{

    /* This has to be replaced by valid attributes, currently they are just 
     *  fake variables. #FIXME: I do not know the real attribute names yet.
     */
    public $installBootstrapMethod = '';
    public $installBootstrapMethodList = NULL;

    public $installConfigManagement = '';
    public $installConfigManagementList = NULL;

    /* Plugin attributes
     */
    public $installTemplateDN = "";
    public $installTemplateDNList = array();

    public $installMirrorDN = "";
    public $installMirrorDNList = array();

    public $installKernelPackage = "";
    public $installKernelPackageList = array();

    public $installKeyboardlayout = "";
    public $installKeyboardlayoutList = array();

    public $installSystemLocale = "";
    public $installSystemLocaleList = array();

    public $installTimezone = "";
    public $installTimeUTC = "";

    public $installNTPServer = array();
    public $installRootEnabled = "";
    public $installRootPasswordHash = "";

    public $setKickstartRootPasswordHash = FALSE;
    public $installPartitionTable = "";
    private $installNTPServerList = NULL;

    public $objectclasses = array('installRecipe');
    public $attributes = array("installTemplateDN","installKeyboardlayout","installSystemLocale",
            "installTimezone","installTimeUTC","installNTPServer","installMirrorDN",
            "installRootEnabled","installRootPasswordHash","installKernelPackage",
            "installPartitionTable","installConfigManagement","installBootstrapMethod");

    private $map = array('puppet' => array('objectClass' => 'puppetClient', 'type' => 'puppet', 'bootstrap' => 'preseed'));

    /* Some plugin management related attributes 
     * See class plugin for details.
     */
    public $view_logged = FALSE;
    public $ignore_account = FALSE;


    /*! \brief  Initialize the plugin and its widgets. 
     *          Finally call init() to load values from the backend.
     */ 
    function __construct(&$config, $dn)
    {
        plugin::plugin($config, $dn);

        // Set default password hash
        $this->hash = $this->config->get_cfg_value("core","passwordDefaultHash"); 

        // Prepare list of timezones
        $this->timezones = $this->getTimezones();

        // Get list of password hashes
        $tmp = passwordMethod::get_available_methods();
        $this->hashes = array();
        foreach($tmp['name'] as $name){
            $this->hashes[$name] = $name;
        }

        // Prepare NTP servers list 
        $this->installNTPServerList= new sortableListing($this->installNTPServer);
        $this->installNTPServerList->setDeleteable(true);
        $this->installNTPServerList->setEditable(false);
        $this->installNTPServerList->setColspecs(array('*'));
        $this->installNTPServerList->setWidth("100%");
        $this->installNTPServerList->setHeight("70px");

        // Init the object values.
        $this->init();
    }

   
    /*! \brief   Load attribute values from the GOsa-backend using jsonRPC 
     */ 
    function init()
    {
        // Detect account status by checking the used objectClasses
        $this->is_account = FALSE;
        foreach($this->map as $type){
            if(in_array($type['objectClass'], $this->attrs['objectClass'])){
                $this->installBootstrapMethod = $type['bootstrap'];
                $this->installConfigManagement = $type['type'];
                $this->is_account = TRUE;
            }
        }

        // Start without error.
        $this->initFailed = FALSE;

        // Get list of NTP servers
        $this->installNTPServer = array();
        if(isset($this->attrs['installNTPServer']['count'])){
            for($i = 0; $i < $this->attrs['installNTPServer']['count']; $i++){
                $this->installNTPServer[] = $this->attrs['installNTPServer'][$i];
            }
        }

        // Load list of bootstrap methods.
        $res = $this->loadInstallationMethods();
        if($res == NULL){
            $this->initFailed = TRUE;
            msg_dialog(_("Error"), msgPool::rpcError($this->rpcErrorMessage), ERROR_DIALOG);
            return;
        }
        $this->installBootstrapMethodList = $res['installBootstrapMethodList'];
        $this->installConfigManagementList = $res['installConfigManagementList'];


        // Load system locales 
        $locales = $this->getSystemLocales();
        if($locales == NULL){
            $this->initFailed = TRUE;
            msg_dialog(_("Error"), msgPool::rpcError($this->rpcErrorMessage), ERROR_DIALOG);
            return;
        }
        $this->installSystemLocaleList = $locales;


        // Load installation templates
        $res = $this->loadInstallationTemplates();
        if($res == NULL){
            $this->initFailed = TRUE;
            msg_dialog(_("Error"), msgPool::rpcError($this->rpcErrorMessage), ERROR_DIALOG);
            return;
        }
        $this->installTemplateDNList = $res;


        // Load installation keyboard layouts
        $res = $this->loadInstallationKeyboardLayouts();
        if($res == NULL){
            $this->initFailed = TRUE;
            msg_dialog(_("Error"), msgPool::rpcError($this->rpcErrorMessage), ERROR_DIALOG);
            return;
        }
        $this->installKeyboardlayoutList = $res;


        // Load kernel package list
        $res = $this->loadInstallationKernelPackages();
        if($res == NULL){
            $this->initFailed = TRUE;
            msg_dialog(_("Error"), msgPool::rpcError($this->rpcErrorMessage), ERROR_DIALOG);
            return;
        }
        $this->installKernelPackageList = $res;


        // Load installation mirrors
        $res = $this->loadInstallationMirrors();
        if($res == NULL){
            $this->initFailed = TRUE;
            msg_dialog(_("Error"), msgPool::rpcError($this->rpcErrorMessage), ERROR_DIALOG);
            return;
        }
        $this->installMirrorDNList = $res;


        // Preset some values for new accounts
        if(!$this->is_account){

            // Preset the device timezone
            $tz = timezone::get_default_timezone();
            $this->installTimezone = $tz['name'];

            // Set initial bootstrap and install method.
            $this->installBootstrapMethod = key($this->installBootstrapMethodList);
            $this->installConfigManagement = key($this->installConfigManagementList[$this->installBootstrapMethod]);
        }


        // Check if we've a valid bootstrap and configMangement method selected
        if(!isset($this->installConfigManagementList[$this->installBootstrapMethod])){
            $newMethod = key($this->installBootstrapMethodList);
            msg_dialog::display(_("Error"), 
                sprintf(_("The selected bootstrap method %s is invalid. Using %s instead!"),
                bold($this->installBootstrapMethod), bold($newMethod)), ERROR_DIALOG);
            $this->installBootstrapMethod = $newMethod;
        }
        if(!isset($this->installConfigManagementList[$this->installBootstrapMethod][$this->installConfigManagement])){
            $newMethod = key($this->installConfigManagementList[$this->installBootstrapMethod]);
            msg_dialog::display(_("Error"), 
                sprintf(_("The selected config management method %s is invalid. Using %s instead!"),
                bold($this->installConfigManagement),bold($newMethod)), ERROR_DIALOG);
            $this->installConfigManagement = $newMethod;
        }
    }


    function getTimezones()
    {
        $tmp = timezone::_get_tz_zones();
        $list = array();
        foreach($tmp['TIMEZONES'] as $name => $offset){
            if($offset >= 0){
                $list[$name] = $name." ( + ".sprintf("%0.2f",$offset/(60*60))." "._("hours").")";
            }else{
                $offset = $offset * -1;
                $list[$name] = $name." ( - ".sprintf("%0.2f",($offset/(60*60)))." "._("hours").")";
            }
        }  
        uksort($list, 'strnatcasecmp'); 
        return($list);
    }
 
    
    function loadInstallationKernelPackages()
    {
        return(array('kernel1','kk1'));
    }
    

    function loadInstallationMirrors()
    {
        $res = array(
                'dc=intranet,dc=gonicus,dc=de' => 'Intranet',
                'ou=systems,dc=intranet,dc=gonicus,dc=de' => 'System');
        return($res);
    }

    function loadInstallationTemplates()
    {
        $res = array(
                'dc=intranet,dc=gonicus,dc=de' => 'Intranet-Tpl',
                'ou=systems,dc=intranet,dc=gonicus,dc=de' => 'System-Tpl');
        return($res);
    } 

    function loadInstallationKeyboardLayouts()
    {
        return(array('104','105'));
    } 

    function getSystemLocales()
    {
        $locales = json_decode('{"hu_HU.UTF-8": "Hungarian", 
                "es_DO.UTF-8": "Spanish (Dominican Republic)", 
                "es_CL.UTF-8": "Spanish (Chile)"}');

        return($locales);
    }
 

    /*! \brief  Try to load install and bootstrap methods from the GOsa-backend
     *          This is done via jsonRPC, if calling the backend method fails,
     *           this method returns FALSE, in case of success it return TRUE; 
     */ 
    function loadInstallationMethods()
    { 
        $rpc = $this->config->getRpcHandle();
        $res = $rpc->getSupportedBaseInstallMethods();
       
        // Failed to load backend values, keep reported error and return with FALSE; 
        $this->rpcError = !$rpc->success();
        if(!$rpc->success()){
            $this->rpcErrorMessage = $rpc->get_error();
            return(NULL);
        }

        // Walk through result and set bootstrap methods.
        foreach($res as $name => $method){
            $res['installBootstrapMethodList'][$name] = $method['name'];
            foreach($method['methods'] as $m){
                $res['installConfigManagementList'][$name][$m] = $m; 
            }
        }
        return($res);
    }
    

    function execute()
    {
        // Log account access
        if($this->is_account && !$this->view_logged){
            $this->view_logged = TRUE;
            new log("view","Device/".get_class($this),$this->dn);
        }

        /***************
          Handle account state
         ***************/

        // Allow to add or remove the distribution list extension
        if(isset($_POST['modify_state'])){
            if($this->is_account && $this->acl_is_removeable()){
                $this->is_account= FALSE;
            }elseif(!$this->is_account && $this->acl_is_createable()){
                $this->is_account= TRUE;
            }
        }

        // Show account status-changer
        $display = "";
        if ($this->parent !== NULL){
            if ($this->is_account){
                $display= $this->show_disable_header(_("Remove install profile"),
                        msgPool::featuresEnabled(_("Install profile")));
            } else {
                $display= $this->show_enable_header(_("Add install profile"),
                        msgPool::featuresDisabled(_("Install profile")));
                return ($display);
            }
        }

        /***************
          Root password hash dialog
         ***************/

        if($this->setKickstartRootPasswordHash){
            $this->dialog = TRUE;
            $smarty = get_smarty();
            $smarty->assign('hashes', set_post($this->hashes));
            $smarty->assign('hash', set_post($this->hash));
            return($smarty->fetch(get_template_path('goto/Device/SetPassword.tpl', TRUE)));
        }

        /***************
          Generate HTML content
         ***************/

        $this->installNTPServerList->setAcl($this->getacl('installNTPServer'));
        $this->installNTPServerList->update();

        plugin::execute();    
        $smarty = get_smarty();
        $smarty->assign('timezones', $this->timezones);
        $smarty->assign('installKeyboardlayoutList', $this->installKeyboardlayoutList);
        $smarty->assign('installKernelPackageList', $this->installKernelPackageList);
        $smarty->assign('installTemplateDNList', $this->installTemplateDNList);
        $smarty->assign('installMirrorDNList', $this->installMirrorDNList);
        $smarty->assign('installSystemLocaleList', $this->installSystemLocaleList);
        $smarty->assign('installNTPServerList', $this->installNTPServerList->render());

        $smarty->assign('installBootstrapMethod', $this->installBootstrapMethod);
        $smarty->assign('installBootstrapMethodList', $this->installBootstrapMethodList);
        $smarty->assign('installConfigManagement', $this->installConfigManagement);

        $cfgList = $this->installConfigManagementList[$this->installBootstrapMethod];
        $smarty->assign('installConfigManagementList', $cfgList);
        foreach($this->attributes as $attr){
            $smarty->assign($attr, $this->$attr);
        }
        $this->dialog = false;
        return($display.$smarty->fetch(get_template_path('goto/Device/InstallRecipe.tpl', TRUE)));
    }


    function save_object()
    {
        if(isset($_POST['InstallRecipePosted'])){

            $currentInstallMethod = $this->installConfigManagement;
            plugin::save_object();
            $this->installRootEnabled = isset($_POST['installRootEnabled']);
            $this->installTimeUTC = isset($_POST['installTimeUTC']);


            if(isset($_POST['setKickstartRootPasswordHash'])){
                $this->setKickstartRootPasswordHash = true;
            }

            $this->installNTPServerList->save_object();
            $this->installNTPServer = $this->installNTPServerList->getMaintainedData();
            if(isset($_POST['installNTPServer_Input']) && isset($_POST['installNTPServer_Add'])){
                $add = get_post('installNTPServer_Input');
                if(!in_array($add, $this->installNTPServer) && !empty($add)){
                    $this->installNTPServer[] = $add;
                }
            }
            $this->installNTPServerList->setListData($this->installNTPServer);
        }
        if(isset($_POST['cancelPassword'])) $this->setKickstartRootPasswordHash =false;
        if(isset($_POST['setPassword'])) {
            $this->setKickstartRootPasswordHash =false;
            $hash = get_post('passwordHash');

            // Not sure, why this is here, but maybe some encryption methods require it.
            mt_srand((double) microtime()*1000000);

            // Generate the requested hash
            $methods = new passwordMethod($this->config, $this->dn);
            $available = $methods->get_available_methods();
            $test = new $available[$hash]($this->config,$this->dn);
            $this->installRootPasswordHash = @$test->generate_hash(get_post('rootPassword'));
        }

    }


    function save()
    {
        // 
        if(!$this->installRootEnabled) $this->installRootPasswordHash = "";
        $this->installRootEnabled = ($this->installRootEnabled)?'TRUE':'FALSE';
        $this->installTimeUTC = ($this->installTimeUTC)?'TRUE':'FALSE';
        
        plugin::save();

        unset($this->attrs['installConfigManagement']);
        unset($this->attrs['installBootstrapMethod']);
        echo "Skipp saving of 'installConfigManagement' and 'installBootstrapMethod', i've no idea where to store them.";

        // Remove all objectClasses which belongs to a bootstrap method, 
        //  and then add the selected one.
        foreach($this->map as $type){
            $this->attrs['objectClass'] = array_remove_entries_ics(array($type['objectClass']), $this->attrs['objectClass']);
        }
        $this->attrs['objectClass'][] = $this->map[$this->installConfigManagement]['objectClass'];
        
        print_a($this->attrs);

        $this->cleanup();
        $ldap=$this->config->get_ldap_link();
        $ldap->cd($this->config->current['BASE']);
        $ldap->cd($this->dn);
        $ldap->modify($this->attrs);
        new log("modify","Device/".get_class($this),$this->dn,array_keys($this->attrs),$ldap->get_error());
        if (!$ldap->success()){
            msg_dialog::display(_("LDAP error"), msgPool::ldaperror($ldap->get_error(), $this->dn, LDAP_MOD, get_class()));
        }else{
            $this->handle_post_events("modify");
        }
    }

    
    function remove_from_parent()
    {
        plugin::remove_from_parent();
        unset($this->attrs['installConfigManagement']);
        unset($this->attrs['installBootstrapMethod']);
        echo "Skipp saving of 'installConfigManagement' and 'installBootstrapMethod', i've no idea where to store them.";

        $ldap=$this->config->get_ldap_link();
        $ldap->cd($this->config->current['BASE']);
        $ldap->cd($this->dn);
        $ldap->modify($this->attrs);
        new log("remove","Device/".get_class($this),$this->dn,array_keys($this->attrs),$ldap->get_error());
        if (!$ldap->success()){
            msg_dialog::display(_("LDAP error"), msgPool::ldaperror($ldap->get_error(), $this->dn, LDAP_DEL, get_class()));
        }else{
            $this->handle_post_events("remove");
        }
        
    }
    

    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Device"),
                    "plDescription" => _("Registered device"),
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 1,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("Device"),
                    "plProvidedAcls" => array(
                        "member" => _("Member"),
                        "installTemplateDN" => _("Template"),
                        "installBootstrapMethod" => _("Bootstrap method"),
                        "installConfigManagement" => _("Config management"),
                        "installKeyboardlayout" => _("Keyboard layout"),
                        "installSystemLocale" => _("System locale"),
                        "installTimezone" => _("Timezone"),
                        "installTimeUTC" => _("Time"),
                        "installNTPServer" => _("NTP-Server"),
                        "installMirrorDN" => _("Kickstart mirror"),
                        "installRootEnabled" => _("Root login enabled"),
                        "installRootPasswordHash" => _("Root password hash"),
                        "installKernelPackage" => _("Kernel package"),
                        "installPartitionTable" => _("Partition table")
                        )
                        )
                        );
    }
}

?>
