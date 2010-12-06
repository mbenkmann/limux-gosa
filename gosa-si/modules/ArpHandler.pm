package ArpHandler;

use strict;
use warnings;

use Net::LDAP;
use Net::LDAP::LDIF;
use Net::LDAP::Entry;
use Net::DNS;
use Switch;
use Data::Dumper;
use GOsaSI::GosaSupportDaemon;

use Exporter;
use POSIX;
use Fcntl;
use Socket;

our @ISA = ("Exporter");

# Don't start if some of the modules are missing
my $start_service=1;
my $lookup_vendor=1;
BEGIN{
	unless(eval('use Socket qw(PF_INET SOCK_DGRAM inet_ntoa sockaddr_in)')) {
		$start_service=0;
	}
	unless(eval('use POE qw(Component::Pcap Component::ArpWatch)')) {
		$start_service=0;
	}
	unless(eval('use Net::MAC::Vendor')) {
		$lookup_vendor=0;
	}
}

END{}

my ($timeout, $mailto, $mailfrom, $user, $group);
my ($arp_enabled, $arp_interface, $arp_update, $ldap_uri, $ldap_base, $ldap_admin_dn, $ldap_admin_password);
my $hosts_database={};
my $ldap;

my %cfg_defaults =
(
    "ArpHandler" => {
        "enabled"             => [\$arp_enabled,         "true"],
        "interface"           => [\$arp_interface,       "all"],
        "update-entries"      => [\$arp_update,          "false"],
    },
    "server" => {
        "ldap-uri"            => [\$ldap_uri,            ""],
        "ldap-base"           => [\$ldap_base,           ""],
        "ldap-admin-dn"       => [\$ldap_admin_dn,       ""],
        "ldap-admin-password" => [\$ldap_admin_password, ""],
    },
);

# to be removed use only main::read_configfile
#===  FUNCTION  ================================================================
#         NAME:  read_configfile
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
}

sub get_module_info {
	my @info = (undef, undef);

	&local_read_configfile();
	# Don't start if some of the modules are missing
	if(($arp_enabled eq 'true') && $start_service) {
		if($lookup_vendor) {
			# put the file in /etc/gosa/oui.txt or use the native oui.txt from snmp 
			eval("Net::MAC::Vendor::load_cache('file:///usr/lib/gosa-si/modules/oui.txt')");
			if($@) {
				&main::daemon_log("Loading OUI cache file failed! MAC Vendor lookup disabled", 1);
				$lookup_vendor=0;
			} else {
				&main::daemon_log("Loading OUI cache file suceeded!", 6);
			}
		}

		# When interface is not configured (or 'all'), start arpwatch on all possible interfaces
		if ((!defined($arp_interface)) || $arp_interface eq 'all') {
			foreach my $device(&get_interfaces) {
				# TODO: Need a better workaround for IPv4-to-IPv6 bridges
				if($device =~ m/^sit\d+$/) {
					next;
				}

				# If device has a valid mac address
				# TODO: Check if this should be the right way
				if(not(&get_mac($device) eq "00:00:00:00:00:00")) {
					&main::daemon_log("Starting ArpWatch on $device", 1);
					POE::Session->create( 
						inline_states => {
							_start => sub {
								&start(@_,$device);
							},
							_stop => sub {
								$_[KERNEL]->post( sprintf("arp_watch_$device") => 'shutdown' )
							},
							got_packet => \&got_packet,
						},
					);
				}
			}
		} else {
			foreach my $device(split(/[\s,]+/, $arp_interface)) {
				&main::daemon_log("Starting ArpWatch on $device", 1);
				POE::Session->create( 
					inline_states => {
						_start => sub {
							&start(@_,$device);
						},
						_stop => sub {
							$_[KERNEL]->post( sprintf("arp_watch_$device") => 'shutdown' )
						},
						got_packet => \&got_packet,
					},
				);
			}
		}
	} else {
		&main::daemon_log("ArpHandler disabled. Not starting any capture processes");
	}
	return \@info;
}

sub process_incoming_msg {
	return 1;
}

sub start {
	my $device = (exists($_[ARG0])?$_[ARG0]:'eth0');
	POE::Component::ArpWatch->spawn( 
		Alias => sprintf("arp_watch_$device"),
		Device => $device, 
		Dispatch => 'got_packet',
		Session => $_[SESSION],
	);

	$_[KERNEL]->post( sprintf("arp_watch_$device") => 'run' );
}

sub got_packet {
	my ($kernel, $heap, $sender, $packet) = @_[KERNEL, HEAP, SENDER, ARG0];

	if(	$packet->{source_haddr} eq "00:00:00:00:00:00" || 
		$packet->{source_haddr} eq "ff:ff:ff:ff:ff:ff" || 
		$packet->{source_ipaddr} eq "0.0.0.0") {
		return;
	}
	
	my $capture_device = sprintf "%s", $kernel->alias_list($sender) =~ /^arp_watch_(.*)$/;

	my $ldap_handle = &main::get_ldap_handle(); 
    if(!exists($hosts_database->{$packet->{source_haddr}})) {
		my $dnsname= gethostbyaddr(inet_aton($packet->{source_ipaddr}), AF_INET) || $packet->{source_ipaddr};
		my $ldap_result=&get_host_from_ldap($packet->{source_haddr});
		if(exists($ldap_result->{dn}) and $arp_update eq "true") {
			$hosts_database->{$packet->{source_haddr}}=$ldap_result;
			$hosts_database->{$packet->{source_haddr}}->{dnsname}= $dnsname;
			if(!exists($ldap_result->{ipHostNumber})) {
				$hosts_database->{$packet->{source_haddr}}->{ipHostNumber}=$packet->{source_ipaddr};
			} else {
				if(!($ldap_result->{ipHostNumber} eq $packet->{source_ipaddr})) {
					&main::daemon_log(
						"Current IP Address ".$packet->{source_ipaddr}.
						" of host ".$hosts_database->{$packet->{source_haddr}}->{dnsname}.
						" differs from LDAP (".$ldap_result->{ipHostNumber}.")", 4);
				}
			}
			$hosts_database->{$packet->{source_haddr}}->{dnsname}=$dnsname;
			&main::daemon_log("Host was found in LDAP as ".$ldap_result->{dn}, 8);
		} else {
			$hosts_database->{$packet->{source_haddr}}={
				macAddress => $packet->{source_haddr},
				ipHostNumber => $packet->{source_ipaddr},
				dnsname => $dnsname,
				cn => (($dnsname =~ /^(\d){1,3}\.(\d){1,3}\.(\d){1,3}\.(\d){1,3}/) ? $dnsname : sprintf "%s", $dnsname =~ /([^\.]+)\./),
				macVendor => (($lookup_vendor) ? &get_vendor_for_mac($packet->{source_haddr}) : "Unknown Vendor"),
			};
			&main::daemon_log("A DEBUG: Host was not found in LDAP (".($hosts_database->{$packet->{source_haddr}}->{dnsname}).")",522);
			&main::daemon_log(
				"A INFO: New Host ".($hosts_database->{$packet->{source_haddr}}->{dnsname}).
				": ".$hosts_database->{$packet->{source_haddr}}->{ipHostNumber}.
				"/".$hosts_database->{$packet->{source_haddr}}->{macAddress},5);
			&add_ldap_entry(
				$ldap_handle, 
				$ldap_base, 
				$hosts_database->{$packet->{source_haddr}}->{macAddress},
				'new-system',
				$hosts_database->{$packet->{source_haddr}}->{ipHostNumber},
				'interface',
				$hosts_database->{$packet->{source_haddr}}->{macVendor});
		}
		$hosts_database->{$packet->{source_haddr}}->{device}= $capture_device;
	} else {
		if(($arp_update eq "true") and !($hosts_database->{$packet->{source_haddr}}->{ipHostNumber} eq $packet->{source_ipaddr})) {
			&main::daemon_log(
				"A INFO: IP Address change of MAC ".$packet->{source_haddr}.
				": ".$hosts_database->{$packet->{source_haddr}}->{ipHostNumber}.
				"->".$packet->{source_ipaddr}, 5);
			$hosts_database->{$packet->{source_haddr}}->{ipHostNumber}= $packet->{source_ipaddr};
			&change_ldap_entry(
				$ldap_handle, 
				$ldap_base, 
				$hosts_database->{$packet->{source_haddr}}->{macAddress},
				'ip-changed',
				$hosts_database->{$packet->{source_haddr}}->{ipHostNumber},
			);

		}
		&main::daemon_log("Host already in cache (".($hosts_database->{$packet->{source_haddr}}->{device})."->".($hosts_database->{$packet->{source_haddr}}->{dnsname}).")",8);
	}
} 

sub get_host_from_ldap {
	my $mac=shift;
	my $result={};
		
    my $ldap_handle = &main::get_ldap_handle();
	if(defined($ldap_handle)) {
		my $ldap_result= &search_ldap_entry(
			$ldap_handle,
			$ldap_base,
			"(|(macAddress=$mac)(dhcpHWAddress=ethernet $mac))"
		);

		if(defined($ldap_result) && $ldap_result->count==1) {
			if(exists($ldap_result->{entries}[0]) && 
				exists($ldap_result->{entries}[0]->{asn}->{objectName}) && 
				exists($ldap_result->{entries}[0]->{asn}->{attributes})) {

				for my $attribute(@{$ldap_result->{entries}[0]->{asn}->{attributes}}) {
					if($attribute->{type} eq 'cn') {
						$result->{cn} = $attribute->{vals}[0];
					}
					if($attribute->{type} eq 'macAddress') {
						$result->{macAddress} = $attribute->{vals}[0];
					}
					if($attribute->{type} eq 'dhcpHWAddress') {
						$result->{dhcpHWAddress} = $attribute->{vals}[0];
					}
					if($attribute->{type} eq 'ipHostNumber') {
						$result->{ipHostNumber} = $attribute->{vals}[0];
					}
				}
			}
			$result->{dn} = $ldap_result->{entries}[0]->{asn}->{objectName};
		}
	}

	return $result;
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

sub get_vendor_for_mac {
	my $mac=shift;
	my $result="Unknown Vendor";

	if(defined($mac)) {
		my $vendor= Net::MAC::Vendor::fetch_oui_from_cache(Net::MAC::Vendor::normalize_mac($mac));
		if(length($vendor) > 0) {
			$result= @{$vendor}[0];
		}
		&main::daemon_log("A INFO: Looking up Vendor for MAC ".$mac.": $result", 5);
	}

	return $result;
}

#===  FUNCTION  ================================================================
#         NAME:  add_ldap_entry
#      PURPOSE:  adds an element to ldap-tree
#   PARAMETERS:  
#      RETURNS:  none
#  DESCRIPTION:  ????
#       THROWS:  no exceptions
#     COMMENTS:  none
#     SEE ALSO:  n/a/bin
#===============================================================================
sub add_ldap_entry {
	my ($ldap_tree, $ldap_base, $mac, $gotoSysStatus, $ip, $interface, $desc) = @_;
	if(defined($ldap_tree)) {
		my $dn = "cn=".$hosts_database->{$mac}->{cn}.",ou=incoming,$ldap_base";
		my $s_res = &search_ldap_entry($ldap_tree, $ldap_base, "(|(macAddress=$mac)(dhcpHWAddress=ethernet $mac))");
		my $c_res = (defined($s_res))?$s_res->count:0;
		if($c_res == 1) {
			&main::daemon_log("A WARNING: macAddress $mac already in LDAP", 3);
			return;
		} elsif($c_res > 0) {
			&main::daemon_log("A ERROR: macAddress $mac exists $c_res times in LDAP", 1);
			return;
		}

		# create LDAP entry 
		my $entry = Net::LDAP::Entry->new( $dn );
		$entry->dn($dn);
		$entry->add("objectClass" => "GOhard");
		$entry->add("cn" => $hosts_database->{$mac}->{cn});
		$entry->add("macAddress" => $mac);
		if(defined $gotoSysStatus) {$entry->add("gotoSysStatus" => $gotoSysStatus)}
		if(defined $ip) {$entry->add("ipHostNumber" => $ip) }
		#if(defined $interface) { }
		if(defined $desc) {$entry->add("description" => $desc) }

		# submit entry to LDAP
		my $result = $entry->update ($ldap_tree); 

		# for $result->code constants please look at Net::LDAP::Constant
		if($result->code == 68) {   # entry already exists 
			&main::daemon_log("A WARNING: $dn ".$result->error, 3);
		} elsif($result->code == 0) {   # everything went fine
			&main::daemon_log("A INFO: Add entry $dn to ldap", 5);
		} else {  # if any other error occur
			&main::daemon_log("A ERROR: $dn, ".$result->code.", ".$result->error, 1);
		}
	} else {
		&main::daemon_log("A INFO: Not adding new Entry: LDAP disabled", 5);
	}
	return;
}


#===  FUNCTION  ================================================================
#         NAME:  change_ldap_entry
#      PURPOSE:  ????
#   PARAMETERS:  ????
#      RETURNS:  ????
#  DESCRIPTION:  ????
#       THROWS:  no exceptions
#     COMMENTS:  none
#     SEE ALSO:  n/a
#===============================================================================
sub change_ldap_entry {
	my ($ldap_tree, $ldap_base, $mac, $gotoSysStatus, $ip) = @_;

	if(defined($ldap_tree)) {
		# check if ldap_entry exists or not
		my $s_res = &search_ldap_entry($ldap_tree, $ldap_base, "(|(macAddress=$mac)(dhcpHWAddress=ethernet $mac))");
		my $c_res = (defined $s_res)?$s_res->count:0;
		if($c_res == 0) {
			&main::daemon_log("WARNING: macAddress $mac not in LDAP", 1);
			return;
		} elsif($c_res > 1) {
			&main::daemon_log("ERROR: macAddress $mac exists $c_res times in LDAP", 1);
			return;
		}

		my $s_res_entry = $s_res->pop_entry();
		my $dn = $s_res_entry->dn();
		my $replace = {
			'gotoSysStatus' => $gotoSysStatus,
		};
		if (defined($ip)) {
			$replace->{'ipHostNumber'} = $ip;
		}
		my $result = $ldap_tree->modify( $dn, replace => $replace );

		# for $result->code constants please look at Net::LDAP::Constant
		if($result->code == 32) {   # entry doesnt exists 
			&add_ldap_entry($mac, $gotoSysStatus);
		} elsif($result->code == 0) {   # everything went fine
			&main::daemon_log("entry $dn changed successful", 1);
		} else {  # if any other error occur
			&main::daemon_log("ERROR: $dn, ".$result->code.", ".$result->error, 1);
		}
	} else {
		&main::daemon_log("Not changing Entry: LDAP disabled", 6);
	}

	return;
}

#===  FUNCTION  ================================================================
#         NAME:  search_ldap_entry
#      PURPOSE:  ????
#   PARAMETERS:  [Net::LDAP] $ldap_tree - object of an ldap-tree
#                string $sub_tree - dn of the subtree the search is performed
#                string $search_string - either a string or a Net::LDAP::Filter object
#      RETURNS:  [Net::LDAP::Search] $msg - result object of the performed search
#  DESCRIPTION:  ????
#       THROWS:  no exceptions
#     COMMENTS:  none
#     SEE ALSO:  n/a
#===============================================================================
sub search_ldap_entry {
	my ($ldap_tree, $sub_tree, $search_string) = @_;
	my $msg;
	if(defined($ldap_tree)) {
		$msg = $ldap_tree->search( # perform a search
			base   => $sub_tree,
			filter => $search_string,
		) or &main::daemon_log("cannot perform search at ldap: $@", 1);
	}
	return $msg;
}

# vim:ts=4:shiftwidth:expandtab
1;
