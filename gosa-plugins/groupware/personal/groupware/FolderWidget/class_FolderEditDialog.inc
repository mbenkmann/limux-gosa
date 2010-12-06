<?php

class FolderEditDialog extends plugin
{
    private $folderItem = NULL;    
    private $permissions = array();

    function __construct($config, $folderItem, $isNew = FALSE)
    {
        $this->config = $config;
        $this->folderItem = $folderItem;
        $this->isNew = $isNew;

        // Prepare new item
        if($this->isNew){
            $entry = array();
            $entry['name'] = "";
            $entry['acls'] = array();
            $this->folderItem = $entry;
        }
    }

    function setPermissions($perms)
    {
        $this->permissions = $perms;
    }

    function execute()
    {
        $smarty = get_smarty();

        // Prepare folder name and path     
        $smarty->assign('folderName', preg_replace("/\{%cn\}/", "<i>Group name</i>", $this->folderItem['name']));
        if(isset($this->folderItem['path'])){
            $smarty->assign('folderPath', preg_replace("/\{%cn\}/", "<i>Group name</i>", $this->folderItem['path']));
        }else{
            $smarty->assign('folderPath', '');
        }
 
        $smarty->assign('folderItem', $this->folderItem);
        $smarty->assign('permissions', $this->permissions);
        $smarty->assign('permissionCnt', count($this->permissions));
        $smarty->assign('isNew', $this->isNew);
        return($smarty->fetch(get_template_path("FolderEditDialog.tpl", TRUE, dirname(__FILE__))));
    }


    function save_object()
    {
        // Get folder name 
        if($this->is_new){
            if(isset($_POST['folderName'])){
                $this->folderItem['name'] = get_post('folderName');
            }
        }

        // Get all posted value modifications
        foreach($this->folderItem['acls'] as $id => $aclEntry){

            // Name or ACL modified?
            if(isset($_POST["permission_{$id}_name"])) {
                $name = get_post("permission_{$id}_name");
                $this->folderItem['acls'][$id]['name'] = get_post("permission_{$id}_name");
            }
            if(isset($_POST["permission_{$id}_acl"])) {
                $this->folderItem['acls'][$id]['acl'] = get_post("permission_{$id}_acl");
            }

            // Check if entry has to be removed.
            if(isset($_POST["permission_{$id}_del"])){
                unset($this->folderItem['acls'][$id]);
            }
        }

        // New ACL entry to add?
        if(isset($_POST['permission_add'])){
            $this->folderItem['acls'][] = array('name' => '', 'acl' => 0, 'type' => 'user');
        }

        $this->folderItem['acls'] = array_values($this->folderItem['acls']);
    }

    
    function isNew()
    {
        return($this->isNew);
    }


    function check()
    {
        $message = array();

        // Check if we've duplicate ACL definitions for a user 
        $aclNames = array();
        foreach($this->folderItem['acls'] as $entry){
            if(!isset($aclNames[$entry['name']])){
                $aclNames[$entry['name']] = TRUE;
            }elseif(!empty($entry['name'])){
                $message[] = sprintf(_("Duplicate permission setting for user '%s'."), $entry['name']);
            }
        }

        return($message);
    }


    function save()
    {
        // Remove empty ACL defintions
        foreach($this->folderItem['acls'] as $id => $entry){
            if(empty($entry['name'])) unset($this->folderItem['acls'][$id]);
        }

        return($this->folderItem);
    }
}

?>
