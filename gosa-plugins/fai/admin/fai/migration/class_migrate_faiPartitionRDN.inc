<?php 
class migrate_faiPartitionRDN extends migrateFaiRDN implements propertyMigration
{
	protected $filter = "(&(objectClass=FAIpartitionTable)%s)";
}
?>
