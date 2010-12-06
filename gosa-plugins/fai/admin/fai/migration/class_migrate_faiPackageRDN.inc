<?php 
class migrate_faiPackageRDN extends migrateFaiRDN implements propertyMigration
{
	protected $filter = "(&(objectClass=FAIpackageList)%s)";
}
?>
