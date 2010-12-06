<?php

/*! \brief  A GOsa plugin which generates a device configuration dialog  
 */
class ConfigManagement extends management
{

    // Used to render the item-configuration dialog 
    private $TemplateEngine = NULL;

    // A list of all configured items for this device.
    private $allConfiguredItems = array();

    // The currently active item
    // Add add and remove action will be performed on this item
    private $currentItem = array();
    
    // A baseSelector which will be fed with a simulated 
    //  department list. All entries will get a fake base
    //  which can then be used to fill the baseSelector widget.
    private $itemContainerSelector = NULL;

    // The currently selected base within management list.
    private $base ;

    // This array contains a list of all item-types we can add 
    //  to the currently selected item (currentItem) 
    private $addableContainerItems = array();

    // Due to the fakt that we use a fake 'base/dn' for all items
    //  we've to map this 'base/dn' to the items 'id'.
    private $mappingBaseToID = array();

    private $rootItemID;
    private $itemConfig = array();
    private $lastItemID = 1;

    private $rpcError = FALSE;
    private $initFailed = FALSE;
    private $invalidInstallMethod = FALSE;
    private $errorMessage = "";

    // Some plugin related memebers which are not member of 
    //  the management class. See class plugin.
    public $initTime;
    public $ignore_account = FALSE;
    public $pl_notify;
    public $read_only;

    public $allItemConfigurations = NULL;

    /*! \brief  Constructs the device configuration plugin 
     *  @param  Config  The GOsa configuration object.
     */
    function __construct(&$config, $dn)
    {
        // Load the template engine and tell her what template
        //  to use for the HTML it produces.
        $this->TemplateEngine = new TemplateEngine($config);
        $this->TemplateEngine->setTemplate('puppet.tpl');
        $this->config = $config;

        // Set storage points - We do not have any - We just create a fake list which lists all items
        $this->storagePoints = array("");
        
        // Try to initialize
        $this->init();
        $this->rebuildListing();
    }

    
    /*! \brief  Sets the installation method to the given method.
     *          Updates the template engine and adds the initial root 
     *           object for the selected method.
     *  @param  The method to use.
     *  @return TRUE on success else FALSE. 
     */
    function setInstallMethod($str)
    {
        if(!isset($this->allItemConfigurations[$str])){
            $this->itemConfig = array();
            $this->invalidInstallMethod =TRUE;
            $this->errorMessage = sprintf(_("Invalid installation method %s selected!"), bold($str));
            msg_dialog::display(_("Setup"), $this->errorMessage, ERROR_DIALOG);
            return(FALSE);
        }else{

            $this->itemConfig = $this->allItemConfigurations[$str]['items'];
            $this->invalidInstallMethod =FALSE;
            $this->TemplateEngine->load($this->itemConfig);

            // Detect root item, its name is / 
            $root = NULL;
            foreach($this->itemConfig as $key => $item){
                if($item['name'] == '/') {
                    $root = $key;
                    break;
                }
            }
            if(!$root){
                $this->errorMessage = sprintf(_("Installation method %s is invalid: no root object found!"), bold($str));
                msg_dialog::display(_("Setup"), $this->errorMessage , ERROR_DIALOG);
                $this->initFailed = TRUE;
                $this->itemConfig = array();
                return(FALSE);
            }

            // Set current item to 'root', this is the minimum to get things running.
            $idRoot = $this->addItem($root,'root',array());
            $this->rootItemID = $idRoot;
            $this->setCurrentItem($idRoot);
            $this->setSelectedListItemID($idRoot);
            $this->rebuildListing();
            return(TRUE);
        }
    }


    /*! \brief  Intializes this plugin
     *          All available installation methods will be loaded
     *           and populated.
     */
    function init()
    {
        // Reset erros
        $this->rpcError = $this->initFailed = FALSE;

        // Load configuration via rpc.
        $rpc = $this->config->getRpcHandle();

        // Populate install methods on success.
        $res = $rpc->getSupportedInstallMethods();
        if(!$rpc->success()){
            $this->rpcError = TRUE;
            $this->errorMessage = $rpc->get_error();;
            return;
        }       
        $this->allItemConfigurations = $res;
        if(!count($this->allItemConfigurations)){
            $this->errorMessage = _("No selectable install methods available!");
            msg_dialog::display(_("Setup"), $this->errorMessage , ERROR_DIALOG);
            $this->initFailed = TRUE;
            return;
        }
    }


    /*! \brief      Add a new child-item to the currently selected one. 
     *               
     *  @param  String  type    The 'type' of the new object, eg. 'KickstartTemplate'
     *  @param  String  name    The 'name' of the new object.
     *  @param  Array   values  The initial values for this object.
     *  @return 
     */
    function addItem($type,$name, $values = array())
    {
        if(!isset($this->itemConfig[$type])){
            echo "Invalid type {$type}, skipping item!<br>";
            return;
        }

        // Add missing values with the item-type defaults.
        $allValuesSet = TRUE;
        foreach($this->itemConfig[$type]['options'] as $oName => $oValue){
            if(!isset($values[$oName])){
                $values[$oName] = (isset($oValue['default']))?$oValue['default']:"";
            }
        }

        // Get the currently selected entry, its the parent for the newly 
        //  added one. 
        $current = &$this->currentItem; 

        // Create a FAKE base to be able to use the management lists
        //  which are currently ldap and thus dn based.
        $base = (isset($current['base']))? ",".$current['base'] : '';
        $base = "{$type}={$name}{$base}";


        if(isset($this->mappingBaseToID[$base])){
            echo "Cannot add entry, dn already in use! {$base}<br>";
            return(null);
        }
    
        // Get next free item slot.
        $id = ($this->lastItemID ++);
        $new = array(
                'base' => $base,
                'children' => array(),
                'id' => $id, 
                'type' => $type, 
                'name' => $name, 
                'values' => $values);

        // Append the entry to the list of all items.
        $this->allConfiguredItems[$id] = $new;

        // Create a child referenc, this creates some kind of entry tree.
        $current['children'][$id] = &$this->allConfiguredItems[$id];

        // Add entries to the list of base and id mappings
        //  this allows us to easily detect the base for an id and vice versa.
        $this->mappingBaseToID[$id] = $base;
        $this->mappingBaseToID[$base] = $id;
        return($id);
    }


    /*! \brief      Selects an item as active.
     *              All further add and remove actions will be performed
     *               on the obejcts children.
     *  @param  String  The 'id' of the item we want to select.
     *  @return 
     */
    function setCurrentItem($item)
    {
        if(!isset($this->allConfiguredItems[$item])){
            echo "Invalid item name {$name}! Skipping selection!";
            return;
        }

        // Set the new item info.
        $this->currentItem = &$this->allConfiguredItems[$item];
        $this->currentItemType = $this->currentItem['type'];
        $this->currentItemDescriptor = $this->itemConfig[$this->currentItem['type']];
    }


    /*! \brief      Removes a given item ID.
     *  @param  String  The 'id' of the item we want to remove.
     *  @return 
     */
    function removeItem($id, &$data = NULL)
    {
        if($data === NULL){
            $data = &$this->allConfiguredItems;
        }

        // Remove the item and its children
        if(isset($data[$id])){
            foreach($data[$id]['children'] as $cid => $item){
                $this->removeItem($cid, $data);
            }
            unset($data[$id]);
        }
    
        // Remove to current id from sub entries
        foreach($data as $key => $item){
            $this->removeItem($id, $data[$key]['children']);
        }
    }


    /*! \brief      Initiate item edit.
     *              An action send from the management list.
     *  @param  See management::editEntry 
     *  @return 
     */
    function editEntry($action="",$target=array(),$all=array(), 
            $altTabClass ="", $altTabType = "", $altAclCategory="")
    {
        $this->setCurrentItem($target[0]);
        $this->dialogObject = $this->TemplateEngine;
        $this->skipFooter = TRUE;
        $this->dialog = TRUE;

        // Update the template engine to use another type of item and 
        //  some other values.
        $this->TemplateEngine->setValues($this->currentItemType,$this->currentItem['values']);
    }


    /*! \brief      Save changes for the currently edited item.
     */
    function saveItemChanges()
    {
        // Save eventually changed values
        if($this->currentItem){

            // Check if everything is fine.
            $msgs = $this->TemplateEngine->save_object();
            $msgs = $this->TemplateEngine->check();
            if(count($msgs)){
                msg_dialog::displayChecks($msgs);
            }else{
                foreach($this->TemplateEngine->getWidgets() as $widget){
                    $this->currentItem['values'][$widget->getName()] = $widget->getValue();
                }
                $this->closeDialogs();
            }
        }
    }


    /*! \brief      React on open requests from the management list 
     */
    function openEntry($action="",$target=array(),$all=array(), 
            $altTabClass ="", $altTabType = "", $altAclCategory="")
    {
        $this->setSelectedListItemID($target[0]);
    }


    /*! \brief   Overridden render method of class mangement.
     *            this allows us to add a release selection box.
     */
    function renderList()
    {
        // In case of an error abort here.
        if($this->rpcError || $this->initFailed || $this->invalidInstallMethod){
            $smarty = get_smarty();
            $smarty->assign('initFailed', $this->initFailed);
            $smarty->assign('rpcError', $this->rpcError);
            $smarty->assign('invalidInstallMethod', $this->invalidInstallMethod);
            $smarty->assign('error', $this->errorMessage);
            return($smarty->fetch(get_template_path('failed.tpl', TRUE)));
        }

        // Collect item container list to be able to render the fake-base selector
        if(!$this->itemContainerSelector){
            $this->itemContainerSelector = new releaseSelector(
                    $this->getContainerList(), 
                    $this->base, 
                    $this->allConfiguredItems[$this->rootItemID]['base']);
        }else{
            $this->itemContainerSelector->setBases($this->getContainerList());
        }
        $this->itemContainerSelector->update(true);
        $this->itemContainerSelector->setBase($this->base);

        session::set('DEVICE_ITEMS', $this->allConfiguredItems);
        $this->rebuildListing();
        $filter = $this->getFilter();
        $headpage = $this->getHeadpage();

        $headpage->update();
        $smarty = get_smarty();
        $smarty->assign("RELEASE", $this->itemContainerSelector->render());
        $display = $headpage->render();
        return($this->getHeader().$display);
    }


    /*! \brief   Build up a list of items useable for the itemSelector.
     */
    function getContainerList($array = NULL)
    {
        $array = ($array == NULL)?$this->allConfiguredItems[$this->rootItemID]: $array;
        $ret[$array['base']] = $array['type'];
        if(count($array['children'])){
            foreach($array['children'] as $subItem){
                $ret = array_merge($ret, $this->getContainerList($subItem));
            }
        }
        return($ret);    
    }


    /*! \brief   Update the management class and tell her which 
     *            items are available for the itemSelector (baseSelector).
     */
    function rebuildListing()
    {
        // Build filter
        if (session::global_is_set(get_class($this)."_filter")){
            $filter= session::global_get(get_class($this)."_filter");
        } else {
            $filter = new filter(get_template_path("DeviceConfig-filter.xml", true));
            $filter->setObjectStorage($this->storagePoints);
        }
        $this->setFilter($filter);

        // Load service xml file and fill in placeholders
        $contents =file_get_contents(get_template_path("DeviceConfig-list.xml", true));

        // Build up device-list configuration   
        $types ="";
        $images = array();
        $images[] = 'images/lists/edit.png';
        $images[] = 'images/caps.png';
        $images[] = 'images/lists/trash.png';
        $images[] = 'images/filter.png';
        $images[] = 'images/find.png';
        $i = 0; 
        foreach($this->itemConfig as $type => $item){
            $desc = $item['description'];
            $img = $images[$i++];
            $types .= 
                "   <objectType>".
                "     <label>{$desc}</label>".
                "     <objectClass>{$type}</objectClass>".
                "     <category>Device</category>".
                "     <class>dummy</class>".
                "     <image>{$img}</image>".
                "   </objectType>";
        }
        $contents = preg_replace("/%TYPES%/", $types, $contents);


        $items = "";
        $i = 0;
        foreach($this->addableContainerItems as $item){
            $desc = $this->itemConfig[$item]['description'];
            $img = $images[$i++];
            $items .=
                "<action>".
                "    <name>add_{$item}</name>".
                "    <type>entry</type>".
                "    <image>{$img}</image>".
                "    <label>{$desc}</label>".
                "</action>";
        }

        if(!empty($items)){
            $items = 

                "<action>".
                " <type>sub</type>".
                " <image>images/lists/element.png[new]</image>".
                " <label>Create</label>".
                " {$items}".
                "</action>";
        }

        $contents = preg_replace("/%ITEMS%/", $items, $contents);

        $headpage = new listing($contents,TRUE);
        $headpage->setBase($this->base);
        $headpage->setFilter($filter);

        parent::__construct($this->config, $this->ui, "services", $headpage);

        // Register default actions
        $this->registerAction("new",        "newEntry");
        $this->registerAction("edit",       "openEntry"); // !! We forward 'edit' to 'open' to have a department like navigation.
        $this->registerAction("editEntry",  "editEntry");

        $this->registerAction("saveItemChanges", "saveItemChanges");
        $this->registerAction("cancelItemEdit", "closeDialogs");
        $this->registerAction("cancelItemAdd", "closeDialogs");
        $this->registerAction("saveItemAdd", "saveItemAdd");
        foreach($this->itemConfig as $name => $item){
            $this->registerAction("add_{$name}", "newEntry");
        }
    }


    /*! \brief  This method intiates the object creation.
     *
     *  @param  String  'action'  The name of the action which was the used as trigger.
     *  @param  Array   'target'  A list of object dns, which should be affected by this method.
     *  @param  Array   'all'     A combination of both 'action' and 'target'.
     */
    function newEntry($action="",$target=array(),$all=array(), 
            $altTabClass ="", $altTabType = "", $altAclCategory="")
    {
        $toAdd = preg_replace("/^add_/", "",$action);
        $itemToAdd = $this->itemConfig[$toAdd];

        $this->dialogObject = new AddItemDialog($this->config,$toAdd,$itemToAdd);
        $this->dialog = true;
    }

    
    /*! \brief   Saves newly created items and adds them as child to 
     *            the currently selected item.
     */
    function saveItemAdd()
    {
        if(!$this->dialogObject instanceOf AddItemDialog) return;

        $msgs = $this->dialogObject->save_object();
        $msgs = $this->dialogObject->check();
        if(count($msgs)){
            msg_dialog::displayChecks($msgs);
        }else{
            $itemName = $this->dialogObject->getName();
            $itemCfg = $this->dialogObject->getItemCfg();
            $itemType = $this->dialogObject->getItemType();

            $this->setCurrentItem($this->mappingBaseToID[$this->base]);
    
            $this->addItem($itemType, $itemName);
            $this->closeDialogs();

#           $this->setCurrentItem($itemName);
#           $this->dialogObject = $this->TemplateEngine;
#           $this->skipFooter = TRUE;
#           $this->dialog = TRUE;

        }
    }


    /*! \brief  Keep track of posted values, some may be interesting for us. 
     *          Tell the template engine to take care of posted values too.
     *  @param  String
     *  @return 
     */
    function save_object()
    {
        if(isset($_POST['retryInit'])){
            $this->init();
            return;
        }
            
        // Do nothing else in case of an error
        if($this->rpcError || $this->initFailed) return;

        // Add sub-module requested.
        if(isset($_POST['addSubModule']) && isset($_POST['subModule'])){
            $sub = get_post('subModule');
            if(in_array($sub, $this->currentItemDescriptor['container'])){

                // Check if this is a valid item
                if(!isset($this->itemConfig[$sub])) {
                    echo "Invalid item type '{$sub}'!";
                    $values = array();
                }else{
                    $values = $this->itemConfig[$sub]['options'];
                }
                $name = 'test'.rand(0,99999);
                $this->addItem($sub,$name,$values);
            }
        }

        // Get the selected item-id from the item list and populate it.
        if($this->itemContainerSelector){
            $this->itemContainerSelector->update();
            $id  = $this->mappingBaseToID[$this->itemContainerSelector->getBase()]; 
            $this->setSelectedListItemID($id);
        }
    }


    /* \brief   Updates the currenlty seleted item in the management list
     */   
    function setSelectedListItemID($id)
    {
        $this->base = $this->mappingBaseToID[$id];
        $type = $this->allConfiguredItems[$id]['type'];
        $this->addableContainerItems = $this->itemConfig[$type]['container'];
    }


    /*! \brief    Forward plugin acls
     */
    function set_acl_base($base)
    {
        $this->acl_base = $base;
    }


    /*! \brief    Forward plugin acls
     */
    function set_acl_category($category)
    {
        $this->acl_category = $category;
    }

    function save()
    {
        foreach($this->allConfiguredItems as $name => $item){
            foreach($item['values'] as $oName => $oValue){
                if(!is_array($oValue)) $oValue = array($oValue);
                foreach($oValue as $val){
                    echo "<br>{$name} -- <i>{$item['type']}</i>: <b>{$oName}</b>: {$val}";
                }
            }
        }
    }


    /*! \brief  Initiates the removal for the given entries
     *           and displays a confirmation dialog.
     *
     *  @param  String  'action'  The name of the action which was the used as trigger.
     *  @param  Array   'target'  A list of object dns, which should be affected by this method.
     *  @param  Array   'all'     A combination of both 'action' and 'target'.
     */
    protected function removeEntryRequested($action="",$target=array(),$all=array())
    {
        foreach($target as $id){
            $this->removeItem($id);
        }
    }

    
    // Inject user actions
    function detectPostActions()
    {
        if($this->rpcError || $this->initFailed) return(array('action' => ''));

        $action = management::detectPostActions();
        if(isset($_POST['saveItemEdit'])) $action['action'] = "saveItemChanges";
        if(isset($_POST['saveItemAdd'])) $action['action'] = "saveItemAdd";
        if(isset($_POST['cancelItemEdit'])) $action['action'] = "cancelItemEdit";
        if(isset($_POST['cancelItemAdd'])) $action['action'] = "cancelItemAdd";
        return($action);
    }

    
    function closeDialogs()
    {
        parent::closeDialogs();
        $this->dialog = false;
    }
   
 
    function check()
    {
        return(array());
    }

    function getRootItemId()
    {
        return($this->rootItemID);
    }


    public static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Config management"),
                    "plDescription" => _("Config management"),
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 0,
                    "plSection"     => array("administration"),
                    "plCategory"    => array(
                        "ConfigManagement" => array("description"  => _("Config management"),
                            "objectClass"  => "FAKE_OC_ConfigManagement")),
                    "plProvidedAcls"=> array()
                    ));
    }
}
?>
