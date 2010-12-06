<?php

class TemplateWidget_combobox extends TemplateWidget
{
    function render()
    {
        $str = "";

        // Build up list data
        $str .= "<select size='1' name='{$this->postName}'>";
        foreach($this->values as $name => $value){
            if($name == $this->value){
                $str .= "<option selected value=\"{$name}\">{$value}</option>\n";
            }else{
                $str .= "<option value=\"{$name}\">{$value}</option>\n";
            }
        }
        $str .= "</select>";
        return($str);
    }
}
?>
