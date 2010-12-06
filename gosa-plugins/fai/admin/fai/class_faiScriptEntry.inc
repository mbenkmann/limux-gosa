<?php

class faiScriptEntry extends plugin
{
    /* attribute list for save action */
    var $ignore_account= TRUE;
    var $attributes   = array("cn","description","FAIpriority","FAIscript");
    var $objectclasses= array();

    var $orig_cn              = "";

    var $dn            = "";
    var $cn            = "";
    var $FAIpriority   = "0";
    var $FAIscript     = "";
    var $description   = "";
    var $status        = "new";
    var $parent        = NULL;
    var $FAIstate      = "";

    // Encoding validation
    var $enc_before_edit = "";
    var $enc_after_edit = "";
    var $write_protect = FALSE;

    function faiScriptEntry (&$config, $dn= NULL,$object=false)
    {
        plugin::plugin ($config, NULL);
        if($dn != "new"){
            $this->orig_cn= $object['cn'];
            $this->dn=$object['dn'];
            foreach($object as $name=>$value){
                $this->$name=$value;
            }
        }else{
            if(is_array($object)&&count($object)){
                $this->orig_cn= $object['cn'];
                $this->dn=$object['dn'];
                foreach($object as $name=>$value){
                    $oname = $name;
                    $this->$oname=$value;
                }
            }else{
                $this->status = "new";
                $this->orig_cn = false;
            }
        }

        // Keep an eye on dangerous encodings, we may break scripts while editing.
        $this->mb_extension = function_exists("mb_detect_encoding");
        if($this->mb_extension){
            $this->enc_before_edit = mb_detect_encoding($this->FAIscript);
            if($this->enc_before_edit != "ASCII"){
                $this->write_protect = TRUE;
            }
        }
    }


    function execute()
    {
        /* Call parent execute */
        plugin::execute();

        /* Fill templating stuff */
        $smarty     = get_smarty();
        $smarty->assign("freeze", preg_match("/freeze/i",$this->FAIstate));
        $display = "";

        if(isset($_POST['ImportUpload']) && $this->acl_is_writeable("FAIscript")){
            if(($_FILES['ImportFile']['error']!=0)){
                msg_dialog::display(_("Error"), msgPool::incorrectUpload(), ERROR_DIALOG);
            }else
                if(($_FILES['ImportFile']['size']==0)){
                    msg_dialog::display(_("Error"), msgPool::incorrectUpload(_("file is empty")), ERROR_DIALOG);
                }else{
                    $str = file_get_contents(gosa_file_name($_FILES['ImportFile']['tmp_name']));
                    $this->FAIscript = $str;

                    // Check encoding again 
                    if($this->mb_extension){
                        $this->enc_before_edit = mb_detect_encoding($this->FAIscript);
                        if($this->enc_before_edit != "ASCII"){
                            $this->write_protect = TRUE;
                        }
                    }
                }
        }

        // Assign encoding related variables.
        $smarty->assign("write_protect",$this->write_protect);

        foreach($this->attributes as $attrs){
            $smarty->assign($attrs, set_post($this->$attrs));
        }

        /* File download requested */
        if(isset($_POST['download'])){
            send_binary_content($this->FAIscript,$this->cn.".FAIscript");
        }

        /* Create download button*/
        if($this->dn != "new" && $this->acl_is_readable("FAIscript")){
            $smarty->assign("DownMe", image('images/save.png','download',_("Download")));
        }else{
            $smarty->assign("DownMe","");
        }

        for($i =0 ; $i < 100 ; $i++){
            $FAIprioritys[$i]=$i;
        }

        $tmp = $this->plInfo();
        foreach($tmp['plProvidedAcls'] as $name => $translated){
            $acl = $this->getacl($name,preg_match("/freeze/i",$this->FAIstate));
            $smarty->assign($name."ACL",$acl);
        }
        $smarty->assign("FAIprioritys",$FAIprioritys);
        $display.= $smarty->fetch(get_template_path('faiScriptEntry.tpl', TRUE));
        return($display);
    }

    /* Save data to object */
    function save_object()
    {
        if((isset($_POST['SubObjectFormSubmitted'])) && !preg_match("/freeze/", $this->FAIstate)){
            foreach($this->attributes as $attrs){

                // Do not touch FAIscriot values here, w'll do this a few lines below.
                if($attrs == "FAIscript") continue;
                if($this->acl_is_writeable($attrs)){
                    if(isset($_POST[$attrs])){
                        $this->$attrs = get_post($attrs);
                    }else{
                        $this->$attrs = "";
                    }
                }
            }

            // Get FAIscript changes
            if(isset($_POST['FAIscript']) && 
                    $this->acl_is_writeable('FAIscript') &&
                    !$this->write_protect){

                // Check encoding again. 
                $this->FAIscript = get_post('FAIscript');
                $this->enc_after_edit = mb_detect_encoding($this->FAIscript);
            }
        }
        if(isset($_POST['editAnyway'])) $this->write_protect = FALSE;
    }


    /* Check supplied data */
    function check()
    {
        /* Call common method to give check the hook */
        $message= plugin::check();

        if($this->mb_extension && !$this->write_protect && $this->enc_after_edit !== $this->enc_before_edit ){
            $msg = sprintf(_("The script encodig has changed from '%s' to '%s'. Do you really want to save?"),
                    "<i>".$this->enc_before_edit."</i>","<i>".$this->enc_after_edit."</i>");
            $message[] = $msg;
            $this->enc_before_edit = $this->enc_after_edit;
        }


        if(isset($this->parent->SubObjects[$this->cn]) && $this->cn != $this->orig_cn){
            $message[] = msgPool::duplicated(_("Name"));
        }

        $c = trim($this->cn);
        if($c == ""){
            $message[] = msgPool::required(_("Name"));
        }
        if(preg_match("/[^a-z0-9_\-]/i",$c)){
            $message[] = msgPool::invalid(_("Name"),$c,"/[a-z0-9_\-]/i");
        }

        $s = trim($this->FAIscript);
        if($s == ""){
            $message[]= msgPool::required(_("Script"));
        }

        return ($message);
    }

    function save()
    {
        $tmp=array();
        foreach($this->attributes as $attrs){ 
            $tmp[$attrs] = $this->$attrs;
        }

        if(($this->orig_cn)&&($tmp['cn']!=$this->orig_cn)){
            $tmp['remove']['from']  = $this->orig_cn;
            $tmp['remove']['to']    = $tmp['cn'];
        }

        $tmp['dn']      = $this->dn;  
        $tmp['status']  = $this->status; 
        return($tmp);
    }


    /* Return plugin informations for acl handling */
    static function plInfo()
    {
        return (array(
                    "plShortName" => _("Script entry"),
                    "plDescription" => _("FAI script entry"),
                    "plSelfModify"  => FALSE,
                    "plDepends"     => array(),
                    "plPriority"    => 19,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("fai"),
                    "plProvidedAcls" => array(
                        "cn"              => _("Name"),
                        "description"     => _("Description"),
                        "FAIscript"       => _("Script entry"),
                        "FAIpriority"     => _("Script Priority"))
                    ));
    }


}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
