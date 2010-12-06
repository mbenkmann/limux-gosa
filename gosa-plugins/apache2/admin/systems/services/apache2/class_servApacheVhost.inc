<?php

/*
 * This code is part of GOsa (http://www.gosa-project.org)
 * Copyright (C) 2003-2008 GONICUS GmbH
 *
 * ID: $$Id: class_gosaSupportDaemon.inc 10788 2008-05-06 11:15:57Z hickert $$
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

class servapache extends goService
{
    /* attribute list for save action */
    var $ignore_account   = FALSE;
    var $attributes       = array(); 
    var $objectclasses    = array("whatever");

    var $RecordTypes      = array();
    var $Vhosts           = array();
    var $dialog           = NULL;
    var $orig_dn          = "";

    var $conflicts        = array("servapache");

    var $initially_was_account;
    var $krb_service_prefix = "HTTP/";

    function servapache ($config, $dn= NULL, $parent= NULL)
    {
        plugin::plugin ($config, $dn, $parent);

        $this->orig_dn = $dn;

        /* Get all vhost Informations
         */
        $this->Vhosts = apacheUtils::getVhostEntries($config,$dn);

        /* If there is at least one entry in this -> types, we have apache vhosts enabled 
         */
        if(count($this->Vhosts) == 0){
            $this->is_account = false;
        }else{
            $this->is_account = true;
        }
        $this->initially_was_account = $this->is_account;

        /* Set service name */
        $this->DisplayName = _("Apache service");

        // Prepare lists
        $this->vHostList = new sortableListing();
        $this->vHostList->setDeleteable(true);
        $this->vHostList->setInstantDelete(false);
        $this->vHostList->setEditable(true);
        $this->vHostList->setWidth("100%");
        $this->vHostList->setHeight("100px");
        $this->vHostList->setColspecs(array('*','40px'));
        $this->vHostList->setHeader(array(_("Name")));
        $this->vHostList->setDefaultSortColumn(0);

    }


    function execute()
    {
        /* Call parent execute 
         */
        plugin::execute();

        /* Fill templating stuff 
         */
        $smarty= get_smarty();
        $display= "";

        /* Edited or Added vhost 
         */
        if((isset($_POST['SaveVhostChanges'])) && is_object($this->dialog)){
            $this->dialog->save_object();

            /* Check for errors  
             */
            if(count($this->dialog->check())){
                msg_dialog::displayChecks($this->dialog->check());
            }else{
                /* add new/edited vhost 
                 */
                $ret = $this->dialog->save();
                if(!$this->dialog->isNew){
                    unset($this->Vhosts[$this->dialog->OldApacheServerName]);
                }
                $this->Vhosts[$ret['apacheServerName']] = $ret;
                $this->dialog = NULL;
            }
        }

        /* Cancel vhost edit / new 
         */
        if(isset($_POST['CancelVhostChanges'])){
            $this->dialog = NULL;
        }

        /* Add empty new vhost 
         */
        if(isset($_POST['AddVhost']) && $this->acl_is_writeable("VirtualHosts")){
            $this->dialog = new servapacheVhostEdit($this->config,$this->dn);
            $this->dialog->set_acl_category('server');
        }

        /* Check for edit vhost request 
         */
        $this->vHostList->save_object();
        $action = $this->vHostList->getAction();
        if($action['action'] == 'delete'){
            $id = $this->vHostList->getKey($action['targets'][0]);
            $this->RemoveVhost($id);
        }
        if($action['action'] == 'edit'){
            $id = $this->vHostList->getKey($action['targets'][0]);
            $this->dialog= new servapacheVhostEdit($this->config,$this->dn,$this->Vhosts[$id]);
            $this->dialog->set_acl_category('server');
        }

        /* Show dialog 
         */
        if($this->dialog!= NULL){
            $this->dialog->save_object();
            $this->dialog->parent = $this;
            return($this->dialog->execute());
        }

        /* Assign ACL to smarty 
         */    
        $info = $this->plInfo();
        foreach($info['plProvidedAcls'] as $name => $desc){
            $smarty->assign($name."ACL",$this->getacl($name));
        }

        /* Create Listbox with existing Vhosts 
         */
        $this->vHostList->setAcl($this->getacl('VirtualHosts'));
        $data = $lData = array();
        foreach($this->Vhosts as $vhost => $values){
            $data[$vhost]=$values;
            $lData[$vhost]=array('data'=>array($vhost));
        }    
        $this->vHostList->setListData($data,$lData);
        $this->vHostList->update();

        /* Display template 
         */
        $smarty->assign("VhostList",$this->vHostList->render());
        $display.= $smarty->fetch(get_template_path('servApacheVhost.tpl', TRUE , dirname(__FILE__)));
        return($display);
    }


    /* Delete specified vhost
     */  
    function RemoveVhost($id)
    {
        unset($this->Vhosts[$id]);
        return(true);
    }


    /* This function returns all used Vhostnames 
     */
    function getUsedServerNames()
    {
        $ret = array();
        $ldap = $this->config->get_ldap_link();
        $ldap->cd($this->config->current['BASE']);
        $ldap->search("(&(objectClass=apacheConfig)(apacheServerName=*))",array("apacheServerName"));
        while($attr = $ldap->fetch()){
            $ret[$attr['apacheServerName'][0]][] = $attr['dn'];
        }
        return($ret);
    }


    /* Remove apache service
     */
    function remove_from_parent()
    {
        foreach($this->Vhosts as $key => $vhost){
            $this->RemoveVhost($key);
        }
        $this->save();
    }


    /* Save to LDAP */
    function save()
    {
        $ldap = $this->config->get_ldap_link();
        $ldap->cd($this->config->current['BASE']);  

        /* Get differences 
         */

        $old_dn =  $this->orig_dn;
        if($old_dn == "new"){
            $old_dn = $this->dn;
        }

        $tmp = apacheUtils::getVhostEntriesDiff($this->config,$this->Vhosts,$old_dn);

        /* Updated vhost entries if reverser or forward name has changed  
         * Must be done before moving entries, else the given dn is invalid
         */
        if(isset($tmp['vhostUpdates'])){
            foreach($tmp['vhostUpdates'] as $dn => $attrs){
                $ldap->cd($dn);
                $ldap->modify($attrs);

                if (!$ldap->success()){
                    msg_dialog::display(_("LDAP error"), msgPool::ldaperror($ldap->get_error(), $this->dn, LDAP_MOD, get_class()));
                }
            }
        }

        /* Delete apache vhost
         */
        foreach($tmp['del'] as $dn => $del){
            $ldap->cd($dn);
            $ldap->rmdir_recursive($dn);
            if (!$ldap->success()){
                msg_dialog::display(_("LDAP error"), msgPool::ldaperror($ldap->get_error(), $this->dn, LDAP_DEL, get_class()));
            }
        }

        /* move follwoing entries
         */
        foreach($tmp['move'] as $src => $dst){
            $this->recursive_move($src,$dst);
        }

        /* Add || Update new apache entries
         */
        foreach($tmp['add'] as $dn => $attrs){
            $ldap->cd($dn);
            $ldap->cat($dn, array('dn'));
            if(count($ldap->fetch())){
                $ldap->cd($dn);
                $ldap->modify ($attrs);
            }else{
                $ldap->cd($dn);
                $ldap->add($attrs);
            }
            if (!$ldap->success()){
                msg_dialog::display(_("LDAP error"), msgPool::ldaperror($ldap->get_error(), $this->dn, LDAP_ADD, get_class()));
            }
        }
    }


    /*! \brief Returns the required information about this plugin for the 
      service list 
      @return Array  With information for the service plugin.
     */
    function getListEntry()
    {
        $fields = goService::getListEntry();
        $fields['Message']    = _("Apache service");
#$fields['AllowEdit']  = TRUE;
#$fields['AllowRemove']= TRUE;
        return($fields);
    }


    /*  \brief Return plugin information used by the ACL handling.
        @return Array ACL infos.
     */  
    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Apache service"),
                    "plDescription" => _("Apache virtual host service")." ("._("Services").")",
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 88,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("server"),

                    "plProvidedAcls"=> array(
                        "VirtualHosts"        => _("Virtual hosts"),
                        "apacheServerName"    => _("Server name"),
                        "apacheDocumentRoot"  => _("Document root"),
                        "apacheServerAlias"   => _("Server alias"),
                        "apacheServerAdmin"   => _("Server admin"),
                        "apacheScriptAlias"   => _("Script alias")
                        )
                    ));
    }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
