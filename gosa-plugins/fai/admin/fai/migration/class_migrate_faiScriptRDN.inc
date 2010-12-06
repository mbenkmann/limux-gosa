<?php 
class migrate_faiScriptRDN extends migrateFaiRDN implements propertyMigration
{
	protected $filter = "(&(objectClass=FAIscript)%s)";
}
?>
