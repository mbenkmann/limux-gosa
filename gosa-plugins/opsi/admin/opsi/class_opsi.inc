<?php
/*
   This code is part of GOsa (https://gosa.gonicus.de)
   Copyright (C) 2008  Fabian Hickert

   This program is free software: you can redistribute it and/or modify
   it under the terms of the GNU General Public License as published by
   the Free Software Foundation, either version 3 of the License, or
   (at your option) any later version.

   This program is distributed in the hope that it will be useful,
   but WITHOUT ANY WARRANTY; without even the implied warranty of
   MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
   GNU General Public License for more details.

   You should have received a copy of the GNU General Public License
   along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */


/********

  public function __construct($config)
  public function enabled()
  function get_hosts_for_system_management()
  private function xml_to_array($xml,$alternative_method = FALSE)
  public function send_action($type,$hostId,$mac)
  public function list_clients( $hostId = "")
  public function add_client($hostId,$macaddress,$notes,$description)
  public function modify_client($hostId,$mac,$notes,$description)
  public function get_netboot_products($host = "")
  public function get_local_products($host = "")
  public function get_product_properties($productId,$hostId = "")
  public function set_product_properties($productId,$cfg,$hostId = "")
  public function add_product_to_client($productId,$hostId)
  public function del_product_from_client($productId,$hostId)
  public function get_client_hardware($hostId)
  public function get_client_software($hostId)
  public function del_client($hostId)
  public function job_opsi_install_client($hostId,$mac)

 ********/


/*! \brief  This is the opsi base class, it handles 
  .          gosa daemon requests and prepares data for opsi plugins.
 */
class opsi extends gosaSupportDaemon 
{
    private $config = NULL;
    protected $use_alternative_xml_parse_method = TRUE;
    protected $target = "";

    /*! \brief 		Create opsi object.
      @param
      @return 		
     */
    public function __construct($config)
    {
        $this->config = $config;
        gosaSupportDaemon::__construct($config);

        /* Detect the target opsi host 
         */
        if($this->config->pluginEnabled("faiManagement")){
            $opsi_hosts = $this->get_hosts_with_module("opsi_com");

            /* Just use the first result of the opsi hosts 
             */
            if(count($opsi_hosts) == 1 && isset($opsi_hosts[0])){
                $this->target = $opsi_hosts[0];
            }elseif(count($opsi_hosts) > 1){
                $this->target = $opsi_hosts[0];
                msg_dialog::display(_("Opsi"),sprintf(_("More than one Opsi server were found, using the first result '%s'."),$this->target));
            }
        }
    }


    public function enabled()
    {
        return($this->config->pluginEnabled('faiManagement') && !empty($this->target));
    }


    /******************
      Opsi handling 
     ******************/

    function get_hosts_for_system_management()
    {
        $res = $this->list_clients();
        $data = array();
        $ui = get_userinfo();
        foreach($res as $entry){
            if(!isset($entry['MAC'][0]['VALUE'])) $entry['MAC'][0]['VALUE'] = "";
            $obj = array(
                    "dn"          => "opsi:=".$entry['NAME'][0]['VALUE'].",".get_ou("wingeneric", "sambaMachineAccountRDN").$this->config->current['BASE'],
                    "objectClass" => array("gosa_opsi_client"),
                    "cn"          => array(0 => $entry['NAME'][0]['VALUE']),
                    "macAddress"  => array(0 => $entry['MAC'][0]['VALUE']),
                    "opsi_notes"  => array(0 => $entry['NOTES'][0]['VALUE']));

            /* Check permissions */
            $opsi_acl = $ui->get_permissions($obj['dn'],"opsi/opsiGeneric");
            if(preg_match("/r/",$opsi_acl)){
                if(!empty($entry['DESCRIPTION'][0]['VALUE'])){ 
                    $obj["description"]= array();
                    $obj["description"][0]= $entry['DESCRIPTION'][0]['VALUE'];
                }
                $data[] = $obj;
            }
        }

        return($data);
    }


    /*! \brief  Maps all xml to array conversion to an alternative method
      then used in the parent class 'gosaSupportDaemon'.
      The alternative method is able to handle more complex data.
     */
    private function xml_to_array($xml,$alternative_method = FALSE)
    {
        return(gosaSupportDaemon::xml_to_array($xml,TRUE));
    }


    /*! \brief  Trigger an event like wake or install for a specific hostId. 
     */
    public function send_action($type,$hostId,$mac)
    {
        switch($type){
            case 'install'  :  $this->job_opsi_install_client($hostId,$mac); break;
            default         :  trigger_error('Unknown type '.$type.'.');
        }
    }


    public function job_opsi_activate_client($id,$mac)
    {
        $events = DaemonEvent::get_event_types(SYSTEM_EVENT | HIDDEN_EVENT);
        $o_queue = new gosaSupportDaemon();
        if(isset($events['TRIGGERED']['DaemonEvent_activate'])){
            $evt = $events['TRIGGERED']['DaemonEvent_activate'];
            $tmp = new $evt['CLASS_NAME']($this->config);
            $tmp->set_type(TRIGGERED_EVENT);
            $tmp->add_targets(array($mac));
            if(!$o_queue->append($tmp)){
                msg_dialog::display(_("Service infrastructure"),msgPool::siError($o_queue->get_error()),ERROR_DIALOG);
            }
        }
    }



    /******************
      SI Communication functions
     ******************/



    /*! \brief 		Returns a list of all opsi clients.
      @param
      @return 		
     */
    public function list_clients( $hostId = "")
    {
        $data   = array();
        $res    = $this->send_data("gosa_opsi_list_clients",$this->target,$data,TRUE);
        $items  = array();
        if(isset($res['XML'][0]['ITEM'])){
            $items = $res['XML'][0]['ITEM'];
        }
        return($items);
    }


    /*! \brief 		Adds a new opsi client.
      @param
      @return 		
     */
    public function add_client($hostId,$macaddress,$notes,$description)
    {
        $data = array("hostId" => $hostId,"macaddress" => $macaddress);

        if(empty($hostId)){
            trigger_error("No valid host id given, check parameter 1.");
            return;
        }

        /* Add optional attributes */ 
        foreach(array("notes","description") as $attr) {
            if(!empty($$attr)){
                $data[$attr] = $$attr;
            }
        }

        /* Query SI server */
        $res    = $this->send_data("gosa_opsi_add_client",$this->target,$data,TRUE);
    }


    /*! \brief 		Modify an opsi client.
      @param
      @return 		
     */
    public function modify_client($hostId,$mac,$notes,$description)
    {
        $data = array("hostId" => $hostId,"mac" => $mac);

        if(empty($hostId)){
            trigger_error("No valid host id given, check parameter 1.");
            return;
        }

        /* Add optional attributes */ 
        foreach(array("notes","description") as $attr) {
            $data[$attr] = $$attr;
        }

        /* Query SI server */
        $res = $this->send_data("gosa_opsi_modify_client",$this->target,$data,TRUE);
    }



    /*! \brief 		Returns a list of netboot products.
      @param
      @return 		
     */
    public function get_netboot_products($host = "")
    {
        /* Append host attribute to query data 
         */
        $data = array();
        if(!empty($host)){
            $data['hostId'] = trim($host);
        }

        $res    = $this->send_data("gosa_opsi_get_netboot_products",$this->target,$data,TRUE);
        $items = array();
        if(isset($res['XML'][0]['ITEM'])){
            foreach($res['XML'][0]['ITEM'] as $entry){
                $e = array("DESC" => $entry['DESCRIPTION'][0]['VALUE'],
                        "NAME" => $entry['PRODUCTID'][0]['VALUE']);
                $items[$entry['PRODUCTID'][0]['VALUE']] = $e;
            }
        }
        return($items);
    }


    /*! \brief 		Returns a list of all local products.
      @param
      @return 		
     */
    public function get_local_products($host = "")
    {
        /* Append host attribute to query data 
         */
        $data = array();
        if(!empty($host)){
            $data['hostId'] = trim($host);
        }

        $res    = $this->send_data("gosa_opsi_get_local_products",$this->target,$data,TRUE);
        $items = array();
        if(isset($res['XML'][0]['ITEM'])){
            foreach($res['XML'][0]['ITEM'] as $entry){
                $e = array("DESC" => $entry['DESCRIPTION'][0]['VALUE'],
                        "NAME" => $entry['PRODUCTID'][0]['VALUE']);
                $items[$entry['PRODUCTID'][0]['VALUE']] = $e; 
            }
        }
        return($items);
    }


    /*! \brief 		Returns a list of netboot products.
      @param
      @return 		
     */
    public function get_full_product_host_information($host = "")
    {
        /* Append host attribute to query data 
         */
        $data = array();
        if(!empty($host)){
            $data['hostId'] = trim($host);
        }

        $res    = $this->send_data("gosa_opsi_get_full_product_host_information",$this->target,$data,TRUE);
        $items = array();
        $attrs = array("TYPE","PRIORITY","ONCESCRIPT","LICENSEREQUIRED","PACKAGEVERSION","PRODUCTVERSION",
                "ADVICE","SETUPSCRIPT","WINDOWSSOFTWAREIDS","PXECONFIGTEMPLATE","NAME","CREATIONTIMESTAMP", "TYPE",
                "ALWAYSSCRIPT","PRODUCTID","DESCRIPTION","UNINSTALLSCRIPT","UPDATESCRIPT","PRODUCTCLASSNAMES");

        if(isset($res['XML'][0]['ITEM'])){
            foreach($res['XML'][0]['ITEM'] as $entry){
                $e = array();
                foreach($attrs as $attr){
                    if(isset($entry[$attr])){
                        foreach($entry[$attr] as $key => $value){
                            if(isset($value['VALUE'])){
                                $e['data'][$attr] = $value['VALUE'];
                            }elseif($value['ELEMENT']){
                                foreach($value['ELEMENT'] as $element){
                                    $e['data'][$attr][] = $element['VALUE'];
                                }
                            }
                        }
                    }
                }

                $e["configurable"] = FALSE;
                if(isset($entry['PROPERTIES']) && count($entry['PROPERTIES'])){
                    $e["configurable"] = TRUE;
                    $p_data = array();

                    foreach($entry['PROPERTIES'][0] as $p_name => $p_values){
                        if(empty($p_values)) continue;
                        $p_data[$p_name]= array();
                        foreach(array('CURRENT','DEFAULT','DESCRIPTION') as $p_tmp){
                            if(isset($p_values[0][$p_tmp])){
                                if(isset($p_values[0][$p_tmp][0]['VALUE'])){
                                    $p_data[$p_name][$p_tmp] = $p_values[0][$p_tmp][0]['VALUE'];
                                }
                            }
                        }

                        if(isset($p_values[0]['VALUES'][0]['ELEMENT'])){
                            foreach($p_values[0]['VALUES'][0]['ELEMENT'] as $val){
                                $p_data[$p_name]['VALUE'][] = $val['VALUE'];
                            }
                            $p_data[$p_name]['VALUE_CNT'] = count($p_data[$p_name]['VALUE']);
                        } 
                    }

                    $e["data"]['PROPERTIES'] = $p_data;
                }


                $e["installed"] = FALSE;
                if(isset($entry['ACTIONREQUEST'])){
                    $e["installed"] = TRUE;
                }

                $e["requires_licence"] = FALSE;
                if(isset($e['data']['LICENSEREQUIRED']) && preg_match("/true/i",$e['data']['LICENSEREQUIRED'])){
                    $e["requires_licence"] = TRUE;
                }
                $items[$entry['PRODUCTID'][0]['VALUE']] = $e;
            }
        }
        return($items);
    }


    /*! \brief 		Returns a list of all product properties. \ 
      .           Additionally you can specify the host parameter to \
      .           get host specific product properties
      @param
      @return 		
     */
    public function get_product_properties($productId,$hostId = "")
    {
        $data = array("productId" => $productId);

        /* Append host attribute to query data 
         */
        if(!empty($hostId)){
            $data['hostId'] = trim($hostId);
        }

        /* Check parameter */ 
        if(empty($productId)){
            trigger_error("No valid product id given, check parameter 1.");
            return(array());
        }

        /* Query SI server */
        $res    = $this->send_data("gosa_opsi_get_product_properties",$this->target,$data,TRUE);
        $items  = array();
        if(isset($res['XML'][0]['ITEM'])){   
            foreach($res['XML'][0]['ITEM'] as $entry){
                foreach($entry as $name => $val){

                    foreach(array("DESCRIPTION","CURRENT") as $attr){
                        $items[$name][$attr] = "";
                        if(isset($val[0][$attr])){
                            $items[$name][$attr] = $val[0][$attr][0]['VALUE'];
                        }
                    }
                    $items[$name]['VALUE'] = array();
                    if(isset($val['0']['VALUE'])){
                        foreach($val['0']['VALUE'] as $value){
                            $items[$name]['VALUE'][] = $value['VALUE'];
                        }
                    }
                    $items[$name]['VALUE_CNT'] = count($items[$name]['VALUE']);
                }
            }
        }
        return($items);
    }


    /*! \brief 		Set product properties, globally or per host. 
      @param
      @return 		
     */
    public function set_product_properties($productId,$cfg,$hostId = "")
    {
        $data = array("productId" => $productId);

        /* Append host attribute to query data 
         */
        if(!empty($hostId)){
            $data['hostId'] = trim($hostId);
        }

        /* Check parameter */ 
        if(empty($productId)){
            trigger_error("No valid product id given, check parameter 1.");
            return(array());
        }

        if(!count($cfg)) return;

        /* Add properties */
        $data['item'] = array();
        foreach($cfg as $name => $value){
            $data['item'][] = "<name>".$name."</name><value>".$value['CURRENT']."</value>";
        }  

        /* Query SI server */
        $res    = $this->send_data("gosa_opsi_set_product_properties",$this->target,$data,TRUE);
    }


    /*! \brief 		Adds a given product to a client.
      @param
      @return 		
     */
    public function add_product_to_client($productId,$hostId)
    {
        $data = array("productId" => $productId,"hostId" => $hostId);

        /* Check parameter */ 
        if(empty($productId)){
            trigger_error("No valid product id given, check parameter 1.");
            return;
        }
        if(empty($hostId)){
            trigger_error("No valid host id given, check parameter 2.");
            return;
        }

        /* Query SI server */
        $res    = $this->send_data("gosa_opsi_add_product_to_client",$this->target,$data,TRUE);
    }


    /*! \brief      Removes a given product from a client.
      @param
      @return
     */
    public function del_product_from_client($productId,$hostId)
    {
        $data = array("productId" => $productId,"hostId" => $hostId);

        /* Check parameter */ 
        if(empty($productId)){
            trigger_error("No valid product id given, check parameter 1.");
            return;
        }
        if(empty($hostId)){
            trigger_error("No valid host id given, check parameter 2.");
            return;
        }

        /* Query SI server */
        $res    = $this->send_data("gosa_opsi_del_product_from_client",$this->target,$data,TRUE);
    }


    /*! \brief 		Returns the clients hardware setup.
      @param
      @return 		
     */
    public function get_client_hardware($hostId)
    {
        $data = array("hostId" => $hostId);

        /* Check parameter */ 
        if(empty($hostId)){
            trigger_error("No valid host id given, check parameter 1.");
            return;
        }

        /* Query SI server */
        $res    = $this->send_data("gosa_opsi_get_client_hardware",$this->target,$data,TRUE);
        if(isset($res['XML'][0]['ITEM'])){
            return($res['XML'][0]['ITEM']);
        }
        return(array());
    }


    /*! \brief 		Returns the clients software setup.
      @param
      @return 		
     */
    public function get_client_software($hostId)
    {
        $data = array("hostId" => $hostId);

        /* Check parameter */ 
        if(empty($hostId)){
            trigger_error("No valid host id given, check parameter 1.");
            return;
        }

        /* Query SI server */
        $res    = $this->send_data("gosa_opsi_get_client_software",$this->target,$data,TRUE);
        if(isset($res['XML'][0]['ITEM'])){
            return($res['XML'][0]['ITEM']);
        }
        return(array());
    }



    /*! \brief 		Deletes the given opsi client.
      @param
      @return 		
     */
    public function del_client($hostId)
    {
        $data = array("hostId" => $hostId);

        /* Check parameter */ 
        if(empty($hostId)){
            trigger_error("No valid host id given, check parameter 1.");
            return;
        }

        /* Query SI server */
        $res    = $this->send_data("gosa_opsi_del_client",$this->target,$data,TRUE);
        if(isset($res['XML'][0]['ITEM'])){
            return($res['XML'][0]['ITEM']);
        }
        return(array());
    }


    /*! \brief 		Triggers install/reinstall of an opsi client.
      @param
      @return 		
     */
    public function job_opsi_install_client($hostId,$mac)
    {
        $data = array("hostId" => $hostId,"macaddress"=>$mac);

        /* Check parameter */ 
        if(empty($hostId)){
            trigger_error("No valid host id given, check parameter 1.");
            return;
        }

        /* Query SI server */
        $this->send_data("job_opsi_install_client",$this->target,$data,TRUE);
    }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
