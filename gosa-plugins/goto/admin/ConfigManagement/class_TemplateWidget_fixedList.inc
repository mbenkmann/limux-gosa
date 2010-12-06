<?php

class TemplateWidget_fixedList extends TemplateWidget
{
    protected $value = array();
    
    function __construct(&$config, $name, $value, $description,$syntax,$required,$type,$display,$values)
    {
        parent:: __construct($config, $name, $value, $description,$syntax,$required,$type,$display,$values);

        $this->listWidget= new sortableListing($this->value);
        $this->listWidget->setEditable(false);
        $this->listWidget->setDeleteable(true);
        $this->listWidget->setColspecs(array('*'));
        $this->listWidget->setWidth("100%");
        $this->listWidget->setHeight("70px");
        $this->listWidget->setAcl("rwcdm");
    }

    function getAvailableOptions()
    {
        $tmp = array();
        foreach($this->values as $key => $name){
            if(!in_array($key, $this->value)){
                $tmp[$key]=$name;
            }
        }
        return($tmp);
    }

    function render()
    {
        $str = "";

        // Build up list data
        $data = $this->value;
        foreach($this->value as $key => $name){
            $lData[$key] = array('data' => array($this->values[$name]));
        }
        $this->listWidget->setListData($data, $lData);
        $this->listWidget->update();
        $str .= $this->listWidget->render();
        $str .= "<select size='1' name='{$this->postName}_Input'>";
        foreach($this->getAvailableOptions() as $name => $value){
            $str .= "<option value=\"{$name}\">{$value}</option>\n";
        }
        $str .= "</select>";
        $str .= "<button name='{$this->postName}_Add'>".msgPool::addButton()."</button>";
        return($str);
    }

    function save_object()
    {
        $this->listWidget->save_object();
        $action = $this->listWidget->getAction();
        if($action['action'] == 'delete'){
            $id = $this->listWidget->getKey($action['targets'][0]);
            unset($this->value[$id]);
            $this->value = array_values($this->value);
        }

        if(isset($_POST["{$this->postName}_Add"]) && isset($_POST["{$this->postName}_Input"])){
            $input = get_post("{$this->postName}_Input");

            if(!empty($input) && !empty($this->syntax) && !preg_match("/".$this->syntax."/", $input)){
                msg_dialog::displayChecks(array(msgPool::invalid($this->display, $input, "/".$this->syntax."/")));
            }elseif(!empty($input)){
                $this->value[] = $input;
            }
        }
    }

    /*! \brief  Check the value entry using the provieded syntax.
     * @return  Array   Returns a list of errors
     */
    function check()
    {
        if($this->required && empty($this->value)){
            return(array(msgPool::required($this->display)));
        }
        return(array());
    }
}


?>
