package ServerPackages;



# Each module has to have a function 'process_incoming_msg'. This function works as a interface to gosa-sd and receives the msg hash from gosa-sd. 'process_incoming_function checks, wether it has a function to process the incoming msg and forward the msg to it. 

use strict;
use warnings;

use Data::Dumper;
use GOsaSI::GosaSupportDaemon;

use Exporter;

our @ISA = ("Exporter");

my $event_dir = "/usr/lib/gosa-si/server/ServerPackages";
use lib "/usr/lib/gosa-si/server/ServerPackages";

BEGIN{}
END {}


### START #####################################################################

# import local events
my ($error, $result, $event_hash) = &import_events($event_dir);
foreach my $log_line (@$result) {
    if ($log_line =~ / succeed: /) {
        &main::daemon_log("0 INFO: ServerPackages - $log_line", 5);
    } else {
        &main::daemon_log("0 ERROR: ServerPackages - $log_line", 1);
    }
}

# build vice versa event_hash, event_name => module
my $event2module_hash = {};
while (my ($module, $mod_events) = each %$event_hash) {
    while (my ($event_name, $nothing) = each %$mod_events) {
        $event2module_hash->{$event_name} = $module;
    }

}

### FUNCTIONS #####################################################################

sub get_module_info {
    my @info = ($main::server_address,
            $main::ServerPackages_key, 
            $event_hash,
            );
    return \@info;
}

sub process_incoming_msg {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{header}}[0];
    my $source = @{$msg_hash->{source}}[0]; 
    my $target = @{$msg_hash->{target}}[0];
    my $sql_events;

    my @msg_l;
    my @out_msg_l = ( 'nohandler' );


    # if message is being forwarded from another server, strip of header prefixes
    $header =~ s/^gosa_|^job_//;
    $msg =~ s/<header>gosa_(\w+)<\/header>|<header>job_(\w+)<\/header>/<header>$1<\/header>/;
    

    &main::daemon_log("$session_id DEBUG: ServerPackages: msg to process '$header'", 26);
    if( exists $event2module_hash->{$header} ) {
        # a event exists with the header as name
        &main::daemon_log("$session_id INFO: found event '$header' at event-module '".$event2module_hash->{$header}."'", 26);
        no strict 'refs';
        @out_msg_l = &{$event2module_hash->{$header}."::$header"}($msg, $msg_hash, $session_id);

    } else {
        $sql_events = "SELECT * FROM $main::known_clients_tn WHERE ( (macaddress LIKE '$target') OR (hostname='$target') )"; 
        my $res = $main::known_clients_db->select_dbentry( $sql_events );
        my $l = keys(%$res);


# TODO
# $l == 1, knownclienterror wird eigentlich nicht gebraucht. hier soll nohandler anspringen
        # set error if no or more than 1 hits are found for sql query
        if ( $l != 1) {
            @out_msg_l = ('knownclienterror');
        
        # found exact 1 hit in db
        } else {
            my $client_events = $res->{'1'}->{'events'};

            # client is registered for this event, deliver this message to client
            if ($client_events =~ /,$header,/) {
                $msg =~ s/<header>gosa_/<header>/;
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
        &main::daemon_log("$session_id ERROR: ServerPackages: no event handler defined for '$header'", 1);
        @out_msg_l = ();
    } elsif ($out_msg_l[0] eq 'knownclienterror') {
        &main::daemon_log("$session_id ERROR: no or more than 1 hits are found at known_clients_db with sql query: '$sql_events'", 1);
        &main::daemon_log("$session_id ERROR: processing is aborted and message will not be forwarded", 1);
        @out_msg_l = ();
    } elsif ($out_msg_l[0] eq 'noeventerror') {
        &main::daemon_log("$session_id ERROR: client '$target' is not registered for event '$header', processing is aborted", 1); 
        @out_msg_l = ();
    }
      
    return \@out_msg_l;
}


1;
