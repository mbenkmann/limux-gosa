<?php 
class migrate_faiHookRDN extends migrateFaiRDN implements propertyMigration
{
	protected $filter = "(&(objectClass=FAIhook)%s)";
}
?>
