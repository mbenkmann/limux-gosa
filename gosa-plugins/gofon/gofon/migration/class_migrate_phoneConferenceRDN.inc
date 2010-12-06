<?php 
class migrate_phoneConferenceRDN extends migrateRDN implements propertyMigration
{
    protected $filter = "(&(objectClass=goFonConference)%s)";
}
?>
