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


class licenseByProduct extends plugin 
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

        // Is this a new object? 
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

            // Get license information for the current product.
            $res = $this->si->getLicensesForProduct($this->cn);
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
        // Handle initialization failures.
        if(isset($_POST['retry_init'])) $this->init();
        if(!$this->init_successfull){
            $smarty = get_smarty();
            $smarty->assign("init_successfull", $this->init_successfull);
            return($smarty->fetch(get_template_path('licenseByProduct.tpl',TRUE,dirname(__FILE__))));
        }

        $ui = get_userinfo();
        $readable = $ui->get_permissions($this->config->current['BASE'], "opsi/licenseByProduct","");
        $data = array();
        $this->usageList->setAcl($readable);
        foreach($this->licenseUses as $license){
            $data[] = array('data'=> array($license['HOSTID'][0]['VALUE'],
                        $license['SOFTWARELICENSEID'][0]['VALUE'],
                        $license['LICENSEPOOLID'][0]['VALUE'])); 
        } 
        $this->usageList->setListData($data,$data);
        $this->usageList->update();


        $smarty = get_smarty();

        // Assign ACls 
        $plInfo = $this->plInfo();
        $smarty->assign("licenseUses", $this->usageList->render());
        $smarty->assign("init_successfull", $this->init_successfull);
        $smarty->assign("initially_was_account", $this->initially_was_account);
        return($smarty->fetch(get_template_path('licenseByProduct.tpl',TRUE,dirname(__FILE__))));
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


    function save( ){}
    function remove_from_parent(){ }


    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Usage by product"),
                    "plDescription" => _("License usage by product"),
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 12,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("opsi"),
                    "plProvidedAcls"=> array()));
    }
}


// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
