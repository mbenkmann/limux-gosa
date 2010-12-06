<?php

class servRepositorySetup  extends plugin
{
    /* attribute list for save action */
    var $ignore_account   = TRUE;
    var $attributes       = array("Release","ParentServer","Url");
    var $objectclasses    = array("whatever");

    /* Attributes */
    var $Release          = "";
    var $ParentServer     = "";
    var $Url              = "";   
    var $Sections         = array();
    var $ParentServers    = "";
    var $initialy_was     = false;
    var $cn               = "";
    var $parent           = "";

    function servRepositorySetup (&$config, $dn= NULL,$data = false)
    {
        plugin::plugin ($config, $dn);

        if(isset($this->attrs['cn'][0])){
            $this->cn=$this->attrs['cn'][0];
        }

        if($data != false){
            foreach(array("Sections","Release","Url","ParentServer","initialy_was") as $atr){
                if(isset($data[$atr])){
                    $this->$atr = $data[$atr];
                }
            }
        }

        // Prepare lists
        $this->sectionList = new sortableListing();
        $this->sectionList->setDeleteable(true);
        $this->sectionList->setInstantDelete(true);
        $this->sectionList->setEditable(false);
        $this->sectionList->setWidth("100%");
        $this->sectionList->setHeight("100px");
        $this->sectionList->setHeader(array(_("Name")));
        $this->sectionList->setColspecs(array('*','20px'));
        $this->sectionList->setDefaultSortColumn(0);

    }

    function GetName()
    {
        return($this->Release);
    }

    function is_new_name()
    {
        if(!$this->initialy_was){
            return(true);
        }else{
            if($this->Release != $this->initialy_was){
                return(true);
            }
        }
        return(false);
    }



    function execute()
    {
        /* Call parent execute */
        plugin::execute();

        /* Fill templating stuff */
        $smarty= get_smarty();

        if(preg_match("/w/",$this->parent->getacl("Section"))){
            if((isset($_POST['AddSection']))&&(isset($_POST['SectionName']))&&(!empty($_POST['SectionName']))){

                /* Replace multiple spaces with a single, and cut of white spaces (trim)*/
                $val = preg_replace("/\ \ * /" , " ", trim(get_post('SectionName')));

                /* check if there are more than one entry given ( "section1 section2 )*/
                if(preg_match("/ /",$val)){

                    /* Generate list of new section names */
                    $vals = explode(" ",$val);

                    /* Add new entries */
                    foreach($vals as $entry){
                        $entry = trim($entry);
                        if(!empty($entry)){
                            $this->Sections[$entry]=$entry;
                        }
                    }
                }else{
                    $this->Sections[$val]=$val;
                }
            }

            $this->sectionList->save_object();
            $action = $this->sectionList->getAction();
            if($action['action'] == 'delete'){
                $this->Sections = $this->sectionList->getMaintainedData();  
            }
        }

        $this->sectionList->setAcl($this->parent->getacl('Section'));
        $this->sectionList->setListData(set_post($this->Sections) );
        $this->sectionList->update();

        $smarty->assign("Sections",$this->sectionList->render());

        /* Get && assign acls */
        $tmp = $this->parent->plInfo();
        foreach($tmp['plProvidedAcls'] as $name => $translated){
            $smarty->assign($name."ACL",$this->parent->getacl($name));
        }

        /* Assign values */
        foreach($this->attributes as $attr){
            $smarty->assign($attr, set_post($this->$attr));
        }

        $tmp = $this->getParentServers();
        $smarty->assign("ParentServers"   , set_post($tmp));
        $smarty->assign("ParentServerKeys", set_post(array_flip($tmp)));

        return($smarty->fetch(get_template_path('servRepositorySetup.tpl', TRUE,dirname(__FILE__))));
    }

    /* Save data to object */
    function save_object()
    {
        if(isset($_POST['servRepositorySetup_Posted'])) {

            foreach($this->attributes as $attr){
                if(($this->parent->acl_is_writeable($attr)) && (isset($_POST[$attr]))){
                    $this->$attr = get_post($attr);      
                }
            }
        }
    }


    /* Check supplied data */
    function check()
    {
        /* Call common method to give check the hook */
        $message= plugin::check();

        if(empty($this->Release)){
            $message[]= msgPool::required(_("Release"));
        }

        if(empty($this->Url)){
            $message[] = msgPool::required(_("URL"));
        }

        return ($message);
    }


    /* Save to LDAP */
    function save()
    {
        $tmp = array();
        $tmp['ParentServer']  = $this->ParentServer;
        $tmp['Url']           = $this->Url;
        $tmp['Release']       = $this->Release;
        $tmp['Sections']      = $this->Sections;
        return($tmp);
    }

    function getParentServers()
    {
        $ret = array();
        $ldap = $this->config->get_ldap_link();
        $ldap->cd($this->config->current['BASE']);
        $ldap->search("(objectClass=FAIrepositoryServer)",array("*"));
        while($attr = $ldap->fetch()){
            if($attr['cn'][0] == $this->cn) continue;
            $ret[$attr['cn'][0]]= $attr['cn'][0];   
        }

        $ret = array_merge($ret,$this->GetHookElements());

        $ret['none']= " ";
        asort($ret);
        return($ret);
    }

    /* this funtions calls a defined hook 
       and parses all additional serverdata 
     */
    function GetHookElements()
    {
        $ret = array();
        $cmd = $this->config->get_cfg_value("servrepository", "repositoryBranchHook");
        if(!empty($cmd)){
            $res = shell_exec($cmd);
            $res2 = trim($res);
            if(!$res || empty($res2)){
                msg_dialog::display(_("Error"), msgPool::cmdexecfailed("repositoryBranchHook", $cmd, _("Repository service")), ERROR_DIALOG);
            }else{  
                $tmp = preg_split("/\n/",$res);
                foreach($tmp as $hook){
                    /* skip empty */
                    if(empty($hook)) continue;

                    if(preg_match("/;/",$hook)){ 
                        $hookinfo = explode(";",$hook);
                        $ret[$hookinfo[0]] = $hookinfo[0];
                    }else{
                        $ret[$hook] = $hook;
                    }
                }
            }
        }
        return($ret);
    }

}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
