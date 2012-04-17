<?php

class MemberInheritanceDialog
{
    
    protected $members = array();  
    
    function __construct($config, $members)
    {
        $this->config  = $config;
        $this->members = $members;
        
        $this->memberList = new sortableListing();
        $this->memberList->setDeleteable(false);
        $this->memberList->setEditable(false);
        $this->memberList->setWidth("100%");
        $this->memberList->setHeight("300px");
        $this->memberList->setColspecs(array('20px', '*'));
        $this->memberList->setHeader(array("<input type='checkbox' id='member-checkbox-all' checked>",                 
                                     _("System")));
        $this->memberList->setDefaultSortColumn(1);
        $this->memberList->setAcl('r'); // All ACLs, we filter on our own here.
        $this->memberList->setReorderable(TRUE);
        
    }
    
    private function makeDisplayEntry($dn)
    {
        $ldap = $this->config->get_ldap_link();
        $ldap->cat($dn);
        $entry = $ldap->fetch();
        $cn = $entry['cn'][0];
        
        $checkbox_html = "<input class='center member-checkbox' type='checkbox' checked".
                         " name='members[]' value='" . set_post($dn). "'>";
        
        return array('data' => array($checkbox_html, $cn));
    }
    
    public function execute()
    {
        $displayData = array_map(array($this, 'makeDisplayEntry'), $this->members);
                
        $this->memberList->setListData($this->members, $displayData);
        $this->memberList->update();
        
        $smarty = get_smarty();
        $smarty->assign('list', $this->memberList->render());
        return $smarty->fetch(get_template_path('MemberInheritance-list.tpl', TRUE, dirname(__FILE__)));
    }
        
    public function getEnabledMembers()
    {
        if (isset($_POST['members'])) {
            return get_post('members');
        } else {
            return array();
        }
    }
    
}
