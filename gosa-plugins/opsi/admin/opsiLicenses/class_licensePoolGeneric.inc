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


class licensePoolGeneric extends plugin 
{

    var $cn = "";
    var $orig_cn = "";
    var $description = "";
    var $orig_dn    = "";

    var $data       = array();
    var $orig_data  = array();
    var $productIds = array();
    var $softwareIds= array();
    var $licenses   = array();
    var $orig_licenses   = array();

    var $availableProductIds = array();
    var $attributes =array("cn","description");
    var $si = NULL;

    var $opsiHosts = array();

    var $licenseMap = array( 
            "BOUNDTOHOST"=> "boundToHost",
            "HOSTIDS"=> "usedByHost",
            "LICENSEPOOLIDS"=> "licensePoolId",
            "LICENSETYPE"=> "licenseModel",
            "LICENSEKEYS"=> "licenseKey",
            "MAXINSTALLATIONS"=> "maximumInstallations",
            "EXPIRATIONDATE"=> "expirationDate",
            "SOFTWARELICENSEID"=> "cn"
            );

    var $licenseContractMap = array( 
            "CONCLUSIONDATE"=> "conclusionDate",
            "NOTIFICATIONDATE"=> "notificationDate",
            "NOTES"=> "description",
            "PARTNER"=> "partner"
            );

    function __construct(&$config,$dn)
    {
        $this->initTime = microtime(TRUE);
        $this->config = $config;
        $this->dn = $this->orig_dn = $dn;

        // initialize gosa-si handle for opsi licenses
        $this->si = new opsiLicenceHandler($this->config);

        // Detect if this is a new or existings license
        $this->is_account=TRUE;
        if($this->dn == "new"){
            $this->initially_was_account = FALSE;
        }else{
            $this->initially_was_account = TRUE;

            // Extract pool name out of the fake dn.
            $this->cn = $this->orig_cn = preg_replace("/^opsi:cn=([^,]*),.*$/","\\1",$dn);
        }

        $this->init();

        // Prepare lists
        $this->licenseList = new sortableListing();
        $this->licenseList->setDeleteable(true);
        $this->licenseList->setEditable(true);
        $this->licenseList->setWidth("100%");
        $this->licenseList->setHeight("120px");
        $this->licenseList->setColspecs(array('200px','*'));
        $this->licenseList->setHeader(array(_("Section"),_("Description")));
        $this->licenseList->setDefaultSortColumn(1);

        // Create statistic table entry
        stats::log('plugin', $class = get_class($this), $category = array($this->acl_category),  $action = 'open',
                $amount = 1, $duration = (microtime(TRUE) - $this->initTime));
    }


    function init()
    {
        // Load local Boot Products 
        $res = $this->si->get_local_products();
        $this->availableProductIds=array();
        if($this->si->is_error()){
            msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
            $this->init_successfull = FALSE;
            return(FALSE);
        }
        $this->availableProductIds=array_keys($res);

        // Load opsi hosts
        $res = $this->si->list_clients();
        $this->opsiHosts=array();
        if($this->si->is_error()){
            msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
            $this->init_successfull = FALSE;
            return(FALSE);
        }
        $this->opsiHosts=$res;

        // Load Pool
        if(!$this->initially_was_account){
            $this->init_successfull = TRUE;
        }else{
            $res = $this->si->getPool($this->cn);
            if($this->si->is_error()){    
                $this->init_successfull = FALSE;
                msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
                return(FALSE);
            }else{
                $this->data = $this->orig_data = $res;
                $this->description = $this->data['description'][0];

                // Load software IDs 
                $this->softwareIds = array();
                if(isset($this->data['softwareId'])){
                    for($i = 0; $i < $this->data['softwareId']['count']; $i++){
                        $this->softwareIds[] = $this->data['softwareId'][$i];
                    }
                }

                // Load product IDs 
                $this->productIds = array();
                if(isset($this->data['productId'])){
                    for($i = 0; $i < $this->data['productId']['count']; $i++){
                        $this->productIds[] = $this->data['productId'][$i];
                    }
                }

                // Load Licences 
                $this->licenses = array();
                if(isset($this->data['licenses'])){
                    for($i =0; $i< $this->data['licenses']['count']; $i++){
                        $license = $this->data['licenses'][$i];

                        // Do not parse invalid licenses
                        if(!is_array($license) || !isset($license['LICENSEPOOLIDS'])){
                            continue;
                        }

                        // Prepare Software License Key(s)
                        $upper = strtoupper($license['LICENSEPOOLIDS'][0]['VALUE']);
                        $license['LICENSEKEYS'] = $license['LICENSEKEYS'][0][$upper];

                        // Map license attributes to a useable format
                        $entry = array();
                        foreach($this->licenseMap as $source => $target){
                            $entry[$target] = "";
                            if(isset($license[$source])){
                                if(count($license[$source]) >= 2){
                                    $entry[$target] = array();
                                    foreach($license[$source] as $sub){
                                        $entry[$target][] = $sub['VALUE'];
                                    }
                                }elseif(isset($license[$source][0]['VALUE'])){
                                    $entry[$target] = $license[$source][0]['VALUE'];
                                }
                            }
                        }

                        // Extract contract data
                        $lData= $license['LICENSECONTRACTDATA'][0];
                        foreach($this->licenseContractMap as $source => $target){
                            if(isset($lData[$source])){
                                if(count($lData[$source]) >= 2){
                                    $entry[$target] = array();
                                    foreach($lData[$source] as $sub){
                                        $entry[$target][] = $sub['VALUE'];
                                    }
                                }elseif(isset($lData[$source][0]['VALUE'])){
                                    $entry[$target] = $lData[$source][0]['VALUE'];
                                }
                            }
                        }

                        // There are some multi value attributes - force them to be of type array.
                        foreach(array("boundToHost","usedByHost","licenseKey") as $attr){
                            if(!is_array($entry[$attr])){
                                $entry[$attr] = array($entry[$attr]);
                            }
                        }
                        $entry['MODIFIED'] = FALSE;
                        $this->licenses[$entry['cn']] = $entry;
                    }
                }

                $this->orig_licenses = $this->licenses;
                $this->init_successfull = TRUE;
                return;
            }
        }
    }


    function execute()
    {
        plugin::execute();

        $this->licenseList->setAcl($this->getacl('licenses'));

        // Handle initialization failures.
        if(isset($_POST['retry_init'])) $this->init();
        if(!$this->init_successfull){
            $smarty = get_smarty();
            $smarty->assign("init_successfull", $this->init_successfull);
            return($smarty->fetch(get_template_path('licensePoolGeneric.tpl',TRUE,dirname(__FILE__))));
        }

        // Display dialogs! 
        if($this->dialog instanceOf plugin){
            $this->dialog->save_object();
            $display = $this->dialog->execute();
            $display.= "<hr><div class='plugin-actions'>";
            if($this->acl_is_writeable("licenses")){
                $display.="<button type='submit' name='license_finish'>".msgPool::okButton()."</button>";
            }
            $display.="<button type='submit' name='license_cancel'>".msgPool::cancelButton()."</button>";
            $display.="</div>";
            return($display);
        }

        $smarty = get_smarty();

        // Assign ACls 
        $plInfo = $this->plInfo();
        foreach($plInfo['plProvidedAcls'] as $name => $desc){
            $smarty->assign("{$name}ACL",$this->getacl($name));
        }
        foreach($this->attributes as $attr){
            $smarty->assign($attr,set_post($this->$attr));
        }
        $smarty->assign("init_successfull", $this->init_successfull);
        $smarty->assign("availableProductIds", array_diff( $this->availableProductIds, $this->productIds));
        $smarty->assign("productIds", $this->productIds);
        $smarty->assign("softwareIds", $this->softwareIds);
        $smarty->assign("licenses", $this->getLicenseList());
        $smarty->assign("initially_was_account", $this->initially_was_account);
        return($smarty->fetch(get_template_path('licensePoolGeneric.tpl',TRUE,dirname(__FILE__))));
    }


    function getLicenseList()
    {
        $data = $lData = array();
        if($this->acl_is_readable("licenses")){
            foreach($this->licenses as $i => $license){
                $maxInst = "";
                if($license['maximumInstallations'] == 0){
                    $maxInst = _("unlimited");
                }else{
                    $maxInst = $license['maximumInstallations'];
                }

                $map = array(
                        "VOLUME" => sprintf(_("Volume license (#%s)"), $maxInst),
                        "OEM"=>_("OEM"),
                        "RETAIL"=>_("Retail"),
                        "CONCURRENT"=>_("Concurrent"));

                $data[$license['cn']] = $license['cn'];
                $lData[$license['cn']] = array('data' => array($license['cn'],$map[$license['licenseModel']]));
            }
        }
        $this->licenseList->setListData($data,$lData);
        return($this->licenseList->render());
    }


    /* Save HTML inputs
     */
    function save_object()
    {

        $this->licenseList->save_object();
        $tmp = $this->licenseList->getAction();

        // Allow license edit via href links
        if($tmp['action'] == 'edit'){
            $id = $this->licenseList->getKey($tmp['targets'][0]); 
            if(isset($this->licenses[$id])){
                $this->dialog = new licenseGeneric($this->config,$this->dn,$this->licenses[$id], $this->opsiHosts);
                $this->dialog->set_acl_category('opsi');
                $this->dialog->set_acl_base($this->config->current['BASE']);
            }
        }
        if($tmp['action'] == 'delete'){
            $id = $this->licenseList->getKey($tmp['targets'][0]); 
            if(isset($this->licenses[$id])){
                unset($this->licenses[$id]);
            }
        }

        // Close license edit dialogs.
        if($this->dialog instanceOf plugin && isset($_POST['license_cancel'])){
            $this->dialog = NULL;
            return;
        }

        // Save license modifications
        if($this->dialog instanceOf plugin && isset($_POST['license_finish'])){
            $this->dialog->save_object();
            $msgs = $this->dialog->check();
            if(count($msgs)){
                msg_dialog::displayChecks($msgs);
            }else{
                $attrs = $this->dialog->save();
                $attrs['MODIFIED'] = TRUE;
                $this->licenses[$attrs['cn']] = $attrs;
                $this->dialog = NULL;
            }
            return;
        }

        if(isset($_POST['opsiLicensePoolPosted'])){
            plugin::save_object();  

            // Restore license cn, to avoid creating a copy...
            if($this->initially_was_account) $this->cn = $this->orig_cn;

            // We've to add prodcuts here 
            if(preg_match("/w/",$this->getacl("productIds"))){
                if(isset($_POST['availableProduct']) && isset($_POST['addProduct'])){
                    $pro = get_post('availableProduct');
                    if(isset($this->availableProductIds[$pro]) && !in_array($this->availableProductIds[$pro], $this->productIds)){
                        $this->productIds[] =$this->availableProductIds[$pro];
                    }
                }
            }

            // We've to remove products here
            if(preg_match("/w/",$this->getacl("productIds"))){
                if(isset($_POST['productIds']) && isset($_POST['removeProduct'])){
                    foreach($_POST['productIds'] as $key){
                        if(isset($this->productIds[$key])){
                            unset($this->productIds[$key]);
                        }
                    }
                }
            }

            // We've to add software here 
            if(preg_match("/w/",$this->getacl("windowsSoftwareIds"))){
                if(isset($_POST['newSoftwareId']) && isset($_POST['addSoftware'])){
                    $soft = trim(get_post('newSoftwareId'));
                    if(!empty($soft) && !in_array($soft, $this->softwareIds)){
                        $this->softwareIds[] = $soft;
                    }
                }
            }

            // We've to remove software Ids here
            if(preg_match("/w/",$this->getacl("windowsSoftwareIds"))){
                if(isset($_POST['softwareIds']) && isset($_POST['removeSoftware'])){
                    foreach($_POST['softwareIds'] as $key){
                        if(isset($this->softwareIds[$key])){
                            unset($this->softwareIds[$key]);
                        }
                    }
                }
            }

            // We've to create a new license
            if(preg_match("/w/",$this->getacl("licenses"))){
                if(isset($_POST['addLicense'])){
                    $this->dialog = new licenseGeneric($this->config,$this->dn,array(), $this->opsiHosts);
                    $this->dialog->set_acl_category('opsi');
                    $this->dialog->set_acl_base($this->config->current['BASE']);
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



    /* Removes the object from the opsi database
     */ 
    function remove_from_parent()
    {
        $this->si->deletePool($this->orig_cn);
        if($this->si->is_error()){
            msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
        }else{

            // Trigger remove signal
            $this->handle_post_events("remove");
        }

        new log("remove","users/".get_class($this),$this->dn,array_keys($this->attrs),$this->si->get_error());
    }


    /* Saves object modifications
     */  
    function save()
    {
        plugin::save();

        // Send modify/add events
        $mode = "modify";
        if($this->orig_dn == "new"){
            $mode = "add";
        }

        // Create or update the pool
        $this->si->createPool($this->cn, $this->description,$this->productIds,$this->softwareIds);#
            if($this->si->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
            }else{
                $this->handle_post_events($mode);
            }

        // Create, remove or update licenses
        $add    = array_diff_assoc($this->licenses,$this->orig_licenses);
        $del    = array_diff_assoc($this->orig_licenses,$this->licenses);
        $update = array_intersect($this->licenses,$this->orig_licenses);

        // Remove licenses 
        foreach($del as $license){
            $this->si->removeLicenseFromPool($this->cn, $license['cn']);
            if($this->si->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
            }
        }

        // Add licenses 
        foreach($add as $license){
            $this->si->createLicense(
                    $this->cn,  // Pool id
                    $license['cn'], 
                    $license['licenseKey'][0],
                    $license['licenseModel'],
                    $license['partner'],
                    $license['conclusionDate'],
                    $license['notificationDate'],
                    $license['description'],
                    $license['cn'],
                    $license['maximumInstallations'],
                    $license['boundToHost'],
                    $license['expirationDate']);

            if($this->si->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
            }
        }

        // Update licenses
        foreach($update as $license){

            // Do not save untouched licenses
            if(!$license['MODIFIED']){
                continue;
            }

            $this->si->createLicense(
                    $this->cn,  // Pool id
                    $license['cn'],
                    $license['licenseKey'][0],
                    $license['licenseModel'],
                    $license['partner'],
                    $license['conclusionDate'],
                    $license['notificationDate'],
                    $license['description'],
                    $license['cn'],
                    $license['maximumInstallations'],
                    $license['boundToHost'],
                    $license['expirationDate']);

            if($this->si->is_error()){
                msg_dialog::display(_("Error"),msgPool::siError($this->si->get_error()),ERROR_DIALOG);
            }
        }

        // Log action
        if($mode == "modify"){
            new log("modify","users/".get_class($this),$this->dn,array_keys($this->attrs),$this->si->get_error());
        }else{
            new log("create","users/".get_class($this),$this->dn,array_keys($this->attrs),$this->si->get_error());
        }

        return 0;
    }

    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Pool generic"),
                    "plDescription" => _("License pool generic"),
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 7,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("opsi"),
                    "plProvidedAcls"=> array(
                        "cn"                => _("Name"),
                        "description" => _("Description"),
                        "productIds"  => _("Applications"),
                        "windowsSoftwareIds"  => _("Windows software IDs"),
                        "licenses"  => _("Licenses"))
                    ));
    }
}


// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
