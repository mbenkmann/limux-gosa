<?php

class servRepositorySetup  extends plugin
{
    /* attribute list for save action */
    var $ignore_account   = TRUE;
    var $attributes       = array("Release","ParentServer","Url");
    var $objectclasses    = array("whatever");

    /* Attributes */
    var $Release          = "";
    var $SelectedRelease  = "";
    var $ParentServer     = "none";
    var $Url              = "";   
    var $Sections         = array();
    var $ParentServers    = "";
    var $initialy_was     = false;
    var $cn               = "";
    var $parent           = "";
    var $OldParentServer  = "";
    var $OldRelease       = "";

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

        /* Generate the list of possible parent servers */
        $repositories = $this->getParentServers();

        /* Set parent server */
        if (isset($_POST["ParentServer"])) {
            /* name of parent server */
            $this->ParentServer = $_POST["ParentServer"];
        } 

        /* Set release */
        $SelectedRelease = 0;
        if (!$this->Release || ($this->OldParentServer != "" &&
                $this->ParentServer != $this->OldParentServer)) {
            if ($this->ParentServer == "none") {
                $this->Release = "";
            } else {
                /* Set release to first available release of parent server */
                $this->Release = $repositories[$this->ParentServer][1]["release"];
                /* Unset selected release (which is from old server) */
                unset($_POST["SelectedRelease"]);
            }
        }


        /* Detect selected release */
        if ($this->ParentServer != "none") {
            if (isset($_POST["SelectedRelease"])) {
                /* Some release was selected */
                $SelectedRelease = $_POST["SelectedRelease"];
            } else {
                /* Determine release offset by comparing with old relase string */
                $SelectedRelease = 0;
                while(isset($repositories[$this->ParentServer][$SelectedRelease+1])) {
                     $bla = $this->Release;
                     if ($repositories[$this->ParentServer][$SelectedRelease+1]["release"] == $this->Release) {
                        break;
                    }
                    $SelectedRelease++;
                }
            }
            $this->Release = $repositories[$this->ParentServer][$SelectedRelease+1]["release"];
        }

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

        $sections = array();
        if (isset($_POST["ParentServer"]) && $this->ParentServer != "none" &&
                ($this->OldParentServer != $this->ParentServer || 
                $this->OldRelease != $SelectedRelease)) {
            /* Parent server or release changed, prefill section list */
            foreach ($repositories[$this->ParentServer][$SelectedRelease+1]["sections"] as $key => $entry) {
                $sections[$entry] = $entry;
            }
            $this->Sections = $sections;
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

        $servers = array();
        foreach ($repositories as $key => $entry) {
            if (is_array($entry)) {
                $servers[$entry[0]] =  $entry[0];
            } else {
                $servers["none"] = "none";
            }
        }
        $smarty->assign("ParentServers"   , set_post($servers));
        $smarty->assign("ParentServerKeys", set_post(array_flip($servers)));

        $releases = array();
        if ($this->ParentServer != "none") {
            foreach ($repositories[$this->ParentServer] as $key => $entry) {
                if (is_array($entry)) {
                    array_push($releases, $entry["release"]);
                }
            }
        }
        $smarty->assign("SelectedRelease"   , set_post($SelectedRelease));
        $smarty->assign("Releases"   , set_post($releases));
        $smarty->assign("ReleasesKeys", set_post(array_flip($releases)));

        /* Save ParentServer and Release so we can detect whether they got
           changed on next POST */
        $this->OldParentServer = $this->ParentServer;
        $this->OldRelease = $SelectedRelease;

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

        /* Check whether list of sections is consistent with parent server */
        if ($this->ParentServer != "none") {
            $servers = $this->getParentServers();
            foreach ($servers[$this->ParentServer] as $key => $release) {
                if (!is_array($release)) {
                    continue;
                }
                if ($release["release"] == $this->Release) {
                    foreach($this->Sections as $key => $section) {
                        if (!in_array($section, $release["sections"])) {
                           $invalid_sections[] = $section;
                        } 
                    }
                }
            }
            if (isset($invalid_sections) && !empty($invalid_sections)) {
              $message[] = sprintf(_("The following sections are not present on the parent server: '%s'"),implode(", ",$invalid_sections));
            }
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
        $ldap->search("(objectClass=FAIrepositoryServer)",array("cn", "FAIrepository"));
        while($attr = $ldap->fetch()){
            $i = 0;
            if($attr['cn'][0] == $this->cn || !isset($attr['FAIrepository'][0])) continue;
            $ret[$attr['cn'][0]] = array($attr['cn'][0]);
            while (isset($attr['FAIrepository'][$i])) {
                list($url, $foo, $release, $sections) = explode("|", $attr['FAIrepository'][$i]);
                $sections = explode(",", $sections);
                $entry['cn'] = $attr['cn'][0];
                $entry['release'] = $release;
                $entry['sections'] = $sections;
                array_push($ret[$attr['cn'][0]], $entry);
                $i++;
            }
        }
        
        /*
        Mix in elements provided by hook. The hook provides names only. We search for these names from base c=de,
        because the hook provided elements reside high up in the tree.
        */
        foreach($this->GetHookElements() as $servername) {
            $ldap->cd("c=de");
            $ldap->search("(&(objectClass=FAIrepositoryServer)(cn=$servername))",array("cn", "FAIrepository"));
            while($attr = $ldap->fetch()){
                $i = 0;
                if($attr['cn'][0] == $this->cn || !isset($attr['FAIrepository'][0])) continue;
                $ret[$attr['cn'][0]] = array($attr['cn'][0]);
                while (isset($attr['FAIrepository'][$i])) {
                    list($url, $foo, $release, $sections) = explode("|", $attr['FAIrepository'][$i]);
                    $sections = explode(",", $sections);
                    $entry['cn'] = $attr['cn'][0];
                    $entry['release'] = $release;
                    $entry['sections'] = $sections;
                    array_push($ret[$attr['cn'][0]], $entry);
                    $i++;
                }
            }
        }

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
        
        /*
          Die Funktion config->search() ist deprecated und man sollte stattdessen get_cfg_value() verwenden.
          ABER WENN MAN DIESE ANPASSUNG MACHT MUSS MAN TESTEN OB ES NOCH FUNKTIONIERT. Der folgende Befehl
          funktioniert NICHT:
            $cmd = $this->config->get_cfg_value("servrepository", "repositoryBranchHook");
          Ich habe keine Ahnung warum, aber liefert immer einen leeren String zurück.
        */
        $cmd = $this->config->search("servrepository", "repositoryBranchHook",array('tabs'));
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
