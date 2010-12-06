package corefunctions;

use strict;
use warnings;

use File::Basename;
use GOsaSI::GosaSupportDaemon;

use Exporter;
use Fcntl;


our @ISA = qw(Exporter);

my @events = (
  "get_events",
  "registered",
  "new_syslog_config",
  "new_ntp_config",
  "new_ldap_config",
  "new_key",
  "generate_hw_digest",     # no implementations
  "detect_hardware",
  "confirm_new_key",
  "ping",
  "import_events",    # no implementations
  );
  
our @EXPORT = @events;

my ($ldap_enabled, $offline_enabled, $ldap_config, $pam_config, $nss_config, $fai_logpath, $ldap_config_exit_hook);

my $chrony_file = "/etc/chrony/chrony.conf";
my $syslog_file = "/etc/syslog.conf";

# why is it re read here, the config is read at the start of the program no !!
my %cfg_defaults = (
	"client" => {
		"ldap" => [\$ldap_enabled, 1],
		"offline-ldap" => [\$offline_enabled, 0],
		"ldap-config" => [\$ldap_config, "/etc/ldap/ldap.conf"],
		"pam-config" => [\$pam_config, "/etc/pam_ldap.conf"],
		"nss-config" => [\$nss_config, "/etc/libnss-ldap.conf"],
		"fai-logpath" => [\$fai_logpath, "/var/log/fai/fai.log"],
		"ldap-config-exit-hook" => [\$ldap_config_exit_hook, undef],
	},
);

BEGIN {}

END {}

### Start ######################################################################

# why not using  the config read in the main ?? !!
&main::read_configfile($main::cfg_file, %cfg_defaults);


my $server_address = $main::server_address;
my $server_key = $main::server_key;
my $client_mac_address = $main::client_mac_address;

sub write_to_file {
	my ($string, $file) = @_;
	my $error = 0;

	if( not defined $file || not -f $file ) {
		&main::daemon_log("ERROR: $0: check '-f file' failed: $file", 1);
		$error++;
	}
	if( not defined $string || 0 == length($string)) {
		&main::daemon_log("ERROR: $0: empty string to write to file '$file'", 1);
		$error++;
	}
	
	if( $error == 0 ) {

		chomp($string);
			
		if( not -f $file ) {
			open (my $FD_FILE, "$file");
			close($FD_FILE);
		}
		open(my $FD_FILE, ">>", "$file") or &main::daemon_log("ERROR in corefunctions.pm: can not open '$file' to write '$string'", 1);;
		print $FD_FILE $string."\n";
		close($FD_FILE);
	}

	return;
}

# should be the first function of each module of gosa-si !!
sub get_events {
	return \@events;
}

sub daemon_log {
	my ($msg, $level) = @_ ;
	&main::daemon_log($msg, $level);
	return;
}

sub registered {
	my ($msg, $msg_hash) = @_ ;

	my $header = @{$msg_hash->{'header'}}[0];
	if( $header eq "registered" ) {
		my $source = @{$msg_hash->{'source'}}[0];
		&main::daemon_log("INFO: registration at $source", 1);
		$main::server_address = $source;
	}

	# set globaly variable client_address
	my $target =  @{$msg_hash->{'target'}}[0];
	$main::client_address = $target;

	# set registration_flag to true 
	&main::_setREGISTERED(1);

	# Write the MAC address to file
	if(stat($main::opts_file)) { 
		unlink($main::opts_file);
	}

	my $opts_file_FH;
	my $hostname= $main::client_dnsname;
	$hostname =~ s/\..*$//;
	$hostname =~ tr/A-Z/a-z/;
	sysopen($opts_file_FH, $main::opts_file, O_RDWR | O_CREAT | O_TRUNC , 0644);
	print $opts_file_FH "MAC=\"$main::client_mac_address\"\n";
	print $opts_file_FH "IPADDRESS=\"$main::client_ip\"\n";
	print $opts_file_FH "HOSTNAME=\"$hostname\"\n";
	print $opts_file_FH "FQDN=\"$main::client_dnsname\"\n";
	if(defined(@{$msg_hash->{'ldap_available'}}) &&
			   @{$msg_hash->{'ldap_available'}}[0] eq "true") {
		print $opts_file_FH "LDAP_AVAILABLE=\"true\"\n";
	}
	if(defined(@{$msg_hash->{'error'}})) {
		my $errormsg= @{$msg_hash->{'error'}}[0];
		print $opts_file_FH "GOSA_SI_ERROR=\"$errormsg\"\n";
		&write_to_file($errormsg, $fai_logpath);
	}
	close($opts_file_FH);
	 
	return;
}

sub server_leaving {
	my ($msg_hash) = @_ ;
	my $source = @{$msg_hash->{'source'}}[0]; 
	my $header = @{$msg_hash->{'header'}}[0];
	
	daemon_log("gosa-si-server $source is going down, cause registration procedure", 1);
	$main::server_address = "none";
	$main::server_key = "none";

	# reinitialization of default values in config file
	&main::read_configfile;
	
	# registrated at new daemon
	&main::register_at_server();
	   
	return;
}


## @method new_syslog_config
# Update or add syslog messages forwarding to specified syslog server.
# @param msg - STRING - xml message with tag server
# @param msg_hash - HASHREF - message information parsed into a hash
sub new_syslog_config {
	my ($msg, $msg_hash) = @_ ;

	# Sanity check of incoming message
	if ((not exists $msg_hash->{'server'}) || (not @{$msg_hash->{'server'}} == 1) ) {
		&main::daemon_log("ERROR: 'new_syslog_config'-message does not contain a syslog server: $msg", 1);
		return;
	}

	# Fetch the new syslog server from incoming message
	my $syslog_server = @{$msg_hash->{'server'}}[0];
	&main::daemon_log("INFO: found syslog server: ".join(", ", $syslog_server), 5); 
	my $found_server_flag = 0;
	
	# Sanity check of /etc/syslog.conf
	if (not -f $syslog_file) {
		&main::daemon_log("ERROR: file '$syslog_file' does not exist, cannot do syslog reconfiguration!", 1);
		return;
	}
	
	# Substitute existing server with new syslog server
	open (my $syslog, "<","$syslog_file");
	my @file = <$syslog>;
	close($syslog);
	my $syslog_server_line = "*.*\t@".$syslog_server."\n"; 
	foreach my $line (@file) {
		if ($line =~ /^\*\.\*\s+@/) {
			$line = $syslog_server_line;
			$found_server_flag++;
		}
	}
	
	# Append new server if no old server configuration found
	if (not $found_server_flag) {
		push(@file, "\n#\n# syslog server configuration written by GOsa-si\n#\n");
		push(@file, $syslog_server_line);
	}
	
	# Write changes to file and close it
	open (my $new_syslog, "+>","$syslog_file");
	print $new_syslog join("", @file);
	close($new_syslog);
	&main::daemon_log("INFO: Wrote new configuration file: $syslog_file", 5);

	# Restart syslog deamon
	my $res = qx(/etc/init.d/sysklogd restart);
	&main::daemon_log("INFO: restart syslog daemon: $res", 5);

	return;
}


## @method new_ntp_config
# Updates the server options in /etc/chrony/chrony.conf and restarts the chrony service
# @param msg - STRING - xml message with tag server
# @param msg_hash - HASHREF - message information parsed into a hash
sub new_ntp_config {
	my ($msg, $msg_hash) = @_ ;

	# Sanity check of incoming message
	if ((not exists $msg_hash->{'server'}) || (not @{$msg_hash->{'server'}} >= 1) ) {
		&main::daemon_log("ERROR: 'new_ntp_config'-message does not contain a ntp server: $msg", 1);
		return;
	}

	# Fetch the new ntp server from incoming message
	my $ntp_servers = $msg_hash->{'server'};
	&main::daemon_log("INFO: found ntp server: ".join(", ", @$ntp_servers), 5); 
	my $ntp_servers_string = "server\t".join("\nserver\t", @$ntp_servers)."\n";
	my $found_server_flag = 0;

	# Sanity check of /etc/chrony/chrony.conf
	if (not -f $chrony_file) {
		&main::daemon_log("ERROR: file '$chrony_file' does not exist, cannot do ntp reconfiguration!", 1);
		return;
	}

	# Substitute existing server with new ntp server
	open (my $ntp, "<","$chrony_file");
	my @file = <$ntp>;
	close($ntp);
	my @new_file;
	foreach my $line (@file) {
		if ($line =~ /^server\s+/) {
			if ($found_server_flag) {	
				$line =~ s/^server\s+[\S]+\s+$//;
			} else {
				$line =~ s/^server\s+[\S]+\s+$/$ntp_servers_string/;
			}
			$found_server_flag++;
		}
		push(@new_file, $line);
	}

	# Append new server if no old server configuration found
	if (not $found_server_flag) {
		push(@new_file, "\n# ntp server configuration written by GOsa-si\n");
		push(@new_file, $ntp_servers_string);
	}

	# Write changes to file and close it
	open (my $new_ntp, ">","$chrony_file");
	print $new_ntp join("", @new_file);
	close($new_ntp);
	&main::daemon_log("INFO: Wrote new configuration file: $chrony_file", 5);

	# Restart chrony deamon
	my $res = qx(/etc/init.d/chrony force-reload);
	&main::daemon_log("INFO: restart chrony daemon: $res", 5);

	return;
}


sub new_ldap_config {
	my ($msg, $msg_hash) = @_ ;

	if( $ldap_enabled != 1 ) {
		return;
	}

	my $element;
	my @ldap_uris;
	my $ldap_base;
	my @ldap_options;
	my @pam_options;
	my @nss_options;
	my $goto_admin;
	my $goto_secret;
	my $admin_base= "";
	my $department= "";
	my $release= "";
	my $unit_tag;
	my $ldap_file;
	my $pam_file;
	my $nss_file;
	my $goto_file;
	my $goto_secret_file;
	my $ldap_offline_file;
	my $ldap_shell_file;
	
	my $ldap_shell_config = "/etc/ldap/ldap-shell.conf";
	my $ldap_offline_config = "/etc/ldap/ldap-offline.conf";
	my $goto_secret_config = "/etc/goto/secret";
	
	# Transform input into array
	while ( my ($key, $value) = each(%$msg_hash) ) {
		if ($key =~ /^(source|target|header)$/) {
				next;
		}

		foreach $element (@$value) {
				if ($key =~ /^ldap_uri$/) {
						push (@ldap_uris, $element);
						next;
				}
				if ($key =~ /^ldap_base$/) {
						$ldap_base= $element;
						next;
				}
				if ($key =~ /^goto_admin$/) {
						$goto_admin= $element;
						next;
				}
				if ($key =~ /^goto_secret$/) {
						$goto_secret= $element;
						next;
				}
				if ($key =~ /^ldap_cfg$/) {
						push (@ldap_options, "$element");
						next;
				}
				if ($key =~ /^pam_cfg$/) {
						push (@pam_options, "$element");
						next;
				}
				if ($key =~ /^nss_cfg$/) {
						push (@nss_options, "$element");
						next;
				}
				if ($key =~ /^admin_base$/) {
						$admin_base= $element;
						next;
				}
				if ($key =~ /^department$/) {
						$department= $element;
						next;
				}
				if ($key =~ /^unit_tag$/) {
						$unit_tag= $element;
						next;
				}
				if ($key =~ /^release$/) {
						$release= $element;
						next;
				}
		}
	}

	# Unit tagging enabled?
	if (defined $unit_tag){
			push (@pam_options, "pam_filter gosaUnitTag=$unit_tag");
			push (@nss_options, "nss_base_passwd  $admin_base?sub?gosaUnitTag=$unit_tag");
			push (@nss_options, "nss_base_group   $admin_base?sub?gosaUnitTag=$unit_tag");
	}

	# Setup ldap.conf
	open($ldap_file, ">","$ldap_config");
	print $ldap_file "# This file was automatically generated by gosa-si-client. Do not change.\n";
	print $ldap_file "URI";
	
	foreach $element (@ldap_uris) {
		print $ldap_file " $element";
	}
	
	print $ldap_file "\nBASE $ldap_base\n";
	foreach $element (@ldap_options) {
		print $ldap_file "$element\n";
	}
	
	close ($ldap_file);
	daemon_log("INFO: Wrote $ldap_config", 5);

	# Setup pam_ldap.conf / libnss-ldap.conf
	open($pam_file, ">","$pam_config");
	open($nss_file, ">","$nss_config");
	print $pam_file "# This file was automatically generated by gosa-si-client. Do not change.\n";
	print $nss_file "# This file was automatically generated by gosa-si-client. Do not change.\n";
	print $pam_file "uri";
	print $nss_file "uri";
	
	foreach $element (@ldap_uris) {
		print $pam_file " $element";
		print $nss_file " $element";
	}
	
	print $pam_file "\nbase $ldap_base\n";
	print $nss_file "\nbase $ldap_base\n";
	
	foreach $element (@pam_options) {
		print $pam_file "$element\n";
	}
	
	foreach $element (@nss_options) {
		print $nss_file "$element\n";
	}
	
	close ($nss_file);
	daemon_log("INFO: Wrote $nss_config", 5);
	close ($pam_file);
	daemon_log("INFO: Wrote $pam_config", 5);

	# Create goto.secrets if told so - for compatibility reasons
	if (defined $goto_admin){
		open($goto_file, ">",$goto_secret_config);
		print $goto_file "GOTOADMIN=\"$goto_admin\"\nGOTOSECRET=\"$goto_secret\"\n";
		close($goto_file);
		chown(0,0, $goto_file);
		chmod(0600, $goto_file);
		daemon_log("INFO: Wrote $goto_secret_config", 5);
	}

	# Write shell based config

    # Get first LDAP server
    my $ldap_server= $ldap_uris[0];
    $ldap_server=~ s/^ldap:\/\/([^:]+).*$/$1/;

    open($ldap_shell_file, ">","$ldap_shell_config");
    print $ldap_shell_file "LDAP_BASE=\"$ldap_base\"\n";
    print $ldap_shell_file "LDAP_SERVER=\"$ldap_server\"\n";
    print $ldap_shell_file "LDAP_URIS=\"@ldap_uris\"\n";
    print $ldap_shell_file "ADMIN_BASE=\"$admin_base\"\n";
    print $ldap_shell_file "DEPARTMENT=\"$department\"\n";
    print $ldap_shell_file "RELEASE=\"$release\"\n";
    print $ldap_shell_file "UNIT_TAG=\"".(defined $unit_tag ? "$unit_tag" : "")."\"\n";
    print $ldap_shell_file "UNIT_TAG_FILTER=\"".(defined $unit_tag ? "(gosaUnitTag=$unit_tag)" : "")."\"\n";
    close($ldap_shell_file);

		# Set permissions and ownership structure of
		chown(0, 0, $ldap_shell_file);
		chmod(0644, $ldap_shell_file);
			
    daemon_log("INFO: Wrote $ldap_shell_config", 5);

    # Write offline config
    if ($offline_enabled){

	    # Get first LDAP server
	    open( $ldap_offline_file, ">","$ldap_offline_config");
	    print $ldap_offline_file "LDAP_BASE=\"$ldap_base\"\n";
	    print $ldap_offline_file "LDAP_SERVER=\"127.0.0.1\"\n";
	    print $ldap_offline_file "LDAP_URIS=\"ldap://127.0.0.1\"\n";
	    print $ldap_offline_file "ADMIN_BASE=\"$admin_base\"\n";
	    print $ldap_offline_file "DEPARTMENT=\"$department\"\n";
	    print $ldap_offline_file "RELEASE=\"$release\"\n";
	    print $ldap_offline_file "UNIT_TAG=\"".(defined $unit_tag ? "$unit_tag" : "")."\"\n";
	    print $ldap_offline_file "UNIT_TAG_FILTER=\"".(defined $unit_tag ? "(gosaUnitTag=$unit_tag)" : "")."\"\n";
	    close($ldap_offline_file);

			# Set permissions and ownership structure of
			chown(0, 0, $ldap_offline_file);
			chmod(0644, $ldap_offline_file);
			
	    daemon_log("INFO: Wrote $ldap_offline_config", 5);
    }



    # Allow custom scripts to be executed
    if (defined $ldap_config_exit_hook) {
        system($ldap_config_exit_hook);
        daemon_log("Hook $ldap_config_exit_hook returned with code ".($? >> 8), 5);
    }

    return;
}


sub new_key {
	# Create new key
    my $new_server_key = &main::create_passwd();

	# Send new_key message to server
    my $errSend = &main::send_msg_hash_to_target(
		&main::create_xml_hash("new_key", $main::client_address, $main::server_address, $new_server_key),
		$main::server_address, 
		$main::server_key,
	);

	# Set global key
	if (not $errSend) {
		$main::server_key = $new_server_key;
	}

  return;
}


sub confirm_new_key {
    my ($msg, $msg_hash) = @_ ;
    my $source = @{$msg_hash->{'source'}}[0];

    &main::daemon_log("confirm new key from $source", 5);
    return;

}


sub detect_hardware {

    &write_to_file('goto-hardware-detection-start', $fai_logpath);

	my $hwinfo= `which hwinfo`;
	chomp $hwinfo;

	if (!(defined($hwinfo) && length($hwinfo) > 0)) {
		&main::daemon_log("ERROR: hwinfo was not found in \$PATH! Hardware detection will not work!", 1);
		return;
	}

	my $result= {
		gotoHardwareChecksum => &main::generate_hw_digest(),
		macAddress      => $client_mac_address,
		gotoXMonitor    => "",
		gotoXDriver     => "",
		gotoXMouseType  => "",
		gotoXMouseport  => "",
		gotoXkbModel    => "",
		gotoXHsync      => "",
		gotoXVsync      => "",
		gotoXResolution => "",
		ghUsbSupport    => "",
		gotoSndModule   => "",
		ghGfxAdapter    => "",
		ghNetNic        => "",
		ghSoundAdapter  => "",
		ghMemSize       => "",
		ghCpuType       => "",
		gotoModules     => [],
		ghIdeDev        => [],
		ghScsiDev       => [],
	};

	&main::daemon_log("Starting hardware detection", 4);
	my $gfxcard= `$hwinfo --gfxcard`;
	my $primary_adapter= $1 if $gfxcard =~ /^Primary display adapter:\s#(\d+)\n/m;
	if(defined($primary_adapter)) {
		($result->{ghGfxAdapter}, $result->{gotoXDriver}) = ($1,$2) if 
			$gfxcard =~ /$primary_adapter:.*?Model:\s\"([^\"]*)\".*?Server Module:\s(\w*).*?\n\n/s;
	}
	my $monitor= `$hwinfo --monitor`;
	my $primary_monitor= $1 if $monitor =~ /^(\d*):.*/m;
	if(defined($primary_monitor)) {
		($result->{gotoXMonitor}, $result->{gotoXResolution}, $result->{gotoXVsync}, $result->{gotoXHsync})= ($1,$2,$3,$4) if 
		$monitor =~ /$primary_monitor:\s.*?Model:\s\"(.*?)\".*?Max\.\sResolution:\s([0-9x]*).*?Vert\.\sSync\sRange:\s([\d\-]*)\sHz.*?Hor\.\sSync\sRange:\s([\d\-]*)\skHz.*/s;
	}

	if(length($result->{gotoXHsync}) == 0) {
		# set default values
		$result->{gotoXHsync} = "30+50";
		$result->{gotoXVsync} = "30+90";
	}

	my $mouse= `$hwinfo --mouse`;
	my $primary_mouse= $1 if $mouse =~ /^(\d*):.*/m;
	if(defined($primary_mouse)) {
		($result->{gotoXMouseport}, $result->{gotoXMouseType}) = ($1,$2) if
		$mouse =~ /$primary_mouse:\s.*?Device\sFile:\s(.*?)\s.*?XFree86\sProtocol:\s(.*?)\n.*?/s;
	}

	my $sound= `$hwinfo --sound`;
	my $primary_sound= $1 if $sound =~ /^(\d*):.*/m;
	if(defined($primary_sound)) {
		($result->{ghSoundAdapter}, $result->{gotoSndModule})= ($1,$2) if 
		$sound =~ /$primary_sound:\s.*?Model:\s\"(.*?)\".*?Driver\sModules:\s\"(.*?)\".*/s;
	}

	my $netcard= `hwinfo --netcard`;
	my $primary_netcard= $1 if $netcard =~ /^(\d*):.*/m;
	if(defined($primary_netcard)) {
		$result->{ghNetNic}= $1 if $netcard =~ /$primary_netcard:\s.*?Model:\s\"(.*?)\".*/s;
	}

	my $keyboard= `hwinfo --keyboard`;
	my $primary_keyboard= $1 if $keyboard =~ /^(\d*):.*/m;
	if(defined($primary_keyboard)) {
		$result->{gotoXkbModel}= $1 if $keyboard =~ /$primary_keyboard:\s.*?XkbModel:\s(.*?)\n.*/s;
	}

	$result->{ghCpuType}= sprintf "%s / %s - %s", 
	`cat /proc/cpuinfo` =~ /.*?vendor_id\s+:\s(.*?)\n.*?model\sname\s+:\s(.*?)\n.*?cpu\sMHz\s+:\s(.*?)\n.*/s;
	$result->{ghMemSize}= $1 if `cat /proc/meminfo` =~ /^MemTotal:\s+(.*?)\skB.*/s;

	my @gotoModules=();
	for my $line(`lsmod`) {
		if (($line =~ /^Module.*$/) or ($line =~ /^snd.*$/)) {
			next;
		} else {
			push @gotoModules, $1 if $line =~ /^(\w*).*$/
		}
	}
	my %seen = ();
	
	# Remove duplicates and save
	push @{$result->{gotoModules}}, grep { ! $seen{$_} ++ } @gotoModules;

	$result->{ghUsbSupport} = (-d "/proc/bus/usb")?"true":"false";
	
	foreach my $device(`hwinfo --ide` =~ /^.*?Model:\s\"(.*?)\".*$/mg) {
		push @{$result->{ghIdeDev}}, $device;
	}

	foreach my $device(`hwinfo --scsi` =~ /^.*?Model:\s\"(.*?)\".*$/mg) {
		push @{$result->{ghScsiDev}}, $device;
	}

	&main::daemon_log("Hardware detection done!", 4);

    &write_to_file('goto-hardware-detection-stop', $fai_logpath);

	&main::send_msg_hash_to_target(
	&main::create_xml_hash("detected_hardware", $main::client_address, $main::server_address, $result),
	$main::server_address,
	$main::server_key,
	);

	return;
}


sub ping {
    my ($msg, $msg_hash) = @_ ;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $out_msg;
    my $out_hash;

    # there is no session_id so send 'got_new_ping'-msg
    if (not defined $session_id) {
        $out_hash = &main::create_xml_hash("got_new_ping", $target, $source);

    # there is a session_id so send 'answer_$session_id'-msg because there is 
    # a process waiting for this message
    } else {
        $out_hash = &main::create_xml_hash("answer_$session_id", $target, $source);
        &add_content2xml_hash($out_hash, "session_id", $session_id);
    }

    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }
    $out_msg = &main::create_xml_string($out_hash);
    return $out_msg;

}

1;
