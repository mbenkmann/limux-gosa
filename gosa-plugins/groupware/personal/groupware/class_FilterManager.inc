<?php

class filterManager extends plugin{

    var $filterListing   = NULL;
    var $filterEditor = NULL;

    function __construct($config,$parent,$rules)
    {
        plugin::plugin($config);
        
        $this->parent = &$parent;

        $this->filter = $rules;

        // If we've read permissions then allow to edit the entries
        $acl = $this->parent->getacl('mailFilter');
        $acl = preg_replace("/r/","rw", $acl);
    
        $this->filterListing= new sortableListing();
        $this->filterListing->setDeleteable(TRUE);
        $this->filterListing->setEditable(TRUE);
        $this->filterListing->setColspecs(array('*'));
        $this->filterListing->setWidth("100%");
        $this->filterListing->setHeight("150px;");
        $this->filterListing->setAcl($acl);
        $this->filterListing->setColspecs(array('30px','200px','*'));
        $this->filterListing->setHeader(array('-',_("Name"),_("Description")));
        $this->filterListing->setDefaultSortColumn(1);
    }

    function execute()
    {
        // If we've read permissions then allow to edit the entries
        $acl = $this->parent->getacl('mailFilter');
        $acl = preg_replace("/r/","rw", $acl);
        $this->filterListing->setAcl($acl);
        // Display filter editor while a filter rule is edited
        if($this->filterEditor instanceOf FilterEditor){
            $this->filterEditor->save_object();
            return($this->filterEditor->execute());
        }
        

        $smarty = get_smarty();
        $data = $lData = array();
        foreach($this->filter as $key => $filter){
            $data[$key] = $filter;

            switch($filter['STATUS']){
                case 'NEW' : $img = image('images/lists/edit.png[new]');break;
                case 'MODIFIED' : $img = image('images/lists/edit.png');break;
                case 'EXISTS' : $img = image('images/lists/edit.png');break;
                default : $img = "";
            }

            $lData[$key] = array('data' => array($img,$filter['NAME'], $filter['DESC']));
        }
        $this->filterListing->setListData($data,$lData);
        $this->filterListing->update();


        $smarty->assign('acl', $this->parent->getacl('mailFilter'));
        $smarty->assign('list', $this->filterListing->render());

        return($smarty->fetch(get_template_path('filterManager.tpl',TRUE,dirname(__FILE__))));
    }

    function save_object()
    {
        $this->filterListing->save_object();
        $action = $this->filterListing->getAction();

        // Remove filter was requested.
        if($action['action'] == 'delete'){
            $key = $action['targets'][0];
            $key = $this->filterListing->getKey($key);
            if(isset($this->filter[$key])){
                unset($this->filter[$key]);
                $this->filter = array_values($this->filter);
            }
        }

        // Edit filter was requested.
        if($action['action'] == 'edit'){
            $key = $action['targets'][0];
            $key = $this->filterListing->getKey($key);
            if(isset($this->filter[$key])){
                $filter = $this->filter[$key];
                $this->filterEditor = new FilterEditor($this->config,$this->parent, $filter);
                $this->currentFilter = $key;
            }
        }

        // Add new filter
        if(isset($_POST['addFilter'])){
            $filter =   array (
                    'STATUS' => 'NEW',
                    'TYPE' => 'AND',
                    'NAME' => _('name'),
                    'DESC' => _('description'),
                    'CONDITIONS' => array  (
                        array('FIELD' => 'from',
                            'COMPARATOR' => 'equals',
                            'MATCH' => 'example@domain.com'),
                        array('FIELD' => 'subject',
                            'COMPARATOR' => 'contains',
                            'MATCH' => _('advertising')),
                        ),
                    'ACTION' => array (
                        array('ACTION'=>'MARK',
                            'VALUE' => 'SPAM'),
                        array('ACTION'=>'MOVE',
                            'VALUE' => '')
                        )
                    );
            $this->filterEditor = new FilterEditor($this->config,$this->parent, $filter);
        }

        // Close filter editor 
        if(isset($_POST['filterEditor_cancel']) && $this->filterEditor instanceOf FilterEditor){
            $this->currentFilter = NULL;
            $this->filterEditor = NULL;
        }

        // Save filter modifications and close the dialog
        if(isset($_POST['filterEditor_ok']) && $this->filterEditor instanceOf FilterEditor){
            $this->filterEditor->save_object();
            $msgs = $this->filterEditor->check();
            if(count($msgs)){
                msg_dialog::displayChecks($msgs);
            }else{
                $filter = $this->filterEditor->save();
                if($filter['STATUS'] == 'NEW'){
                    $this->filter[] = $filter;
                }else{
                    if($filter['STATUS'] != 'NEW') $filter['STATUS'] = 'MODIFIED';
                    $this->filter[$this->currentFilter] = $filter;
                }
                $this->filterEditor = NULL;
                $this->currentFilter = NULL;
            }
        }
    }


    function save()
    {
        return($this->filter);
    }
}

?>
