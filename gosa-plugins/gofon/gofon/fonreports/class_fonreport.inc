<?php

class fonreport extends plugin
{
    /* Definitions */
    var $plHeadline= "Phone Reports";
    var $plDescription= "View the phone report for incoming and outgoing calls";
    var $plIcon       = "plugins/gofon/images/phonereport.png";

    var $ui             = NULL;

    /* For internal use */
    var $start          = 0;
    var $search_for     = "*";
    var $search_base    = "";
    var $year           = "";
    var $month          = "";
    var $sort           = 0;
    var $sort_direction = "down";
    var $range          = 20;
    var $selected_server= "";

    /* attribute list for save action */
    var $fields         = array("calldate", "src", "dst", "channel", "lastapp", "disposition", "duration");
    var $attributes_SO  = array("start","search_for","search_base","range","month","sort_direction","sort","year");
    var $objectclasses  = array();
    var $view_logged = FALSE;

    var $telephoneNumberToDN = array();

    /* Construct class */
    function fonreport (&$config, $ui)
    {
        /* Include config object */
        $this->initTime = microtime(TRUE);
        stats::log('plugin', $class = get_class($this), $category = array($this->acl_category),  $action = 'open',
                $amount = 1, $duration = (microtime(TRUE) - $this->initTime));

        $this->config   = $config;
        $this->ui      = $ui;
        $this->search_base= $this->config->current['BASE'];

        $this->month      = date("m");
        $this->year       = date("Y");

        /* Use filter settings if we have already searched */
        if (!session::is_set("fonfilter")){
            $fonfilter = array();
            foreach($this->attributes_SO as $name){
                $fonfilter[$name]=$this->$name;
            }
            session::set("fonfilter", $fonfilter);
        }else{
            $fonfilter = session::get("fonfilter");
            foreach($this->attributes_SO as $name){
                $this->$name = $fonfilter[$name];
            }
        }

        // Get ALL valid FAX-Accounts and their dns, this allows us to perform correct
        //  permissions checks later.
        $filter= "(&(objectClass=gosaAccount)(!(objectClass=gosaUserTemplate))".      
            "(objectClass=goFonAccount)(telephoneNumber=*))";
        $tmp= get_list($filter, "users/viewFonEntries", $this->search_base,
                array("telephoneNumber"), GL_SUBSEARCH | GL_NO_ACL_CHECK);
        $this->telephoneNumberToDN = array();
        foreach($tmp as $attrs){
            for($i=0;$i<$attrs['telephoneNumber']['count']; $i++){
                $this->telephoneNumberToDN[$attrs['telephoneNumber'][$i]] = $attrs['dn'];
            }
        }
    }


    /* Save ui interactions and store results in session, 
       to remember settings */
    function save_object()
    {
        $fonfilter= session::get("fonfilter");
        if(isset($_POST['EntryPerPage'])){
            $this->range = get_post('EntryPerPage');
        }
        if (isset($_GET['start'])){
            $this->start= (int)$_GET['start'];
        }
        foreach( array("year", "month", "search_for","selected_server") as $type){
            if (isset($_POST[$type])){
                $this->$type= get_post($type);
            }
        }

        /* Adapt sorting */
        if (isset($_GET['sort'])){
            if ($this->sort == (int)$_GET['sort']){
                if ($this->sort_direction == "down"){
                    $this->sort_direction= "up";
                } else {
                    $this->sort_direction= "down";
                }
            }
            $this->sort= (int)$_GET['sort'];
            if ($this->sort < 0 || $this->sort > 6){
                $this->sort= 0;
            }
        }

        /* remove unwanted tags */
        $this->search_for = stripslashes(preg_replace("/[^0-9a-z\*\+ \-\/]/i","",$this->search_for));

        foreach($this->attributes_SO as $name){
            $fonfilter[$name] = $this->$name;
        }
        session::set("fonfilter", $fonfilter);
    }


    /* Search & display results */
    function execute()
    {
        /* Call parent execute */
        plugin::execute();

        // Use the configured global timezone settings 
        timezone::get_default_timezone();

        /* GVet template engine */
        $smarty= get_smarty();

        /* Log view */
        if(!$this->view_logged){
            $this->view_logged = TRUE;
            new log("view","fonreport/".get_class($this),$this->dn);
        }

        /*****************
          Variable Init
         *****************/

        $fields_str = "";

        $months= array();
        for($i = 1 ; $i <= 12 ; $i ++ ){
            $months[$i] = _(date("F",gmmktime(0,0,0,$i)));
        }

        /* Prepare template */
        $current= date("Y");
        $years= array();
        for ($y= $current - 5; $y<=$current; $y++){
            $years[$y]= $y;
        }

        /*****************
          Smarty 
         *****************/

        $smarty->assign("plug",           "?plug=".validate($_GET['plug']));
        $smarty->assign("search_for",     set_post($this->search_for));
        $smarty->assign("months",         $months);
        $smarty->assign("month_select",   $this->month);
        $smarty->assign("years",          $years);
        $smarty->assign("year_select",    $this->year);
        $smarty->assign("search_result",  "");


        /*****************
          Check Database , Table , Connection 
         *****************/

        // Collect servers and allow to select the server in the ui.
        $servers = array();
        foreach($this->config->data['SERVERS']['FON'] as $key => $server){
            $servers[$server['SERVER']] = $server['SERVER'];
        }
        $smarty->assign("servers",  set_post($servers));
        $smarty->assign("selected_server",  set_post($this->selected_server));

        /* Connecting, selecting database */
        if (!isset($this->config->data['SERVERS']['FON'][0])){
            msg_dialog::display(_("Configuration error"), msgPool::noserver(
                        _("GOfon")), WARNING_DIALOG);
            return($smarty->fetch(get_template_path('contents.tpl', TRUE)));
        }elseif(!is_callable("mysql_connect")){
            msg_dialog::display(_("Configuration error"), msgPool::missingext("php-mysql"), WARNING_DIALOG);
            return($smarty->fetch(get_template_path('contents.tpl', TRUE)));
        }else{

            // Get CFG for the selected server, if empty use first.
            if($this->selected_server == ""){
                $cfg= $this->config->data['SERVERS']['FON'][0];
            }else{
                foreach($this->config->data['SERVERS']['FON'] as $server){
                    if($server['SERVER'] == $this->selected_server){
                        $cfg = $server;
                    }
                }
            }

            $link = @mysql_pconnect($cfg['SERVER'], $cfg['LOGIN'], $cfg['PASSWORD']);
            if ($link === FALSE){
                msg_dialog::display(_("Error"), msgPool::dbconnect(
                            _("GOfon"),@mysql_error(),$cfg['SERVER']),ERROR_DIALOG);
                return($smarty->fetch(get_template_path('contents.tpl', TRUE)));
            }
            if (! @mysql_select_db("gophone")){
                msg_dialog::display(_("Error"), msgPool::dbselect(
                            _("GOfon"),@mysql_error(),$cfg['DB']),ERROR_DIALOG);
                return($smarty->fetch(get_template_path('contents.tpl', TRUE)));
            }
        }


        /*****************
          Get Query String && Search
         *****************/

        // Get entries for the selected range only.
        $link   = @mysql_pconnect($cfg['SERVER'], $cfg['LOGIN'], $cfg['PASSWORD']);
        @DEBUG (DEBUG_MYSQL, __LINE__, __FUNCTION__, __FILE__,$query, "Database query");
        $query  = $this->CreateQuerySyntax();
        $result = @mysql_query($query);
        if ($result === false){
            msg_dialog::display(_("Error"), msgPool::dbquery(
                        _("GOfon"),@mysql_error(),$cfg['SERVER']),ERROR_DIALOG);
            return($smarty->fetch(get_template_path('contents.tpl', TRUE)));
        }


        /*****************
          Fetch results 
         *****************/

        $report_list= array();

        /* Restricted attributes will not be displayed, this will be displayed instead */
        $no_acl = "<img class='center' src='images/lists/locked.png'
            title='".msgPool::permView()."' alt='"._("Insufficient permissions")."'>";

        $no_acl = "&nbsp;-";

        while ($line = mysql_fetch_array($result, MYSQL_ASSOC)) {

            // Get dn to check ACLs for
            // Existing ldap-user -> use its dn
            // Not existing user  -> use ldap base dn
            $dn = $this->search_base;
            if(isset($this->telephoneNumberToDN[$line['src']])){
                $dn = $this->telephoneNumberToDN[$line['src']];
            }
            // We do not have any ACLs for this entry, so continue.
            $acls = $this->ui->get_permissions($dn,"users/viewFonEntries","");
            if(!preg_match("/r/",$acls)){
                continue;
            }

            // Check attribute permissions
            foreach($line as $attr => $value){
                if($attr == "duration") continue;
                $acl = $this->ui->get_permissions($dn,"users/viewFonEntries",$attr);
                if(!preg_match("/r/",$acl)){
                    $line[$attr] = $no_acl;
                }
            }

            // Check date permissions
            if($this->ui->get_permissions($dn,"users/viewFonEntries","calldate")){
                $hour=  substr($line["calldate"], 11, 2);
                $minute=substr($line["calldate"], 14, 2);
                $format= _("Y-M-D");
                $date= preg_replace("/Y/", substr($line["calldate"], 0, 4), $format);
                $date= preg_replace("/M/", substr($line["calldate"], 5, 2), $date);
                $date= preg_replace("/D/", substr($line["calldate"], 8, 2), $date);
                $date_str = $date." ".$hour.":".$minute;
            }else{
                $date_str = $no_acl;
            }

            $append_str =  "<td class='list1'>".$date_str."</td>";
            foreach(array("src","dst","channel","lastapp","disposition") as $atr){
                if(isset($line[$atr])){
                    $append_str .=  "<td class='list1'>".$line[$atr]."</td>";
                }
            }
            if($this->ui->get_permissions($dn,"users/viewFonEntries","duration")){
                $append_str .=  "<td class='list1' style='border-right: 0pt none;'>".
                    $this->gen_duration($line["duration"])."</td>";
            }else{
                $append_str .=  "<td class='list1' style='border-right: 0pt none;'>".
                    $no_acl."</td>";
            }
            $report_list[] = $append_str;
        }

        @mysql_close($link);


        /*****************
          Create list of results  
         *****************/

        $res_count = count($report_list);
        $report_list = array_slice($report_list, $this->start, $this->range);
        if($res_count < $this->start) $this->start = 0;

        /* Generate output */
        $output ="";
        foreach($report_list as $val){
            $output.= "<tr>$val</tr>";
        }

        /*****************
          Tell smarty what we have found  
         *****************/

        $smarty->assign("search_result", $output);
        $smarty->assign("range_selector", range_selector($res_count, $this->start,$this->range,"EntryPerPage"));

        /* Show main page */
        for($i= 0; $i<7; $i++){
            $smarty->assign("mode$i", "");
        }
        $smarty->assign("mode".$this->sort, image("images/lists/sort-".$this->sort_direction.".png"));

        return($smarty->fetch(get_template_path('contents.tpl', TRUE)));
    }


    /* Create duration string   12'11" */
    function gen_duration($seconds)
    {
        if ($seconds / 60 > 1){
            $minutes= (int)($seconds / 60);
            $seconds= $seconds % 60;
            return ("$minutes&rsquo;$seconds&rdquo;");
        }
        return ("$seconds&rdquo;");
    }


    /* Create WHERE part for our mysql serach */
    function GetUidMatchingFilter()
    {
        $ldap= $this->config->get_ldap_link();
        $ldap->cd($this->search_base);
        $s = $this->search_for;

        $s = preg_replace("/\%/","",$s);  
        $s2 = preg_replace("/\*/","%",$s);  

        $filter = "(&(objectClass=gosaAccount)(!(objectClass=gosaUserTemplate))".
            "(|(uid=$s)(l=$s)(homePhone=$s)(telephoneNumber=$s)(facsimileTelephoneNumber=$s)(mobile=$s)".
            "(pager=$s)(cn=$s)(givenName=$s)(sn=$s)(personalTitle=$s)(title=$s)))";

        $attrs  = array("uid");
        $res    = get_list($filter,"users/viewFonEntries",$this->search_base,$attrs);

        $str = " AND (";
        $fields = array("dstchannel","channel");
        if(count($res)){
            foreach($res as $attrs){
                $uid =  $attrs["uid"][0];
                foreach($fields as $name){
                    $str .= $name." like '%".$uid."%' OR ";
                }
            }
        }
        $str .= " channel     like '%".$s."%' OR  
            dstchannel  like '%".$s."%' OR  
            dst         like '%".$s2."%' OR 
            src         like '%".$s2."%' OR 
            lastapp     like '%".$s2."%')"; 
            return($str);
    }


    /* Create query string */
    function CreateQuerySyntax()
    {
        /* Get extended search filter which contain uids and so on */
        $uidstring = $this->GetUidMatchingFilter();

        /* Create string with all fields seperated by ,*/
        $fields_str ="";
        foreach($this->fields as $field){
            if($field == "calldate") {
                continue; 
            }
            $fields_str .= $field.", ";
        }
        $fields_str = preg_replace("/, $/","",$fields_str);

        /* Create Sort tag */
        if ($this->sort_direction == "down"){
            $desc= "DESC";
        } else {
            $desc= "ASC";
        }
        /* Create times */
        $start= date ("YmdHis", mktime(0,0,0,$this->month,1,$this->year));
        $end=   date ("YmdHis", mktime(23,59,59,($this->month +1),0,$this->year));

        /* Create string with all fields seperated by ,*/
        $fields_str ="";
        foreach($this->fields as $field){
            if($field == "calldate") continue;
            $fields_str .= $field.", ";
        }
        $fields_str = preg_replace("/, $/","",$fields_str);
        $fieldset = $fields_str.",calldate"; 

        $query = "SELECT {$fieldset} FROM cdr ".
            "WHERE   
            calldate <= ".mysql_real_escape_string($end)." 
            AND 
            calldate >= ".mysql_real_escape_string($start)." 
            ". $uidstring." 
            ORDER BY ".$this->fields[$this->sort]." $desc";

        $query.=";";
        return($query);
    }
}

class viewFonEntries{

    static function plInfo()
    {
        return (array(
                    "plShortName"   => _("View phone reports"),
                    "plDescription" => _("View phone reports")."&nbsp;<i>"._("All entries are read-only")."</i>",
                    "plSelfModify"  => TRUE,
                    "plDepends"     => array(),
                    "plPriority"    => 88,
                    "plSection"     => array("administration"),
                    "plCategory"    => array("users"),

                    "plProvidedAcls" => array(
                        "calldate"      =>_("Date"), 
                        "src"           =>_("Source"), 
                        "dst"           =>_("Destination"), 
                        "channel"       =>_("Channel"), 
                        "lastapp"       =>_("Application called"),
                        "disposition"   =>_("Disposition"), 
                        "duration"      =>_("Duration"))
                    ));
    }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
