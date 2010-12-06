package load_reporter;


use strict;
use warnings;

use GOsaSI::GosaSupportDaemon;

use Exporter;

BEGIN {}
END {}

our @ISA = qw(Exporter);

my @events = (
    "get_events",
    "get_terminal_server",
    "get_load",
    "report_load",
    "set_terminal_server",
    );

our @EXPORT = @events;

my $ts_load_file;
my $waiting_for_ts_info;
my %cfg_defaults = (
			"client" => {
			"ts-load-file" => [\$ts_load_file, "/var/run/gosa-si/gosa-si-client-ts-load.txt"],
			"waiting-for-ts-info" => [\$waiting_for_ts_info, 5],
			},
		);

# to be removed ugly !! why not using main::_read_configfile
&GOsaSI::GosaSupportDaemon::read_configfile($main::cfg_file, %cfg_defaults);


### FUNCTIONS #################################################################
sub get_events { return \@events; }

sub get_terminal_server 
{
	my ($content, $poe_kernel) = @_ ;

	# Create message
	my $msg_hash = &create_xml_hash("get_terminal_server", $main::client_address, $main::server_address);
	&add_content2xml_hash($msg_hash, "macaddress", $main::client_mac_address);
	my $msg = &create_xml_string($msg_hash);
	
	$poe_kernel->delay_set('trigger_set_terminal_server', $waiting_for_ts_info);
	&main::daemon_log("INFO: Start obtaining terminal server load information. Set waiting time to '$waiting_for_ts_info' sec.", 5);
	return $msg;
}

sub get_load 
{
	my ($msg, $msg_hash) = @_ ;	
	my $source = @{$msg_hash->{'source'}}[0];
	my $target = @{$msg_hash->{'target'}}[0];
	my $out_msg;


	my $file = "/proc/loadavg";
	if ((not -f $file) || (not -r $file)) { return }
	open(my $FHD, "<", "$file");
	my $line = <$FHD>;
	close($FHD);
	chomp($line);

	$out_msg = &create_xml_string(&create_xml_hash("report_load", $target, $source, $line));

	return $out_msg;
}

sub report_load
{
	my ($msg, $msg_hash) = @_ ;
	my $source = @{$msg_hash->{'source'}}[0];
	my $load = @{$msg_hash->{'report_load'}}[0];

	$main::terminal_server_hash->{$source} = $load;

	return;
}

sub set_terminal_server
{
	my $file_content = "";
	while (my ($ts, $load) = each %$main::terminal_server_hash)
	{
		$file_content .= "$ts $load\n";
	}
	open(my $FHD, ">", "$ts_load_file.part");
	printf $FHD $file_content;
	close($FHD);

	system("mv $ts_load_file.part $ts_load_file");
	&main::daemon_log("INFO: Wrote terminal server load information to $ts_load_file", 5);
	return;
}

1;
