<?php


/*! \brief  The opsi client base class.
  This class can be implemented in tow different ways:
 * as standalone opsi client
 * as part of the samba tabs 
 both types will be detected automatically.

 This class allows to edit the properties of an opsi client
 and its products.
 */
class opsiGeneric extends plugin
{
    /* Contains a list of all available netboot products 
     */
    private $a_availableNetbootProducts = array();
    private $a_initial_availableNetbootProducts = array();
    private $s_selectedNetbootProduct = "";  
    private $s_initial_selectedNetbootProduct = "";  

    /* Contains a list of all available local products
     */
    private $a_availableLocalProducts = array();
    private $a_selectedLocalProducts = array();
    private $a_initial_selectedLocalProducts = array();

    /* Internal veriables 
     */ 
    private $opsi;            // The opsi handle
    public  $parent = NULL;   // The parent object (in case of samba)

    public  $hostId       = ""; // The host Id of the currently edit opsi host  
    public  $mac          = ""; // The hosts mac address
    public  $note         = ""; // A note
    public  $description  = ""; // The client description

    public  $initial_mac          = ""; 
    public  $initial_note         = ""; 
    public  $initial_description  = ""; 

    private $init_failed = FALSE; // Is true if the opsi communication failed
    private $standalone  = TRUE;  // Is true if this is a standlone plugin. (Not samba)
    private $is_installed= FALSE; // Is true is the hast is already installed.

    public $attributes = array("mac","note","description");

    public $netConfigDNS = NULL;

    /*! \brief  Initialize this class 
      @param  Object    The GOsa base config.
      @param  String    The Id of the host that we want to edit.
      @param  Object    The parent object. (in case of samba)
     */
    public function __construct(&$config,$hostId,&$parent = NULL)
    {
        /* Create opsi handle
         */
        $this->initTime = microtime(TRUE);
        $this->opsi = new opsi($config); 
        $this->config = $config;

        /* Check if we are are part of a windows workstation 
         */
        $this->parent = $parent;
        if($parent instanceof wingeneric){
            $this->standalone  = FALSE;
        }

        /* Get hostId 
         */
        if($hostId != "new"){
            if(preg_match("/^opsi:/",$hostId)){
                $this->hostId = preg_replace("/^opsi:=([^,]*),.*$/","\\1",$hostId);
            }elseif($this->parent instanceof wingeneric){
                $this->hostId = $this->parent->cn;
                $this->hostId = preg_replace('/\$$/',"",$this->hostId);
            }
        }

        /* Try to plugin */
        $this->init();

        // Prepare lists
        $this->selectedList = new sortableListing();
        $this->selectedList->setDeleteable(false);
        $this->selectedList->setEditable(false);
        $this->selectedList->setWidth("100%");
        $this->selectedList->setHeight("200px");
        $this->selectedList->setColspecs(array('*','*','40px'));
        $this->selectedList->setHeader(array(_("Name"),_("Description")));
        $this->selectedList->setDefaultSortColumn(1);
        $this->selectedList->setAcl('rwcdm'); // All ACLs, we filter on our own here.

        // Prepare lists
        $this->availableList = new sortableListing();
        $this->availableList->setDeleteable(false);
        $this->availableList->setEditable(false);
        $this->availableList->setWidth("100%");
        $this->availableList->setHeight("200px");
        $this->availableList->setColspecs(array('20px','*','*'));
        $this->availableList->setHeader(array("",_("Name"),_("Description")));
        $this->availableList->setDefaultSortColumn(1);

        // Create statistic table entry
        stats::log('plugin', $class = get_class($this), $category = array($this->acl_category),  $action = 'open',
                $amount = 1, $duration = (microtime(TRUE) - $this->initTime));

    }


    /*! \brief  Try to load opsi client informations from the 
      gosa support daemon.
     */
    private function init()
    {
        $err = FALSE;
        $this->init_failed = FALSE;
        $this->initially_was_account = FALSE; 

        /* We are a standalone plugin.
         */
        if($this->standalone ) {
            $this->is_account = TRUE;
        }


        /* Try to load client infos from the gosa support daemon
         */
        if(!empty($this->hostId)){
            $list = $this->opsi->list_clients($this->hostId);
            $err |= $this->opsi->is_error();

            /* Walk through all returned opsi clients and try to detect 
               one that matches our hostId.
#FIXME Implement an opsi method which returns infos for only one opsi client, not all. 
             */
            foreach($list as $entry){
                if(preg_match("/^".preg_quote($this->hostId, '/')."$/i",$entry['NAME'][0]['VALUE'])){
                    $this->initially_was_account = TRUE; 
                    $this->is_account = TRUE;
                    foreach(array(
                                "is_installed" => "LASTSEEN",
                                "description"  => "DESCRIPTION",
                                "mac"          => "MAC", 
                                "note"         => "NOTES") as $des => $src){
                        $des2 = "initial_".$des;
                        $this->$des2 = $this->$des = $entry[$src][0]['VALUE'];
                    } 
                    break;
                }
            }
        }

        // Get package info
        $list = array(); 
        $list = $this->opsi->get_full_product_host_information($this->hostId);
        $err |= $this->opsi->is_error();

        /* Read informations about available netboot products. 
           If not already done, before.
         */
        if(!$err){
            foreach($list as $key => $entry){

                $tmp = array('DESC'=>$entry['data']['DESCRIPTION'],'NAME'=>$key);
                if($entry['configurable']){
                    $tmp['CFG'] = $entry['data']['PROPERTIES'];
                } 

                if($entry['data']['TYPE'] == "localboot"){
                    if(!$entry['installed']){
                        $this->a_availableLocalProducts[$key] = $tmp;
                    }else{
                        $this->a_selectedLocalProducts[$key] = $tmp;
                    }
                }else{
                    $this->a_availableNetbootProducts[$key] = $tmp;
                    if($entry['installed']){
                        $this->s_selectedNetbootProduct= $key;
                    }
                }
            }
        }

        /* Check if everything went fine else reset everything and display a retry button 
         */
        if($err){
            $this->init_failed = TRUE;

        }else{

            /* Remember initial settings */
            $this->a_initial_selectedLocalProducts = $this->a_selectedLocalProducts;
            $this->s_initial_selectedNetbootProduct = $this->s_selectedNetbootProduct;
            $this->a_initial_availableNetbootProducts = $this->a_availableNetbootProducts;

            /* Ensure that a valid netboot is selected product is.
             */
            if(empty($this->s_selectedNetbootProduct)){
                $this->s_selectedNetbootProduct = key($this->a_availableNetbootProducts);
            }

            // Now fake a valid ldap entry ... this is necessary to avoid 
            //  modifications in the dns/dhcp classes

            // First fake cn 
            $this->attrs['hostId'][0] = $this->hostId;
            $this->attrs['cn'][0] = $this->hostId;

            // Second detect DNS settings. 
            $ldap = $this->config->get_ldap_link();
            $ldap->cd($this->config->current['BASE']);
            $strippedHostId = preg_replace("/\..*$/","",$this->hostId);
            $ldap->search("(&(objectClass=dNSZone)(|(relativeDomainName=".$this->hostId.")(relativeDomainName=".$strippedHostId."))(aRecord=*))",array("aRecord"));
            if($ldap->count()){
                $attrs = $ldap->fetch();
                $this->attrs['ipHostNumber']['count'] = 0;
                $this->attrs['ipHostNumber'][0] = $attrs['aRecord'][0];
            }

            $this->attrs['macAddress']['count'] = 1;
            $this->attrs['macAddress'][0] = &$this->mac;

            // Initialize DHCP and DNS 
            $this->netConfigDNS = new termDNS($this->config,$this,$this->objectclasses, FALSE, "hostId");
            $this->netConfigDNS->set_acl_category("opsi");
            $this->netConfigDNS->set_acl_base($this->config->current['BASE']);
            $this->netConfigDNS->IPisMust = FALSE;
            $this->netConfigDNS->MACisMust = FALSE;
        }
    }


    /*! \brief  Check given data.
      @return Array   Returns an array with all issues.
     */
    public function check()
    {

        // In case of initialization problem, we do not save anything.
        // We can skip checks here, the date isn't usable.
        if($this->init_failed){ 
            return;
        };

        $messages = plugin::check();
        $messages= array_merge($messages, $this->netConfigDNS->check());

        if(empty($this->hostId)){
            $messages[] = msgPool::required(_("Name"));
        }elseif(!preg_match("/\./",$this->hostId)){

            /* The hostId must contain a domain part 
             */
            $messages[] = msgPool::invalid(_("Name"),$this->hostId,"",
                    _("The field 'Name' must contain a domain part!"));
        }elseif(preg_match("/[^a-z0-9\.\-_]/",$this->hostId)){
            $messages[] = msgPool::invalid(_("Name"),$this->hostId,"/[a-z0-9\.\-_]/");
        }

        /* Ensure that the mac address is valid
         */
        if(!tests::is_mac($this->mac) || empty($this->mac)){
            $messages[] = msgPool::invalid(_("MAC address"),$this->mac,"","00:0C:7F:31:33:F1");
        }
        return($messages);
    }


    /*! \brief  Create the html ui of this plugin
      @return String  HTML content.
     */
    public function execute()
    {
        plugin::execute();
        $display ="";

        /* The pluign initialization failed due to communication problems with the gosa daemon. 
           A retry button will be displayed here.
         */
        if($this->init_failed){
            $smarty = get_smarty();
            $smarty->assign("standalone ", $this->standalone );
            $smarty->assign("init_failed",TRUE);
            $smarty->assign("message",$this->opsi->get_error());
            return($smarty->fetch(get_template_path("generic.tpl",TRUE,dirname(__FILE__))));
        }  


        /* If we are not a stand alone opsi client, we must be a samba client 
           which has the opsi tab enabled.
           Check if the opsi is added or removed and display state buttons.
         */
        if(!$this->standalone ){

            /* Do we need to flip is_account state? */
            if(isset($_POST['modify_state'])){
                if($this->is_account && $this->acl_is_removeable()){
                    $this->is_account= FALSE;
                }elseif(!$this->is_account && $this->acl_is_createable()){
                    $this->is_account= TRUE;
                }
            }
            if($this->is_account){
                $display = $this->show_disable_header(msgPool::removeFeaturesButton(_("OPSI")), 
                        msgPool::featuresEnabled(_("OPSI")));
            }else{
                $display = $this->show_enable_header(msgPool::addFeaturesButton(_("OPSI")), 
                        msgPool::featuresDisabled(_("OPSI")));
                return($display);
            } 
        } 

        /* Check if we have a sub dialog opened
         */
        if(is_object($this->dialog)){
            $this->dialog->save_object();
            return($this->dialog->execute());
        }

        /* Create HTML output of this plugin
         */
        $smarty = get_smarty();
        $smarty->assign("standalone", $this->standalone );
        foreach($this->attributes as $attr){
            $smarty->assign($attr,$this->$attr);
        }

        /* Assign ACLs */    
        $tmp = $this->plInfo();
        foreach($tmp['plProvidedAcls'] as $name => $translated){
            $smarty->assign($name."ACL",$this->getacl($name));
        }

        $smarty->assign("is_installed", $this->is_installed);
        $smarty->assign("init_failed",FALSE);

        /* Create list of available local products 
         */
        $data = $lData=array();
        foreach($this->a_availableLocalProducts as $name => $data){
            if(isset($this->a_selectedLocalProducts[$name])) continue;
            $add_tab = "";
            if($this->acl_is_writeable("localProduct")){
                $add_tab  = image('images/back.png','add_lp_'.$name);
            }
            $data[$name] = $name;
            $lData[$name] = array('data' => array($add_tab,$name,$data['DESC']));
        }
        $this->availableList->setAcl($this->getacl("localProduct"));
        $this->availableList->setListData($data,$lData);
        $this->availableList->update();

        /* Create list of selected local products 
         */
        $data = $lData=array();
        foreach($this->a_selectedLocalProducts as $name => $data){
            $edit = image('images/empty.png');
            if(count($data['CFG'])){
                $edit = image('images/lists/edit.png','edit_lp_'.$name);
            }
            $del = image('images/lists/trash.png','del_lp_'.$name);
            $data[$name] = $name;
            $lData[$name] = array('data' => array( $name,$data['DESC'],$edit.$del));
        }
        $this->selectedList->setAcl($this->getacl("localProduct"));
        $this->selectedList->setListData($data,$lData);
        $this->selectedList->update();

        /* Check if netboot product is configurable 
         */
        $cfg_able =FALSE;
        if(isset($this->a_availableNetbootProducts[$this->s_selectedNetbootProduct]['CFG'])){
            $cfg_able = count($this->a_availableNetbootProducts[$this->s_selectedNetbootProduct]['CFG']);
            $cfg_able &= $this->acl_is_readable("netbootProduct");
        }

        // Get net config template
        $str = $this->netConfigDNS->execute();
        if(is_object($this->netConfigDNS->dialog)){
            return($str);
        }
        $smarty->assign("netconfig", $str);

        $smarty->assign("netboot_configurable",$cfg_able);
        $smarty->assign("hostId", $this->hostId);
        $smarty->assign("divSLP", $this->selectedList->render());
        $smarty->assign("divALP", $this->availableList->render());
        $smarty->assign("SNP", $this->s_selectedNetbootProduct);
        $smarty->assign("ANP", $this->a_availableNetbootProducts);
        return($display.$smarty->fetch(get_template_path("generic.tpl",TRUE,dirname(__FILE__))));
    }


    /*! \brief  Save modifications using the gosa support daemon.
     */
    public function save()
    {
        if($this->init_failed){ 
            return;
        }

        /* Check if we have to create a new opsi client
           or just have to save client modifications.
         */
        if(!$this->initially_was_account && $this->is_account){
            $res = $this->opsi->add_client($this->hostId,$this->mac,$this->note,$this->description);
            if($this->opsi->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->opsi->get_error()),ERROR_DIALOG);    
                return;
            }
        }else{

            /* Update client modifcations.
               -Only if necessary  
             */
            if($this->note != $this->initial_note || 
                    $this->description != $this->initial_description ||
                    $this->mac != $this->initial_mac){
                $this->opsi->modify_client($this->hostId,$this->mac,$this->note,$this->description);
                if($this->opsi->is_error()){
                    msg_dialog::display(_("Error"),msgPool::siError($this->opsi->get_error()),ERROR_DIALOG);
                    return;
                }
            }
        }


        /***********
          Detect local netboot product changes
          - Check which products were removed.
          - Check which products were added. 
         ***********/


        /* Detect which products were removed an which added.
         */
        $del = array_diff_assoc($this->a_initial_selectedLocalProducts,$this->a_selectedLocalProducts);
        $add = array_diff_assoc($this->a_selectedLocalProducts,$this->a_initial_selectedLocalProducts);

        /* Remove products from client
         */
        foreach($del as $name => $data){
            $this->opsi->del_product_from_client($name,$this->hostId);
            if($this->opsi->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->opsi->get_error()),ERROR_DIALOG);    
                return;
            }
        }

        /* Add products to client
           And set the product properties.
         */
        foreach($add as $name => $data){
            $this->opsi->add_product_to_client($name,$this->hostId);
            if($this->opsi->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->opsi->get_error()),ERROR_DIALOG);    
                return;
            }
            if(!empty($data['CFG'])){
                $this->opsi->set_product_properties($name,$data['CFG'],$this->hostId);
                if($this->opsi->is_error()){
                    msg_dialog::display(_("Error"),msgPool::siError($this->opsi->get_error()),ERROR_DIALOG);    
                    return;
                }
            }
        }

        /* Save local product properties 
         */
        foreach($this->a_selectedLocalProducts as $name => $data){
            if(isset($del[$name]) || isset($add[$name])) continue;

            /* Update product properties if there are changes 
             */
            $diffs = $this->get_config_changes($data['CFG'],$this->a_initial_selectedLocalProducts[$name]['CFG']);
            if(count($diffs)){
                $this->opsi->set_product_properties($name,$diffs,$this->hostId);
                if($this->opsi->is_error()){
                    msg_dialog::display(_("Error"),msgPool::siError($this->opsi->get_error()),ERROR_DIALOG);    
                    return;
                }
            }
        }

        /*********
          Detect Netboot product changes
          - Check if another netboot product was selected. 
          - Check if the product properties were changes.
         *********/

        /* Update used netboot product. 
         */
        if($this->s_selectedNetbootProduct != $this->s_initial_selectedNetbootProduct){
            if(!empty($this->s_initial_selectedNetbootProduct)){
                $this->opsi->del_product_from_client($this->s_initial_selectedNetbootProduct,$this->hostId);
                if($this->opsi->is_error()){
                    msg_dialog::display(_("Error"),msgPool::siError($this->opsi->get_error()),ERROR_DIALOG);    
                    return;
                }
            }
            $this->opsi->add_product_to_client($this->s_selectedNetbootProduct,$this->hostId);
            if($this->opsi->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->opsi->get_error()),ERROR_DIALOG);    
                return;
            }
        }

        /* Check if we have to update the netboot product properties 
           This is the case, if this product is newly selected.
           Or if there was at least one configuration attribute modified.
         */
        $cfg_1 = $cfg_2 = array();
        if(isset($this->a_availableNetbootProducts[$this->s_selectedNetbootProduct]['CFG'])){
            $cfg_1 = $this->a_availableNetbootProducts[$this->s_selectedNetbootProduct]['CFG'];
        }
        if(isset($this->a_initial_availableNetbootProducts[$this->s_selectedNetbootProduct]['CFG'])){
            $cfg_2 = $this->a_initial_availableNetbootProducts[$this->s_selectedNetbootProduct]['CFG'];
        }
        $diffs = $this->get_config_changes($cfg_1,$cfg_2);
        $to_update = array();
        if( !$this->initially_was_account || 
                $this->s_selectedNetbootProduct != $this->s_initial_selectedNetbootProduct){
            $to_update = $this->a_availableNetbootProducts[$this->s_selectedNetbootProduct]['CFG'];
        }elseif(count($diffs)){
            $to_update = $diffs;
        }

        if(count($to_update)){
            $name = $this->s_selectedNetbootProduct;
            $this->opsi->set_product_properties($name,$to_update,$this->hostId);
            if($this->opsi->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->opsi->get_error()),ERROR_DIALOG);
                return;
            }
        }

        $this->netConfigDNS->cn = $this->hostId;
        $this->netConfigDNS->save();
    }


    public function get_config_changes($c1,$c2)
    {
        /* Get key which are not present in both entries 
         */
        $res = array();
        foreach($c2 as $name => $value){
            if(!isset($c1[$name]) || $c1[$name]['CURRENT'] != $c2[$name]['CURRENT']){
                $res[$name] = $c2[$name];
            }
        }
        foreach($c1 as $name => $value){
            if(!isset($c2[$name]) || $c2[$name]['CURRENT'] != $c1[$name]['CURRENT']){
                $res[$name] = $c1[$name];
            }
        }
        return($res);
    }


    /*! \brief  Removes the opsi client 
     */  
    public function remove_from_parent()
    {
        if($this->init_failed){ 
            return;
        }
        $this->netConfigDNS->remove_from_parent();

        $this->opsi->del_client($this->hostId);
        if($this->opsi->is_error()){
            msg_dialog::display(_("Error"),msgPool::siError($this->opsi->get_error()),ERROR_DIALOG);
            return;
        }
    }


    /*! \brief  Save html posts 
     */
    public function save_object()
    {
        /* Init failed; reinit is triggered here.
         */
        if(isset($_POST['reinit']) && $this->init_failed){
            $this->init();
        }

        /* Property are currently edited, close the dialog. 
         */
        if(isset($_POST['cancel_properties']) && is_object($this->dialog)){
            $this->dialog = NULL;
        }

        /* Save product property changes 
         */
        if(isset($_POST['save_properties']) && ($this->dialog instanceof opsiProperties)){
            $this->dialog->save_object();
            $pro = $this->dialog->get_product();
            $CFG = $this->dialog->get_cfg();
            if(isset($this->a_selectedLocalProducts[$pro])){
                if($this->acl_is_writeable("localProduct")){
                    $this->a_selectedLocalProducts[$pro]['CFG'] = $CFG;
                }
                $this->dialog = NULL;
            }elseif($this->s_selectedNetbootProduct == $pro){
                if($this->acl_is_writeable("netbootProduct")){
                    $this->a_availableNetbootProducts[$pro]['CFG'] = $CFG;
                }
                $this->dialog = NULL;
            }else{
                trigger_error("Fatal, unknown product was configured.");
            }
        }

        /* Save html post
         */
        if(isset($_POST['opsiGeneric_posted'])){

            plugin::save_object();
            $this->netConfigDNS->save_object();
            $this->mac = $this->netConfigDNS->macAddress;

            /* Get hostId 
             */
            if(isset($_POST['hostId']) && $this->standalone && $this->acl_is_writeable("hostId")){
                $this->hostId = get_post('hostId');
            }

            /* Send actions like 'install' or 'wake' to the si server 
             */
            if($this->acl_is_writeable("triggerAction") && 
                    isset($_POST['opsi_action']) && 
                    isset($_POST['opsi_trigger_action']) && 
                    $this->standalone ){
                $action = get_post('opsi_action');
                if($action == "install"){
                    $this->install_client(); 
                }
                if($action == "wake"){
                    $this->wake_client(); 
                }
            }

            /* Get selected netboot product.
             */
            if(isset($_POST['opsi_netboot_product']) && $this->acl_is_writeable("netbootProduct")){
                $SNP = get_post('opsi_netboot_product');
                if(isset($this->a_availableNetbootProducts[$SNP])){
                    if(!isset($this->a_availableNetbootProducts[$SNP]['CFG'])){
                        $CFG = $this->opsi->get_product_properties($SNP);
                        $this->a_availableNetbootProducts[$SNP]['CFG'] = $CFG;
                        if($this->opsi->is_error()){
                            $this->init_failed = TRUE;
                            return;
                        }
                    }
                    $this->s_selectedNetbootProduct = $SNP;
                }
            }

            /* Add/remove/edit local products 
             */
            foreach($_POST as $name => $value){

                /* Check if netboot product configuration is requested 
                 */
                if(preg_match("/^configure_netboot/",$name) && $this->acl_is_readable("netbootProduct")){
                    $pro = $this->s_selectedNetbootProduct;
                    $cfg = $this->a_availableNetbootProducts[$pro]['CFG'];
                    $this->dialog = new opsiProperties($this->config,$pro,$cfg,$this->hostId);
                    break;
                }

                /* Add product 
                 */
                if(preg_match("/^add_lp_/",$name) && $this->acl_is_writeable("localProduct")){
                    $product = preg_replace("/^add_lp_(.*)$/","\\1",$name);
                    if(isset($this->a_availableLocalProducts[$product]) && !isset($this->a_selectedLocalProducts[$product])){
                        $this->a_selectedLocalProducts[$product] = $this->a_availableLocalProducts[$product];
                        $CFG = $this->opsi->get_product_properties($product);
                        if($this->opsi->is_error()){
                            $this->init_failed = TRUE;
                            return;
                        }
                        $this->a_selectedLocalProducts[$product]['CFG'] = $CFG;
                    }
                    break;
                }

                /* Delete product 
                 */
                if(preg_match("/^del_lp_/",$name) && $this->acl_is_writeable("localProduct")){
                    $product = preg_replace("/^del_lp_(.*)$/","\\1",$name);
                    if(isset($this->a_selectedLocalProducts[$product])){
                        unset($this->a_selectedLocalProducts[$product]);
                    }
                    break;
                }

                /* Edit a product  
                 */
                if(preg_match("/^edit_lp_/",$name) && $this->acl_is_readable("localProduct")){
                    $product = preg_replace("/^edit_lp_(.*)$/","\\1",$name);
                    $this->dialog = new opsiProperties($this->config,
                            $product,$this->a_selectedLocalProducts[$product]['CFG'],$this->hostId);
                    break;
                }
            }   
        }
    }


    /* Triggers client installation 
     */
    function install_client()
    {
        $this->opsi->send_action("install",$this->hostId,$this->mac);
        if($this->opsi->is_error()){
            msg_dialog::display(_("Error"),msgPool::siError($this->opsi->get_error()),ERROR_DIALOG);
        }
    }


    /* Wake up client
     */
    function wake_client()
    {
        /* Check if we are able to communicate with the GOsa supprot daemon
         */
        if(class_available("gosaSupportDaemon")){
            $o = new gosaSupportDaemon();
            if($o->connect() && class_available("DaemonEvent_wakeup")){
                $evt = new DaemonEvent_wakeup($this->config);      
                $evt->add_targets(array($this->mac));
                $o->append($evt);
            }
        }
    }


    /* Return plugin informations for acl handling */
    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Generic"),
                    "plDescription" => _("OPSI generic"),
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 1,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("opsi" => array("description"  => _("Opsi"))),

                    "plProvidedAcls"=> array(
                        "hostId"          => _("Name"),
                        "mac"             => _("MAC address"),
                        "description"     => _("Description"),
                        "note"            => _("Note"),
                        "netbootProduct"  => _("Net boot product"),
                        "localProduct"    => _("Local boot product"),
                        "triggerAction"   => _("Action"))
                    ));
    }
}


// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
