<?php

/* Remove classes and variables from session
 */
$lock_msg ="";
if ( $cleanup ){
  session::un_set('fonreport');
}else{
    /* Reset requested? */
    if (isset($_GET['reset']) && $_GET['reset'] == 1){
        session::un_set ('fonreport');
    }

	/* Create fonreport object on demand */
	if (!session::is_set('fonreport')){
		session::set('fonreport',new fonreport ($config, $ui));
	}
	$fonreport = session::get('fonreport');

	/* Execute formular */
	$fonreport->save_object();
	$display= $fonreport->execute ();
	$display.= "<input type=\"hidden\" name=\"ignore\">\n";

	/* Store changes  in session */
	session::set('fonreport',$fonreport);
}
?>
