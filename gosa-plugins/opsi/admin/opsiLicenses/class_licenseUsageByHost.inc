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


class licenseUsageByHost extends plugin 
{

    var $cn = "";
    var $licenseUses = array();
    var $reservedLicenses = array();
    var $init_successfull = FALSE;

    var $availableLicenses = array();

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
            $this->cn = $this->orig_cn = preg_replace("/^opsi:=([^,]*),.*$/","\\1",$dn);
        }

        // Extract pool name out of the fake dn.
        $this->init();

        // Prepare lists
        $this->reservedList = new sortableListing();
        $this->reservedList->setDeleteable(true);
        $this->reservedList->setEditable(false);
        $this->reservedList->setWidth("100%");
        $this->reservedList->setHeight("220px");
        $this->reservedList->setColspecs(array('200px','*'));
        $this->reservedList->setHeader(array(_("Pool"),_("License ID")));
        $this->reservedList->setDefaultSortColumn(1);
        $this->reservedList->setAcl('rwcdm'); // All ACLs, we filter on our own here.

        $this->usedList = new sortableListing();
        $this->usedList->setDeleteable(false);
        $this->usedList->setEditable(false);
        $this->usedList->setWidth("100%");
        $this->usedList->setHeight("220px");
        $this->usedList->setColspecs(array('200px','*'));
        $this->usedList->setHeader(array(_("Key"),_("Pool"),_("License ID")));
        $this->usedList->setDefaultSortColumn(1);
        $this->usedList->setAcl('rwcdm'); // All ACLs, we filter on our own here.

        // Create statistic table entry
        stats::log('plugin', $class = get_class($this), $category = array($this->acl_category),  $action = 'open',
                $amount = 1, $duration = (microtime(TRUE) - $this->initTime));
    }


    function init()
    {
        $this->licenseUses = array();
        $this->reservedLicenses = array();
        $this->init_reservedLicenses = array();
        if(!$this->initially_was_account){
            $this->init_successfull = TRUE;
        }else{

            // Get license usage  
            $res = $this->si->getLicenseUsage($this->cn);
            if($this->si->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
                $this->init_successfull = FALSE;
                return;
            }
            $this->licenseUses = $res;

            // Get reservations
            $res = $this->si->getReservedLicensesForHost($this->cn);
            if($this->si->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
                $this->init_successfull = FALSE;
                return;
            }
            foreach($res as $pool){
                $l = $pool['softwareLicenseId'][0];
                $this->reservedLicenses[$l] = $l;
            }
            $this->init_reservedLicenses = $this->reservedLicenses;

            // Get list of license pools 
            $res = $this->si->listLicenses();
            if($this->si->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
                $this->init_successfull = FALSE;
                return;
            }
            $this->availableLicenses = array();
            foreach($res as $license){
                if(!isset($license['licensePoolId'])) $license['licensePoolId'][0] = '-';
                $this->availableLicenses[$license['softwareLicenseId'][0]] = $license;
            }

            $this->init_successfull = TRUE;
        }
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

        // Create usage list
        $data = array();
        $this->usedList->setAcl($this->getacl('hostId'));
        foreach($this->licenseUses as $license){
            $data[] = array('data' => array(
                        $license['licenseKey'][0],
                        $license['licensePoolId'][0],
                        $license['softwareLicenseId'][0]));
        } 
        $this->usedList->setListData($data,$data);
        $this->usedList->update();

        // Create reserved list
        $data = $lData = array();
        $this->reservedList->setAcl($this->getacl('boundToHost'));
        foreach($this->reservedLicenses as $key => $license){
            $l = $this->availableLicenses[$key];
            $data[$key] = $key;
            $lData[$key] = array('data' => array(
                        $l['softwareLicenseId'][0],
                        $l['licensePoolId'][0]));
        } 
        $this->reservedList->setListData($data,$lData);
        $this->reservedList->update();

        $smarty = get_smarty();

        // Assign ACls 
        $plInfo = $this->plInfo();
        foreach($plInfo['plProvidedAcls'] as $name => $desc){
            $smarty->assign("{$name}ACL",$this->getacl($name));
        }
        foreach($this->attributes as $attr){
            $smarty->assign($attr,set_post($this->$attr));
        }

        // build up a available licenses list. 
        $licenses = array();
        foreach($this->availableLicenses as $key => $license){
            if(!in_array($license['softwareLicenseId'][0], $this->reservedLicenses)){
                $licenses[$key] = $license['softwareLicenseId'][0]." [".$license['licensePoolId'][0]."]";
            }
        }

        $smarty->assign("licenseUses", $this->usedList->render());
        $smarty->assign("licenseReserved", $this->reservedList->render());
        $smarty->assign("init_successfull", $this->init_successfull);
        $smarty->assign("availableLicenses", $licenses);
        $smarty->assign("initially_was_account", $this->initially_was_account);
        return($smarty->fetch(get_template_path('licenseUsageByHost.tpl',TRUE,dirname(__FILE__))));
    }


    /* Save HTML inputs
     */
    function save_object()
    {
        if(isset($_POST['opsiLicenseUsagePosted'])){
            plugin::save_object();  

            if(preg_match("/w/i",$this->getacl('boundToHost'))){

                $this->reservedList->save_object();
                $action = $this->reservedList->getAction();
                if($action['action'] == "delete") {
                    $id = $this->reservedList->getKey($action['targets'][0]);
                    if(isset($this->reservedLicenses[$id])) {
                        unset($this->reservedLicenses[$id]);
                    }
                }

                // Check if we've to add reservations
                if(isset($_POST['availableLicense']) && isset($_POST['addReservation'])){
                    $id = get_post('availableLicense');
                    if(isset($this->availableLicenses[$id]) && !in_array($this->availableLicenses[$id],$this->reservedLicenses)){
                        $this->reservedLicenses[$id] =  $this->availableLicenses[$id]['softwareLicenseId'][0];
                    }
                }
            }
        }
    }  


    /* Check user input and return a list of 'invalid input' messages.
     */
    function check()
    {
        $message = plugin::check();
        return($message);
    }


    function save()
    {
        $del = array_diff($this->init_reservedLicenses, $this->reservedLicenses);
        $add = array_diff($this->reservedLicenses, $this->init_reservedLicenses);

        foreach($del as $license){
            $this->si->removeLicenseReservationFromHost($license, $this->cn);
            if($this->si->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
            }
        }

        foreach($add as $license){
            $this->si->reserverLicenseForHost($license, $this->cn);
            if($this->si->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
            }
        }
    }


    function remove_from_parent(){ }


    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Usage by host"),
                    "plDescription" => _("License usage by host"),
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 13,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("opsi"),
                    "plProvidedAcls"=> array(
                        "hostId" => _("Used by"),
                        "boundToHost" => _("License reservation"))
                    ));
    }
}


// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
