<?php

class FolderWidgetListing extends management
{
   
    private $folderSelector = NULL; 
    private $folderList = array();

    private $rootBase = '';
    private $parent;

    public $pathTitle = "Folder editor";
    public $plHeadline = "Folder editor";

    function __construct($config,$ui, $rootBase, $parent)
    {
        $this->config = $config;
        $this->ui = $ui;
        $this->storagePoints = array("");
        $this->rootBase = $rootBase;
        $this->parent = &$parent;

        // Build filter
        if (FALSE && session::global_is_set(get_class($this)."_filter")){
            $filter= session::global_get(get_class($this)."_filter");
        } else {
            $filter = new filter(get_template_path("FolderWidget-filter.xml", true, dirname(__FILE__)));
            $filter->setObjectStorage($this->storagePoints);
        }
        $this->setFilter($filter);

        // Build headpage
        $headpage = new listing(get_template_path("FolderWidget-list.xml", true, dirname(__FILE__)));
        $headpage->setFilter($filter);
        $headpage->setBase($this->rootBase);

        // Create folderSelector using dummy entries, those will be replaced 
        //  once setFolderList is called.
        $this->folderSelector = new releaseSelector(array($this->rootBase => '/'),$this->rootBase,$this->rootBase);
        parent::__construct($config, $ui, "Folder", $headpage);

        // Register default actions
        $this->registerAction("new",        "newEntry");
        $this->registerAction("edit",       "openEntry"); // !! We forward 'edit' to 'open' to have a department like navigation.
        $this->registerAction("editEntry",  "editEntry");
    }

    function openEntry($action, $targets)
    {
        $this->setBase($targets[0]);
    }

    function editEntry($action="",$target=array(),$all=array(), $altTabClass ="", $altTabType = "", $altAclCategory="")
    {
        $this->parent->editByBase($target[0]);
    }

    function newEntry($action="",$target=array(),$all=array(), $altTabClass ="", $altTabType = "", $altAclCategory="")
    {
        $this->parent->addFolder();
    }

    protected function removeEntryRequested($action="",$target=array(),$all=array())
    {   
        foreach($target as $tar){
            $this->parent->removeByBase($tar);
            if($tar == $this->base) $this->setBase($this->rootBase);
        }
    }
    
    function setFolderList($folderList)
    {
        $this->folderList = $folderList;
    }


    function getBase()
    {
        return($this->base);
    }


    function setBase($base)
    {
        $this->base = $base;
        $this->folderSelector->setBase($this->base);
        $headpage = $this->getHeadpage();
        $headpage->setBase($this->base);
    }

    
    function setFolderRoot($base)
    {
        $this->rootBase = $base;
        $this->folderSelector->setRootBase($base);
    }
   
 
    /*! \brief   Overridden render method of class mangement.
     *            this allows us to add a release selection box.
     */
    function renderList()
    {
        $this->folderSelector->setBases($this->getSimpleFolderList($this->folderList['recursive']['/']['children']));
        $this->folderSelector->update(true);

        $headpage = $this->getHeadpage();
        $headpage->update();
        $smarty = get_smarty();
        $smarty->assign("RELEASE", $this->folderSelector->render());
        $display = $headpage->render();
        return($this->getHeader().$display);
    }


    function getSimpleFolderList($arr)
    {
        $ret = array($this->rootBase => '/');
        foreach($arr as $path => $entry){
            if($entry['status'] != 'removed'){
                $ret[$entry['base']] = $entry['name'];
                if(isset($entry['children'])){
                    $ret = array_merge($ret, $this->getSimpleFolderList($entry['children']));
                }
            }
        }
        return($ret);
    }

    
    function save_object()
    {
        $this->folderSelector->update();
        $this->setBase($this->folderSelector->getBase());
    }
}
?>
