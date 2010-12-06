## @file
# @details A GOsa-SI-server event module containing all functions for message handling.
# @brief Implementation of an event module for GOsa-SI-server. 


package opsi_com;

use strict;
use warnings;

use Data::Dumper;
use XML::Quote qw(:all);
use GOsaSI::GosaSupportDaemon;

use Exporter;
use UNIVERSAL 'isa';

our @ISA = qw(Exporter);

my @events = (
    "get_events",
    "opsi_install_client",
    "opsi_get_netboot_products",  
    "opsi_get_local_products",
    "opsi_get_client_hardware",
    "opsi_get_client_software",
    "opsi_get_product_properties",
    "opsi_get_full_product_host_information",
    "opsi_set_product_properties",
    "opsi_list_clients",
    "opsi_del_client",
    "opsi_add_client",
    "opsi_modify_client",
    "opsi_add_product_to_client",
    "opsi_del_product_from_client",
    "opsi_createLicensePool",
    "opsi_deleteLicensePool",
    "opsi_createLicense",
    "opsi_assignSoftwareLicenseToHost",
    "opsi_unassignSoftwareLicenseFromHost",
    "opsi_unassignAllSoftwareLicensesFromHost",
    "opsi_getSoftwareLicense_hash",
    "opsi_getLicensePool_hash",
    "opsi_getSoftwareLicenseUsages",
    "opsi_getSoftwareLicenseUsagesForProductId",
    "opsi_getLicensePools_listOfHashes",
    "opsi_getLicenseInformationForProduct",
    "opsi_getPool",
    "opsi_getAllSoftwareLicenses",
    "opsi_removeLicense",
    "opsi_getReservedLicenses",
    "opsi_boundHostToLicense",
    "opsi_unboundHostFromLicense",
    "opsi_test",
   );

our @EXPORT = @events;


BEGIN {}

END {}

# ----------------------------------------------------------------------------
#                          D E C L A R A T I O N S
# ----------------------------------------------------------------------------

my $licenseTyp_hash = { 'OEM'=>'', 'VOLUME'=>'', 'RETAIL'=>''};
my ($opsi_enabled, $opsi_server, $opsi_admin, $opsi_password, $opsi_url, $opsi_client);
my %cfg_defaults = (
		"Opsi" => {
		"enabled"  => [\$opsi_enabled, "false"],
		"server"   => [\$opsi_server, "localhost"],
		"admin"    => [\$opsi_admin, "opsi-admin"],
		"password" => [\$opsi_password, "secret"],
		},
);

&read_configfile($main::cfg_file, %cfg_defaults);

if ($opsi_enabled eq "true") {
	use JSON::RPC::Client;
	use XML::Quote qw(:all);
	use Time::HiRes qw( time );
	$opsi_url= "https://".$opsi_admin.":".$opsi_password."@".$opsi_server.":4447/rpc";
	$opsi_client = new JSON::RPC::Client;

	# Check version dependencies
	eval { &myXmlHashToString(); };
	if ($@ ) {
		die "\nThe version of the Opsi plugin you want to use requires a newer version of GosaSupportDaemon. Please update your GOsa-SI or deactivate the Opsi plugin.\n";
	}
}

# ----------------------------------------------------------------------------
#   external methods handling the comunication with GOsa/GOsa-si
# ----------------------------------------------------------------------------

################################
# @brief A function returning a list of functions which are exported by importing the module.
# @return List of all provided functions
sub get_events {
    return \@events;
}

################################
# @brief Adds an Opsi product to an Opsi client.
# @param msg - STRING - xml message with tags hostId and productId
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_add_product_to_client {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];

    # Build return message
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # Sanity check of needed parameter
    if ((not exists $msg_hash->{'hostId'}) || (@{$msg_hash->{'hostId'}} != 1) || (@{$msg_hash->{'hostId'}}[0] eq ref 'HASH'))  {
		return &_giveErrorFeedback($msg_hash, "no hostId specified or hostId tag invalid", $session_id);
    }
    if ((not exists $msg_hash->{'productId'}) || (@{$msg_hash->{'productId'}} != 1) || (@{$msg_hash->{'productId'}}[0] eq ref 'HASH')) {
		return &_giveErrorFeedback($msg_hash, "no productId specified or productId tag invalid", $session_id);
    }

	# Get hostId
	my $hostId = @{$msg_hash->{'hostId'}}[0];
	&add_content2xml_hash($out_hash, "hostId", $hostId);

	# Get productID
	my $productId = @{$msg_hash->{'productId'}}[0];
	&add_content2xml_hash($out_hash, "productId", $productId);

	# Do an action request for all these -> "setup".
	my $callobj = {
		method  => 'setProductActionRequest',
		params  => [ $productId, $hostId, "setup" ],
		id  => 1, }; 
	my $res = $main::opsi_client->call($main::opsi_url, $callobj);

	if (&check_opsi_res($res)) { return ( (caller(0))[3]." : ".$_, 1 ); };

	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( &create_xml_string($out_hash) );
}

################################
# @brief Deletes an Opsi-product from an Opsi-client. 
# @param msg - STRING - xml message with tags hostId and productId
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_del_product_from_client {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    my ($hostId, $productId);
    my $error = 0;
    my ($sres, $sres_err, $sres_err_string);

    # Build return message
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # Sanity check of needed parameter
    if ((not exists $msg_hash->{'hostId'}) || (@{$msg_hash->{'hostId'}} != 1) || (@{$msg_hash->{'hostId'}}[0] eq ref 'HASH'))  {
        $error++;
        &add_content2xml_hash($out_hash, "error_string", "no hostId specified or hostId tag invalid");
        &add_content2xml_hash($out_hash, "error", "hostId");
        &main::daemon_log("$session_id ERROR: no hostId specified or hostId tag invalid: $msg", 1); 

    }
    if ((not exists $msg_hash->{'productId'}) || (@{$msg_hash->{'productId'}} != 1) || (@{$msg_hash->{'productId'}}[0] eq ref 'HASH')) {
        $error++;
        &add_content2xml_hash($out_hash, "error_string", "no productId specified or productId tag invalid");
        &add_content2xml_hash($out_hash, "error", "productId");
        &main::daemon_log("$session_id ERROR: no productId specified or procutId tag invalid: $msg", 1); 
    }

    # All parameter available
    if (not $error) {
        # Get hostId
        $hostId = @{$msg_hash->{'hostId'}}[0];
        &add_content2xml_hash($out_hash, "hostId", $hostId);

        # Get productID
        $productId = @{$msg_hash->{'productId'}}[0];
        &add_content2xml_hash($out_hash, "productId", $productId);

        # Check to get product action list 
        my $callobj = {
            method  => 'getPossibleProductActions_list',
            params  => [ $productId ],
            id  => 1, };
        $sres = $main::opsi_client->call($main::opsi_url, $callobj);
        ($sres_err, $sres_err_string) = &check_opsi_res($sres);
        if ($sres_err){
            &main::daemon_log("$session_id ERROR: cannot get product action list: ".$sres_err_string, 1);
            &add_content2xml_hash($out_hash, "error", $sres_err_string);
            $error++;
        }
    }

    # Check action uninstall of product
    if (not $error) {
        my $uninst_possible= 0;
        foreach my $r (@{$sres->result}) {
            if ($r eq 'uninstall') {
                $uninst_possible= 1;
            }
        }
        if (!$uninst_possible){
            &main::daemon_log("$session_id ERROR: cannot uninstall product '$productId', product do not has the action 'uninstall'", 1);
            &add_content2xml_hash($out_hash, "error", "cannot uninstall product '$productId', product do not has the action 'uninstall'");
            $error++;
        }
    }

    # Set product state to "none"
    # Do an action request for all these -> "setup".
    if (not $error) {
        my $callobj = {
            method  => 'setProductActionRequest',
            params  => [ $productId, $hostId, "none" ],
            id  => 1, 
        }; 
        $sres = $main::opsi_client->call($main::opsi_url, $callobj);
        ($sres_err, $sres_err_string) = &check_opsi_res($sres);
        if ($sres_err){
            &main::daemon_log("$session_id ERROR: cannot delete product: ".$sres_err_string, 1);
            &add_content2xml_hash($out_hash, "error", $sres_err_string);
        }
    }

    # Return message
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( &create_xml_string($out_hash) );
}

################################
# @brief Adds an Opsi client to Opsi.
# @param msg - STRING - xml message with tags hostId and macaddress
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_add_client {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    my ($hostId, $mac);
    my $error = 0;
    my ($sres, $sres_err, $sres_err_string);

    # Build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # Sanity check of needed parameter
    if ((not exists $msg_hash->{'hostId'}) || (@{$msg_hash->{'hostId'}} != 1) || (@{$msg_hash->{'hostId'}}[0] eq ref 'HASH'))  {
        $error++;
        &add_content2xml_hash($out_hash, "error_string", "no hostId specified or hostId tag invalid");
        &add_content2xml_hash($out_hash, "error", "hostId");
        &main::daemon_log("$session_id ERROR: no hostId specified or hostId tag invalid: $msg", 1); 
    }
    if ((not exists $msg_hash->{'macaddress'}) || (@{$msg_hash->{'macaddress'}} != 1) || (@{$msg_hash->{'macaddress'}}[0] eq ref 'HASH'))  {
        $error++;
        &add_content2xml_hash($out_hash, "error_string", "no macaddress specified or macaddress tag invalid");
        &add_content2xml_hash($out_hash, "error", "macaddress");
        &main::daemon_log("$session_id ERROR: no macaddress specified or macaddress tag invalid: $msg", 1); 
    }

    if (not $error) {
        # Get hostId
        $hostId = @{$msg_hash->{'hostId'}}[0];
        &add_content2xml_hash($out_hash, "hostId", $hostId);

        # Get macaddress
        $mac = @{$msg_hash->{'macaddress'}}[0];
        &add_content2xml_hash($out_hash, "macaddress", $mac);

        my $name= $hostId;
        $name=~ s/^([^.]+).*$/$1/;
        my $domain= $hostId;
        $domain=~ s/^[^.]+\.(.*)$/$1/;
        my ($description, $notes, $ip);

        if (defined @{$msg_hash->{'description'}}[0]){
            $description = @{$msg_hash->{'description'}}[0];
        }
        if (defined @{$msg_hash->{'notes'}}[0]){
            $notes = @{$msg_hash->{'notes'}}[0];
        }
        if (defined @{$msg_hash->{'ip'}}[0]){
            $ip = @{$msg_hash->{'ip'}}[0];
        }

        my $callobj;
        $callobj = {
            method  => 'createClient',
            params  => [ $name, $domain, $description, $notes, $ip, $mac ],
            id  => 1,
        };

        $sres = $main::opsi_client->call($main::opsi_url, $callobj);
        ($sres_err, $sres_err_string) = &check_opsi_res($sres);
        if ($sres_err){
            &main::daemon_log("$session_id ERROR: cannot create client: ".$sres_err_string, 1);
            &add_content2xml_hash($out_hash, "error", $sres_err_string);
        } else {
            &main::daemon_log("$session_id INFO: add opsi client '$hostId' with mac '$mac'", 5); 
        }
    }

    # Return message
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( &create_xml_string($out_hash) );
}

################################
# @brief Modifies the parameters description, mac or notes for an Opsi client if the corresponding message tags are given.
# @param msg - STRING - xml message with tag hostId and optional description, mac or notes
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message    
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_modify_client {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    my $hostId;
    my $error = 0;
    my ($sres, $sres_err, $sres_err_string);

    # Build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # Sanity check of needed parameter
    if ((not exists $msg_hash->{'hostId'}) || (@{$msg_hash->{'hostId'}} != 1) || (@{$msg_hash->{'hostId'}}[0] eq ref 'HASH'))  {
        $error++;
        &add_content2xml_hash($out_hash, "error_string", "no hostId specified or hostId tag invalid");
        &add_content2xml_hash($out_hash, "error", "hostId");
        &main::daemon_log("$session_id ERROR: no hostId specified or hostId tag invalid: $msg", 1); 
    }

    if (not $error) {
        # Get hostId
        $hostId = @{$msg_hash->{'hostId'}}[0];
        &add_content2xml_hash($out_hash, "hostId", $hostId);
        my $name= $hostId;
        $name=~ s/^([^.]+).*$/$1/;
        my $domain= $hostId;
        $domain=~ s/^[^.]+(.*)$/$1/;

        # Modify description, notes or mac if defined
        my ($description, $notes, $mac);
        my $callobj;
        if ((exists $msg_hash->{'description'}) && (@{$msg_hash->{'description'}} == 1) ){
            $description = @{$msg_hash->{'description'}}[0];
            $callobj = {
                method  => 'setHostDescription',
                params  => [ $hostId, $description ],
                id  => 1,
            };
            my $sres = $main::opsi_client->call($main::opsi_url, $callobj);
            my ($sres_err, $sres_err_string) = &check_opsi_res($sres);
            if ($sres_err){
                &main::daemon_log("ERROR: cannot set description: ".$sres_err_string, 1);
                &add_content2xml_hash($out_hash, "error", $sres_err_string);
            }
        }
        if ((exists $msg_hash->{'notes'}) && (@{$msg_hash->{'notes'}} == 1)) {
            $notes = @{$msg_hash->{'notes'}}[0];
            $callobj = {
                method  => 'setHostNotes',
                params  => [ $hostId, $notes ],
                id  => 1,
            };
            my $sres = $main::opsi_client->call($main::opsi_url, $callobj);
            my ($sres_err, $sres_err_string) = &check_opsi_res($sres);
            if ($sres_err){
                &main::daemon_log("ERROR: cannot set notes: ".$sres_err_string, 1);
                &add_content2xml_hash($out_hash, "error", $sres_err_string);
            }
        }
        if ((exists $msg_hash->{'mac'}) && (@{$msg_hash->{'mac'}} == 1)){
            $mac = @{$msg_hash->{'mac'}}[0];
            $callobj = {
                method  => 'setMacAddress',
                params  => [ $hostId, $mac ],
                id  => 1,
            };
            my $sres = $main::opsi_client->call($main::opsi_url, $callobj);
            my ($sres_err, $sres_err_string) = &check_opsi_res($sres);
            if ($sres_err){
                &main::daemon_log("ERROR: cannot set mac address: ".$sres_err_string, 1);
                &add_content2xml_hash($out_hash, "error", $sres_err_string);
            }
        }
    }

    # Return message
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( &create_xml_string($out_hash) );
}
 
################################
# @brief Get netboot products for specific host.
# @param msg - STRING - xml message with tag hostId
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_get_netboot_products {
    my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    my $hostId;

    # Build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }
    &add_content2xml_hash($out_hash, "xxx", "");

    # Get hostId if defined
    if ((exists $msg_hash->{'hostId'}) && (@{$msg_hash->{'hostId'}} == 1))  {
        $hostId = @{$msg_hash->{'hostId'}}[0];
        &add_content2xml_hash($out_hash, "hostId", $hostId);
    }

    # Move to XML string
    my $xml_msg= &create_xml_string($out_hash);

    my $callobj;
    # Check if we need to get host or global information
    if (defined $hostId){
      $callobj = {
          method  => 'getProductHostInformation_list',
          params  => [ $hostId, undef, 'netboot'],
          id  => 1,
      };

      my $res = $main::opsi_client->call($main::opsi_url, $callobj);
      if (not &check_opsi_res($res)){
          foreach my $product (@{$res->result}){
               my $replace= "<item><productId>".xml_quote($product->{'productId'})."<\/productId><name>".xml_quote($product->{'name'})."<\/name><description>".xml_quote($product->{'description'})."<\/description><state>".xml_quote($product->{'installationStatus'})."</state><action>".xml_quote($product->{'actionRequest'})."</action><\/item><xxx><\/xxx>";
               $xml_msg=~ s/<xxx><\/xxx>/\n$replace/;
          }
      }

    } else {

      # For hosts, only return the products that are or get installed
      $callobj = {
          method  => 'getProductInformation_list',
          params  => [ undef, 'netboot' ],
          id  => 1,
      };

      my $res = $main::opsi_client->call($main::opsi_url, $callobj);
      if (not &check_opsi_res($res)){
          foreach my $product (@{$res->result}) {
               my $replace= "<item><productId>".xml_quote($product->{'productId'})."<\/productId><name>".xml_quote($product->{'name'})."<\/name><description>".xml_quote($product->{'description'})."<\/description><\/item><xxx><\/xxx>";
               $xml_msg=~ s/<xxx><\/xxx>/\n$replace/;
          }
      }
    }

    $xml_msg=~ s/<xxx><\/xxx>//;

    # Retrun Message
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( $xml_msg );
}

################################   
# @brief Get product properties for a product and a specific host or gobally for a product.
# @param msg - STRING - xml message with tags productId and optional hostId
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_get_product_properties {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    my ($hostId, $productId);
    my $xml_msg;

    # Build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # Sanity check of needed parameter
    if ((not exists $msg_hash->{'productId'}) || (@{$msg_hash->{'productId'}} != 1) || (@{$msg_hash->{'productId'}}[0] eq ref 'HASH'))  {
        &add_content2xml_hash($out_hash, "error_string", "no productId specified or productId tag invalid");
        &add_content2xml_hash($out_hash, "error", "productId");
        &main::daemon_log("$session_id ERROR: no productId specified or productId tag invalid: $msg", 1); 

        # Return message
        return ( &create_xml_string($out_hash) );
    }

    # Get productid
    $productId = @{$msg_hash->{'productId'}}[0];
    &add_content2xml_hash($out_hash, "producId", "$productId");

    # Get hostId if defined
    if (defined @{$msg_hash->{'hostId'}}[0]){
      $hostId = @{$msg_hash->{'hostId'}}[0];
      &add_content2xml_hash($out_hash, "hostId", $hostId);
    }

    # Load actions
    my $callobj = {
      method  => 'getPossibleProductActions_list',
      params  => [ $productId ],
      id  => 1,
    };
    my $res = $main::opsi_client->call($main::opsi_url, $callobj);
    if (not &check_opsi_res($res)){
      foreach my $action (@{$res->result}){
        &add_content2xml_hash($out_hash, "action", $action);
      }
    }

    # Add place holder
    &add_content2xml_hash($out_hash, "xxx", "");

    # Move to XML string
    $xml_msg= &create_xml_string($out_hash);

    # JSON Query
    if (defined $hostId){
      $callobj = {
          method  => 'getProductProperties_hash',
          params  => [ $productId, $hostId ],
          id  => 1,
      };
    } else {
      $callobj = {
          method  => 'getProductProperties_hash',
          params  => [ $productId ],
          id  => 1,
      };
    }
    $res = $main::opsi_client->call($main::opsi_url, $callobj);

    # JSON Query 2
    $callobj = {
      method  => 'getProductPropertyDefinitions_listOfHashes',
      params  => [ $productId ],
      id  => 1,
    };

    # Assemble options
    my $res2 = $main::opsi_client->call($main::opsi_url, $callobj);
    my $values = {};
    my $descriptions = {};
    if (not &check_opsi_res($res2)){
        my $r= $res2->result;

          foreach my $entr (@$r){
            # Unroll values
            my $cnv;
            if (UNIVERSAL::isa( $entr->{'values'}, "ARRAY" )){
              foreach my $v (@{$entr->{'values'}}){
                $cnv.= "<value>$v</value>";
              }
            } else {
              $cnv= $entr->{'values'};
            }
            $values->{$entr->{'name'}}= $cnv;
            $descriptions->{$entr->{'name'}}= "<description>".$entr->{'description'}."</description>";
          }
    }

    if (not &check_opsi_res($res)){
        my $r= $res->result;
        foreach my $key (keys %{$r}) {
            my $item= "\n<item>";
            my $value= $r->{$key};
            my $dsc= "";
            my $vals= "";
            if (defined $descriptions->{$key}){
              $dsc= $descriptions->{$key};
            }
            if (defined $values->{$key}){
              $vals= $values->{$key};
            }
            $item.= "<$key>$dsc<current>".xml_quote($value)."</current>$vals</$key>";
            $item.= "</item>";
            $xml_msg=~ s/<xxx><\/xxx>/$item<xxx><\/xxx>/;
        }
    }

    $xml_msg=~ s/<xxx><\/xxx>//;

    # Return message
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( $xml_msg );
}

################################   
# @brief Set product properities for a specific host or globaly. Message needs one xml tag 'item' and within one xml tag 'name' and 'value'. The xml tags action and state are optional.
# @param msg - STRING - xml message with tags productId, action, state and optional hostId, action and state
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_set_product_properties {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    my ($productId, $hostId);

    # Build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # Sanity check of needed parameter
    if ((not exists $msg_hash->{'productId'}) || (@{$msg_hash->{'productId'}} != 1) || (@{$msg_hash->{'productId'}}[0] eq ref 'HASH'))  {
        &add_content2xml_hash($out_hash, "error_string", "no productId specified or productId tag invalid");
        &add_content2xml_hash($out_hash, "error", "productId");
        &main::daemon_log("$session_id ERROR: no productId specified or productId tag invalid: $msg", 1); 
        return ( &create_xml_string($out_hash) );
    }
    if (not exists $msg_hash->{'item'}) {
        &add_content2xml_hash($out_hash, "error_string", "message needs one xml-tag 'item' and within the xml-tags 'name' and 'value'");
        &add_content2xml_hash($out_hash, "error", "item");
        &main::daemon_log("$session_id ERROR: message needs one xml-tag 'item' and within the xml-tags 'name' and 'value': $msg", 1); 
        return ( &create_xml_string($out_hash) );
    } else {
        if ((not exists @{$msg_hash->{'item'}}[0]->{'name'}) || (@{@{$msg_hash->{'item'}}[0]->{'name'}} != 1 )) {
            &add_content2xml_hash($out_hash, "error_string", "message needs within the xml-tag 'item' one xml-tags 'name'");
            &add_content2xml_hash($out_hash, "error", "name");
            &main::daemon_log("$session_id ERROR: message needs within the xml-tag 'item' one xml-tags 'name': $msg", 1); 
            return ( &create_xml_string($out_hash) );
        }
        if ((not exists @{$msg_hash->{'item'}}[0]->{'value'}) || (@{@{$msg_hash->{'item'}}[0]->{'value'}} != 1 )) {
            &add_content2xml_hash($out_hash, "error_string", "message needs within the xml-tag 'item' one xml-tags 'value'");
            &add_content2xml_hash($out_hash, "error", "value");
            &main::daemon_log("$session_id ERROR: message needs within the xml-tag 'item' one xml-tags 'value': $msg", 1); 
            return ( &create_xml_string($out_hash) );
        }
    }
    # if no hostId is given, set_product_properties will act on globally
    if ((exists $msg_hash->{'hostId'}) && (@{$msg_hash->{'hostId'}} > 1))  {
        &add_content2xml_hash($out_hash, "error_string", "hostId contains no or more than one values");
        &add_content2xml_hash($out_hash, "error", "hostId");
        &main::daemon_log("$session_id ERROR: hostId contains no or more than one values: $msg", 1); 
        return ( &create_xml_string($out_hash) );
    }

        
    # Get productId
    $productId =  @{$msg_hash->{'productId'}}[0];
    &add_content2xml_hash($out_hash, "productId", $productId);

    # Get hostId if defined
    if (exists $msg_hash->{'hostId'}){
        $hostId = @{$msg_hash->{'hostId'}}[0];
        &add_content2xml_hash($out_hash, "hostId", $hostId);
    }

    # Set product states if requested
    if (defined @{$msg_hash->{'action'}}[0]){
        &_set_action($productId, @{$msg_hash->{'action'}}[0], $hostId);
    }
    if (defined @{$msg_hash->{'state'}}[0]){
        &_set_state($productId, @{$msg_hash->{'state'}}[0], $hostId);
    }

    # Find properties
    foreach my $item (@{$msg_hash->{'item'}}){
        # JSON Query
        my $callobj;

        if (defined $hostId){
            $callobj = {
                method  => 'setProductProperty',
                params  => [ $productId, $item->{'name'}[0], $item->{'value'}[0], $hostId ],
                id  => 1,
            };
        } else {
            $callobj = {
                method  => 'setProductProperty',
                params  => [ $productId, $item->{'name'}[0], $item->{'value'}[0] ],
                id  => 1,
            };
        }

        my $res = $main::opsi_client->call($main::opsi_url, $callobj);
        my ($res_err, $res_err_string) = &check_opsi_res($res);

        if ($res_err){
            &main::daemon_log("$session_id ERROR: communication failed while setting '".$item->{'name'}[0]."': ".$res_err_string, 1);
            &add_content2xml_hash($out_hash, "error", $res_err_string);
        }
    }


    # Return message
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( &create_xml_string($out_hash) );
}

################################   
# @brief Reports client hardware inventory.
# @param msg - STRING - xml message with tag hostId
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_get_client_hardware {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    my $hostId;
    my $error = 0;
    my $xml_msg;

    # Sanity check of needed parameter
	if (&_check_xml_tag_is_ok ($msg_hash, 'hostId')) {
        $hostId = @{$msg_hash->{'hostId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}


    # Build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
      &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }
	&add_content2xml_hash($out_hash, "hostId", "$hostId");
	&add_content2xml_hash($out_hash, "xxx", "");

    # Move to XML string
    $xml_msg= &create_xml_string($out_hash);
    
	my $res = &_callOpsi(method=>'getHardwareInformation_hash', params=>[ $hostId ]);
	if (not &check_opsi_res($res)){
		my $result= $res->result;
		if (ref $result eq "HASH") {
			foreach my $r (keys %{$result}){
				my $item= "\n<item><id>".xml_quote($r)."</id>";
				my $value= $result->{$r};
				foreach my $sres (@{$value}){

					foreach my $dres (keys %{$sres}){
						if (defined $sres->{$dres}){
							$item.= "<$dres>".xml_quote($sres->{$dres})."</$dres>";
						}
					}

				}
				$item.= "</item>";
				$xml_msg=~ s%<xxx></xxx>%$item<xxx></xxx>%;

			}
		}
	}

	$xml_msg=~ s/<xxx><\/xxx>//;

    # Return message
	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
    return ( $xml_msg );
}

################################   
# @brief Reports all Opsi clients. 
# @param msg - STRING - xml message 
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_list_clients {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];

    # Build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
      &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }
    &add_content2xml_hash($out_hash, "xxx", "");

    # Move to XML string
    my $xml_msg= &create_xml_string($out_hash);

    # JSON Query
    my $callobj = {
        method  => 'getClientsInformation_listOfHashes',
        params  => [ ],
        id  => 1,
    };

    my $res = $main::opsi_client->call($main::opsi_url, $callobj);
    if (not &check_opsi_res($res)){
        foreach my $host (@{$res->result}){
            my $item= "\n<item><name>".$host->{'hostId'}."</name>";
            $item.= "<mac>".xml_quote($host->{'macAddress'})."</mac>";
            if (defined($host->{'description'})){
                $item.= "<description>".xml_quote($host->{'description'})."</description>";
            }
            if (defined($host->{'notes'})){
                $item.= "<notes>".xml_quote($host->{'notes'})."</notes>";
            }
            if (defined($host->{'lastSeen'})){
                $item.= "<lastSeen>".xml_quote($host->{'lastSeen'})."</lastSeen>";
            }

            $item.= "</item>";
            $xml_msg=~ s%<xxx></xxx>%$item<xxx></xxx>%;
        }
    }
    $xml_msg=~ s/<xxx><\/xxx>//;

	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( $xml_msg );
}

################################   
# @brief Reports client software inventory.
# @param msg - STRING - xml message with tag hostId
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_get_client_software {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    my $error = 0;
    my $hostId;
    my $xml_msg;

    # Build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
      &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # Sanity check of needed parameter
    if ((exists $msg_hash->{'hostId'}) && (@{$msg_hash->{'hostId'}} != 1) || (@{$msg_hash->{'hostId'}}[0] eq ref 'HASH'))  {
        $error++;
        &add_content2xml_hash($out_hash, "error_string", "hostId contains no or more than one values");
        &add_content2xml_hash($out_hash, "error", "hostId");
        &main::daemon_log("$session_id ERROR: hostId contains no or more than one values: $msg", 1); 
    }

    if (not $error) {

    # Get hostId
        $hostId = @{$msg_hash->{'hostId'}}[0];
        &add_content2xml_hash($out_hash, "hostId", "$hostId");
        &add_content2xml_hash($out_hash, "xxx", "");
    }

    $xml_msg= &create_xml_string($out_hash);

    if (not $error) {

    # JSON Query
        my $callobj = {
            method  => 'getSoftwareInformation_hash',
            params  => [ $hostId ],
            id  => 1,
        };

        my $res = $main::opsi_client->call($main::opsi_url, $callobj);
        if (not &check_opsi_res($res)){
            my $result= $res->result;
        }

        $xml_msg=~ s/<xxx><\/xxx>//;

    }

    # Return message
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( $xml_msg );
}

################################   
# @brief Reports product for given hostId or globally.
# @param msg - STRING - xml message with optional tag hostId
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_get_local_products {
    my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    my $hostId;

    # Build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }
    &add_content2xml_hash($out_hash, "xxx", "");

    # Get hostId if defined
    if ((exists $msg_hash->{'hostId'}) && (@{$msg_hash->{'hostId'}} == 1))  {
        $hostId = @{$msg_hash->{'hostId'}}[0];
        &add_content2xml_hash($out_hash, "hostId", $hostId);
    }

    my $callobj;

    # Move to XML string
    my $xml_msg= &create_xml_string($out_hash);

    # Check if we need to get host or global information
    if (defined $hostId){
      $callobj = {
          method  => 'getProductHostInformation_list',
          params  => [ $hostId ],
          id  => 1,
      };

      my $res = $main::opsi_client->call($main::opsi_url, $callobj);
      if (not &check_opsi_res($res)){
          foreach my $product (@{$res->result}){
               my $replace= "<item><productId>".xml_quote($product->{'productId'})."<\/productId><name>".xml_quote($product->{'name'})."<\/name><description>".xml_quote($product->{'description'})."<\/description><state>".xml_quote($product->{'installationStatus'})."</state><action>".xml_quote($product->{'actionRequest'})."</action><\/item><xxx><\/xxx>";
               $xml_msg=~ s/<xxx><\/xxx>/\n$replace/;
          }
      }

    } else {

      # For hosts, only return the products that are or get installed
      $callobj = {
          method  => 'getProductInformation_list',
          params  => [ undef, 'localboot' ],
          id  => 1,
      };

      my $res = $main::opsi_client->call($main::opsi_url, $callobj);
      if (not &check_opsi_res($res)){
          foreach my $product (@{$res->result}) {
               my $replace= "<item><productId>".xml_quote($product->{'productId'})."<\/productId><name>".xml_quote($product->{'name'})."<\/name><description>".xml_quote($product->{'description'})."<\/description><\/item><xxx><\/xxx>";
               $xml_msg=~ s/<xxx><\/xxx>/\n$replace/;
          }
      }
    }

    $xml_msg=~ s/<xxx><\/xxx>//;

    # Retrun Message
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( $xml_msg );
}

################################   
# @brief Deletes a client from Opsi.
# @param msg - STRING - xml message with tag hostId
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_del_client {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    my $hostId;
    my $error = 0;

    # Build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
      &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # Sanity check of needed parameter
    if ((exists $msg_hash->{'hostId'}) && (@{$msg_hash->{'hostId'}} != 1) || (@{$msg_hash->{'hostId'}}[0] eq ref 'HASH'))  {
        $error++;
        &add_content2xml_hash($out_hash, "error_string", "hostId contains no or more than one values");
        &add_content2xml_hash($out_hash, "error", "hostId");
        &main::daemon_log("$session_id ERROR: hostId contains no or more than one values: $msg", 1); 
    }

    if (not $error) {

    # Get hostId
        $hostId = @{$msg_hash->{'hostId'}}[0];
        &add_content2xml_hash($out_hash, "hostId", "$hostId");

    # JSON Query
        my $callobj = {
            method  => 'deleteClient',
            params  => [ $hostId ],
            id  => 1,
        };
        my $res = $main::opsi_client->call($main::opsi_url, $callobj);
    }

    # Move to XML string
    my $xml_msg= &create_xml_string($out_hash);

    # Return message
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( $xml_msg );
}

################################   
# @brief Set a client in Opsi to install and trigger a wake on lan message (WOL).  
# @param msg - STRING - xml message with tags hostId, macaddress
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
# @return out_msg - STRING - feedback to GOsa in success and error case
sub opsi_install_client {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    my ($hostId, $macaddress);
    my $error = 0;
    my @out_msg_l;

    # Build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # Sanity check of needed parameter
    if ((not exists $msg_hash->{'hostId'}) || (@{$msg_hash->{'hostId'}} != 1) || (@{$msg_hash->{'hostId'}}[0] eq ref 'HASH'))  {
        $error++;
        &add_content2xml_hash($out_hash, "error_string", "no hostId specified or hostId tag invalid");
        &add_content2xml_hash($out_hash, "error", "hostId");
        &main::daemon_log("$session_id ERROR: no hostId specified or hostId tag invalid: $msg", 1); 
    }
    if ((not exists $msg_hash->{'macaddress'}) || (@{$msg_hash->{'macaddress'}} != 1) || (@{$msg_hash->{'macaddress'}}[0] eq ref 'HASH') )  {
        $error++;
        &add_content2xml_hash($out_hash, "error_string", "no macaddress specified or macaddress tag invalid");
        &add_content2xml_hash($out_hash, "error", "macaddress");
        &main::daemon_log("$session_id ERROR: no macaddress specified or macaddress tag invalid: $msg", 1); 
    } else {
        if ((exists $msg_hash->{'macaddress'}) && 
                ($msg_hash->{'macaddress'}[0] =~ /^([0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2})$/i)) {  
            $macaddress = $1; 
        } else { 
            $error ++; 
            &add_content2xml_hash($out_hash, "error_string", "given mac address is not correct");
            &add_content2xml_hash($out_hash, "error", "macaddress");
            &main::daemon_log("$session_id ERROR: given mac address is not correct: $msg", 1); 
        }
    }

    if (not $error) {

    # Get hostId
        $hostId = @{$msg_hash->{'hostId'}}[0];
        &add_content2xml_hash($out_hash, "hostId", "$hostId");

        # Load all products for this host with status != "not_installed" or actionRequest != "none"
        my $callobj = {
            method  => 'getProductStates_hash',
            params  => [ $hostId ],
            id  => 1,
        };

        my $hres = $main::opsi_client->call($main::opsi_url, $callobj);
        if (not &check_opsi_res($hres)){
            my $htmp= $hres->result->{$hostId};

            # check state != not_installed or action == setup -> load and add
            foreach my $product (@{$htmp}){
                # Now we've a couple of hashes...
                if ($product->{'installationStatus'} ne "not_installed" or
                        $product->{'actionRequest'} ne "none"){

                    # Do an action request for all these -> "setup".
                    $callobj = {
                        method  => 'setProductActionRequest',
                        params  => [ $product->{'productId'}, $hostId, "setup" ],
                        id  => 1,
                    };
                    my $res = $main::opsi_client->call($main::opsi_url, $callobj);
                    my ($res_err, $res_err_string) = &check_opsi_res($res);
                    if ($res_err){
                        &main::daemon_log("$session_id ERROR: cannot set product action request for '$hostId': ".$product->{'productId'}, 1);
                    } else {
                        &main::daemon_log("$session_id INFO: requesting 'setup' for '".$product->{'productId'}."' on $hostId", 1);
                    }
                }
            }
        }
        push(@out_msg_l, &create_xml_string($out_hash));
    

    # Build wakeup message for client
        if (not $error) {
            my $wakeup_hash = &create_xml_hash("trigger_wake", "GOSA", "KNOWN_SERVER");
            &add_content2xml_hash($wakeup_hash, 'macaddress', $macaddress);
            my $wakeup_msg = &create_xml_string($wakeup_hash);
            push(@out_msg_l, $wakeup_msg);

            # invoke trigger wake for this gosa-si-server
            &main::server_server_com::trigger_wake($wakeup_msg, $wakeup_hash, $session_id);
        }
    }
    
    # Return messages
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return @out_msg_l;
}

################################
# @brief Set action for an Opsi client
# @param product - STRING - Opsi product
# @param action - STRING - action
# @param hostId - STRING - Opsi hostId
sub _set_action {
  my $product= shift;
  my $action = shift;
  my $hostId = shift;
  my $callobj;

  $callobj = {
    method  => 'setProductActionRequest',
    params  => [ $product, $hostId, $action],
    id  => 1,
  };

  $main::opsi_client->call($main::opsi_url, $callobj);
}

################################
# @brief Set state for an Opsi client
# @param product - STRING - Opsi product
# @param action - STRING - state
# @param hostId - STRING - Opsi hostId
sub _set_state {
  my $product = shift;
  my $state = shift;
  my $hostId = shift;
  my $callobj;

  $callobj = {
    method  => 'setProductState',
    params  => [ $product, $hostId, $state ],
    id  => 1,
  };

  $main::opsi_client->call($main::opsi_url, $callobj);
}

################################
# @brief Create a license pool at Opsi server.
# @param licensePoolId The name of the pool (optional). 
# @param description The description of the pool (optional).
# @param productIds A list of assigned porducts of the pool (optional). 
# @param windowsSoftwareIds A list of windows software IDs associated to the pool (optional). 
sub opsi_createLicensePool {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
	my $out_hash;
	my $licensePoolId = defined $msg_hash->{'licensePoolId'} ? @{$msg_hash->{'licensePoolId'}}[0] : undef;
	my $description = defined $msg_hash->{'description'} ? @{$msg_hash->{'description'}}[0] : undef;
	my @productIds = defined $msg_hash->{'productIds'} ? $msg_hash->{'productIds'} : undef;
	my @windowsSoftwareIds = defined $msg_hash->{'windowsSoftwareIds'} ? $msg_hash->{'windowsSoftwareIds'} : undef;

	# Create license Pool
    my $callobj = {
        method  => 'createLicensePool',
        params  => [ $licensePoolId, $description, @productIds, @windowsSoftwareIds],
        id  => 1,
    };
    my $res = $main::opsi_client->call($main::opsi_url, $callobj);

	# Check Opsi error
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		# Create error message
		&main::daemon_log("$session_id ERROR: cannot create license pool at Opsi server: ".$res_error_str, 1);
		$out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source, $res_error_str);
		return ( &create_xml_string($out_hash) );
	}

	# Create function result message
	$out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source, $res->result);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }

	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
	return ( &create_xml_string($out_hash) );
}

################################
# @brief Return licensePoolId, description, productIds and windowsSoftwareIds for all found license pools.
sub opsi_getLicensePools_listOfHashes {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
	my $out_hash;

	# Fetch infos from Opsi server
    my $callobj = {
        method  => 'getLicensePools_listOfHashes',
        params  => [ ],
        id  => 1,
    };
    my $res = $main::opsi_client->call($main::opsi_url, $callobj);

	# Check Opsi error
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		# Create error message
		&main::daemon_log("$session_id ERROR: cannot get license pool ID list from Opsi server: ".$res_error_str, 1);
		$out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source, $res_error_str);
		return ( &create_xml_string($out_hash) );
	}

	# Create function result message
	my $res_hash = { 'hit'=> [] };
	foreach my $licensePool ( @{$res->result}) {
		my $licensePool_hash = { 'licensePoolId' => [$licensePool->{'licensePoolId'}],
			'description' => [$licensePool->{'description'}],
			'productIds' => $licensePool->{'productIds'},
			'windowsSoftwareIds' => $licensePool->{'windowsSoftwareIds'},
			};
		push( @{$res_hash->{hit}}, $licensePool_hash );
	}

	# Create function result message
	$out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }
	$out_hash->{result} = [$res_hash];

	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
	return ( &create_xml_string($out_hash) );
}

################################
# @brief Return productIds, windowsSoftwareIds and description for a given licensePoolId
# @param licensePoolId The name of the pool. 
sub opsi_getLicensePool_hash {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $licensePoolId;
	my $out_hash;

	# Check input sanity
	if (&_check_xml_tag_is_ok ($msg_hash, 'licensePoolId')) {
		$licensePoolId = @{$msg_hash->{'licensePoolId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, "", $session_id, $_);
	}

	# Fetch infos from Opsi server
    my $callobj = {
        method  => 'getLicensePool_hash',
        params  => [ $licensePoolId ],
        id  => 1,
    };
    my $res = $main::opsi_client->call($main::opsi_url, $callobj);

	# Check Opsi error
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		# Create error message
		&main::daemon_log("$session_id ERROR: cannot get license pool from Opsi server: ".$res_error_str, 1);
		$out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source);
		&add_content2xml_hash($out_hash, "error", $res_error_str);
		return ( &create_xml_string($out_hash) );
	}

	# Create function result message
	$out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }
	&add_content2xml_hash($out_hash, "licensePoolId", $res->result->{'licensePoolId'});
	&add_content2xml_hash($out_hash, "description", $res->result->{'description'});
	map(&add_content2xml_hash($out_hash, "productIds", "$_"), @{ $res->result->{'productIds'} });
	map(&add_content2xml_hash($out_hash, "windowsSoftwareIds", "$_"), @{ $res->result->{'windowsSoftwareIds'} });

	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
	return ( &create_xml_string($out_hash) );
}

sub _parse_getSoftwareLicenseUsages {
	my $res = shift;

	# Parse Opsi result
	my $tmp_licensePool_cache = {};
	my $res_hash = { 'hit'=> [] };
	foreach my $license ( @{$res}) {
		my $tmp_licensePool = $license->{'licensePoolId'};
		if (not exists $tmp_licensePool_cache->{$tmp_licensePool}) {
			# Fetch missing informations from Opsi and cache the results for a possible later usage
			my ($res, $err) = &_getLicensePool_hash('licensePoolId'=>$tmp_licensePool);
			if (not $err) {
				$tmp_licensePool_cache->{$tmp_licensePool} = $res;
			}
		}
		my $license_hash = { 'softwareLicenseId' => [$license->{'softwareLicenseId'}],
			'notes' => [$license->{'notes'}],
			'licenseKey' => [$license->{'licenseKey'}],
			'hostId' => [$license->{'hostId'}],
			'licensePoolId' => [$tmp_licensePool],
			};
		if (exists $tmp_licensePool_cache->{$tmp_licensePool}) {
			$license_hash->{$tmp_licensePool} = {'productIds'=>[], 'windowsSoftwareIds'=>[]};
			map (push (@{$license_hash->{$tmp_licensePool}->{productIds}}, $_), @{$tmp_licensePool_cache->{$tmp_licensePool}->{productIds}});
			map (push (@{$license_hash->{$tmp_licensePool}->{windowsSoftwareIds}}, $_), @{$tmp_licensePool_cache->{$tmp_licensePool}->{windowsSoftwareIds}});
		}
		push( @{$res_hash->{hit}}, $license_hash );
	}

	return $res_hash;
}

################################
# @brief Returns softwareLicenseId, notes, licenseKey, hostId and licensePoolId for optional given licensePoolId and hostId
# @param hostid Something like client_1.intranet.mydomain.de (optional).
# @param licensePoolId The name of the pool (optional). 
sub opsi_getSoftwareLicenseUsages {
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
	my $header = @{$msg_hash->{'header'}}[0];
	my $source = @{$msg_hash->{'source'}}[0];
	my $target = @{$msg_hash->{'target'}}[0];
	my $licensePoolId = defined $msg_hash->{'licensePoolId'} ? @{$msg_hash->{'licensePoolId'}}[0] : undef;
	my $hostId = defined $msg_hash->{'hostId'} ? @{$msg_hash->{'hostId'}}[0] : undef;
	my $out_hash;

	my ($res, $err) = &_getSoftwareLicenseUsages_listOfHashes('licensePoolId'=>$licensePoolId, 'hostId'=>$hostId);
	if ($err){
		return &_giveErrorFeedback($msg_hash, "cannot fetch software licenses from license pool : ".$res, $session_id);
	}

	# Parse Opsi result
	my $res_hash = &_parse_getSoftwareLicenseUsages($res);

	# Create function result message
	$out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }
	$out_hash->{result} = [$res_hash];

	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
	return ( &create_xml_string($out_hash) );
}

################################
# @brief Returns softwareLicenseId, notes, licenseKey, hostId and licensePoolId. Function return is identical to opsi_getSoftwareLicenseUsages
# @param productId Something like 'firefox', 'python' or anything else .
sub opsi_getSoftwareLicenseUsagesForProductId {
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
	my $header = @{$msg_hash->{'header'}}[0];
	my $source = @{$msg_hash->{'source'}}[0];

	# Check input sanity
	my $productId;
	if (&_check_xml_tag_is_ok ($msg_hash, 'productId')) {
		$productId= @{$msg_hash->{'productId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}

	# Fetch licensePoolId for productId
	my ($res, $err) = &_getLicensePoolId('productId'=>$productId);
	if ($err){
                my $out_hash = &create_xml_hash("answer_$header", $main::server_address, $source);
                $out_hash->{result} = [];
		return ( &create_xml_string($out_hash) );
	}
	my $licensePoolId = $res;   # We assume that there is only one pool for each productID!!!

	# Fetch softwareLiceceUsages for licensePoolId
	($res, $err) = &_getSoftwareLicenseUsages_listOfHashes('licensePoolId'=>$licensePoolId);
	if ($err){
		return &_giveErrorFeedback($msg_hash, "cannot fetch software licenses from license pool : ".$res, $session_id);
	}
	# Parse Opsi result
	my $res_hash = &_parse_getSoftwareLicenseUsages($res);

	# Create function result message
	my $out_hash = &create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }
	$out_hash->{result} = [$res_hash];

	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
	return ( &create_xml_string($out_hash) );
}

################################
# @brief Returns expirationDate, boundToHost, maxInstallation, licenseTyp, licensePoolIds and licenseKeys for a given softwareLicense ID.
# @param softwareLicenseId Identificator of a license.
sub opsi_getSoftwareLicense_hash {
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
	my $header = @{$msg_hash->{'header'}}[0];
	my $source = @{$msg_hash->{'source'}}[0];
	my $target = @{$msg_hash->{'target'}}[0];
	my $softwareLicenseId;
	my $out_hash;

	# Check input sanity
	if (&_check_xml_tag_is_ok ($msg_hash, 'softwareLicenseId')) {
		$softwareLicenseId = @{$msg_hash->{'softwareLicenseId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}

	my $callobj = {
		method  => 'getSoftwareLicense_hash',
		params  => [ $softwareLicenseId ],
		id  => 1,
	};
	my $res = $main::opsi_client->call($main::opsi_url, $callobj);

	# Check Opsi error
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		# Create error message
		&main::daemon_log("$session_id ERROR: cannot fetch information for license '$softwareLicenseId': ".$res_error_str, 1);
		$out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source, $res_error_str);
		return ( &create_xml_string($out_hash) );
	}
	
	# Create function result message
	$out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }
	&add_content2xml_hash($out_hash, "expirationDate", $res->result->{'expirationDate'});
	&add_content2xml_hash($out_hash, "boundToHost", $res->result->{'boundToHost'});
	&add_content2xml_hash($out_hash, "maxInstallations", $res->result->{'maxInstallations'});
	&add_content2xml_hash($out_hash, "licenseTyp", $res->result->{'licenseTyp'});
	foreach my $licensePoolId ( @{$res->result->{'licensePoolIds'}}) {
		&add_content2xml_hash($out_hash, "licensePoolId", $licensePoolId);
		&add_content2xml_hash($out_hash, $licensePoolId, $res->result->{'licenseKeys'}->{$licensePoolId});
	}

	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
	return ( &create_xml_string($out_hash) );
}

################################
# @brief Delete licnese pool by license pool ID. A pool can only be deleted if there are no software licenses bound to the pool. 
# The fixed parameter deleteLicenses=True specifies that all software licenses bound to the pool are being deleted. 
# @param licensePoolId The name of the pool. 
sub opsi_deleteLicensePool {
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $licensePoolId;
	my $out_hash;

	# Check input sanity
	if (&_check_xml_tag_is_ok ($msg_hash, 'licensePoolId')) {
		$licensePoolId = @{$msg_hash->{'licensePoolId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}

	# Fetch softwareLicenseIds used in license pool
	# This has to be done because function deleteLicensePool deletes the pool and the corresponding software licenses
	# but not the license contracts of the software licenses. In our case each software license has exactly one license contract. 
	my $callobj = {
		method  => 'getSoftwareLicenses_listOfHashes',
		params  => [ ],
		id  => 1,
	};
	my $res = $main::opsi_client->call($main::opsi_url, $callobj);

	# Keep list of licenseContractIds in mind to delete it after the deletion of the software licenses
	my @lCI_toBeDeleted;
	foreach my $softwareLicenseHash ( @{$res->result} ) {
		if ((@{$softwareLicenseHash->{'licensePoolIds'}} == 0) || (@{$softwareLicenseHash->{'licensePoolIds'}}[0] ne $licensePoolId)) { 
			next; 
		}  
		push (@lCI_toBeDeleted, $softwareLicenseHash->{'licenseContractId'});
	}

	# Delete license pool at Opsi server
    $callobj = {
        method  => 'deleteLicensePool',
        params  => [ $licensePoolId, 'deleteLicenses=True'  ],
        id  => 1,
    };
    $res = $main::opsi_client->call($main::opsi_url, $callobj);
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		# Create error message
		&main::daemon_log("$session_id ERROR: cannot delete license pool at Opsi server: ".$res_error_str, 1);
		$out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source, $res_error_str);
		return ( &create_xml_string($out_hash) );
	} 

	# Delete each license contract connected with the license pool
	foreach my $licenseContractId ( @lCI_toBeDeleted ) {
		my $callobj = {
			method  => 'deleteLicenseContract',
			params  => [ $licenseContractId ],
			id  => 1,
		};
		my $res = $main::opsi_client->call($main::opsi_url, $callobj);
		my ($res_error, $res_error_str) = &check_opsi_res($res);
		if ($res_error){
			# Create error message
			&main::daemon_log("$session_id ERROR: cannot delete license contract '$licenseContractId' connected with license pool '$licensePoolId' at Opsi server: ".$res_error_str, 1);
			$out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source, $res_error_str);
			return ( &create_xml_string($out_hash) );
		}
	}

	# Create function result message
	$out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }

	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
	return ( &create_xml_string($out_hash) );
}

################################
# @brief Create a license contract, create a software license and add the software license to the license pool
# @param licensePoolId The name of the pool the license should be assigned.
# @param licenseKey The license key.
# @param partner Name of the license partner (optional).
# @param conclusionDate Date of conclusion of license contract (optional)
# @param notificationDate Date of notification that license is running out soon (optional).
# @param notes This is the place for some notes (optional)
# @param softwareLicenseId Identificator of a license (optional).
# @param licenseTyp Typ of a licnese, either "OEM", "VOLUME" or "RETAIL" (optional).
# @param maxInstallations The number of clients use this license (optional). 
# @param boundToHost The name of the client the license is bound to (optional).
# @param expirationDate The date when the license is running down (optional). 
sub opsi_createLicense {
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
	my $partner = defined $msg_hash->{'partner'} ? @{$msg_hash->{'partner'}}[0] : undef;
	my $conclusionDate = defined $msg_hash->{'conclusionDate'} ? @{$msg_hash->{'conclusionDate'}}[0] : undef;
	my $notificationDate = defined $msg_hash->{'notificationDate'} ? @{$msg_hash->{'notificationDate'}}[0] : undef;
	my $notes = defined $msg_hash->{'notes'} ? @{$msg_hash->{'notes'}}[0] : undef;
	my $licenseContractId = undef;
	my $softwareLicenseId = defined $msg_hash->{'softwareLicenseId'} ? @{$msg_hash->{'softwareLicenseId'}}[0] : undef;
	my $licenseType = defined $msg_hash->{'licenseType'} ? @{$msg_hash->{'licenseType'}}[0] : undef;
	my $maxInstallations = defined $msg_hash->{'maxInstallations'} ? @{$msg_hash->{'maxInstallations'}}[0] : undef;
	my $boundToHost = defined $msg_hash->{'boundToHost'} ? @{$msg_hash->{'boundToHost'}}[0] : undef;
	my $expirationDate = defined $msg_hash->{'expirationDate'} ? @{$msg_hash->{'expirationDate'}}[0] : undef;
	my $licensePoolId;
	my $licenseKey;
	my $out_hash;

	# Check input sanity
	if (&_check_xml_tag_is_ok ($msg_hash, 'licenseKey')) {
		$licenseKey = @{$msg_hash->{'licenseKey'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}
	if (&_check_xml_tag_is_ok ($msg_hash, 'licensePoolId')) {
		$licensePoolId = @{$msg_hash->{'licensePoolId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}
	if ((defined $licenseType) && (not exists $licenseTyp_hash->{$licenseType})) {
		return &_giveErrorFeedback($msg_hash, "The typ of a license can be either 'OEM', 'VOLUME' or 'RETAIL'.", $session_id);
	}
	
	# Automatically define licenseContractId if ID is not given
	if (defined $softwareLicenseId) { 
		$licenseContractId = "c_".$softwareLicenseId;
	}

	# Create license contract at Opsi server
    my $callobj = {
        method  => 'createLicenseContract',
        params  => [ $licenseContractId, $partner, $conclusionDate, $notificationDate, undef, $notes ],
        id  => 1,
    };
    my $res = $main::opsi_client->call($main::opsi_url, $callobj);

	# Check Opsi error
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		# Create error message
		&main::daemon_log("$session_id ERROR: cannot create license contract at Opsi server: ".$res_error_str, 1);
		$out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source, $res_error_str);
		return ( &create_xml_string($out_hash) );
	}
	
	$licenseContractId = $res->result;

	# Create software license at Opsi server
    $callobj = {
        method  => 'createSoftwareLicense',
        params  => [ $softwareLicenseId, $licenseContractId, $licenseType, $maxInstallations, $boundToHost, $expirationDate ],
        id  => 1,
    };
    $res = $main::opsi_client->call($main::opsi_url, $callobj);

	# Check Opsi error
	($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		# Create error message
		&main::daemon_log("$session_id ERROR: cannot create software license at Opsi server: ".$res_error_str, 1);
		$out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source, $res_error_str);
		return ( &create_xml_string($out_hash) );
	}

	$softwareLicenseId = $res->result;

	# Add software license to license pool
	$callobj = {
        method  => 'addSoftwareLicenseToLicensePool',
        params  => [ $softwareLicenseId, $licensePoolId, $licenseKey ],
        id  => 1,
    };
    $res = $main::opsi_client->call($main::opsi_url, $callobj);

	# Check Opsi error
	($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		# Create error message
		&main::daemon_log("$session_id ERROR: cannot add software license to license pool at Opsi server: ".$res_error_str, 1);
		$out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source, $res_error_str);
		return ( &create_xml_string($out_hash) );
	}

	# Create function result message
	$out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }
	
	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
	return ( &create_xml_string($out_hash) );
}

################################
# @brief Assign a software license to a host
# @param hostid Something like client_1.intranet.mydomain.de
# @param licensePoolId The name of the pool.
sub opsi_assignSoftwareLicenseToHost {
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
	my $hostId;
	my $licensePoolId;

	# Check input sanity
	if (&_check_xml_tag_is_ok ($msg_hash, 'hostId')) {
		$hostId = @{$msg_hash->{'hostId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}
	if (&_check_xml_tag_is_ok ($msg_hash, 'licensePoolId')) {
		$licensePoolId = @{$msg_hash->{'licensePoolId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}

	# Assign a software license to a host
	my $callobj = {
        method  => 'getAndAssignSoftwareLicenseKey',
        params  => [ $hostId, $licensePoolId ],
        id  => 1,
    };
    my $res = $main::opsi_client->call($main::opsi_url, $callobj);

	# Check Opsi error
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		# Create error message
		&main::daemon_log("$session_id ERROR: cannot assign a software license to a host at Opsi server: ".$res_error_str, 1);
		my $out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source, $res_error_str);
		return ( &create_xml_string($out_hash) );
	}

	# Create function result message
	my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }
	
	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
	return ( &create_xml_string($out_hash) );
}

################################
# @brief Unassign a software license from a host.
# @param hostid Something like client_1.intranet.mydomain.de
# @param licensePoolId The name of the pool.
sub opsi_unassignSoftwareLicenseFromHost {
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
	my $hostId;
	my $licensePoolId;

	# Check input sanity
	if (&_check_xml_tag_is_ok ($msg_hash, 'hostId')) {
		$hostId = @{$msg_hash->{'hostId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}
	if (&_check_xml_tag_is_ok ($msg_hash, 'licensePoolId')) {
		$licensePoolId = @{$msg_hash->{'licensePoolId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}

	# Unassign a software license from a host
	my $callobj = {
        method  => 'deleteSoftwareLicenseUsage',
        params  => [ $hostId, '', $licensePoolId ],
        id  => 1,
    };
    my $res = $main::opsi_client->call($main::opsi_url, $callobj);

	# Check Opsi error
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		# Create error message
		&main::daemon_log("$session_id ERROR: cannot unassign a software license from a host at Opsi server: ".$res_error_str, 1);
		my $out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source, $res_error_str);
		return ( &create_xml_string($out_hash) );
	}

	# Create function result message
	my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }
	
	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
	return ( &create_xml_string($out_hash) );
}

################################
# @brief Unassign all software licenses from a host
# @param hostid Something like client_1.intranet.mydomain.de
sub opsi_unassignAllSoftwareLicensesFromHost {
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
	my $hostId;

	# Check input sanity
	if (&_check_xml_tag_is_ok ($msg_hash, 'hostId')) {
		$hostId = @{$msg_hash->{'hostId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}

	# Unassign all software licenses from a host
	my $callobj = {
        method  => 'deleteAllSoftwareLicenseUsages',
        params  => [ $hostId ],
        id  => 1,
    };
    my $res = $main::opsi_client->call($main::opsi_url, $callobj);

	# Check Opsi error
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		# Create error message
		&main::daemon_log("$session_id ERROR: cannot unassign a software license from a host at Opsi server: ".$res_error_str, 1);
		my $out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source, $res_error_str);
		return ( &create_xml_string($out_hash) );
	}

	# Create function result message
	my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }
	
	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
	return ( &create_xml_string($out_hash) );
}


################################
# @brief Returns the assigned licensePoolId and licenses, how often the product is installed and at which host
# and the number of max and remaining installations for a given OPSI product.
# @param productId Identificator of an OPSI product.
sub opsi_getLicenseInformationForProduct {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
	my $productId;
	my $out_hash;

	# Check input sanity
	if (&_check_xml_tag_is_ok ($msg_hash, 'productId')) {
		$productId = @{$msg_hash->{'productId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}

	# Fetch infos from Opsi server
    my $callobj = {
        method  => 'getLicensePoolId',
        params  => [ $productId ],
        id  => 1,
    };
    #my $res = $main::opsi_client->call($main::opsi_url, $callobj);
    my $res = $opsi_client->call($opsi_url, $callobj);

	# Check Opsi error
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		return &_giveErrorFeedback($msg_hash, "cannot get license pool for product '$productId' : ".$res_error_str, $session_id);
	} 
	
	my $licensePoolId = $res->result;

	# Fetch statistic information for given pool ID
	$callobj = {
		method  => 'getLicenseStatistics_hash',
		params  => [ ],
		id  => 1,
	};
	$res = $opsi_client->call($opsi_url, $callobj);

	# Check Opsi error
	($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){
		# Create error message
		&main::daemon_log("$session_id ERROR: cannot get statistic informations for license pools : ".$res_error_str, 1);
		$out_hash = &main::create_xml_hash("error_$header", $main::server_address, $source, $res_error_str);
		return ( &create_xml_string($out_hash) );
	}

	# Create function result message
	$out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }
	&add_content2xml_hash($out_hash, "licensePoolId", $licensePoolId);
	&add_content2xml_hash($out_hash, "licenses", $res->result->{$licensePoolId}->{'licenses'});
	&add_content2xml_hash($out_hash, "usageCount", $res->result->{$licensePoolId}->{'usageCount'});
	&add_content2xml_hash($out_hash, "maxInstallations", $res->result->{$licensePoolId}->{'maxInstallations'});
	&add_content2xml_hash($out_hash, "remainingInstallations", $res->result->{$licensePoolId}->{'remainingInstallations'});
	map(&add_content2xml_hash($out_hash, "usedBy", "$_"), @{ $res->result->{$licensePoolId}->{'usedBy'}});

	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
	return ( &create_xml_string($out_hash) );
}


################################
# @brief Returns licensePoolId, description, a list of productIds, al list of windowsSoftwareIds and a list of licenses for a given licensePoolId. 
# Each license contains softwareLicenseId, maxInstallations, licenseType, licensePoolIds, licenseKeys, hostIds, expirationDate, boundToHost and licenseContractId.
# The licenseContract contains conclusionDate, expirationDate, notes, notificationDate and partner. 
# @param licensePoolId The name of the pool.
sub opsi_getPool {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];

	# Check input sanity
	my $licensePoolId;
	if (&_check_xml_tag_is_ok ($msg_hash, 'licensePoolId')) {
		$licensePoolId = @{$msg_hash->{'licensePoolId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}

	# Create hash for the answer
	my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }

	# Call Opsi
	my ($res, $err) = &_getLicensePool_hash( 'licensePoolId'=> $licensePoolId );
	if ($err){
		return &_giveErrorFeedback($msg_hash, "cannot get license pool from Opsi server: ".$res, $session_id);
	}
	# Add data to outgoing hash
	&add_content2xml_hash($out_hash, "licensePoolId", $res->{'licensePoolId'});
	&add_content2xml_hash($out_hash, "description", $res->{'description'});
	map(&add_content2xml_hash($out_hash, "productIds", "$_"), @{ $res->{'productIds'} });
	map(&add_content2xml_hash($out_hash, "windowsSoftwareIds", "$_"), @{ $res->{'windowsSoftwareIds'} });


	# Call Opsi two times
	my ($usages_res, $usages_err) = &_getSoftwareLicenseUsages_listOfHashes('licensePoolId'=>$licensePoolId);
	if ($usages_err){
		return &_giveErrorFeedback($msg_hash, "cannot get software license usage information from Opsi server: ".$usages_res, $session_id);
	}
	my ($licenses_res, $licenses_err) = &_getSoftwareLicenses_listOfHashes();
	if ($licenses_err){
		return &_giveErrorFeedback($msg_hash, "cannot get software license information from Opsi server: ".$licenses_res, $session_id);
	}

	# Add data to outgoing hash
	# Parse through all software licenses and select those associated to the pool
	my $res_hash = { 'hit'=> [] };
	foreach my $license ( @$licenses_res) {
		# Each license hash has a list of licensePoolIds so go through this list and search for matching licensePoolIds
		my $found = 0;
		my @licensePoolIds_list = @{$license->{licensePoolIds}};
		foreach my $lPI ( @licensePoolIds_list) {
			if ($lPI eq $licensePoolId) { $found++ }
		}
		if (not $found ) { next; };
		# Found matching licensePoolId
		my $license_hash = { 'softwareLicenseId' => [$license->{'softwareLicenseId'}],
			'licenseKeys' => {},
			'expirationDate' => [$license->{'expirationDate'}],
			'boundToHost' => [$license->{'boundToHost'}],
			'maxInstallations' => [$license->{'maxInstallations'}],
			'licenseType' => [$license->{'licenseType'}],
			'licenseContractId' => [$license->{'licenseContractId'}],
			'licensePoolIds' => [],
			'hostIds' => [],
			};
		foreach my $licensePoolId (@{ $license->{'licensePoolIds'}}) {
			push( @{$license_hash->{'licensePoolIds'}}, $licensePoolId);
			$license_hash->{licenseKeys}->{$licensePoolId} =  [ $license->{'licenseKeys'}->{$licensePoolId} ];
		}
		foreach my $usage (@$usages_res) {
			# Search for hostIds with matching softwareLicenseId
			if ($license->{'softwareLicenseId'} eq $usage->{'softwareLicenseId'}) {
				push( @{ $license_hash->{hostIds}}, $usage->{hostId});
			}
		}

		# Each softwareLicenseId has one licenseContractId, fetch contract details for each licenseContractId
		my ($lContract_res, $lContract_err) = &_getLicenseContract_hash('licenseContractId'=>$license->{licenseContractId});
		if ($lContract_err){
			return &_giveErrorFeedback($msg_hash, "cannot get software license contract information from Opsi server: ".$licenses_res, $session_id);
		}
		$license_hash->{$license->{'licenseContractId'}} = [];
		my $licenseContract_hash = { 'conclusionDate' => [$lContract_res->{conclusionDate}],
			'notificationDate' => [$lContract_res->{notificationDate}],
			'notes' => [$lContract_res->{notes}],
			'exirationDate' => [$lContract_res->{expirationDate}],
			'partner' => [$lContract_res->{partner}],
		};
		push( @{$license_hash->{licenseContractData}}, $licenseContract_hash );

		push( @{$res_hash->{hit}}, $license_hash );
	}
	$out_hash->{licenses} = [$res_hash];

	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
    return ( &create_xml_string($out_hash) );
}


################################
# @brief Removes at first the software license from license pool and than deletes the software license. 
# Attention, the software license has to exists otherwise it will lead to an Opsi internal server error.
# @param softwareLicenseId Identificator of a license.
# @param licensePoolId The name of the pool.
sub opsi_removeLicense {
	my $startTime = Time::HiRes::time;
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];

	# Check input sanity
	my $softwareLicenseId;
	if (&_check_xml_tag_is_ok ($msg_hash, 'softwareLicenseId')) {
		$softwareLicenseId = @{$msg_hash->{'softwareLicenseId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}
	my $licensePoolId;
	if (&_check_xml_tag_is_ok ($msg_hash, 'licensePoolId')) {
		$licensePoolId = @{$msg_hash->{'licensePoolId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}
	
	# Call Opsi
	my ($res, $err) = &_removeSoftwareLicenseFromLicensePool( 'licensePoolId' => $licensePoolId, 'softwareLicenseId' => $softwareLicenseId );
	if ($err){
		return &_giveErrorFeedback($msg_hash, "cannot delete software license from pool: ".$res, $session_id);
	}

	# Call Opsi
	($res, $err) = &_deleteSoftwareLicense( 'softwareLicenseId'=>$softwareLicenseId );
	if ($err){
		return &_giveErrorFeedback($msg_hash, "cannot delete software license from Opsi server: ".$res, $session_id);
	}

	# Create hash for the answer
	my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }
	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
	return ( &create_xml_string($out_hash) );
}


################################
# @brief Return softwareLicenseId, maxInstallations, licenseType, licensePoolIds, licenseContractId, expirationDate, boundToHost and a list of productIds.
# @param hostId Something like client_1.intranet.mydomain.de
sub opsi_getReservedLicenses {
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
	my $header = @{$msg_hash->{'header'}}[0];
	my $source = @{$msg_hash->{'source'}}[0];

	# Check input sanity
	my $hostId;
	if (&_check_xml_tag_is_ok ($msg_hash, 'hostId')) {
		$hostId = @{$msg_hash->{'hostId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}

	# Fetch informations from Opsi server
	my ($license_res, $license_err) = &_getSoftwareLicenses_listOfHashes();
	if ($license_err){
		return &_giveErrorFeedback($msg_hash, "cannot get software license information from Opsi server: ".$license_res, $session_id);
	}

	# Parse result
	my $res_hash = { 'hit'=> [] };
	foreach my $license ( @$license_res) {
		if ($license->{boundToHost} ne $hostId) { next; }

		my $license_hash = { 'softwareLicenseId' => [$license->{'softwareLicenseId'}],
			'maxInstallations' => [$license->{'maxInstallations'}],
			'boundToHost' => [$license->{'boundToHost'}],
			'expirationDate' => [$license->{'expirationDate'}],
			'licenseContractId' => [$license->{'licenseContractId'}],
			'licenseType' => [$license->{'licenseType'}],
			'licensePoolIds' => [],
			};
		
		foreach my $licensePoolId (@{$license->{'licensePoolIds'}}) {
			# Fetch information for license pools containing a software license which is bound to given host
			my ($pool_res, $pool_err) = &_getLicensePool_hash( 'licensePoolId'=>$licensePoolId );
			if ($pool_err){
				return &_giveErrorFeedback($msg_hash, "cannot get license pool from Opsi server: ".$pool_res, $session_id);
			}

			# Add licensePool information to result hash
			push (@{$license_hash->{licensePoolIds}}, $licensePoolId);
			$license_hash->{$licensePoolId} = {'productIds'=>[], 'windowsSoftwareIds'=>[]};
			map (push (@{$license_hash->{$licensePoolId}->{productIds}}, $_), @{$pool_res->{productIds}});
			map (push (@{$license_hash->{$licensePoolId}->{windowsSoftwareIds}}, $_), @{$pool_res->{windowsSoftwareIds}});
		}
		push( @{$res_hash->{hit}}, $license_hash );
	}
	my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }
	$out_hash->{licenses} = [$res_hash];

	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
    return ( &create_xml_string($out_hash) );
}

################################
# @brief Bound the given softwareLicenseId to the given host.
# @param hostId Opsi hostId
# @param softwareLicenseId Identificator of a license (optional).
sub opsi_boundHostToLicense {
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
	my $header = @{$msg_hash->{'header'}}[0];
	my $source = @{$msg_hash->{'source'}}[0];

	# Check input sanity
	my $hostId;
	if (&_check_xml_tag_is_ok ($msg_hash, 'hostId')) {
		$hostId = @{$msg_hash->{'hostId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}
	my $softwareLicenseId;
	if (&_check_xml_tag_is_ok ($msg_hash, 'softwareLicenseId')) {
		$softwareLicenseId = @{$msg_hash->{'softwareLicenseId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}

	# Fetch informations from Opsi server
	my ($license_res, $license_err) = &_getSoftwareLicenses_listOfHashes();
	if ($license_err){
		return &_giveErrorFeedback($msg_hash, "cannot get software license information from Opsi server: ".$license_res, $session_id);
	}

	# Memorize parameter for given softwareLicenseId
	my $licenseContractId;
	my $licenseType;
	my $maxInstallations;
	my $boundToHost;
	my $expirationDate = "";
	my $found;
	foreach my $license (@$license_res) {
		if ($license->{softwareLicenseId} ne $softwareLicenseId) { next; }
		$licenseContractId = $license->{licenseContractId};
		$licenseType = $license->{licenseType};
		$maxInstallations = $license->{maxInstallations};
		$expirationDate = $license->{expirationDate};
		$found++;
	}

	if (not $found) {
		return &_giveErrorFeedback($msg_hash, "no softwarelicenseId found with name '".$softwareLicenseId."'", $session_id);
	}

	# Set boundToHost option for a given software license
	my ($bound_res, $bound_err) = &_createSoftwareLicense('softwareLicenseId'=>$softwareLicenseId, 
			'licenseContractId' => $licenseContractId, 
			'licenseType' => $licenseType, 
			'maxInstallations' => $maxInstallations, 
			'boundToHost' => $hostId, 
			'expirationDate' => $expirationDate);
	if ($bound_err) {
		return &_giveErrorFeedback($msg_hash, "cannot set boundToHost for given softwareLicenseId and hostId: ".$bound_res, $session_id);
	}

	my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }

	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));
	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : $elapsedTime seconds", 1034);
    return ( &create_xml_string($out_hash) );
}

################################
# @brief Release a software license formerly bound to a host.
# @param softwareLicenseId Identificator of a license.
sub opsi_unboundHostFromLicense {
	# This is really mad! Opsi is not able to unbound a lincense from a host. To provide the functionality for GOsa
	# 4 rpc calls to Opsi are necessary. First, fetch all data for the given softwareLicenseId, then all details for the associated
	# licenseContractId, then delete the softwareLicense and finally recreate the softwareLicense without the boundToHost option. NASTY!
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
	my $header = @{$msg_hash->{'header'}}[0];
	my $source = @{$msg_hash->{'source'}}[0];

	# Check input sanity
	my $softwareLicenseId;
	if (&_check_xml_tag_is_ok ($msg_hash, 'softwareLicenseId')) {
		$softwareLicenseId = @{$msg_hash->{'softwareLicenseId'}}[0];
	} else {
		return &_giveErrorFeedback($msg_hash, $_, $session_id);
	}
	
	# Memorize parameter witch are required for this procedure
	my $licenseContractId;
	my $licenseType;
	my $maxInstallations;
	my $expirationDate;
	my $partner;
	my $conclusionDate;
	my $notificationDate;
	my $notes;
	my $licensePoolId;
	my $licenseKey;

	# Fetch license informations from Opsi server
	my ($license_res, $license_err) = &_getSoftwareLicenses_listOfHashes();
	if ($license_err){
		return &_giveErrorFeedback($msg_hash, "cannot get software license information from Opsi server, required to unbound license from host: ".$license_res, $session_id);
	}
	my $found = 0;
	foreach my $license (@$license_res) {
		if (($found > 0) || ($license->{softwareLicenseId} ne $softwareLicenseId)) { next; }
		$licenseContractId = $license->{licenseContractId};
		$licenseType = $license->{licenseType};
		$maxInstallations = $license->{maxInstallations};
		$expirationDate = $license->{expirationDate};
		$licensePoolId = @{$license->{licensePoolIds}}[0];
		$licenseKey = $license->{licenseKeys}->{$licensePoolId};
		$found++;
	}
	
	# Fetch contract informations from Opsi server
	my ($contract_res, $contract_err) = &_getLicenseContract_hash('licenseContractId'=>$licenseContractId);
	if ($contract_err){
		return &_giveErrorFeedback($msg_hash, "cannot get contract license information from Opsi server, required to unbound license from host: ".$license_res, $session_id);
	}
	$partner = $contract_res->{partner};
	$conclusionDate = $contract_res->{conclusionDate};
	$notificationDate = $contract_res->{notificationDate};
	$expirationDate = $contract_res->{expirationDate};
	$notes = $contract_res->{notes};

	# Delete software license
	my ($res, $err) = &_deleteSoftwareLicense( 'softwareLicenseId' => $softwareLicenseId, 'removeFromPools'=> "true" );
	if ($err) {
		return &_giveErrorFeedback($msg_hash, "cannot delet license from Opsi server, required to unbound license from host : ".$res, $session_id);
	}

	# Recreate software license without boundToHost
	($res, $err) = &_createLicenseContract(	'licenseContractId' => $licenseContractId, 'partner' => $partner, 'conclusionDate' => $conclusionDate, 
			'notificationDate' => $notificationDate, 'expirationDate' => $expirationDate, 'notes' => $notes	);
	if ($err) {
		return &_giveErrorFeedback($msg_hash, "cannot create license contract at Opsi server, required to unbound license from host : ".$res, $session_id);
	}
	($res, $err) = &_createSoftwareLicense( 'softwareLicenseId' => $softwareLicenseId, 'licenseContractId' => $licenseContractId, 'licenseType' => $licenseType, 
			'maxInstallations' => $maxInstallations, 'boundToHost' => "", 'expirationDate' => $expirationDate	);
	if ($err) {
		return &_giveErrorFeedback($msg_hash, "cannot create software license at Opsi server, required to unbound license from host : ".$res, $session_id);
	}
	($res, $err) = &_addSoftwareLicenseToLicensePool( 'softwareLicenseId' => $softwareLicenseId, 'licensePoolId' => $licensePoolId, 'licenseKey' => $licenseKey );
	if ($err) {
		return &_giveErrorFeedback($msg_hash, "cannot add software license to license pool at Opsi server, required to unbound license from host : ".$res, $session_id);
	}

	my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }

	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( &create_xml_string($out_hash) );
}

################################
# @brief Returns a list of licenses with softwaerLicenseId, maxInstallations, boundToHost, expirationDate, licenseContractId, licenseType, a list of licensePoolIds with associated licenseKeys
sub opsi_getAllSoftwareLicenses {
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
	my $header = @{$msg_hash->{'header'}}[0];
	my $source = @{$msg_hash->{'source'}}[0];

	my ($res, $err) = &_getSoftwareLicenses_listOfHashes();
	if ($err) {
		return &_giveErrorFeedback($msg_hash, "cannot fetch software licenses from Opsi server : ".$res, $session_id);
	}

	# Parse result
	my $res_hash = { 'hit'=> [] };
	foreach my $license ( @$res) {
		my $license_hash = { 'softwareLicenseId' => [$license->{'softwareLicenseId'}],
			'maxInstallations' => [$license->{'maxInstallations'}],
			'boundToHost' => [$license->{'boundToHost'}],
			'expirationDate' => [$license->{'expirationDate'}],
			'licenseContractId' => [$license->{'licenseContractId'}],
			'licenseType' => [$license->{'licenseType'}],
			'licensePoolIds' => [],
			'licenseKeys'=> {}
			};
		foreach my $licensePoolId (@{$license->{'licensePoolIds'}}) {
			push( @{$license_hash->{'licensePoolIds'}}, $licensePoolId);
			$license_hash->{licenseKeys}->{$licensePoolId} =  [ $license->{'licenseKeys'}->{$licensePoolId} ];
		}
		push( @{$res_hash->{hit}}, $license_hash );
	}
	
	my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
	$out_hash->{licenses} = [$res_hash];
	if (exists $msg_hash->{forward_to_gosa}) { &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]); }

	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( &create_xml_string($out_hash) );
}


################################
# @brief Returns a list of values for a given host. Values: priority, onceScript, licenseRequired, packageVersion, productVersion, advice, setupScript, windowsSoftwareIds, installationStatus, pxeConfigTemplate, name, creationTimestamp, alwaysScript, productId, description, properties, actionRequest, uninstallScript, action, updateScript and productClassNames 
# @param hostId Opsi hostId
sub opsi_get_full_product_host_information {
	my $startTime = Time::HiRes::time;
	my ($msg, $msg_hash, $session_id) = @_;
	my $header = @{$msg_hash->{'header'}}[0];
	my $source = @{$msg_hash->{'source'}}[0];
        my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
        my $hostId;

	my ($res, $err) = &_get_full_product_host_information( hostId=>@{$msg_hash->{'hostId'}}[0]);
	if ($err) {
		return &_giveErrorFeedback($msg_hash, "cannot fetch full_product_host_information from Opsi server : ".$res, $session_id);
	}

        # Build return message with twisted target and source
        my $out_hash = &main::create_xml_hash("answer_$header", $main::server_address, $source);
        if (defined $forward_to_gosa) {
            &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
        }
        &add_content2xml_hash($out_hash, "xxx", "");

        # Get hostId if defined
        if ((exists $msg_hash->{'hostId'}) && (@{$msg_hash->{'hostId'}} == 1))  {
            $hostId = @{$msg_hash->{'hostId'}}[0];
            &add_content2xml_hash($out_hash, "hostId", $hostId);
        }

        # Move to XML string
        my $xml_msg= &create_xml_string($out_hash);
        
        # Convert result in something usable
        my $replace= "";
	foreach my $product ( @$res) {

          # Open item
          $replace.= "<item>";

          # Add flat hash information
          my @entries= ( "priority", "onceScript", "licenseRequired", "packageVersion", "productVersion", "advice",
                              "setupScript", "windowsSoftwareIds", "installationStatus", "pxeConfigTemplate", "name", "type",
                              "creationTimestamp", "alwaysScript", "productId", "description", "actionRequest", "uninstallScript",
                              "action", "updateScript", "productClassNames");
          foreach my $entry (@entries) {
            if (defined $product->{$entry}) {
              my $value= $product->{$entry};

              if(ref($value) eq 'ARRAY'){
                my $tmp= "";
                foreach my $element (@$value) {
                  $tmp.= "<element>$element</element>";
                }
                $replace.= "<$entry>$tmp</$entry>";
              } else {
                $replace.= "<$entry>$value</$entry>";
              }
            }
          }

          # Add property information
          if (defined $product->{'properties'}) {
            $replace.= "<properties>";
            while ((my $key, my $value) = each(%{$product->{'properties'}})){
              $replace.= "<$key>";

              while ((my $pkey, my $pvalue) = each(%$value)){
                if(ref($pvalue) eq 'ARRAY'){
                  my $tmp= "";
                  foreach my $element (@$pvalue) {
                    $tmp.= "<element>$element</element>";
                  }
                  $replace.= "<$pkey>$tmp</$pkey>";
                } else {
                  $replace.= "<$pkey>$pvalue</$pkey>";
                }
              }
              $replace.= "</$key>";
            }
            $replace.= "</properties>";
          }

          # Close item
          $replace.= "</item>";
        }

        $xml_msg=~ s/<xxx><\/xxx>/\n$replace/;

	&main::daemon_log("0 DEBUG: time to process gosa-si message '$header' : ".sprintf("%.4f", (Time::HiRes::time - $startTime))." seconds", 1034);
    return ( $xml_msg );
}


sub opsi_test {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
	my $pram1 = @{$msg_hash->{'productId'}}[0];


	# Fetch infos from Opsi server
    my $callobj = {
        method  => 'getLicensePoolId',
        params  => [ $pram1 ],
        id  => 1,
    };
    my $res = $main::opsi_client->call($main::opsi_url, $callobj);

	return ();
}


# ----------------------------------------------------------------------------
#  internal methods handling the comunication with Opsi
# ----------------------------------------------------------------------------

################################
# @brief Checks if there is a specified tag and if the the tag has a content.
sub _check_xml_tag_is_ok {
	my ($msg_hash,$tag) = @_;
	if (not defined $msg_hash->{$tag}) {
		$_ = "message contains no tag '$tag'";
		return 0;
	}
	if (ref @{$msg_hash->{$tag}}[0] eq 'HASH') {
		$_ = "message tag '$tag' has no content";
		return  0;
	}
	return 1;
}

################################
# @brief Writes the log line and returns the error message for GOsa.
sub _giveErrorFeedback {
	my ($msg_hash, $err_string, $session_id) = @_;
	&main::daemon_log("$session_id ERROR: $err_string", 1);
	my $out_hash = &main::create_xml_hash("error", $main::server_address, @{$msg_hash->{source}}[0], $err_string);
    if (exists $msg_hash->{forward_to_gosa}) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]);
    }
	return ( &create_xml_string($out_hash) );
}


################################
# @brief Perform the call to the Opsi server and measure the time for the call
sub _callOpsi {
	my %arg = ('method'=>undef, 'params'=>[], 'id'=>1, @_);

	my $callObject = {
		method => $arg{method},
		params => $arg{params},
		id => $arg{id},
	};

	my $startTime = Time::HiRes::time;
	my $opsiResult = $opsi_client->call($opsi_url, $callObject);
	my $endTime = Time::HiRes::time;
	my $elapsedTime = sprintf("%.4f", ($endTime - $startTime));

	&main::daemon_log("0 DEBUG: time to process opsi call '$arg{method}' : $elapsedTime seconds", 1034); 

	return $opsiResult;
}

sub _getLicensePool_hash {
	my %arg = ( 'licensePoolId' => undef, @_ );

	if (not defined $arg{licensePoolId} ) { 
		return ("function requires licensePoolId as parameter", 1);
	}

	my $res = &_callOpsi( method  => 'getLicensePool_hash', params =>[$arg{licensePoolId}], id  => 1 );
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){ return ( (caller(0))[3]." : ".$res_error_str, 1 ); }

	return ($res->result, 0);
}

sub _getSoftwareLicenses_listOfHashes {
	
	my $res = &_callOpsi( method  => 'getSoftwareLicenses_listOfHashes' );
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){ return ( (caller(0))[3]." : ".$res_error_str, 1 ); }

	return ($res->result, 0);
}

sub _getSoftwareLicenseUsages_listOfHashes {
	my %arg = ( 'hostId' => "", 'licensePoolId' => "", @_ );

	my $res = &_callOpsi( method=>'getSoftwareLicenseUsages_listOfHashes', params=>[ $arg{hostId}, $arg{licensePoolId} ] );
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){ return ( (caller(0))[3]." : ".$res_error_str, 1 ); }

	return ($res->result, 0);
}

sub _removeSoftwareLicenseFromLicensePool {
	my %arg = ( 'softwareLicenseId' => undef, 'licensePoolId' => undef, @_ );

	if (not defined $arg{softwareLicenseId} ) { 
		return ("function requires softwareLicenseId as parameter", 1);
		}
		if (not defined $arg{licensePoolId} ) { 
		return ("function requires licensePoolId as parameter", 1);
	}

	my $res = &_callOpsi( method=>'removeSoftwareLicenseFromLicensePool', params=>[ $arg{softwareLicenseId}, $arg{licensePoolId} ] );
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){ return ( (caller(0))[3]." : ".$res_error_str, 1 ); }

	return ($res->result, 0);
}

sub _deleteSoftwareLicense {
	my %arg = ( 'softwareLicenseId' => undef, 'removeFromPools' => "false", @_ );

	if (not defined $arg{softwareLicenseId} ) { 
		return ("function requires softwareLicenseId as parameter", 1);
	}
	my $removeFromPools = "";
	if ((defined $arg{removeFromPools}) && ($arg{removeFromPools} eq "true")) { 
		$removeFromPools = "removeFromPools";
	}

	my $res = &_callOpsi( method=>'deleteSoftwareLicense', params=>[ $arg{softwareLicenseId}, $removeFromPools ] );
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){ return ( (caller(0))[3]." : ".$res_error_str, 1 ); }

	return ($res->result, 0);
}

sub _getLicensePoolId {
	my %arg = ( 'productId' => undef, @_ );
	
	if (not defined $arg{productId} ) {
		return ("function requires productId as parameter", 1);
	}

    my $res = &_callOpsi( method  => 'getLicensePoolId', params  => [ $arg{productId} ] );
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){ return ( (caller(0))[3]." : ".$res_error_str, 1 ); }

	return ($res->result, 0);
}

sub _getLicenseContract_hash {
	my %arg = ( 'licenseContractId' => undef, @_ );
	
	if (not defined $arg{licenseContractId} ) {
		return ("function requires licenseContractId as parameter", 1);
	}

    my $res = &_callOpsi( method  => 'getLicenseContract_hash', params  => [ $arg{licenseContractId} ] );
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){ return ( (caller(0))[3]." : ".$res_error_str, 1 ); }

	return ($res->result, 0);
}

sub _createLicenseContract {
	my %arg = (
			'licenseContractId' => undef,
			'partner' => undef,
			'conclusionDate' => undef,
			'notificationDate' => undef,
			'expirationDate' => undef,
			'notes' => undef,
			@_ );

	my $res = &_callOpsi( method  => 'createLicenseContract', 
			params  => [ $arg{licenseContractId}, $arg{partner}, $arg{conclusionDate}, $arg{notificationDate}, $arg{expirationDate}, $arg{notes} ],
			);
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){ return ( (caller(0))[3]." : ".$res_error_str, 1 ); }

	return ($res->result, 0);
}

sub _createSoftwareLicense {
	my %arg = (
			'softwareLicenseId' => undef,
			'licenseContractId' => undef,
			'licenseType' => undef,
			'maxInstallations' => undef,
			'boundToHost' => undef,
			'expirationDate' => undef,
			@_ );

    my $res = &_callOpsi( method  => 'createSoftwareLicense',
        params  => [ $arg{softwareLicenseId}, $arg{licenseContractId}, $arg{licenseType}, $arg{maxInstallations}, $arg{boundToHost}, $arg{expirationDate} ],
		);
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){ return ( (caller(0))[3]." : ".$res_error_str, 1 ); }

	return ($res->result, 0);
}

sub _addSoftwareLicenseToLicensePool {
	my %arg = (
            'softwareLicenseId' => undef,
            'licensePoolId' => undef,
            'licenseKey' => undef,
            @_ );

	if (not defined $arg{softwareLicenseId} ) {
		return ("function requires softwareLicenseId as parameter", 1);
	}
	if (not defined $arg{licensePoolId} ) {
		return ("function requires licensePoolId as parameter", 1);
	}

	my $res = &_callOpsi( method  => 'addSoftwareLicenseToLicensePool', params  => [ $arg{softwareLicenseId}, $arg{licensePoolId}, $arg{licenseKey} ] );
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){ return ( (caller(0))[3]." : ".$res_error_str, 1 ); }

	return ($res->result, 0);
}

sub _getProductStates_hash {
	my %arg = (	'hostId' => undef, @_ );

	if (not defined $arg{hostId} ) {
		return ("function requires hostId as parameter", 1);
	}

	my $res = &_callOpsi( method => 'getProductStates_hash', params => [$arg{hostId}]);
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){ return ( (caller(0))[3]." : ".$res_error_str, 1 ); }

	return ($res->result, 0);
}

sub _get_full_product_host_information {
	my %arg = ( 'hostId' => undef, @_ );

	my $res = &_callOpsi( method => 'getFullProductHostInformation_list',  params => [$arg{hostId}]);
	my ($res_error, $res_error_str) = &check_opsi_res($res);
	if ($res_error){ return ((caller(0))[3]." : ".$res_error_str, 1); }

	return ($res->result, 0);
}

1;
