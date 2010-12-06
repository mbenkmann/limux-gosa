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


class licenseUsage extends plugin 
{

    var $cn = "";
    var $licenseUses = array();
    var $init_successfull = FALSE;


    function __construct(&$config,$dn)
    {
        $this->initTime = microtime(TRUE);
        $this->config = $config;
        $this->dn = $this->orig_dn = $dn;
        $this->si = new opsiLicenceHandler($this->config);

        $this->is_account=TRUE;
        if($this->dn == "new"){
            $this->initially_was_account = FALSE;
        }else{
            $this->initially_was_account = TRUE;
            $this->cn = $this->orig_cn = preg_replace("/^opsi:cn=([^,]*),.*$/","\\1",$dn);
        }

        // Extract pool name out of the fake dn.
        $this->init();

        // Prepare lists
        $this->usageList = new sortableListing();
        $this->usageList->setDeleteable(false);
        $this->usageList->setEditable(false);
        $this->usageList->setWidth("100%");
        $this->usageList->setHeight("300px");
        $this->usageList->setHeader(array(_("Host"),_("Key"),_("Pool"), _("License")));
        $this->usageList->setDefaultSortColumn(1);

        // Create statistic table entry
        stats::log('plugin', $class = get_class($this), $category = array($this->acl_category),  $action = 'open',
                $amount = 1, $duration = (microtime(TRUE) - $this->initTime));

    }


    function init()
    {
        $this->licenseUses = array();
        if(!$this->initially_was_account){
            $this->init_successfull = TRUE;
        }else{

            $res = $this->si->getLicenseUsage("", $this->cn);
            if($this->si->is_error()){
                $this->init_successfull = FALSE;
                msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
                return;
            }

            $this->licenseUses = $res;
            $this->init_successfull = TRUE;
        }
    }


    function execute()
    {
        plugin::execute();

        $ui = get_userinfo();
        $readable = $ui->get_permissions($this->config->current['BASE'], "opsi/licenseUsage","");
        $this->usageList->setAcl($readable);

        // Handle initialization failures.
        if(isset($_POST['retry_init'])) $this->init();
        if(!$this->init_successfull){
            $smarty = get_smarty();
            $smarty->assign("init_successfull", $this->init_successfull);
            return($smarty->fetch(get_template_path('licenseGeneric.tpl',TRUE,dirname(__FILE__))));
        }

        // Create usage list
        $data = array();
        if($readable) {
            foreach($this->licenseUses as $license){
                $data[] = array('data'=>array(
                            $license['hostId'][0],
                            $license['licenseKey'][0],
                            $license['licensePoolId'][0],
                            $license['softwareLicenseId'][0]));
            } 
        } 
        $this->usageList->setListData($data,$data);
        $smarty = get_smarty();

        // Assign ACls 
        $plInfo = $this->plInfo();
        foreach($plInfo['plProvidedAcls'] as $name => $desc){
            $smarty->assign("{$name}ACL",$this->getacl($name));
        }
        foreach($this->attributes as $attr){
            $smarty->assign($attr,set_post($this->$attr));
        }

        $smarty->assign("licenseUses", $this->usageList->render());
        $smarty->assign("init_successfull", $this->init_successfull);
        $smarty->assign("initially_was_account", $this->initially_was_account);
        return($smarty->fetch(get_template_path('licenseUsage.tpl',TRUE,dirname(__FILE__))));
    }


    /* Save HTML inputs
     */
    function save_object()
    {
        if(isset($_POST['opsiLicenseUsagePosted'])){
            plugin::save_object();  
        }
    }  


    /* Check user input and return a list of 'invalid input' messages.
     */
    function check()
    {
        $message = plugin::check();
        return($message);
    }


    /* Saves object modifications
     */  
    function save() { }
    function remove_from_parent(){ return; }


    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("License usage"),
                    "plDescription" => _("License usage"),
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 11,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("opsi"),
                    "plProvidedAcls"=> array()));
    }
}


// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
