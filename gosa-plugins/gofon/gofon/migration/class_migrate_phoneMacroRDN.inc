<?php 
class migrate_phoneMacroRDN extends migrateRDN implements propertyMigration
{
    protected $filter = "(&(objectClass=goFonMacro)%s)";
}
?>
