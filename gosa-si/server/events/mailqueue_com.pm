=pod

=head1 NAME

mailqueue_com - Implementation of a GOsa-SI event module. 

=head1 SYNOPSIS

 use GOSA::GosaSupportDaemon;
 use Time::HiRes qw(usleep);
 use MIME::Base64;

=head1 DESCRIPTION

A GOsa-SI event module containing all functions used by GOsa mail queue. This module will
be automatically imported by GOsa-SI if it is under F</usr/lib/gosa-si/server/E<lt>PACKAGEMODULEE<gt>/> .


=head1 METHODS

=cut

package mailqueue_com;

use strict;
use warnings;

use Data::Dumper;
use Time::HiRes qw( usleep);
use MIME::Base64;
use GOsaSI::GosaSupportDaemon;

use Exporter;

our @ISA = qw(Exporter);

my @events = (
    "get_events",
    "mailqueue_query",
    "mailqueue_header",
);

our @EXPORT = @events;

BEGIN {}

END {}

### Start ######################################################################

=pod

=over 4

=item get_events ( )

Returns a list of functions which are exported by the module.

=back

=cut

sub get_events {
    return \@events;
}


=pod

=over 4

=item mailqueue_query( $msg, $msg_hash, $session_id )

This function do for incoming messages with header 'mailqueue_query' the target translation from mac address to ip:port address, updates job_queue, send message to client and wait for client answer.

Returns the answer of the client.

=back

=cut

sub mailqueue_query {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{header}}[0];
    my $target = @{$msg_hash->{target}}[0];
    my $source = @{$msg_hash->{source}}[0];
    my $jobdb_id = @{$msg_hash->{'jobdb_id'}}[0];
    my $error = 0;
    my $error_string;
    my $answer_msg;
    my ($sql, $res);

    if( defined $jobdb_id) {
        my $sql_statement = "UPDATE $main::job_queue_tn SET status='processed' WHERE id=$jobdb_id";
        &main::daemon_log("$session_id DEBUG: $sql_statement", 7); 
        my $res = $main::job_db->exec_statement($sql_statement);
    }

    # search for the correct target address
    $sql = "SELECT * FROM $main::known_clients_tn WHERE ((hostname='$target') || (macaddress LIKE '$target'))"; 
    $res = $main::known_clients_db->exec_statement($sql);
    my ($host_name, $host_key);  # sanity check of db result
    if ((defined $res) && (@$res > 0) && @{@$res[0]} > 0) {
        $host_name = @{@$res[0]}[0];
        $host_key = @{@$res[0]}[2];
    } else {
        &main::daemon_log("$session_id ERROR: cannot determine host_name and host_key from known_clients_db\n$msg", 1);
        $error_string = "Cannot determine host_name and host_key from known_clients_db";
        $error = 1;
    }

    # send message to target
    if (not $error) {
        $msg =~ s/<source>GOSA<\/source>/<source>$main::server_address<\/source>/g; 
        $msg =~ s/<\/xml>/<session_id>$session_id<\/session_id><\/xml>/;
        &main::send_msg_to_target($msg, $host_name, $host_key, $header, $session_id);
    }

    # waiting for answer
    if (not $error) {
        my $message_id;
        my $i = 0;
        while (1) {
            $i++;
            $sql = "SELECT * FROM $main::incoming_tn WHERE headertag='answer_$session_id'";
            $res = $main::incoming_db->exec_statement($sql);
            if (ref @$res[0] eq "ARRAY") { 
                $message_id = @{@$res[0]}[0];
                last;
            }
            if ($i > 100) { last; } # do not run into a endless loop
            usleep(100000);
        }
        # if answer exists
        if (defined $message_id) {
            $answer_msg = decode_base64(@{@$res[0]}[4]);
            $answer_msg =~ s/<target>\S+<\/target>/<target>$source<\/target>/;
            $answer_msg =~ s/<header>\S+<\/header>/<header>$header<\/header>/;

            my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
            if (defined $forward_to_gosa){
                $answer_msg =~s/<\/xml>/<forward_to_gosa>$forward_to_gosa<\/forward_to_gosa><\/xml>/;
            }
            $sql = "DELETE FROM $main::incoming_tn WHERE id=$message_id"; 
            $res = $main::incoming_db->exec_statement($sql);
        }
    }

    return ( $answer_msg );
}

=pod

=over 4

=item mailqueue_header ( $msg, $msg_hash, $session_id )

This function do for incoming messages with header 'mailqueue_header' the target translation from mac address to ip:port address, updates job_queue, send message to client and wait for client answer.

Returns the answer of the client.

=back

=cut
sub mailqueue_header {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{header}}[0];
    my $target = @{$msg_hash->{target}}[0];
    my $source = @{$msg_hash->{source}}[0];
    my $jobdb_id = @{$msg_hash->{'jobdb_id'}}[0];
    my $error = 0;
    my $error_string;
    my $answer_msg;
    my ($sql, $res);

    if( defined $jobdb_id) {
        my $sql_statement = "UPDATE $main::job_queue_tn SET status='processed' WHERE id=$jobdb_id";
        &main::daemon_log("$session_id DEBUG: $sql_statement", 7); 
        my $res = $main::job_db->exec_statement($sql_statement);
    }

    # search for the correct target address
    $sql = "SELECT * FROM $main::known_clients_tn WHERE ((hostname='$target') || (macaddress LIKE '$target'))"; 
    $res = $main::known_clients_db->exec_statement($sql);
    my ($host_name, $host_key);
    if ((defined $res) && (@$res > 0) && @{@$res[0]} > 0) {   # sanity check of db result
        $host_name = @{@$res[0]}[0];
        $host_key = @{@$res[0]}[2];
    } else {
        &main::daemon_log("$session_id ERROR: cannot determine host_name and host_key from known_clients_db\n$msg", 1);
        $error_string = "Cannot determine host_name and host_key from known_clients_db";
        $error = 1;
    }

    # send message to target
    if (not $error) {
        $msg =~ s/<source>GOSA<\/source>/<source>$main::server_address<\/source>/g; 
        $msg =~ s/<\/xml>/<session_id>$session_id<\/session_id><\/xml>/;
        &main::send_msg_to_target($msg, $host_name, $host_key, $header, $session_id);
    }

    # waiting for answer
    if (not $error) {
        my $message_id;
        my $i = 0;
        while (1) {
            $i++;
            $sql = "SELECT * FROM $main::incoming_tn WHERE headertag='answer_$session_id'";
            $res = $main::incoming_db->exec_statement($sql);
            if (ref @$res[0] eq "ARRAY") { 
                $message_id = @{@$res[0]}[0];
                last;
            }
            if ($i > 100) { last; } # do not run into a endless loop
            usleep(100000);
        }
        # if answer exists
        if (defined $message_id) {
            $answer_msg = decode_base64(@{@$res[0]}[4]);
            $answer_msg =~ s/<target>\S+<\/target>/<target>$source<\/target>/;
            $answer_msg =~ s/<header>\S+<\/header>/<header>$header<\/header>/;

            my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
            if (defined $forward_to_gosa){
                $answer_msg =~s/<\/xml>/<forward_to_gosa>$forward_to_gosa<\/forward_to_gosa><\/xml>/;
            }
            $sql = "DELETE FROM $main::incoming_tn WHERE id=$message_id"; 
            $res = $main::incoming_db->exec_statement($sql);
        }
    }

    return ( $answer_msg );
}


=pod

=head1 BUGS

Please report any bugs, or post any suggestions, to the GOsa mailing list E<lt>gosa-devel@oss.gonicus.deE<gt> or to L<https://oss.gonicus.de/labs/gosa>

=head1 COPYRIGHT

This code is part of GOsa (L<http://www.gosa-project.org>)

Copyright (C) 2003-2008 GONICUS GmbH

ID: $$Id$$

This program is free software; you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation; either version 2 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 59 Temple Place, Suite 330, Boston, MA  02111-1307  USA


=cut

# vim:ts=4:shiftwidth:expandtab
1;
