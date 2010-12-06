<?php

class TemplateWidget_checkbox extends TemplateWidget
{
    function render()
    {
        $str = "";
        $str .= "<input type='checkbox' value='1' name='{$this->postName}' ";
            if($this->value) $str .= " checked ";
        $str .= ">";
        return($str);
    }

    function save_object()
    {
        $this->value = isset($_POST[$this->postName]);
    }
}
?>
