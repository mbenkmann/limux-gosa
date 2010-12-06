<?php 
class migrate_phoneRDN extends migrateRDN implements propertyMigration
{
	protected $filter = "(&(objectClass=goFonHardware)%s)";
}
?>
