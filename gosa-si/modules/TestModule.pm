package TestModule;


use strict;
use warnings;

use GOsaSI::GOsaSupportDaemon;

use Exporter;

our @ISA = ("Exporter");

BEGIN{
}

END{}

### START ##########


sub get_module_tags {
    
    # lese config file aus dort gibt es eine section Basic
    # dort stehen drei packettypen, für die sich das modul anmelden kann, gosa-admin-packages, 
    #   server-packages, client-packages
    my %tag_hash = (gosa_admin_packages => "yes", 
                    server_packages => "yes", 
                    client_packages => "yes",
                    );
    return \%tag_hash;
}


sub process_incoming_msg {
    my ($crypted_msg) = @_ ;
    if(not defined $crypted_msg) {
        &main::daemon_log("function 'process_incoming_msg': got no msg", 7);
    }
    &main::daemon_log("TestModule: crypted_msg:$crypted_msg", 7);
    &main::daemon_log("TestModule: crypted_msg len:".length($crypted_msg), 7);


    # chomp address from host who send the message
    $crypted_msg =~ /^([\s\S]*?)\.(\d{1,3}?)\.(\d{1,3}?)\.(\d{1,3}?)\.(\d{1,3}?)$/;
    $crypted_msg = $1;
    my $host = sprintf("%s.%s.%s.%s", $2, $3, $4, $5);

    my $gosa_passwd = $main::gosa_passwd;
    my $gosa_cipher = &create_ciphering($gosa_passwd);    

    my $in_msg;
    my $in_hash;
    eval{
        $in_msg = &decrypt_msg($crypted_msg, $gosa_cipher);
        $in_hash = &transform_msg2hash($in_msg);
    };
    if ($@) {
        &main::daemon_log("TestModul konnte msg nicht entschlüsseln:", 5);
        &main::daemon_log("$@", 7);
        return;
    }

    my $header = @{$in_hash->{header}}[0];
    my $ip_address = @{$in_hash->{target}}[0];


    # hier kommt die logik suche den entsprechenden daemon, der den client target hat

    my $out_hash = &create_xml_hash("halt", $main::server_address, $ip_address);

    &send_msg_hash2address($out_hash, $ip_address);

    &main::daemon_log("TestModul: ip $ip_address bekommt $header ");
    return ;
}




