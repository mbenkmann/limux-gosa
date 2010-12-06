<?php
/*
 * This code is part of GOsa (http://www.gosa-project.org)
 * Copyright (C) 2003-2008 GONICUS GmbH
 *
 * ID: $$Id: class_opsiLicenses.inc 13520 2009-03-09 14:54:13Z hickert $$
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


class licenseGeneric extends plugin 
{

    // License attributes
    var $si = NULL;
    var $data = array();
    var $cn = "";
    var $orig_cn = "";
    var $description = "";  
    var $partner = "";

    // Date attributes 
    var $conclusionDate = "";
    var $expirationDate = "";
    var $notificationDate = "";

    // License model related attribues
    var $licenseModel = "";
    var $licenseKey = array();
    var $orig_licenseModel = "";
    var $licensePoolId = "";
    var $boundToHost= array(); // Reserved for Host.   
    var $usedByHost = array(); // Used by Host.   

    var $maximumInstallations = 0;
    var $opsiHosts; 

    var $attributes = array(
            "cn","description","partner","conclusionDate","expirationDate",
            "notificationDate","licenseModel","licenseKey","maximumInstallations",
            "licensePoolId", "usedByHost","boundToHost");

    function __construct(&$config, $dn, $license, $hosts = array())
    {
        $this->initTime = microtime(TRUE);
        $this->config = $config;
        $this->data = $license;
        $this->dn = $dn;
        $this->si = new opsiLicenceHandler($this->config);
        $this->opsiHosts = $hosts;

        // Detect account state.
        if(count($this->data) == 0){
            $this->initially_was_account = FALSE;
        }else{
            $this->initially_was_account = TRUE;
        }
        $this->is_account = TRUE;

        // Extract pool name out of the fake dn.
        $this->init();

        // Create statistic table entry
        stats::log('plugin', $class = get_class($this), $category = array($this->acl_category),  $action = 'open',
                $amount = 1, $duration = (microtime(TRUE) - $this->initTime));
    }


    function init()
    {
        // Extract license information out of the license object ($this->data)
        $this->boundToHost = array('0'=>"");
        $this->usedByHost = array('0'=>"");
        $this->licenseKey = array('0'=>"");
        if($this->initially_was_account){
            foreach($this->attributes as $attr){
                $this->$attr = $this->data[$attr];
            }

            // Fix dates 
            foreach(array("notificationDate","expirationDate","conclusionDate") as $date) {
                if(!empty($this->$date)){
                    $this->$date = date("d.m.Y",strtotime($this->$date));
                }
            }
        }

        $this->orig_cn = $this->cn;
        $this->orig_licenseModel = $this->licenseModel;
        $this->init_successfull = TRUE;
        return;    
    }


    function execute()
    {
        plugin::execute();
        // Handle initialization failures.
        if(isset($_POST['retry_init'])) $this->init();
        if(!$this->init_successfull){
            $smarty = get_smarty();
            $smarty->assign("init_successfull", $this->init_successfull);
            return($smarty->fetch(get_template_path('licenseGeneric.tpl',TRUE,dirname(__FILE__))));
        }

        $smarty = get_smarty();

        // Assign attributes and its ACls 
        $plInfo = $this->plInfo();
        foreach($plInfo['plProvidedAcls'] as $name => $desc){
            $smarty->assign("{$name}ACL",$this->getacl($name));
        }
        foreach($this->attributes as $attr){
            $smarty->assign($attr,set_post($this->$attr));
        }

        // Assign list of available license models
        $smarty->assign("licenseModels",array(
                    "RETAIL" => _("Retail"),
                    "OEM"=>_("OEM"),
                    "VOLUME" => _("Volume")));


        $smarty->assign("init_successfull", $this->init_successfull);
        $smarty->assign("initially_was_account", $this->initially_was_account);
        $smarty->assign("hosts", $this->getHosts());

        $ui = get_userinfo();
        $acl_base = $this->dn;
        if($acl_base == "new"){
            $acl_base = $this->config->current['BASE'];
        }
        $smarty->assign("licenseACL", $ui->get_permissions($acl_base,"opsi/licensePoolGeneric","licenses"));
        $smarty->assign("writeable", preg_match("/w/",$ui->get_permissions($acl_base,"opsi/licensePoolGeneric","licenses")));
        $smarty->assign("notUsedHosts", array_diff($this->getHosts(), $this->usedByHost));
        $smarty->assign("boundToHost", set_post($this->boundToHost[0]));
        $smarty->assign("licenseKey", set_post($this->licenseKey[0]));

        // w3c fix, do not show empty options.
        $tmp = $this->usedByHost;
        if(isset($tmp[0]) && empty($tmp[0])) unset($tmp[0]);
        $smarty->assign('usedByHost', $tmp);

        foreach(array("notificationDate","expirationDate","conclusionDate") as $date) {
            $smarty->assign($date."Writeable", $this->acl_is_writeable($date));
        }

        return($smarty->fetch(get_template_path('licenseGeneric.tpl',TRUE,dirname(__FILE__))));
    }


    function getHosts()
    {
        $ret = array();
        foreach($this->opsiHosts as $host){
            $cn = $host['NAME'][0]['VALUE'];
            $ret[$cn] = $cn;
        }
        return($ret);
    }



    /* Save HTML inputs
     */
    function save_object()
    {

        if(isset($_POST['opsiLicensesPosted'])){
            plugin::save_object();

            if(isset($_POST['addLicenseUsage']) && isset($_POST['selectedHostToAdd'])){
                $host = get_post('selectedHostToAdd');
                if(!empty($host) && 
                        in_array($host,$this->getHosts()) && 
                        !in_array($host, $this->usedByHost)){
                    $this->usedByHost[] = $host;
                }
            }

            if(isset($_POST['removeLicenseUsage']) && isset($_POST['selectedUsedHosts'])){
                $todel = get_post('selectedUsedHosts');
                foreach($todel as $host){
                    if(isset($this->usedByHost[$host])){
                        unset($this->usedByHost[$host]);
                    }
                }
            }

            // Force licenseKey to be of type array.
            if(!is_array($this->licenseKey)){
                $this->licenseKey = array($this->licenseKey);
            }

            // BoundToHost maybe multiple too, later.
            if(!is_array($this->boundToHost)){
                $this->boundToHost = array($this->boundToHost);
            }    

            if($this->initially_was_account){
                $this->cn = $this->orig_cn;
                $this->licenseModel = $this->orig_licenseModel;
            }
        }
    }  


    /* Check user input and return a list of 'invalid input' messages.
     */
    function check()
    {
        $message = plugin::check();

        // Very simple date input checks
        if(!empty($this->expirationDate) && 
                !preg_match("/^[0-9]{2}\.[0-9]{2}\.[0-9]{4}$/",$this->expirationDate)){
            $message[] = msgPool::invalid(_("Expiration date"),$this->expirationDate,"","23.02.2009");
        }
        if(!empty($this->conclusionDate) && 
                !preg_match("/^[0-9]{2}\.[0-9]{2}\.[0-9]{4}$/",$this->conclusionDate)){
            $message[] = msgPool::invalid(_("Expiration date"),$this->conclusionDate,"","23.02.2009");
        }
        if(!empty($this->notificationDate) && 
                !preg_match("/^[0-9]{2}\.[0-9]{2}\.[0-9]{4}$/",$this->notificationDate)){
            $message[] = msgPool::invalid(_("Expiration date"),$this->notificationDate,"","23.02.2009");
        }

        if(empty($this->cn)){
            $message[] = msgPool::required(_("Name"));
        }

        if(empty($this->licenseKey[0])){
            $message[] = msgPool::required(_("License key"));
        }

        return($message);
    }


    /* Removes the object from the opsi database
     */ 
    function remove_from_parent() {}


    /* Saves object modifications
     */  
    function save()
    {
        $data = array();
        foreach($this->attributes as $target){
            $data[$target] = $this->$target;
        }

        // Return opsi like dates.
        foreach(array("notificationDate","expirationDate","conclusionDate") as $date) {
            if(!empty($this->$date)){
                $data[$date] = date("Y-m-d",strtotime($this->$date));
            }
        }

        return($data);
    }

    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Generic"),
                    "plDescription" => _("License generic"),
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 8,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("opsi"),
                    "plProvidedAcls"=> array(
                        "cn"                  => _("Name"),
                        "partner"             => _("Partner"),
                        "licenseKey"          => _("License key"),
                        "licenseModel"        => _("License model"),
                        "licensePoolId"       => _("License pool id"),
                        "maximumInstallations"=> _("Maximum installations"),
                        "boundToHost"         => _("Reserved for host"),
                        "usedByHost"          => _("Usage"),
                        "notificationDate"    => _("Notification date"),
                        "conclusionDate"      => _("Conclusion date"),
                        "expirationDate"      => _("Expiration date"),
                        "description"         => _("Description"))
                        ));
    }
}


// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
