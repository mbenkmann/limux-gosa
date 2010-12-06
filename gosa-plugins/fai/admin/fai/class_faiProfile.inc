<?php

class faiProfile extends plugin
{
    /* attribute list for save action */
    var $ignore_account   = TRUE;

    /* Attributes for this Object */
    var $attributes       = array("cn","description","FAIclass");

    /* ObjectClasses for this Object*/
    var $objectclasses    = array("top","FAIclass","FAIprofile");

    /* Specific attributes */
    var $old_cn           = "";
    var $cn               = "";       // The class name for this object
    var $description      = "";       // The description for this set of partitions
    var $is_dialog        = false;    // specifies which buttons will be shown to save or abort
    var $FAIclass         = "";       // Classnames used for this profile
    var $FAIclasses       = array();  // Contains classname seperated in an array
    var $FAIAllclasses    = array();  // Contains all possible Classnames
    var $ui               ;
    var $FAIstate      = "";
    var $view_logged      = FALSE;

    var $classSelect;

    function faiProfile(&$config, $dn= NULL)
    {
        /* Load Attributes */
        plugin::plugin ($config, $dn);
        $ldap=$this->config->get_ldap_link();

        $this->ui = get_userinfo();

        /* Parse ldap attribute to get all assigned classes */
        $tmp = explode(" ",$this->FAIclass);
        $tmp2 = array();
        foreach($tmp as $class){
            if(!empty($class)){
                $tmp2[trim($class)] = trim($class);
            }
        }

        /* Sort assigned classes */ 
        if(is_array($tmp2)){
            foreach($tmp2 as $class){
                $this->FAIclasses[$class]=$class;
            }
        }

        $categories = array("FAIscript","FAItemplate","FAIhook","FAIvariable","FAIpartitionTable","FAIpackageList");

        /* Build filter */
        $filter= "";
        foreach ($categories as $cat){
            $filter.= "(objectClass=$cat)";
        }

        /* Get ldap connection */ 
        $base  = session::get('CurrentMainBase');
        $ldap->cd($base);
        $sort = array();

        /* search all FAI classes */
        $ldap->search("(|$filter)",array("*"));
        while($attrs = $ldap->fetch()){

            /* Sort by categorie */
            foreach($categories as $cat){
                if(in_array($cat,$attrs['objectClass'])){

                    /* Append entry */
                    $this->FAIAllclasses[$attrs['cn'][0]][$cat]=$attrs;

                    /* Create sort array, because the array above is a multidimensional array, and can't be sorted by php sorting functions*/
                    $sort[strtolower($attrs['cn'][0])] = $attrs['cn'][0];
                }
            }
        } 

        /* Sort the sort array */
        //ksort($sort);

        /* Reorder the FAIclasses array */
        foreach($sort as $name){
            $tmp[$name] =$this->FAIAllclasses[$name];
        }

        /* Assign sorted classes */
        $this->FAIAllclasses = array();
        $this->FAIAllclasses = $tmp;

        if($dn != "new"){
            $this->dn =$dn;
        }
        $this->old_cn   = $this->cn;

        // Prepare lists
        $this->classList = new sortableListing(array(),array(), TRUE);
        $this->classList->setDeleteable(true);
        $this->classList->setEditable(false);
        $this->classList->setWidth("100%");
        $this->classList->setHeight("200px");
        $this->classList->setHeader(array(_("Name"),_("Types")));
        $this->classList->setDefaultSortColumn(0);
    }


    function acl_base_for_current_object($dn)
    {
        if($dn == "new" || $dn == ""){
            if($this->dn == "new"){
                $dn= $this->parent->parent->acl_base;
            }else{
                $dn = $this->dn;
            }
        }
        return($dn);
    }


    function execute()
    {
        /* Call parent execute */
        plugin::execute();

        if($this->is_account && !$this->view_logged){
            $this->view_logged = TRUE;
            new log("view","fai/".get_class($this),$this->dn);
        }

        /* Fill templating stuff */
        $smarty= get_smarty();
        $display= "";

        $s_entry = "";
        $s_action = "";

        /* Remove class name From list */
        $sort_once = false;
        if(!preg_match("/freeze/",$this->FAIstate)){

        if(isset($_POST['AddClass']) && $this->acl_is_writeable("FAIclass")){
                $this->classSelect = new classSelect($this->config, get_userinfo());
                $this->dialog  =true;
        }


            /* Save Dialog */
            if(isset($_POST['classSelect_save']) && $this->classSelect instanceOf classSelect){
                $list = $this->classSelect->save();
                foreach($list as $entry){
                    $class = $entry['cn'][0];
                    $this->FAIclasses[$class] =$class; 
                }
                $this->dialog=false;
                $this->classSelect=FALSE;
            }
        }

        /* Cancel Dialog */
        if(isset($_POST['classSelect_cancel']) && $this->classSelect instanceOf classSelect){
            $this->dialog=false;
            $this->classSelect=FALSE;
        }

        if($this->classSelect instanceOf classSelect){
            session::set('filterFAIClass_base',$this->parent->parent->fai_release);
            session::set('filterBlacklist', array('cn' => array_values($this->FAIclasses)));
            return($this->classSelect->execute());
        }

        /* item images */
        $objTypes['FAIhook']            = image('plugins/fai/images/fai_hook.png','',_("Hook bundle"));
        $objTypes['FAItemplate']        = image('plugins/fai/images/fai_template.png','',_("Template bundle"));
        $objTypes['FAIscript']          = image('plugins/fai/images/fai_script.png','',_("Script bundle"));
        $objTypes['FAIvariable']        = image('plugins/fai/images/fai_variable.png','',_("Variable bundle"));
        $objTypes['FAIpackageList']     = image('plugins/fai/images/fai_packages.png','',_("Package bundle"));
        $objTypes['FAIpartitionTable']  = image('plugins/fai/images/fai_partitionTable.png','',_("Partition table"));

        $emp = image('images/empty.png');

        /* Delete button */
        $actions = image('images/lists/trash.png','DEL_%KEY%',_("Remove class from profile")); 

        /* Up down buttons */
        $linkupdown = image('images/lists/sort-up.png','sortup_%s',_("Up"), 'top');
        $linkupdown.= image('images/lists/sort-down.png','sortdown_%s',_("Down"), 'bottom');

        $this->classList->setAcl($this->getacl('FAIclass',preg_match('/^freeze/', $this->FAIstate)));
        $data = $lData = array();
        foreach($this->FAIclasses as $usedClass){
            $str = "";
            if(isset($this->FAIAllclasses[$usedClass])){
                foreach($this->FAIAllclasses[$usedClass] as $class => $obj){
                    $str.= $objTypes[$class]; 
                }
            }
            $data[$usedClass] = $usedClass;
            $lData[$usedClass] = array('data'=>array($usedClass,$str));
        }
        $this->classList->setListData($data,$lData);
        $this->classList->update();


        $smarty->assign("freeze", preg_match("/freeze/i",$this->FAIstate));
        $smarty->assign("FAIclasses"  ,$this->FAIclasses);
        $smarty->assign("listing"     ,$this->classList->render());

        /* Magic quotes GPC, escapes every ' " \, to solve some security risks
         * If we post the escaped strings they will be escaped again
         */
        foreach($this->attributes as $attrs){
           $smarty->assign($attrs,set_post($this->$attrs));
        }


        $dn = $this->acl_base_for_current_object($this->dn);
        $smarty->assign("sub_object_is_addable",
                preg_match("/c/",$this->ui->get_permissions($dn,"fai/faiScriptEntry")) &&
                !preg_match("/freeze/",$this->FAIstate));

        $tmp = $this->plInfo();
        foreach($tmp['plProvidedAcls'] as $name => $translated){
            $smarty->assign($name."ACL",$this->getacl($name, preg_match("/freeze/",$this->FAIstate)));
        }

        $display.= $smarty->fetch(get_template_path('faiProfile.tpl', TRUE));
        return($display);
    }

    function remove_from_parent()
    {
        $ldap = $this->config->get_ldap_link();
        $ldap->cd ($this->dn);
        $release = $this->parent->parent->fai_release;
        $use_dn = preg_replace("/".preg_quote(FAI::get_release_dn($this->dn), '/')."/i", $release, $this->dn);
        new log("remove","fai/".get_class($this),$use_dn,$this->attributes);
        FAI::prepare_to_save_FAI_object($use_dn,array(),true);
        $this->handle_post_events("remove");    
    }


    /* Save data to object 
     */
    function save_object()
    {
        if(!preg_match("/freeze/",$this->FAIstate)){
            plugin::save_object();

            $this->classList->save_object();    
            $action = $this->classList->getAction();
            if($action['action'] == 'reorder'){
                $this->FAIclasses = $this->classList->getMaintainedData();
            }
            if($action['action'] == 'delete'){
                $s_entry = $this->classList->getKey($action['targets'][0]);
                unset($this->FAIclasses[$s_entry]);
            }
        }
    }


    /* Check supplied data */
    function check()
    {
        /* Call common method to give check the hook */
        $message= plugin::check();

        if(count($this->FAIclasses) == 0){
            $message[]=_("No class specified for this profile!");
        }

        if($this->cn == ""){
            $message[]= msgPool::required(_("Name"));
        }

        /* Ensure that we do not overwrite an allready existing entry 
         */
        if($this->dn == "new" || $this->cn != $this->old_cn){
            $release = $this->parent->parent->fai_release;
            $new_dn= 'cn='.$this->cn.",".get_ou("faiProfile", "faiProfileRDN").get_ou("faiManagement", "faiBaseRDN").$release;
            $res = faiManagement::check_class_name("FAIprofile",$this->cn,$new_dn);
            if(isset($res[$this->cn])){
                $message[] = msgPool::duplicated(_("Name"));
            }
        }

        return ($message);
    }


    /* Save to LDAP */
    function save()
    {
        plugin::save();

        $ldap = $this->config->get_ldap_link();

        $this->FAIclass = "";
        foreach($this->FAIclasses as $class){
            $this->FAIclass.=$class." ";
        }

        $this->attrs['FAIclass']=trim($this->FAIclass);

        /* Remove the old FAI profile, if the dn has changed.
         */
        if($this->cn != $this->old_cn && $this->old_cn != ""){

            $old_dn = preg_replace("/^cn=[^,]+,/","cn=".$this->old_cn.",",$this->dn);
            FAI::prepare_to_save_FAI_object($old_dn,array(),TRUE);
        }
        FAI::prepare_to_save_FAI_object($this->dn,$this->attrs);

        if($this->initially_was_account){
            new log("modify","fai/".get_class($this),$this->dn,$this->attributes);
        }else{
            new log("create","fai/".get_class($this),$this->dn,$this->attributes);
        }
    }


    function PrepareForCopyPaste($source)
    {
        plugin::PrepareForCopyPaste($source);

        /* Parse ldap attribute to get all assigned classes */
        $tmp = explode(" ",$this->FAIclass);
        $tmp2 = array();
        foreach($tmp as $class){
            if(!empty($class)){
                $tmp2[trim($class)] = trim($class);
            }
        }

        /* Sort assigned classes */
        if(is_array($tmp2)){
            foreach($tmp2 as $class){
                $this->FAIclasses[$class]=$class;
            }
        }
    }


    /* Return plugin informations for acl handling */ 
    static function plInfo()
    {
        return (array( 
                    "plShortName" => _("Profile"),
                    "plDescription" => _("FAI profile"),
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 30,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("fai"),

                    "plProperties" =>
                    array(
                        array(
                            "name"          => "faiProfileRDN",
                            "type"          => "rdn",
                            "default"       => "ou=profiles,",
                            "description"   => _("RDN for FAI profile storage."),
                            "check"         => "gosaProperty::isRdn",
                            "migrate"       => "migrate_faiProfileRDN",
                            "group"         => "plugin",
                  "mandatory"     => TRUE)),


                    "plProvidedAcls" => array(
                        "cn"                => _("Name"),
                        "description"       => _("Description"),
                        "FAIclass"          => _("FAI classes"))
                    ));
    }


    /*! \brief  Used for copy & paste.
      Returns a HTML input mask, which allows to change the cn of this entry.
      @param  Array   Array containing current status && a HTML template.
     */
    function getCopyDialog()
    {
        $vars = array("cn");
        $smarty = get_smarty();
        $smarty->assign("cn", set_post($this->cn));
        $str = $smarty->fetch(get_template_path("paste_generic.tpl",TRUE));
        $ret = array();
        $ret['string'] = $str;
        $ret['status'] = "";
        return($ret);
    }


    /*! \brief  Used for copy & paste.
      Some entries must be renamed to avaoid duplicate entries.
     */
    function saveCopyDialog()
    {
        if(isset($_POST['cn'])){
            $this->cn = get_post('cn');
        }
    }
}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
