<?php
/*
   This code is part of GOsa (https://gosa.gonicus.de)
   Copyright (C) 2008 Fabian Hickert

   This program is free software; you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation; either version 2 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program; if not, write to the Free Software
   Foundation, Inc., 59 Temple Place, Suite 330, Boston, MA  02111-1307  USA
 */


/*! brief   The kerberos service class 
 */
class goKrbServer extends goService{

    var $cli_summary      = "This pluign is used within the ServerService Pluign \nand indicates that this server supports NTP service.";
    var $cli_description  = "Some longer text\nfor help";
    var $cli_parameters   = array("eins" => "Eins ist toll", "zwei" => "Zwei ist noch besser");

    /* This plugin only writes its objectClass */
    var $objectclasses    = array("goKrbServer");
    var $attributes       = array("goKrbRealm");
    var $StatusFlag       = "goKrbServerStatus";

    /* This class can't be assigned twice so it conflicts with itsself */
    var $conflicts        = array("goKrbServer");

    var $DisplayName      = "";
    var $dn               = NULL;
    var $goKrbServerStatus= "";
    var $cn               = "";
    var $goKrbRealm       = "";
    var $view_logged      = FALSE;

    var $policies         = array();
    var $id               = -1;
    var $macAddress       = "" ;


    /*! \brief Initializes the kerberos service 
      @param  Object  GOsa configuration object.
      @param  String  The object dn we are currently editing.
      @param  Object  The parent object.
     */ 
    public function goKrbServer(&$config,$dn,$parent)
    {
        goService::goService($config,$dn);
        $this->DisplayName = _("Kerberos service");
        $this->parent = $parent;

        /* Detect macAddress of this device 
         */
        $this->macAddress = "";
        if(isset($this->parent->parent->netConfigDNS->macAddress)){
            $this->macAddress = &$this->parent->parent->netConfigDNS->macAddress;
        }

        /* Get configured policies 
         */ 
        if(!empty($this->macAddress) && $this->is_account){
            $o = new gosaSupportDaemon();
            $tmp = $o->krb5_list_policies($this->macAddress);
            if($o->is_error()){
                msg_dialog::display(_("Service infrastructure"),msgPool::siError($o->get_error()),ERROR_DIALOG);
            }else{
                $this->policies = array();
                foreach($tmp as $policy){
                    $this->policies[] = array("NAME" => $policy,"STATUS" => "LOADED","DATA" => array());
                }
            }
        }

        // Prepare lists
        $this->principleList = new sortableListing();
        $this->principleList->setDeleteable(true);
        $this->principleList->setInstantDelete(false);
        $this->principleList->setEditable(true);
        $this->principleList->setWidth("100%");
        $this->principleList->setHeight("100px");
        $this->principleList->setHeader(array(_("Name")));
        $this->principleList->setColspecs(array('*','40px'));
        $this->principleList->setDefaultSortColumn(0);

    }


    /*! \brief  Create HTML output
      @return String HMTL output.
     */
    public function execute()
    { 
        $smarty = get_smarty(); 
        $smarty->assign("MIT_KRB", class_available("passwordMethodMIT"));

        if(!$this->view_logged){
            $this->view_logged = TRUE;
            new log("view","server/".get_class($this),$this->dn);
        }

        /*******  
          Display sub dialogs
         *******/

        /*  CANCEL Policy dialog 
         */
        if(isset($_POST['cancel_policy'])){
            $this->dialog = NULL;
        }

        /*  SAVE Policy dialog 
         */
        if($this->dialog instanceof krb5_policy && isset($_POST['save_policy'])){
            $this->dialog->save_object();
            $msgs = $this->dialog->check();
            if(count($msgs)){
                msg_dialog::displayChecks($msgs);
            }else{
                $this->AddPolicy($this->id,$this->dialog->save());
                $this->dialog = NULL;
            }
        }

        /* DISPLAY policy dialog
         */
        if($this->dialog instanceof krb5_policy){
            $this->dialog->save_object();
            return($this->dialog->execute());
        } 

        /*******
          Create HTML output for this plugin
         *******/  

        $tmp = $this->plinfo();
        foreach($tmp['plProvidedAcls'] as $name => $translation){
            $smarty->assign($name."ACL",$this->getacl($name));
        }

        foreach($this->attributes as $attr){
            $smarty->assign($attr, set_post($this->$attr));
        }

        $this->principleList->setAcl($this->getAcl('goKrbPolicy'));
        $data = $lData = array();
        foreach($this->policies as $key => $policy){
            if($policy['STATUS'] == "REMOVED") continue;
            $data[$key] = $policy;
            $lData[$key]= array('data' => array($policy['NAME']));
        }
        $this->principleList->setListData($data,$lData);
        $this->principleList->update();
        $smarty->assign("list",$this->principleList->render());
        return($smarty->fetch(get_template_path("goKrbServer.tpl",TRUE,dirname(__FILE__))));
    }


    /* \brief  Return serice informations, which will be shown in the service overview.
       @return  Array  Some service information.
     */
    public function getListEntry()
    {
        $fields               = goService::getListEntry();
        $fields['Message']    = _("Kerberos service (kadmin access informations)");
#$fields['AllowEdit']  = true;
        return($fields);
    }


    /*! \brief Checks if all given values are valid 
      @return Array   An array containing all error messages.
     */
    public function check()
    { 
        $message = plugin::check();
        if (empty($this->goKrbRealm)){
            $message[]= msgPool::required(_("Realm"));
        }
        if(count($this->policies) && (empty($this->macAddress) || !tests::is_mac($this->macAddress))){
            $message[] = _("Cannot apply policy changes! The defined Kerberos server has no MAC address.");
        }
        return($message);
    }


    /*! \brief Adds or updated a given policy.
      @param  Integer ID  The policy to update, or -1 if it is a new one.
      @param  Array  The policy settings.
     */
    public function AddPolicy($id,$policy)
    {
        if($id != -1 && $this->policies[$id]['NAME'] == $policy['NAME']){

            /* Policy was edited 
             */
            if($this->policies[$id]['STATUS'] == "LOADED"){
                $policy['STATUS'] = "EDITED";
            }else{
                $policy['STATUS'] = $this->policies[$id]['STATUS'];
            }
            $this->policies[$this->id] = $policy;

        }elseif($id != -1 && $this->policies[$id]['NAME'] != $policy['NAME']){

            /* Policy was renamed, remove old and add new policy  
             */
            $this->RemovePolicy($id);
            $this->AddPolicy(-1,$policy);
        }else{

            /* Policy was added 
             */
            $name = $policy['NAME'];

            /* Check if there is already a policy with this name 
               which was removed before
             */
            foreach($this->policies as $pid => $entry){
                if($entry['NAME'] == $name && $entry['STATUS'] == "REMOVED"){
                    $id = $pid;
                    break;
                }
            }

            /* Update existing policy or create new one
             */ 
            if(isset($this->policies[$id])){
                $policy['STATUS'] = "EDITED";
                $this->policies[$id] = $policy;
            }else{
                $policy['STATUS'] = "ADDED";
                $this->policies[] = $policy;
            }
        }
    }


    /*! \brief  Returns all used policy names.
      @return Array A list of used policy names.
     */
    public function getPolicyNames()
    {
        $ret = array();
        foreach($this->policies as $policy){
            if($policy['STATUS'] == "REMOVED") continue;
            $ret[] = $policy['NAME'];
        }
        return($ret);
    }


    /*! \brief  Marks a policy as remvoed 
      @param  Integer the Id of the policy to remove 
     */ 
    public function RemovePolicy($id)
    {
        /* Load policy information, if not done before 
         */
        if(!isset($this->policies[$id]))  return;
        if($this->policies[$id]['STATUS'] == "LOADED" && empty($this->policies[$id]['DATA'])){
            $o = new gosaSupportDaemon();
            $this->policies[$id]['DATA'] = $o->krb5_get_policy($this->macAddress,$this->policies[$id]['NAME']);
        }
        if($this->policies[$id]['DATA']['POLICY_REFCNT']){
            msg_dialog::display(_("Remove"),msgPool::stillInUse(_("Policy")),ERROR_DIALOG) ;
        }else{
            if($this->policies[$id]['STATUS'] == "ADDED"){
                unset($this->policies[$id]);
            }else{
                $this->policies[$id]['STATUS'] = "REMOVED";
            }
        } 
    }  


    /*! \brief  Save POSTed html variables
     */
    public function save_object()
    {
        if(isset($_POST['goKrbServerPosted'])){
            plugin::save_object();

            /* Add new Policy requested 
             */
            if(isset($_POST['policy_add']) && class_available("krb5_policy")){
                $this->dialog = new krb5_policy($this->config,array(),$this);
                $this->id = -1;
            }


            $this->principleList->save_object();
            $action = $this->principleList->getAction();
            if($action['action'] == 'delete'){
                $id = $this->principleList->getKey($action['targets'][0]);
                $this->RemovePolicy($id);
            }
            if($action['action'] == 'edit'){
                $id = $this->principleList->getKey($action['targets'][0]);
                if(isset($this->policies[$id])){

                    /* Load policy information, if not done before 
                     */ 
                    if($this->policies[$id]['STATUS'] == "LOADED" && empty($this->policies[$id]['DATA'])){
                        $o = new gosaSupportDaemon();
                        $this->policies[$id]['DATA'] = $o->krb5_get_policy($this->macAddress,$this->policies[$id]['NAME']);
                    }

                    /* Open dialog */
                    $this->id = $id;
                    $this->dialog = new krb5_policy($this->config,$this->policies[$id], $this);
                }
            }
        }
    } 


    /*! \brief Save changes to ldap
     */
    public function save()
    {
        goService::save();

        /* Send policy changes back to the si daemon 
         */
        $actions = array("del" => array(),"add_edit" => array());
        foreach($this->policies as $policy){

            /* Unset not used vars */
            if(isset($policy['DATA']['MASK']))          unset($policy['DATA']['MASK']);
            if(isset($policy['DATA']['POLICY_REFCNT'])) unset($policy['DATA']['POLICY_REFCNT']);

            switch($policy['STATUS']){
                case "REMOVED" : $actions['del'] [] = $policy;break; 
                case "ADDED"   : 
                    case "EDITED"  : $actions['add_edit'] [] = $policy;break; 
            }
        }

        $o = new gosaSupportDaemon();
        $policies = $o->krb5_list_policies($this->macAddress);

        /* Send remove policy event  
         */
        foreach($actions['del'] as $policy){
            if(!in_array($policy['NAME'],$policies)) continue;
            if(!$o->krb5_del_policy($this->macAddress,$policy['NAME'])){
                msg_dialog::display(_("Service infrastructure"),msgPool::siError($o->get_error()),ERROR_DIALOG);
            }
        }

        /* Send add new/edit policy event
         */
        foreach($actions['add_edit'] as $policy){
            if(in_array($policy['NAME'],$policies)){
                if(!$o->krb5_set_policy($this->macAddress,$policy['NAME'],$policy['DATA'])){
                    msg_dialog::display(_("Service infrastructure"),msgPool::siError($o->get_error()),ERROR_DIALOG);
                }
            }else{
                if(!$o->krb5_add_policy($this->macAddress,$policy['NAME'],$policy['DATA'])){
                    msg_dialog::display(_("Service infrastructure"),msgPool::siError($o->get_error()),ERROR_DIALOG);
                }
            }
        }
    }


    /*! \brief Return plugin informations for acl handling 
      @return Array   ACL infos.
     */
    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Kerberos"),
                    "plDescription" => _("Kerberos access information")." ("._("Services").")",
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 95,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("server"),
                    "plRequirements"=> array(
                        'ldapSchema' => array('goKrbServer' => '>=2.7'),
                        'onFailureDisablePlugin' => array(get_class())
                        ),

                    "plProvidedAcls"=> array(
                        "start"         => _("Start"),
                        "stop"          => _("Stop"),
                        "restart"       => _("Restart"),
                        "goKrbPolicy"   => _("Policies"),
                        "goKrbRealm"    => _("Realm")) 
                    ));
    }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
