package GosaPackages;

use strict;
use warnings;

use Exporter;
use GOsaSI::GosaSupportDaemon;
use IO::Socket::INET;
use Socket;
use XML::Simple;
use File::Spec;
use Data::Dumper;
use MIME::Base64;
use Net::ARP;

our @ISA = ("Exporter");

my $event_dir = "/usr/lib/gosa-si/server/GosaPackages";
use lib "/usr/lib/gosa-si/server/GosaPackages";

BEGIN{}
END{}

my $network_interface;
my $gosa_mac_address;

## START ##########################

$network_interface= &get_interface_for_ip($main::server_ip);
$gosa_mac_address= &get_mac($network_interface);

# complete addresses
if( inet_aton($main::server_ip) ){ $main::server_ip = inet_ntoa(inet_aton($main::server_ip)); } 
$main::server_address = $main::server_ip.":".$main::server_port;



# import local events
my ($error, $result, $event_hash) = &import_events($event_dir);
foreach my $log_line (@$result) {
    if ($log_line =~ / succeed: /) {
        &main::daemon_log("0 INFO: GosaPackages - $log_line", 5);
    } else {
        &main::daemon_log("0 ERROR: GosaPackages - $log_line", 1);
    }
}

# build vice versa event_hash, event_name => module
my $event2module_hash = {};
while (my ($module, $mod_events) = each %$event_hash) {
    while (my ($event_name, $nothing) = each %$mod_events) {
        $event2module_hash->{$event_name} = $module;
    }

}


## FUNCTIONS #################################################################

sub get_module_info {
    my @info = ($main::gosa_address,
                $main::gosa_passwd,
                $event_hash,
                );
    return \@info;
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
                # A configured MAC Address should always override a guessed value
                if ($gosa_mac_address and length($gosa_mac_address) > 0) {
                    $result= $gosa_mac_address;
                }

          $result = Net::ARP::get_mac($ifreq);
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
    my $header = @{$msg_hash->{header}}[0];
    my @msg_l;
    my @out_msg_l;

    &main::daemon_log("$session_id DEBUG: GosaPackages: msg to process '$header'", 26);
    
    if ($header =~ /^job_/) {
        @msg_l = &process_job_msg($msg, $msg_hash, $session_id);
    } 
    elsif ($header =~ /^gosa_/) {
        @msg_l = &process_gosa_msg($msg, $msg_hash, $session_id);
    } 
    else {
        &main::daemon_log("$session_id ERROR: $header is not a valid GosaPackage-header, need a 'job_' or a 'gosa_' prefix", 1);
    }

    foreach my $out_msg ( @msg_l ) {
        # determine the correct outgoing source address to the corresponding target address
        $out_msg =~ /<target>(\S*)<\/target>/;
        my $act_target = $1;
        $act_target =~ s/GOSA/$main::server_address/;
        my $act_server_ip = &main::get_local_ip_for_remote_ip(sprintf("%s", $act_target =~ /^([0-9\.]*?):.*$/));

        # Patch the correct outgoing source address
        if ($out_msg =~ /<source>GOSA<\/source>/ ) {
            $out_msg =~ s/<source>GOSA<\/source>/<source>$act_server_ip:$main::server_port<\/source>/g;
        }

        # Add to each outgoing message the current POE session id
        $out_msg =~ s/<\/xml>/<session_id>$session_id<\/session_id><\/xml>/;


        if (defined $out_msg){
            push(@out_msg_l, $out_msg);
        }
    }

    return \@out_msg_l;
}


sub process_gosa_msg {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $out_msg;
    my @out_msg_l = ('nohandler');
    my $sql_events;

    # strip gosa_ prefix from header, it is not used any more
    @{$msg_hash->{'header'}}[0] =~ s/gosa_//;
    $msg =~ s/<header>gosa_/<header>/;

    my $header = @{$msg_hash->{'header'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];

    # check local installed events
    if( exists $event2module_hash->{$header} ) {
        # a event exists with the header as name
        &main::daemon_log("$session_id DEBUG: found event '$header' at event-module '".$event2module_hash->{$header}."'", 26);
        no strict 'refs';
        @out_msg_l = &{$event2module_hash->{$header}."::$header"}( $msg, $msg_hash, $session_id );

    # check client registered events
    } else {
        $sql_events = "SELECT * FROM $main::known_clients_tn WHERE ( (macaddress LIKE '$target') OR (hostname='$target') )"; 
        my $res = $main::known_clients_db->select_dbentry( $sql_events );
        my $l = keys(%$res);
        
        # set error if no or more than 1 hits are found for sql query
        if ( $l != 1) {
            @out_msg_l = ('knownclienterror');
        # found exact 1 hit in db
        } else {
            my $client_events = $res->{'1'}->{'events'};

            # client is registered for this event, deliver this message to client
            if (($client_events =~ /^$header,/) || ($client_events =~ /,$header,/) || ($client_events =~ /,$header$/)) {
                &main::daemon_log("$session_id DEBUg: client '$target' is registerd for event '$header', forward message to client.", 26);
                @out_msg_l = ( $msg );

            # client is not registered for this event, set error
            } else {
                @out_msg_l = ('noeventerror');
            }
        }
    }

    # if delivery not possible raise error and return 
    if (not defined $out_msg_l[0]) {
        @out_msg_l = ();
    } elsif ($out_msg_l[0] eq 'nohandler') {
        &main::daemon_log("$session_id ERROR: GosaPackages: no event handler or core function defined for '$header'", 1);
        @out_msg_l = ();
    } elsif ($out_msg_l[0] eq 'knownclienterror') {
        if ($header eq "ping") {
            &main::daemon_log("$session_id WARNING: Cannot send '$header' to '$target'. GOsa-si do not know client. Maybe client is offline or gosa-si-client process is not running.", 3);
        } else {
            &main::daemon_log("$session_id ERROR: no general event handler found for '$header', check client registration events!", 1);
            &main::daemon_log("$session_id ERROR: no or more than 1 hits are found at known_clients_db with sql query: '$sql_events'", 1);
            &main::daemon_log("$session_id ERROR: processing is aborted and message will not be forwarded", 1);
        }
        @out_msg_l = ();
    } elsif ($out_msg_l[0] eq 'noeventerror') {
        &main::daemon_log("$session_id ERROR: no general event handler found for '$header', check client registration events!", 1);
        &main::daemon_log("$session_id ERROR: client '$target' is not registered for event '$header', processing is aborted", 1); 
        @out_msg_l = ();
    }

    return @out_msg_l;
}


sub process_job_msg {
    my ($msg, $msg_hash, $session_id)= @_ ;    
    my $out_msg;
    my $error = 0;

    my $header = @{$msg_hash->{'header'}}[0];
    $header =~ s/job_//;
	my $target = @{$msg_hash->{'target'}}[0];
    
    # If no timestamp is specified, use 19700101000000
    my $timestamp = "19700101000000";
    if( exists $msg_hash->{'timestamp'} ) {
        $timestamp = @{$msg_hash->{'timestamp'}}[0];
    }

    # If no macaddress is specified, raise error 
    my $macaddress;
    if( exists $msg_hash->{'macaddress'} ) {
        $macaddress = @{$msg_hash->{'macaddress'}}[0];
    } elsif (@{$msg_hash->{'target'}}[0] =~ /^([0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2})$/i ) {
        $macaddress = $1;
    } else {
        $error ++;
        $out_msg = "<xml>".
            "<header>answer</header>".
            "<source>$main::server_address</source>".
            "<target>GOSA</target>".
            "<answer1>1</answer1>".
            "<error_string>no mac address specified, neither in target-tag nor in macaddres-tag</error_string>".
            "</xml>";

        return ($out_msg);
    }
    
    # Determine plain_name for host
    my $plain_name;
    if ($header eq "opsi_install_client") {   # Opsi installing clients use hostId as plain_name
        if (not exists $msg_hash->{'hostId'}) {
            $error++;
            &daemon_log("$session_id ERROR: opsi_install_client-message has no xml-tag 'hostID', job was not created: $msg", 1);
        } else {
            $plain_name = $msg_hash->{'hostId'}[0];
            $header = "trigger_action_reinstall"
        }

    } else {   # Try to determine plain_name via ldap search
        my $ldap_handle=&main::get_ldap_handle();
        my $mesg = $ldap_handle->search(
                base => $main::ldap_base,
                scope => 'sub',
                attrs => ['cn'],
                filter => "(macAddress=$macaddress)");
        if($mesg->code || ($mesg->count!=1)) {
            &main::daemon_log($mesg->error, 1);
            $plain_name = "none";
        } else {
            my $entry= $mesg->entry(0);
            $plain_name = $entry->get_value("cn");
        }
        &main::release_ldap_handle($ldap_handle);
    }
	
    # Check if it is a periodical job
    my $periodic = 'none';
    if (exists $msg_hash->{periodic})
    {
        $periodic = $msg_hash->{periodic}[0];
        if ($periodic ne 'none' and not $periodic =~ /[0-9]+_(hours|minutes|days|weeks|months)/)    # Periodic tag is not valid
        {
            &main::daemon_log("$session_id ERROR: Message contains invalid periodic-tag '$periodic'.".
                    " Please use the following pattern for the tag: 'INTEGER_[minutes|hours|days|weeks|months]'".
                    " Aborted message: $msg", 1);
            $out_msg = "<xml>".
                "<header>answer</header><source>$main::server_address</source><target>GOSA</target>".
                "<answer1>1</answer1><error_string>Message contains invalid periodic-tag '$periodic'</error_string>".
                "</xml>";
            return ($out_msg);
        }
    }

    # Add job to job queue
    if( $error == 0 ) {
        my $func_dic = {table=>$main::job_queue_tn, 
            primkey=>['macaddress', 'headertag'],
            timestamp=>$timestamp,
            status=>'waiting', 
            result=>'none',
            progress=>'none',
            headertag=>$header, 
            targettag=>$target,
            xmlmessage=>$msg,
            macaddress=>$macaddress,
			plainname=>$plain_name,
            siserver=>"localhost",
            modified=>"1",
            periodic=>$periodic,
        };
        my $res = $main::job_db->add_dbentry($func_dic);
        if (not $res == 0) {
            &main::daemon_log("$session_id ERROR: GosaPackages: process_job_msg: $res", 1);
        } else {
            &main::daemon_log("$session_id INFO: GosaPackages: $header job successfully added to job queue", 5);
        }
        $out_msg = "<xml><header>answer</header><source>$main::server_address</source><target>GOSA</target><answer1>$res</answer1></xml>";
    }
    
    my @out_msg_l = ( $out_msg );
    return @out_msg_l;
}

# vim:ts=4:shiftwidth:expandtab
1;
