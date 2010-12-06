<?php
/*
 * This code is part of GOsa (http://www.gosa-project.org)
 * Copyright (C) 2003-2008 GONICUS GmbH
 *
 * ID: $$Id: class_gosaSupportDaemon.inc 10788 2008-05-06 11:15:57Z $$
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


class servapacheVhostEdit extends plugin
{
    /* attribute list for save action */
    var $ignore_account= TRUE;

    var $attributes	= array("apacheServerName","apacheDocumentRoot","apacheServerAlias",
            "apacheServerAdmin","apacheScriptAlias");

    var $objectclasses  = array("whatever");

    var $apacheServerName		= "";
    var $apacheDocumentRoot	= "";
    var $apacheServerAdmin	= "";
    var $apacheSuexecUid		= "";
    var $apacheSuexecGid		= "";

    var $apacheServerAlias                = array();
    var $apacheScriptAlias                = array();

    var $OldApacheServerName              = ""; // To detect changes made with this edit
    var $InitialApacheServerName          = "";

    var $Records                  = array();

    var $dialog                   = false;

    var $isNew                    = true;
    var $cn;
    var $VhostObject               = array();


    function servapacheVhostEdit ($config, $dn= NULL,$attrs = array())
    {
        plugin::plugin ($config, $dn);
        if(!count($attrs)){
            $this->OldApacheServerName        = "";
            $this->isNew              = true;
            $this->InitialApacheServerName    = "";//$attrs['InitialApacheServerName'];
        }else{
            $this->VhostObject         = $attrs;
            $this->OldApacheServerName        = $attrs['apacheServerName'];
            $this->InitialApacheServerName    = $attrs['InitialApacheServerName'];
            $this->isNew                  = false;
            foreach($this->attributes as $value){
                if(isset($attrs[$value])){
                    $this->$value = $attrs[$value];
                }
            }

            if(!isset($attrs['apacheServerAlias'])) $this->apacheServerAlias  = array();
            if(!isset($attrs['apacheScriptAlias'])) $this->apacheScriptAlias  = array();
        }


        // Prepare lists
        $this->serverList = new sortableListing(array(),array(), TRUE);
        $this->serverList->setDeleteable(true);
        $this->serverList->setInstantDelete(false);
        $this->serverList->setEditable(false);
        $this->serverList->setWidth("100%");
        $this->serverList->setHeight("100px");
        $this->serverList->setHeader(array(_("Alias"),_("Directory")));
        $this->serverList->setColspecs(array('*','*','20px'));
        $this->serverList->setDefaultSortColumn(0);

        $this->scriptList = new sortableListing(array(),array(), TRUE);
        $this->scriptList->setDeleteable(true);
        $this->scriptList->setInstantDelete(false);
        $this->scriptList->setEditable(false);
        $this->scriptList->setWidth("100%");
        $this->scriptList->setHeight("100px");
        $this->scriptList->setColspecs(array('*','*','20px'));
        $this->scriptList->setHeader(array(_("Alias"),_("Directory")));
        $this->scriptList->setDefaultSortColumn(0);
    }

    function execute()
    {
        /* Call parent execute */
        plugin::execute();

    
        return "<h3>Plugin is deactivated<h3>";

        /* Fill templating stuff */
        $smarty= get_smarty();

        $display= "";

        $this->serverList->save_object();
        $action = $this->serverList->getAction();
        if($action['action'] == 'delete'){
            $key = $this->serverList->getKey($action['targets'][0]);
            unset($this->apacheServerAlias[$key]);
        }
        if($action['action'] == 'reorder'){
            $this->apacheServerAlias = $this->serverList->getMaintainedData(); 
        }

        $this->scriptList->save_object();
        $action = $this->scriptList->getAction();
        if($action['action'] == 'delete'){
            $key = $this->scriptList->getKey($action['targets'][0]);
            unset($this->apacheScriptAlias[$key]);
        }
        if($action['action'] == 'reorder'){
            $this->apacheScriptAlias = $this->scriptList->getMaintainedData(); 
        }

        /* Add a new "Server Alias" 
         */
        if((isset($_POST['AddSARecord'])) && (!empty($_POST['StrSAAlias'])) && (!empty($_POST['StrSADir']))){
            $this->apacheServerAlias[] = trim(get_post('StrSAAlias')." ".get_post('StrSADir'));      
        }

        /* Add a new "Script Alias" 
         */
        if((isset($_POST['AddSCRecord'])) && (!empty($_POST['StrSCAlias'])) && (!empty($_POST['StrSCDir']))){
            $this->apacheScriptAlias[] = trim(get_post('StrSCAlias')." ".get_post('StrSCDir'));      
        }

        /* Fill in values */
        foreach($this->attributes as $name){
            $smarty->assign($name, set_post($this->$name));
        }

        /* Set apacheServerNames without server suffix */
        $smarty->assign("apacheServerName", set_post($this->apacheServerName));
        $smarty->assign("NotNew", false);


        $this->serverList->setAcl($this->parent->getacl('apacheServerAlias'));
        $data = $lData = array();
        foreach($this->apacheServerAlias as $key => $rec){
            $data[$key] = $rec;
            $lData[$key]= array('data' => preg_split("/ /",$rec));
        }
        $this->serverList->setListData($data,$lData);
        $this->serverList->update();
        $smarty->assign("apacheServerAlias", $this->serverList->render());

        $this->scriptList->setAcl($this->parent->getacl('apacheScriptAlias'));
        $data = $lData = array();
        foreach($this->apacheScriptAlias as $key => $rec){
            $data[$key] = $rec;
            $lData[$key]= array('data' => preg_split("/ /",$rec));
        }
        $this->scriptList->setListData($data,$lData);
        $this->scriptList->update();
        $smarty->assign("apacheScriptAlias", $this->scriptList->render());

        /* Display template */
        $display.= $smarty->fetch(get_template_path('servApacheVhostEdit.tpl', TRUE, dirname(__FILE__)));
        return($display);
    }


    function remove_from_parent()
    {
    }


    /* Save data to object */
    function save_object()
    {
        plugin::save_object();
    }


    /* Check supplied data */
    function check()
    {
        /* Call common method to give check the hook */
        $message= plugin::check();

        /* Check if apacheServerName is already in use */
        $usedVhosts = $this->getUsedServerNames();
        if(($this->isNew == true)||($this->apacheServerName  != $this->InitialApacheServerName)){
            if((isset($usedVhosts[$this->apacheServerName]))&&($this->apacheServerName  != $this->InitialApacheServerName)){
                $message[] = msgPool::duplicated(_("Name"));
            }
        }

        /* Check given virtual server name 
         */
        if(!tests::is_domain($this->apacheServerName) || empty($this->apacheServerName)){
            $message[] = msgPool::invalid(_("Virtual host name"),$this->apacheServerName,"",
                    _("Only lowercase strings are allowed as Virtual Host Name.")."&nbsp;"._("e.g. example.com"));
        }

        /* check if there is a valid document root given 
         */
        if(!tests::is_path($this->apacheDocumentRoot) || empty($this->apacheDocumentRoot)){
            $message[] = msgPool::invalid(_("Document root"),$this->apacheDocumentRoot);
        }

        /* Check given mail address 
         */ 
        if(!tests::is_email($this->apacheServerAdmin) || empty($this->apacheServerAdmin)){
            $message[] = msgPool::invalid(_("Administrator mail address"),$this->apacheServerAdmin);
        }

        /* Check server aliases 
         */    
        foreach($this->apacheServerAlias as $key => $line){
            $apacheServerAlias_ar=explode(" ",$line);
            $url=$apacheServerAlias_ar[1];
            if(!tests::is_path($url)){
                $message[] = msgPool::invalid(_("Server alias"));
            }
        }

        /* Check script aliases 
         */    
        foreach($this->apacheScriptAlias as $key => $line){
            $apacheScriptAlias_ar=explode(" ",$line);
            $url=$apacheScriptAlias_ar[1];
            if(!tests::is_path($url)){
                $message[] = msgPool::invalid(_("Script alias"));
            }
        }

        return ($message);
    }


    /* This funtion returns all used apacheServerNames */
    function getUsedServerNames()
    {
        $ret = array();
        $ldap = $this->config->get_ldap_link();
        $ldap->cd($this->config->current['BASE']);
        $ldap->search("(&(objectClass=apacheConfig)(apacheServerName=*))",array("apacheServerName"));
        while($attr = $ldap->fetch()){
            $ret[$attr['apacheServerName'][0]]="";
        }
        return($ret);
    }


    /* Save to LDAP */
    function save()
    {
        $ret =array();
        foreach($this->attributes as $name){
            $ret[$name] = $this->$name;
        }

        /* Create SA records 
         */
        foreach($this->apacheServerAlias as $key => $rec){
            $rec['value']= $key." ".$rec['value'];
            $this->Records [] = $rec;
        }

        /* Create SC records 
         */
        foreach($this->apacheScriptAlias as $key => $rec){
            $rec['value']= $key." ".$rec['value'];
            $this->Records [] = $rec;
        }

        $ret['RECORDS'] = $this->Records; 
        $ret['InitialApacheServerName']   =  $this->InitialApacheServerName;
        return($ret);
    }

    function acl_is_writeable($attribute,$skip_write = FALSE)
    { 
        return($this->parent->acl_is_writeable($attribute,$skip_write));
    }


}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
