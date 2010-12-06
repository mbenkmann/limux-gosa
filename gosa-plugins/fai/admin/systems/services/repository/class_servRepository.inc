<?php


class servrepository extends goService
{
    /* attribute list for save action */
    //  var $ignore_account         = TRUE;
    var $attributes             = array("FAIrepository");
    var $objectclasses          = array("FAIrepositoryServer");

    /* Repositories */
    var $repositories          = array();
    var $FAIrepository         = array();
    var $conflicts             = array("goKioskService");
    var $DisplayName           = "";
    var $StatusFlag            = "";

    var $view_logged            = FALSE;
    var $fai_activated          = FALSE;

    var $repositoryMangement = NULL;

    function servrepository (&$config, $dn= NULL, $parent= NULL)
    {
        plugin::plugin ($config, $dn,$parent);

        $this->DisplayName = _("Repository service");

        /* Create repositoryMangement */
        $this->repositoryMangement = new repositoryManagement($this->config,$this);

        /* Skip this if fai is deactivated */
        if($this->config->pluginEnabled("faiManagement")){
            $this->fai_activated = TRUE;    
        }else{
            return;
        }

        $this->repositories = array();
        if(isset($this->attrs['FAIrepository'])){
            for($i = 0; $i < $this->attrs['FAIrepository']['count']; $i++){
                $tmp = explode("|",$this->attrs['FAIrepository'][$i]);
                $tmp2 = array();  
                $tmp3 = array();   

                if(isset($tmp[1])){
                    $tmp2['ParentServer'] = $tmp[1];
                    if(empty($tmp[1])){
                        $tmp2['ParentServer'] = "none";
                    }
                }else{
                    $tmp2['ParentServer'] = "none";
                }

                if(isset($tmp[0])){
                    $tmp2['Url']          = $tmp[0];
                }else{
                    $tmp2['Url']          = "";
                }

                if(isset($tmp[2])){
                    $tmp2['Release']      = $tmp[2];
                }else{
                    $tmp2['Release']      = "";
                }

                if(isset($tmp[3])){
                    $tmp3 = explode(",",$tmp[3]);
                    foreach($tmp3 as $sec){
                        $tmp2['Sections'][$sec]=$sec;
                    }    
                }else{
                    $tmp['Section']=array();
                }

                $this->repositories[$tmp[2]]=$tmp2;      
            }
        }
        session::set('RepositoryList', $this->repositories);
    }


    /*! \brief    Act on events frowarded by the repositoryManagement
     */
    function newEntry()
    {
        if(!$this->dialog){
            $this->dialog = new servRepositorySetup($this->config,$this->dn);
            $this->dialog->parent = $this;
        }
    }


    /*! \brief    Act on events frowarded by the repositoryManagement
     */
    function removeEntry($action,$target,$all)
    {
        foreach($target as $value){

            if(!isset($this->repositories[$value])) continue;

            $url = $this->repositories[$value]['Url'];
            $release = $this->repositories[$value]['Release'];

            $ldap = $this->config->get_ldap_link();
            $ldap->cd ($this->config->current['BASE']);

            $found= false;
            $auto= false;
            $obj= array();
            $aobj= array();

            $ldap->search("(&(objectClass=gotoWorkstation)(objectClass=FAIobject)(|(FAIdebianMirror=".$url.")(FAIdebianMirror=auto)))",array("cn","FAIclass", "FAIdebianMirror"));
            if ($ldap->count() != 0){
                while($attrs = $ldap->fetch()){
                    if (!isset($attrs['FAIclass'])){
                        continue;
                    }
                    foreach($attrs['FAIclass'] as $class){
                        if(preg_match("/:".str_replace("/","\/",$release)."$/i",$class)){
                            if (isset($attrs['FAIdebianMirror'][0]) && $attrs['FAIdebianMirror'][0] == "auto") {
                                $aobj[$ldap->getDN()]= $attrs['cn'][0];
                                $auto= true;
                            } else {
                                $obj[$ldap->getDN()]= $attrs['cn'][0];
                                $found= true;
                            }
                        }
                    }
                }
            }
            // Look for automatic releases
            if ($auto) {
                $usage= 0;
                $ldap->search("(&(objectClass=FAIrepositoryServer))",array("FAIrepository"));
                if ($ldap->count() != 0){
                    while($sattrs = $ldap->fetch()){
                        if (!isset($sattrs['FAIrepository'])){
                            continue;
                        }
                        for ($i= 0; $i < $sattrs['FAIrepository']['count']; $i++){
                            $t= preg_split('/\|/', $sattrs['FAIrepository'][$i]);
                            if($release == $t[2]){
                                $usage++;
                            }
                        }
                    }

                    // Are we the last one to provide this release?
                    if ($usage < 2) {
                        $obj= array_merge($obj, $aobj);
                        $found= true;
                    }
                } 
            }

            if ($found){
                msg_dialog::display(_("Error"), msgPool::stillInUse(_("FAI release"), msgPool::buildList($obj)), ERROR_DIALOG);
            }else{
                if(isset($this->repositories[$value])){
                    unset($this->repositories[$value]);
                }
            }
        }
    }


    /*! \brief    Act on events frowarded by the repositoryManagement
     */
    function editEntry($action,$target,$all)
    {
        if(count($target) == 1 && !$this->dialog){
            $value = array_pop($target);
            if(isset($this->repositories[$value])){
                $once = true;
                $obj = $this->repositories[$value];

                /* to be able to detect if this was renamed */
                $obj['initialy_was'] = $obj['Release'];
                $this->dialog = new servRepositorySetup($this->config,$this->dn,$obj);
                $this->dialog->parent = $this;
            }
        }
    }

    function execute()
    {
        /* Call parent execute */
        plugin::execute();

        $smarty= get_smarty();
        $this->repositoryMangement->handleActions($this->repositoryMangement->detectPostActions());

        if($this->is_account && !$this->view_logged){
            $this->view_logged = TRUE;
            new log("view","server/".get_class($this),$this->dn);
        }

        if(!$this->fai_activated){
            $str = "<h3>"._("You can't use this plug-in until FAI is activated.")."</h3>";
            $display = $smarty->fetch(get_template_path('servRepository.tpl', TRUE,dirname(__FILE__)));
            return($str.$display);
        }

        /* Fill templating stuff */
        $smarty->assign("is_createable",$this->acl_is_createable());
        $display= "";


        if(isset($_POST['repository_setup_save']) && is_object($this->dialog)){
            $this->dialog->save_object();
            if(($this->dialog->is_new_name())&&(isset($this->repositories[$this->dialog->GetName()]))){
                msg_dialog::display(_("Error"), msgPool::duplicated(_("Name")), ERROR_DIALOG);
            }else

                if(count($this->dialog->check())!=0){
                    foreach($this->dialog->check() as $msg){
                        msg_dialog::display(_("Error"), $msg, ERROR_DIALOG);
                    }
                }else{
                    $obj = $this->dialog->save();
                    if($this->dialog->is_new_name()){
                        $oldname = $this->dialog->initialy_was;
                        $this->repositories[$obj['Release']]=$obj;        
                        unset($this->repositories[$oldname]);
                    }else{ 
                        $this->repositories[$obj['Release']]=$obj;        
                    }
                    $this->dialog = FALSE;
                    $this->is_dialog= false;
                }
        }

        if(isset($_POST['repository_setup_cancel'])){
            $this->dialog=FALSE;
            $this->is_dialog = false;
        }

        if(is_object($this->dialog)){
            $this->dialog->save_object();
            $this->is_dialog = true;
            return($this->dialog->execute());
        }

        /*
           Repository setup dialog handling /END
         */
        $link   = "<a href='?plug=".$_GET['plug']."&amp;act=open_repository&amp;id=%s'>%s</a>";
        $edit   = "<input type='image' value='%s' name='edit_%s'   src='images/lists/edit.png'>&nbsp;";

        /* Hide delete icon, if delete is not allowed */ 
        if($this->acl_is_removeable()){
            $delete = "<input type='image' value='%s' name='delete_%s' src='images/lists/trash.png'>";
        }else{
            $delete = "<img src='images/empty.png' alt='&nbsp;'>";
        }

        session::set('RepositoryList', $this->repositories);
        $str = $this->repositoryMangement->execute(); 
        $display.= $smarty->fetch(get_template_path('servRepository.tpl', TRUE,dirname(__FILE__)));
        return($str.$display);
    }


    /* Save data to object */
    function save_object()
    {
        plugin::save_object();
    }


    /* Check supplied data */
    function check()
    {
        /* Call common method to give check the hook */
        $message= plugin::check();
        return ($message);
    }


    /* Save to LDAP */
    function save()
    {
        if(!$this->fai_activated) return;

        plugin::save();

        $arr = array();
        foreach($this->repositories as $servername => $conf){
            $str = "";
            foreach($conf['Sections'] as $sec){
                $str.=$sec.",";
            }
            $str=preg_replace("/,$/","",$str);

            if($conf['ParentServer']=="none"){
                $conf['ParentServer'] ="";
            }    

            $arr[]=$conf['Url']."|".$conf['ParentServer']."|".$conf['Release']."|".$str;
        }
        $this->attrs['FAIrepository'] = $arr;

        $ldap= $this->config->get_ldap_link();
        $ldap->cd ($this->config->current['BASE']);

        $ldap->cat($this->dn, array('dn'));

        if($ldap->count()){
            $ldap->cd($this->dn);
            $this->cleanup();
            $ldap->modify ($this->attrs);       
            $this->handle_post_events("modify");
        }else{
            $ldap->cd ($this->config->current['BASE']);
            $ldap->create_missing_trees($this->dn);
            $ldap->cd($this->dn);
            $ldap->add($this->attrs);
            $this->handle_post_events("add");
        }

# If there were changes, just tell the server to reload information
        if(count($this->attrs)){
            $this->trigger_si_fai_server_reload();
        }

        if($this->initially_was_account){
            new log("modify","server/".get_class($this),$this->dn,array_keys($this->attrs),$ldap->get_error());
        }else{
            new log("create","server/".get_class($this),$this->dn,array_keys($this->attrs),$ldap->get_error());
        }
    }


    function getListEntry()
    {
        $fields = goService::getListEntry();
        $fields['Message']    = _("Repository service");
        return($fields);
    }


    function trigger_si_fai_server_reload()
    {
        /* Reload GOsa si FAI DB/cache
         */
        if(class_available("DaemonEvent") && class_available("gosaSupportDaemon")){
            $events = DaemonEvent::get_event_types(SYSTEM_EVENT | HIDDEN_EVENT);        
            if(isset($events['TRIGGERED']['DaemonEvent_recreate_fai_server_db'])){
                $evt = $events['TRIGGERED']['DaemonEvent_recreate_fai_server_db']; 
                $tmp = new $evt['CLASS_NAME']($this->config);
                $tmp->set_type(TRIGGERED_EVENT);
                $tmp->add_targets(array("GOSA"));
                $o_queue = new gosaSupportDaemon();
                if(!$o_queue->append($tmp)){
                    msg_dialog::display(_("Service infrastructure"),msgPool::siError($o_queue->get_error()),ERROR_DIALOG);
                }
            }
        }
    }


    function allow_remove()
    {
        /* Check if the service is still in use 
         */
        $ldap = $this->config->get_ldap_link();
        $ldap->cd($this->config->current['BASE']);
        foreach($this->repositories as $repository) {
            $url = $repository['Url'];
            $ldap->search("(&(objectClass=gotoWorkstation)(objectClass=FAIobject)(FAIdebianMirror=".$url."))",array("cn","FAIclass"));
            if ($ldap->count() != 0){
                $obj= array();
                $found= false;
                while($attrs = $ldap->fetch()){
                    foreach($attrs['FAIclass'] as $class){
                        $obj[$ldap->getDN()]= $attrs['cn'][0];
                        $found= true;
                    }
                }
                if ($found){
                    $message = msgPool::stillInUse(_("Repository service")) . msgPool::buildList($obj);
                    return $message;
                }
            }
        }
    }



    function remove_from_parent()
    {
        goService::remove_from_parent();
        $this->trigger_si_fai_server_reload();
    }

    /* Return plugin informations for acl handling */
    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("Repository"),
                    "plDescription" => _("Repository service")." ("._("Services").")",
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 84,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("server"),

                    "plProperties" => array(
                        array(
                            "name"          => "repositoryBranchHook",
                            "type"          => "command",
                            "default"       => "",
                            "description"   => _("Script to manage branching and freezing of FAI repositories. Please read the FAQ for details.")
                        )),

                    "plProvidedAcls"=> array(
#            "start"         => _("Service start"),  // Remove this to hide the start button at all.
#            "stop"          => _("Service stop"),   // Remove this to hide the stop button at all.
#            "restart"       => _("Service restart"),// Remove this to hide the restart button at all.

#            "cn"            => _("Name"),
                            "ParentServer"  => _("Parent server"),
                            "Release"       => _("Releases"),
                            "Url"           => _("URL"),
                            "Section"       => _("Sections"))
                        ));
    }

    /*! \brief    Returns the list of of services, active services.
     *            Used in the filter class for services.
     *             class_filterServerService.inc
     */
    static function getRepositoryList()
    {
        return(session::get('RepositoryList'));
    }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
