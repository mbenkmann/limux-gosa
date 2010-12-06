<?php

class mailqueue extends plugin
{
  /* Definitions */
  var $plHeadline     = "Mail queue";
  var $plDescription  = "View and control the mailservers mail processing queue";
  var $plIcon         = "plugins/mail/images/mailqueue.png";

  /* attribute list for save action */
  var $attributes     = array();
  var $objectclasses  = array();

  var $Server         = "all";
  var $ServerList     = array(); // The list of all available servers.
  var $Search         = "*";
  var $Time           = 0;
  var $Page           = 0;
  var $Stat           = "all";
  var $OrderBy        = "Arrival";
  var $SortType       = "up";
  var $disp_header    = false;
  var $range          = 20;   

  /* Logging detection */
  var $view_logged    = FALSE;

  function mailqueue(&$config, $dn= NULL)
  {
    $this->config   = &$config;
    $this->si_queue = new si_mailqueue($this->config);
    $this->getServer();

    // Create statistic table entry
    $this->initTime = microtime(TRUE);
    stats::log('plugin', $class = get_class($this), $category = array($this->acl_category),  $action = 'open',
            $amount = 1, $duration = (microtime(TRUE) - $this->initTime));

  }


  function execute()
  {
    /* Call parent execute */
    plugin::execute();

    /* Log view */
    if(!$this->view_logged){
      $this->view_logged = TRUE;
      new log("view","mailqueue/".get_class($this),$this->dn);
    }

    $smarty= get_smarty();
    $tmp = $this->plInfo();
    foreach($tmp['plProvidedAcls'] as $name => $desc){
      $smarty->assign($name."ACL",$this->getacl($name));
      $smarty->assign($name."_W",$this->acl_is_writeable($name));
    }
    $error =false;

    /******************
      Handle options 
     ******************/

    $action = $server = $entry = "";
    $types = array( 
        "all_del"     => "del",
        "all_hold"    => "hold",
        "all_unhold"  => "unhold",
        "all_requeue" => "requeue");
    foreach($_POST as $name => $value){
      foreach($types as $type => $target){
        if(preg_match("/^".$type."/",$name) && $this->acl_is_writeable($target."All")){
          $entry  = $this->list_get_selected_items();
          $action = $target;
          break;
        }
      }
      if(!empty($action)) break;
    }

    $types = array("del","hold","unhold","header","requeue");
    foreach($_POST as $name => $value){
      foreach($types as $type){
        if(preg_match("/^".$type."__/",$name) && $this->acl_is_writeable($type)){
          $action = $type;
          $server = preg_replace("/^".$type."__[^_]*__([^_]*)$/","\\1",$name); 
          $entry[$server][] = preg_replace("/^".$type."__([^_]*)__.*/","\\1",$name); 
          break;
        }
      }
      if(!empty($action)) break;
    }

    /* Send action for given mail id */
    if(in_array($action,array("del","hold","unhold","requeue"))){
      foreach($entry as $server => $entries){
        $this->si_queue->send_queue_action($entries,$server,$action);
      }
    }


    /******************
      Display mail header
     ******************/

    if($action == "header"){
      $server = key($entry);
      $entry = $entry[$server];

      /* Create table which displays the header informations */
      $data = $this->si_queue->header($entry,$server);
      $data = preg_replace("/([^\s]*:)/","\n\\1",$data);
      $this->disp_header = $data;
      if($this->si_queue->is_error()){
        msg_dialog::display(_("Error"),msgPool::siError($this->si_queue->get_error()),ERROR_DIALOG);
        $this->disp_header = FALSE;
      }
    }

    /* Back is posted from the header display page */
    if(isset($_POST['back'])){
      $this->disp_header = false;
    }

    /* If there is a header in disp_header, then display it */
    if($this->disp_header){
      $smarty->assign("header",$this->disp_header);
      return ($smarty->fetch (get_template_path('header.tpl', TRUE)));
    }


    /******************
      Query mailqueues 
     ******************/

    $entries = array();
    if($this->acl_is_readable("query")){
      $within_minutes = -1;
      if($this->Time != "nolimit"){
        $within_minutes = 60*60*$this->Time;
      }

      if($this->Server == "all"){
        $entries = array();
        foreach($this->ServerList as $mac => $name){
          if(!tests::is_mac($mac)) continue;
          $entries = array_merge($entries,$this->si_queue->query_mailqueue($mac,$this->Search,$within_minutes));
          if($this->si_queue->is_error()){
            msg_dialog::display(_("Error"),msgPool::siError($this->si_queue->get_error()),ERROR_DIALOG);
          }
        }
      }else{
        $entries = $this->si_queue->query_mailqueue($this->Server,$this->Search,$within_minutes);
        if($this->si_queue->is_error()){
          msg_dialog::display(_("Error"),msgPool::siError($this->si_queue->get_error()),ERROR_DIALOG);
        }
      }
    }

    /* Sort entries 
     */ 
    $data = array();
    foreach($entries as $entry){
      $data[uniqid($entry[$this->OrderBy])] = $entry;
    }

    /* Sort entries by given direction 
     */
    if($this->SortType == "down"){
      uksort($data, 'strnatcasecmp');
    }else{
      uksort($data, 'strnatcasecmp');
      $data = array_reverse($data);
    }

    $count = count($data);
    $entries = array_slice($data,$this->Page,$this->range);

    /* Add ServerName to results 
     */
    foreach($entries as $key => $data){
      $entries[$key]['ServerName'] = $this->ServerList[$data['Server']];
    }   
 
    /******************
      create html output 
     ******************/

    $smarty->assign("query_allowed",$this->acl_is_readable("query"));
    $smarty->assign("all_ok"        , count($entries));
    $smarty->assign("entries"       , $entries);
    $smarty->assign("plug"          , "?plug=".$_GET['plug']);

    $smarty->assign("r_stats"       , $this->getStats());
    $smarty->assign("stats"         , array_flip($this->getStats()));

    $smarty->assign("stat"          , $this->Stat);
    $smarty->assign("p_server"      , set_post($this->Server));
    $smarty->assign("p_servers"     , set_post($this->ServerList));
    $smarty->assign("p_serverKeys"  , set_post(array_flip($this->ServerList)));
    $smarty->assign("p_time"        , $this->Time);
    $smarty->assign("p_times"       , $this->getTimes());
    $smarty->assign("p_timeKeys"    , array_flip($this->getTimes()));
    $smarty->assign("search_for"    , set_post($this->Search));
    $smarty->assign("range_selector", range_selector($count, $this->Page, $this->range,"EntriesPerPage")); 
    $smarty->assign("OrderBy"       , set_post($this->OrderBy));

    /* Display sort arrow */
    if($this->SortType == "up"){
      $smarty->assign("SortType","<img src='images/lists/sort-up.png' alt='"._("up")."' border='0'>");
    }else{
      $smarty->assign("SortType","<img src='images/lists/sort-down.png' alt='"._("down")."' border='0'>");
    }

    return ($smarty->fetch (get_template_path('contents.tpl', TRUE)));
  }


  /* return selectable server 
   */
  function getServer()
  {
    $ret= array("all"=>_("All"));

    /* First of all, detect all servers that supports the mailqueue extension 
        -If this fails, the mailqueue(s) can't be queried.
     */
    $hosts          = $this->si_queue->get_hosts_with_module("mailqueue_com");
    $this->si_error = $this->si_queue->is_error();
    if(!count($hosts)){
      return(array());
    }    

    /* Create search filter and try to resolv mac to hostname 
     */
    $filter = "";
    foreach($hosts as $mac){
      $filter .= "(macAddress=".$mac.")";
    }
    $filter = "(&(objectClass=GOhard)(|".$filter."))";
    $res = get_list($filter,"no_acls",$this->config->current['BASE'],
        array("cn","macAddress"),GL_SUBSEARCH | GL_NO_ACL_CHECK); 

    /* Create result array 
     */
    foreach($hosts as $mac){
      $found = FALSE;
      foreach($res as $entry){
        if(preg_match("/^".preg_quote($mac, '/')."$/i",$entry['macAddress'][0])){
          $ret[$mac] = $entry['cn'][0];
          $found = TRUE;
          break;
        }
      }
      if(!$found){
        $ret[$mac] = $mac;
      }
    }
    $this->ServerList = $ret;
  }


  /* Return selectable times*/
  function getTimes()
  {
    $ret = array();
    $ret['nolimit']=_("no limit"); 
    foreach(array(1,2,4,8,12,24,36,48) as $i){
      if($i == 1){
        $ret[$i] = $i."&nbsp;"._("hour");
      }else{
        $ret[$i] = $i."&nbsp;"._("hours");
      }
    }
    return($ret);
  }


  /* Save post values*/
  function save_object($save_current= FALSE)
  {
    /* Update amount of entries displayed */
    if(isset($_POST['EntriesPerPage'])){
      $this->range = get_post('EntriesPerPage');
    }

    if(isset($_POST['p_server']) && isset($this->ServerList[$_POST['p_server']])){
      $this->Server = get_post('p_server');
    }

    if(isset($_POST['p_time'])){
      $this->Time = get_post('p_time');
    }
    if(isset($_POST['search_for'])){
      $this->Search = get_post('search_for');
    }
    if(isset($_POST['Stat'])){
      $this->Stat = get_post('Stat');
    }
    if((isset($_GET['start']))&&(is_numeric($_GET['start']))&&($_GET['start']>=0)){
      $this->Page = $_GET['start'];
    }

    if((isset($_GET['sort']))&&(!empty($_GET['sort']))){
      $old = $this->OrderBy;
      $this->OrderBy = $_GET['sort'];
      if($this->OrderBy == $old)
      {
        if($this->SortType== "up"){
          $this->SortType = "down";
        }else{
          $this->SortType = "up";
        }
      }
    }

  }

  /* Return stats */
  function getStats()
  {
    return(array(
          "all"     =>_("All"),
          "hold"    =>_("Hold"),
          "unhold"  =>_("Release"),
          "active"  =>_("Active"),
          "nonactive"  =>_("Not active")
          ));
  }

  /* Return plugin informations for acl handling
     #FIXME You can only read attributes within this report plugin */
  static function plInfo()
  {
    return (array(
        "plShortName"   => _("Mail queue"),
        "plDescription" => _("Mail queue add-on"),
        "plSelfModify"  => FALSE,
        "plDepends"     => array(),
        "plPriority"    => 1,
        "plSection"     => array("addon"),
        "plCategory"    => array("mailqueue" => array("description" => _("Mail queue add-on"))),

        "plProvidedAcls" => array(
            "unholdAll"       => _("Release all messages"),
            "holdAll"         => _("Hold all messages"),
            "delAll"          => _("Delete all messages"),
            "requeueAll"      => _("Re-queue all messages"),
            "unhold"          => _("Release message"),
            "hold"            => _("Hold message"),
            "del"             => _("Delete message"),
            "requeue"         => _("Re-queue message"),
            "query"           => _("Gathering queue data"),
            "header"          => _("Get header information")
          )
        ));
  }

  function list_get_selected_items()
  {
    $ids = array();
    foreach($_POST as $name => $value){
      if(preg_match("/^selected_*/",$name)){
        $server = preg_replace("/^selected_.*_/","",$name) ;
        $ids[$server][] = preg_replace("/^selected_([^_]*)_.*$/","\\1",$name);
      }
    }
    return($ids);
  }



}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
