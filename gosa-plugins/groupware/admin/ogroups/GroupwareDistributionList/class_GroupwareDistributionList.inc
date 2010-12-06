<?php

new GroupwareDefinitions();

class GroupwareDistributionList extends plugin
{
    public $view_logged = FALSE;

    // Error hanlding related attributes.
    private $initialized  = FALSE;
    private $rpcError = FALSE;
    private $rpcErrorMessage = "";


    // Attribute definition 
    public $attributes = array('primaryMailAddress','alternateAddresses','memberList','mailSizeLimit');
    public $mailSizeLimit = NULL;
    public $primaryMailAddress = "";
    public $alternateAddresses = array();
    public $memberList = array();   

    // Feature handling
    private $featuresEnabled = array();


    /*! \brief  Constructs the plugin, loads required parent values 
     *           and initiates the initialization.
     */
    function __construct($config, $dn, $attrs = NULL)
    {
        plugin::plugin($config, $dn, $attrs);

        // Get attributes from parent object
        $this->cn = "";
        if(isset($this->attrs['cn'])){
            $this->cn = $this->attrs['cn'][0];
        }
        $this->orig_cn = $this->cn;

        // Initialize the distribution list using the gosa-ng backend 
        $this->init();
    }


    /*! \brief      Check whether a feature is enabled or not.
     *  @param      The feature name to check for
     *  @return     TRUE on success else FALSE
     */
    function featureEnabled($name)
    {
        return(isset($this->featuresEnabled[$name]) && $this->featuresEnabled[$name]);
    }


    /*! \brief      Try to initialize the plugin using a json-rpc connection
     *               to the gosa-ng server.
     */
    function init()
    {
        // Detect supported capabilities 
        $rpc = $this->config->getRpcHandle();
        $capabilities = $rpc->gwGetCapabilities();
        if(!$rpc->success()){
            $this->rpcError = TRUE;
            $this->rpcErrorMessage = $rpc->get_error();
            $message = sprintf(_("Failed to load supported capabilities from server! Error was: '%s'."), 
                    $rpc->get_error());
            msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
            return;
        }

        // Detect features we can use
        $map['alternateAddresses'] = array('distGetAlternateMailAddresses','distSetAlternateMailAddresses');
        $map['mailSizeLimit'] = array('distGetMailLimit','distGetMailLimit');
        foreach($map as $name => $required){
            $this->featuresEnabled[$name] = TRUE;
            foreach($required as $func){
                $this->featuresEnabled[$name] &= isset($capabilities[$func]) && $capabilities[$func];
            }
        }

        // If we're creating a new ogroup, then we've definately no extension, yet.
        $this->rpcError = FALSE;
        if($this->cn == "" || $this->dn == "new"){
            $is_account = FALSE;
        }else{

            // Check whether a mathing distribution-list exists or not!
            $is_account = $rpc->gwDistExists($this->orig_cn);
            if(!$rpc->success()){
                $this->rpcError = TRUE;
                $this->rpcErrorMessage = $rpc->get_error();
                $message = sprintf(_("Failed to check existence for distribution list '%s'! Error was: '%s'."), 
                        $this->orig_cn, $rpc->get_error());
                msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                return;
            }
        }

        // We've detected a valid distribution list, now load all
        //  configured members, so we're able to update the memberlist
        //  on save();
        $memberList = array();
        $primaryMailAddress = "";
        $mailSizeLimit = NULL;
        $alternateAddresses = array();

        if($is_account){

            // Load list of members 
            $memberList = $rpc->gwDistGetMembers($this->orig_cn);
            if(!$rpc->success()){
                $this->rpcError = TRUE;
                $this->rpcErrorMessage = $rpc->get_error();
                $message = sprintf(_("Failed to load members for distribution list '%s'! Error was: '%s'."), 
                        $this->orig_cn, $rpc->get_error());
                msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                return;
            }

            // Now get the primary mail address
            $primaryMailAddress = $rpc->gwDistGetPrimaryMailAddress($this->orig_cn); 
            if(!$rpc->success()){
                $this->rpcError = TRUE;
                $this->rpcErrorMessage = $rpc->get_error();
                $message = sprintf(_("Failed to load the primary mail address for distribution list '%s'! Error was: '%s'."), 
                        $this->orig_cn, $rpc->get_error());
                msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                return;
            }

            // Load alternate mail address 
            if($this->featureEnabled('alternateAddresses')){
                $alternateAddresses = $rpc->gwDistGetAlternateMailAddresses($this->orig_cn); 
                if(!$rpc->success()){
                    $this->rpcError = TRUE;
                    $this->rpcErrorMessage = $rpc->get_error();
                    $message = sprintf(_("Failed to load alternate mail addresses for distribution list '%s'! Error was: '%s'."), 
                            $this->orig_cn, $rpc->get_error());
                    msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                    return;
                }
            }

            // Load mail size limitation settings 
            if($this->featureEnabled('mailSizeLimit')){
                $mailSizeLimit = $rpc->gwDistGetMailLimit($this->orig_cn); 
                if(!$rpc->success()){
                    $this->rpcError = TRUE;
                    $this->rpcErrorMessage = $rpc->get_error();
                    $message = sprintf(_("Failed to load mail size limit for distribution list '%s'! Error was: '%s'."), 
                            $this->orig_cn, $rpc->get_error());
                    msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                    return;
                }
                $mailSizeLimit = $mailSizeLimit['receive'];
            }
        }

        // Store values as current and initial values (saved_attributes) 
        //  to be able to keep track och changes.
        $this->is_account = $this->initially_was_account = $is_account;
        $this->saved_attributes = array();
        $this->memberList = $this->saved_attributes['memberList'] = $memberList;
        $this->primaryMailAddress = $this->saved_attributes['primaryMailAddress'] = $primaryMailAddress;
        $this->alternateAddresses = $this->saved_attributes['alternateAddresses'] = $alternateAddresses;
        $this->mailSizeLimit = $this->saved_attributes['mailSizeLimit'] = $mailSizeLimit;
        $this->capabilities = $capabilities;        
        $this->initialized = TRUE;
    }


    /*!  \brief     Gets the uids for the ogroup members.
     */
    function getOgroupMemberList()
    {
        $ldap = $this->config->get_ldap_link();
        $ldap->cd($this->config->current['BASE']);
        $members = array();
        foreach($this->parent->by_object['ogroup']->memberList as $dn => $data){
            if($data['type'] == 'U'){
                $ldap->cat($dn, array('uid'));
                if($ldap->count()){
                    $attrs = $ldap->fetch();
                    if(isset($attrs['uid'][0])) $members[] = $attrs['uid'][0];
                }
            }
        }
        return($members);
    }


    function execute()
    {
        plugin::execute();

        // Initialization failed - Display a stripped template which allows 
        //  to retry initialization
        if(!$this->initialized){
            $smarty = get_smarty();
            $smarty->assign('rpcError' , $this->rpcError);
            $smarty->assign('rpcErrorMessage' , $this->rpcErrorMessage);
            return($smarty->fetch(get_template_path('GroupwareDistributionList/initFailed.tpl', TRUE)));
        }

        // Log account access
        if($this->is_account && !$this->view_logged){
            $this->view_logged = TRUE;
            new log("view","ogroups/".get_class($this),$this->dn);
        }

        // Allow to add or remove the distribution list extension 
        if(isset($_POST['modify_state'])){
            if($this->is_account && $this->acl_is_removeable()){
                $this->is_account= FALSE;
            }elseif(!$this->is_account && $this->acl_is_createable()){
                $this->is_account= TRUE;
            }
        }

        // Show account status-changer
        if ($this->parent !== NULL){
            if ($this->is_account){
                $display= $this->show_disable_header(_("Remove distribution list"),
                        msgPool::featuresEnabled(_("Distribution list")));
            } else {
                $display= $this->show_enable_header(_("Create distribution list"),
                        msgPool::featuresDisabled(_("Distribution list")));
                return ($display);
            }
        }


        /****************
          Alternate addresses
         ****************/

        // Add manually inserted alternate mail address.
        if (isset($_POST['addAlternateAddress'])){
            $valid= FALSE;
            if (!tests::is_email($_POST['alternateAddressInput'])){
                msg_dialog::display(_("Error"),msgPool::invalid(_("Mail address"),
                            "","","your-domain@your-domain.com"),ERROR_DIALOG);
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
          Generate HTML output
         ****************/

        $smarty = get_smarty();
        $smarty->assign('rpcError' , $this->rpcError);
        foreach(array("primaryMailAddress","alternateAddresses","mailSizeLimit") as $attr){
            $smarty->assign("{$attr}ACL", $this->getacl($attr));
            $smarty->assign($attr, set_post($this->$attr));
        }
        foreach(array('mailSizeLimit','alternateAddresses') as $feature){
            $smarty->assign("{$feature}_isActive", $this->featureEnabled($feature));
        }
        

        $smarty->assign("useMailSizeLimit", ($this->mailSizeLimit != NULL));
        $smarty->assign("memberList", set_post($this->memberList));
        $smarty->assign('rpcErrorMessage' , $this->rpcErrorMessage);
        return($display.$smarty->fetch(get_template_path('GroupwareDistributionList/generic.tpl', TRUE)));
    }


    /*! \brief  ACL settings
     */
    static function plInfo()
    {
        return (array(
                    "plShortName"     => _("Distribution list"),
                    "plDescription"   => _("Groupware distribution lists"),
                    "plSelfModify"    => FALSE,
                    "plDepends"       => array("ogroup"),                     // This plugin depends on
                    "plPriority"      => 4,                                 // Position in tabs
                    "plSection"     => array("administration"),
                    "plCategory"    => array("ogroups"),
                    "plOptions"       => array(),
                    "plProvidedAcls"  => array(
                        "primaryMailAddress"   => _("Mail address"),
                        "mailSizeLimit"   => _("Mail size limit"),
                        "alternateAddresses"   => _("Alternate mail addresses"))
                    ));
    }


    /*! \brief  Get posted values and check which are interesting for us.
     */ 
    function save_object()
    {
        if(isset($_POST['retryInit'])){
            $this->init();
        }
        if(isset($_POST['DistributionList_posted'])){
            if(!isset($_POST['useMailSizeLimit'])) $this->mailSizeLimit = NULL;
            plugin::save_object();
        }
    }


    /*! \brief  Add given mail address to the list of alternate adresses,
     *           check if this mal address is used, skip addition in this case.
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
            if (!in_array($address, $this->alternateAddresses)){
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


    /*!   \brief    Removes the distribution list extension from the current 
     *               object group.
     */  
    function remove_from_parent()
    {
        // Do nothing if this asn't an active account before.
        if(!$this->initially_was_account) return;

        // Remove distribution list 
        $rpc = $this->config->getRpcHandle();
        $rpc->gwDistDel($this->orig_cn);

        // An error occured abort here
        if(!$rpc->success()){
            $this->rpcError = TRUE;
            $this->rpcErrorMessage = $rpc->get_error();
            $message = sprintf(_("Failed to remove the distribution list '%s'! Error was: '%s'."), 
                    $this->orig_cn, $rpc->get_error());
            msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
            return;
        }
    }


    /*!   \brief    Saves the distribution list extension for the current 
     *               object group.
     */  
    function save()
    {
        // Cast mailSizeLimit to integer
        if($this->mailSizeLimit!=NULL){
           $this->mailSizeLimit += 0;
        }

        // If this is a new account then add it.
        $rpc = $this->config->getRpcHandle();
        if(!$this->initially_was_account){
            $rpc->gwDistAdd($this->cn, $this->primaryMailAddress);
            if(!$rpc->success()){
                $message = sprintf(_("Failed to add distribution list '%s'! Error was: '%s'."), 
                        $this->cn, $rpc->get_error());
                return;
            }
        }

        // Rename distribution list if cn has changed.
        if($this->cn != $this->orig_cn && $this->initially_was_account){
            $rpc->gwDistRename($this->orig_cn, $this->cn);
            if(!$rpc->success()){
                $message = sprintf(_("Failed to rename distribution list '%s' to '%s'! Error was: '%s'."), 
                        $this->orig_cn,$this->cn, $rpc->get_error());
                msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                return;
            }
        }

        // Updated primaryMailAddress value, if needed:  
        //  -> the value for 'primaryMailAddress' has changed.
        if($this->initially_was_account && $this->primaryMailAddress != $this->saved_attributes['primaryMailAddress']){
            $rpc->gwDistSetPrimaryMailAddress($this->cn, $this->primaryMailAddress);
            if(!$rpc->success()){
                $message = sprintf(_("Failed to update the primary mail address for distribution list '%s'! Error was: '%s'."), 
                        $this->cn, $rpc->get_error());
                msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
            }
        }

        // Updated mail size limitations, if needed:  
        //  -> is a new distribution list or
        //  -> the value for 'mailSizeLimit' has changed.
        if($this->featureEnabled('mailSizeLimit')){
            if(!$this->initially_was_account || $this->mailSizeLimit != $this->saved_attributes['mailSizeLimit']){
                $rpc->gwDistSetMailLimit($this->cn, $this->mailSizeLimit);
                if(!$rpc->success()){
                    $message = sprintf(_("Failed to update mail size limitations for distribution list '%s'! Error was: '%s'."), 
                            $this->cn, $rpc->get_error());
                    msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                }
            }
        }

        // Updated alternate mail addresses, if needed:  
        //  -> is a new distribution list or
        //  -> the value for 'alternateAddresses' has changed.
        if($this->featureEnabled('alternateAddresses')){
            sort($this->alternateAddresses);
            sort($this->saved_attributes['alternateAddresses']);
            $changed = array_differs($this->alternateAddresses,$this->saved_attributes['alternateAddresses']);
            if(!$this->initially_was_account || $changed){
                $rpc->gwDistSetAlternateMailAddresses($this->cn, $this->alternateAddresses);
                if(!$rpc->success()){
                    $message = sprintf(_("Failed to update alternate addresses for distribution list '%s'! Error was: '%s'."), 
                            $this->cn, $rpc->get_error());
                    msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                }
            }
        }

        // Check which object-group member have a valid groupware mail addresses.
        // We can only add valid groupware extensions as distribution list member.
        $uids = $this->getOgroupMemberList();
        $rpc = $this->config->getRpcHandle(); 
        $verified = $rpc->gwVerifyAcct($uids);
        $hasExt = array();
        if(!$rpc->success()){
            $messages[] = sprintf(_("Failed to verify account ids! Error was: '%s'."),$rpc->get_error());
        }else{
            $hasExt = array();
            foreach($verified as $uid => $mail){
                if(!empty($mail)) $hasExt[] = $mail;
            }   
        }

        // Check for added or removed distribution list members now.
        $all = array_merge($hasExt, $this->saved_attributes['memberList']);
        foreach($all as $member){
            if(!in_array($member,$hasExt)){
                $rpc->gwDistDelMember($this->cn, $member);
                if(!$rpc->success()){
                    $message = sprintf(_("Failed to remove member '%s' from distribution list '%s'! Error was: '%s'."), 
                            $member, $this->cn, $rpc->get_error());
                    msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                }
            }elseif(!in_array($member,$this->saved_attributes['memberList'])){
                $rpc->gwDistAddMember($this->cn, $member);
                if(!$rpc->success()){
                    $message = sprintf(_("Failed to add member '%s' to distribution list '%s'! Error was: '%s'."), 
                            $member, $this->cn, $rpc->get_error());
                    msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                }
            }
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

    
    function check()
    {
        $messages = plugin::check();

        // Get current object-group name maybe it is invalid for us.
        if(isset($this->parent->by_object['ogroup']->cn)){
            $this->cn = &$this->parent->by_object['ogroup']->cn;
        }

        // Validate email address
        if(!tests::is_email($this->primaryMailAddress)) {
            $messages[] = msgPool::invalid(_("Mail address"), $this->primaryMailAddress, '', 'example@gonicus.de');
        }

        // Validate email address
        if($this->featureEnabled('mailSizeLimit')){
            if($this->mailSizeLimit != NULL && !is_numeric($this->mailSizeLimit)){
                $messages[] = msgPool::invalid(_("Incoming mail size limitation"), $this->mailSizeLimit, '/[0-9]/' );
            }
        }

        // Check which accounts have valid groupware mail addresses.
        if(!count($messages)){
            $uids = $this->getOgroupMemberList();
            $rpc = $this->config->getRpcHandle(); 
            $verified = $rpc->gwVerifyAcct($uids);
            if(!$rpc->success()){
                $messages[] = sprintf(_("Failed to verify account ids! Error was: '%s'."),$rpc->get_error());
            }else{
                $noExt = array();
                foreach($verified as $uid => $mail){
                    if(empty($mail)) $noExt[] = $uid;
                }   
                if(count($noExt)){
                    $message = sprintf(_("The following object group members do not have a valid groupware extension and will be ignored as distribution list members: %s"),msgPool::buildList($noExt));
                    msg_dialog::display(_("Info"),$message, INFO_DIALOG);
                }
            }
        }
        
        return($messages);
    }
}
?>
