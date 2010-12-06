<?php



class migrateFaiRDN extends migrateRDN implements propertyMigration
{
    protected $property = NULL;
    protected $config = NULL;
    protected $found = array();
    protected $filter ="";

    // Additional suffixes or prefixes 
    // e.g. for 'faiScriptRDN' (ou=scripts,) moving to new destination lets say 
    //  to 'ou=FAIscripts,' would break stuff with having 'ou=fai,ou=systems,ou=config' 
    // prepended.
    // 
    protected $suffix = ""; 
    protected $prefix = ""; 

    public $faiBaseRDN;

    function __construct($config,$property)
    {
        parent::__construct($config,$property);
    }   

    function checkForIssues()
    {
        // This is very tricky ... have to think about it. Its deactivated currently.
        $this->found = array();
        $ldap= $this->config->get_ldap_link();
        $ldap->cd($this->config->current['BASE']);
        $ldap2= $this->config->get_ldap_link();
        $ldap2->cd($this->config->current['BASE']);

        // If the userRDN wasn't empty, then only search for users inside of the old userRDN.
        $initialValue = $this->prefix.$this->property->getValue().$this->suffix;
        $targetValue = $this->prefix.$this->property->getValue(TRUE).$this->suffix;

        if(!empty($initialValue) && !preg_match("/,$/", $initialValue)) $initialValue.=",";
        if(!empty($targetValue) && !preg_match("/,$/", $targetValue)) $targetValue.=",";

        $dnMatch = "";
        if(!empty($initialValue)){
            foreach(preg_split("/,/", $initialValue) as $rdnPart){
                if(empty($rdnPart)) continue;
                list($namingAttrs, $container) = preg_split("/=/",$rdnPart,2);
                $container = trim($container,', ');
                $dnMatch.= "({$namingAttrs}:dn:={$container})";
            }
        }

        // Collect all FAI releases - this is nescessary to detect the release part
        //  of the dn
        $ldap->cd($this->config->current['BASE']);
        $releases = array();
        $ldap->search("(objectClass=FAIbranch)",array('dn'));
        while($attrs = $ldap->fetch()){
            $releases[$attrs['dn']] = $attrs['dn'];
        }

        // Add release bases 
        $faiBase = $this->config->get_cfg_value('faiManagement','faiBaseRDN').$this->config->current['BASE'];
        $releases[$faiBase] = $faiBase;
        
        // Search for users
        $filter = sprintf($this->filter,$dnMatch);
        $ldap->search($filter,array('dn'));
        $found = FALSE;
        while($attrs = $ldap->fetch()){

            // Detect release dn 
            $releaseDn  = $attrs['dn'];
            while(!isset($releases[$releaseDn]) && !isset($this->config->idepartments[$releaseDn])){
                $releaseDn = preg_replace("/^[^,]+,/","",$releaseDn);
            }
            
            // Build up the new dn
            $name = preg_replace("/^([^,]*+,).*$/","\\1",$attrs['dn']);
            $newDn = $name.$targetValue.$releaseDn;

            // Check if we've to create new sub departments 
            if(!$ldap2->dn_exists($targetValue.$releaseDn)){
                $this->found['add'][$targetValue.$releaseDn] = array();
            }

            // Queue object to be moved.
            if($newDn != $attrs['dn']){
                $this->found['move'][] = array('from' => $attrs['dn'], 'to' => $newDn);
                $found = TRUE;
            }
        }
        return($found);
    }
} 


?>
