<?php 
class migrate_faiTemplateRDN extends migrateFaiRDN implements propertyMigration
{
	protected $filter = "(&(objectClass=FAItemplate)%s)";
}
?>
