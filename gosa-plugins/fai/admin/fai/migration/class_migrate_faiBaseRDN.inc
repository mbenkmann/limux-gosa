<?php 
class migrate_faiBaseRDN extends migrateFaiRDN implements propertyMigration
{
    protected $filter = "(&(objectClass=nothing)%s)";

    function checkForIssues()
    {
        $this->found = array();
        $ldap= $this->config->get_ldap_link();
        $ldap->cd($this->config->current['BASE']);

        $initialValue = $this->prefix.$this->property->getValue().$this->suffix;
        $targetValue = $this->prefix.$this->property->getValue(TRUE).$this->suffix;

        if(!empty($initialValue) && !preg_match("/,$/", $initialValue)) $initialValue.=",";
        if(!empty($targetValue) && !preg_match("/,$/", $targetValue)) $targetValue.=",";

        $from = $initialValue.$this->config->current['BASE'];
        $to = $targetValue.$this->config->current['BASE'];

        if($initialValue != $targetValue && $ldap->dn_exists($from)){
            $toParent = preg_replace("/^[^,]*+,/","",$to);
            if(!$ldap->dn_exists($toParent)){
                $this->found['add'][$toParent] = array();
            }
            $this->found['move'][] = array('from'=>$from, 'to'=>$to);
            return(TRUE);
        }
        return(FALSE);
    }

    function _execute()
    {
        $str = "<h3>"._("The storage position for FAI object has changed!")."</h3>";
       # $str.= _("Unfortunately, GOsa can not perform the necessary steps automatically to migrate the FAI objects. You have to do this manually, to keep the System deployment in a usable state!");
        $str .= migrateFaiRDN::execute();
        return($str);
    }

}
?>
