package ClientPackages;

# Each module has to have a function 'process_incoming_msg'. This function works as a interface to gosa-sd and receives the msg hash from gosa-sd. 'process_incoming_function checks, wether it has a function to process the incoming msg and forward the msg to it. 

use strict;
use warnings;

use IO::Socket::INET;
use XML::Simple;
use Data::Dumper;
use NetAddr::IP;
use Net::LDAP;
use Net::LDAP::Util;
use Socket;
use Net::hostent;
use GOsaSI::GosaSupportDaemon;

use Exporter;

our @ISA = ("Exporter");

my $event_dir = "/usr/lib/gosa-si/server/ClientPackages";
use lib "/usr/lib/gosa-si/server/ClientPackages";

BEGIN{}
END {}

my ($server_ip, $server_port, $ClientPackages_key, $max_clients, $ldap_uri, $ldap_base, $ldap_admin_dn, $ldap_admin_password, $server_interface);
my $server;
my $network_interface;
my (@ldap_cfg, @pam_cfg, @nss_cfg, $goto_admin, $goto_secret);
my $mesg;

my %cfg_defaults = (
"Server" => {
    "ip" => [\$server_ip, "0.0.0.0"],
    "mac-address" => [\$main::server_mac_address, "00:00:00:00:00"],
    "port" => [\$server_port, "20081"],
    "ldap-uri" => [\$ldap_uri, ""],
    "ldap-base" => [\$ldap_base, ""],
    "ldap-admin-dn" => [\$ldap_admin_dn, ""],
    "ldap-admin-password" => [\$ldap_admin_password, ""],
    "max-clients" => [\$max_clients, 100],
    },
"ClientPackages" => {
    "key" => [\$ClientPackages_key, ""],
    },
);

### START #####################################################################

# read configfile and import variables
#why not using the main::read_configfile !!
&local_read_configfile();


# if server_ip is not an ip address but a name
if( inet_aton($server_ip) ){ $server_ip = inet_ntoa(inet_aton($server_ip)); } 
$network_interface= &get_interface_for_ip($server_ip);
$main::server_mac_address= &get_mac($network_interface);


# import local events
my ($error, $result, $event_hash) = &import_events($event_dir);

foreach my $log_line (@$result) {
    if ($log_line =~ / succeed: /) {
        &main::daemon_log("0 INFO: ClientPackages - $log_line", 5);
    } else {
        &main::daemon_log("0 ERROR: ClientPackages - $log_line", 1);
    }
}
# build vice versa event_hash, event_name => module
my $event2module_hash = {};
while (my ($module, $mod_events) = each %$event_hash) {
    while (my ($event_name, $nothing) = each %$mod_events) {
        $event2module_hash->{$event_name} = $module;
    }

}

# Unit tag can be defined in config
if((not defined($main::gosa_unit_tag)) || length($main::gosa_unit_tag) == 0) {
	# Read gosaUnitTag from LDAP
        
    my $ldap_handle = &main::get_ldap_handle(); 
    if( defined($ldap_handle) ) {
		&main::daemon_log("0 INFO: Searching for servers gosaUnitTag with mac address $main::server_mac_address",5);
		# Perform search for Unit Tag
		$mesg = $ldap_handle->search(
			base   => $ldap_base,
			scope  => 'sub',
			attrs  => ['gosaUnitTag'],
			filter => "(macaddress=$main::server_mac_address)"
		);

		if ((! $main::server_mac_address eq "00:00:00:00:00:00") and $mesg->count == 1) {
			my $entry= $mesg->entry(0);
			my $unit_tag= $entry->get_value("gosaUnitTag");
			$main::ldap_server_dn= $mesg->entry(0)->dn;
			if(defined($unit_tag) && length($unit_tag) > 0) {
				&main::daemon_log("0 INFO: Detected gosaUnitTag $unit_tag for creating entries", 5);
				$main::gosa_unit_tag= $unit_tag;
			}
		} else {
			# Perform another search for Unit Tag
			my $hostname= `hostname -f`;
			chomp($hostname);
			&main::daemon_log("0 INFO: Searching for servers gosaUnitTag with hostname $hostname",5);
			$mesg = $ldap_handle->search(
				base   => $ldap_base,
				scope  => 'sub',
				attrs  => ['gosaUnitTag'],
				filter => "(&(cn=$hostname)(objectClass=goServer))"
			);
			if ($mesg->count == 1) {
				my $entry= $mesg->entry(0);
				my $unit_tag= $entry->get_value("gosaUnitTag");
			        $main::ldap_server_dn= $mesg->entry(0)->dn;
				if(defined($unit_tag) && length($unit_tag) > 0) {
					&main::daemon_log("0 INFO: Detected gosaUnitTag $unit_tag for creating entries", 5);
					$main::gosa_unit_tag= $unit_tag;
				}
			} else {
				# Perform another search for Unit Tag
				$hostname= `hostname -s`;
				chomp($hostname);
				&main::daemon_log("0 INFO: Searching for servers gosaUnitTag with hostname $hostname",5);
				$mesg = $ldap_handle->search(
					base   => $ldap_base,
					scope  => 'sub',
					attrs  => ['gosaUnitTag'],
					filter => "(&(cn=$hostname)(objectClass=goServer))"
				);
				if ($mesg->count == 1) {
					my $entry= $mesg->entry(0);
					my $unit_tag= $entry->get_value("gosaUnitTag");
			        	$main::ldap_server_dn= $mesg->entry(0)->dn;
					if(defined($unit_tag) && length($unit_tag) > 0) {
						&main::daemon_log("INFO: Detected gosaUnitTag $unit_tag for creating entries", 5);
						$main::gosa_unit_tag= $unit_tag;
					}
				} else {
					&main::daemon_log("0 WARNING: No gosaUnitTag detected. Not using gosaUnitTag", 3);
				}
			}
		}
	} else {
		&main::daemon_log("0 INFO: Using gosaUnitTag from config-file: $main::gosa_unit_tag",5);
	}
    &main::release_ldap_handle($ldap_handle);
}


my $server_address = "$server_ip:$server_port";
$main::server_address = $server_address;

{
  # Check if ou=incoming exists
  # TODO: This should be transferred to a module init-function
  my $ldap_handle = &main::get_ldap_handle();
  if( defined($ldap_handle) ) {
    &main::daemon_log("0 INFO: Searching for ou=incoming container for new clients", 5);
    # Perform search
    my $mesg = $ldap_handle->search(
      base   => $ldap_base,
      scope  => 'one',
      filter => "(&(ou=incoming)(objectClass=organizationalUnit))"
    );
    if(not defined($mesg->count) or $mesg->count == 0) {
            my $incomingou = Net::LDAP::Entry->new();
            $incomingou->dn('ou=incoming,'.$ldap_base);
            $incomingou->add('objectClass' => 'organizationalUnit');
            $incomingou->add('ou' => 'incoming');
            my $result = $incomingou->update($ldap_handle);
            if($result->code != 0) {
                &main::daemon_log("0 ERROR: Problem adding ou=incoming: '".$result->error()."'!", 1);
            }
    }
  }
  &main::release_ldap_handle($ldap_handle);
}


### functions #################################################################


sub get_module_info {
    my @info = ($server_address,
                $ClientPackages_key,
                $event_hash,
                );
    return \@info;
}


#===  FUNCTION  ================================================================
#         NAME:  local_read_configfile
#   PARAMETERS:  cfg_file - string -
#      RETURNS:  nothing
#  DESCRIPTION:  read cfg_file and set variables
#===============================================================================
sub local_read_configfile {
    my $cfg;
    if( defined( $main::cfg_file) && ( (-s $main::cfg_file) > 0 )) {
        if( -r $main::cfg_file ) {
            $cfg = Config::IniFiles->new( -file => $main::cfg_file );
        } else {
            print STDERR "Couldn't read config file!";
        }
    } else {
        $cfg = Config::IniFiles->new() ;
    }
    foreach my $section (keys %cfg_defaults) {
        foreach my $param (keys %{$cfg_defaults{ $section }}) {
            my $pinfo = $cfg_defaults{ $section }{ $param };
            ${@$pinfo[0]} = $cfg->val( $section, $param, @$pinfo[1] );
        }
    }

    # Read non predefined sections
    my $param;
    if ($cfg->SectionExists('ldap')){
		foreach $param ($cfg->Parameters('ldap')){
			push (@ldap_cfg, "$param ".$cfg->val('ldap', $param));
		}
    }
    if ($cfg->SectionExists('pam_ldap')){
		foreach $param ($cfg->Parameters('pam_ldap')){
			push (@pam_cfg, "$param ".$cfg->val('pam_ldap', $param));
		}
    }
    if ($cfg->SectionExists('nss_ldap')){
		foreach $param ($cfg->Parameters('nss_ldap')){
			push (@nss_cfg, "$param ".$cfg->val('nss_ldap', $param));
		}
    }
    if ($cfg->SectionExists('goto')){
    	$goto_admin= $cfg->val('goto', 'terminal_admin');
    	$goto_secret= $cfg->val('goto', 'terminal_secret');
    } else {
    	$goto_admin= undef;
    	$goto_secret= undef;
    }

}


#===  FUNCTION  ================================================================
#         NAME:  get_mac 
#   PARAMETERS:  interface name (i.e. eth0)
#      RETURNS:  (mac address) 
#  DESCRIPTION:  Uses ioctl to get mac address directly from system.
#===============================================================================
sub get_mac {
	my $ifreq= shift;
	my $result;
	if ($ifreq && length($ifreq) > 0) { 
		if($ifreq eq "all") {
			$result = "00:00:00:00:00:00";
		} else {
			my $SIOCGIFHWADDR= 0x8927;     # man 2 ioctl_list

			# A configured MAC Address should always override a guessed value
			if ($main::server_mac_address and length($main::server_mac_address) > 0) {
				$result= $main::server_mac_address;
			}

			socket SOCKET, PF_INET, SOCK_DGRAM, getprotobyname('ip')
				or die "socket: $!";

			if(ioctl SOCKET, $SIOCGIFHWADDR, $ifreq) {
				my ($if, $mac)= unpack 'h36 H12', $ifreq;

				if (length($mac) > 0) {
					$mac=~ m/^([0-9a-f][0-9a-f])([0-9a-f][0-9a-f])([0-9a-f][0-9a-f])([0-9a-f][0-9a-f])([0-9a-f][0-9a-f])([0-9a-f][0-9a-f])$/;
					$mac= sprintf("%s:%s:%s:%s:%s:%s", $1, $2, $3, $4, $5, $6);
					$result = $mac;
				}
			}
		}
	}
	return $result;
}


#===  FUNCTION  ================================================================
#         NAME:  process_incoming_msg
#   PARAMETERS:  crypted_msg - string - incoming crypted message
#      RETURNS:  nothing
#  DESCRIPTION:  handels the proceeded distribution to the appropriated functions
#===============================================================================
sub process_incoming_msg {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $error = 0;
    my $host_name;
    my $host_key;
    my @out_msg_l = ("nohandler");
    my $sql_events;

    # process incoming msg
    my $header = @{$msg_hash->{header}}[0]; 
    my $source = @{$msg_hash->{source}}[0]; 
    my @target_l = @{$msg_hash->{target}};

    # skip PREFIX
    $header =~ s/^CLMSG_//;

    &main::daemon_log("$session_id DEBUG: ClientPackages: msg to process: $header", 26);

    if( 0 == length @target_l){     
        &main::daemon_log("$session_id ERROR: no target specified for msg $header", 1);
        $error++;
    } elsif( 1 == length @target_l) {
        my $target = $target_l[0];
		if(&server_matches($target, $session_id)) {
            if ($header eq 'new_key') {
                @out_msg_l = &new_key($msg_hash)
            } elsif ($header eq 'here_i_am') {
                @out_msg_l = &here_i_am($msg, $msg_hash, $session_id)
            } else {
                # a event exists with the header as name
                if( exists $event2module_hash->{$header} ) {
                    &main::daemon_log("$session_id DEBUG: found event '$header' at event-module '".$event2module_hash->{$header}."'", 26);
                    no strict 'refs';
                    @out_msg_l = &{$event2module_hash->{$header}."::$header"}($msg, $msg_hash, $session_id);

                # if no event handler is implemented   
                } else {
                    $sql_events = "SELECT * FROM $main::known_clients_tn WHERE ( (macaddress LIKE '$source') OR (hostname='$source') )"; 
                    my $res = $main::known_clients_db->select_dbentry( $sql_events );
                    my $l = keys(%$res);

                    # set error if no or more than 1 hits are found for sql query
                    if ( $l != 1) {
                        @out_msg_l = ('knownclienterror');

                    # found exact 1 hit in db
                    } else {
                        my $client_events = $res->{'1'}->{'events'};

                        # client is registered for this event, deliver this message to client
                        $header =~ s/^answer_//;
                        if ($client_events =~ /,$header,/) {
                            # answer message finally arrived destination server, so forward messages to GOsa
                            if ($target eq $main::server_address) {        
                                $msg =~ s/<header>answer_/<header>/;
                                $msg =~ s/<target>\S+<\/target>/<target>GOSA<\/target>/;
                            }
                            @out_msg_l = ( $msg );

                        # client is not registered for this event, set error
                        } else {
                            @out_msg_l = ('noeventerror');
                        }
                    }
                }
            }

            # if delivery not possible raise error and return 
            if( not defined $out_msg_l[0] ) {
                @out_msg_l = ();
            } elsif( $out_msg_l[0] eq 'nohandler') {
                &main::daemon_log("$session_id ERROR: ClientPackages: no event handler or core function defined for '$header'", 1);
                @out_msg_l = ();
            }  elsif ($out_msg_l[0] eq 'knownclienterror') {
                &main::daemon_log("$session_id ERROR: no or more than 1 hits are found at known_clients_db with sql query: '$sql_events'", 1);
                &main::daemon_log("$session_id ERROR: processing is aborted and message will not be forwarded", 1);
                @out_msg_l = ();
            } elsif ($out_msg_l[0] eq 'noeventerror') {
                &main::daemon_log("$session_id ERROR: client '$target' is not registered for event '$header', processing is aborted", 1); 
                @out_msg_l = ();
            }
        } else {
			&main::daemon_log("DEBUG: msg is not for gosa-si-server '$server_address', deliver it to target '$target'", 26);
			push(@out_msg_l, $msg);
		}
    }

    return \@out_msg_l;
}


#===  FUNCTION  ================================================================
#         NAME:  new_passwd
#   PARAMETERS:  msg_hash - ref - hash from function create_xml_hash
#      RETURNS:  nothing
#  DESCRIPTION:  process this incoming message
#===============================================================================
sub new_key {
    my ($msg_hash) = @_;
    my @out_msg_l;
    
    my $header = @{$msg_hash->{header}}[0];
    my $source_name = @{$msg_hash->{source}}[0];
    my $source_key = @{$msg_hash->{new_key}}[0];
    my $query_res;

    # check known_clients_db
    my $sql_statement = "SELECT * FROM known_clients WHERE hostname='$source_name'";
    $query_res = $main::known_clients_db->select_dbentry( $sql_statement );
    if( 1 == keys %{$query_res} ) {
        my $act_time = &get_time;
        my $sql_statement= "UPDATE known_clients ".
            "SET hostkey='$source_key', timestamp='$act_time' ".
            "WHERE hostname='$source_name'";
        my $res = $main::known_clients_db->update_dbentry( $sql_statement );
        my $hash = &create_xml_hash("confirm_new_key", $server_address, $source_name);
        my $out_msg = &create_xml_string($hash);
        push(@out_msg_l, $out_msg);
    }

    # only do if host still not found
    if( 0 == @out_msg_l ) {
        # check known_server_db
        $sql_statement = "SELECT * FROM known_server WHERE hostname='$source_name'";
        $query_res = $main::known_server_db->select_dbentry( $sql_statement );
        if( 1 == keys %{$query_res} ) {
            my $act_time = &get_time;
            my $sql_statement= "UPDATE known_server ".
                "SET hostkey='$source_key', timestamp='$act_time' ".
                "WHERE hostname='$source_name'";
            my $res = $main::known_server_db->update_dbentry( $sql_statement );

            my $hash = &create_xml_hash("confirm_new_key", $server_address, $source_name);
            my $out_msg = &create_xml_string($hash);
            push(@out_msg_l, $out_msg);
        }
    }

    return @out_msg_l;
}


#===  FUNCTION  ================================================================
#         NAME:  here_i_am
#   PARAMETERS:  msg_hash - hash - hash from function create_xml_hash
#      RETURNS:  nothing
#  DESCRIPTION:  process this incoming message
#===============================================================================
sub here_i_am {
    my ($msg, $msg_hash, $session_id) = @_;
    my @out_msg_l;
    my $out_hash;
    my $source = @{$msg_hash->{source}}[0];
    my $mac_address = @{$msg_hash->{mac_address}}[0];
	my $gotoHardwareChecksum = @{$msg_hash->{gotoHardwareChecksum}}[0];
    my $client_status = @{$msg_hash->{client_status}}[0];
    my $client_revision = @{$msg_hash->{client_revision}}[0];
    my $key_lifetime = @{$msg_hash->{key_lifetime}}[0];

    # Move forced hostname to heap - if used
    #FIXME: move to some global POE namespace - please
    if ( defined($msg_hash->{'force-hostname'}[0]) &&
       length($msg_hash->{'force-hostname'}[0]) > 0){
    #      $heap->{force-hostname}->{$mac_address}= $msg_hash->{'force-hostname'}[0];
	    open (my $TFILE, ">", "/var/tmp/$mac_address");
	    print $TFILE $msg_hash->{'force-hostname'}[0];
	    close ($TFILE); 
    } else {
    #      $heap->{force-hostname}->{$mac_address}= undef;
	if ( -e "/var/tmp/$mac_address") {
		unlink("/var/tmp/$mac_address")
	}; 
    }

    # number of known clients
    my $nu_clients= $main::known_clients_db->count_dbentries('known_clients');

    # check wether client address or mac address is already known
    my $sql_statement= "SELECT * FROM known_clients WHERE hostname='$source'";
    my $db_res= $main::known_clients_db->select_dbentry( $sql_statement );
    
    if ( 1 == keys %{$db_res} ) {
        &main::daemon_log("$session_id WARNING: $source is already known as a client", 3);
        &main::daemon_log("$session_id WARNING: values for $source are being overwritten", 3);   
        $nu_clients --;
    }

    # number of current active clients
    my $act_nu_clients = $nu_clients;

    &main::daemon_log("$session_id DEBUG: number of current active clients: $act_nu_clients", 26);
    &main::daemon_log("$session_id DEBUG: number of maximal allowed clients: $max_clients", 26);

    if($max_clients <= $act_nu_clients) {
        my $out_hash = &create_xml_hash("denied", $server_address, $source);
        &add_content2xml_hash($out_hash, "denied", "I_cannot_take_any_more_clients!");
        my $passwd = @{$msg_hash->{new_passwd}}[0]; 
        &send_msg_hash2address($out_hash, $source, $passwd);
        return;
    }
    
    # new client accepted
    my $new_passwd = @{$msg_hash->{new_passwd}}[0];

    # add entry to known_clients_db
    my $events = @{$msg_hash->{events}}[0];
    my $act_timestamp = &get_time;
    my $res = $main::known_clients_db->add_dbentry( {table=>'known_clients', 
                                                primkey=>['hostname'],
                                                hostname=>$source,
                                                events=>$events,
                                                macaddress=>$mac_address,
                                                status=>'registered',
                                                hostkey=>$new_passwd,
                                                timestamp=>$act_timestamp,
                                                keylifetime=>$key_lifetime,
                                                } );

    if ($res != 0)  {
        &main::daemon_log("$session_id ERROR: cannot add entry to known_clients: $res",1);
        return;
    }
    
    # return acknowledgement to client
    $out_hash = &create_xml_hash("registered", $server_address, $source);

    # give the new client his ldap config
    # Workaround: Send within the registration response, if the client will get an ldap config later
	my $new_ldap_config_out = &new_ldap_config($source, $session_id);
	if($new_ldap_config_out && (!($new_ldap_config_out =~ /error/))) {
		&add_content2xml_hash($out_hash, "ldap_available", "true");
	} elsif($new_ldap_config_out && $new_ldap_config_out =~ /error/){
		&add_content2xml_hash($out_hash, "error", $new_ldap_config_out);

		my $sql_statement = "UPDATE $main::job_queue_tn ".
		"SET status='error', result='$new_ldap_config_out' ".
		"WHERE status='processing' AND macaddress LIKE '$mac_address'";
		my $res = $main::job_db->update_dbentry($sql_statement);
		&main::daemon_log("$session_id DEBUG: $sql_statement RESULT: $res", 26);         
	}
    my $register_out = &create_xml_string($out_hash);
    push(@out_msg_l, $register_out);

    # Really send the ldap config
    if( $new_ldap_config_out && (!($new_ldap_config_out =~ /error/))) {
            push(@out_msg_l, $new_ldap_config_out);
    }

    # Send client hardware configuration
	my $hardware_config_out = &hardware_config($msg, $msg_hash, $session_id);
	if( $hardware_config_out ) {
		push(@out_msg_l, $hardware_config_out);
	}

    # Send client ntp server
    my $ntp_config_out = &new_ntp_config($mac_address, $session_id);
    if ($ntp_config_out) {
        push(@out_msg_l, $ntp_config_out);
    }

    # Send client syslog server
    my $syslog_config_out = &new_syslog_config($mac_address, $session_id);
    if ($syslog_config_out) {
        push(@out_msg_l, $syslog_config_out);
    }

    # update ldap entry if exists
    my $ldap_handle= &main::get_ldap_handle();
    my $ldap_res= $ldap_handle->search(
                base   => $ldap_base,
                scope  => 'sub',
                #attrs => ['ipHostNumber'],
                filter => "(&(objectClass=GOhard)(macAddress=$mac_address))");
    if($ldap_res->code) {
            &main::daemon_log("$session_id ERROR: LDAP Entry for client with mac address $mac_address not found: ".$ldap_res->error, 1);
    } elsif ($ldap_res->count != 1) {
            &main::daemon_log("$session_id WARNING: client with mac address $mac_address not found/unique/active - not updating ldap entry".
                            "\n\tbase: $ldap_base".
                            "\n\tscope: sub".
                            "\n\tattrs: ipHostNumber".
                            "\n\tfilter: (&(objectClass=GOhard)(macaddress=$mac_address))", 1);
    } else {
            my $entry= $ldap_res->pop_entry();
            my $ip_address= $entry->get_value('ipHostNumber');
            my $source_ip= ($1) if $source =~ /^([0-9\.]*?):[0-9]*$/;
            if(not defined($ip_address) and defined($source_ip)) {
                $entry->add( 'ipHostNumber' => $source_ip );
                my $mesg= $entry->update($ldap_handle);
                $mesg->code && &main::daemon_log("$session_id ERROR: Updating IP Address for client with mac address $mac_address failed with '".$mesg->mesg()."'", 1);
            } elsif(defined($source_ip) and not ($source_ip eq $ip_address)) {
                $entry->replace( 'ipHostNumber' => $source_ip );
                my $mesg= $entry->update($ldap_handle);
                $mesg->code && &main::daemon_log("$session_id ERROR: Updating IP Address for client with mac address $mac_address failed with '".$mesg->mesg()."'", 1);
            } elsif (not defined($source_ip)) {
                &main::daemon_log("ERROR: Could not parse source value '$source' perhaps not an ip address?", 1);
            }
    }
    &main::release_ldap_handle($ldap_handle);

    # notify registered client to all other server
    my %mydata = ( 'client' => $source, 'macaddress' => $mac_address);
    my $mymsg = &build_msg('new_foreign_client', $main::server_address, "KNOWN_SERVER", \%mydata);
    push(@out_msg_l, $mymsg);

    &main::daemon_log("$session_id INFO: register client $source ($mac_address), $client_status - $client_revision", 5);
    return @out_msg_l;
}


#===  FUNCTION  ================================================================
#         NAME:  who_has
#   PARAMETERS:  msg_hash - hash - hash from function create_xml_hash
#      RETURNS:  nothing 
#  DESCRIPTION:  process this incoming message
#===============================================================================
sub who_has {
    my ($msg_hash) = @_ ;
    my @out_msg_l;
    
    # what is your search pattern
    my $search_pattern = @{$msg_hash->{who_has}}[0];
    my $search_element = @{$msg_hash->{$search_pattern}}[0];
    #&main::daemon_log("who_has-msg looking for $search_pattern $search_element", 7);

    # scanning known_clients for search_pattern
    my @host_addresses = keys %$main::known_clients;
    my $known_clients_entries = length @host_addresses;
    my $host_address;
    foreach my $host (@host_addresses) {
        my $client_element = $main::known_clients->{$host}->{$search_pattern};
        if ($search_element eq $client_element) {
            $host_address = $host;
            last;
        }
    }
        
    # search was successful
    if (defined $host_address) {
        my $source = @{$msg_hash->{source}}[0];
        my $out_hash = &create_xml_hash("who_has_i_do", $server_address, $source, "mac_address");
        &add_content2xml_hash($out_hash, "mac_address", $search_element);
        my $out_msg = &create_xml_string($out_hash);
        push(@out_msg_l, $out_msg);
    }
    return @out_msg_l;
}


sub who_has_i_do {
    my ($msg_hash) = @_ ;
    my $header = @{$msg_hash->{header}}[0];
    my $source = @{$msg_hash->{source}}[0];
    my $search_param = @{$msg_hash->{$header}}[0];
    my $search_value = @{$msg_hash->{$search_param}}[0];
    print "\ngot msg $header:\nserver $source has client with $search_param $search_value\n";
}


sub new_syslog_config {
    my ($mac_address, $session_id) = @_;
    my $syslog_msg;
    my $ldap_handle=&main::get_ldap_handle();

	# Perform search
    my $ldap_res = $ldap_handle->search( base   => $ldap_base,
		scope  => 'sub',
		attrs => ['gotoSyslogServer'],
		filter => "(&(objectClass=GOhard)(macaddress=$mac_address))");
	if($ldap_res->code) {
		&main::daemon_log("$session_id ERROR: new_syslog_config: ldap search: ".$ldap_res->error, 1);
        &main::release_ldap_handle($ldap_handle);
		return;
	}

	# Sanity check
	if ($ldap_res->count != 1) {
		&main::daemon_log("$session_id WARNING: client with mac address $mac_address not found/unique/active - not sending syslog config".
                "\n\tbase: $ldap_base".
                "\n\tscope: sub".
                "\n\tattrs: gotoSyslogServer".
                "\n\tfilter: (&(objectClass=GOhard)(macaddress=$mac_address))", 1);
        &main::release_ldap_handle($ldap_handle);
		return;
	}

	my $entry= $ldap_res->entry(0);
    my $filter_dn = &Net::LDAP::Util::escape_filter_value($entry->dn);
	my $syslog_server = $entry->get_value("gotoSyslogServer");

    # If no syslog server is specified at host, just have a look at the object group of the host
    # Perform object group search
    if (not defined $syslog_server) {
        my $ldap_res = $ldap_handle->search( base   => $ldap_base,
                scope  => 'sub',
                attrs => ['gotoSyslogServer'],
                filter => "(&(gosaGroupObjects=[W])(objectClass=gosaGroupOfNames)(member=$filter_dn))");
        if($ldap_res->code) {
            &main::daemon_log("$session_id ERROR: new_syslog_config: ldap search: ".$ldap_res->error, 1);
            &main::release_ldap_handle($ldap_handle);
            return;
        }

        # Sanity check
        if ($ldap_res->count != 1) {
            &main::daemon_log("$session_id ERROR: client with mac address $mac_address not found/unique/active - not sending syslog config".
                    "\n\tbase: $ldap_base".
                    "\n\tscope: sub".
                    "\n\tattrs: gotoSyslogServer".
                    "\n\tfilter: (&(gosaGroupObjects=[W])(objectClass=gosaGroupOfNames)(member=$filter_dn))", 1);
            &main::release_ldap_handle($ldap_handle);
            return;
        }

        my $entry= $ldap_res->entry(0);
        $syslog_server= $entry->get_value("gotoSyslogServer");
    }

    # Return if no syslog server specified
    if (not defined $syslog_server) {
        &main::daemon_log("$session_id WARNING: no syslog server specified for this host '$mac_address'", 3);
        &main::release_ldap_handle($ldap_handle);
        return;
    }

 
    # Add syslog server to 'syslog_config' message
    my $syslog_msg_hash = &create_xml_hash("new_syslog_config", $server_address, $mac_address);
    &add_content2xml_hash($syslog_msg_hash, "server", $syslog_server);

    &main::release_ldap_handle($ldap_handle);
    return &create_xml_string($syslog_msg_hash);
}


sub new_ntp_config {
    my ($address, $session_id) = @_;
    my $ntp_msg;
    my $ldap_handle=&main::get_ldap_handle();

	# Perform search
    my $ldap_res = $ldap_handle->search( base   => $ldap_base,
		scope  => 'sub',
		attrs => ['gotoNtpServer'],
		filter => "(&(objectClass=GOhard)(macaddress=$address))");
	if($ldap_res->code) {
		&main::daemon_log("$session_id ERROR: new_ntp_config: ldap search: ".$ldap_res->error, 1);
        &main::release_ldap_handle($ldap_handle);
		return;
	}

	# Sanity check
	if ($ldap_res->count != 1) {
		&main::daemon_log("$session_id ERROR: client with mac address $address not found/unique/active - not sending ntp config".
                "\n\tbase: $ldap_base".
                "\n\tscope: sub".
                "\n\tattrs: gotoNtpServer".
                "\n\tfilter: (&(objectClass=GOhard)(macaddress=$address))", 1);
        &main::release_ldap_handle($ldap_handle);
		return;
	}

	my $entry= $ldap_res->entry(0);
    my $filter_dn = &Net::LDAP::Util::escape_filter_value($entry->dn);
	my @ntp_servers= $entry->get_value("gotoNtpServer");

    # If no ntp server is specified at host, just have a look at the object group of the host
    # Perform object group search
    if ((not @ntp_servers) || (@ntp_servers == 0)) {
        my $ldap_res = $ldap_handle->search( base   => $ldap_base,
                scope  => 'sub',
                attrs => ['gotoNtpServer'],
                filter => "(&(gosaGroupObjects=[W])(objectClass=gosaGroupOfNames)(member=$filter_dn))");
        if($ldap_res->code) {
            &main::daemon_log("$session_id ERROR: new_ntp_config: ldap search: ".$ldap_res->error, 1);
            &main::release_ldap_handle($ldap_handle);
            return;
        }

        # Sanity check
        if ($ldap_res->count != 1) {
            &main::daemon_log("$session_id ERROR: client with mac address $address not found/unique/active - not sending ntp config".
                    "\n\tbase: $ldap_base".
                    "\n\tscope: sub".
                    "\n\tattrs: gotoNtpServer".
                    "\n\tfilter: (&(gosaGroupObjects=[W])(objectClass=gosaGroupOfNames)(member=$filter_dn))", 1);
            &main::release_ldap_handle($ldap_handle);
            return;
        }

        my $entry= $ldap_res->entry(0);
        @ntp_servers= $entry->get_value("gotoNtpServer");
    }

    # Return if no ntp server specified
    if ((not @ntp_servers) || (@ntp_servers == 0)) {
        &main::daemon_log("$session_id WARNING: no ntp server specified for this host '$address'", 3);
        &main::release_ldap_handle($ldap_handle);
        return;
    }
 
    # Add each ntp server to 'ntp_config' message
    my $ntp_msg_hash = &create_xml_hash("new_ntp_config", $server_address, $address);
    foreach my $ntp_server (@ntp_servers) {
        &add_content2xml_hash($ntp_msg_hash, "server", $ntp_server);
    }

    &main::release_ldap_handle($ldap_handle);
    return &create_xml_string($ntp_msg_hash);
}


#===  FUNCTION  ================================================================
#         NAME:  new_ldap_config
#   PARAMETERS:  address - string - ip address and port of a host
#      RETURNS:  gosa-si conform message
#  DESCRIPTION:  send to address the ldap configuration found for dn gotoLdapServer
#===============================================================================
sub new_ldap_config {
	my ($address, $session_id) = @_ ;

	my $sql_statement= "SELECT * FROM known_clients WHERE hostname='$address' OR macaddress LIKE '$address'";
	my $res = $main::known_clients_db->select_dbentry( $sql_statement );

	# check hit
	my $hit_counter = keys %{$res};
	if( not $hit_counter == 1 ) {
		&main::daemon_log("$session_id ERROR: new_ldap_config: more or no hit found in known_clients_db by query '$sql_statement'", 1);
        return;
	}

    $address = $res->{1}->{hostname};
	my $macaddress = $res->{1}->{macaddress};
	my $hostkey = $res->{1}->{hostkey};
	
	if (not defined $macaddress) {
		&main::daemon_log("$session_id ERROR: new_ldap_config: no mac address found for client $address", 1);
		return;
	}

	# Perform search
    my $ldap_handle=&main::get_ldap_handle();
    $mesg = $ldap_handle->search( base   => $ldap_base,
		scope  => 'sub',
		attrs => ['dn', 'gotoLdapServer', 'gosaUnitTag', 'FAIclass'],
		filter => "(&(objectClass=GOhard)(macaddress=$macaddress))");
	if($mesg->code) {
		&main::daemon_log("$session_id ERROR: new_ldap_config: ldap search: ".$mesg->error, 1);
        &main::release_ldap_handle($ldap_handle);
		return;
	}

	# Sanity check
	if ($mesg->count != 1) {
		&main::daemon_log("$session_id ERROR: client with mac address $macaddress not found/unique/active - not sending ldap config".
                "\n\tbase: $ldap_base".
                "\n\tscope: sub".
                "\n\tattrs: dn, gotoLdapServer".
                "\n\tfilter: (&(objectClass=GOhard)(macaddress=$macaddress))", 1);
        &main::release_ldap_handle($ldap_handle);
		return;
	}

	my $entry= $mesg->entry(0);
	my $filter_dn= &Net::LDAP::Util::escape_filter_value($entry->dn);
	my @servers= $entry->get_value("gotoLdapServer");
	my $unit_tag= $entry->get_value("gosaUnitTag");
	my @ldap_uris;
	my $server;
	my $base;
	my $release;
    my $dn= $entry->dn;

	# Fill release if available
	my $FAIclass= $entry->get_value("FAIclass");
	if (defined $FAIclass && $FAIclass =~ /^.* :([A-Za-z0-9\/.]+).*$/) {
		$release= $1;
	}

	# Do we need to look at an object class?
	if (not @servers){
	        $mesg = $ldap_handle->search( base   => $ldap_base,
			scope  => 'sub',
			attrs => ['dn', 'gotoLdapServer', 'FAIclass'],
			filter => "(&(gosaGroupObjects=[W])(objectClass=gosaGroupOfNames)(member=$filter_dn))");
		if($mesg->code) {
			&main::daemon_log("$session_id ERROR: new_ldap_config: unable to search for '(&(objectClass=gosaGroupOfNames)(member=$filter_dn))': ".$mesg->error, 1);
            &main::release_ldap_handle($ldap_handle);
			return;
		}

		# Sanity check
        if ($mesg->count != 1) {
            &main::daemon_log("$session_id WARNING: new_ldap_config: client with mac address $macaddress not found/unique/active - not sending ldap config".
                    "\n\tbase: $ldap_base".
                    "\n\tscope: sub".
                    "\n\tattrs: dn, gotoLdapServer, FAIclass".
                    "\n\tfilter: (&(gosaGroupObjects=[W])(objectClass=gosaGroupOfNames)(member=$filter_dn))", 1);
            &main::release_ldap_handle($ldap_handle);
            return;
        }

		$entry= $mesg->entry(0);
		$dn= $entry->dn;
		@servers= $entry->get_value("gotoLdapServer");

		if (not defined $release){
			$FAIclass= $entry->get_value("FAIclass");
			if (defined $FAIclass && $FAIclass =~ /^.* :([A-Za-z0-9\/.]+).*$/) {
				$release= $1;
			}
		}
	}

	@servers= sort (@servers);

    # complain if no ldap information found
    if (@servers == 0) {
        &main::daemon_log("$session_id ERROR: no gotoLdapServer information for LDAP entry '$dn'", 1);
    }

	foreach $server (@servers){
                # Conversation for backward compatibility
                if (not $server =~ /^\d+:[^:]+:ldap[^:]*:\/\// ) {
                    if ($server =~ /^([^:]+):([^:]+)$/ ) {
                      $server= "1:dummy:ldap://$1/$2";
                    } elsif ($server =~ /^(\d+):([^:]+):(.*)$/ ) {
                      $server= "$1:dummy:ldap://$2/$3";
                    }
                }

                $base= $server;
                $server =~ s%^[^:]+:[^:]+:(ldap.*://[^/]+)/.*$%$1%;
                $base =~ s%^[^:]+:[^:]+:ldap.*://[^/]+/(.*)$%$1%;
                push (@ldap_uris, $server);
	}

	# Assemble data package
	my %data = ( 'ldap_uri'  => \@ldap_uris, 'ldap_base' => $base,
		'ldap_cfg' => \@ldap_cfg, 'pam_cfg' => \@pam_cfg,'nss_cfg' => \@nss_cfg );
	if (defined $release){
		$data{'release'}= $release;
	}

	# Need to append GOto settings?
	if (defined $goto_admin and defined $goto_secret){
		$data{'goto_admin'}= $goto_admin;
		$data{'goto_secret'}= $goto_secret;
	}

	# Append unit tag if needed
	if (defined $unit_tag){

		# Find admin base and department name
		$mesg = $ldap_handle->search( base   => $ldap_base,
			scope  => 'sub',
			attrs => ['dn', 'ou'],
			filter => "(&(objectClass=gosaAdministrativeUnit)(gosaUnitTag=$unit_tag))");
		#$mesg->code && die $mesg->error;
		if($mesg->code) {
			&main::daemon_log("$session_id ERROR: new_ldap_config: ldap search: ".$mesg->error, 1);
            &main::release_ldap_handle($ldap_handle);
			return "error-unit-tag-count-0";
		}

		# Sanity check
		if ($mesg->count != 1) {
			&main::daemon_log("WARNING: cannot find administrative unit for client with tag $unit_tag", 3);
            &main::release_ldap_handle($ldap_handle);
			return "error-unit-tag-count-".$mesg->count;
		}

		$entry= $mesg->entry(0);
		$data{'admin_base'}= $entry->dn;
		$data{'department'}= $entry->get_value("ou");

		# Append unit Tag
		$data{'unit_tag'}= $unit_tag;
	}
    &main::release_ldap_handle($ldap_handle);

	# Send information
	return &build_msg("new_ldap_config", $server_address, $address, \%data);
}


#===  FUNCTION  ================================================================
#         NAME:  hardware_config
#   PARAMETERS:  address - string - ip address and port of a host
#      RETURNS:  
#  DESCRIPTION:  
#===============================================================================
sub hardware_config {
	my ($msg, $msg_hash, $session_id) = @_ ;
	my $address = @{$msg_hash->{source}}[0];
	my $header = @{$msg_hash->{header}}[0];
	my $gotoHardwareChecksum = @{$msg_hash->{gotoHardwareChecksum}}[0];

	my $sql_statement= "SELECT * FROM known_clients WHERE hostname='$address'";
	my $res = $main::known_clients_db->select_dbentry( $sql_statement );

	# check hit
	my $hit_counter = keys %{$res};
	if( not $hit_counter == 1 ) {
		&main::daemon_log("$session_id ERROR: hardware_config: more or no hit found in known_clients_db by query by '$address'", 1);
	}
	my $macaddress = $res->{1}->{macaddress};
	my $hostkey = $res->{1}->{hostkey};

	if (not defined $macaddress) {
		&main::daemon_log("$session_id ERROR: hardware_config: no mac address found for client $address", 1);
		return;
	}

	# Perform search
    my $ldap_handle=&main::get_ldap_handle();
	$mesg = $ldap_handle->search(
		base   => $ldap_base,
		scope  => 'sub',
		filter => "(&(objectClass=GOhard)(|(macAddress=$macaddress)(dhcpHWaddress=ethernet $macaddress)))"
	);

	if($mesg->count() == 0) {
		&main::daemon_log("$session_id INFO: Host was not found in LDAP!", 5);

		# set status = hardware_detection at jobqueue if entry exists
		# TODO
		# resolve plain name for host
		my $func_dic = {table=>$main::job_queue_tn,
				primkey=>['macaddress', 'headertag'],
				timestamp=>&get_time,
				status=>'processing',
				result=>'none',
				progress=>'hardware-detection',
				headertag=>'trigger_action_reinstall',
				targettag=>$address,
				xmlmessage=>'none',
				macaddress=>$macaddress,
				plainname=>'none',
                siserver=>'localhost',
                modified=>'1',
		};
		my $hd_res = $main::job_db->add_dbentry($func_dic);
		&main::daemon_log("$session_id INFO: add '$macaddress' to job queue as an installing job", 5);
	
	} else {
		my $entry= $mesg->entry(0);
		if (defined($entry->get_value("gotoHardwareChecksum"))) {
			if (! ($entry->get_value("gotoHardwareChecksum") eq $gotoHardwareChecksum)) {
				$entry->replace(gotoHardwareChecksum => $gotoHardwareChecksum);
				if($entry->update($ldap_handle)) {
					&main::daemon_log("$session_id INFO: Hardware changed! Detection triggered.", 5);
				}
			} else {
				# Nothing to do
                &main::release_ldap_handle($ldap_handle);
				return;
			}
		} 
	} 

	# Assemble data package
	my %data = ();

	# Need to append GOto settings?
	if (defined $goto_admin and defined $goto_secret){
		$data{'goto_admin'}= $goto_admin;
		$data{'goto_secret'}= $goto_secret;
	}

    &main::release_ldap_handle($ldap_handle);

	# Send information
	return &build_msg("detect_hardware", $server_address, $address, \%data);
}

sub server_matches {
    my ($target, $session_id) = @_ ;
	my $target_ip = ($1) if $target =~ /^([0-9\.]*?):.*$/;
	if(!defined($target_ip) or length($target_ip) == 0) {
		return;
	}

	my $result = 0;

	if($server_ip eq $target_ip) {
		$result= 1;
	} elsif ($target_ip eq "0.0.0.0") {
		$result= 1;
	} elsif ($server_ip eq "0.0.0.0") {	
		if ($target_ip eq "127.0.0.1") {
			$result= 1;
		} else {
			my $PROC_NET_ROUTE= ('/proc/net/route');

			open(my $FD_PROC_NET_ROUTE, "<", "$PROC_NET_ROUTE")
				or die "Could not open $PROC_NET_ROUTE";

			my @ifs = <$FD_PROC_NET_ROUTE>;

			close($FD_PROC_NET_ROUTE);

			# Eat header line
			shift @ifs;
			chomp @ifs;
			foreach my $line(@ifs) {
				my ($Iface,$Destination,$Gateway,$Flags,$RefCnt,$Use,$Metric,$Mask,$MTU,$Window,$IRTT)=split(/\s/, $line);
				my $destination;
				my $mask;
				my ($d,$c,$b,$a)=unpack('a2 a2 a2 a2', $Destination);
				$destination= sprintf("%d.%d.%d.%d", hex($a), hex($b), hex($c), hex($d));
				($d,$c,$b,$a)=unpack('a2 a2 a2 a2', $Mask);
				$mask= sprintf("%d.%d.%d.%d", hex($a), hex($b), hex($c), hex($d));
				if(new NetAddr::IP($target_ip)->within(new NetAddr::IP($destination, $mask))) {
					# destination matches route, save mac and exit
					$result= 1;
					last;
				}
			}
		}
	} else {
		&main::daemon_log("$session_id INFO: Target ip $target_ip does not match Server ip $server_ip",5);
	}

	return $result;
}

# vim:ts=4:shiftwidth:expandtab
1;
