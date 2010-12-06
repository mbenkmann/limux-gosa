<?php

//!  The Phone Macro Class: Handles Macro Contents, and some attributes. 
/*!
  This class handles the basic information about phone macros, like
  cn base description displayName goFonMacroContent goFonMacroVisible

  This is not the only Class that manages phone Macros, there ist also the class_goFonMacroParameter.
 */
class macro extends plugin
{
    /*! Macro attributes,  */
    var $generate_error= "";

    /*! The name of the Macro in the openldap drirectory */
    var $cn               = ""; 

    /*! Display error once */
    var $error_shown = false; 

    /*! This ist the variable that contains the description of the macro*/
    var $description      = "";

    /*! The base of the macro, is used to save the macro in the correct directory tree */
    var $base             = "";

    /*! This is the name of the macro which the enduser will see, instead of the cn */
    var $displayName      = "";

    /*! Here is the macro content, the real macroscript */
    var $goFonMacroContent= "";

    /*! To allow user to use this macro this var must be true, else false */
    var $goFonMacroVisible= 0;

    /*! attribute list for save action */
    var $attributes     = array("cn","base", "description","displayName","goFonMacroContent","goFonMacroVisible");
    var $view_logged = FALSE;
    var $orig_cn = ""; 
    var $orig_base = ""; 
    var $orig_dn = ""; 
    /*! Objectclasses that this calls handles */
    var $objectclasses  = array("top", "goFonMacro");

    var $goFonHomeServers = array(); // Contains all available asterisk database server 
    var $baseSelector;

    //! The Konstructor   
    /*!  Konstructor, load class with  attributes of the given dn*/
    function macro (&$config, $dn= NULL, $parent= NULL)
    {
        plugin::plugin ($config, $dn, $parent);

        /* This is always an account */
        $this->is_account= TRUE;

        /* Edit or new one ?*/
        if ($this->dn == "new"){
            if(session::is_set('CurrentMainBase')){
                $this->base  = session::get('CurrentMainBase');
            }else{
                $ui= get_userinfo();
                $this->base= dn2base($ui->dn);
            }
        } else {
            $this->orig_cn=$this->cn;
            $this->base= preg_replace ("/^[^,]+,".preg_quote(get_ou("macro", "phoneMacroRDN"), '/')."/i", "", $this->dn);
        }

        /* Check server configurations
         * Load all server configuration in $this->goFonHomeServers if available
         */
        $a_SETUP= array();
        if(isset($config->data['SERVERS']['FON'])){

            /* Set available server */
            $this->goFonHomeServers = $config->data['SERVERS']['FON'];

            /* Remove default entry, not necessary here */
            if(isset($this->goFonHomeServers[0])){
                unset($this->goFonHomeServers[0]);  
            }
        }

        $this->orig_base = $this->base;
        $this->orig_dn = $this->dn;

        /* Instanciate base selector */
        $this->baseSelector= new baseSelector($this->get_allowed_bases(), $this->base);
        $this->baseSelector->setSubmitButton(false);
        $this->baseSelector->setHeight(300);
        $this->baseSelector->update(true);
    }


    /*!  Execute this plugin */
    function execute()
    {
        /* Call parent execute */
        plugin::execute();

        /* Log view */
        if($this->is_account && !$this->view_logged){
            $this->view_logged = TRUE;
            new log("view","gofonmacro/".get_class($this),$this->dn);
        }

        /* Variables */
        $tmp  	= array();
        $number = 0; 

        /* Fill templating stuff */
        $smarty= get_smarty();


        $tmp = $this->plInfo();
        foreach($tmp['plProvidedAcls'] as $name => $translation){
            $smarty->assign($name."ACL",$this->getacl($name));
        }

        if($this->acl_is_writeable("base")){
            $smarty->assign("baseSelect",true);
        }else{
            $smarty->assign("baseSelect",false);
        }


        /* Assign all vars to Smarty */
        foreach($this->attributes as $ar){
            $smarty->assign($ar, set_post($this->$ar));
        }

        $smarty->assign("base", $this->baseSelector->render());

        if($this->goFonMacroVisible){
            $smarty->assign("goFonMacroVisibleChecked"," checked ");
        }else{
            $smarty->assign("goFonMacroVisibleChecked","");
        }

        $smarty->assign("cnACL",$this->getacl("cn",$this->initially_was_account));
        $smarty->assign("cn",set_post($this->cn));

        /* Ensure that macro content is displayed correctly encoded */
        $smarty->assign("goFonMacroContent",set_post($this->goFonMacroContent));

        /* Show main page */
        return($smarty->fetch (get_template_path('generic.tpl', TRUE)));
    }


    /* This method check if all databases are reachable.  
     *  Returns with error message or an empty string on success.
     * 
     * - Is mysql extension available  
     * - Is every server reachable 
     * - Does the database exists/is accessible
     */
    function check_database_accessibility()
    {
        /* Check if mysql extension is available */
        if(!is_callable("mysql_pconnect")){
            return(msgPool::missingext("php-mysql"));
        }

        /********************
         * Check all home server
         ********************/
        foreach($this->goFonHomeServers as $goFonHomeServer => $cfg_Current){
            $r_current    =  @mysql_pconnect($cfg_Current['SERVER'],$cfg_Current['LOGIN'],$cfg_Current['PASSWORD']);
            if(!$r_current){
                new log("debug","gofonmacro/".get_class($this),"",array(),@mysql_error($r_current));
                return ( msgPool::dbconnect("GOfon",@mysql_error(),$cfg_Current['SERVER']));
            }
            $db_current  =  @mysql_select_db($cfg_Current['DB'],$r_current);
            if(!$db_current){
                new log("debug","gofonmacro/".get_class($this),"",array(),@mysql_error($r_current));
                mysql_close($r_current);
                return ( msgPool::dbselect("GOfon",@mysql_error(),$cfg_Current['DB']));
            }
        }
    }


    /* Remove current macro from all asterisk server.
     * First of all check if we have access to all databases. 
     * - Remove old entries 
     */ 
    function remove_from_database($save)
    {
        /* Check if all databases are reachable */
        $str = $this->check_database_accessibility();
        if($str){
            return($str);
        }

        /* Create query string */
        $context  = addslashes("macro-".$this->cn);

        /* Remove current macro from each server available */ 
        if($save){
            foreach($this->goFonHomeServers as $dn => $Server){
                $query      = "DELETE FROM ".$Server['EXT_TABLE']." WHERE context='".mysql_real_escape_string($context)."';";
                $r_current  =  @mysql_pconnect($Server['SERVER'],$Server['LOGIN'],$Server['PASSWORD']);
                $db_current =  @mysql_select_db($Server['DB'],$r_current);
                $res = @mysql_query($query,$r_current);
                @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,$query, "Database query");
                if(!$res){
                    new log("debug","gofonmacro/".get_class($this),"",array(),@mysql_error($r_current));
                    @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,"<b>".$query."</b>", "ERROR");
                    trigger_error(mysql_error($r_current));
                    return(
                            sprintf(
                                _("Macro could not be removed from server '%s', please check database consistency!"), 
                                $Server['SERVER']));
                }
                @mysql_close($r_current);
            }
        }
    }


    /* Add current macro to all asterisk server.
     * First of all check if we have access to all databases. 
     * - Remove old entries 
     * - Add new entries 
     */ 
    function add_to_database($save)
    {
        /* Check if all databases are reachable */
        $str = $this->check_database_accessibility();
        if($str){
            return($str);
        }

        /* Remove old entries first. Else we got duplicated entries */
        $str = $this->remove_from_database($save);
        if($str){
            return($str);
        }

        /* Create query string */
        $context      = "macro-".$this->cn;

        /* Macro are spread to each asterisk server */
        foreach($this->goFonHomeServers as $dn => $cfg){
            $r_con  = @mysql_pconnect($cfg['SERVER'],$cfg['LOGIN'],$cfg['PASSWORD']); 
            $db     = @mysql_select_db($cfg['DB'],$r_con);

            /************
             * Parse Macro content
             ************/
            $sql = 
                "INSERT INTO %TABLENAME% ".
                " (context,exten,priority,app,appdata) ".
                " VALUES ";

            $a_contentLines = preg_split("/\n/",$this->goFonMacroContent);

            // Keep all macro priorities to avoid duplicates!
            $priorities = array();
            foreach($a_contentLines as $i_linenum => $s_linestr){

                /* Remove the 'exten => ' string in front of the macro content line 
                 *  example line  'exten => s,2,GotoIf(${ARG3}?3:5)'
                 * Remove comments introduced by ;
                 * Skip empty lines 
                 */ 
                $s_linestr = trim($s_linestr);
                $s_linestr = preg_replace("/;.*$/","",$s_linestr) ;
                $s_linestr = preg_replace ("/^.*=\> /","",$s_linestr);

                if(empty($s_linestr)){
                    continue;
                }

                /* A line that passes the check above should look like this 
                 *  s,1,SetLanguage(de)
                 * 3 parts seperated by , 
                 * If there are more or less parts, abort.
                 * The preg_replace exclude parameters from split .. 
                 */
                $tmp  = explode(",", $s_linestr,3);

                /* Check if there is an application given */
                if(!isset($tmp[2]) || $tmp[2]  == ""){
                    return(sprintf(_("Application missing in line %s!"),$i_linenum));
                } 
                if(!isset($tmp[1]) || $tmp[1] == ""){
                    return(sprintf(_("Priority missing in line %s!"),$i_linenum));
                } 
                /* Check if there is an extension given */
                if(!isset($tmp[0]) || $tmp[0] == ""){
                    return(sprintf(_("Extension missing in line %s!"),$i_linenum));
                } 

                // Table 'extensions' has a primary key over the following fields 
                //  'context', 'exten' and 'priority' - 
                // Ensure that we do not write duplicates here.
                $du = $context.$tmp[0].$tmp[1];
                if(!isset($priorities[$du])){
                    $priorities[$du] = 1;
                }else{
                    return(sprintf(_("Duplicate combination of 'extension' and 'priority' in line %s!"),$i_linenum));
                }

                /* Create extension entry for current line 
                 *  and add this line to an array that will be inserted 
                 *  to each database.
                 */
                $context= mysql_real_escape_string($context);
                $exten  = mysql_real_escape_string($tmp[0]);
                $prio   = mysql_real_escape_string($tmp[1]);
                $app    = mysql_real_escape_string(preg_replace("/\(.*\).*$/","",$tmp[2]));
                $para   = mysql_real_escape_string(preg_replace("/^[^(]*\((.*)\)[^)]*$/", "$1", $tmp[2]));
                $sql.= " ('".$context."','".$exten."','".$prio."','".$app."','".$para."'),";
            }

            /* Remove last , from query string */
            $sql = preg_replace("/,$/","",$sql);

            /* Save current changes to the database */
            $query  = preg_replace("/%TABLENAME%/",$cfg['EXT_TABLE'],$sql);
            if($save){
                $res    = @mysql_query($query,$r_con);
                @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,$query, "Database query");
                if(!$res){
                    new log("debug","gofonmacro/".get_class($this),"",array(),@mysql_error($r_con));
                    @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,"<b>".$query."</b>", "ERROR");
                    trigger_error(mysql_error($r_con));
                    return(
                            sprintf(
                                _("Macro could not be added to server '%s', please check database consistency!"), 
                                $cfg['SERVER']));
                }
            }
            @mysql_close($r_con);
        }
    }


    function save_object()
    {
        if (isset($_POST['gofonMacroGenericPosted'])){

            $old_cn       = $this->cn;
            $old_visible  = $this->goFonMacroVisible;

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

            /* Restore old cn if we have insuficient acls to change cn ... */
            if(!$this->acl_is_writeable("cn",$this->initially_was_account)){
                $this->cn = $old_cn;
            }

            /* check if we are allowed to toggle visibility */
            if($this->acl_is_writeable("goFonMacroVisible")) {

                /* Checkbox selected ? */
                if(isset($_POST['goFonMacroVisible'])) {
                    $this->goFonMacroVisible= 1 ;
                }else  {
                    if(isset($_POST['displayName'])){
                        $this->goFonMacroVisible= 0 ;
                    }
                }
            }else{
                $this->goFonMacroVisible = $old_visible;
            }
        }
    }


    /*! Check values */
    function check()
    {
        /* Call common method to give check the hook */
        $message= plugin::check();

        if(!count($this->goFonHomeServers)){
            $message[] = msgPool::noserver(_("GOfon"));
        }

        /* Check if insert/replace is possible and all servers are available */
        $str = $this->add_to_database(false);
        if($str){
            $message[] = $str;
        }

        /* Check if cn is already used  */
        if(($this->dn=="new")||($this->orig_cn!=$this->cn)){
            $ldap = $this->config->get_ldap_link();
            $ldap->search("(&(objectClass=goFonMacro)(cn=".$this->cn."))",array("cn"));
            if($ldap->count()>0){
                $message[]= msgPool::duplicated(_("Name"));
            }
        }

        // Check if a wrong base was supplied
        if(!$this->baseSelector->checkLastBaseUpdate()){
            $message[]= msgPool::check_base();;
        }

        /* Check if display name is set */
        if($this->displayName == ""){
            $message[] = msgPool::required(_("Name"));
        }  
        /* CN is restricted to 20 chars */
        if(strlen("Makro-".$this->cn)>20 ){
            $message[]=_("Name can be 20 characters at maximum!");
        }

        /* If this macro is still in use we should not change the visible for user flag to invisible */
        if(!$this->goFonMacroVisible){
            $ldap = $this->config->get_ldap_link();
            $res = $ldap->search("(&(objectClass=goFonAccount)(objectClass=gosaAccount)(goFonMacro=*))", array("goFonMacro"));
            while ($val = $ldap->fetch()){
                if(strstr($val['goFonMacro'][0],$this->dn)){
                    $message[] = _("Macro is still in use!");
                    return($message);
                }
            }
        }

        if(empty($this->goFonMacroContent)){
            $message[] = _("Macro is empty!");
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


    /*! Remove makro from all given databases 
     *   and ldap too.
     */
    function remove_from_parent()
    {
        $ldap= $this->config->get_ldap_link();

        /* Skip remove if this macro is still in use */
        $res = $ldap->search("(&(objectClass=goFonAccount)(objectClass=gosaAccount)(goFonMacro=*))", array("goFonMacro", "cn"));
        while ($val = $ldap->fetch()){ 
            if(strstr($val['goFonMacro'][0],$this->dn)){ 
                msg_dialog::display(_("Error"), sprintf(_("Cannot delete entry because it is still in use by '%s'!"), $val['cn'][0]), ERROR_DIALOG);
                return false;
            }
        }

        /* Try to remove from database */
        if(count($this->goFonHomeServers)){
            $str = $this->remove_from_database(true);
            if($str){ 
                msg_dialog::display(_("Error"), $str, ERROR_DIALOG);
                return false;
            }
        }else{
            msg_dialog::display(_("Configuration error"), msgPool::noserver(_("GOfon")), WARNING_DIALOG);
            return false;
        }

        /* Remove phone macro */ 
        $ldap->rmDir($this->dn);
        new log("remove","gofonmacro/".get_class($this),$this->dn,array_keys($this->attrs),$ldap->get_error()); 
        if (!$ldap->success()){
            msg_dialog::display(_("LDAP error"), msgPool::ldaperror($ldap->get_error(), $this->dn, LDAP_DEL, get_class()));
        }

        /* Delete references to object groups */
        $ldap->cd ($this->config->current['BASE']);
        $ldap->search ("(&(objectClass=gosaGroupOfNames)(member=".LDAP::prepare4filter($this->dn)."))", array("cn"));
        while ($ldap->fetch()){
            $og= new ogroup($this->config, $ldap->getDN());
            unset($og->member[$this->dn]);
            $og->save ();
            if (!$ldap->success()){
                msg_dialog::display(_("LDAP error"), msgPool::ldaperror($ldap->get_error(), $this->dn, 0, get_class()));
            }
        }
    }


    /*! Save to LDAP */
    function save()
    {
        plugin::save();
        unset($this->attrs['base']);

        /* Try to add entries to databases */
        $str = $this->add_to_database(true);
        if($str){
            msg_dialog::display(_("Error"), $str, ERROR_DIALOG);
        }

        /* Write back to ldap */
        $ldap= $this->config->get_ldap_link();
        $ldap->cat($this->dn, array('dn'));
        $a= $ldap->fetch();

        if (count($a)){
            $ldap->cd($this->dn);
            $this->cleanup();
            $ldap->modify ($this->attrs); 

            $this->handle_post_events("modify");
        } else {
            $ldap->cd($this->config->current['BASE']);
            $ldap->create_missing_trees(preg_replace('/^[^,]+,/', '', $this->dn));
            $ldap->cd($this->dn);
            $ldap->add($this->attrs);
            $this->handle_post_events("add");
        }
        if (!$ldap->success()){
            msg_dialog::display(_("LDAP error"), msgPool::ldaperror($ldap->get_error(), $this->dn, 0, get_class()));
        }

        /* Log last action */
        if($this->initially_was_account){
            new log("modify","gofonmacro/".get_class($this),$this->dn,array_keys($this->attrs),$ldap->get_error());
        }else{
            new log("create","gofonmacro/".get_class($this),$this->dn,array_keys($this->attrs),$ldap->get_error());
        }
    }


    function getCopyDialog()
    {
        $smarty = get_smarty();
        $smarty->assign("cn" ,set_post($this->cn));
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
    }


    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Generic"),
                    "plDescription" => _("Asterisk macro management"),
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 0,
                    "plSection"     => array("administration"),
                    "plRequirements"=> array(
                        'ldapSchema' => array('goFonMacro' => '>=2.7'),
                        'onFailureDisablePlugin' => array(get_class(),'phoneAccount','phoneConferenceManagment','goFonMacro')
                        ),
                    "plCategory"    => array("gofonmacro" => array("description" => _("GOfon macro"),
                            "objectClass" => "goFonMacro")),

                    "plProperties" =>
                    array(
                        array(
                            "name"          => "phoneMacroRDN",
                            "type"          => "rdn",
                            "default"       => "ou=macros,ou=asterisk,ou=configs,ou=systems,",
                            "description"   => _("RDN for macro storage."),
                            "check"         => "gosaProperty::isRdn",
                            "migrate"       => "migrate_phoneMacroRDN",
                            "group"         => "plugin",
                            "mandatory"     => FALSE
                            )
                        ),



            "plProvidedAcls" => array(
                    "cn"                            => _("Macro name"),
                    "base"                          => _("Base"),
                    "description"                   => _("Description"),
                    "displayName"                   => _("Display name"),
                    "goFonMacroContent"             => _("Macro content and parameter"),
                    "goFonMacroVisible"             => _("Visibility flag"))
                ));
    }

}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
