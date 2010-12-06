<?php

class tabsVariable extends tabs
{
  var $base= "";
  var $parent ="";

  function tabsVariable($config, $data, $dn,$category)
  {
	tabs::tabs($config, $data, $dn, $category);

	/* Add references/acls/snapshots */
#	$this->addSpecialTabs();
  }

  function check($ignore_account= FALSE)
  {
	return (tabs::check(TRUE));
  }
  

  function save($ignore_account= FALSE)
  {
  	$baseobject= $this->by_object['faiVariable'];
    $cn      = preg_replace('/,/', '\,', $baseobject->cn); 
    $cn      = preg_replace('/"/', '\"', $cn); 
	$new_dn ='cn='.$cn.",".get_ou("faiVariable", "faiVariableRDN").$this->parent->fai_release;
			
 	if ($this->dn != $new_dn && $this->dn != "new"){
		
		/* if( new_dn is subtree of this->dn ) */
		$cnt1 = count(explode(",",$this->dn));
		$cnt2 = count(explode(",",$new_dn));
		if((strstr($new_dn,$this->dn))&&($cnt1<$cnt2)){
			msg_dialog::display(_("Error"), _("Moving the tree failed. Destination tree is subtree of source tree."), ERROR_DIALOG);
		}else{
//			$baseobject->recursive_move($this->dn, $new_dn);
		}
	}
	$this->dn= $new_dn;

	tabs::save(TRUE);
  }

}

?>
