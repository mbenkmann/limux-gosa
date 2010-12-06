package installation;

use strict;
use warnings;

use Exporter;
use Fcntl;

our @ISA = qw(Exporter);

my @events = qw(get_events set_activated_for_installation);

our @EXPORT = @events;

BEGIN {}

END {}

### Start ######################################################################


sub get_events {
    return \@events;
}

sub set_activated_for_installation {
    my ($msg, $msg_hash) = @_ ;
    my $header = @{$msg_hash->{'header'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];

    my $Datei = "/var/run/gosa-si/gosa-si-client.activated";
    open(my $FILE, ">", "$Datei");
    print $FILE "$msg\n";
    close($FILE);

    return;
}



1;
