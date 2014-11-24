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
        $o_queue = new gosaSupportDaemon();
        $servers = $o_queue->FAI_get_server();
        if($o_queue->is_error()){
            msg_dialog::display(_("Service infrastructure"),msgPool::siError($o_queue->get_error()),ERROR_DIALOG);
        }else{

            foreach($servers as $server){
                // The old code used the cn here, but at this time go-susi
                // does not include the cn in its reply, so we use the repository
                // URL instead. I think this makes more sense anyway because the
                // same server may host multiple repositories. Think for instance
                // about a server that mirrors both Debian and Ubuntu. You would
                // need to create 2 LDAP objects with different CNs for the same
                // server. Using CN here is also consistent with the FAI-Repository
                // selector in class_workstationStartup.inc
                $url = $server['SERVER'];
                
                // We don't want a server to be its own parent, so filter out
                // the repositories from this.
                if(isset($this->attrs['FAIrepository'])){
                  $i = 0;
                  while (isset($this->attrs['FAIrepository'][$i])) {
                    if (strpos($this->attrs['FAIrepository'][$i], $url) === 0)
                      continue 2;
                    $i++;
                  }
                }
                
                if (!isset($ret[$url])) $ret[$url] = array($url);
                
                $entry['cn'] = $url;
                $entry['release'] = $server['REPOPATH'];
                $entry['sections'] = $sections = explode(",", $server['SECTIONS']);
                array_push($ret[$url], $entry);
            }
        }
        
        $ret['none']= " ";
        asort($ret);
        return($ret);
    }

}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
