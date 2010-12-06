<?php

class TemplateWidget_string extends TemplateWidget
{
    function render()
    {
        $desc = set_post($this->description);
        $value = set_post($this->value);

        $name = " name=\"{$this->postName}\" ";
        $value = " value=\"{$value}\" ";
        $title = (empty($this->description))?"": " title=\"{$desc}\"";

        return("<input type='text' {$title} {$name} {$value }>");
    }
}

?>
