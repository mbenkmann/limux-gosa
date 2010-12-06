<?php

class TemplateWidget_textEditor extends TemplateWidget
{
    protected $value = array();
    protected $write_protect = FALSE;
    
    function __construct(&$config, $name, $value, $description,$syntax,$required,$type,$display,$values)
    {
        parent:: __construct($config, $name, $value, $description,$syntax,$required,$type,$display,$values);

        // Keep an eye on dangerous encodings, we may break scripts while editing.
        $this->mb_extension = function_exists("mb_detect_encoding");
        if($this->mb_extension){
            $this->enc_before_edit = mb_detect_encoding($this->value);
            if($this->enc_before_edit != "ASCII"){
                $this->write_protect = TRUE;
            }
        }
    }


    function render()
    {
        $smarty = get_smarty();
        $smarty->assign("postName", set_post($this->postName));
        $smarty->assign("write_protect", set_post($this->write_protect));
        $smarty->assign("value", set_post($this->value));
        return($smarty->fetch(get_template_path("TemplateWidget_textEditor.tpl", TRUE, dirname(__FILE__))));
    }


    function save_object()
    {
        TemplateWidget::save_object();
        if(isset($_POST['editAnyway'])) $this->write_protect = FALSE;

        if(isset($_POST['ImportUpload'])){
            if(($_FILES['ImportFile']['error']!=0)){
                msg_dialog::display(_("Error"), msgPool::incorrectUpload(), ERROR_DIALOG);
            }elseif(($_FILES['ImportFile']['size']==0)){
                msg_dialog::display(_("Error"), msgPool::incorrectUpload(_("file is empty")), ERROR_DIALOG);
            }else{
                $str = file_get_contents(gosa_file_name($_FILES['ImportFile']['tmp_name']));
                $this->value = $str;

                // Check encoding again
                if($this->mb_extension){
                    $this->enc_before_edit = mb_detect_encoding($this->value);
                    if($this->enc_before_edit != "ASCII"){
                        $this->write_protect = TRUE;
                    }
                }
            }
        }
        $this->enc_after_edit = mb_detect_encoding($this->value);
    }


    /*! \brief  Check the value entry using the provieded syntax.
     * @return  Array   Returns a list of errors
     */
    function check()
    {
        $msgs = TemplateWidget::check();
        if($this->mb_extension && !$this->write_protect && $this->enc_after_edit !== $this->enc_before_edit ){
            $msg = sprintf(_("The text encoding has changed from %s to %s. Do you really want to save?"),
                    bold($this->enc_before_edit),bold($this->enc_after_edit));
            $msgs[] = $msg;
            $this->enc_before_edit = $this->enc_after_edit;
        }
        return($msgs);
    }
}
?>
