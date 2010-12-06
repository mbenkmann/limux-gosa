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


/*! \brief   Allows the creation of service and host principals.
  e.g.  host/server.intranet.company.de@COMPANY.DE
  e.g.  ldap/server.intranet.company.de@COMPANY.DE
  The is specified by the third parameter of the constructor, the   
  hostname is specified by the parent object. ($parent->cn).

  The keys can be created and removed.
  A running "GOsa support Daemon" with a krb5 module is required.
 */
class krbHostKeys extends plugin 
{

    public  $config;
    private $object;
    private $o_queue;
    private $goKrbRealm = "disabled";
    private $kerberos_support= FALSE;

    private $pwd_handle   = NULL;
    private $server_list  = array(); 
    private $last_list    = array(); // The last array created for the smarty template

    private $namingAttr       = "";
    private $is_service_key   = FALSE;
    private $confirm_box      = NULL;

    /*! \brief  Initalizes the kerberos host/service key generation. 
      (See class comment for more information)
      @param  Object  The GOsa configuration class. 
      @param  Object  The parent class (e.g. servgeneric) // $parent->cn MUST be set. 
     */
    public function __construct($config,&$object)
    {

        /* Initialize this plugin 
         */
        plugin::plugin($config, NULL);
        $this->object = $object;

        $this->acl_category = &$this->object->acl_category;
        $this->acl_base = &$this->object->acl_base;

        /* Instantiate si class, to be able to communicate with the support daemon.
         */
        $this->o_queue = new gosaSupportDaemon();

        /*  Create principal name, to detect whether there is a valid host key or not
         */
        if(!isset($this->object->cn)){
            trigger_error("Cannot initialize kerberos key service, invalid parent object given (Paramter 2).".
                    " Should be of class 'plugin' with accessible class member variable 'cn'.");
        }else{

            // Check if we've enabled the DNS settings and assemble an according entry
            $this->namingAttr= $this->getNamingAttr();
            $this->init();
        }

        // Prepare lists
        $this->keyList = new sortableListing();
        $this->keyList->setDeleteable(false);
        $this->keyList->setInstantDelete(false);
        $this->keyList->setEditable(false);
        $this->keyList->setWidth("100%");
        $this->keyList->setHeight("100px");
        $this->keyList->setColspecs(array('*','*','*','60px'));
        $this->keyList->setHeader(array(_("Realm"),_("Principle"),_("Status")));
        $this->keyList->setDefaultSortColumn(0);
    }


    function getNamingAttr()
    {
        // Do we have DNS settings?
        if (empty($this->object->netConfigDNS->dnsEntry['zoneName'])) {

            // no... use systems cn
            return $this->object->cn;

        } else {

            // yes... assemble new naming attribute
            return $this->object->cn.".".preg_replace('/^.*\/(.*)\.$/', '$1', $this->object->netConfigDNS->dnsEntry['zoneName']);
        }

    }



    /*! \brief  Reloads the list of available principals. 
     */ 
    public function reload_principals()
    {
        if($this->kerberos_support){
            foreach($this->server_list as $mac => $data){
                $principals = $this->pwd_handle->load_principals_for_server($mac);
                $this->server_list[$mac]['PRINCIPALS'] = $principals;
            }
        }
    }


    /*! \brief  Load/reload all key settings for the current host ($this->object).
      This reloads the list of all kerberos realms and which keys 
      are set for the current host.
     */
    private function init()
    {
        /* Check if the SI based kerberos method is available 
         */
        $this->kerberos_support = FALSE;
        if(class_available("passwordMethodMIT")){

            /* Update the naming attribute.
             */
            $this->namingAttr= $this->getNamingAttr();

            /* Create password handle, it has already a working principal parsing 
             */
            $this->pwd_handle = new passwordMethodMIT($this->config);
            if(!$this->pwd_handle->is_available()){
                return;
            }

            /* Check class initialation */
            if(!($this->pwd_handle instanceOf passwordMethodMIT)){
                trigger_error("Could not initialize kerberos password method 'passwordMethodMIT'.");
                return;
            }
            $this->pwd_handle->clear_cache();

            /* Get a list of all kerberos servers, defined in ldap
               and get a list of principals they are providing.
             */
            $ldap = $this->config->get_ldap_link();
            $ldap->cd($this->config->current['BASE']);
            $ldap->search("(&(objectClass=goServer)(objectClass=goKrbServer))",array("goKrbRealm","cn","description","macAddress"));
            $this->server_list = array();
            while($attrs = $ldap->fetch()){
                if(!isset($attrs['macAddress'][0])) continue;
                if(!isset($attrs['description'][0])) $attrs['description'][0] ="";

                /* Get all required informations for each realm 
                 */
                $mac        = $attrs['macAddress'][0];
                $realm      = $attrs['goKrbRealm'][0];
                $cn         = $attrs['cn'][0];
                $principals = $this->pwd_handle->load_principals_for_server($mac);

                /* Create a list of all realms and their attributes.
                 */ 
                $this->server_list[$mac]['CN']          = $cn;
                $this->server_list[$mac]['MAC']         = $mac;
                $this->server_list[$mac]['REALM']       = $realm;
                $this->server_list[$mac]['PRINCIPALS']  = $principals;

                /* Set first realm as selected.
                 */
                if($this->goKrbRealm == ""){
                    $this->goKrbRealm = $attrs['goKrbRealm'][0];
                }
            }

            if(isset($this->server_list) && count($this->server_list) && $this->o_queue->is_connected()){
                $this->kerberos_support = TRUE;
            }
        }
    }


    /*! \brief  Create the HTML output used in the host generic dialogs 
      or in server-services.
      It also checks if the host cn has changed and asks the user 
      if he want to update the keys to the current host name.
     */
    public function execute_by_prefix($prefix, $service_plugin =FALSE)
    {
        /* Skip if there is no kerberos support available 
         */
        if(!$this->kerberos_support || !$this->acl_is_readable("0")) return("");

        /* Check if naming context has changed,
           in this case ask user if he wants to update the keys. 
         */

        if($this->getNamingAttr() != $this->namingAttr){

            /* The confirm dialog was shown. Check if OK was pressed. 
             */
            if(is_object($this->confirm_box) && $this->confirm_box instanceof msg_dialog){
                if($this->confirm_box->is_confirmed()){

                    /* Walk through each server and check which keys have to be updated.
                     */
                    foreach($this->server_list as $server => $data){
                        foreach($data['PRINCIPALS'] as $p_name){
                            if(preg_match("/\/".preg_quote($this->namingAttr, '/')."\@/",$p_name)){
                                $pre = preg_replace("/\/.*$/","/",$p_name);
                                $this->delete_key($server, $pre.$this->namingAttr."@".$data['REALM']);
                                $this->create_key($server, $pre.$this->object->cn."@".$data['REALM']);
                            }
                        }
                    }
                }
                $this->init();
                $this->confirm_box = NULL;
            }else{

                /* The host name has changed, check if there are kerberos key which have to be updated 
                 */
                $found = array();
                $this->reload_principals();
                foreach($this->server_list as $server => $data){
                    foreach($data['PRINCIPALS'] as $p_name){

                        /* Collect all principals for the current host. 
                         */
                        if(preg_match("/\/".preg_quote($this->namingAttr, '/')."\@/",$p_name)){
                            $pre = preg_replace("/\/.*$/","/",$p_name);
                            $found[] = "<b>".$pre.$this->namingAttr."@".$data['REALM']."</b> ".
                                _("updated to").
                                " <b>".$pre.$this->object->cn."@".$data['REALM']."</b>";
                        }
                    }
                }

                /* If there is at leat one key that have to updated, then display a confirm dialog. 
                 */
                if(count($found)){
                    $this->confirm_box = new msg_dialog(_("Kerberos"),sprintf(_("The principal name of this system has changed. Do you want to update your principals? The affected principals are listed below: %s"),"<br>".msgPool::buildList($found)),OK_CANCEL_DIALOG); 
                }else{
                    $this->init();
                }
            }
        }

        $this->keyList->setAcl($this->getAcl(''));
        $princ_id = 0; // Used in posts

        $this->last_list[$prefix] = array();

        /* Create actions */
        $new = image('images/lists/element.png[new]', 'create_%ID%', _("Create key"));

        $rec= image('images/lists/reload.png', 'recreate_%ID%', _("Recreate key"));
        $rec.= image('images/lists/trash.png',  'remove_%ID%', _("Remove key"));

        /* Check ACLs */
        if(!$this->acl_is_writeable("0")){
            $new = $rec = "";
        }


        $data = $lData = array();
        foreach($this->server_list as $mac => $server){

            /* Recreate/Remove actions */
            $realm = $server['REALM'];

            /* Check if the given prefix is configured for a realm
               If it is configured display the recreate and remove button 
               if not display the create button. 
             */
            $found        = FALSE;

            // The master principal is specified by the $prefix parameter of this function.
            $master_princ = $prefix.$this->namingAttr."@".$realm;

            foreach($server['PRINCIPALS'] as $id => $princ){
                if(preg_match("/^".preg_quote($master_princ, '/')."$/i",$princ)){
                    $found = TRUE;
                    break;
                }
            }      

            /* Display buttons to "create" or to "remove/recreate" wheteher the prefix esists or not 
             */
            $princ_id ++;
            $this->last_list[$prefix][$princ_id]['REALM']  = $realm;
            $this->last_list[$prefix][$princ_id]['MAC']    = $mac;
            $this->last_list[$prefix][$princ_id]['NAME']   = $master_princ;

            if($found){
                $data[$princ_id] = $princ_id;
                $lData[$princ_id] = array('data' => array($realm,$master_princ,_("installed"),preg_replace("/%ID%/",$princ_id,$rec)));
                $this->last_list[$prefix][$princ_id]['USED'] = TRUE;
            }else{
                $data[$princ_id] = $princ_id;
                $lData[$princ_id] = array('data' => array($realm,$master_princ,"-",preg_replace("/%ID%/",$princ_id,$new)));
                $this->last_list[$prefix][$princ_id]['USED'] = FALSE;
            }

            /* Get all additionally configured service host keys, this are all keys that 
               do not match the master_princ.
             */
            if(!$service_plugin){
                foreach($server['PRINCIPALS'] as $id => $princ){
                    if(preg_match("/".preg_quote("/".$this->namingAttr."@", '/')."/i",$princ) &&
                            !preg_match("/^".preg_quote($prefix, '/')."/",$princ)){

                        /* Display buttons to "create" or to "remove/recreate" wheteher the prefix esists or not 
                         */
                        $princ_id ++;

                        $this->last_list[$prefix][$princ_id]['USED']  = TRUE;
                        $this->last_list[$prefix][$princ_id]['REALM'] = $realm;
                        $this->last_list[$prefix][$princ_id]['MAC']   = $mac;
                        $this->last_list[$prefix][$princ_id]['NAME']  = $princ;

                        $data[$princ_id] = $princ_id;
                        $lData[$princ_id] = array('data' => array($realm,$princ,_("installed"),preg_replace("/%ID%/",$princ_id,$rec)));
                        $this->last_list[$prefix][$princ_id]['USED'] = FALSE;
                    } 
                }
            }
        }
        $this->keyList->setListData($data,$lData);    
        $this->keyList->update();

        $smarty = get_smarty(); 
        $smarty->assign("list",$this->keyList->render());
        $smarty->assign("server_list" ,$this->server_list);
        $smarty->assign("service_plugin" ,$service_plugin);
        $smarty->assign("is_service_key" , $this->is_service_key);
        return($smarty->fetch (get_template_path('krb_host_keys.tpl', TRUE,dirname(__FILE__))));
    }


    /*! \brief  This function creates a new host/service key for the given server and principal name 
      @param  String  The servers mac address. (Must be available in $this->server_list)
      @param  $name   The new principal name. (e.g. ldap/some.host@REALM.FASEL)
      @retrun Boolean TRUE on success else FALSE.
     */
    public function create_key($id,$name)
    {
        $this->o_queue->krb5_add_principal($id,$name,array());
        if($this->o_queue->is_error()){
            msg_dialog::display(_("Service infrastructure"),msgPool::siError($this->o_queue->get_error()),ERROR_DIALOG);
            return(FALSE);
        }
        return(TRUE);
    }  


    /*! \brief  This function recreates a host/service key for the given server and principal name.
      It remove the old key first and then tries to create a new one.
      @param  String  The servers mac address. (Must be available in $this->server_list)
      @param  $name   The new principal name. (e.g. ldap/some.host@REALM.FASEL)
      @retrun Boolean TRUE on success else FALSE.
     */
    public function recreate_key($id,$name)
    {
        if($this->delete_key($id,$name)){
            if($this->create_key($id,$name)){
                return(TRUE);
            }
        }
        return(FALSE);
    }  


    /*! \brief  This function removes a host/service key for the given server and principal name.
      @param  String  The servers mac address. (Must be available in $this->server_list)
      @param  $name   The new principal name. (e.g. ldap/some.host@REALM.FASEL)
      @retrun Boolean TRUE on success else FALSE.
     */
    public function delete_key($id,$name)
    {
        $this->o_queue->krb5_del_principal($id,  $name);
        if($this->o_queue->is_error()){
            msg_dialog::display(_("Service infrastructure"),msgPool::siError($this->o_queue->get_error()),ERROR_DIALOG);
            return(FALSE);
        }
        return(TRUE);
    }  


    /*! \brief  This function acts on the HTML posts and ensures that our 
      delete/create/recreate are executed. 
     */
    public function save_object_by_prefix($prefix)
    {
        if(!isset($this->last_list[$prefix])) return; // No posts for us 

        if(!$this->acl_is_writeable("0")) return; // No permissions to change anything

        foreach($_POST as $name => $value){
            if(preg_match("/^create_/",$name)){
                $id = preg_replace("/^create_([0-9]*)$/","\\1",$name);
                if(isset($this->last_list[$prefix][$id])){
                    $data = $this->last_list[$prefix][$id]; 
                    $this->create_key($data['MAC'],$data['NAME']);
                    $this->init();
                }
                break;
            }
            if(preg_match("/^recreate_/",$name)){
                $id = preg_replace("/^recreate_([0-9]*)$/","\\1",$name);
                if(isset($this->last_list[$prefix][$id])){
                    $data = $this->last_list[$prefix][$id]; 
                    $this->recreate_key($data['MAC'],$data['NAME']);
                    $this->init();
                }
                break;
            }
            if(preg_match("/^remove_/",$name)){
                $id = preg_replace("/^remove_([0-9]*)$/","\\1",$name);
                if(isset($this->last_list[$prefix][$id])){
                    $data = $this->last_list[$prefix][$id]; 
                    $this->delete_key($data['MAC'],$data['NAME']);
                    $this->init();
                }
                break;
            }
        }    
    } 


    /*! \brief  Removes all key settings for the given host ($this->object->cn)
     */
    public function remove_from_parent_by_prefix($prefix)
    {
        if(!$this->kerberos_support)  return;
        foreach($this->server_list as $server => $data){
            foreach($data['PRINCIPALS'] as $p_name){

                /* Detect all principals for the current host.
                 */
                if(preg_match("/".preg_quote($prefix.$this->namingAttr, '/')."\@/",$p_name)){
                    $this->delete_key($server, $p_name);
                }
            }
        }
    }


    /*! \brief  Ensures that a service plugin suitable HTML output is generated.
      @param  Boolean TRUE enables servie plugin output, FALSE diables it. 
     */
    public function ServiceKey($bool = TRUE)
    {
        $this->is_service_key = $bool;
    }


    /*! \brief Return plugin informations for acl handling
      @return Array   ACL infos.
     */
    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Kerberos key"),
                    "plDescription" => _("Kerberos host key"),
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 2,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("server","workstation","terminal"),

                    "plProvidedAcls"=> array()
                    ));


    }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
