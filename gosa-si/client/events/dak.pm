## @file
# @details A GOsa-SI event module containing all functions used by GOsa dak
# @brief Implementation of a GOsa-SI-client event module. 

package dak;

use strict;
use warnings;

use GOsaSI::GosaSupportDaemon;
use MIME::Base64;

use Exporter;

our @ISA = qw(Exporter);

my @events = (
    "get_events", 
    "get_dak_keyring",
    "import_dak_key",
    "remove_dak_key",
    );

our @EXPORT = @events;

BEGIN {}

END {}

our ($dak_base_directory, $dak_signing_keys_directory, $dak_queue_directory, $dak_user);

my %cfg_defaults = (
"client" => 
    {"dak-base" => [\$dak_base_directory, "/srv/archive"],
     "dak-keyring" => [\$dak_signing_keys_directory, "/srv/archive/keyrings"],
     "dak-queue" => [\$dak_queue_directory, "/srv/archive/queue"],
     "dak-user" => [\$dak_user, "deb-dak"],
    },
);
&GOsaSI::GosaSupportDaemon::read_configfile($main::config_file, %cfg_defaults);


## @method get_events()
# A brief function returning a list of functions which are exported by importing the module.
# @return List of all provided functions
sub get_events { return \@events; }


sub get_dak_keyring {
    my ($msg, $msg_hash) = @_;
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $header= @{$msg_hash->{'header'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];

    # build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    my @keys;
    my %data;

    my $keyring = $main::dak_signing_keys_directory."/keyring.gpg";

    my $gpg_cmd = `which gpg`; chomp $gpg_cmd;
    my $gpg     = "$gpg_cmd --no-default-keyring --no-random-seed --keyring $keyring";

    # Check if the keyrings are in place and readable
    if(
        &run_as($main::dak_user, "test -r $keyring")->{'resultCode'} != 0
    ) {
        &add_content2xml_hash($out_hash, "error", "DAK Keyring is not readable");
    } else {
        my $command = "$gpg --list-keys";
        my $output = &run_as($main::dak_user, $command);
        &main::daemon_log("$session_id DEBUG: ".$output->{'command'}, 7);

        my $i=0;
        foreach (@{$output->{'output'}}) {
            if ($_ =~ m/^pub\s.*$/) {
                ($keys[$i]->{'pub'}->{'length'}, $keys[$i]->{'pub'}->{'uid'}, $keys[$i]->{'pub'}->{'created'}) = ($1, $2, $3)
                if $_ =~ m/^pub\s*?(\w*?)\/(\w*?)\s(\d{4}-\d{2}-\d{2})/;
                $keys[$i]->{'pub'}->{'expires'} = $1 if $_ =~ m/^pub\s*?\w*?\/\w*?\s\d{4}-\d{2}-\d{2}\s\[expires:\s(\d{4}-\d{2}-\d{2})\]/;
                $keys[$i]->{'pub'}->{'expired'} = $1 if $_ =~ m/^pub\s*?\w*?\/\w*?\s\d{4}-\d{2}-\d{2}\s\[expired:\s(\d{4}-\d{2}-\d{2})\]/;
            } elsif ($_ =~ m/^sub\s.*$/) {
                ($keys[$i]->{'sub'}->{'length'}, $keys[$i]->{'sub'}->{'uid'}, $keys[$i]->{'sub'}->{'created'}) = ($1, $2, $3)
                if $_ =~ m/^sub\s*?(\w*?)\/(\w*?)\s(\d{4}-\d{2}-\d{2})/;
                $keys[$i]->{'sub'}->{'expires'} = $1 if $_ =~ m/^pub\s*?\w*?\/\w*?\s\d{4}-\d{2}-\d{2}\s\[expires:\s(\d{4}-\d{2}-\d{2})\]/;
                $keys[$i]->{'sub'}->{'expired'} = $1 if $_ =~ m/^pub\s*?\w*?\/\w*?\s\d{4}-\d{2}-\d{2}\s\[expired:\s(\d{4}-\d{2}-\d{2})\]/;
            } elsif ($_ =~ m/^uid\s.*$/) {
                push @{$keys[$i]->{'uid'}}, $1 if $_ =~ m/^uid\s*?([^\s].*?)$/;
            } elsif ($_ =~ m/^$/) {
                $i++;
            }
        }
    }

    my $i=0;
    foreach my $key (@keys) {
        &add_content2xml_hash($out_hash, "answer".$i++, $key);
    }
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }
    return &create_xml_string($out_hash);
}


sub import_dak_key {
    my ($msg, $msg_hash) = @_;
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $header= @{$msg_hash->{'header'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $key = &decode_base64(@{$msg_hash->{'key'}}[0]);

    # build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    my %data;

    my $keyring = $main::dak_signing_keys_directory."/keyring.gpg";

    my $gpg_cmd = `which gpg`; chomp $gpg_cmd;
    my $gpg     = "$gpg_cmd --no-default-keyring --no-random-seed --keyring $keyring";

    # Check if the keyrings are in place and writable
    if(
        &run_as($main::dak_user, "test -w $keyring")->{'resultCode'} != 0
    ) {
        &add_content2xml_hash($out_hash, "error", "DAK Keyring is not writable");
    } else {
        my $keyfile;
        open(my $keyfile, ">","/tmp/gosa_si_tmp_dak_key");
        print $keyfile $key;
        close($keyfile);
        my $command = "$gpg --import /tmp/gosa_si_tmp_dak_key";
        my $output = &run_as($main::dak_user, $command);
        &main::daemon_log("$session_id DEBUG: ".$output->{'command'}, 7);
        unlink("/tmp/gosa_si_tmp_dak_key");

        if($output->{'resultCode'} != 0) {
            &add_content2xml_hash($out_hash, "error", "Import of DAK key failed! Output was '".$output->{'output'}."'");
        } else {
            &add_content2xml_hash($out_hash, "answer", "Import of DAK key successfull! Output was '".$output->{'output'}."'");
        }
    }

    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }
    return &create_xml_string($out_hash);
}


sub remove_dak_key {
    my ($msg, $msg_hash) = @_;
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $header= @{$msg_hash->{'header'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $key = @{$msg_hash->{'keyid'}}[0];
    # build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    my %data;

    my $keyring = $main::dak_signing_keys_directory."/keyring.gpg";

    my $gpg_cmd = `which gpg`; chomp $gpg_cmd;
    my $gpg     = "$gpg_cmd --no-default-keyring --no-random-seed --homedir ".$main::dak_signing_keys_directory." --keyring $keyring";

    # Check if the keyrings are in place and writable
    if(
        &run_as($main::dak_user, "test -w $keyring")->{'resultCode'} != 0
    ) {
        &add_content2xml_hash($out_hash, "error", "DAK keyring is not writable");
    } else {
        # Check if the key is present in the keyring
        if(&run_as($main::dak_user, "$gpg --list-keys $key")->{'resultCode'} == 0) {
            my $command = "$gpg --batch --yes --delete-key $key";
            my $output = &run_as($main::dak_user, $command);
            &main::daemon_log("$session_id DEBUG: ".$output->{'command'}, 7);
        } else {
            &add_content2xml_hash($out_hash, "error", "DAK key with id '$key' was not found in keyring");
        }
    }

    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }
    return &create_xml_string($out_hash);
}


#sub get_dak_queue {
#    my ($msg, $msg_hash, $session_id) = @_;
#    my %data;
#    my $source = @{$msg_hash->{'source'}}[0];
#    my $target = @{$msg_hash->{'target'}}[0];
#    my $header= @{$msg_hash->{'header'}}[0];
#
#    my %data;
#
#    foreach my $dir ("unchecked", "new", "accepted") {
#        foreach my $file(<"$main::dak_queue_directory/$dir/*.changes">) {
#        }
#    }
#
#    my $out_msg = &build_msg("get_dak_queue", $target, $source, \%data);
#    my @out_msg_l = ($out_msg);
#    return @out_msg_l;
#}

1;
