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

class passwordMethodMIT extends passwordMethod
{

    var $dn             = "new";   // DN of the current object
    var $parent_dn      = "new";   // parents DN
    var $is_account     = FALSE;   // This is TRUE if this object already has a krb extension
    var $server_list    = array(); // A list with all configured servers
    var $map            = array(); // Mapping array, maps SERVER-REALM, REALM-SERVER ...

    var $goKrbRealm     = "";      // The realm name this principal belongs to
    var $principal      = "";      // The principals name (e.g. user@MY-DOMAIN.SYS)
    var $is_new         = TRUE;    // Is TRUE if principal is new 

    var $si_error       = FALSE;   // TRUE is daemon communication failed
    var $si_error_msg   = "";      // The last error message if above attribute is TRUE.

    var $values = array(
            "PRINC_EXPIRE_TIME",      // Expiry date of this principal
            "PW_EXPIRATION",          // Password expiration 
            "MAX_LIFE",               // Ticket lifetime
            "MASK",                   // I'dont know 
            "MAX_RENEWABLE_LIFE",     // Max ticket lifetime when renewed
            "POLICY");                // The policy used by this principal

    var $PRINC_EXPIRE_TIME        = 0;
    var $PW_EXPIRATION            = 0;
    var $PRINC_EXPIRE_TIME_clear  = TRUE;
    var $PW_EXPIRATION_clear      = TRUE;
    var $MAX_LIFE                 = 36000;
    var $MAX_RENEWABLE_LIFE       = 604800;
    var $MASK                     = 0;

    var $flags = array(
            "DISALLOW_POSTDATED"    =>0x00000001 ,  // Pohibit postdated tickets
            "DISALLOW_FORWARDABLE"  =>0x00000002 ,  // Prohibit forwardable tickets
            "DISALLOW_TGT_BASED"    =>0x00000004 ,  // Disallow Ticket-Granting Service
            "DISALLOW_RENEWABLE"    =>0x00000008 ,  // Prohibit renewable tickets
            "DISALLOW_PROXIABLE"    =>0x00000010 ,  // Disallow proxiable tickets
            "DISALLOW_DUP_SKEY"     =>0x00000020 ,  // Disallow user to user authentification
            "DISALLOW_ALL_TIX"      =>0x00000040 ,  // Forbid ticket issuance
            "REQUIRES_PRE_AUTH"     =>0x00000080 ,  // Preauthentication required
            "REQUIRES_HW_AUTH"      =>0x00000100 ,  // Hardware preauthentication
            "REQUIRES_PWCHANGE"     =>0x00000200 ,  // Force a password change
            "UNKNOWN_0x00000400"    =>0x00000400 ,  // ? 
            "UNKNOWN_0x00000800"    =>0x00000800 ,  // ?
            "DISALLOW_SVR"          =>0x00001000 ,  // Prohibit issuance of service tickets
            "PWCHANGE_SERVICE"      =>0x00002000 ,  // Password change service
            "SUPPORT_DESMD5"        =>0x00004000 ,  // ?
            "NEW_PRINC"             =>0x00008000 ); // ?

    var $used_flags = 128;        // Flags, see below

    var $readonly = array(
            "FAIL_AUTH_COUNT",        // The number of failed logins 
            "KVNO",                   // Key version number
            "LAST_FAILED",            // Last failed login time
            "LAST_PWD_CHANGE",        // Password last change time
            "LAST_SUCCESS",           // Last successful login 
            "MOD_DATE");              // Last modification time

    var $FAIL_AUTH_COUNT          = 0;
    var $KVNO                     = "";
    var $LAST_FAILED              = 0;
    var $LAST_PWD_CHANGE          = 0;
    var $LAST_SUCCESS             = 0;
    var $MOD_DATE                 = 0;

    var $POLICY                   = "_none_";
    var $POLICIES                 = array(); // Policies provided by the corrently selected realm/server

    public function is_locked($config,$dn = "")
    {
        return(FALSE);
    }

    public function lock_account($config,$dn = "")
    {
        return(FALSE);
    }

    public function unlock_account($config,$dn = "")
    {
        return(FALSE);
    }

    public function __construct(&$config,$dn = "new")  
    {
        $this->config= $config;
        $this->parent_dn = $dn;

        /* No config object given, this may be the case 
           if there is only a is_available() request triggered.
         */
        if(!is_object($this->config)){
            return;
        }  

        /* Keep the cached valued and skip loading principals 
           from si until this method gets configured.
         */
        $skip_si_access = TRUE;
        if($dn != "new" && $dn != ""){
            session::un_set("MIT_CACHE");
            session::un_set("MIT_PRINCIPAL_CACHE");
            session::un_set("MIT_POLICY_CACHE");
            $this->clear_cache();
            $skip_si_access = FALSE;
        }

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

            if($skip_si_access){
                $principals = array();
            }else{
                $principals = $this->load_principals_for_server($attrs['macAddress'][0]);
            }

            /* Create Realm/Server/Principal mapping.
             */
            foreach($principals as $principal){
                $this->map["PRINCIPAL_SERVER"][$principal] = $attrs['cn'][0];
                $this->map["PRINCIPAL_REALM"] [$principal] = $attrs['goKrbRealm'][0];
            }
            $this->map["SERVER_REALM"][$attrs['cn'][0]] = $attrs['goKrbRealm'][0];
            $this->map["REALM_SERVER"][$attrs['goKrbRealm'][0]] = $attrs['cn'][0];

            /* Set first realm as selected.  
             */
            if($this->goKrbRealm == ""){
                $this->goKrbRealm = $attrs['goKrbRealm'][0];
            }

            /* Create Server list
             */
            $this->server_list[$attrs['cn'][0]] = array("macAddress" => $attrs['macAddress'][0],
                    "description"=> $attrs['description'][0],
                    "dn"         => $attrs['dn'],
                    "principals" => $principals,
                    "goKrbRealm" => $attrs['goKrbRealm'][0],
                    "cn"         => $attrs['cn'][0]);
        }

        /*  If this methods is initialized with a valid object dn then 
            load the object data from ldap and the SI daemon && initialize this class.
         */
        $this->is_new = TRUE;
        if(!$skip_si_access){
            $ldap = $this->config->get_ldap_link();
            $ldap->cd($dn);
            $ldap->cat($dn);
            $this->attrs = $ldap->fetch();

            /* Set initial pwd hash which take effect if no password method was set yet.  
               Will be overwritten by the following lines, if the user has already a valid principal.
             */
            $this->principal = $this->attrs['uid'][0]."@".$this->goKrbRealm;

            if(isset($this->attrs['userPassword']) && preg_match("/^\{".$this->get_hash_name()."\}/",$this->attrs['userPassword'][0])){

                /* Extract principal name out of userPassword attribute 
                 */
                $p_name = preg_replace("/^\{".$this->get_hash_name()."\}/","",$this->attrs['userPassword'][0]);

                /* Try to detect server our principal is configured on
                 */
                if(isset($this->map['PRINCIPAL_SERVER'][$p_name])){
                    $server= $this->map['PRINCIPAL_SERVER'][$p_name];
                    $this->goKrbRealm = $this->map['SERVER_REALM'][$server];
                    $this->principal  = $p_name;

                    /* Load policies */
                    $server_name = $this->map['REALM_SERVER'][$this->goKrbRealm];
                    $server_mac  = $this->server_list[$server_name]['macAddress'];
                    $this->POLICIES = $this->load_policies_for_server($server_mac);

                    /* Load principal */
                    $this->load_principal($this->server_list[$server]['macAddress'],$p_name);
                    $this->is_new = FALSE;
                }
            }
        }
    }


    public static function clear_cache()
    {
        session::un_set("MIT_CACHE");
        session::un_set("MIT_PRINCIPAL_CACHE");
        session::un_set("MIT_POLICY_CACHE");
    }


    /*! \brief  Load a specific principal from the si daemon 
      and initialize this plugin with it.
      @param  String  The macAddress of the kerberos server.
      @param  String  The name of the principal to load.
     */
    public function load_principal($server,$name)
    {
        $o = new gosaSupportDaemon();
        $tmp = array();
        $tmp = $o->krb5_get_principal($server,$name);

        if($o->is_error()){
            $this->si_error     = TRUE;
            $this->si_error_msg = sprintf(_("Cannot load principal '%s', from server '%s'!"),$name,$server).":&nbsp;<br>".$o->get_error(); 
            msg_dialog::display(_("Service infrastructure"),msgPool::siError($o->get_error()),ERROR_DIALOG);      
        }else{

            /* Load flags 
             */
            if(isset($tmp['ATTRIBUTES'])){
                $this->used_flags = $tmp['ATTRIBUTES'];
            }

            /* Load readonly attributes 
             */
            foreach($this->readonly as $attr){
                if(isset($tmp[$attr])){
                    $this->$attr = $tmp[$attr];
                }
            } 

            /* Load modifyable attributes
             */
            foreach($this->values as $attr){
                if(isset($tmp[$attr])){
                    $this->$attr = $tmp[$attr];
                }
            } 

            /* Update time checkboxes 
             */
            $date_values = array("PW_EXPIRATION","PRINC_EXPIRE_TIME");
            foreach($date_values as $value){
                if(!empty($this->$value)){
                    $clear = $value."_clear";
                    $this->$clear = FALSE;
                }
            }
        }
    }


    /*! \brief  Get the list of all configured principals for a given server.
      @param  String The servers mac address.
      @return Array A list with all principals
      The results will cached.  
     */
    public function load_principals_for_server($server)
    {
        if(!session::is_set("MIT_PRINCIPAL_CACHE")){
            session::set("MIT_PRINCIPAL_CACHE",array());
        }
        $cache = session::get("MIT_PRINCIPAL_CACHE");
        if(!isset($cache[$server])){
            $o = new gosaSupportDaemon();
            if($o->is_configured()){
                $tmp = $o->krb5_list_principals($server);
                if($o->is_error()){
                    $this->si_error     = TRUE;
                    $this->si_error_msg = sprintf(_("Cannot load principals from server '%s'!"),$server).":&nbsp;<br>".$o->get_error(); 
                    msg_dialog::display(_("Service infrastructure"),msgPool::siError($o->get_error()),ERROR_DIALOG);
                    return(array());
                }else{
                    $cache[$server] = $tmp;
                }
            }
            session::set("MIT_PRINCIPAL_CACHE",$cache);
        }
        return($cache[$server]);
    }


    /*! \brief get list of all configured policies
      for a given server. 
      The results will cached.  
     */
    public function load_policies_for_server($server)
    {
        if(!session::is_set("MIT_POLICY_CACHE")){
            session::set("MIT_POLICY_CACHE",array());
        }
        $cache = session::get("MIT_POLICY_CACHE");
        if(!isset($cache[$server])){
            $o = new gosaSupportDaemon();
            $tmp = $o->krb5_list_policies($server);
            if($o->is_error()){
                $this->si_error     = TRUE;
                $this->si_error_msg = sprintf(_("Cannot load policies from server '%s'!"),$server).":&nbsp;<br>".$o->get_error(); 
                msg_dialog::display(_("Service infrastructure"),msgPool::siError($o->get_error()),ERROR_DIALOG);
                return(array());
            }else{
                $cache[$server] = array();
                $cache[$server]["_none_"] = _("none"); 
                foreach($tmp as $policy){
                    $cache[$server][$policy] = $policy;
                }
                ksort($cache[$server]);
            }
            session::set("MIT_POLICY_CACHE",$cache);
        }
        return($cache[$server]);
    }


    /*! \brief Check if this password method is useable. 
      This is the case if there is a si server running and at least one server configured.
      kerberos support. 
     */ 
    public function is_available()
    {
        global $config;

        /* No config object given, this may be the case 
           if there is only a is_available() request triggered.
         */
        if(!is_object($config)){
            return;
        }


        $o = new gosaSupportDaemon(FALSE);

        if(count($this->server_list) && $o->connect() && $o->is_configured()){
            return TRUE; 
        }
        return(FALSE);  
    }


    /*! \brief Create the password hash. In this case: {kerberos/sasl}name@RELAM 
      @param  String  The password -in this case unusued.             
      @return String  The generated hash
     */
    public function generate_hash($pwd = "")
    {
        $mode= "kerberos";

        /* No config object given, this may be the case 
           if there is only a is_available() request triggered.
         */
        if (is_object($this->config) && $this->config->get_cfg_value("core","useSaslForKerberos") == "true"){
            $mode= "sasl";
        }
        return "{".$mode."}".$this->attrs['uid'][0]."@".$this->goKrbRealm;
    }


    public function create_template_hash($attrs)
    {
        return($this->generate_hash());
    }


    /*! \brief  Removes this principal.
     */
    public function remove_from_parent()
    {
        if(!empty($this->principal) && $this->goKrbRealm){
            $server = $this->map['REALM_SERVER'][$this->goKrbRealm];
            $o = new gosaSupportDaemon();
            if(!$o->krb5_del_principal($this->server_list[$server]['macAddress'],$this->principal)){
                $this->si_error     = TRUE;
                $this->si_error_msg = $o->get_error();
                msg_dialog::display(_("Service infrastructure"),msgPool::siError($o->get_error()),ERROR_DIALOG);      
            }
        }
    }


    /*! \brief  Set a new password for this principal 
      @param  String The new password.
     */
    public function set_password($password)
    {
        if(!empty($this->principal) && $this->goKrbRealm){
            $server = $this->map['REALM_SERVER'][$this->goKrbRealm];
            $o = new gosaSupportDaemon();
            if(!$o->krb5_set_password($this->server_list[$server]['macAddress'],$this->principal,$password)){
                msg_dialog::display(_("Service infrastructure"),msgPool::siError($o->get_error()),ERROR_DIALOG);      
                return(FALSE);
            }
        }
        return(TRUE);
    }


    /*! \brief  Return the hash name of this mehtod,  e.g. to identify methods.
      @return String  The hash used by this method.
     */
    public function get_hash_name()
    {
        $mode= "kerberos";

        /* No config object given, this may be the case 
           if there is only a is_available() request triggered.
         */
        if (is_object($this->config) && $this->config->get_cfg_value("core","useSaslForKerberos") == "true"){
            $mode= "sasl";
        }
        return "$mode";
    }


    /*! \brief  Returns TRUE if this method is configurable else FALSE
      @retrun Boolena TRUE if configurable, else FALSE.
     */
    public function is_configurable()
    {
        return TRUE;
    }


    /*! \brief  Additional info displayed in the users password method drop down.
      @retunr String  Additional password method info.
     */
    public function get_description()
    {
        return(_("Daemon based"));
    }


    /*! \brief  Display a HTML based configuration dialog for this plugin
      @return String  HTML.
     */
    public function configure()
    {
        $this->save_object();

        $years = array();
        $start = date("Y")-1;
        for($i = $start; $i < ($start +20) ; $i++){
            $years[$i] = $i;
        }
        $month= array();
        for($i = 1; $i <= 12  ; $i++){
            $month[str_pad($i,2,"0",STR_PAD_LEFT)] = $i;
        }
        $days= array();
        for($i = 1; $i <= 31  ; $i++){
            $days[str_pad($i,2,"0",STR_PAD_LEFT)] = $i;
        }
        $hours= array();
        for($i = 0; $i <= 23  ; $i++){
            $hours[str_pad($i,2,"0",STR_PAD_LEFT)] = $i;
        }
        $minutes= array();
        for($i = 0; $i <= 59  ; $i++){
            $minutes[str_pad($i,2,"0",STR_PAD_LEFT)] = $i;
        }


        /* Cancel heimdal options */
        if (isset($_POST['pw_abort']) || $this->display == FALSE){
            $this->display = FALSE;
            return("");
        }

        /* Cancel heimdal options */
        if (isset($_POST['pw_save'])){
            $msgs = $this->check();
            if(count($msgs)){
                foreach($msgs as $msg){
                    msg_dialog::display(_("Kerberos"),$msg,WARNING_DIALOG);
                }
            }else{
                $this->display = FALSE;
                return "";
            }
        }

        $smarty = get_smarty();
        $smarty->assign("si_error",$this->si_error);
        $smarty->assign("si_error_msg",$this->si_error_msg);
        $smarty->assign("years",       set_post($years));
        $smarty->assign("month",       set_post($month));
        $smarty->assign("days",        set_post($days));
        $smarty->assign("hours",       set_post($hours));
        $smarty->assign("minutes",     set_post($minutes));
        $smarty->assign("server_list",  $this->server_list);
        $smarty->assign("POLICY",       $this->POLICY);
        $smarty->assign("goKrbRealm",   $this->goKrbRealm);
        $server_name = $this->map['REALM_SERVER'][$this->goKrbRealm];
        $server_mac  = $this->server_list[$server_name]['macAddress'];
        $this->POLICIES = $this->load_policies_for_server($server_mac);
        $smarty->assign("POLICIES"  ,$this->POLICIES);

        foreach($this->values as $attr){
            $smarty->assign($attr ,set_post($this->$attr));
        }
        foreach($this->readonly as $attr){
            $smarty->assign($attr ,set_post($this->$attr));
        }
        foreach($this->flags as $attr => $hex){
            $smarty->assign($attr, ($this->used_flags & $hex ));
        }

        $date_values = array("PRINC_EXPIRE_TIME","PW_EXPIRATION");
        foreach($date_values as $date_val){
            $clear = $date_val."_clear";
            $smarty->assign($date_val."_clear",$this->$clear);
            $smarty->assign($date_val."_y",date("Y",$this->$date_val));
            $smarty->assign($date_val."_m",date("m",$this->$date_val));
            $smarty->assign($date_val."_d",date("d",$this->$date_val));
            $smarty->assign($date_val."_h",date("h",$this->$date_val));
            $smarty->assign($date_val."_i",date("i",$this->$date_val));
        }

        return($smarty->fetch(get_template_path("pwd_kerberos_mit.tpl",TRUE,dirname(__FILE__))));
    }


    /*! \brief  Saves all relevant HTML posts for this plugin
     */
    public function save_object()
    {
        /* If the communication with the si server failed, 
           you are able to retry to connect to the server.
           Here we hanlde those requests.
         */
        if(isset($_POST['retry_si'])){
            $this->si_error= FALSE;
            $this->si_error_msg= "";
            session::un_set("MIT_PRINCIPAL_CACHE");
            session::un_set("MIT_POLICY_CACHE");
            $this->__construct($this->config,$this->parent_dn);
        }

        /* Only handle posts for this plugin, it its content was posted
         */
        if(isset($_POST['pwd_heimdal_posted'])){

            if(isset($_POST['goKrbRealm'])){
                $this->goKrbRealm = get_post("goKrbRealm");
            }

            $this->used_flags = 0;
            foreach($this->flags as $attr => $hex){
                if(isset($_POST[$attr])){
                    $this->used_flags |= $hex; 
                }
            }

            foreach(array("MAX_LIFE","MAX_RENEWABLE_LIFE","POLICY") as $attr){
                if(isset($_POST[$attr])){
                    $this->$attr = get_post($attr);
                }
            }

            $date_values = array("PW_EXPIRATION","PRINC_EXPIRE_TIME");
            foreach($date_values as $date_value){
                $clear = $date_value."_clear";
                if(isset($_POST[$date_value."_clear"])){
                    $this->$clear = TRUE;
                }else{
                    $this->$clear = FALSE;
                    $this->$date_value = gmmktime( 
                            get_post($date_value."_h"),
                            get_post($date_value."_i"),
                            0,
                            get_post($date_value."_m"),
                            get_post($date_value."_d"),
                            get_post($date_value."_y"));
                }
            }
        }
    }


    /*! \brief  Checks the values specified in the configuration dialog. 
      @return Array Containing all error messages.
     */
    public function check()
    {
        $message = array();

        if(!preg_match("/^[0-9]*$/",$this->MAX_LIFE)){
            $message[] = msgPool::invalid(_("Ticket max life"),$this->MAX_LIFE,"/[0-9]/");
        }
        if(!preg_match("/^[0-9]*$/",$this->MAX_RENEWABLE_LIFE)){
            $message[] = msgPool::invalid(_("Ticket max renew"),$this->MAX_RENEWABLE_LIFE,"/[0-9]/");
        }
        return($message);
    }


    /*! \brief  Adapt account settings from given dn 
     */ 
    public function adapt_from_template ($dn)
    {
        $source = new passwordMethodMIT($this->config,$dn);
        $attrs = array("PRINC_EXPIRE_TIME","PW_EXPIRATION","POLICY","MAX_LIFE","MAX_RENEWABLE_LIFE","MASK","used_flags");
        foreach($attrs as $attr){
            $this->$attr = $source->$attr;
        }
    }


    /*! \brief Saves changes back to the SI daemon.
     */
    public function save($dn)
    {
        $ldap = $this->config->get_ldap_link();
        $ldap->cd($dn);
        $ldap->cat($dn,array('uid'));
        $attrs = $ldap->fetch();

        if(isset($attrs['uid'][0])){

            /* Get servers mac */
            $server_name = $this->map['REALM_SERVER'][$this->goKrbRealm];
            $server_mac  = $this->server_list[$server_name]['macAddress'];

            $uid       = $attrs['uid'][0];
            $principal = $uid."@".strtoupper($this->goKrbRealm); 
            $policy    = $this->POLICY;

            /* Collect flags */
            $flags = array();
            $entry = array();

            $entry['ATTRIBUTES'] = $this->used_flags;

            /* Append other values */
            foreach($this->values as $attr){
                if($attr == "POLICY") continue;
                $entry[$attr] = $this->$attr;
            }

            /* Prepare entry to be saved */
            if($policy != "_none_"){
                $entry['POLICY'] = $policy;
            }

            /* Set date values 
             */
            $date_values = array("PW_EXPIRATION","PRINC_EXPIRE_TIME");
            foreach($date_values as $value){
                $clear = $value."_clear";
                if($this->$clear){
                    $entry[$value] = 0;
                }
            }    


            /* Save principal changes */
            $o = new gosaSupportDaemon();
            if(in_array($principal,$this->server_list[$server_name]['principals'])){
                $this->is_new = FALSE;
            }

            if($this->is_new){
                $o->krb5_add_principal($server_mac,$principal,$entry);
            }else{
                $o->krb5_set_principal($server_mac,$principal,$entry);
            }
            if($o->is_error()){
                $this->si_error     = TRUE;
                $this->si_error_msg = $o->get_error();
                msg_dialog::display(_("Service infrastructure"),msgPool::siError($o->get_error()),ERROR_DIALOG);      
            }
        }
    }



}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
