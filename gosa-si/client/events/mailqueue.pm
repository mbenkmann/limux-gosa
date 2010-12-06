
=head1 NAME

mailqueue.pm

=head1 SYNOPSIS

use GOSA::GosaSupportDaemon;

=head1 DESCRIPTION

This module contains all GOsa-SI-client processing instructions concerning the mailqueue in GOsa.

=head1 VERSION

Version 1.0

=head1 AUTHOR

Andreas Rettenberger <rettenberger at gonicus dot de>

=head1 FUNCTIONS

=cut


package mailqueue;


use strict;
use warnings;

use MIME::Base64;
use GOsaSI::GosaSupportDaemon;

use Exporter;

our @ISA = qw(Exporter);

my @events = (
    "get_events",
    "mailqueue_query",
    "mailqueue_hold",
    "mailqueue_unhold",
    "mailqueue_requeue",
    "mailqueue_del",
    "mailqueue_header",
    );

our @EXPORT = @events;

BEGIN {}

END {}


###############################################################################
=over 

=item B<get_events ()>

=over

=item description 

    Reports all provided functions.

=item parameter

    None.

=item return 

    \@events - ARRAYREF - array containing all functions 

=back

=back

=cut
###############################################################################
sub get_events { return \@events; }


###############################################################################
=over 

=item B<mailqueue_query ($$)>

=over

=item description 

    Executes /usr/sbin/mailq, parse the informations and return them

=item parameter

    $msg - STRING - complete GOsa-si message
    $msg_hash - HASHREF - content of GOsa-si message in a hash

=item GOsa-si message xml content

    None.

=item return 

    $out_msg - STRING - GOsa-SI valid xml message containing msg_id, msg_hold, msg_size, arrival_time, sender and recipient.

=back

=back

=cut
###############################################################################
sub mailqueue_query {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    # q_tag can be: msg_id | msg_hold | msg_size | arrival_time | sender | recipient
    my $q_tag = exists $msg_hash->{'q_tag'} ? @{$msg_hash->{'q_tag'}}[0] : undef ;
    # q_operator can be: eq | gt | lt
    my $q_operator = exists $msg_hash->{'q_operator'} ? @{$msg_hash->{'q_operator'}}[0] : undef ;
    my $q_value = exists $msg_hash->{'q_value'} ? @{$msg_hash->{'q_value'}}[0] : undef ;
    my $error = 0;
    my $error_string;
    my $msg_id;
#my $msg_hold;
#my $msg_size;
#my $arrival_time;
    my $sender;
    my $recipient;
#my $status_message;
    my $out_hash;
    my $out_msg;

    &main::daemon_log("DEBUG: run /usr/bin/mailq\n", 7); 
    my $result = qx("/usr/bin/mailq");
    my @result_l = split(/([0-9A-Z]{10,12})/, $result);

    if (length($result) == 0) {
        $error = 1;
        $error_string = "/usr/bin/mailq has no result";
        &main::daemon_log("ERROR: $error_string : $msg", 1);
    }

    my $result_collection = {};
    if (not $error) {
        # parse information
        my $result_length = @result_l;
        my $j = 0;
        for (my $i = 1; $i < $result_length; $i+=2) {

            # Fetch and prepare all information 
            my $act_result;
            $act_result->{'msg_id'} = $result_l[$i];
            $result_l[$i+1] =~ /^([\!| ])\s+(\d+)\s+(\w{3}\s+\w{3}\s+\d+\s+\d+:\d+:\d+)\s+([\w.-]+@[\w.-]+)\s+/ ;
            $act_result->{'msg_hold'} =  $1 eq "!" ? 1 : 0 ;
            $act_result->{'msg_size'} = $2;
            $act_result->{'arrival_time'} = $3;
            $act_result->{'sender'} = $4;
            my @info_l = split(/\n/, $result_l[$i+1]);
            $act_result->{'recipient'} = $info_l[2] =~ /([\w.-]+@[\w.-]+)/ ? $1 : 'unknown' ;
            $act_result->{'msg_status'} = $info_l[1] =~ /^([\s\S]*)$/ ? $1 : 'unknown' ;

            # If all query tags exists, perform the selection
            my $query_positiv = 0;
            if (defined $q_tag && defined $q_operator && defined $q_value) {

                # Query for message id
                if ( $q_tag eq 'msg_id') {
                    if (not $q_operator eq 'eq') {
                        &main::daemon_log("$session_id WARNING: query option '$q_operator' is not allowed with query tag '$q_tag'".
                                ", return return complete mail queue as fallback", 3);
                        &main::daemon_log("$session_id DEBUG: \n$msg", 9); 
                        $query_positiv++;
                    } else {
                        if ( &_exec_op($act_result->{'msg_id'}, $q_operator, $q_value) ) { 
                            $query_positiv++; 
                        }
                    }

                # Query for message size
                } elsif ($q_tag eq 'msg_size') {
                    my $result_size = int($act_result->{'msg_size'});
                    my $query_size = int($q_value);
                    if ( &_exec_op($result_size, $q_operator, $query_size) ) {
                        $query_positiv++;
                    }

                # Query for arrival time
                } elsif ($q_tag eq 'arrival_time') {
                    my $result_time = int(&_parse_mailq_time($act_result->{'arrival_time'}));
                    my $query_time = int($q_value);

                    if ( &_exec_op($result_time, $q_operator, $query_time) ) {
                        $query_positiv++;
                    }

                # Query for sender
                }elsif ($q_tag eq 'sender') {
                    if (not $q_operator eq 'eq') {
                        &main::daemon_log("$session_id WARNING: query option '$q_operator' is not allowed with query tag '$q_tag'".
                                ", return return complete mail queue as fallback", 3);
                        &main::daemon_log("$session_id DEBUG: \n$msg", 9); 
                        $query_positiv++;
                    } else {
                        if ( &_exec_op($act_result->{'sender'}, $q_operator, $q_value)) { 
                            $query_positiv++; 
                        }
                    }

                # Query for recipient
                } elsif ($q_tag eq 'recipient') {
                    if (not $q_operator eq 'eq') {
                        &main::daemon_log("$session_id WARNING: query option '$q_operator' is not allowed with query tag '$q_tag'".
                                ", return return complete mail queue as fallback", 3);
                        &main::daemon_log("$session_id DEBUG: \n$msg", 9); 
                        $query_positiv++;
                    } else {
                        if ( &_exec_op($act_result->{'recipient'}, $q_operator, $q_value)) { 
                            $query_positiv++; 
                        }
                    }
                }
            
            # If no query tag exists, return all mails in mailqueue
            } elsif ((not defined $q_tag) && (not defined $q_operator) && (not defined $q_value)) {
                $query_positiv++; 

            # If query tags are not complete return error message
            } elsif ((not defined $q_tag) || (not defined $q_operator) || (not defined $q_value)) {
                $error++;
                $error_string = "'mailqueue_query'-msg is not complete, some query tags (q_tag, q_operator, q_value) are missing";
                &main::daemon_log("$session_id WARNING: $error_string", 3);
            }           

            # If query was successful, add results to answer
            if ($query_positiv) {
                $j++;
		foreach my $key (keys %{ $act_result }) {
			$act_result->{$key} =~ s/\</\&lt\;/g;
			$act_result->{$key} =~ s/\>/\&gt\;/g;
		}
                $result_collection->{$j} = $act_result;    
            }
        }
    }

    #create outgoing msg
    $out_hash = &main::create_xml_hash("answer_$session_id", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);
    &add_content2xml_hash($out_hash, "error", $error);
    if (defined @{$msg_hash->{'forward_to_gosa'}}[0]){
        &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]);
    }

    # add error infos to outgoing msg
    if ($error) {
        &add_content2xml_hash($out_hash, "error_string", $error_string);
        $out_msg = &main::create_xml_string($out_hash);

    # add mail infos to outgoing msg
    } else {
        my $collection_string = &db_res2xml($result_collection);
        $out_msg = &main::create_xml_string($out_hash);
        $out_msg =~ s/<\/xml>/$collection_string<\/xml>/
    }
    
    return $out_msg;

}


###############################################################################
=over 

=item B<mailqueue_hold ($$)>

=over

=item description 

    Executes '/usr/sbin/postsuper -h' and set mail to hold. 

=item parameter

    $msg - STRING - complete GOsa-si message
    $msg_hash - HASHREF - content of GOsa-si message in a hash

=item GOsa-si message xml content

    <msg_id> - STRING - postfix mail id

=item return 

    Nothing.

=back

=back

=cut
###############################################################################
sub mailqueue_hold {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $error = 0;
    my $error_string;

    # sanity check of input
    if (not exists $msg_hash->{'msg_id'}) {
        $error_string = "Message doesn't contain a XML tag 'msg_id"; 
        &main::daemon_log("ERROR: $error_string : $msg", 1);
        $error = 1;
    } elsif (ref @{$msg_hash->{'msg_id'}}[0] eq "HASH") { 
        $error_string = "XML tag 'msg_id' is empty";
        &main::daemon_log("ERROR: $error_string : $msg", 1);
        $error = 1;
    }

    if (not $error) {
        my @msg_ids = @{$msg_hash->{'msg_id'}};
        foreach my $msg_id (@msg_ids) {
            my $error = 0;   # clear error status

            # sanity check of each msg_id
            if (not $msg_id =~ /^[0-9A-Z]{10,12}$/) {
                $error = 1;
                $error_string = "message ID is not valid ([0-9A-Z]{10,12}) : $msg_id";
                &main::daemon_log("ERROR: $error_string : $msg", 1);
            }

            if (not $error) {
                my $cmd = "/usr/sbin/postsuper -h $msg_id 2>&1";
                &main::daemon_log("DEBUG: run $cmd", 7); 
                my $result = qx($cmd);
                if ($result =~ /^postsuper: ([0-9A-Z]{10}): placed on hold/ ) {
                    &main::daemon_log("INFO: Mail $msg_id placed on hold", 5);
                } elsif ($result eq "") {
                    &main::daemon_log("INFO: Mail $msg_id is alread placed on hold", 5);
                
                } else {
                    &main::daemon_log("ERROR: '$cmd' failed : $result", 1); 
                }
            }
        }
    }

    return;
}

###############################################################################
=over 

=item B<mailqueue_unhold ($$)>

=over

=item description 

    Executes '/usr/sbin/postsuper -H' and set mail to unhold. 

=item parameter

    $msg - STRING - complete GOsa-si message
    $msg_hash - HASHREF - content of GOsa-si message in a hash

=item GOsa-si message xml content

    <msg_id> - STRING - postfix mail id

=item return 

Nothing.

=back

=back

=cut
###############################################################################
sub mailqueue_unhold {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $error = 0;
    my $error_string;
    
    # sanity check of input
    if (not exists $msg_hash->{'msg_id'}) {
        $error_string = "Message doesn't contain a XML tag 'msg_id'"; 
        &main::daemon_log("ERROR: $error_string : $msg", 1);
        $error = 1;
    } elsif (ref @{$msg_hash->{'msg_id'}}[0] eq "HASH") { 
        $error_string = "XML tag 'msg_id' is empty";
        &main::daemon_log("ERROR: $error_string : $msg", 1);
        $error = 1;
    }
        
    if (not $error) {
        my @msg_ids = @{$msg_hash->{'msg_id'}};
        foreach my $msg_id (@msg_ids) {
            my $error = 0;   # clear error status

            # sanity check of each msg_id
            if (not $msg_id =~ /^[0-9A-Z]{10,12}$/) {
                $error = 1;
                $error_string = "message ID is not valid ([0-9A-Z]{10,12}) : $msg_id";
                &main::daemon_log("ERROR: $error_string : $msg", 1);
            }

            if (not $error) {
                my $cmd = "/usr/sbin/postsuper -H $msg_id 2>&1";
                &main::daemon_log("DEBUG: run $cmd\n", 7); 
                my $result = qx($cmd);
                if ($result =~ /^postsuper: ([0-9A-Z]{10}): released from hold/ ) {
                    &main::daemon_log("INFO: Mail $msg_id released from on hold", 5);
                } elsif ($result eq "") {
                    &main::daemon_log("INFO: Mail $msg_id is alread released from hold", 5);

                } else {
                    &main::daemon_log("ERROR: '$cmd' failed : $result", 1); 
                }

            }
        }
    }

    return;
}

###############################################################################
=over 

=item B<mailqueue_requeue ($$)>

=over

=item description 

    Executes '/usr/sbin/postsuper -r' and requeue the mail.

=item parameter

    $msg - STRING - complete GOsa-si message
    $msg_hash - HASHREF - content of GOsa-si message in a hash

=item GOsa-si message xml content

    <msg_id> - STRING - postfix mail id

=item return 

Nothing.

=back

=back

=cut
###############################################################################
sub mailqueue_requeue {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my @msg_ids = @{$msg_hash->{'msg_id'}};
    my $error = 0;
    my $error_string;  

    # sanity check of input
    if (not exists $msg_hash->{'msg_id'}) {
        $error_string = "Message doesn't contain a XML tag 'msg_id'"; 
        &main::daemon_log("ERROR: $error_string : $msg", 1);
        $error = 1;
    } elsif (ref @{$msg_hash->{'msg_id'}}[0] eq "HASH") { 
        $error_string = "XML tag 'msg_id' is empty";
        &main::daemon_log("ERROR: $error_string : $msg", 1);
        $error = 1;
    }
        
    if (not $error) {
        my @msg_ids = @{$msg_hash->{'msg_id'}};
        foreach my $msg_id (@msg_ids) {
            my $error = 0;   # clear error status

            # sanity check of each msg_id
            if (not $msg_id =~ /^[0-9A-Z]{10,12}$/) {
                $error = 1;
                $error_string = "message ID is not valid ([0-9A-Z]{10,12}) : $msg_id";
                &main::daemon_log("ERROR: $error_string : $msg", 1);
            }

            if (not $error) {
                my $cmd = "/usr/sbin/postsuper -r $msg_id 2>&1";
                &main::daemon_log("DEBUG: run '$cmd'", 7); 
                my $result = qx($cmd);
                if ($result =~ /^postsuper: ([0-9A-Z]{10}): requeued/ ) {
                    &main::daemon_log("INFO: Mail $msg_id requeued", 5);
                } elsif ($result eq "") {
                    &main::daemon_log("WARNING: Cannot requeue mail '$msg_id', mail not found!", 3);

                } else {
                    &main::daemon_log("ERROR: '$cmd' failed : $result", 1); 
                }

            }
        }
    }

    return;
}


###############################################################################
=over 

=item B<mailqueue_del ($$)>

=over

=item description 

    Executes '/usr/sbin/postsuper -d' and deletes mail from queue.

=item parameter

    $msg - STRING - complete GOsa-si message
    $msg_hash - HASHREF - content of GOsa-si message in a hash

=item GOsa-si message xml content

    <msg_id> - STRING - postfix mail id

=item return 

Nothing.

=back

=back

=cut
###############################################################################
sub mailqueue_del {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my @msg_ids = @{$msg_hash->{'msg_id'}};
    my $error = 0;
    my $error_string;

    # sanity check of input
    if (not exists $msg_hash->{'msg_id'}) {
        $error_string = "Message doesn't contain a XML tag 'msg_id'"; 
        &main::daemon_log("ERROR: $error_string : $msg", 1);
        $error = 1;
    } elsif (ref @{$msg_hash->{'msg_id'}}[0] eq "HASH") { 
        $error_string = "XML tag 'msg_id' is empty";
        &main::daemon_log("ERROR: $error_string : $msg", 1);
        $error = 1;
    }
        
    if (not $error) {
        my @msg_ids = @{$msg_hash->{'msg_id'}};
        foreach my $msg_id (@msg_ids) {
            my $error = 0;   # clear error status

            # sanity check of each msg_id
            if (not $msg_id =~ /^[0-9A-Z]{10,12}$/) {
                $error = 1;
                $error_string = "message ID is not valid ([0-9A-Z]{10,12}) : $msg_id";
                &main::daemon_log("ERROR: $error_string : $msg", 1);
            }

            if (not $error) {
                my $cmd = "/usr/sbin/postsuper -d $msg_id 2>&1";
                &main::daemon_log("DEBUG: run '$cmd'", 7); 
                my $result = qx($cmd);
                if ($result =~ /^postsuper: ([0-9A-Z]{10}): removed/ ) {
                    &main::daemon_log("INFO: Mail $msg_id deleted", 5);
                } elsif ($result eq "") {
                    &main::daemon_log("WARNING: Cannot remove mail '$msg_id', mail not found!", 3);

                } else {
                    &main::daemon_log("ERROR: '$cmd' failed : $result", 1); 
                }

            }
        }
    }

    return;
}

###############################################################################
=over 

=item B<mailqueue_header ($$)>

=over

=item description 

    Executes 'postcat -q', parse the informations and return them. 

=item parameter

    $msg - STRING - complete GOsa-si message
    $msg_hash - HASHREF - content of GOsa-si message in a hash

=item GOsa-si message xml content

    <msg_id> - STRING - postfix mail id

=item return 

    $out_msg - STRING - GOsa-si valid xml message containing recipient, sender and subject.

=back

=back

=cut
###############################################################################
sub mailqueue_header {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $error = 0;
    my $error_string;
    my $sender;
    my $recipient;
    my $subject;
    my $out_hash;
    my $out_msg;

    # sanity check of input
    if (not exists $msg_hash->{'msg_id'}) {
        $error_string = "Message doesn't contain a XML tag 'msg_id'"; 
        &main::daemon_log("ERROR: $error_string : $msg", 1);
        $error = 1;
    } elsif (ref @{$msg_hash->{'msg_id'}}[0] eq "HASH") { 
        $error_string = "XML tag 'msg_id' is empty";
        &main::daemon_log("ERROR: $error_string : $msg", 1);
        $error = 1;
    }

    # sanity check of each msg_id
    my $msg_id;
    if (not $error) {
        $msg_id = @{$msg_hash->{'msg_id'}}[0];
        if (not $msg_id =~ /^[0-9A-Z]{10,12}$/) {
            $error = 1;
            $error_string = "message ID is not valid ([0-9A-Z]{10,12}) : $msg_id";
            &main::daemon_log("ERROR: $error_string : $msg", 1);
        }
    }

    # parsing information
    my $msg_header;
    if (not $error) {
        my $cmd = "postcat -q $msg_id";
        &main::daemon_log("DEBUG: run '$cmd'", 7); 
        my $result = qx($cmd);

        my @header_l = split(/\n\n/, $result);
        $msg_header = $header_l[0];
    }       

    # create outgoing msg
    $out_hash = &main::create_xml_hash("answer_$session_id", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);
    &add_content2xml_hash($out_hash, "error", $error);
    if (defined @{$msg_hash->{'forward_to_gosa'}}[0]){
        &add_content2xml_hash($out_hash, "forward_to_gosa", @{$msg_hash->{'forward_to_gosa'}}[0]);
    }

    # add error infos to outgoing msg
    if ($error) {
        &add_content2xml_hash($out_hash, "error_string", $error_string);
        $out_msg = &main::create_xml_string($out_hash);

    # add mail infos to outgoing msg
    } else {
        #&add_content2xml_hash($out_hash, "msg_header", &decode_base64($msg_header));        
        &add_content2xml_hash($out_hash, "msg_header", $msg_header);        
        $out_msg = &main::create_xml_string($out_hash);
    }
 
    return $out_msg;
}

sub _exec_op {
    my ($a, $op, $b) = @_ ;
    my $res;

    if ($op eq "eq") {
        $res = $a =~ /$b/ ? 1 : 0 ;
    } elsif ($op eq "gt") {
        $res = $a > $b ? 1 : 0 ;
    } elsif ($op eq "lt") {
        $res = $a < $b ? 1 : 0 ;
    } 

    return $res;
}

my $mo_hash = { "Jan"=>'01', "Feb"=>'02',"Mar"=>'03',"Apr"=>'04',"May"=>'05',"Jun"=>'06',
    "Jul"=>'07',"Aug"=>'08',"Sep"=>'09',"Oct"=>'10',"Nov"=>'11',"Dec"=>'12'};

sub _parse_mailq_time {
    my ($time) = @_ ;

    my $local_time = &get_time();
    my $local_year = substr($local_time,0,4);     

    my ($dow, $mo, $dd, $date) = split(/\s/, $time);
    my ($hh, $mi, $ss) = split(/:/, $date);
    my $mailq_time = $local_year.$mo_hash->{$mo}."$dd$hh$mi$ss"; 

    # This is realy nasty
    if (int($local_time) < int($mailq_time)) {
        # Mailq_time is in the future, this cannot be possible, so mail must be from last year
        $mailq_time = int($local_year) - 1 .$mo_hash->{$mo}."$dd$hh$mi$ss";
    }

    return $mailq_time;
}

# vim:ts=4:shiftwidth:expandtab

1;
