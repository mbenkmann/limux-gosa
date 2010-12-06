package logHandling;


use strict;
use warnings;

use Data::Dumper;
use File::Spec;
use MIME::Base64;
use GOsaSI::GosaSupportDaemon;

use Exporter;

our @ISA = qw(Exporter);

my @events = (
    "get_events",
    "show_log_by_mac",
    "show_log_by_date",
    "show_log_by_date_and_mac",
    "show_log_files_by_date_and_mac",
    "get_log_file_by_date_and_mac",
    "get_recent_log_by_mac",
    "delete_log_by_date_and_mac",
    );

our @EXPORT = @events;

BEGIN {}

END {}

### Start ######################################################################


#===  FUNCTION  ================================================================
#         NAME:  get_events
#   PARAMETERS:  none
#      RETURNS:  reference of exported events
#  DESCRIPTION:  tells the caller which functions are available
#===============================================================================
sub get_events {
    return \@events
}


#===  FUNCTION  ================================================================
#         NAME: show_log_by_date
#  DESCRIPTION: reporting installed hosts matching to regex of date
#   PARAMETERS: [$msg]        original incoming message
#               [$msg_hash]   incoming message transformed to hash concerning XML::Simple
#               [$session_id] POE session id 
#      RETURNS: gosa-si valid answer string
#===============================================================================
sub show_log_by_date {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{header}}[0];
    my $target = @{$msg_hash->{target}}[0];
    my $source = @{$msg_hash->{source}}[0];
    my $date_l =  $msg_hash->{date};
    my $out_msg;
    $header =~ s/gosa_//;

    if (not -d $main::client_fai_log_dir) {
        my $error_string = "client fai log directory '$main::client_fai_log_dir' do not exist";
        &main::daemon_log("$session_id ERROR: $error_string", 1); 
        return &create_xml_string(&create_xml_hash($header, $target, $source, $error_string));
    }

    # build out_msg
    my $out_hash = &create_xml_hash($header, $target, $source);
    
    # read mac directory
    opendir(DIR, $main::client_fai_log_dir); 
    my @avail_macs = readdir(DIR);
    closedir(DIR);   
    foreach my $avail_mac (@avail_macs) {
        # check mac address 
        if ($avail_mac eq ".." || $avail_mac eq ".") { next; }

        # read install dates directory
        my $mac_dir = File::Spec->catdir($main::client_fai_log_dir, $avail_mac);
        opendir(DIR, $mac_dir);
        my @avail_dates = readdir(DIR);
        closedir(DIR);
        foreach my $date ( @{$date_l} ) {    # multiple date selection is allowed
            foreach my $avail_date (@avail_dates) {
                # check install date
                if ($avail_date eq ".." || $avail_date eq ".") { next; }
                if (not $avail_date =~ /$date/i) { next; }

                # add content to out_msg
                &add_content2xml_hash($out_hash, $avail_date, $avail_mac); 
            }
        }
    }

    $out_msg = &create_xml_string($out_hash);
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        $out_msg =~s/<\/xml>/<forward_to_gosa>$forward_to_gosa<\/forward_to_gosa><\/xml>/;
    }

    return ($out_msg);
}


#===  FUNCTION  ================================================================
#         NAME: show_log_by_mac
#  DESCRIPTION: reporting installation dates matching to regex of mac address
#   PARAMETERS: [$msg]        original incoming message
#               [$msg_hash]   incoming message transformed to hash concerning XML::Simple
#               [$session_id] POE session id 
#      RETURNS: gosa-si valid answer string 
#===============================================================================
sub show_log_by_mac {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{header}}[0];
    my $target = @{$msg_hash->{target}}[0];
    my $source = @{$msg_hash->{source}}[0];
    my $mac_l = $msg_hash->{mac};

    $header =~ s/gosa_//;

    if (not -d $main::client_fai_log_dir) {
        my $error_string = "client fai log directory '$main::client_fai_log_dir' do not exist";
        &main::daemon_log("$session_id ERROR: $error_string", 1); 
        return &create_xml_string(&create_xml_hash($header, $target, $source, $error_string));
    }

    # build out_msg
    my $out_hash = &create_xml_hash($header, $target, $source);

    # read mac directory
    opendir(DIR, $main::client_fai_log_dir); 
    my @avail_macs = readdir(DIR);
    closedir(DIR);   
    foreach my $mac (@{$mac_l}) {   # multiple mac selection is allowed
        foreach my $avail_mac ( @avail_macs ) {
            # check mac address
            if ($avail_mac eq ".." || $avail_mac eq ".") { next; }
            if (not $avail_mac =~ /$mac/i) { next; }
            
            # read install dates directory
            my $act_log_dir = File::Spec->catdir($main::client_fai_log_dir, $avail_mac);
            if (not -d $act_log_dir) { next; }
            opendir(DIR, $act_log_dir); 
            my @avail_dates = readdir(DIR);
            closedir(DIR);   
            $avail_mac =~ s/:/_/g;   # make mac address XML::Simple valid
            foreach my $avail_date (@avail_dates) {
                # check install date
                if ($avail_date eq ".." || $avail_date eq ".") { next; }

                # add content to out_msg
                &add_content2xml_hash($out_hash, "mac_$avail_mac", $avail_date);
            }
        }
    }

    my $out_msg = &create_xml_string($out_hash);
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        $out_msg =~s/<\/xml>/<forward_to_gosa>$forward_to_gosa<\/forward_to_gosa><\/xml>/;
    }

    return ($out_msg);
}


#===  FUNCTION  ================================================================
#         NAME: show_log_by_date_and_mac
#  DESCRIPTION: reporting host and installation dates matching to regex of date and regex of mac address
#   PARAMETERS: [$msg]        original incoming message
#               [$msg_hash]   incoming message transformed to hash concerning XML::Simple
#               [$session_id] POE session id 
#      RETURNS: gosa-si valid answer string 
#===============================================================================
sub show_log_by_date_and_mac {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{header}}[0];
    my $target = @{$msg_hash->{target}}[0];
    my $source = @{$msg_hash->{source}}[0];
    my $date = @{$msg_hash->{date}}[0];
    my $mac = @{$msg_hash->{mac}}[0];
    $header =~ s/gosa_//;

    if (not -d $main::client_fai_log_dir) {
        my $error_string = "client fai log directory '$main::client_fai_log_dir' do not exist"; 
        &main::daemon_log("$session_id ERROR: $error_string", 1); 
        return &create_xml_string(&create_xml_hash($header, $target, $source, $error_string));
    }

    # build out_msg
    my $out_hash = &create_xml_hash($header, $target, $source);

    # read mac directory
    opendir(DIR, $main::client_fai_log_dir); 
    my @avail_macs = readdir(DIR);
    closedir(DIR);   
    foreach my $avail_mac ( @avail_macs ) {
        # check mac address
        if ($avail_mac eq ".." || $avail_mac eq ".") { next; }
        if (not $avail_mac =~ /$mac/i) { next; }
        my $act_log_dir = File::Spec->catdir($main::client_fai_log_dir, $avail_mac);
    
        # read install date directory
        opendir(DIR, $act_log_dir); 
        my @install_dates = readdir(DIR);
        closedir(DIR);   
        foreach my $avail_date (@install_dates) {
            # check install date
            if ($avail_date eq ".." || $avail_date eq ".") { next; }
            if (not $avail_date =~ /$date/i) { next; }

            # add content to out_msg
            &add_content2xml_hash($out_hash, $avail_date, $avail_mac);
        }
    }

    my $out_msg = &create_xml_string($out_hash);
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        $out_msg =~s/<\/xml>/<forward_to_gosa>$forward_to_gosa<\/forward_to_gosa><\/xml>/;
    }

    return $out_msg;
}


#===  FUNCTION  ================================================================
#         NAME: show_log_files_by_date_and_mac
#  DESCRIPTION: reporting installation log files matching exatly to date and mac address
#   PARAMETERS: [$msg]        original incoming message
#               [$msg_hash]   incoming message transformed to hash concerning XML::Simple
#               [$session_id] POE session id 
#      RETURNS: gosa-si valid answer string 
#===============================================================================
sub show_log_files_by_date_and_mac {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{header}}[0];
    my $target = @{$msg_hash->{target}}[0];
    my $source = @{$msg_hash->{source}}[0];
    my $date = @{$msg_hash->{date}}[0];
    my $mac = @{$msg_hash->{mac}}[0];
    $header =~ s/gosa_//;

    my $act_log_dir = File::Spec->catdir($main::client_fai_log_dir, $mac, $date);
    if (not -d $act_log_dir) {
        my $error_string = "client fai log directory '$act_log_dir' do not exist";
        &main::daemon_log("$session_id ERROR: $error_string", 1); 
        return &create_xml_string(&create_xml_hash($header, $target, $source, $error_string));
    }

    # build out_msg
    my $out_hash = &create_xml_hash($header, $target, $source);

    # read mac / install date directory
    opendir(DIR, $act_log_dir); 
    my @log_files = readdir(DIR);
    closedir(DIR);   

    foreach my $log_file (@log_files) {
        if ($log_file eq ".." || $log_file eq ".") { next; }

        # add content to out_msg
        &add_content2xml_hash($out_hash, $header, $log_file);
    }

    my $out_msg = &create_xml_string($out_hash);
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        $out_msg =~s/<\/xml>/<forward_to_gosa>$forward_to_gosa<\/forward_to_gosa><\/xml>/;
    }

    return $out_msg;
}


#===  FUNCTION  ================================================================
#         NAME: get_log_file_by_date_and_mac
#  DESCRIPTION: returning the given log file, base64 coded, matching exactly to date and mac address
#   PARAMETERS: [$msg]        original incoming message
#               [$msg_hash]   incoming message transformed to hash concerning XML::Simple
#               [$session_id] POE session id 
#      RETURNS: gosa-si valid answer string 
#===============================================================================
sub get_log_file_by_date_and_mac {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{header}}[0];
    my $target = @{$msg_hash->{target}}[0];
    my $source = @{$msg_hash->{source}}[0];
    my $date = @{$msg_hash->{date}}[0];
    my $mac = @{$msg_hash->{mac}}[0];
    my $log_file = @{$msg_hash->{log_file}}[0];
    $header =~ s/gosa_//;
 
    # sanity check
    my $act_log_file = File::Spec->catfile($main::client_fai_log_dir, $mac, $date, $log_file);
    if (not -f $act_log_file) {
        my $error_string = "client fai log file '$act_log_file' do not exist or could not be read"; 
        &main::daemon_log("$session_id ERROR: $error_string", 1); 
        &main::daemon_log("$session_id ERROR: mac='$mac', date='$date', log_file='$log_file'", 1); 
        &main::daemon_log("$session_id ERROR: could not process message: $msg", 1); 
        return &create_xml_string(&create_xml_hash($header, $target, $source, $error_string));
    }
    
    # read log file
    my $log_content;
    open(my $FILE, "<$act_log_file");
    my @log_lines = <$FILE>;
    close($FILE);

    # prepare content for xml sending
    $log_content = join("", @log_lines); 
    $log_content = &encode_base64($log_content);

    # build out_msg and send
    my $out_hash = &create_xml_hash($header, $target, $source);
    &add_content2xml_hash($out_hash, $log_file, $log_content);
    my $out_msg = &create_xml_string($out_hash);
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        $out_msg =~s/<\/xml>/<forward_to_gosa>$forward_to_gosa<\/forward_to_gosa><\/xml>/;
    }

    return $out_msg;
}


# sorting function for fai log directory names
# used by get_recent_log_by_mac
sub transform {
    my $a = shift;
    $a =~ /_(\d{8}?)_(\d{6}?)$/ || return 0;
    return int("$1$2");
}
sub by_log_date {
    &transform($a) <=> &transform($b);
}
#===  FUNCTION  ================================================================
#         NAME: get_recent_log_by_mac
#  DESCRIPTION: reporting the latest installation date matching to regex of mac address
#   PARAMETERS: [$msg]        original incoming message
#               [$msg_hash]   incoming message transformed to hash concerning XML::Simple
#               [$session_id] POE session id 
#      RETURNS: gosa-si valid answer string 
#===============================================================================
sub get_recent_log_by_mac {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{header}}[0];
    my $target = @{$msg_hash->{target}}[0];
    my $source = @{$msg_hash->{source}}[0];
    my $mac = @{$msg_hash->{mac}}[0];
    $header =~ s/gosa_//;

    # sanity check
    if (not -d $main::client_fai_log_dir) {
        my $error_string = "client fai log directory '$main::client_fai_log_dir' do not exist";
        &main::daemon_log("$session_id ERROR: $error_string", 1); 
        return &create_xml_string(&create_xml_hash($header, $target, $source, $error_string));
    }
    
    opendir (DIR, $main::client_fai_log_dir);
    my @avail_macs = readdir(DIR);
    closedir(DIR);
    my $act_log_dir;
    my $act_mac;
    foreach my $avail_mac (@avail_macs) { 
        if ($avail_mac eq ".." || $avail_mac eq ".") { next; }
        if (not $avail_mac =~ /$mac/i) { next; }
        $act_log_dir = File::Spec->catdir($main::client_fai_log_dir, $avail_mac);
        $act_mac = $avail_mac;
    }
    if (not defined $act_log_dir) {
        my $error_string = "do not find mac '$mac' in directory '$main::client_fai_log_dir'";
        &main::daemon_log("$session_id ERROR: $error_string", 1); 
        return &create_xml_string(&create_xml_hash($header, $target, $source, $error_string));
    }

    # read mac directory
    opendir(DIR, $act_log_dir); 
    my @avail_dates = readdir(DIR);
    closedir(DIR);   

    # search for the latest log 
    my @sorted_dates = sort by_log_date @avail_dates;
    my $latest_log = pop(@sorted_dates);

    # build out_msg
    my $out_hash = &create_xml_hash($header, $target, $source);

    # read latest log directory
    my $latest_log_dir = File::Spec->catdir($main::client_fai_log_dir, $act_mac, $latest_log);
    opendir(DIR, $latest_log_dir); 
    my @log_files = readdir(DIR);
    closedir(DIR);   

    # add all log_files to out_msg
    foreach my $log_file (@log_files) {
        if ($log_file eq ".." || $log_file eq ".") { next; }
        &add_content2xml_hash($out_hash, $latest_log, $log_file);
    }

    my $out_msg = &create_xml_string($out_hash);
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        $out_msg =~s/<\/xml>/<forward_to_gosa>$forward_to_gosa<\/forward_to_gosa><\/xml>/;
    }

    return $out_msg;
}


#===  FUNCTION  ================================================================
#         NAME: delete_log_by_date_and_mac
#  DESCRIPTION: delete installation date directory matching to regex of date and regex of mac address
#               missing date or mac is substitutet with regex '.'; if both is missing, deleting is rejected
#   PARAMETERS: [$msg]        original incoming message
#               [$msg_hash]   incoming message transformed to hash concerning XML::Simple
#               [$session_id] POE session id 
#      RETURNS: gosa-si valid answer string
#===============================================================================
sub delete_log_by_date_and_mac {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{header}}[0];
    my $target = @{$msg_hash->{target}}[0];
    my $source = @{$msg_hash->{source}}[0];
    my $date = @{$msg_hash->{date}}[0];
    my $mac = @{$msg_hash->{mac}}[0];
    $header =~ s/gosa_//;

    # sanity check
    if (not -d $main::client_fai_log_dir) {
        my $error_string = "client fai log directory '$main::client_fai_log_dir' do not exist";
        &main::daemon_log("$session_id ERROR: $session_id", 1); 
        return &create_xml_string(&create_xml_hash($header, $target, $source, $error_string));
    }
    if ((not defined $date) && (not defined $mac)) {
        my $error_string = "deleting all log files from gosa-si-server by an empty delete message is not permitted";
        &main::daemon_log("$session_id INFO: $error_string", 5);
        return &create_xml_string(&create_xml_hash($header, $target, $source, $error_string));
    }
    if (not defined $date) { $date = "."; }   # set date to a regular expression matching to everything
    if (not defined $mac) { $mac = "."; }     # set mac to a regular expression matching to everything
 
    # build out_msg
    my $out_hash = &create_xml_hash($header, $target, $source);

    # read mac directory
    opendir(DIR, $main::client_fai_log_dir); 
    my @avail_macs = readdir(DIR);
    closedir(DIR);   
    foreach my $avail_mac ( @avail_macs ) {
        # check mac address
        if ($avail_mac eq ".." || $avail_mac eq ".") { next; }
        if (not $avail_mac =~ /$mac/i) { next; }
        my $act_log_dir = File::Spec->catdir($main::client_fai_log_dir, $avail_mac);
    
        # read install date directory
        opendir(DIR, $act_log_dir); 
        my @install_dates = readdir(DIR);
        closedir(DIR);   
        foreach my $avail_date (@install_dates) {
            # check install date
            if ($avail_date eq ".." || $avail_date eq ".") { next; }
            if (not $avail_date =~ /$date/i) { next; }
            
            # delete directory and reptorting
            my $dir_to_delete = File::Spec->catdir($main::client_fai_log_dir, $avail_mac, $avail_date);
            #my $error = rmdir($dir_to_delete);
            my $error = 0;
            if ($error == 1) {
                &main::daemon_log("$session_id ERROR: log directory '$dir_to_delete' cannot be deleted: $!", 1); 
            } else {
                &main::daemon_log("$session_id INFO: log directory '$dir_to_delete' deleted", 5); 
                &add_content2xml_hash($out_hash, $avail_date, $avail_mac);
            }
        }
    }

    my $out_msg = &create_xml_string($out_hash);
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        $out_msg =~s/<\/xml>/<forward_to_gosa>$forward_to_gosa<\/forward_to_gosa><\/xml>/;
    }

    return $out_msg;
}

1;
