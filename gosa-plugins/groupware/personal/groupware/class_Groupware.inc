<?php
/*
 * This code is part of GOsa (https://gosa.gonicus.de)
 * Copyright (C) 2008 Cajus Pollmeier
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software
 * Foundation, Inc., 59 Temple Place, Suite 330, Boston, MA  02111-1307  USA
 */

# Load Groupware defintion table
new GroupwareDefinitions();


class Groupware extends plugin
{
    var $plHeadline = "Groupware";
    var $plDescription = "GOsa groupware extension.";
    var $pathTitle = "GOsa groupware extension.";
    var $view_logged = FALSE;

    var $FolderWidget = NULL;

    var $accountInitialized = FALSE;
    var $rpcError = FALSE;
    var $rpcErrorMessage = "";

    var $attributes = array(
            "mailAddress", "mailLocation", "quotaUsage", "quotaSize", "alternateAddresses",
            "forwardingAddresses", "vacationEnabled", "vacationMessage", 
            "mailBoxWarnLimitEnabled", "mailBoxWarnLimitValue",
            "mailBoxSendSizelimitEnabled", "mailBoxSendSizelimitValue", "mailBoxHardSizelimitEnabled",
            "mailBoxHardSizelimitValue", "mailBoxAutomaticRemovalEnabled", "mailBoxAutomaticRemovalValue",
            "localDeliveryOnly", "dropOwnMails", "mailFolder", "mailLimitReceiveValue", "mailLimitSendValue",
            "mailLimitReceiveEnabled", "mailLimitSendEnabled");

    var $enabledFeatures = array();

    var $flagAttributes = array("vacationEnabled","mailBoxWarnLimitEnabled","mailBoxSendSizelimitEnabled",
            "mailBoxHardSizelimitEnabled","mailBoxAutomaticRemovalEnabled","localDeliveryOnly","dropOwnMails", 
            "mailLimitReceiveEnabled", "mailLimitSendEnabled");

    var $mailAddressSelectDialog = NULL;
    var $filterManager = NULL;
    var $filterRules = array();
    var $vacationTemplates = array(); 


    //the dropdown
    var $mailLocations = array();

    var $mailAddress = "";
    var $mailLocation = "";
    var $mailFolder = NULL;
	var $mailFolderChanged = NULL;
	
    var $quotaUsage = 0;
    var $quotaSize = 0;
    var $alternateAddresses = array();
    var $forwardingAddresses = array();
    var $vacationEnabled = FALSE;
    var $vacationMessage = "";
    var $mailBoxWarnLimitEnabled = FALSE;
    var $mailBoxWarnLimitValue = 100;
    var $mailBoxSendSizelimitEnabled = FALSE;
    var $mailBoxSendSizelimitValue = 100;
    var $mailBoxHardSizelimitEnabled = FALSE;
    var $mailBoxHardSizelimitValue = 100;
    var $mailBoxAutomaticRemovalEnabled = FALSE;
    var $mailBoxAutomaticRemovalValue = 100;
    var $localDeliveryOnly = FALSE;
    var $dropOwnMails = FALSE;
    var $mailLimitReceiveEnabled = FALSE;
    var $mailLimitReceiveValue = 0;
    var $mailLimitSendEnabled = FALSE;
    var $mailLimitSendValue = 0;


    var $groupwareDao = null;

    // Used in the folder editor as the starting folder, usually user/username
    var $folderPrefix = "";


    /*! \brief      
     */
    function __construct ($config, $dn= NULL)
    {
        plugin::plugin($config,$dn); 

        // Get attributes from parent object 
        if(isset($this->attrs['uid'])){
            $this->uid = $this->attrs['uid'][0];
        }

        // Set initial folder prefix
        $this->folderPrefix = "user/{$this->uid}";

        // Initialize the plugin using rpc.
        $this->init();
    }


    /*! \brief  Try to execute a function on the gosa backend using json-rpc.
     *          This method also takes care about errors and sets the required
     *           class members, such as rpcError and rpcErrorMessage. 
     *  @param  String  function    The name of the function to call.
     *  @param  Mixed   args[0-n]   The parameter to use.
     *  @return Mixed   The result of the function call on success else NULL.
     */
    function rpcExec($function)
    {
        $params = func_get_args();
        unset($params[0]);
        $rpc = $this->config->getRpcHandle();
        $res = call_user_func_array(array($rpc,$function),array_values($params));
        $this->rpcError = !$rpc->success();
        if($this->rpcError){
            $this->rpcErrorMessage = $rpc->get_error();
            return(NULL);
        }
        return($res);
    }


    /*! \brief    Checks if feature is availavle on the server (a call to getCapabilities achieves this.)
     */
    public function isFeatureEnabled($featureName)
    {
        if(!isset($this->enabledFeatures[$featureName])){
            trigger_error("No such feature '{$featureName}'!");
        }
        return(isset($this->enabledFeatures[$featureName]) && $this->enabledFeatures[$featureName]);
    }


    /*! \brief     Checking if the feature has been changed since loading it from server (initial state)
     */
    public function hasFeaturebeenChanged($featureName)
    {
        //TODO: must be implemented.
        return TRUE;
    }


    /*! \brief  Try initialize the groupware account.
     *          This method fetches all required information to manage the
     *           account using the GOsa gui.
     */
    function init()
    {
        // Detect feature availability and enable/disable services correspondingly.
        $this->groupwareDao = new GroupwareDao($this);

        // Feature names with a list of groupware functions that must be available.
        $features = array();
        $featureReq = array(
                "primaryMail" 				=> array(
                    'acctGetPrimaryMailAddress'),
                "quotaUsage"                => array(
                    'acctGetQuota'),
                "quotaSize"                 => array(
                    'acctSetQuota','acctGetQuota'),
                "mailLocations"				=> array("_off",
                    'getMailboxLocations'),
                "mailFilter"                => array("_off",
                    'acctDelFilter','acctGetFilters','acctSetFilters','acctSetFilters'),
                "alternateAddresses"        => array(
                    'acctDelAlternateMailAddress','acctSetAlternateMailAddresses',
                    'acctAddAlternateMailAddress','acctGetAlternateMailAddresses'),
                "forwardingAddresses"       => array(
                    'acctAddMailForwardAddress','acctDelMailForwardAddress',
                    'acctGetMailForwardAddresses','acctSetMailForwardAddresses'),
                "vacationMessage"           => array(
                    'acctSetOutOfOfficeReply', 'acctGetOutOfOfficeReply'),
                "mailBoxWarnLimit"          => array(
                    'acctSetQuota','acctGetQuota'),
                "mailBoxSendSizelimit"      => array(
                        'acctSetQuota','acctGetQuota'),
                //This is the same as qotaSize - should stay turned off
                "mailBoxHardSizelimit"      => array("_off",
                        'acctSetQuota','acctGetQuota'),
                "mailBoxAutomaticRemoval"   => array("_off",
                        'acctSetQuota','acctGetQuota'),
                "localDeliveryOnly"         => array( "_off",
                        'acctDelFilter','acctGetFilters','acctSetFilters','acctSetFilters'),
                "dropOwnMails"              => array( "_off",
                        'acctDelFilter','acctGetFilters','acctSetFilters','acctSetFilters'),
                "mailFolder"				=> array(
                        'folderList','folderDel','folderAdd'),
                "mailLimit"			=> array(
                        'acctGetMailLimit', 'acctSetMailLimit'),
				//Just for speedup		
				"folderListWithMembers"		=> array(
                        'acctGetComprehensiveUser'),
				"folderACLs" 				=> array(
						'folderGetMembers', 'folderSetMembers'),
				"comprehensiveUser"			=> array(
                        'acctGetComprehensiveUser')
                    );

        // Check if all required methods can be called! 
        foreach($featureReq as $name => $requires){
            $active = TRUE;
            foreach($requires as $methodName){
                $active &= $this->groupwareDao->gwFeatureAvailable($methodName);
            }
            $this->enabledFeatures[$name] = $active;
        }

        // Load mail locations.
        if ($this->isFeatureEnabled("mailLocations")){
            $this->mailLocations = $this->groupwareDao->get('mailLocations', array());
        }

        // If this is a new user, then do not try to check the account status,
        //  the account cannot exists.			
        if($this->dn == "new"){
            $this->initially_was_account = $this->is_account = FALSE;
            $this->accountInitialized = TRUE;
        }else{

            // Get rpc handle to fetch account info and feature availability.
            $status = $this->rpcExec('gwAcctExists', $this->uid);
            if($status !== NULL){
                $this->initially_was_account = $this->is_account = $status;
                if($this->is_account){
                    $response = $this->groupwareDao->getComprehensiverUser($this->uid);
                    $this->saved_attributes = $response;
                    $this->mapComprehensiveUserData($response);
                }
                $this->accountInitialized = TRUE;
            }
        }

        // Set vacation start/stop if not set alreasy
        $this->vacationStart = time();
        $this->vacationStop = time() + (14 * 60*60*24);

        // Prepare vacation start/stop time to be initially valid.  
        $this->vacationStart= date('d.m.Y', $this->vacationStart);
        $this->vacationStop= date('d.m.Y', $this->vacationStop);
    }


    /*! \brief  Generates the HTML user interface for the groupware plugin
     *           and take of several ui actions like adding or removing 
     *           forward addresses, filters and the account itself.
     */
    function execute()
    {
        // Register plugin execution 
        $display = plugin::execute();

        // Log plugin execution.
        if($this->is_account && !$this->view_logged){
            $this->view_logged = TRUE;
            new log("view","users/".get_class($this),$this->dn);
        }

        // Check if we were able to initialize the account already.
        if(!$this->accountInitialized){
            $this->init();
            if(!$this->accountInitialized){
                $smarty = get_smarty();
                $smarty->assign("initFailed", !$this->accountInitialized);
                return($smarty->fetch(get_template_path("generic.tpl",TRUE,dirname(__FILE__))));
            }
        }

        // Check if we were able to initialize the account already.
        if($this->rpcError){
            $smarty = get_smarty();
            $smarty->assign("initFailed", !$this->accountInitialized);
            $smarty->assign("rpcError", $this->rpcError);
            return($smarty->fetch(get_template_path("generic.tpl",TRUE,dirname(__FILE__))));
        }

        /****************
          Filter editor
         ****************/

        if(isset($_POST['filterManager_cancel'])) $this->filterManager = NULL;
        if(isset($_POST['filterManager_ok'])){
            $this->filterManager->save_object();
            $msgs = $this->filterManager->check();
            if(count($msgs)){
                msg_dialog::displayChecks($msgs);
            }else{
                $this->filterRules = $this->filterManager->save();
                $this->filterManager = NULL;
            }
        }
        if(isset($_POST['configureFilter'])){
            $this->filterManager = new FilterManager($this->config, $this,$this->filterRules);
            $this->filterManager->acl_base = $this->acl_base;
            $this->filterManager->acl_category = $this->acl_category;
        }
        $this->dialog = FALSE;
        if($this->filterManager instanceOf FilterManager){
            $this->filterManager->save_object();
            $this->dialog = TRUE;
            return($this->filterManager->execute());
        }

        /****************
          Folder editor
         ****************/

        if(isset($_POST['FolderWidget_cancel'])) $this->FolderWidget = NULL;
        if(isset($_POST['FolderWidget_ok'])){
            $this->FolderWidget->save_object();
            $msgs = $this->FolderWidget->check();
            if(count($msgs)){
                msg_dialog::displayChecks($msgs);
            }else{
				
				if(!empty($this->mailFolderChanges)){
					$this->mailFolderChanges = array_merge($this->mailFolderChanges, $this->FolderWidget->save());
				}
				else{
					$this->mailFolderChanges = $this->FolderWidget->save();
				}
				
				$this->folderSave($this->uid);
                $this->FolderWidget = NULL;
            }
        }

        // Act on requests to edit mailFolder entries.
        if(isset($_POST['configureFolder'])){

            // Load mail folder entries on demand, this will increase the account 
            //  instantiation a lot.
            if($this->mailFolder === NULL){
                $this->mailFolder = array();
                if($this->initially_was_account){
                    $this->mailFolder = $this->groupwareDao->getListOfMailFolders();
                }
            }

            $folders = $this->mailFolder;
            if(!isset($folders[$this->folderPrefix])){
                $folders[$this->folderPrefix] = array('name' => $this->uid,'status' => 'added','acls' => array());
            }
            $this->FolderWidget = new FolderWidget($this->config,$folders, $this->folderPrefix);
            $this->FolderWidget->setPermissions(GroupwareDefinitions::getPermissions());
            $this->FolderWidget->acl_base = $this->acl_base;
            $this->FolderWidget->acl_category = $this->acl_category;
        }
        $this->dialog = FALSE;
        if($this->FolderWidget instanceOf FolderWidget){
            $this->FolderWidget->save_object();
            $this->dialog = TRUE;
            return($this->FolderWidget->execute());
        }

        /****************
          Account status
         ****************/

        if(isset($_POST['modify_state'])){
            if($this->is_account && $this->acl_is_removeable()){
                $this->is_account= FALSE;
            }elseif(!$this->is_account && $this->acl_is_createable()){
                $this->is_account= TRUE;
            }
        }
        if(!$this->multiple_support_active){
            if (!$this->is_account && $this->parent === NULL){
                $display= "<img alt=\"\" src=\"images/small-error.png\" align=\"middle\">&nbsp;<b>".
                    msgPool::noValidExtension(_("Mail"))."</b>";
                $display.= back_to_main();
                return ($display);
            }
            if ($this->parent !== NULL){
                if ($this->is_account){ 
                    $display= $this->show_disable_header(msgPool::removeFeaturesButton(_("Groupware")),
                            msgPool::featuresEnabled(_("Groupware")));
                } else {
                    $display= $this->show_enable_header(msgPool::addFeaturesButton(_("Groupware")),
                            msgPool::featuresDisabled(_("Groupware")));
                    return ($display);
                }
            }
        }

        /****************
          Forward addresses 
         ****************/

        // Display dialog to select a local fowarder 
        if (isset($_POST['addLocalForwardingAddress'])){
            $this->mailAddressSelectDialog=  new mailAddressSelect($this->config, get_userinfo());

            $this->dialog= TRUE;
        }

        // Close dialogs, action was canceled 
        if (isset($_POST['mailAddressSelect_cancel'])){
            $this->mailAddressSelectDialog= FALSE;
            $this->dialog= FALSE;
        }

        // Append selected forwarding addresses now.
        if (isset($_POST['mailAddressSelect_save']) && $this->mailAddressSelectDialog instanceOf mailAddressSelect){
            if($this->acl_is_writeable("forwardingAddresses")){
                $list = $this->mailAddressSelectDialog->save();
                foreach ($list as $entry){
                    $val = $entry['mail'][0];
                    if (!in_array_strict($val, $this->alternateAddresses) && $val != $this->mailAddress){
                        $this->addForwarder($val);
                        $this->is_modified= TRUE;
                    }
                }
                $this->mailAddressSelectDialog= FALSE;
                $this->dialog= FALSE;
            } else {
                msg_dialog::display(_("Error"), _("Please select an entry!"), ERROR_DIALOG);
            }
        }

        // Display the address selection dialog.
        if($this->mailAddressSelectDialog instanceOf mailAddressSelect){
            $used  = array();
            $used['mail'] = array_values($this->alternateAddresses);  
            $used['mail'] = array_merge($used['mail'], array_values($this->forwardingAddresses));  
            $used['mail'][] = $this->mailAddress;

            // Build up blocklist
            session::set('filterBlacklist', $used);
            return($this->mailAddressSelectDialog->execute());
        }

        // Add manually inserted forwarding address.
        if (isset($_POST['addForwardingAddress'])){
            if ($_POST['forwardingAddressInput'] != ""){
                $address= get_post('forwardingAddressInput');
                $valid= FALSE;
                if (!tests::is_email($address)){
                    if (!tests::is_email($address, TRUE)){
                        if ($this->is_template){
                            $valid= TRUE;
                        } else {
                            msg_dialog::display(_("Error"), msgPool::invalid(_("Mail address"),
                                        "","","your-address@your-domain.com"),ERROR_DIALOG);
                        }
                    }
                } elseif ($address == $this->mailAddress || in_array_strict($address, $this->alternateAddresses)) {
                    msg_dialog::display(_("Error"),_("Cannot add primary address to the list of forwarders!") , ERROR_DIALOG);
                } else {
                    $valid= TRUE;
                }
                if ($valid){
                    if($this->acl_is_writeable("forwardingAddresses")){
                        $this->addForwarder ($address);
                        $this->is_modified= TRUE;
                    }
                }
            }
        }
        if (isset($_POST['deleteForwardingAddress'])){
            $this->delForwarder ($_POST['forwardingAddressList']);
        }

        /****************
          Alternate addresses 
         ****************/
        // Add manually inserted alternate mail address.
        if (isset($_POST['addAlternateAddress'])){
            $valid= FALSE;
            if (!tests::is_email($_POST['alternateAddressInput'])){
                if ($this->is_template){
                    if (!(tests::is_email($_POST['alternateAddressInput'], TRUE))){
                        msg_dialog::display(_("Error"),msgPool::invalid(_("Mail address"),
                                    "","","your-domain@your-domain.com"),ERROR_DIALOG);
                    } else {
                        $valid= TRUE;
                    }
                } else {
                    msg_dialog::display(_("Error"),msgPool::invalid(_("Mail address"),
                                "","","your-domain@your-domain.com"),ERROR_DIALOG);
                }
            } else {
                $valid= TRUE;
            }
            if ($valid && ($user= $this->addAlternate (get_post('alternateAddressInput'))) != ""){
                $ui= get_userinfo();
                $addon= "";
                if ($user[0] == "!") {
                    $addon= sprintf(_("Address is already in use by group '%s'."), mb_substr($user, 1));
                } else {
                    $addon= sprintf(_("Address is already in use by user '%s'."), $user);
                }
                msg_dialog::display(_("Error"), msgPool::duplicated(_("Mail address"))."<br><br><i>".
                        "$addon</i>", ERROR_DIALOG);
            }
        }

        // Remove alternate mail address.
        if (isset($_POST['deleteAlternateAddress']) && isset($_POST['alternateAddressList'])){
            $this->delAlternate ($_POST['alternateAddressList']);
        }

        /****************
          SMARTY- Assign smarty variables 
         ****************/

        $smarty = get_smarty();
        foreach($this->attributes as $attr){
            print
                $smarty->assign($attr, $this->$attr);
        }

        $plInfo = $this->plInfo();
        foreach($plInfo['plProvidedAcls'] as $acl => $name){
            $smarty->assign($acl."ACL", $this->getacl($acl));
        }
        foreach($this->enabledFeatures as $feature => $state){
            $smarty->assign($feature."_isActive", $state);
        }

        $smarty->assign("mailLocations", $this->mailLocations);
        if (count($this->vacationTemplates)){
            $smarty->assign("displayTemplateSelector", "TRUE");
            $smarty->assign("vacationTemplate", set_post($this->vacationTemplate));
            $smarty->assign("vacationTemplates", set_post($this->vacationTemplates));
            $smarty->assign("template", set_post(get_post('vacation_template')));
        } else {
            $smarty->assign("displayTemplateSelector", "FALSE");
        }

        $smarty->assign("uid", $this->uid);
        $smarty->assign("initFailed", !$this->accountInitialized);
        $smarty->assign("rpcError", $this->rpcError);
        $smarty->assign("rpcErrorMessage", $this->rpcErrorMessage);

        return($display.$smarty->fetch(get_template_path("generic.tpl",TRUE,dirname(__FILE__))));
    }


    /*! \brief      This method handles potential _POST and _GET values.
     *              It captures modifcations from the ui, like changing 
     *               the mailAddress.
     *              This method respects the attribute permissions.
     */    
    function save_object()
    {
        if(isset($_POST['groupwarePluginPosted'])){

            // We ran into a communication error with the backend. 
            // Try a simple communication operation with the backend 
            // again and let us see if it works.
            if(isset($_POST['retry'])){
                $this->rpcExec('gwGetCapabilities');
            }

            // Get posted mail address modifications
            if(isset($_POST['mailAddress']) && $this->acl_is_writeable('mailAddress') && $this->isFeatureEnabled('primaryMail')){
                $this->mailAddress = get_post('mailAddress');
            }

            // Get ui modifications and store them in the class.
            $testAttrs = array("mailLocation","quotaUsage","quotaSize",
                    "alternateAddresses","forwardingAddresses","vacationMessage");
            foreach($testAttrs as $attr){
                if(isset($_POST[$attr]) && $this->acl_is_writeable($attr) && $this->isFeatureEnabled($attr)){
                    $this->$attr = get_post($attr);
                }
            }

            // Detect checkbox states 
            $checkAttrs = array(
                    "mailBoxWarnLimit" => "mailBoxWarnLimit",
                    "mailBoxSendSizelimit" => "mailBoxSendSizelimit",
                    "mailBoxHardSizelimit" => "mailBoxHardSizelimit",
                    "mailBoxAutomaticRemoval" => "mailBoxAutomaticRemoval", 
                    "mailLimitReceive" => "mailLimit", 
                    "mailLimitSend" => "mailLimit");
            foreach($checkAttrs as $boxname => $feature){
                if($this->acl_is_writeable($boxname) && $this->isFeatureEnabled($feature)){
                    $v = $boxname."Value"; 
                    $e = $boxname."Enabled"; 
                    $this->$e = isset($_POST[$e]);
                    if($this->$e){
                        $this->$v = get_post($v);
                    }
                }
            }

            // Check for a changed vacation status
            if($this->acl_is_writeable('vacationMessage') && $this->isFeatureEnabled('vacationMessage')){
                $this->vacationEnabled = isset($_POST['vacationEnabled']);
            }

            // Get posted flag changes 
            $flagAttrs = array("localDeliveryOnly","dropOwnMails");
            foreach($flagAttrs as $attr){
                if($this->acl_is_writeable($attr) && $this->isFeatureEnabled($attr)){
                    $this->$attr = isset($_POST[$attr]);
                }
            }
        }
    }


    /*! \brief  Parse vacation templates and build up an array
     *	containing 'filename' => 'description'. 
     *	Used to fill vacation dropdown box.
     *	@return Array   All useable vacation templates.
     */ 
    function get_vacation_templates_usedSomewhere()
    {
        $vct = array();
        if ($this->config->get_cfg_value("core","vacationTemplateDirectory") != ""){
            $dir= $this->config->get_cfg_value("core","vacationTemplateDirectory");
            if (is_dir($dir) && is_readable($dir)){
                $dh = opendir($dir);
                while($file = readdir($dh)){
                    $description= "";
                    if (is_file($dir."/".$file)){
                        $fh = fopen($dir."/".$file, "r");
                        $line= fgets($fh, 256);
                        if (!preg_match('/^DESC:/', $line)){
                            msg_dialog::display(_("Configuration error"), sprintf(_("No DESC tag in vacation template '%s'!"), $file), ERROR_DIALOG);
                        }else{
                            $description= trim(preg_replace('/^DESC:\s*/', '', $line));
                        }
                        fclose ($fh);
                    }
                    if ($description != ""){
                        $vct["$dir/$file"]= $description;
                    }
                }
                closedir($dh);
            }
        }
        return($vct); 
    }


    /*! \brief  Adds the given mail address to the list of mail forwarders 
     */ 
    function addForwarder($address)
    {
        if(empty($address)) return;
        if($this->acl_is_writeable("forwardingAddresses")){		
            //Check if the address is a valid address on the Groupwareserver

            $isMailAvailabel = $this->rpcExec("gwMailAddressExists", $address);
            if(empty($isMailAvailabel)){
                msg_dialog::display(_("Permission error"), _("This email address is not available as forwarding recipient in your groupware."), ERROR_DIALOG);
            }
            else{
                $this->forwardingAddresses[]= $address;
                $this->forwardingAddresses= array_unique ($this->forwardingAddresses);
                sort ($this->forwardingAddresses);
                reset ($this->forwardingAddresses);
                $this->is_modified= TRUE;
            }
        }else{
            msg_dialog::display(_("Permission error"), _("You have no permission to modify these addresses!"), ERROR_DIALOG);
        }
    }


    /*! \brief  Removes the given mail address from the list of mail forwarders 
     */ 
    function delForwarder($addresses)
    {

        if($this->acl_is_writeable("forwardingAddresses")){
            $this->forwardingAddresses= array_remove_entries ($addresses, $this->forwardingAddresses);
            $this->is_modified= TRUE;
        }else{
            msg_dialog::display(_("Permission error"), _("You have no permission to modify these addresses!"), ERROR_DIALOG);
        }
    }


    /*! \brief  Add given mail address to the list of alternate adresses ,
     *           check if this mal address is used, skip adding in this case 
     */ 
    function addAlternate($address)
    {
        if(empty($address)) return;
        if($this->acl_is_writeable("alternateAddresses")){
            $ldap= $this->config->get_ldap_link();
            $address= strtolower($address);

            /* Is this address already assigned in LDAP? */
            $ldap->cd ($this->config->current['BASE']);
            $ldap->search ("(&(!(objectClass=gosaUserTemplate))(objectClass=gosaMailAccount)(|(mail=$address)".
                    "(alias=$address)(gosaMailAlternateAddress=$address)))", array("uid", "cn"));
            if ($ldap->count() > 0){
                $attrs= $ldap->fetch ();
                if (!isset($attrs["uid"])) {
                    return ("!".$attrs["cn"][0]);
                }
                return ($attrs["uid"][0]);
            }
            if (!in_array_strict($address, $this->alternateAddresses)){
                $this->alternateAddresses[]= $address;
                $this->is_modified= TRUE;
            }
            sort ($this->alternateAddresses);
            reset ($this->alternateAddresses);
            return ("");
        }else{
            msg_dialog::display(_("Permission error"), _("You have no permission to modify these addresses!"), ERROR_DIALOG);
        }
    }


    /*! \brief  Removes the given mail address from the alternate addresses list 
     */ 
    function delAlternate($addresses)
    {
        if($this->acl_is_writeable("alternateAddresses")){
            $this->alternateAddresses= array_remove_entries ($addresses,$this->alternateAddresses);
            $this->is_modified= TRUE;
        }else{
            msg_dialog::display(_("Permission error"), _("You have no permission to modify these addresses!"), ERROR_DIALOG);
        }
    }


    /*! \brief  Prepare importet vacation string. \
     *           Replace placeholder like %givenName a.s.o.
     * @param  string  Vacation string
     * @return string  Completed vacation string
     */
    private function prepare_vacation_template_usedSomewhere($contents)
    {
        /* Replace attributes */
        $attrs = array();
        $obj   = NULL;
        if(isset($this->parent->by_object['user'])){
            $attrs  = $this->parent->by_object['user']->attributes;
            $obj    = $this->parent->by_object['user'];
        }else{
            $obj    = new user($this->config,$this->dn);
            $attrs  = $obj->attributes;
        }
        if($obj){

            /* Replace vacation start and end time */
            if($this->enabledFeatures['vacationMessage']){
                if(preg_match("/%start/",$contents)){
                    $contents = preg_replace("/%start/",$this->vacationStart,$contents);
                }
                if(preg_match("/%end/",$contents)){
                    $contents = preg_replace("/%end/",$this->vacationStop,$contents);
                }
            }else{
                if(preg_match("/%start/",$contents)){
                    $contents = preg_replace("/%start/", _("unknown"),$contents);
                }
                if(preg_match("/%end/",$contents)){
                    $contents = preg_replace("/%end/", _("unknown"), $contents);
                }
            }

            foreach ($attrs as $val){

                // We can only replace strings here
                if(!is_string($obj->$val)) continue;

                if(preg_match("/dateOfBirth/",$val)){
                    if($obj->use_dob){
                        $contents= preg_replace("/%$val/",date("Y-d-m",$obj->dateOfBirth),$contents);
                    }
                }else {
                    $contents= preg_replace("/%$val/",
                            $obj->$val, $contents);
                }

            }
        }
        $contents = ltrim(preg_replace("/^DESC:.*$/m","",$contents),"\n ");
        return($contents);
    }


    /*! \brief     Remove the account form the groupware server completely. 
     */
    function remove_from_parent()
    {
        // Get rpc handle to remove the account
        if($this->initially_was_account){
            if($this->rpcExec('gwAcctDel', $this->uid) === NULL){
                msg_dialog::display(_("Error"), _("Groupware account removal failed!"), ERROR_DIALOG);
            }
        }
    }


    /* \brief   Persists the values of this plugin to the groupware server.
     *          Save method is called on "apply" and "ok" in the Gosa Frontend.
     *          All other actions will update values in the form or 
     *           cancel and therefore discard the changes so far.
     */
    function save()
    {
        // Get rpc handle to create or update the account
        if(!$this->initially_was_account){
            $res = $this->rpcExec('gwAcctAdd', $this->uid, $this->mailAddress);
            if($res === NULL){
                $message = _("Groupware account creation failed!");
                msg_dialog::display(_("Error"),$message."<br>".sprintf(_("Error was: '%s'."),$this->rpcErrorMessage),ERROR_DIALOG);
                return;
            }
        }

        // Save the primary Email Address.
        if($this->isFeatureEnabled("primaryMail")&& $this->hasFeaturebeenChanged("primaryMail")){
		
            $res = $this->groupwareDao->save("primaryMail", $this->uid, $this->mailAddress);
            if($res === NULL){
                $message = _("Setting account mail address failed!");
                msg_dialog::display(_("Error"),$message."<br>".sprintf(_("Error was: '%s'."),$this->rpcErrorMessage),ERROR_DIALOG);
            }
        }

        // Save alternateAddresses 
        if($this->isFeatureEnabled("alternateAddresses")&& $this->hasFeaturebeenChanged("alternateAddresses")){
            if(isset($this->alternateAddresses) && is_array($this->alternateAddresses)){
				if(!empty($this->alternateAddresses)){
					$res = $this->groupwareDao->save("alternateAddresses", $this->uid, array($this->alternateAddresses));
				}
				else{
				
					$res = $this->groupwareDao->save("alternateAddresses", $this->uid, array(array()));
				}
                if($res === NULL){
                    $message = _("Saving alternate addresses failed!");
                    msg_dialog::display(_("Error"),$message."<br>".sprintf(_("Error was: '%s'."),$this->rpcErrorMessage),ERROR_DIALOG);
                }
            }
        }

        // Save forwardingAddresses
        if($this->isFeatureEnabled("forwardingAddresses") && $this->hasFeaturebeenChanged("forwardingAddresses")){

            // Transform simple address array into,  $addresses[address] = ForwardFlag;
            $addresses = array();
			if(!empty($this->forwardingAddresses)){
				foreach($this->forwardingAddresses as $address) $addresses[$address] = TRUE;
			
				$res = $this->groupwareDao->save("forwardingAddresses", $this->uid, array($addresses));
			}
			else{	
				$res = $this->groupwareDao->save("forwardingAddresses", $this->uid, array(array()));
			}
            
            if($res === NULL){
                $message = _("Saving forward addresses failed!");
                msg_dialog::display(_("Error"),$message."<br>".sprintf(_("Error was: '%s'."),$this->rpcErrorMessage),ERROR_DIALOG);
            }
        }

        //Save quota - it consists of four values... 
        if( ($this->isFeatureEnabled("mailBoxWarnLimit") ||
                    $this->isFeatureEnabled("mailBoxSendSizelimit") ||
                    $this->isFeatureEnabled("quotaSize") ||
                    $this->isFeatureEnabled("mailBoxAutomaticRemoval"))
                && ($this->hasFeaturebeenChanged("mailBoxWarnLimit") ||
                    $this->hasFeaturebeenChanged("mailBoxSendSizelimit") ||
                    $this->hasFeaturebeenChanged("quotaSize") ||
                    $this->hasFeaturebeenChanged("mailBoxAutomaticRemoval") )){
					
			if(!$this->mailBoxWarnLimitEnabled){
				$this->mailBoxWarnLimitValue = null;
			}
			
			if(!$this->mailBoxSendSizelimitEnabled){
				$this->mailBoxSendSizelimitValue = null;
			}
			
            $quota = array(	"warn_limit" => $this->mailBoxWarnLimitValue,
                    "send_limit" => $this->mailBoxSendSizelimitValue,
                    "hard_limit" => $this->quotaSize);
					//hold cannot be saved (in Exchange) right now - should be excluded for now.
                    //"hold" => $this->mailBoxAutomaticRemovalValue);

            $res = $this->groupwareDao->save("quotaSize", $this->uid, $quota);
            if($res === NULL){
                $message = _("Saving quota failed!");
                msg_dialog::display(_("Error"),$message."<br>".sprintf(_("Error was: '%s'."),$this->rpcErrorMessage),ERROR_DIALOG);
            }
        }

        // TODO: Check the following paragraph
		// Save account folders
        if($this->mailFolder !== NULL && $this->isFeatureEnabled("mailFolder") && $this->hasFeaturebeenChanged("mailFolder")){
            //$res = $this->saveFoldersAndAcls($this->uid, $this->mailFolder);
            if($res === NULL){
                $message = _("Saving mail folder failed!");
                msg_dialog::display(_("Error"),$message."<br>".sprintf(_("Error was: '%s'."),$this->rpcErrorMessage),ERROR_DIALOG);
            }
        }

        if($this->isFeatureEnabled("mailLocations") && $this->hasFeaturebeenChanged("mailLocation")){
            $res = $this->groupwareDao->save("mailLocation", $this->uid, $this->mailLocation);
            if($res === NULL){
                $message = _("Saving mail location failed!");
                msg_dialog::display(_("Error"),$message."<br>".sprintf(_("Error was: '%s'."),$this->rpcErrorMessage),ERROR_DIALOG);
            }
        }

        //checks for feature "mailLimit"              
        if($this->isFeatureEnabled("mailLimit") && $this->hasFeaturebeenChanged("mailLimit")){
            if(!$this->mailLimitReceiveEnabled){
                $this->mailLimitReceiveValue = 0;
            }
            if(!$this->mailLimitSendEnabled){
                $this->mailLimitSendValue = 0;
            }
            $res = $this->groupwareDao->save("mailLimit", $this->uid, array( $this->mailLimitSendValue, $this->mailLimitReceiveValue));
            if($res === NULL){
                $message = _("Saving mail limitations failed!");
                msg_dialog::display(_("Error"),$message."<br>".sprintf(_("Error was: '%s'."),$this->rpcErrorMessage),ERROR_DIALOG);
            }
        }


        if($this->isFeatureEnabled("vacationMessage")&& $this->hasFeaturebeenChanged("vacationMessage")){
            if($this->vacationEnabled){
                //setting an message means that the OOOReply is switched off.
                $res = $this->groupwareDao->save("vacationMessage", $this->uid, $this->vacationMessage);
            }else{
                //setting no message means that the OOOReply is switched off.
                $res = $this->groupwareDao->save("vacationMessage", $this->uid, null);
            }
            if($res === NULL){
                $message = _("Saving vacation message failed!");
                msg_dialog::display(_("Error"),$message."<br>".sprintf(_("Error was: '%s'."),$this->rpcErrorMessage),ERROR_DIALOG);
            }
        }
		//Todo: Remove this 
		$this->folderSave($this->uid);
	}
        //checks for feature "quotaUsage"                
        //not to be saved

#       //checks for feature "mailFilter"                 
#       if($this->isFeatureEnabled("mailFilter") && $this->hasFeaturebeenChanged("mailFilter")){}
#
#       //checks for feature "mailBoxAutomaticRemoval"   
#       if($this->isFeatureEnabled("mailBoxAutomaticRemoval") && $this->hasFeaturebeenChanged("mailBoxAutomaticRemoval")){}
#
#       //checks for feature "localDeliveryOnly"         
#       if($this->isFeatureEnabled("localDeliveryOnly") && $this->hasFeaturebeenChanged("localDeliveryOnly")){}
#
#       //checks for feature "dropOwnMails"              
#       if($this->isFeatureEnabled("dropOwnMails") && $this->hasFeaturebeenChanged("dropOwnMails")){}
    
	
	/*! \brief  TODO remove this function later
     */
	function folderSave($id){
		if($this->isFeatureEnabled("mailFolder")&& $this->hasFeaturebeenChanged("mailFolder")){
			//is sorting necessary? 
			//Find folders to add
			if(empty($this->mailFolderChanges)){
				return;
			}
			
			//Remove the root folder - the widget adds it as new 
			unset($this->mailFolderChanges["user/".$this->uid]);
			
			foreach($this->mailFolderChanges as $folder => $memberlist){
				
				if($this->mailFolderChanges[$folder]["status"] == "added"){
					$this->groupwareDao->save("mailFolder", $folder);
				}
				if($this->mailFolderChanges[$folder]["status"] == "removed"){
					$this->groupwareDao->del("mailFolder", $folder);
				}	
				
				$memberListToBeSaved = array();
				if($this->isFeatureEnabled("folderACLs")){
					//checking if ACL has been changed.
					if($this->mailFolderChanges[$folder]["status"] == "added"){
						if(isset($memberlist["acls"])){
							
							//set Default ACLs if not yet set by User
							$defAcls = GroupwareDefinitions::getDefaultFolderACLs();
							$memberListToBeSaved = $defAcls;
							//append all mebers set in Gosa widget.
							foreach($memberlist["acls"] as $member){
								$memberListToBeSaved[$member["name"]] = $member["acl"];
							}
						}
					}
					else{
						//Todo: remove debugs
						/*
						echo "--- Hello1";
						print_a($memberlist["acls"]);
						echo "--- Hello2";
						print_a($this->mailFolderChanges[$folder]["acls"]);
						*/
						if(array_differs($memberlist["acls"], $this->mailFolder[$folder]["acls"])){
							//append all mebers set in Gosa widget.
							echo "ACLs have changed.";
							foreach($memberlist["acls"] as $member){
								$memberListToBeSaved[$member["name"]] = $member["acl"];
							}
						}
					}
					if(!empty($memberListToBeSaved)){
						//Todo: remove debugs
						/*
						echo "List To save:<pre>";
						print_r($memberListToBeSaved);
						echo "</pre><br>";
						*/
						$this->groupwareDao->save("folderACLs", $folder, array($memberListToBeSaved));
					}
				}
			}
		}
		//reset the changes.. we have persisted them.
		$this->mailFolderChanges = null;
	}
	
    /*! \brief  Check given values 
     */
    function check()
    {
        //checke features only if they are enabled.
        $messages = plugin::check();

        // For newly created users!
        // We can input every 'uid' we want for new users, so we've to check if uids weren't used yet.
        if(!$this->initially_was_account){
            $exists = $this->rpcExec('gwAcctExists', $this->uid);
            if($this->rpcError){
                $messages[] = sprintf(_("Failed to validate groupware account."), $this->uid);
            }elseif($exists){
                $messages[] = sprintf(_("Account id, already used '%s'!"), $this->uid);
            }
        } 
    
        // Check for valid mailAddresses
        if ($this->isFeatureEnabled("primaryMail")){		

            if(!tests::is_email ($this->mailAddress)){
                $messages[] = msgPool::invalid(_("Mail address"),$this->mailAddress , "", "user@exdom.intranet.gonicus.de knibbel");
            }
                
            // Ensure that the mailAddress isn't used yet.
            if(!$this->initially_was_account || $this->mailAddress != $this->saved_attributes['primaryMail']){
                $exists = $this->rpcExec("gwMailAddressExists", $this->mailAddress);
                if($this->rpcError){
                    $messages[] = sprintf(_("Failed to validate mail address '%s'."), $this->mailAddress);
                }elseif($exists){
                    $messages[] = msgPool::duplicated(_("Mail address"));
                }
            }
        }

        //checks for feature "forwardingAddresses" 
        if ($this->isFeatureEnabled("forwardingAddresses")){		
            if(isset($this->forwardingAddresses) && is_array($this->forwardingAddresses)){
                foreach($this->forwardingAddresses as $fAddress){
                    if(!tests::is_email ($fAddress)){
                        $messages[] = msgPool::invalid(_("Forwarding address"),$fAddress, "", 
                                "user@exdom.intranet.gonicus.de");
                    }
                    //Check if the address is a valid address on the Groupwareserver
                    $isMailAvailabel = $this->rpcExec("gwMailAddressExists", $fAddress);
                    if(empty($isMailAvailabel)){
                        $messages[] = _("This email address is not available as forwarding recipient in your groupware.");
                    }
                    if($fAddress == $this->mailAddress){
                        $messages[] = _("The primary address cannot be used as forwarding address!");
                    }
                }
            }
        }

        //checks for feature "mailLimit" - integer 
        if ($this->isFeatureEnabled("mailLimit")){
            if (!is_numeric($this->mailLimitReceiveValue)){
                $messages[] = msgPool::invalid(_("Value for mailbox size receive limit"),$this->mailLimitReceiveValue , "", "1024 = 1MB");
            }
            if (!is_numeric($this->mailLimitSendValue)){
                $messages[] = msgPool::invalid(_("Value for mailbox size send limit"),$this->mailLimitSendValue , "", "1024 = 1MB");
            }
        }

        //checks for feature "mailBoxWarnLimit"          
        if ($this->isFeatureEnabled("quotaSize")){
            //Todo: wrong test
            if (!is_numeric($this->quotaSize)){
                $messages[] = msgPool::invalid(_("Value for Quota size"),$this->quotaSize , "", "1024 = 1GB");
            }
        }

        //checks for feature "mailBoxSendSizelimit"      
        if ($this->isFeatureEnabled("mailBoxSendSizelimit")){
            //Todo: wrong test
            if (!is_numeric($this->mailBoxSendSizelimitValue)){
                $messages[] = msgPool::invalid(_("Value refuse incoming mails limit"),$this->mailBoxSendSizelimitValue , "", "1024 = 1GB");
            }
        }
        //checks for feature "mailBoxHardSizelimit"
        if ($this->isFeatureEnabled("mailBoxWarnLimit")){
            //Todo: wrong test
            if (!is_numeric($this->mailBoxWarnLimitValue)){
                $messages[] = msgPool::invalid(_("Value warn about full mailbox"),$this->mailBoxWarnLimitValue , "", "1024 = 1GB");
            }
        }

#       //checks for feature "vacationMessage"   
#       if ($this->isFeatureEnabled("vacationMessage")){}
#
#       //checks for feature "mailBoxAutomaticRemoval"   
#       if ($this->isFeatureEnabled("mailBoxAutomaticRemoval")){}
#
#       //checks for feature "localDeliveryOnly"  
#       if ($this->isFeatureEnabled("localDeliveryOnly")){}	
#
#       //checks for feature "dropOwnMails"
#       if ($this->isFeatureEnabled("dropOwnMails")){}
#
#       //checks for feature "mailFolder"	
#       if ($this->isFeatureEnabled("mailFolder")){}
#
#       //checks for feature "quotaUsage"                
#       if ($this->isFeatureEnabled("quotaUsage")){}
#
#       //checks for feature "mailLocations"				
#       if ($this->isFeatureEnabled("mailLocations")){}
#
#       //checks for feature "mailFilter"                
#       if ($this->isFeatureEnabled("mailFilter")){}
#
#       //checks for feature "alternateAddresses"        
#       if ($this->isFeatureEnabled("alternateAddresses")){}

        return($messages);
    }


    /*! \brief  ACL settings 
     */
    static function plInfo()
    {
        return (array(
                    "plShortName"     => _("Groupware"),
                    "plDescription"   => _("Groupware settings"),
                    "plSelfModify"    => TRUE,
                    "plDepends"       => array("user"),                     // This plugin depends on
                    "plPriority"      => 4,                                 // Position in tabs
                    "plSection"     => array("personal" => _("My account")),
                    "plCategory"    => array("users"),
                    "plOptions"       => array(),
                    "plProvidedAcls"  => array(
                        "mailAddress"                   => _("Mail address"),
                        "mailLocation"                  => _("Mail location"),
                        "mailFolder"                    => _("Mail folder"),
                        "quotaUsage"                    => _("Quota usage"),
                        "mailFilter"                    => _("Mail filter"),
                        "mailFilter"                    => _("Mail filter"),
                        "quotaSize"                     => _("Quota size"),
                        "alternateAddresses"            => _("Alternate mail addresses"),
                        "forwardingAddresses"           => _("Forwarding mail addresses"),
                        "vacationEnabled"               => _("Vaction switch"),
                        "vacationMessage"               => _("Vacation message"),
                        "mailBoxWarnLimit"              => _("Warn sizelimit"),
                        "mailBoxSendSizelimit"          => _("Send sizelimit"),
                        "mailBoxHardSizelimit"          => _("Hard sizelimit"),
                        "mailBoxAutomaticRemoval"       => _("Automatic mail removal"),
                        "localDeliveryOnly"             => _("Local delivery only"),
                        "dropOwnMails"                  => _("Drop own mails"),
                        "mailLimitSend"                 => _("mail Limit send"),
                        "mailLimitReceive"              => _("mail Limit receive")
                        )
                        ));
    }


    /*! \brief  Maps the resultset fetched from the Dao to the class variables 
     * 		     of the plugin.
     */
    function mapComprehensiveUserData($callBackMap)
    {

        $map = array(
                "mailLocations" => "mailLocations",
                "mailFolder" => "mailFolder",
                "mailAddress" => "primaryMail",
                "mailLocation" => "mailLocation",
                "quotaUsage" => "quotaUsage",
                "quotaSize" => "mailBoxHardSizelimitValue",
                "alternateAddresses" => "alternateAddresses",
                "forwardingAddresses" => "forwardingAddresses",
                "vacationEnabled" => "vacationEnabled",
                "vacationMessage" => "vacationMessage",
                "mailBoxWarnLimitEnabled" => "mailBoxWarnLimitEnabled",
                "mailBoxWarnLimitValue" => "mailBoxWarnLimitValue",
                "mailBoxSendSizelimitEnabled" => "mailBoxSendSizelimitEnabled",
                "mailBoxSendSizelimitValue" => "mailBoxSendSizelimitValue",
                "mailBoxHardSizelimitEnabled" => "mailBoxHardSizelimitEnabled",
                "mailBoxHardSizelimitValue" => "mailBoxHardSizelimitValue",
                "mailBoxAutomaticRemovalEnabled" => "mailBoxAutomaticRemovalEnabled",
                "mailBoxAutomaticRemovalValue" => "mailBoxAutomaticRemovalValue",
                "localDeliveryOnly" => "localDeliveryOnly",
                "dropOwnMails" => "dropOwnMails",
                "mailLimitReceiveValue" => "mailLimitReceiveValue",
                "mailLimitSendValue" => "mailLimitSendValue",
                "mailLimitReceiveEnabled" => "mailLimitReceiveEnabled",
                "mailLimitSendEnabled" => "mailLimitSendEnabled"
                    );

        // Map values from source array to class members
        foreach($map as $target => $source){
            if(isset($callBackMap[$source])){
                $this->$target = $callBackMap[$source];
            }
        }
    }


    /*! \brief  This is a feature with many datasets inside which should be 
   
    function saveFoldersAndAcls($id, $mailFolder)
    {
        // In the tests with the exchange server groupware setting acls is very
        // slow. So this function has got o be checked for modifications.
        $res = TRUE;
        foreach($mailFolder as $key => $val)
        {
            if(!empty($val["status"])){
                switch($val["status"]){
                    case "add":
                        $res = $this->groupwareDao->save("mailFolder", $id, $key);
                    break;
                    case "del":
                        $res = $this->groupwareDao->delete("mailFolder", $id, $key);
                    break;
                }
            }
            if($res === NULL) return($res);
        }
        return($res);
    }
	*/
}


?>
