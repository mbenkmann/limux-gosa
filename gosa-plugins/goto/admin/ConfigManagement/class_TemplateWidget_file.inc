<?php

class TemplateWidget_file extends TemplateWidget
{
    function render()
    {
        $title = set_post($this->description);
        if(mb_strlen($this->value) == 0){   
            $ret = "<i>"._("No file uploaded yet")."</i>";
            $ret.= "<br> <input type=\"hidden\" name=\"MAX_FILE_SIZE\" value=\"2000000\">
                <input id=\"{$this->postName}\" 
                name=\"{$this->postName}\" 
                type=\"file\" 
                size=\"20\" 
                maxlength=\"255\" 
                accept=\"*.*\">&nbsp;";
            $ret.= "<button name='{$this->postName}_Upload'>"._("Upload")."</button>";
        }else{
            $ret = "<i>"._("File uploaded").": ".mb_strlen($this->value)." "._("Bytes");
            $ret.= "&nbsp;<button name='{$this->postName}_Remove'>".msgPool::delButton()."</button>";
        }
        return($ret);
    }

    function save_object()
    {
        if(isset($_POST["{$this->postName}_Upload"]) && isset($_FILES[$this->postName]['tmp_name'])){
            $this->value = file_get_contents(gosa_file_name($_FILES[$this->postName]['tmp_name']));
        }
        if(isset($_POST["{$this->postName}_Remove"])) $this->value ="";
    }
}

?>
