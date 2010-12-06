<?php

class AddItemDialog
{
    private $itemCfg = NULL;
    private $config = NULL;

    private $itemName = "";
    private $itemType = NULL;

    function __construct(&$config, $itemType, $itemCfg)
    {
        $this->config = $config;
        $this->itemCfg = $itemCfg;
        $this->itemType = $itemType;
    }

    function execute()
    {
        $smarty = get_smarty();
        $smarty->assign('itemCfg', set_post($this->itemCfg));
        $smarty->assign('itemName', set_post($this->itemName));
        return($smarty->fetch(get_template_path('goto/Config/AddItemDialog.tpl', TRUE)));
    }

    function getItemCfg()
    {
        return($this->itemCfg);
    }

    function getName()
    {
        return($this->itemName);
    }

    function getItemType()
    {
        return($this->itemType);
    }

    function save_object()
    {
        if(isset($_POST['itemName'])){
            $this->itemName = get_post('itemName');
        }
    }

    function check()
    {
        $message = array();
        if(empty($this->itemName)){
            $message[] = msgPool::required(_("Name"));
        }else
        if(!tests::is_uid($this->itemName)){
            $message[] = msgPool::invalid(_("Name"));
        }
        return($message);
    }
}

?>
