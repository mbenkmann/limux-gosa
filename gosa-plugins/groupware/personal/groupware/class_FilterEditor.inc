<?php

class FilterEditor extends plugin
{
    function __construct($config, $parent, $filter)
    {
        $this->config = &$config;
        $this->parent = &$parent;
        $this->filter = $filter;

        $this->types = array(
                'OR' => _("At least one of the following conditions has to match"), 
                'AND' => _("All conditions have to match"));

        $this->actions = array(
                "MOVE" => _("move mail to"),
                "COPY" => _("copy mail to"),
                "FORWARD" => _("forward message to"),
                "MARK" => _("mark mail as "),
                "DROP" => _("remove mail"),
                "REPLY" => _("reply"));

        $this->fields = array(
                'subject' => _("Subject"),
                'from'=> _("From"),
                'body'=> _("Body"),
                'date'=> _("Date"),
                'priority'=> _("Priority"),
                'status'=> _("Status"),
                'to'=> _("To"),
                'cc'=> _("CC"),
                'age'=> _("Age"),
                'size'=> _("Size"));

        $this->comparators = array(
                "is" => _("is"),
                "is not" => _("is not"), 
                "equal" => _("is equal"),       
                "not equal" => _("is not equal"),   
                "is empty" => _("is empty"),
                "is not empty" => _("is not empty"),
                "contains" => _("contains"),
                "contains not" => _("does not contain"),
                "is in addressbook" => _("is in the addressbook"),
                "is not in addressbook" => _("is not in the addressbook"),
                "greater than" => _("is greater than"),
                "less than" => _("smaller than"));
    }

    function execute()
    {
        $smarty = get_smarty();
        $smarty->assign('NAME',set_post($this->filter['NAME']));    
        $smarty->assign('DESC',set_post($this->filter['DESC']));    
        $smarty->assign('filterStr', $this->renderFilter());    
        $smarty->assign('acl', $this->parent->getacl('mailFilter'));    

        return($smarty->fetch(get_template_path('filterEditor.tpl',TRUE,dirname(__FILE__))));
    }



    function check()
    {
        $msgs = array();
        return($msgs);
    }


    function save()
    {
        // Just return the filter array we've modified
        return($this->filter);
    }


    function renderFilter()
    {
        $filter = $this->filter;

        $cnt = count($filter['CONDITIONS']);
        $str = "<h3>"._("Filter conditions")."</h3>";
        $str .= _("Condition type")."&nbsp;";
        $str .="\n<select size='1' name='cType' onChange='document.mainform.submit();'>";
        foreach($this->types as $type => $desc){
            $checked = ($type == $filter['TYPE'])? ' selected ' : '';
            $str.= "\n<option {$checked} value=\"".set_post($type)."\">".set_post($desc)."</option>";
        }
        $str .= "\n</select>";
        $str.= "<ul>";
        foreach($filter['CONDITIONS'] as $id => $cond){
            $str .= "<li>";
#            $str .= _("Check field")."&nbsp;";
            $str .= "\n<select size='1' style='width:100px;' name='cField_{$id}' onChange='document.mainform.submit();'>";
            foreach($this->fields as $field => $desc){
                $checked = ($field == $cond['FIELD'])? ' selected ' : '';
                $str.= "\n<option {$checked} value=\"".set_post($field)."\">".set_post($desc)."</option>";
            }
            $str .= "\n</select>";
#            $str .= "&nbsp;"._("if it")."&nbsp;";
            $str .= "\n<select size='1' style='width:200px;' name='cComparator_{$id}' onChange='document.mainform.submit();'>";
            foreach($this->comparators as $comparator => $desc){
                $checked = ($comparator == $cond['COMPARATOR'])? ' selected ' : '';
                $str.= "\n<option {$checked} value=\"".set_post($comparator)."\">".set_post($desc)."</option>";
            }
            $str .= "\n</select>&nbsp;";
            if(!in_array($cond['COMPARATOR'], array('is in addressbook','is not in addressbook','is empty','is not empty'))){
                $str .= "<input style='width:400px;' type='text' name='cMatch_{$id}' value=\"".set_post($cond['MATCH'])."\">";
            }
            if(count($filter['CONDITIONS']) >= 2){
                $str .= "<button name='removeCondition_{$id}'>".msgPool::delButton()."</button> ";
            }
            $cnt --;
            $str .= "</li>";
#            if($cnt) $str .= $this->types[$filter['TYPE']];
        } 
        $str .= "</ul>";
        $str .= "<button style='margin-left:40px;' name='addCondition'>".msgPool::addButton()."</button> ";
       
        $str .= "<hr>";
        $str .= "<h3>"._("Filter actions")."</h3>";

        $str.= "<ul>";
        foreach($filter['ACTION'] as $id => $action){
            $str .= "<li>";
            $str .= "\n<select style='width:200px;' size='1' name='cAction_{$id}' onChange='document.mainform.submit();'>";
            foreach($this->actions as $act => $desc){
                $checked = ($act == $action['ACTION'])? ' selected ' : '';
                $str.= "\n<option {$checked} value=\"".set_post($act)."\">".set_post($desc)."</option>";
            }
            $str .= "</select>&nbsp;";
            if(!in_array($action['ACTION'], array('DROP'))){
                $str .= "<input style='width:500px;' type='text' name='cValue_{$id}' value=\"".set_post($action['VALUE'])."\">";
            }

            if(count($filter['ACTION']) >= 2){
                $str .= "<button name='removeAction_{$id}'>".msgPool::delButton()."</button>";
            }
            $str .= "</li>";
        }
        $str.= "</ul>";
        $str .= "<button style='margin-left:40px;' name='addAction'>".msgPool::addButton()."</button> ";

        return($str);
    }


    function save_object()
    {
        // Do nothing if the dialog wasn't submitted yet
        if(!isset($_POST['filterEditorPosted'])) return;

        // Get the filter confition type if posted
        $this->filter['TYPE'] = (isset($_POST['cType']))? get_post('cType'): $this->filter['TYPE'];

        // Get condition modifications
        foreach($this->filter['CONDITIONS'] as $id => $value){
            foreach(array('cField_' => 'FIELD',
                        'cComparator_'=>'COMPARATOR',
                        'cMatch_'=>'MATCH') as $post => $name){
                if(isset($_POST[$post.$id])){
                    $this->filter['CONDITIONS'][$id][$name] = get_post($post.$id);
                }
            }
            if(isset($_POST['removeCondition_'.$id]) && count($this->filter['CONDITIONS']) >= 2){
                unset($this->filter['CONDITIONS'][$id]);
            }
        }

        // Get Action modifications 
        foreach($this->filter['ACTION'] as $id => $value){
            foreach(array('cAction_' => 'ACTIOB','cValue_' => 'VALUE') as $post => $name){
                if(isset($_POST[$post.$id])){
                    $this->filter['ACTION'][$id][$name] = get_post($post.$id);
                }
            }
            if(isset($_POST['removeAction_'.$id]) && count($this->filter['ACTION']) >= 2){
                unset($this->filter['ACTION'][$id]);
            }
        }

        // Get NAME and DESC if posted
        if(isset($_POST['NAME'])) $this->filter['NAME'] = get_post('NAME');
        if(isset($_POST['DESC'])) $this->filter['DESC'] = get_post('DESC');

        // Add conditions
        if(isset($_POST['addCondition'])){
            $this->filter['CONDITIONS'][] = array('FIELD' => key($this->fields),'COMPARATOR' => key($this->comparators), 'MATCH' => '');
        }
    
        // Add actions
        if(isset($_POST['addAction'])){
            $this->filter['ACTION'][] = array('ACTION' => 'MOVE', 'VALUE' => '');
        }
    }
}
?>
