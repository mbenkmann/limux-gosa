<?php 
class migrate_faiProfileRDN extends migrateFaiRDN implements propertyMigration
{
	protected $filter = "(&(objectClass=FAIprofile)%s)";
}
?>
