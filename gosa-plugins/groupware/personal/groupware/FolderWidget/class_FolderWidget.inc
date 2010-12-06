<?php

// Not yet finished need to be reviewed once the folder handling is working.


/*! \brief  A groupware folder widget which allow to navigate through a 
 *           given list of user or shared folders.
 *          Folders can be added or removed and ACLs assigned and updated.
 *
 *
 *  @param  Config  The GOsa configuration object
 *  @param  Array   The folder list to handle, e.g.
 *                  
 *                  $folders['shared/support']['status'] = ''; 
 *                  $folders['shared/support']['name'] = 'support'; 
 *                  $folders['shared/support']['acls'] = array();
 *
 *                  $folders['shared/support/2010']['status'] = 'added'; 
 *                  $folders['shared/support/2010']['name'] = '2010'; 
 *                  $folders['shared/support/2010']['acls'] = array('anne' => 1024, 'klaus' => '210');
 *                 
 *                  // 'status' Represents the folder status, maybe it wasn't touched then status is '';
 *                  //          If the folder was added, then the status is 'added'. Status 'removed' 
 *                  //          handles the folder as removed.
 *
 *                  // 'name'   Represents the entry name, this should match the name in the path.
 *
 *                  // 'acls'   Represents ACLs set for the folder.
 *                              Syntax  ['userNmae' => ACL] // See $this->setPermissions();
 *                              
 *
 */
class FolderWidget extends plugin
{
    public $pathTitle = "Folder editor";
    public $plHeadline = "Folder editor";

    private $pathSeparator = "/";
    private $rootName = "/";
    private $fakeCnAttr = "cn=";
    private $rootBase = "";

    public $config;
    private $FolderWidgetListing = NULL;
    private $folderList;

    private $baseEntryMap = array();
    private $permissions = array();

    protected $displayButtons = TRUE;

    
    /*! \brief  Constructs the widget and initializes all required sub-widgets.
     */
    function __construct(&$config, $folderList, $accountPrefix)
    {
        $this->config = $config;

        // The rootBase is a dn-like representation of the root path
        // e.g.  rootPath is '/'  then is the rootBase 'cn=/'
        // These are just internally used variables and should not have an 
        //  effect on the results.
        $this->rootBase = $this->fakeCnAttr.$this->rootName;

        // Remove trailing / from accountPrefix.
        $this->accountPrefix = (empty($accountPrefix)) ? "" : rtrim($accountPrefix,$this->pathSeparator);
        $this->rootBase = $this->pathToDn($accountPrefix);

        // Update the folder list, initially.
        $this->setFolderList($folderList);
        $this->FolderWidgetListing = new FolderWidgetListing($this->config, get_userinfo(), $this->rootBase, $this);
        $this->FolderWidgetListing->setBase($this->rootBase);
        $this->FolderWidgetListing->setFolderRoot($this->rootBase);
    }


    function showButtons($bool = TRUE)
    {
        $this->displayButtons = $bool;
    }
    

    /*! \brief  Updates the list of all selectable ACLs
     *  @param  Array   The list of acls to use 
     *                  Syntax is [ 'ACLname'] => 'diplayName', 0=>'None',1=>'Read',...]
     */
    function setPermissions($perms)
    {
        $this->permissions = $perms;
    }


    /*! \brief  The be able to use the LDAP-Based listint widget we've to create
     *           some kind of fake dns/bases, this method converts a path to such a dn/base.
     *
     *          e.g. 'shared/technik/test' will return 'cn=test,cn=technik,cn=test,cn=/'; 
     *
     * @param   String  The path to convert
     * @return  String  The generated base.
     */
    function pathToDn($path)
    {
        $dn ="cn=/";
        $list = preg_split("/".preg_quote($this->pathSeparator,'/')."/", $path,0,PREG_SPLIT_NO_EMPTY);
        foreach($list as $entry){
            $dn ="cn={$entry},{$dn}"; 
        }
        return($dn);
    }

    
    /*! \brief  A function to detect the folderType for a given foldePath
     *          This folderType tells the listing widget hat images has to be used.
     *          See: FolderWidget-list.xml <objectType>
     */
    function getFolderType($item)
    {
        if(preg_match("/^user\//", $item['path'])) return('userFolder');
        if(preg_match("/^shared\//", $item['path'])) return('sharedFolder');
    }


    /*! \brief  Tell the widget which folders should be displayed.
     */
    function setFolderList($arr)
    {
        $this->buildRecursiveList($arr, $this->pathSeparator);

        // Ensure that we've at least the base folder set
        if(!isset($this->folderList['linear'][$this->accountPrefix])){
            $this->addPath($this->accountPrefix, preg_replace("/^.*\//","", $this->accountPrefix));

            trigger_error(sprintf("You reuquire at least a folder named '%s'! This time it was added automatically.", 
                $this->accountPrefix));
        }
    }


    /* \brief   Builds a hierarchical structure which can then be 
     *           used by the FolderWidgetListing.
     *
     * @param   Array   The folder list 
     * @return  Array   A hierarchical representation of the input array.
     */
    function buildRecursiveList($arr, $separator)
    {
        $data = array();
        $ret = array();

        // Add root Entry
        $data['/'] = array(); 
        $data['/']['name'] = '/';
        $data['/']['status'] = '';
        $data['/']['acls'] = array();
        $data['/']['path'] = '/';
        $data['/']['type'] = 'manual';
        $data['/']['base'] = 'cn=/';
        $data['/']['parentPath'] = "";
        $data['/']['children'] = array();

        $this->baseEntryMap = array();
        $this->baseEntryMap['cn=/'] = '/';
        $this->folderList = array();
        $this->folderList['linear']['/'] = &$data['/'];
        $this->folderList['recursive']['/'] = &$data['/'];

        foreach($arr as $path => $data){
            $name = $data['name'];
            $acls = $data['acls'];
            $status = $data['status'];
            $this->addPath($path, $name, $acls, $status);
        }  
    }
    

    /*! \brief  Adds a new folder entry to the used folderList.
     */
    function addPath($path, $name, $acls = array(), $status = "")
    {
        $base = &$this->folderList['linear']['/']['children'];
        $last = NULL;

        // Only add entries that match the accountPrefix 
        //   e.g.  "/user/admin/"
        $path = rtrim($path,$this->pathSeparator);
        if(!preg_match("/^".preg_quote($this->accountPrefix,'/')."/", $path)){
            // Skipping not mathing folders.
        }else{
            $current = &$base;
            $pathSteps = preg_split("/".preg_quote($this->pathSeparator,'/')."/", $path);
            $entryPath = "";
            $parentPath = "";
            $entryBase = "";

            // Walk through path entries and add missing entries to our folder list
            foreach($pathSteps as $pathName){

                // The parent path is the last path.
                $parentPath = $entryPath;

                // Calculate entry path by combining last path and current pathName.
                $entryPath = (empty($entryPath)) ? $pathName : "{$entryPath}{$this->pathSeparator}{$pathName}";
                $entryBase =  $this->pathToDn($entryPath);

                // To not add sub-folders which are not nescessary to be displayed
                // e.g. We have shared/technik/test
                // Then do not all sub-folders for shared, it is not selectable in the listing later.
                if(!preg_match("/".preg_quote($this->accountPrefix,'/')."/", $entryPath)){
                    continue;
                }

                // Add eventually missing subFolders, to enable the navigation 
                //  into these folders via the management lists.
                if(!isset($current[$entryPath])){
                    $current[$entryPath] = array();
                    $current[$entryPath]['children'] = array();
                    $current[$entryPath]['name'] = $pathName;
                    $current[$entryPath]['path'] = $entryPath;
                    $current[$entryPath]['status'] = '';
                    $current[$entryPath]['acls'] = array();
                    $current[$entryPath]['type'] = 'manual';
                    $current[$entryPath]['base'] = $entryBase;
                    $current[$entryPath]['parentPath'] = $parentPath;
                    $current[$entryPath]['folderType'] = $this->getFolderType($current[$entryPath]);

                    $this->folderList['linear'][$entryPath] = &$current[$entryPath];
                    $this->baseEntryMap[$entryBase] = $entryPath;
                }

                // Remeber the last added sub-folder and update the current folder position.
                $last = &$current[$entryPath];
                $current = &$current[$entryPath]['children'];
            }

            // Now that we've created all subfolders we can set the folder
            //  paramters of the folder we originally wanted to add.
            $last['acls'] = $acls;
            $last['name'] = $name;
            $last['type'] = 'real';
            $last['status'] = $status;
        }
    }
  
 
    /*! \brief  Initiates editing of a folder, a sub dialog will be shown on next
     *          render() call. this sub-dialog allows to edit the ACLs for the folder.
     */ 
    function editByBase($base)
    {
        $item = NULL;
        foreach($this->folderList['linear'] as $item){
            if($item['base'] == $base){
                break;
            }
        }
        if($item){
            $this->currentlyEditedItem = $item;
            $this->dialog = new FolderEditDialog($this->config, $item);
            $this->dialog->setPermissions($this->permissions);
        }
    }


    function dialogOpened()
    {
        return($this->dialog != FALSE);
    }


    /*! \brief  Removes a given folder, internally it is just marked with status => 'removed';
     */
    function removeByBase($base)
    {
        // Do not allow to remove the root base.
        // This would mean the account itself was removed.
        if($base == $this->rootBase) return;
   
 
        $this->_recursiveRemoveByBase($this->folderList['recursive'], $base);
        $this->_recursiveRemoveByBase($this->folderList['linear'], $base);
        $this->FolderWidgetListing->setFolderList($this->folderList);
    }


    /*! \brief  Shows a sub-dialog on next render call, which allows to specify 
     *           a folder name and acls, which will then be added to the list of folders.
     */
    function addFolder()
    {
        $this->currentlyEditedItem = NULL;
        $this->dialog = new FolderEditDialog($this->config, array(), TRUE);
        $this->dialog->setPermissions($this->permissions);
    }


    /*!\brief   Recursivly removes a folder by its base.
     */
    private function _recursiveRemoveByBase(&$arr, $base)
    {
        $ret = array();
        foreach($arr as $path => $item){
            if($item['base'] == $base){

                // remove all children too
                if(isset($item['children']) && count($item['children'])){
                    foreach($item['children'] as $child){
                        $this->_recursiveRemoveByBase($this->folderList['recursive'], $child['base']);
                    }
                }
                
                // Remove the the entry instead of setting its status if it was just added.
                if($arr[$path]['status'] == "added"){
                    unset($arr[$path]) ;
                }else{
                    $arr[$path]['status'] = 'removed';
                }
            }elseif(isset($item['children']) && count($item['children'])){
                $ret = array_merge($ret, $this->_recursiveRemoveByBase($arr[$path]['children'], $base));
            }
        }
        return($ret);
        
    }


    /*!\brief   Recursivly collects all folders, this is called from save().
     */
    private function _saveRecursive($arr)
    {
        $ret = array();
        foreach($arr as $path => $item){
            if($item['type'] == 'real'){ 
                $ret[$path] = array(
                        'name' => $item['name'],
                        'status' => $item['status'],
                        'acls' => $item['acls'],
                        );
            }
            if(isset($item['children']) && count($item['children'])){
                $ret = array_merge($ret, $this->_saveRecursive($item['children']));
            }
        }
        return($ret);
    }

    
    /*! \brief  Returns the folder list with all changes made during 
     *           the session. 
     */
    function save()
    {
        return($this->_saveRecursive($this->folderList['recursive']));
    }


    /*! \brief  Generates the HTML output for the current widget.
     *          Handles add/edit dialogs too.
     */
    function execute()
    {
        // Generate the listing output 
        session::set('FolderWidgetListing::folderList', $this->folderList);
        $this->FolderWidgetListing->setFolderList($this->folderList);

        $smarty = get_smarty();
        $smarty->assign("SHOW_BUTTONS", $this->displayButtons);
        $str = $this->FolderWidgetListing->execute();

        if($this->dialog){
            $this->dialog->save_object();
            return($this->dialog->execute());
        }

        // Return listing HTML
        return($str);
    }


    /*! \brief  Check GET and POST for interesting informations.
     *          Like edit request from the folder listing
     */
    function save_object()
    {
        $this->FolderWidgetListing->save_object();

        // Handle dialogs 
        if(isset($_POST['FolderEditDialog_ok']) && is_object($this->dialog)){
    
            $this->dialog->save_object();
            $msgs = $this->dialog->check();
            if(count($msgs)){
                msg_dialog::displayChecks($msgs);
            }else{
                if($this->dialog->isNew()){
                    // Detect the current entry
                    $base = $this->FolderWidgetListing->getBase();
                    if(isset($this->baseEntryMap[$base])){
                        $entry = $this->folderList['linear'][$this->baseEntryMap[$base]];
                        $path = $entry['path'];
                        $newItem = $this->dialog->save();
                        $name = $newItem['name'];
                        $newPath = rtrim($path,$this->pathSeparator).$this->pathSeparator.$name;

                        $newItem['status'] = 'added';
                        $newItem['type'] = 'real';
                        $newItem['children'] = array();
                        $newItem['path'] = $newPath;
                        $newItem['parentPath'] = $entry['path'];
                        $newItem['folderType'] = $this->getFolderType($newItem);
                        $newItem['base'] = $this->pathToDn($newPath);
    
                        $this->folderList['linear'][$path]['children'][$newPath] = $newItem;
                        $this->folderList['linear'][$newPath] = &$this->folderList['linear'][$path]['children'][$newPath];

                        $this->baseEntryMap[$newItem['base']] = $newPath;
                    }
                }else{
                    $newItem = $this->dialog->save();
                    $path = $this->currentlyEditedItem['path'];
                    $this->folderList['linear'][$path]['acls'] = $newItem['acls']; 
                }
                $this->dialog = NULL;
            }
        }
        if(isset($_POST['FolderEditDialog_cancel'])){
            $this->dialog = NULL;
        }
    }
}

?>
