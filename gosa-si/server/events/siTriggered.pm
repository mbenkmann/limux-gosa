package siTriggered;

use strict;
use warnings;

use Data::Dumper;
use GOsaSI::GosaSupportDaemon;

use Exporter;
use Socket;

our @ISA = qw(Exporter);

my @events = (
    "got_ping",
    "detected_hardware",
    "trigger_wake",
    "reload_ldap_config",
    "get_terminal_server",
    );
    
our @EXPORT = @events;

BEGIN {}

END {}

### Start ######################################################################

my $ldap_uri;
my $ldap_base;
my $ldap_admin_dn;
my $ldap_admin_password;
my $mesg;

my %cfg_defaults = (
    "Server" => {
        "ldap-uri" => [\$ldap_uri, ""],
        "ldap-base" => [\$ldap_base, ""],
        "ldap-admin-dn" => [\$ldap_admin_dn, ""],
        "ldap-admin-password" => [\$ldap_admin_password, ""],
    },
);
# why not using it from main::read_configfile
&GOsaSI::GosaSupportDaemon::read_configfile($main::cfg_file, %cfg_defaults);


sub get_terminal_server
{
	my ($msg, $msg_hash, $session_id) = @_ ;
	my $source = @{$msg_hash->{source}}[0];
	my @out_msg_l;

	# Send get_load message to all si-clients at terminal server specified in LDAP
	my $ldap_handle = &main::get_ldap_handle();
	if (defined $ldap_handle) 
	{
		my $ldap_mesg = $ldap_handle->search(
				base => $ldap_base,
				scope => 'sub',
				attrs => ['macAddress', 'cn', 'ipHostNumber'],
				filter => "objectClass=goTerminalServer",
				);
		if ($ldap_mesg->count) 
		{	
			# Parse all LDAP results to a sql compliant where statement
			my @entries = $ldap_mesg->entries;
			@entries = map ($_->get_value("macAddress"), @entries);
			@entries = map ("macaddress LIKE '$_'", @entries);

			my ($hit, $hash, $db_res, $out_msg);
			# Check known clients if a terminal server is active
			$db_res = $main::known_clients_db->select_dbentry("SELECT * FROM $main::known_clients_tn WHERE ".join(" AND ", @entries));
			while (($hit, $hash) = each %$db_res) 
			{
				$out_msg = &create_xml_string(&create_xml_hash('get_load', $source, $hash->{macaddress}));
				push(@out_msg_l, $out_msg);
			}
			# Check foreign_clients if a terminal server is active
			$db_res = $main::foreign_clients_db->select_dbentry("SELECT * FROM $main::foreign_clients_tn WHERE ".join(" AND ", @entries));
			while (($hit, $hash) = each %$db_res) 
			{
				$out_msg = &create_xml_string(&create_xml_hash('get_load', $source, $hash->{macaddress}));
				push(@out_msg_l, $out_msg);
			}

### JUST FOR DEBUGGING # CAN BE DELETED AT ANY TIME ###########################
#			my $db_res = $main::known_clients_db->select_dbentry("SELECT * FROM $main::known_clients_tn WHERE macaddress LIKE '00:01:6c:9d:b9:fa'");
#			while (($hit, $hash) = each %$db_res) 
#			{
#				$out_msg = &create_xml_string(&create_xml_hash('get_load', $source, $hash->{macaddress}));
#				push(@out_msg_l, $out_msg);
#			}
### JUST FOR DEBUGGING # CAN BE DELETED AT ANY TIME ###########################

			# Found terminal server but no running clients on them
			if (@out_msg_l == 0) 
			{
				&main::daemon_log("$session_id ERROR: Found no running clients (known_clients_db, foreign_clients_db) on the following determined terminal server", 1);
				my @entries = $ldap_mesg->entries;
				foreach my $ts (@entries) 
				{
					my $ip = (defined $ts->get_value("ipHostNumber")) ? "   ip='".$ts->get_value("ipHostNumber")."'" : "" ;
					my $cn = (defined $ts->get_value("cn")) ? "   cn='".$ts->get_value("cn")."'" : "" ;
					my $mac = (defined $ts->get_value("macAddress")) ? "   macAddress='".$ts->get_value("macAddress")."'" : "" ;
					&main::daemon_log("$session_id ERROR: ".$cn.$mac.$ip , 1);
				}
			}

		}
		# No terminal server found in LDAP
		if ($ldap_mesg->count == 0) 
		{
			&main::daemon_log("$session_id ERROR: No terminal server found in LDAP: \n\tbase='$ldap_base'\n\tscope='sub'\n\tattrs='['macAddress']'\n\tfilter='objectClass=goTerminalServer'", 1);
		}

		# Translating errors ?
		if ($ldap_mesg->code) 
		{
			&main::daemon_log("$session_id ERROR: Cannot fetch terminal server from LDAP: \n\tbase='$ldap_base'\n\tscope='sub'\n\tattrs='['macAddress']'\n\tfilter='objectClass=goTerminalServer'", 1);
		}
	}
	&main::release_ldap_handle($ldap_handle);

    return @out_msg_l;
}


sub get_events {
    return \@events;
}

sub reload_ldap_config {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{header}}[0];
    my $target = @{$msg_hash->{$header}}[0];

    my $out_msg = &ClientPackages::new_ldap_config($target, $session_id);
    my @out_msg_l = ( $out_msg );
    return @out_msg_l;
}


sub got_ping {
    my ($msg, $msg_hash, $session_id) = @_;

    my $source = @{$msg_hash->{source}}[0];
    my $target = @{$msg_hash->{target}}[0];
    my $header = @{$msg_hash->{header}}[0];
    my $act_time = &get_time;
    my @out_msg_l;
    my $out_msg;

    $session_id = @{$msg_hash->{'session_id'}}[0];

    # check known_clients_db
    my $sql_statement = "SELECT * FROM known_clients WHERE hostname='$source'";
    my $query_res = $main::known_clients_db->select_dbentry( $sql_statement );
    if( 1 == keys %{$query_res} ) {
         my $sql_statement= "UPDATE known_clients ".
            "SET status='$header', timestamp='$act_time' ".
            "WHERE hostname='$source'";
         my $res = $main::known_clients_db->update_dbentry( $sql_statement );
    } 
    
    # check known_server_db
    $sql_statement = "SELECT * FROM known_server WHERE hostname='$source'";
    $query_res = $main::known_server_db->select_dbentry( $sql_statement );
    if( 1 == keys %{$query_res} ) {
         my $sql_statement= "UPDATE known_server ".
            "SET status='$header', timestamp='$act_time' ".
            "WHERE hostname='$source'";
         my $res = $main::known_server_db->update_dbentry( $sql_statement );
    } 

    # create out_msg
    my $out_hash = &create_xml_hash($header, $source, "GOSA");
    &add_content2xml_hash($out_hash, "session_id", $session_id);
    $out_msg = &create_xml_string($out_hash);
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        $out_msg =~s/<\/xml>/<forward_to_gosa>$forward_to_gosa<\/forward_to_gosa><\/xml>/;
    }
    push(@out_msg_l, $out_msg);
    
    return @out_msg_l;
}


sub detected_hardware {
	my ($msg, $msg_hash, $session_id) = @_;
	my $address = $msg_hash->{source}[0];
	my $header = $msg_hash->{header}[0];
	my $gotoHardwareChecksum= $msg_hash->{detected_hardware}[0]->{gotoHardwareChecksum};

	my $sql_statement= "SELECT * FROM known_clients WHERE hostname='$address'";
	my $res = $main::known_clients_db->select_dbentry( $sql_statement );

	# check hit
	my $hit_counter = keys %{$res};
	if( not $hit_counter == 1 ) {
		&main::daemon_log("$session_id ERROR: more or no hit found in known_clients_db by query by '$address'", 1);
		return;
	}

	my $macaddress = $res->{1}->{macaddress};
	my $hostkey = $res->{1}->{hostkey};

	if (not defined $macaddress) {
		&main::daemon_log("$session_id ERROR: no mac address found for client $address", 1);
		return;
	}

	# Perform search
	my $ldap_handle = &main::get_ldap_handle();
	$mesg = $ldap_handle->search(
		base   => $ldap_base,
		scope  => 'sub',
		filter => "(&(objectClass=GOhard)(|(macAddress=$macaddress)(dhcpHWaddress=ethernet $macaddress)))"
	);

	# We need to create a base entry first (if not done from ArpHandler)
	if($mesg->count == 0) {
		&main::daemon_log("INFO: Need to create a new LDAP Entry for client $address", 4);
		my $ipaddress= $1 if $address =~ /^([0-9\.]*?):.*$/;
		my $dnsname;
		#FIXME: like in ClientPackages!
		#if ( defined($heap->{force-hostname}->{$macaddress}) ){
		#	$dnsname= $heap->{force-hostname}->{$macaddress};
		#	&main::daemon_log("INFO: Using forced hostname $dnsname for client $address", 4);
		if (-e "/var/tmp/$macaddress" ){
			open(my $TFILE, "<", "/var/tmp/$macaddress");
			$dnsname= <$TFILE>;
			close($TFILE);
		} else {
			$dnsname= gethostbyaddr(inet_aton($ipaddress), AF_INET) || $ipaddress;
		}

		my $cn = (($dnsname =~ /^(\d){1,3}\.(\d){1,3}\.(\d){1,3}\.(\d){1,3}/) ? $dnsname : sprintf "%s", $dnsname =~ /([^\.]+)\.?/);
		my $dn = "cn=$cn,ou=incoming,$ldap_base";
		&main::daemon_log("INFO: Creating entry for $dn",5);
		my $entry= Net::LDAP::Entry->new( $dn );
		$entry->dn($dn);
		$entry->add("objectClass" => "GOhard");
		$entry->add("cn" => $cn);
		$entry->add("macAddress" => $macaddress);
		$entry->add("gotomode" => "locked");
		$entry->add("gotoSysStatus" => "new-system");
		$entry->add("ipHostNumber" => $ipaddress);
		if(defined($main::gosa_unit_tag) && length($main::gosa_unit_tag) > 0) {
			$entry->add("objectClass" => "gosaAdministrativeUnitTag");
			$entry->add("gosaUnitTag" => $main::gosa_unit_tag);
		}
		my $res=$entry->update($ldap_handle);
		if(defined($res->{'errorMessage'}) &&
			length($res->{'errorMessage'}) >0) {
			&main::daemon_log("ERROR: can not add entries to LDAP: ".$res->{'errorMessage'}, 1);
			&main::release_ldap_handle($ldap_handle);
			return;
		} else {
			# Fill $mesg again
			$mesg = $ldap_handle->search(
				base   => $ldap_base,
				scope  => 'sub',
				filter => "(&(objectClass=GOhard)(|(macAddress=$macaddress)(dhcpHWaddress=ethernet $macaddress)))"
			);
		}
	}

	if($mesg->count == 1) {
		my $entry= $mesg->entry(0);
		$entry->changetype("modify");
		foreach my $attribute (
			"gotoSndModule", "ghNetNic", "gotoXResolution", "ghSoundAdapter", "ghCpuType", "gotoXkbModel", 
			"ghGfxAdapter", "gotoXMousePort", "ghMemSize", "gotoXMouseType", "ghUsbSupport", "gotoXHsync", 
			"gotoXDriver", "gotoXVsync", "gotoXMonitor", "gotoHardwareChecksum") {
			if(defined($msg_hash->{detected_hardware}[0]->{$attribute}) &&
				length($msg_hash->{detected_hardware}[0]->{$attribute}) >0 ) {
				if(defined($entry->get_value($attribute))) {
					$entry->delete($attribute => []);
				}
				&main::daemon_log("INFO: Adding attribute $attribute with value ".$msg_hash->{detected_hardware}[0]->{$attribute},5);
				$entry->add($attribute => $msg_hash->{detected_hardware}[0]->{$attribute});	
			}
		}
		foreach my $attribute (
			"gotoModules", "ghScsiDev", "ghIdeDev") {
			if(defined($msg_hash->{detected_hardware}[0]->{$attribute}) &&
				length($msg_hash->{detected_hardware}[0]->{$attribute}) >0 ) {
				if(defined($entry->get_value($attribute))) {
					$entry->delete($attribute => []);
				}
				foreach my $array_entry (keys %{{map { $_ => 1 } sort(@{$msg_hash->{detected_hardware}[0]->{$attribute}}) }}) {
					$entry->add($attribute => $array_entry);
				}
			}
		}

		my $res=$entry->update($ldap_handle);
		if(defined($res->{'errorMessage'}) &&
			length($res->{'errorMessage'}) >0) {
			&main::daemon_log("ERROR: can not add entries to LDAP: ".$res->{'errorMessage'}, 1);
		} else {
			&main::daemon_log("INFO: Added Hardware configuration to LDAP", 5);
		}
	}

	# if there is a job in job queue for this host and this macaddress, delete it, cause its no longer used
	my $del_sql = "DELETE FROM $main::job_queue_tn WHERE (macaddress LIKE '$macaddress' AND headertag='$header')";
	my $del_res = $main::job_db->exec_statement($del_sql);
  &main::release_ldap_handle($ldap_handle);

	return ;
}

1;
