<?php
//! This class handles the goFonMacroParameter
/*! In this class all parameters from goFonMacroContent \n
  and all (if given) already defined parameters are managed \n
  \n
  Parameters will be saved to openldap like this :\n
  goFonMacroParameter: ID!NAME:!TYPE(string:combo:bool)!DEFAULT_VALUE\n    
  In Case of type=combo the DEFAULT_VALUE specifies the entries in the listbox, like this\n
  "first:second:third:last" \n
 */
class macroParameter extends plugin
{
    /* Parameter Count*/
    var $para_count = 0 ; 
    /*! macro base  */
    var $base= "";

    /*! This array contains all Parameter defined for the macro*/
    var $goFonMacroParameter =array();

    /*! This are the available types for a macro */
    var $type_shortcut= array("string" => array("selected", "", ""),
            "combo"  => array("", "selected", ""),
            "bool"   => array("", "", "selected"));

    /*! attribute list for save action */
    var $attributes= array("base","goFonMacroParameter");

    /*! Objectclasses needed by the class*/  
    var $objectclasses= array("top", "goFonMacro");
    var $ui;

    //! The konstructor of macroParameter    
    /*! The konstructor of macroParameter...
      - reads goFonMacroParameter and parses them to an array 
      - Set attributes from openldap (edit)
      - Set attributes from default (new)
     */
    function macroParameter (&$config, $dn= NULL, $parent= NULL)
    {
        plugin::plugin ($config, $dn, $parent);

        $tmp = array();  // temporary Var 
        $tmp2 = array(); // temporary Var ...
        $tmp3 = "";
        $ldap= $config->get_ldap_link();

        $this->dn = $dn;

        /* This is always an account */
        $this->is_account= TRUE;

        /* Edit or new one ?*/
        if ($this->dn == "new"){
            $ui= get_userinfo();
            $this->base= dn2base($ui->dn);
        } else {
            $this->base= dn2base($this->dn);
        }

        /* initialising macro parameter */
        if(isset($this->attrs['goFonMacroParameter']) &&
                isset($this->attrs['goFonMacroParameter']['count'])){
            unset($this->attrs['goFonMacroParameter']['count']);
        }

        /* Set Parameters, or a new array if ther are no parameters */
        if(isset($this->attrs['goFonMacroParameter'])){
            $this->goFonMacroParameter = $this->attrs['goFonMacroParameter'];
        }else{
            $this->goFonMacroParameter =array();
        }

        /* Create an array for parameters if not given yet */
        if(!is_array($this->goFonMacroParameter)){
            $tmp3 = $this->goFonMacroParameter;
            $this->goFonMacroParameter =array();	
            if(!empty($tmp3)) {
                $this->goFonMacroParameter[]  = $tmp3;
            }
        }

        /* Load parametersettings*/
        foreach($this->goFonMacroParameter as $para){
            $tmp = explode("!",$para);
            $num = $tmp[0];
            $tmp2[$num]['name']        = base64_decode($tmp[1]);
            $tmp2[$num]['type']        = $tmp[2];
            $tmp2[$num]['default']     = $tmp[3];
            $tmp2[$num]['var']         = "var".$num;
        }


        /* Assign this array */
        $this->goFonMacroParameter = $tmp2;

        $this->para_count = count ($tmp2);

        $this->ui= get_userinfo();
    }

    //! Perform Parameter check 
    /*! 
      Compares the given parameters (goFonMacroParameters) with the parameters defined in goFonContent\n 
      -> Decide which attrs are new and which are unused\n
      -> Sort result array (containing both parameters 'goFonMacroParameters/goFonContent' and new / unused info)\n
      \param $content The given goFonContent for this macro\n
      \param $goFonMacroParameter Array with the already given parameters \n
     */
    function check_paras($content,$goFonMacroParameter)
    { 
        /* Check contents for parameters */
        preg_match_all("/[$]\{ARG[0-9]*\}/",$content,$res,PREG_OFFSET_CAPTURE);

        $new = array();

        /* Detect parameters with positions */
        foreach($res[0] as $val){
            $num = preg_replace("/[^0-9]/","",$val[0]); 
            $new[$num]['val'] = $val[0];
            $new[$num]['num'] = $num;
        }

        /* Compare content parameter and macro parameter */
        foreach($goFonMacroParameter as $gokey => $goval){
            foreach($new as $nkey => $nval){
                if($gokey == $nval['num']){
                    /* sign this as OK */
                    $goFonMacroParameter[$gokey]['check']= true;
                }
            }
        }

        /* Now check if there is new parameter in the content, which is not assigned yet */
        foreach($new as $key => $val){
            /* Assign std values */
            $goFonMacroParameter[$key]['var']="var".$key;
            $goFonMacroParameter[$key]['check']= true;

            /* If this is a new Parameter, name it ${ARG#} by default*/
            if((!isset($goFonMacroParameter[$key]['name']))||(empty($goFonMacroParameter[$key]['name']))){
                $goFonMacroParameter[$key]['name']="\${ARG".$key."}";
            }
        }  

        foreach($goFonMacroParameter as $key => $val){
            /* All attributes with check == false, are unneeded so mark them with ['check']= false */
            if(!isset($goFonMacroParameter[$key]['check'])){
                $goFonMacroParameter[$key]['check']= false;
            }
            /* Ah no default given assign ="" to prevent unsigned index  */
            if(!isset($goFonMacroParameter[$key]['default'])){
                $goFonMacroParameter[$key]['default'] = "";
            }
        }

        /* Sort output for better reading */
        asort($goFonMacroParameter);
        return($goFonMacroParameter);

    }

    //! Execute this Plugin
    /*! 
      Perform Parameter check \n
      Draw paramter table\n
      Show tpl   \n 
     */
    function execute()
    {
        /* Call parent execute */
        plugin::execute();

        /* Variables */
        $vars 	= "";
        $tmp  	= array();
        $number = 0; 

        /* Get acls for this tab, 
           there is only one attribute to write,
           so we use the acls from gofon/marco */
        if($this->is_new){
            $ACLs = $this->ui->get_permissions($this->base,"gofonmacro/macro","goFonMacroContent");
        }else{
            $ACLs = $this->ui->get_permissions($this->dn,"gofonmacro/macro","goFonMacroContent");
        }

        /* get current content */
        $content = $this->parent->by_object['macro']->goFonMacroContent;

        if(strstr($content,"ARG")){
            $vorpos = strpos($content,"ARG");
            $rest   = substr($content,$vorpos, strlen($content));
        }    

        /* Do we represent a valid group? */
        if (!$this->is_account && $this->parent === NULL){
            $display= "<img alt=\"\" src=\"images/small-error.png\" align=\"middle\">&nbsp;<b>".
                msgPool::noValidExtension(_("Phone macro"))."</b>";
            return ($display);
        }

        /* Fill templating stuff */
        $smarty= get_smarty();

        /* Add an empty Parameter */
        if(isset($_POST['addvar']) && preg_match("/w/",$ACLs)){
            if(!is_array($this->goFonMacroParameter)){
                $vars = $this->goFonMacroParameter;
                $this->goFonMacroParameter = array();
                $this->goFonMacroParameter[]= $vars;
            }
            $number= count($this->goFonMacroParameter);
            $number++;
            $this->goFonMacroParameter[]=array("var"=>"var","name"=>"new","type"=>"string","default"=>"test");
        }

        /*generate Table which shows als parameters */
        $FonParas = $this->check_paras($content,$this->goFonMacroParameter); 

        /* Sort by Parameterid, and keep keys */    
        ksort($FonParas);



        if(!preg_match("/r/",$ACLs)){
            $smarty->assign("readable",false);      
        }else{
            $smarty->assign("readable",true);      
            foreach($FonParas as $key=>$para)	{

                /* Select correct item of combobox */
                if(isset($para['type'])){
                    list($sel1, $sel2, $sel3)= $this->type_shortcut[$para['type']];
                }else{
                    list($sel1, $sel2, $sel3)= array("", "", "");
                }

                /* Disable all input fields if we are not allowed to change the parameters */ 
                $disabled = "";
                if(!preg_match("/w/",$ACLs)){
                    $key = "";
                    $disabled = " disabled ";
                }

                /* Assemble output table */
                $vars .="<tr>
                    <td>
                    <input name=\"number".$key."\" value='".$key."' type='hidden' ".$disabled.">
                    <input name='var".$key."' type='hidden'   value='".set_post($para['var'])."' ".$disabled.">ARG".$key."
                    </td>
                    <td><input type='text' name='varname".$key."'  value='".set_post($para['name'])."' ".$disabled."></td>
                    <td>
                    <select size=1 name='vartype".$key."'  ".$disabled.">
                    <option  value='string' ".$sel1.">"._("String")."&nbsp;</option>
                    <option  value='combo'   ".$sel2.">"._("Select box")."&nbsp;</option>
                    <option  value='bool'   ".$sel3.">"._("Boolean")."&nbsp;</option>
                    </select>
                    </td>
                    <td><input type='text' name='default".$key."'   value='".set_post($para['default'])."'  ".$disabled."></td>
                    <td>&nbsp;";
                if($para['check']==false) {
                    $vars.="<button type='submit' name='del".$key."'>"._("Delete unused")."</button>";
                }

                $vars.=" </td></tr>";
            }
        }

        /* Checkboxes */
        $smarty->assign("base_select", $this->base);
        $smarty->assign("vars", $vars);

        /* Show main page */
        return($smarty->fetch (get_template_path('parameter.tpl', TRUE)));
    }

    //! Unused here 
    /*!
      Unused here because goFonMacro will remove this Macro\n 
     */
    function remove_from_parent()
    {
    }

    //! Save our data
    /*! 
      Save POST data to object \n
      This gives us the possibility to leave a tab, without losing our typed informations\n
      \n
      Read the POST fields for the parameters and saves their info the the class\n
     */
    function save_object()
    {
        if (isset($_POST['phoneparameters'])){
            plugin::save_object();
        }
        /* read out post data, and assign it to the parameters */
        /* And or delete */
        foreach($_POST as $name=>$value){

            /* Test if there is a variable begining with "del" */
            if(preg_match("/^del/",$name)){

                /* Extract entry id to delete */
                $nr = str_replace("del","",$name) ;

                /* unset entry */
                unset($this->goFonMacroParameter[$nr]);

            }elseif(preg_match("/^number/",$name)){

                /* Set Post vars */
                $key = get_post($name);

                $this->goFonMacroParameter[$key]['var']    = get_post("var".$key);
                $this->goFonMacroParameter[$key]['name']   = get_post("varname".$key);
                $this->goFonMacroParameter[$key]['type']   = get_post("vartype".$key);
                $this->goFonMacroParameter[$key]['default']= get_post("default".$key);
            }
        }

    }


    //! Checks given values 
    /*! 
      Check values\n 
      If a user enters an invalid value, then this function will output an error msg\n
      (In better words :prepare the errormessages that will be put out )\n
     */
    function check()
    {
        /* Call common method to give check the hook */
        $message= plugin::check();

        foreach($this->goFonMacroParameter as $key=>$val){

            $v1 = utf8_encode($val['default']);
            $v2 = utf8_decode($val['default']);
            if($v1 != $v2){
                $allowed = preg_quote('[]()-+_,.;:/?*\ ','/');
                $message[] = msgPool::invalid(sprintf(_("Parameter %s contains invalid character!"), $val['name']),
                        $val['default'],"/[a-z0-9{$allowed}]/i");
            }
            switch($val['type']){
                case 'bool'   :   $possible = array("","0","1");
                                  if(!in_array($val['default'],$possible)) {
                                      $message[] = sprintf(_("Parameter %s is invalid!"),$val['name']);
                                  };break;
                case 'string' :
                case 'combo'  : 
                default : ;

            }
        }
        return $message;
    }

    //! Save changes to openldap
    /*!
      Save to LDAP 
      This function saves given attributes to the ldap
     */
    function save()
    {
        /* Post checks */

        plugin::save();

        $this->attrs['goFonMacroParameter']=array();

        foreach($this->goFonMacroParameter as $key=>$fonpara){
            $this->attrs['goFonMacroParameter'][]=$key."!".base64_encode($fonpara['name'])."!".$fonpara['type']."!".$fonpara['default'];
        }

        if($this->para_count != count($this->attrs['goFonMacroParameter'])){
            msg_dialog::display(_("Information"), _("Number of parameters for this macro has changed. Please update all users using it!"), INFO_DIALOG);
        }

        unset($this->attrs['base']);

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
            if(count($this->attrs['goFonMacroParameter']==0)){
                unset($this->attrs['goFonMacroParameter']);
            }		
            $ldap->cd($this->dn);
            $ldap->create_missing_trees( $this->dn);
            $ldap->cd($this->dn);
            $ldap->add($this->attrs);
            $this->handle_post_events("add");
        }
        if (!$ldap->success()){
            msg_dialog::display(_("LDAP error"), msgPool::ldaperror($ldap->get_error(), $this->dn, 0, get_class()));
        }
    }

    function PrepareForCopyPaste($source)
    {
        plugin::PrepareForCopyPaste($source);

        $source_o = new macroParameter($this->config,$source['dn']);
        $this->goFonMacroParameter = $source_o-> goFonMacroParameter;
    }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
