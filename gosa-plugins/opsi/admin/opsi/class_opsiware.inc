<?php

class opsiSoftware extends opsiWare
{
    protected $type = 0;
}

class opsiHardware extends opsiWare
{
    protected $type = 1;
}


class opsiWare extends plugin
{
    private $opsi;
    private $hostId;
    private $init_failed = FALSE;
    private $info;
    protected $type = 0;

    public function __construct($config,$hostId)
    {
        $this->initTime = microtime(TRUE);
        $this->config = $config;
        $this->opsi   = new opsi($config);
        $this->hostId = preg_replace("/^opsi:=([^,]*),.*$/","\\1",$hostId);
        $this->init();

        // Create statistic table entry
        stats::log('plugin', $class = get_class($this), $category = array($this->acl_category),  $action = 'open',
                $amount = 1, $duration = (microtime(TRUE) - $this->initTime));
    }

    private function init()
    {
        if($this->type == 0){
            $this->info = $this->opsi->get_client_hardware($this->hostId);
            $err = $this->opsi->is_error(); 
        }else{
            $this->info = $this->opsi->get_client_software($this->hostId);
            $err = $this->opsi->is_error(); 
        }
        $this->init_failed = $err;
    }


    public function execute()
    {
        plugin::execute();
        if(isset($_POST['reinit']) && $this->init_failed){
            $this->init();
        }
        $smarty = get_smarty();
        $smarty->assign("init_failed",$this->init_failed);
        if($this->init_failed){
            $smarty->assign("type", $this->type);
            $smarty->assign("message",$this->opsi->get_error());
            return($smarty->fetch(get_template_path("opsiware.tpl",TRUE,dirname(__FILE__))));
        }  
        $smarty->assign("type", $this->type);
        $smarty->assign("info", $this->info);
        return($smarty->fetch(get_template_path("opsiware.tpl",TRUE,dirname(__FILE__))));
    }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
