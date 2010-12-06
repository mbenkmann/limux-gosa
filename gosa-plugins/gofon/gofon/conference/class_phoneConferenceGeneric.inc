<?php
/*
   This code is part of GOsa (https://gosa.gonicus.de)
   Copyright (C) 2003  Cajus Pollmeier

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

class conference extends plugin
{
    /* department attributes */
    var $cn						  = "";
    var $description		= "";
    var $base 				  = "";
    var $old_base 	    = "";
    var $ou 						= "";
    var $goFonPIN				= "";
    var $old_cn         = "";

    var $goFonConferenceOption   	= "";
    var $goFonConferenceOption_P 	= "";   // Set PIN 
    var $goFonConferenceOption_r 	= "";   // record Conference
    var $goFonConferenceOption_M 	= "";   // Play Music opn hold
    var $goFonConferenceOption_s 	= "";   // activate menu
    var $goFonConferenceOption_i 	= "";   // announce new and leaving user
    var $goFonConferenceOption_c 	= "";   // Count User
    var $goFonConferenceOption_D 	= "";   // Conference Type, no PIN/PIN

    var $goFonConferenceOwner     = "";
    var $goFonHomeServer          = "0";      // Home server of the conference
    var $init_HomeServer          = "0";      // Initial home server of the conference
    var $goFonHomeServers         = array();  // All available home servers

    var $goFonConferenceOptionFormat 		= "";
    var $goFonConferenceOptionLifetime 	= "";
    var $telephoneNumber 		= "";

    var $old_tele_number	        = false;
    var $old_dn;
    var $baseSelector;

    /* Headpage attributes */
    var $view_logged = FALSE;
    var $dialog ;

    /* attribute list for save action */
    var $attributes= array("cn","base", "description", "goFonPIN","goFonConferenceOption_P","goFonConferenceOption_r","goFonHomeServer",
            "goFonConferenceOption_M","goFonConferenceOption_s","goFonConferenceOption_i","goFonConferenceOption_c","goFonHomeServer",
            "goFonConferenceOption_D","goFonConferenceOptionFormat","goFonConferenceOptionLifetime","telephoneNumber","goFonConferenceOwner");

    var $language  = "de";
    var $languages = array();

    var $objectclasses= array("top", "goFonConference");

    var $orig_dn ="";
    var $orig_base = "";

    function conference (&$config, $dn, $plugin= NULL)
    {
        plugin::plugin($config, $dn, $plugin);
        $this->is_account	= TRUE;
        $this->ui			    = get_userinfo();
        $this->orig_dn		= $dn;

        $this->languages= get_languages(TRUE,TRUE);


        /* Check server configurations
         * Load all server configuration in $this->goFonHomeServers if available
         *  and use first server as default if necessary.
         */
        $a_SETUP= array();
        $config = session::get('config');
        if(isset($config->data['SERVERS']['FON']) && is_callable("mysql_connect")) {

            /* Set available server */
            $this->goFonHomeServers = $config->data['SERVERS']['FON'];

            /* Set default server */
            if($this->dn == "new"){
                $this->goFonHomeServer = $this->goFonHomeServers[0]['DN'];
            }

            /* Remember inital home server, to be able to remove old entries */
            $this->init_HomeServer = $this->goFonHomeServer;

            /* get config */
            if(!isset($this->goFonHomeServers[$this->goFonHomeServer])){
                msg_dialog::display(_("Obsolete entry"), sprintf(_("The current home server is not available anymore. It will be moved to '%s' if you save this entry!"), preg_replace("/,/",", ",$this->goFonHomeServers[0]['SERVER'])), ERROR_DIALOG);

                $this->goFonHomeServer = $this->goFonHomeServers[0]['DN'];
                $this->init_HomeServer = $this->goFonHomeServers[0]['DN'];
            }
            $cur_cfg = $this->goFonHomeServers[$this->goFonHomeServer]; 
        }

        /* Set base */
        if ($this->dn == "new"){
            $ui= get_userinfo();
            if(session::is_set('CurrentMainBase')){
                $this->base  = session::get('CurrentMainBase');
            }else{
                $this->base= dn2base($ui->dn);
            }
        } else {

            /* Get base */ 
            $this->base = preg_replace ("/^[^,]+,".preg_quote(get_ou("conference", "phoneConferenceRDN"), '/')."/i","",$this->dn);;
        }

        $this->goFonConferenceOwner=$this->ui->dn;

        /* Parse Options ... 
         * Parameter|Lifetime|number
         */
        if($this->dn!="new"){
            $tmp1= explode("|",$this->attrs['goFonConferenceOption'][0]);

            for($i = 0 ; $i < strlen($tmp1[0]);$i++){
                $varname = "goFonConferenceOption_".$tmp1[0][$i];
                if($tmp1[0][$i]=="d"){
                    $this->goFonConferenceOption_D = $tmp1[0][$i];    
                }else{
                    $this->$varname = $tmp1[0][$i];    
                }
            }

            $this->goFonConferenceOptionLifetime  = $tmp1[1];
            if(isset($tmp1[2]) && isset($this->languages[$tmp1[2]])){
                $this->language = $tmp1[2];
            }

            $this->old_tele_number                = $this->telephoneNumber;
        }
        $this->old_dn = $this->dn;
        $this->old_cn = $this->cn;
        $this->old_base = $this->base;
        $this->orig_base = $this->base;
        $this->orig_dn = $this->dn;

        /* Instanciate base selector */
        $this->baseSelector= new baseSelector($this->get_allowed_bases(), $this->base);
        $this->baseSelector->setSubmitButton(false);
        $this->baseSelector->setHeight(300);
        $this->baseSelector->update(true);
    }


    function execute()
    {
        /* Call parent execute */
        plugin::execute();

        /* Log last action */
        if($this->is_account && !$this->view_logged){
            $this->view_logged = TRUE;
            new log("view","gofonconference/".get_class($this),$this->dn);
        }

        $smarty= get_smarty();


        foreach ($this->attributes as $val){
            $smarty->assign("$val", set_post($this->$val));
            if(!$this->$val){
                $smarty->assign($val."CHK", "");
            }else{
                $smarty->assign($val."CHK", " checked ");
            }
        }

        /* Create array with goFonHomeServer */
        $tmp = array();
        foreach($this->goFonHomeServers as $dn => $val){
            if(!is_numeric($dn)){
                $tmp[$dn]  = $val['SERVER'];
            }
        }
        $smarty->assign("language", $this->language);
        $smarty->assign("languages",$this->languages);
        $smarty->assign("goFonHomeServers",set_post($tmp));
        $smarty->assign("goFonConferenceOptions",		    array("D"=>"Conference ","d"=>"Conference without PIN"));
        $smarty->assign("goFonConferenceOptionFormats",	array("WAV"=>"Wave","GSM"=>"GSM","WAV49"=>"Wave49"));
        $smarty->assign("goFonConferenceOption",        $this->goFonConferenceOption_D);

        $tmp = $this->plInfo();
        foreach($tmp['plProvidedAcls'] as $name => $translation){
            $smarty->assign($name."ACL",$this->getacl($name));
        }

        $smarty->assign("base"  ,$this->baseSelector->render());

        if(session::get('js')==1){
            if($this->goFonConferenceOption_P != "P"){
                $smarty->assign("goFonPINACL", $this->getacl("goFonPIN",TRUE));
                $smarty->assign("goFonPIN","");
            }
            if($this->goFonConferenceOption_r != "r"){
                $smarty->assign("goFonConferenceOptionFormatACL", $this->getacl("goFonConferenceOptionr",TRUE));
            }
        }
        return($smarty->fetch (get_template_path('generic.tpl', TRUE)));
    }


    function remove_from_parent()
    {
        /* Check if 'old' home server is available in gosa FON server configuration 
         * Try to remove this entry from database and display errors.  
         */
        if(isset($this->goFonHomeServers[$this->goFonHomeServer])){
            $str = $this->SQL_remove_me(true); 
            if($str){
                msg_dialog::display(_("Error"), $str, ERROR_DIALOG);
                return false;
            }
        }else{
            msg_dialog::display(_("Error"), sprintf(_("Cannot delete entry from server '%s' because it seems to be removed!"), preg_replace("/,/",", ",$this->goFonHomeServer)), ERROR_DIALOG);
            return false;
        }

        /* Remove ldap entry */
        $ldap= $this->config->get_ldap_link();
        $ldap->cd ($this->dn);
        $ldap->recursive_remove();

        new log("remove","gofonconference/".get_class($this),$this->dn,array_keys($this->attrs),$ldap->get_error());

        /* Optionally execute a command after we're done */
        $this->handle_post_events('remove');
    }


    /* Save data to object */
    function save_object()
    {
        if(isset($_POST['phoneConferenceGeneric'])){

            /* Get selected language */
            if($this->acl_is_writeable("language") && isset($_POST['language']) && isset($this->languages[get_post('language')])){
                $this->language = get_post('language');
            }

            /* Create a base backup and reset the
               base directly after calling plugin::save_object();
               Base will be set seperatly a few lines below */
            $base_tmp = $this->base;
            plugin::save_object();
            $this->base = $base_tmp;

            /* Refresh base */
            if ($this->acl_is_moveable($this->base)){
                if (!$this->baseSelector->update()) {
                    msg_dialog::display(_("Error"), msgPool::permMove(), ERROR_DIALOG);
                }
                if ($this->base != $this->baseSelector->getBase()) {
                    $this->base= $this->baseSelector->getBase();
                    $this->is_modified= TRUE;
                }
            }

            foreach(
                    array("goFonConferenceOption_P","goFonConferenceOption_r",
                        "goFonConferenceOption_M","goFonConferenceOption_s",
                        "goFonConferenceOption_i","goFonConferenceOption_c",
                        "goFonConferenceOption_D") as $attrs){

                /* Acl can't contain _ so we remove it here. */
                $acl_name = preg_replace("/_/","",$attrs);

                if($this->acl_is_writeable($acl_name)){

                    if(isset($_POST[$attrs])){
                        $this->$attrs = get_post($attrs);
                    }else{
                        $this->$attrs = false;
                    }
                }
            }
        }
    }


    function check_database_accessibility()
    {
        /* Check if mysql extension is available */
        if(!is_callable("mysql_pconnect")){
            return(msgPool::missingext("php-mysql"));
        }

        /********************
         * Check currently selected home server 
         ********************/

        $cfg_Current  = $this->goFonHomeServers[$this->goFonHomeServer];
        $r_current    =  @mysql_pconnect($cfg_Current['SERVER'],$cfg_Current['LOGIN'],$cfg_Current['PASSWORD']);
        if(!$r_current){
            new log("debug","gofonconference/".get_class($this),"",array(),@mysql_error($r_current));
            return(msgPool::dbconnect("GOfon",@mysql_error($r_current), $cfg_Current['SERVER']));
        }
        $db_current  =  @mysql_select_db($cfg_Current['DB'],$r_current);
        if(!$db_current){
            new log("debug","gofonconference/".get_class($this),"",array(),@mysql_error($r_current));
            mysql_close($r_current);
            return(msgPool::dbselect("GOfon",@mysql_error($r_current), $cfg_Current['DB']));
        }

        /********************
         * Check init home server 
         ********************/

        if($this->goFonHomeServers != $this->init_HomeServer){
            $cfg_Init  = $this->goFonHomeServers[$this->init_HomeServer] ;
            $r_init    =  @mysql_pconnect($cfg_Init['SERVER'],$cfg_Init['LOGIN'],$cfg_Init['PASSWORD']);
            if(!$r_init){
                new log("debug","gofonconference/".get_class($this),"",array(),@mysql_error($r_init));
                return(msgPool::dbconnect("GOfon",@mysql_error($r_current), $cfg_Init['SERVER']));
            }
            $db_init  =  @mysql_select_db($cfg_Init['DB'],$r_init);
            if(!$db_init){
                new log("debug","gofonconference/".get_class($this),"",array(),@mysql_error($r_init));
                mysql_close($r_init);
                return(msgPool::dbselect("GOfon",@mysql_error($r_current), $cfg_Init['DB']));
            }
        }
    }

    /* Check values */
    function check()
    {
        /* Call common method to give check the hook */
        $message= plugin::check();

        if($this->is_number_used()){
            $message[] =  $this->is_number_used();
        }

        /* Check if previously selected server is still available */
        if($this->initially_was_account && !isset($this->goFonHomeServers[$this->goFonHomeServer])){
            $message[]= sprintf(_("The previously selected asterisk home server '%s' is no longer available!"),preg_replace("/,/",", ",$this->goFonHomeServer));
            return($message);
        }

        if((empty($this->goFonPIN))&&($this->goFonConferenceOption_P=="P")&&($this->goFonConferenceOption_D=="D")){
            $message[]= msgPool::required(_("PIN"));
        }

        // Check if a wrong base was supplied
        if(!$this->baseSelector->checkLastBaseUpdate()){
            $message[]= msgPool::check_base();;
        }

        if($this->cn == ""){
            $message[] = msgPool::required(_("Name"));
        }

        if(!is_numeric($this->telephoneNumber)){
            $message[] = msgPool::invalid(_("Number"),$this->telephoneNumber,"/[0-9]/");
        }

        if(!((is_numeric($this->goFonConferenceOptionLifetime))||(empty($this->goFonConferenceOptionLifetime)))){
            $message[] = msgPool::invalid(_("Lifetime"),$this->goFonConferenceOptionLifetime,"/[0-9]/");
        }

        /* Check if add could be successful */    
        $str = $this->SQL_add_me(false);
        if(!empty($str)){
            $message[] = $str;
        }

        if($this->old_cn != $this->cn || $this->base != $this->old_base){
            $ldap = $this->config->get_ldap_link();
            $ldap->cd(get_ou("conference", "phoneConferenceRDN").$this->base);
            $ldap->search("(&(objectClass=goFonConference)(cn=".$this->cn."))",array("cn"));
            if($ldap->count()){
                $message[] = msgPool::duplicated(_("Name"));
            }
        }

        /* Check if we are allowed to create or move this object
         */
        if($this->orig_dn == "new" && !$this->acl_is_createable($this->base)){
            $message[] = msgPool::permCreate();
        }elseif($this->orig_dn != "new" && $this->base != $this->orig_base && !$this->acl_is_moveable($this->base)){
            $message[] = msgPool::permMove();
        }

        return $message;
    }


    function SQL_add_me($save)
    {
        /* Check if there is at least on server configuration */
        if(!count($this->goFonHomeServers)){
            return(msgPool::noserver(_("GOfon")));
        }

        /********************
         * Get configuration and try to connect 
         ********************/

        /* Check if databases are reachable, returns an error string if anything fails  */
        $error_str = $this->check_database_accessibility();
        if($error_str){
            return($error_str);
        }

        /* Remove old entries, returns an error string if anything fails  */
        $error_str = $this->SQL_remove_me($save);
        if($error_str){
            return($error_str);
        }

        /* Connect to current database to be able to add new entries */
        $SQL=array();
        $cfg_Current  = $this->goFonHomeServers[$this->goFonHomeServer] ;
        $res_cur      =  @mysql_pconnect($cfg_Current['SERVER'],$cfg_Current['LOGIN'],$cfg_Current['PASSWORD']);
        $db_cur       =  @mysql_select_db($cfg_Current['DB'],$res_cur);

        /********************
         * Remove entries that could cause trouble  
         ********************/


        /* Check table definitions
         */
        if($save && !phoneAccount::checkRealtimeTables($cfg_Current)){
            msg_dialog::display(_("Warning"),
                    sprintf(_("GOsa identified problems with your MySQL table definition, please activate debugging for details.")),
                    WARNING_DIALOG);
        }


        /* If the current home server is different to the initial home server,
         *  there may be already some entries with the given telephoneNumber and/or cn.
         * We must remove those entries to avoid duplicate use of the same extension name.
         */
        if($this->goFonHomeServer != $this->init_HomeServer){
            $query = "SELECT id FROM ".$cfg_Current['EXT_TABLE']." WHERE exten='".mysql_real_escape_string($this->telephoneNumber)."' OR exten='".$this->cn."';";
            $res = @mysql_query($query,$res_cur);
            @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,$query, "Database query");
            if(!$res){
                new log("debug","gofonconference/".get_class($this),"",array(),@mysql_error($res_cur));
                return(msgPool::dbquery("GOfon", @mysql_error($res_cur),$cfg_Current['SERVER']));
            }
            if($save && mysql_affected_rows($res_cur)) {
                $SQL[] = "DELETE FROM ".$cfg_Current['EXT_TABLE']." 
                    WHERE   (exten='".mysql_real_escape_string($this->telephoneNumber)."') 
                    OR    (exten='".mysql_real_escape_string($this->cn)."')";
            }
        }

        /********************
         * Add new conference entry  
         ********************/
        if((!empty($this->telephoneNumber))&&($save==true)){

            /* Create string out of conference Flags */
            $parameter  ="";
            foreach(array("goFonConferenceOption_P","goFonConferenceOption_r","goFonConferenceOption_M","goFonConferenceOption_s",
                        "goFonConferenceOption_i","goFonConferenceOption_c","goFonConferenceOption_D") as $attrs){
                $parameter .= $this->$attrs;
            }

            /* 
             * ringing
             * set language()=...
             * wait 2
             * answer
             * wait 2
             * meetme...
             * hangup
             * */

            $i=1;
            $EXT=array();
            $context="GOsa";

            // Ringing
            $EXT[$i]['exten']   =$this->telephoneNumber;
            $EXT[$i]['context'] =$context;
            $EXT[$i]['priority']=$i;
            $EXT[$i]['app']     ="ringing";
            $EXT[$i]['appdata'] ="";
            $i++;      

            // Set Language to German
            $EXT[$i]['exten']   =$this->telephoneNumber;
            $EXT[$i]['context'] = $context;
            $EXT[$i]['priority']= $i;
            $EXT[$i]['app']     ="Set";
            $EXT[$i]['appdata'] ="CHANNEL(language)=".$this->language;
            $i++;      

            // Recordingformat for conference
            if($this->goFonConferenceOption_r == "r"){
                $EXT[$i]['exten']   =$this->telephoneNumber;
                $EXT[$i]['context'] =$context;
                $EXT[$i]['priority']= $i;
                $EXT[$i]['app']     ="Set";
                $EXT[$i]['appdata'] ="MEETME_RECORDINGFORMAT=".$this->goFonConferenceOptionFormat;
                $i++;      
            }

            // Wait for 2 seconds
            $EXT[$i]['exten']   =$this->telephoneNumber;
            $EXT[$i]['context'] =$context;
            $EXT[$i]['priority']=$i;
            $EXT[$i]['app']     ="wait";
            $EXT[$i]['appdata'] ="2";
            $i++;      

            // Answer Call
            $EXT[$i]['exten']   =$this->telephoneNumber;
            $EXT[$i]['context'] =$context;
            $EXT[$i]['priority']=$i;
            $EXT[$i]['app']     ="answer";
            $EXT[$i]['appdata'] ="";
            $i++;      

            // Wait for 2 seconds
            $EXT[$i]['exten']   =$this->telephoneNumber;
            $EXT[$i]['context'] =$context;
            $EXT[$i]['priority']=$i;
            $EXT[$i]['app']     ="wait";
            $EXT[$i]['appdata'] ="2";
            $i++;      

            // Start Conference 
            $EXT[$i]['exten']   =$this->telephoneNumber;
            $EXT[$i]['context'] =$context;
            $EXT[$i]['priority']=$i;
            $EXT[$i]['app']     ="MeetMe";
            if(empty($this->goFonPIN)) {
                $EXT[$i]['appdata'] =$this->telephoneNumber."|".$parameter;
            }else{
                $EXT[$i]['appdata'] =$this->telephoneNumber."|".$parameter."|".$this->goFonPIN;
            }
            $i++; 

            // Hangup command
            $EXT[$i]['exten']   =$this->telephoneNumber;
            $EXT[$i]['context'] =$context;
            $EXT[$i]['priority']=$i;
            $EXT[$i]['app']     ="hangup";
            $EXT[$i]['appdata'] ="";
            $i++;      

            // Map name to number 
            if(!is_numeric($this->cn)){
                $EXT[$i]['exten']   =$this->cn;
                $EXT[$i]['context'] =$context;
                $EXT[$i]['priority']=1;
                $EXT[$i]['app']     ="Goto";
                $EXT[$i]['appdata'] =$this->telephoneNumber."|1";
            }        

            foreach($EXT as $keytop => $valtop){
                $s_keys = "";
                $s_values = ""; 
                foreach($valtop as $key=>$val){
                    $s_keys   .="`".$key."`,";
                    $s_values .="'".mysql_real_escape_string($val)."',";
                }
                $s_keys   =preg_replace("/\,$/","",$s_keys); 
                $s_values =preg_replace("/\,$/","",$s_values); 
                $SQL[]="INSERT INTO ".$cfg_Current['EXT_TABLE']." (".$s_keys.") VALUES (".$s_values.");"; 
            }

            /* Start transaction, to be able to rollback
             */
            @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,"<b>---Updating/Inserting entries---</b>","");

            mysql_query("begin;",$res_cur);
            @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,"<b>begin;</b>","<i>Starting transaction!</i>");

            foreach($SQL as $query){
                @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,"<b>".$query."</b>", "");
                if(!mysql_query($query,$res_cur)){
                    $err = mysql_error($res_cur);
                    @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,"\n".$err, "<b>FAILED</b>");
                    msg_dialog::display(_("Error"),
                            msgPool::mysqlerror($err,__CLASS__)."&nbsp;".
                            "\n<p>"._("Please activate debugging for details!")."</p>",
                            ERROR_DIALOG);

                    mysql_query("rollback;",$res_cur);
                    @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,"<b>rollback;</b>", "<b>ERROR</b> Rollback transaction!");
                    @mysql_close($res_cur);
                    return(false);
                }
            }

            /* Let changes get active, everything was fine;
             */
            mysql_query("commit;",$res_cur);
            @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,"<b>commit;</b>", "");
            @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,"<b>---Transaction sucessful!---</b>", "");
        } 

        @mysql_close($res_cur);
    }


    /* Remove initial entry from database 
     * This function checks if there is an entry in the 
     *  initial home server that uses this->old_cn or $this->old_tele_number
     *  and removes this entries. 
     * This function is called from save and remove_from parent.
     * 
     *  The parameter '$save' is false if we just 
     *   want to check if a remove is possible. 
     *  And true if we realy want to remove the entries.   
     */
    function SQL_remove_me($save)
    {
        /* check database access */
        $str = $this->check_database_accessibility();
        if($str){
            return($str);
        }

        /* Connect to old database */
        $cfg_Init  = $this->goFonHomeServers[$this->init_HomeServer] ;
        $r_init    =  @mysql_pconnect($cfg_Init['SERVER'],$cfg_Init['LOGIN'],$cfg_Init['PASSWORD']);
        $db_init   =  @mysql_select_db($cfg_Init['DB'],$r_init);

        /* Check if there is an old entry */
        $query = "SELECT id FROM ".$cfg_Init['EXT_TABLE']." WHERE exten='".mysql_real_escape_string($this->old_tele_number)."' OR exten='".mysql_real_escape_string($this->old_cn)."';";
        $res = @mysql_query($query,$r_init);
        @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,$query, "Database query");
        if(!$res){
            new log("debug","gofonconference/".get_class($this),"",array(),@mysql_error($r_init));
            return(msgPool::dbquery("GOfon", @mysql_error($res_cur),$cfg_Init['SERVER']));
        }

        /* There are entries using this cn and/or phone number */
        if($save && mysql_affected_rows($r_init)) {
            $SQL = "DELETE FROM ".$cfg_Init['EXT_TABLE']." 
                WHERE   (exten='".mysql_real_escape_string($this->old_tele_number)."') 
                OR    (exten='".mysql_real_escape_string($this->old_cn)."')";

            /* Query and ensure that everything went fine */
            $res =  @mysql_query($SQL,$r_init);
            @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,$SQL, "Database query");
            if(!$res){
                new log("debug","gofonconference/".get_class($this),"",array(),@mysql_error($r_init));
                return(msgPool::dbquery("GOfon", @mysql_error($res_cur),$cfg_Init['SERVER']));
            }

        }//ENDE  old num availiable ...
        @mysql_close($r_init);
        return(false);
    }



    /* This function checks if the given phonenumbers are available or already in use*/
    function is_number_used()
    {
        $ldap= $this->config->get_ldap_link();
        $ldap->cd($this->config->current['BASE']);
        $ldap->search("(|(objectClass=goFonAccount)(objectClass=goFonQueue)(objectClass=goFonConference))", array("telephoneNumber","cn","uid"));
        while($attrs = $ldap->fetch()) {
            unset($attrs['telephoneNumber']['count']);
            foreach($attrs['telephoneNumber'] as $tele){
                if(!isset($attrs['cn'][0])) $attrs['cn'][0]=$attrs['dn'];
                if(!isset($attrs['uid'][0])) $attrs['uid'][0]=$attrs['dn'];
                $numbers[$tele]=$attrs;
            }
        }

        $num = $this->telephoneNumber;
        if((isset($numbers[$num]))&&(($numbers[$num]['cn'][0]!=$this->old_cn))){
            if(isset($numbers[$num]['uid'][0])){
                return sprintf(_("'%s' is already assigned to '%s'!"),$num,$numbers[$num]['uid'][0]);
            }else{
                return sprintf(_("'%s' is already assigned to '%s'!"),$num,$numbers[$num]['cn'][0]);
            }
        }
    }


    /* Save to LDAP */
    function save()
    {

        if((!isset($this->attrs['goFonPIN']))){
            $pin_use = false;
        }else{
            $pin_use = true;
        }

        /* Unset PIN if this is a conference without PIN */
        if((!empty($this->goFonPIN)||($this->goFonConferenceOption_P=="P"))&&($this->goFonConferenceOption_D=="d")){
            $this->goFonPIN = "";
        }

        plugin::save();

        if(empty($this->old_tele_number)){
            $this->old_tele_number= $this->telephoneNumber;
        }

        $this->SQL_add_me(true);

        if(empty($this->goFonConferenceOption_P)){
            if($pin_use){
                $this->attrs['goFonPIN']=array();
            }else{
                unset($this->attrs['goFonPIN']);
            }
        }
        $this->attrs['goFonConferenceOption']="";
        foreach(array("goFonConferenceOption_P","goFonConferenceOption_r","goFonConferenceOption_M","goFonConferenceOption_s",
                    "goFonConferenceOption_i","goFonConferenceOption_c","goFonConferenceOption_D","goFonConferenceOptionFormat") as $attrs){
            $this->attrs['goFonConferenceOption'] .= $this->$attrs;
            unset($this->attrs[$attrs]);         
        }

        $this->attrs['goFonConferenceOption'].="|".$this->goFonConferenceOptionLifetime;
        $this->attrs['goFonConferenceOption'].="|".$this->language;
        unset($this->attrs['goFonConferenceOptionLifetime']);

        /* Write back to ldap */

        unset($this->attrs['base']);

        $ldap= $this->config->get_ldap_link();
        $ldap->cd($this->config->current['BASE']);
        $ldap->cat($this->dn, array('dn'));

        if ($ldap->count()){
            $ldap->cd($this->dn);
            $this->cleanup();
            $ldap->modify ($this->attrs); 
            $mode = 'modify';
        } else {
            $ldap->create_missing_trees(preg_replace('/^[^,]+,/', '', $this->dn));
            $ldap->cd($this->dn);
            $ldap->add($this->attrs);
            $mode = 'add';
        }

        /* Log last action */
        if($this->initially_was_account){
            new log("modify","gofonconference/".get_class($this),$this->dn,array_keys($this->attrs),$ldap->get_error());
        }else{
            new log("create","gofonconference/".get_class($this),$this->dn,array_keys($this->attrs),$ldap->get_error());
        }

        if (!$ldap->success()){
            msg_dialog::display(_("LDAP error"), msgPool::ldaperror($ldap->get_error(), $this->dn, 0, get_class()));
        }else{
            $this->handle_post_events($mode);
        }
    }


    function getCopyDialog()
    {
        $smarty = get_smarty();
        $smarty->assign("cn" ,set_post($this->cn));
        $smarty->assign("telephoneNumber" ,set_post($this->telephoneNumber));
        $str = $smarty->fetch(get_template_path("paste_generic.tpl",TRUE));
        $ret = array();
        $ret['string'] = $str;
        $ret['status'] = "";
        return($ret);
    }


    function saveCopyDialog()
    {
        if(isset($_POST['cn'])){
            $this->cn = get_post('cn');
        }
        if(isset($_POST['telephoneNumber'])){
            $this->telephoneNumber = get_post('telephoneNumber');
        }
    }

    function PrepareForCopyPaste($source)
    {
        plugin::PrepareForCopyPaste($source) ;

        $source_o = new conference($this->config,$source['dn']);

        foreach($this->attributes as $attr){
            $this->$attr = $source_o->$attr;
        }
    }


    /* Return plugin informations for acl handling */
    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Conference"),
                    "plDescription" => _("Phone conference management"),
                    "plSelfModify"  => TRUE,
                    "plDepends"     => array(),
                    "plPriority"    => 0,
                    "plSection"     => array("administration"),
                    "plRequirements"=> array(
                        'ldapSchema' => array('goFonConference' => '>=2.7'),
                        'onFailureDisablePlugin' => array(get_class(),'phoneAccount','phoneConferenceManagment','goFonMacro')
                        ),
                    "plCategory"    => array("gofonconference" => array("description" => _("GOfon conference"),
                            "objectClass" => "goFonConference")),

                    "plProperties" =>
                    array(
                        array(
                            "name"          => "phoneConferenceRDN",
                            "type"          => "rdn",
                            "default"       => "ou=conferences,ou=asterisk,ou=configs,ou=systems,",
                            "description"   => _("RDN for phone conference storage."),
                            "check"         => "gosaProperty::isRdn",
                            "migrate"       => "migrate_phoneConferenceRDN",
                            "group"         => "plugin",
                            "mandatory"     => FALSE
                            )
                        ),


            "plProvidedAcls" => array(
                    "cn"                            => _("Name"),
                    "base"                          => _("Base"),
                    "description"                   => _("Description"),
                    "goFonPIN"                      => _("Conference PIN"),

                    "language"                      => _("Language"),

                    "goFonHomeServer"              => _("Home server"),
                    "goFonConferenceOptionP"       => _("Preset PIN"),
                    "goFonConferenceOptionr"       => _("Record conference"),
                    "goFonConferenceOptionM"       => _("Play music on hold"),
                    "goFonConferenceOptions"       => _("Activate menu"),
                    "goFonConferenceOptioni"       => _("Announce user activity"),
                    "goFonConferenceOptionc"       => _("Count user"),
                    "goFonConferenceOptionD"       => _("Conference type"),

                    "goFonConferenceOptionFormat"   => _("Format"),
                    "goFonConferenceOptionLifetime" => _("Lifetime"),
                    "telephoneNumber"               => _("Telephone number"),
                    "goFonConferenceOwner"          => _("Owner"))
                        ));
    }


}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
