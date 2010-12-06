<?php 
class migrate_faiVariableRDN extends migrateFaiRDN implements propertyMigration
{
	protected $filter = "(&(objectClass=FAIvariable)%s)";
}
?>
