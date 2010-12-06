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

class krb5_policy extends plugin
{
    var $name   = "";
    var $data   = array();
    var $is_new = TRUE;
    var $init_name ="";
    var $parent ;
    var $config ;
    var $attributes = array("MASK","POLICY_REFCNT","PW_HISTORY_NUM","PW_MAX_LIFE",
            "PW_MIN_CLASSES","PW_MIN_LENGTH","PW_MIN_LIFE");


    /*! \brief Initializes the policy class.
      @param Object The GOsa configuration object. 
      @param Array  The policy data array to edit or an empty array()
      @param Object The parent object.
     */
    public function __construct($config,$entry,$parent)
    {
      $this->initTime = microtime(TRUE);
        $this->config = $config;
        $this->parent = $parent;
        if(count($entry)){
            $this->is_new =FALSE;
            $this->name = $entry['NAME'];
            $this->data = $entry['DATA'];
        }else{
            $this->data['MASK']           = 0;
            $this->data['POLICY_REFCNT']  = 0;
            $this->data['PW_HISTORY_NUM'] = 5;
            $this->data['PW_MAX_LIFE']    = 604800;
            $this->data['PW_MIN_CLASSES'] = 3;
            $this->data['PW_MIN_LENGTH']  = 6;
            $this->data['PW_MIN_LIFE']    = 36000;
        }
        $this->init_name = $this->name;

        // Create statistic table entry
        stats::log('plugin', $class = get_class($this), $category = array($this->acl_category),  $action = 'open',
                $amount = 1, $duration = (microtime(TRUE) - $this->initTime));

    }


    /*! \brief  Returns a HTML ui which allows 
      configuring this policy 
      @return String a HTML interface.
     */
    public function execute()
    {
        $display = plugin::execute();
        $smarty = get_smarty();    

        $smarty->assign("name", set_post($this->name));
        foreach($this->attributes as $attr){
            $smarty->assign($attr, set_post($this->data[$attr]));
        }

        $smarty->assign("POLICY_REFCNT", sprintf("<i>"._("This policy is referenced %d times.")."</i>", $this->data["POLICY_REFCNT"]));
        return($smarty->fetch(get_template_path("krb5_policy.tpl",TRUE,dirname(__FILE__))));
    }


    /*! \brief  Saves the HTML posted variables 
     */
    public function save_object()
    {
        if(isset($_POST['Policy_Posted'])){
            if(isset($_POST['name'])){
                $this->name = get_post("name");
            }
            foreach($this->attributes as $attr){
                if(isset($_POST[$attr])){
                    $this->data[$attr] = get_post($attr);
                }
            }
        }
    }


    /*! \brief Checks the given input 
      @return Array   Containing errors about incorrect values.
     */ 
    public function check()
    {
        $message = array();
        $names = $this->parent->getPolicyNames();
        if($this->name != $this->init_name && in_array($this->name,$names)){
            $message[] = msgPool::duplicated(_("Policy name"));
        }
        if(empty($this->name)){
            $message[] = msgPool::required(_("Policy name"));
        }

        if(!preg_match("/^[a-z0-9\@\.\-_]*$/i",$this->name)){
            $message[] = msgPool::invalid(_("Policy name"),$this->name,"/[a-z0-9]/i");
        }

        /* Check password history */
        if(!is_numeric($this->data['PW_HISTORY_NUM'])){
            $message[] = msgPool::invalid(_("Password history size"));
        }elseif($this->data['PW_HISTORY_NUM'] <= 0){
            $message[] = msgPool::toosmall(_("Password history size"));
        }

        /* Check password minimum length */
        if(!is_numeric($this->data['PW_MIN_LENGTH'])){
            $message[] = msgPool::invalid(_("Minimum password length"));
        }elseif($this->data['PW_MIN_LENGTH'] <= 0){
            $message[] = msgPool::toosmall(_("Minimum password length"));
        }

        /* Check password different character classes */
        if(!is_numeric($this->data['PW_MIN_CLASSES'])){
            $message[] = msgPool::invalid(_("Required different characters"));
        }elseif($this->data['PW_MIN_CLASSES'] <= 0){
            $message[] = msgPool::toosmall(_("Required different characters"));
        }elseif($this->data['PW_MIN_CLASSES'] > $this->data['PW_MIN_LENGTH']){
            $message[] = sprintf(_("The value specified for '%s' must be smaller than the value specified for '%s'!"),
                    _("Required different characters"),_("Minimum password length"));
        }

        /* Check password min lifetime */
        if(!is_numeric($this->data['PW_MIN_LIFE'])){
            $message[] = msgPool::invalid(_("Minimum password lifetime"));
        }elseif($this->data['PW_MIN_LIFE'] <= 0){
            $message[] = msgPool::toosmall(_("Minimum password lifetime"));
        }

        /* Check password lifetime */
        if(!is_numeric($this->data['PW_MAX_LIFE'])){
            $message[] = msgPool::invalid(_("Password lifetime"));
        }elseif($this->data['PW_MAX_LIFE'] <= 0){
            $message[] = msgPool::toosmall(_("Password lifetime"));
        }elseif($this->data['PW_MAX_LIFE'] < $this->data['PW_MIN_LIFE']){
            $message[] = sprintf(_("The value specified for '%s' must be smaller than the value specified for '%s'!"),
                    _("Minimum password lifetime"),_("Password lifetime"));
        }

        return($message);
    }


    /*! \brief  Returns object data of the currently edited policy 
      @return Array   A multidimensional array containing policy informations.
     */
    public function save()
    {
        $ret = array();
        $ret['NAME']        = $this->name;
        $ret['DATA']        = $this->data;
        return($ret);
    }
}

// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
