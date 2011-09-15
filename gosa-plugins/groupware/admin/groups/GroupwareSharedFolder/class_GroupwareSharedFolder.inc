<?php

# Load Groupware defintions and ACLs
new GroupwareDefinitions();

/*! \brief  Allows to update groupware shared-folders 
 *           using the jsonRPC backend.
 */
class GroupwareSharedFolder extends plugin
{
    public $view_logged = FALSE;

    // Error hanlding related attributes.
    private $initialized  = FALSE;
    private $rpcError = FALSE;
    private $rpcErrorMessage = "";

    // Attribute definition 
    public $attributes = array('folderList');
    public $folderList = array();   

    // Feature handling
    private $featuresEnabled = array();
    private $FolderWidget = NULL;
    private $folderPrefix = "shared/";


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
        $map['folder'] = array("folderList","folderAdd","folderDel","folderExists");
        $map['members'] = array("folderGetMembers","folderSetMembers","folderAddMember","folderDelMember");
        foreach($map as $name => $required){
            $this->featuresEnabled[$name] = TRUE;
            foreach($required as $func){
                $this->featuresEnabled[$name] &= isset($capabilities[$func]) && $capabilities[$func];
            }
        }

        // If we're creating a new ogroup, then we definately have no extension yet.
        $this->rpcError = FALSE;
        $folderList = array();
        $is_account = FALSE;
        if($this->cn == "" || $this->dn == "new"){
            $is_account = FALSE;
        }else{

            // Load folders for the given folderPrefix           
            $this->folderPrefix = "shared/{$this->cn}";
            $folders = $rpc->gwFolderList($this->folderPrefix);
            if(!$rpc->success()){
                $this->rpcError = TRUE;
                $this->rpcErrorMessage = $rpc->get_error();
                $message = sprintf(_("Failed to load folder list! Error was: '%s'."), 
                        $rpc->get_error());
                msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                return;
            }

            // We've avalid account, if we've received at least one matching folder.
            $is_account = count($folders) != 0;

            // Prepare folders to be used in the folderWidget
            if($is_account){
                $folderList = array();
                foreach($folders as $path){
                    $name = preg_replace("/^.*\//","",$path);
                    $entry = array();
                    $entry['name'] = $name;
                    $entry['status'] = '';
                    $entry['acls'] = array();
                    $acls = array();        
                    $acls = $rpc->gwFolderGetMembers($path);
                    if(!$rpc->success()){
                        $this->rpcError = TRUE;
                        $this->rpcErrorMessage = $rpc->get_error();
                        $message = sprintf(_("Failed to load permissions for folder '%s'! Error was: '%s'."), 
                                $path,$rpc->get_error());
                        msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                        return;
                    }
                    foreach($acls as $name => $acl){
                        $entry['acls'][] = array('type' => 'user', 'acl' => $acl, 'name' => $name);
                    }
                    $folderList[$path] = $entry;
                }
            }
        }

        // We do not have a valid account, use an initial set of folders 
        //  to be able to use the folderWidget.
        // {%cn} will be replaced by the groups cn when save() is called.
        if(!$is_account){
            $this->folderPrefix = "shared/{%cn}";
            $intialFolderEntry = array('name' => '{%cn}', 'status' => 'added', 'acls' => array());
            $folderList = array();
            $folderList[$this->folderPrefix] = $intialFolderEntry;
        }

        // Store values as current and initial values (saved_attributes) 
        //  to be able to keep track och changes.
        $this->is_account = $this->initially_was_account = $is_account;
        $this->saved_attributes = array();
        $this->folderList = $this->saved_attributes['folderList'] = $folderList;
        $this->capabilities = $capabilities;        
        $this->initialized = TRUE;
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
            return($smarty->fetch(get_template_path('GroupwareSharedFolder/initFailed.tpl', TRUE)));
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
                $display= $this->show_disable_header(_("Remove shared folder"),
                        msgPool::featuresEnabled(_("Shared folder")));
            } else {
                $display= $this->show_enable_header(_("Create shared folder"),
                        msgPool::featuresDisabled(_("Shared folder")));
                return ($display);
            }
        }

        /****************
          Folder editor
         ****************/


        if(!$this->FolderWidget){
            $this->FolderWidget = new FolderWidget($this->config,$this->folderList, $this->folderPrefix);
            $this->FolderWidget->setPermissions(GroupwareDefinitions::getPermissions());
            $this->FolderWidget->acl_base = $this->acl_base;
            $this->FolderWidget->acl_category = $this->acl_category;
            $this->FolderWidget->showButtons(FALSE);
        }
        $this->folderList = $this->FolderWidget->save();
        $str = $this->FolderWidget->execute();
        $this->dialog = $this->FolderWidget->dialogOpened();       
        return($str);


#       ONCE WE HAVE MORE FEATURE THAN JUST THE FOLDER SETTINGS, 
#       ENABLE THIS AGAIN AN REMOVE THE ABOVE LINES.
#
#       if(isset($_POST['FolderWidget_cancel'])) $this->FolderWidget = NULL;
#       if(isset($_POST['FolderWidget_ok'])){
#           $this->FolderWidget->save_object();
#           $msgs = $this->FolderWidget->check();
#           if(count($msgs)){
#               msg_dialog::displayChecks($msgs);
#           }else{
#               $this->folderList = $this->FolderWidget->save();
#               $this->FolderWidget = NULL;
#           }
#       }
#       if(isset($_POST['configureFolder'])){
#           $this->FolderWidget = new FolderWidget($this->config,$this->folderList, $this->folderPrefix);
#           $this->FolderWidget->setPermissions(GroupwareDefinitions::getPermissions());
#           $this->FolderWidget->acl_base = $this->acl_base;
#           $this->FolderWidget->acl_category = $this->acl_category;
#       }
#       $this->dialog = FALSE;
#       if($this->FolderWidget instanceOf FolderWidget){
#           $this->FolderWidget->save_object();
#           $this->dialog = TRUE;
#           return($this->FolderWidget->execute());
#       }
#

        /****************
          Generate HTML output
         ****************/

        $plInfo = $this->plInfo();

        $smarty = get_smarty();

        foreach($plInfo['plProvidedAcls'] as $name => $translation){
            $smarty->assign("{$name}ACL", $this->getacl($name));
        }

        $smarty->assign('rpcError' , $this->rpcError);
        $smarty->assign('rpcErrorMessage' , $this->rpcErrorMessage);
        return($display.$smarty->fetch(get_template_path('GroupwareSharedFolder/generic.tpl', TRUE)));
    }


    /*! \brief  Get posted values and check which are interesting for us.
     */ 
    function save_object()
    {
        if(isset($_POST['retryInit'])){
            $this->init();
        }
        if(isset($_POST['GroupwareSharedFolder_posted'])){
            plugin::save_object();
        }
        if($this->FolderWidget) $this->FolderWidget->save_object();
    }


    /*!   \brief    Removes the distribution list extension for the current 
     *               object group.
     */  
    function remove_from_parent()
    {
        echo "<br>Remove all folders";
    }


    /*!   \brief    Saves the distribution list extension for the current 
     *               object group.
     */  
    function save()
    {

        if(!$this->initialized){
            echo "Nothing to save, plugin init failed!";
            return;
        }

        $this->folderList = $this->FolderWidget->save();
        $rpc = $this->config->getRpcHandle();

        // Walk through folders and perform requested actions
        foreach($this->folderList as $path => $folder){

            // Update folder names for newly created groupware extensions.
            $path = preg_replace("/\{%cn\}/", $this->cn, $path);
            $folder['name'] = preg_replace("/\{%cn\}/", $this->cn, $folder['name']);

            // Remove shared folder.
            if($folder['status'] == 'removed'){
                $rpc->gwFolderDel($path);
                if(!$rpc->success()){
                    $message = sprintf(_("Failed to remove shared folder '%s'! Error was: '%s'."),
                            $path, $rpc->get_error());
                    msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                }

                echo "<br>Removed {$path}";
                continue;
            }

            // Add newly created folders 
            if($folder['status'] == 'added'){
                $rpc->gwFolderAdd($path);
                if(!$rpc->success()){
                    $message = sprintf(_("Failed to create shared folder '%s'! Error was: '%s'."),
                            $path, $rpc->get_error());
                    msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                }

                echo "<br>Added {$path}";
            }
          
            // Collect old and new members and their acls, to be able to compare them later.
            $oldList = $this->saved_attributes['folderList'];
            $oldAcls = array();
            $newAcls = array();
            $oldMembers = array();
            $newMembers = array();
            if(isset($oldList[$path])){
                foreach($oldList[$path]['acls'] as $entry){
                    $oldMembers[] = $entry['name'];
                    $oldAcls[$entry['name']] = $entry['acl'];
                }
            }
            foreach($folder['acls'] as $entry){
                $newMembers[] = $entry['name'];
                $newAcls[$entry['name']] = $entry['acl'];
            }

            // Check if member list has changed.
            if(array_differs($newMembers,$oldMembers) || array_differs($oldAcls, $newAcls)){
                $rpc->gwFolderSetMembers($path, $newAcls);
                if(!$rpc->success()){
                    $message = sprintf(_("Failed to create shared folder '%s'! Error was: '%s'."),
                            $path, $rpc->get_error());
                    msg_dialog::display(_("Error"),msgPool::rpcError($message), ERROR_DIALOG);
                }
                echo "<br>Updating members for {$path} ";
            }
        }
    }


    function check()
    {
        $messages = plugin::check();

        // Get current object-group name maybe it is invalid for us.
        if(isset($this->parent->by_object['group']->cn)){
            $this->cn = &$this->parent->by_object['group']->cn;
        }

        // Check if group name has changed, this is not allowed while a groupware extension is active.
        if($this->initially_was_account && $this->cn != $this->orig_cn && $this->orig_cn != ""){
            $messages[] = sprintf(
                _("Groups cannot be renamed, while their groupware extension is active, please use the intial name '%s' again!"),
                $this->orig_cn);
        }

        // Merge Folder messages.
        if($this->FolderWidget instanceOf FolderWidget){
            $messages = array_merge($messages,$this->FolderWidget->check());
        }

        return($messages);
    }


    /*! \brief  ACL settings
     */
    static function plInfo()
    {
        return (array(
                    "plShortName"     => _("Shared folder"),
                    "plDescription"   => _("Groupware shared folder"),
                    "plSelfModify"    => FALSE,
                    "plDepends"       => array("ogroup"),                     // This plugin depends on
                    "plPriority"      => 4,                                 // Position in tabs
                    "plSection"     => array("administration"),
                    "plCategory"    => array("groups"),
                    "plOptions"       => array(),
                    "plProvidedAcls"  => array(
                        "folderList"   => _("Shared folder"))
                    ));
    }
}
?>
