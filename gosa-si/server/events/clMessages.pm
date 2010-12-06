## @file
# @brief Implementation of a GOsa-SI event module. 
# @details A GOsa-SI event module containing all functions to handle incoming messages from clients.

package clMessages;


use strict;
use warnings;

use Data::Dumper;
use MIME::Base64;
use GOsaSI::GosaSupportDaemon;

use Exporter;

our @ISA = qw(Exporter);

my @events = (
    "confirm_usr_msg",
    "PROGRESS",
    "FAIREBOOT",
    "TASKSKIP",
    "TASKBEGIN",
    "TASKEND",
    "TASKERROR",
    "HOOK",
    "GOTOACTIVATION",
    "LOGIN",
    "LOGOUT",
    "CURRENTLY_LOGGED_IN",
    "save_fai_log",
    );
our @EXPORT = @events;

BEGIN {}

END {}


## @method get_events()
# @details A brief function returning a list of functions which are exported by importing the module.
# @return List of all provided functions
sub get_events {
    return \@events;
}

## @method confirm_usr_msg()
# @details Confirmed messages are set in the messaging_db from d (deliverd) to s(seen). 
# @param msg - STRING - xml message with tags 'message', 'subject' and 'usr'
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub confirm_usr_msg {
    my ($msg, $msg_hash, $session_id) = @_;
    my $message = @{$msg_hash->{'message'}}[0];
    my $subject = @{$msg_hash->{'subject'}}[0];
    my $usr = @{$msg_hash->{'usr'}}[0];

    # set update for this message
    my $sql = "UPDATE $main::messaging_tn SET flag='s' WHERE (message='$message' AND subject='$subject' AND message_to='$usr')"; 
    &main::daemon_log("$session_id DEBUG: $sql", 7);
    my $res = $main::messaging_db->exec_statement($sql); 


    return;
}


## @method save_fai_log()
# @details Creates under /var/log/fai/ the directory '$macaddress' and stores within all FAI log files from client.
# @param msg - STRING - xml message with tags 'macaddress' and 'save_fai_log'
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub save_fai_log {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $macaddress = @{$msg_hash->{'macaddress'}}[0];
    my $all_logs = @{$msg_hash->{$header}}[0];

    # if there is nothing to log
    if( ref($all_logs) eq "HASH" ) { 
        &main::daemon_log("$session_id INFO: There is nothing to log!", 5);
        return; 
    }
        
    my $client_fai_log_dir = $main::client_fai_log_dir;
    if (not -d $client_fai_log_dir) {
        mkdir($client_fai_log_dir, 0755)
    }

    $client_fai_log_dir = File::Spec->catfile( $client_fai_log_dir, $macaddress );
    if (not -d $client_fai_log_dir) {
        mkdir($client_fai_log_dir, 0755)
    }

    my $time = &get_time;
    $time = substr($time, 0, 8)."_".substr($time, 8, 6);
    $client_fai_log_dir = File::Spec->catfile( $client_fai_log_dir, "install_$time" );
    mkdir($client_fai_log_dir, 0755);

    my @all_logs = split(/log_file:/, $all_logs); 
    foreach my $log (@all_logs) {
        if (length $log == 0) { next; };
        my ($log_file, $log_string) = split(":", $log);
        my $client_fai_log_file = File::Spec->catfile( $client_fai_log_dir, $log_file);

		open(my $LOG_FILE, ">", "$client_fai_log_file"); 
		print $LOG_FILE &decode_base64($log_string);
		close($LOG_FILE);
		chown($main::root_uid, $main::adm_gid, $client_fai_log_file);
		chmod(0440, $client_fai_log_file);

    }
    return;
}

## @method LOGIN()
# @details Reported user from client is added to login_users_db.
# @param msg - STRING - xml message with tag 'LOGIN'
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub LOGIN {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $login = @{$msg_hash->{$header}}[0];
    my $res;
    my $error_str;

    # Invoke set_last_system; message sets ldap attributes 'gotoLastSystemLogin' and 'gotoLastSystem'
	$res = &set_last_system($msg, $msg_hash, $session_id);

    # Add user to login_users_db
    my %add_hash = ( table=>$main::login_users_tn, 
        primkey=> ['client', 'user'],
        client=>$source,
        user=>$login,
        timestamp=>&get_time,
        regserver=>'localhost',
        ); 
    ($res, $error_str) = $main::login_users_db->add_dbentry( \%add_hash );
    if ($res != 0)  {
        &main::daemon_log("$session_id ERROR: cannot add entry to known_clients: $error_str");
        return;
    }

    # Share login information with all other si-server
    my %data = ( 'new_user'  => "$source;$login" );
    my $info_msg = &build_msg("information_sharing", $main::server_address, "KNOWN_SERVER", \%data);

    return ($info_msg);  
}


## @method LOGOUT()
# @details Reported user from client is deleted from login_users_db.
# @param msg - STRING - xml message with tag 'LOGOUT'
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub LOGOUT {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $login = @{$msg_hash->{$header}}[0];
    
    my $sql_statement = "DELETE FROM $main::login_users_tn WHERE (client='$source' AND user='$login')"; 
    my $res =  $main::login_users_db->del_dbentry($sql_statement);
    &main::daemon_log("$session_id INFO: delete user '$login' at client '$source' from login_user_db", 5); 
    
    return;
}


## @method CURRENTLY_LOGGED_IN()
# @details Reported users from client are updated in login_users_db. Users which are no longer logged in are deleted from DB. 
# @param msg - STRING - xml message
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub CURRENTLY_LOGGED_IN {
    my ($msg, $msg_hash, $session_id) = @_;
    my ($sql_statement, $db_res);
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $login = @{$msg_hash->{$header}}[0];

    if(ref $login eq "HASH") { 
        &main::daemon_log("$session_id INFO: no logged in users reported from host '$source'", 5); 
        return;     
    }

    # Invoke set_last_system; message sets ldap attributes 'gotoLastSystemLogin' and 'gotoLastSystem'
	my $res = &set_last_system($msg, $msg_hash, $session_id);
    
    # fetch all user currently assigned to the client at login_users_db
    my %currently_logged_in_user = (); 
    $sql_statement = "SELECT * FROM $main::login_users_tn WHERE client='$source'"; 
    $db_res = $main::login_users_db->select_dbentry($sql_statement);
    while( my($hit_id, $hit) = each(%{$db_res}) ) {
        $currently_logged_in_user{$hit->{'user'}} = 1;
    }
    &main::daemon_log("$session_id DEBUG: logged in users from login_user_db: ".join(", ", keys(%currently_logged_in_user)), 7); 

    # update all reported users in login_user_db
    my @logged_in_user = split(/\s+/, $login);
    &main::daemon_log("$session_id DEBUG: logged in users reported from client: ".join(", ", @logged_in_user), 7); 
    foreach my $user (@logged_in_user) {
        my %add_hash = ( table=>$main::login_users_tn, 
                primkey=> ['client', 'user'],
                client=>$source,
                user=>$user,
                timestamp=>&get_time,
                regserver=>'localhost',
                ); 
        my ($res, $error_str) = $main::login_users_db->add_dbentry( \%add_hash );
        if ($res != 0)  {
            &main::daemon_log("$session_id ERROR: cannot add entry to known_clients: $error_str");
            return;
        }

        delete $currently_logged_in_user{$user};
    }

    # if there is still a user in %currently_logged_in_user 
    # although he is not reported by client 
    # then delete it from $login_user_db
    foreach my $obsolete_user (keys(%currently_logged_in_user)) {
        &main::daemon_log("$session_id WARNING: user '$obsolete_user' is currently not logged ".
                "in at client '$source' but still found at login_user_db", 3); 
        my $sql_statement = "DELETE FROM $main::login_users_tn WHERE client='$source' AND user='$obsolete_user'"; 
        my $res =  $main::login_users_db->del_dbentry($sql_statement);
        &main::daemon_log("$session_id WARNING: delete user '$obsolete_user' at client '$source' from login_user_db", 3); 
    }

    # Delete all users which logged in information is older than their logged_in_user_date_of_expiry
    my $act_time = &get_time();
    my $expiry_date = &calc_timestamp($act_time, "minus", $main::logged_in_user_date_of_expiry); 

    $sql_statement = "SELECT * FROM $main::login_users_tn WHERE CAST(timestamp as UNSIGNED)<$expiry_date"; 
    $db_res = $main::login_users_db->select_dbentry($sql_statement);

    while( my($hit_id, $hit) = each(%{$db_res}) ) {
        &main::daemon_log("$session_id INFO: user '".$hit->{'user'}."' is no longer reported to be logged in at host '".$hit->{'client'}."'", 5);
        my $sql = "DELETE FROM $main::login_users_tn WHERE (client='".$hit->{'client'}."' AND user='".$hit->{'user'}."')"; 
        my $res =  $main::login_users_db->del_dbentry($sql);
        &main::daemon_log("$session_id INFO: delete user '".$hit->{'user'}."' at client '".$hit->{'client'}."' from login_user_db", 5); 
    }

    # Inform all other server which users are logged in at clients registered at local server
    my $info_sql = "SELECT * FROM $main::login_users_tn WHERE regserver='localhost'";
    my $info_res = $main::login_users_db->select_dbentry($info_sql);
    my $info_msg_hash = &create_xml_hash("information_sharing", $main::server_address, "KNOWN_SERVER");
    while (my ($hit_id, $hit) = each(%$info_res)) {
        &add_content2xml_hash($info_msg_hash, 'user_db', $hit->{'client'}.";".$hit->{'user'});
    }
    my $info_msg = &create_xml_string($info_msg_hash);

    return ($info_msg);  
}


## @method set_last_system()
# @details Message sets ldap attributes 'gotoLastSystemLogin' and 'gotoLastSystem'
# @param msg - STRING - xml message with tag 'last_system_login' and 'last_system'
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub set_last_system {
	my ($msg, $msg_hash, $session_id) = @_;
	my $header = @{$msg_hash->{'header'}}[0];
	my $source = @{$msg_hash->{'source'}}[0];
    my $login = @{$msg_hash->{$header}}[0];
    
	# Sanity check of needed parameter
	if (not exists $msg_hash->{'timestamp'}){
		&main::daemon_log("$session_id ERROR: message does not contain needed xml tag 'timestamp', ".
						"setting of 'gotoLastSystem' and 'gotoLastSystemLogin' stopped!", 1);
		&main::daemon_log($msg, 1);
		return;
	}
	if (@{$msg_hash->{'timestamp'}} != 1)  {
		&main::daemon_log("$session_id ERROR: xml tag 'timestamp' has no content or exists more than one time, ".
						"setting of 'gotoLastSystem' and 'gotoLastSystemLogin' stopped!", 1);
		&ymain::daemon_log($msg, 1);
		return;
	}
	if (not exists $msg_hash->{'macaddress'}){
		&main::daemon_log("$session_id ERROR: message does not contain needed xml tag 'mac_address', ".
						"setting of 'gotoLastSystem' and 'gotoLastSystemLogin' stopped!", 1);
		&main::daemon_log($msg, 1);
		return;
	}
	if (@{$msg_hash->{'macaddress'}} != 1)  {
		&main::daemon_log("$session_id ERROR: xml tag 'macaddress' has no content or exists more than one time, ".
						"setting of 'gotoLastSystem' and 'gotoLastSystemLogin' stopped!", 1);
		&ymain::daemon_log($msg, 1);
		return;
	}

	# Fetch needed parameter
	my $mac =  @{$msg_hash->{'macaddress'}}[0];
	my $timestamp = @{$msg_hash->{'timestamp'}}[0];
	
	# Prepare login list
	my @login_list = split(' ', @{$msg_hash->{$header}}[0] );
    @login_list = &main::del_doubles(@login_list);

	# Sanity check of login list
	if (@login_list == 0) {
		# TODO
		return;
	}

	# Get system info
	my $ldap_handle=&main::get_ldap_handle($session_id);
	my $ldap_mesg= $ldap_handle->search(
					base => $main::ldap_base,
					scope => 'sub',
					filter => "macAddress=$mac",
					);
	if ($ldap_mesg->count == 0) {
		&main::daemon_log("$session_id ERROR: no system with mac address='$mac' was found in base '".
						$main::ldap_base."', setting of 'gotoLastSystem' and 'gotoLastSystemLogin' stopped!", 1);
		&main::release_ldap_handle($ldap_handle);
		return;
	}

	my $ldap_entry = $ldap_mesg->pop_entry();
	my $system_dn = $ldap_entry->dn();
	
	# For each logged in user set gotoLastSystem and gotoLastSystemLogin
	foreach my $user (@login_list) {
		# Search user
		my $ldap_mesg= $ldap_handle->search(
						base => $main::ldap_base,
						scope => 'sub',
						filter => "(&(objectClass=gosaAccount)(uid=$user))",
						);
		# Sanity check of user search
		if ($ldap_mesg->count == 0) {
			&main::daemon_log("$session_id WARNING: no user with uid='$user' was found in base '".
							$main::ldap_base."', setting of 'gotoLastSystem' and 'gotoLastSystemLogin' stopped!", 1);

		# Set gotoLastSystem and gotoLastSystemLogin
		} else {
			my $ldap_entry= $ldap_mesg->pop_entry();
            my $do_update = 0;

            # Set gotoLastSystem information
            my $last_system_dn = $ldap_entry->get_value('gotoLastSystem');
            if ((defined $last_system_dn) && ($last_system_dn eq $system_dn)) {
                &main::daemon_log("$session_id INFO: no new 'gotoLastSystem' information for ldap entry 'uid=$user', do nothing!", 5);
            } elsif ((defined $last_system_dn) && ($last_system_dn ne $system_dn)) {
                $ldap_entry->replace ( 'gotoLastSystem' => $system_dn );
                &main::daemon_log("$session_id INFO: update attribute 'gotoLastSystem'='$system_dn' at ldap entry 'uid=$user'!",5);
                $do_update++;
            } else {
                $ldap_entry->add( 'gotoLastSystem' => $system_dn );
                &main::daemon_log("$session_id INFO: add attribute 'gotoLastSystem'='$system_dn' at ldap entry 'uid=$user'!", 5);
                $do_update++;
            }

            # Set gotoLastSystemLogin information
            # Attention: only write information if last_system_dn and system_dn differs
            my $last_system_login = $ldap_entry->get_value('gotoLastSystemLogin');
            if ((defined $last_system_login) && ($last_system_dn eq $system_dn)) {
                &main::daemon_log("$session_id INFO: no new 'gotoLastSystemLogin' information for ldap entry 'uid=$user', do nothing!", 5);
            } elsif ((defined $last_system_login) && ($last_system_dn ne $system_dn)) {
                $ldap_entry->replace ( 'gotoLastSystemLogin' => $timestamp );
                &main::daemon_log("$session_id INFO: update attribute 'gotoLastSystemLogin'='$timestamp' at ldap entry 'uid=$user'!", 5);
                $do_update++;
            } else {
                $ldap_entry->add( 'gotoLastSystemLogin' => $timestamp );
                &main::daemon_log("$session_id INFO: add attribute 'gotoLastSystemLogin'='$timestamp' at ldap entry 'uid=$user'!",5);
                $do_update++;
            }

            if ($do_update) {
                my $result = $ldap_entry->update($ldap_handle);
                if ($result->code() != 0) {
                    &main::daemon_log("$session_id ERROR: setting 'gotoLastSystem' and 'gotoLastSystemLogin' at user '$user' failed: ".
                            $result->{'errorMessage'}."\n".
                            "\tbase: $main::ldap_base\n".
                            "\tscope: 'sub'\n".
                            "\tfilter: 'uid=$user'\n".
                            "\tmessage: $msg", 1); 
                }
            }
		}
	}
	&main::release_ldap_handle($ldap_handle);

	return;
}


## @method GOTOACTIVATION()
# @details Client is set at job_queue_db to status 'processing' and 'modified'.
# @param msg - STRING - xml message with tag 'macaddress'
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub GOTOACTIVATION {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $macaddress = @{$msg_hash->{'macaddress'}}[0];

    # test whether content is an empty hash or a string which is required
    my $content = @{$msg_hash->{$header}}[0];
    if(ref($content) eq "HASH") { $content = ""; }

    # clean up header
    $header =~ s/CLMSG_//g;

    my $sql_statement = "UPDATE $main::job_queue_tn ".
            "SET progress='goto-activation', modified='1' ".
            "WHERE status='processing' AND macaddress LIKE '$macaddress'"; 
    &main::daemon_log("$session_id DEBUG: $sql_statement", 7);         
    my $res = $main::job_db->update_dbentry($sql_statement);
    &main::daemon_log("$session_id INFO: $header at '$macaddress'", 5); 
    return; 
}


## @method PROGRESS()
# @details Message reports installation progress of the client. Installation job at job_queue_db is going to be updated.
# @param msg - STRING - xml message with tags 'macaddress' and 'PROGRESS'
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub PROGRESS {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $macaddress = @{$msg_hash->{'macaddress'}}[0];

    # test whether content is an empty hash or a string which is required
    my $content = @{$msg_hash->{$header}}[0];
    if(ref($content) eq "HASH") { $content = ""; }

    # clean up header
    $header =~ s/CLMSG_//g;

    my $sql_statement = "UPDATE $main::job_queue_tn ".
        "SET progress='$content', modified='1' ".
        "WHERE status='processing' AND macaddress LIKE '$macaddress'";
    &main::daemon_log("$session_id DEBUG: $sql_statement", 7);         
    my $res = $main::job_db->update_dbentry($sql_statement);
    &main::daemon_log("$session_id INFO: $header at '$macaddress' - $content%", 5); 

    return;
}


## @method FAIREBOOT()
# @details Message reports a FAI reboot. Job at job_queue_db is going to be updated.
# @param msg - STRING - xml message with tag 'macaddress' and 'FAIREBOOT'
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub FAIREBOOT {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $macaddress = @{$msg_hash->{'macaddress'}}[0];

    # test whether content is an empty hash or a string which is required
	my $content = @{$msg_hash->{$header}}[0];
    if(ref($content) eq "HASH") { $content = ""; }

    # clean up header
    $header =~ s/CLMSG_//g;

    my $sql_statement = "UPDATE $main::job_queue_tn ".
            "SET result='$header "."$content', modified='1' ".
            "WHERE status='processing' AND macaddress LIKE '$macaddress'"; 
    &main::daemon_log("$session_id DEBUG: $sql_statement", 7);         
    my $res = $main::job_db->update_dbentry($sql_statement);
    &main::daemon_log("$session_id INFO: $header at '$macaddress' - '$content'", 5); 

    return; 
}


## @method TASKSKIP()
# @details Message reports a skipped FAI task. Job at job_queue_db is going to be updated. 
# @param msg - STRING - xml message with tag 'macaddress'.
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub TASKSKIP {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $macaddress = @{$msg_hash->{'macaddress'}}[0];

    # test whether content is an empty hash or a string which is required
	my $content = @{$msg_hash->{$header}}[0];
    if(ref($content) eq "HASH") { $content = ""; }

    # clean up header
    $header =~ s/CLMSG_//g;

    my $sql_statement = "UPDATE $main::job_queue_tn ".
            "SET result='$header "."$content', modified='1' ".
            "WHERE status='processing' AND macaddress LIKE '$macaddress'"; 
    &main::daemon_log("$session_id DEBUG: $sql_statement", 7);         
    my $res = $main::job_db->update_dbentry($sql_statement);
    &main::daemon_log("$session_id INFO: $header at '$macaddress' - '$content'", 5); 

    return; 
}


## @method TASKBEGIN()
# @details Message reports a starting FAI task. If the task is equal to 'finish', 'faiend' or 'savelog', job at job_queue_db is being set to status 'done' and FAI state is being set to 'localboot'. If task is equal to 'chboot', 'test' or 'confdir', just do nothing. In all other cases, job at job_queue_db is going to be updated or created if not exists. 
# @param msg - STRING - xml message with tag 'macaddress'.
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub TASKBEGIN {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $macaddress = @{$msg_hash->{'macaddress'}}[0];

    # test whether content is an empty hash or a string which is required
	my $content = @{$msg_hash->{$header}}[0];
    if(ref($content) eq "HASH") { $content = ""; }

    # clean up header
    $header =~ s/CLMSG_//g;

    # TASKBEGIN eq finish or faiend 
    if (($content eq 'finish') 
			|| ($content eq 'faiend')
			|| ($content eq 'savelog')
			) {
        my $sql_statement = "UPDATE $main::job_queue_tn ".
            "SET status='done', result='$header "."$content', modified='1' ".
            "WHERE status='processing' AND macaddress LIKE '$macaddress'"; 
        &main::daemon_log("$session_id DEBUG: $sql_statement", 7);         
        my $res = $main::job_db->update_dbentry($sql_statement);
        &main::daemon_log("$session_id INFO: $header at '$macaddress' - '$content'", 5); 
        
        # set fai_state to localboot
        &main::change_fai_state('localboot', \@{$msg_hash->{'macaddress'}}, $session_id);

	# TASKBEGIN eq chboot
	} elsif (($content eq 'chboot')
		|| ($content eq 'test')
		|| ($content eq 'confdir')
		) {
		# just ignor this client message
		# do nothing

	# other TASKBEGIN msgs
    } else {
		# TASKBEGIN msgs do only occour during a softupdate or a reinstallation
		# of a host. Set all waiting update- or reinstall-jobs for host to
		# processing so they can be handled correctly by the rest of the function.
		my $waiting_sql = "UPDATE $main::job_queue_tn SET status='processing' WHERE status='waiting' AND macaddress LIKE '$macaddress' AND (headertag='trigger_action_update' OR headertag='trigger_action_reinstall')"; 
		&main::daemon_log("$session_id DEBUB: $waiting_sql", 7);
		my $waiting_res = $main::job_db->update_dbentry($waiting_sql);

		# Select processing jobs for host
		my $sql_statement = "SELECT * FROM $main::job_queue_tn WHERE status='processing' AND macaddress LIKE '$macaddress'"; 
		&main::daemon_log("$session_id DEBUG: $sql_statement", 7);
		my $res = $main::job_db->select_dbentry($sql_statement);

		# there is exactly one job entry in queue for this host
		if (keys(%$res) == 1) {
			&main::daemon_log("$session_id DEBUG: there is already one processing job in queue for host '$macaddress', run an update for this entry", 7);
			my $sql_statement = "UPDATE $main::job_queue_tn SET result='$header $content', modified='1', siserver='localhost' WHERE status='processing' AND macaddress LIKE '$macaddress'";
			my $err = $main::job_db->update_dbentry($sql_statement);
			if (not defined  $err) {
				&main::daemon_log("$session_id ERROR: cannot update job_db entry: ".Dumper($err), 1);
			}
			
		# There is no entry in queue or more than one entries in queue for this host
		} else {
			# In case of more than one running jobs in queue, delete all jobs
			if (keys(%$res) > 1) {
				&main::daemon_log("$session_id DEBUG: there are more than one processing job in queue for host '$macaddress', ".
								"delete entries", 7); 

                # set job to status 'done', job will be deleted automatically
                my $sql_statement = "UPDATE $main::job_queue_tn ".
                    "SET status='done', modified='1'".
                    "WHERE status='processing' AND macaddress LIKE '$macaddress'";
                &main::daemon_log("$session_id DEBUG: $sql_statement", 7);
                my $res = $main::job_db->update_dbentry( $sql_statement );

			}
		
			# In case of no and more than one running jobs in queue, add one single job

			# Resolve plain name for host $macaddress
			my $plain_name;
			my $ldap_handle=&main::get_ldap_handle($session_id);
			my $mesg = $ldap_handle->search(
					base => $main::ldap_base,
					scope => 'sub',
					attrs => ['cn'],
					filter => "(macAddress=$macaddress)");
			if(not $mesg->code) {
				my $entry= $mesg->entry(0);
				$plain_name = $entry->get_value("cn");
			} else {
				&main::daemon_log($mesg->error, 1);
				$plain_name = "none";
			}
			&main::release_ldap_handle($ldap_handle);

            # In any case add a new job to job queue
			&main::daemon_log("$session_id DEBUG: add job to queue for host '$macaddress'", 7); 
			my $func_dic = {table=>$main::job_queue_tn,
					primkey=>['macaddress', 'headertag'],
					timestamp=>&get_time,
					status=>'processing',
					result=>"$header $content",
					progress=>'none',
					headertag=>'trigger_action_reinstall',
					targettag=>$target,
					xmlmessage=>'none',
					macaddress=>$macaddress,
					plainname=>$plain_name,
                    modified=>'1',
                    siserver=>'localhost',
			};
			my ($err, $error_str) = $main::job_db->add_dbentry($func_dic);
			if ($err != 0)  {
					&main::daemon_log("$session_id ERROR: cannot add entry to job_db: $error_str", 1);
			}
		}
    }

    return; 
}


## @method TASKEND()
# @details Message reports a finished FAI task. If task is equal to 'savelog', job at job_queue_db is going to be set to status 'done'. Otherwise, job is going to be updated. 
# @param msg - STRING - xml message with tag 'macaddress'.
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub TASKEND {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $macaddress = @{$msg_hash->{'macaddress'}}[0];

    # test whether content is an empty hash or a string which is required
    my $content = @{$msg_hash->{$header}}[0];
    if(ref($content) eq "HASH") { $content = ""; }

    # clean up header
    $header =~ s/CLMSG_//g;

	if ($content eq "savelog 0") {
		&main::daemon_log("$session_id DEBUG: got savelog from host '$target' - job done", 7);
        
        # set job to status 'done', job will be deleted automatically
        my $sql_statement = "UPDATE $main::job_queue_tn ".
					"SET status='done', modified='1'".
                    "WHERE status='processing' AND macaddress LIKE '$macaddress'";
        &main::daemon_log("$session_id DEBUG: $sql_statement", 7);
        my $res = $main::job_db->update_dbentry( $sql_statement );

	} else {
        my $sql_statement = "UPDATE $main::job_queue_tn ".
            "SET result='$header "."$content', modified='1' ".
            "WHERE status='processing' AND macaddress LIKE '$macaddress'"; 
        &main::daemon_log("$session_id DEBUG: $sql_statement", 7);         
        my $res = $main::job_db->update_dbentry($sql_statement);
        &main::daemon_log("$session_id INFO: $header at '$macaddress' - '$content'", 5); 
	}

    return; 
}


## @method TASKERROR()
# @details Message reports a FAI error. Job at job_queue_db is going to be updated. 
# @param msg - STRING - xml message with tag 'macaddress' and 'TASKERROR'
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub TASKERROR {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $macaddress = @{$msg_hash->{'macaddress'}}[0];

    # clean up header
    $header =~ s/CLMSG_//g;

    # test whether content is an empty hash or a string which is required
    my $content = @{$msg_hash->{$header}}[0];
    if(ref($content) eq "HASH") { $content = ""; } 

	# set fai_state to localboot
	&main::change_fai_state('error', \@{$msg_hash->{'macaddress'}}, $session_id);
		
    my $sql_statement = "UPDATE $main::job_queue_tn ".
            "SET result='$header "."$content', modified='1' ".
            "WHERE status='processing' AND macaddress LIKE '$macaddress'"; 
    &main::daemon_log("$session_id DEBUG: $sql_statement", 7);         
    my $res = $main::job_db->update_dbentry($sql_statement);
    &main::daemon_log("$session_id INFO: $header at '$macaddress' - '$content'", 5); 

    return; 
}


## @method HOOK()
# @details Message reports a FAI hook. Job at job_queue_db is going to be updated. 
# @param msg - STRING - xml message with tag 'macaddress' and 'HOOK'
# @param msg_hash - HASHREF - message information parsed into a hash
# @param session_id - INTEGER - POE session id of the processing of this message
sub HOOK {
    my ($msg, $msg_hash, $session_id) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $macaddress = @{$msg_hash->{'macaddress'}}[0];

    # clean up header
    $header =~ s/CLMSG_//g;

    # test whether content is an empty hash or a string which is required
	my $content = @{$msg_hash->{$header}}[0];
    if(not ref($content) eq "STRING") { $content = ""; }

    my $sql_statement = "UPDATE $main::job_queue_tn ".
            "SET result='$header "."$content', modified='1' ".
            "WHERE status='processing' AND macaddress LIKE '$macaddress'"; 
    &main::daemon_log("$session_id DEBUG: $sql_statement", 7);         
    my $res = $main::job_db->update_dbentry($sql_statement);
    &main::daemon_log("$session_id INFO: $header at '$macaddress' - '$content'", 5); 

    return;
}


=pod

=head1 NAME

clMessages - Implementation of a GOsa-SI event module for GOsa-SI-server.

=head1 SYNOPSIS

 use GOSA::GosaSupportDaemon;
 use MIME::Base64;

=head1 DESCRIPTION

This GOsa-SI event module containing all functions to handle messages coming from GOsa-SI-clients. 

This module will be automatically imported by GOsa-SI if it is under F</usr/lib/gosa-si/server/E<lt>PACKAGEMODULEE<gt>/> .

=head1 METHODS

=over 4

=item get_events ( )

=item confirm_usr_msg ( )

=item PROGRESS ( )

=item FAIREBOOT ( )

=item TASKSKIP ( )

=item TASKBEGIN ( )

=item TASKEND ( )

=item TASKERROR ( )

=item HOOK ( )

=item GOTOACTIVATION ( )

=item LOGIN ( )

=item LOGOUT ( )

=item CURRENTLY_LOGGED_IN ( )

=item save_fai_log ( )

=back

=head1 BUGS

Please report any bugs, or post any suggestions, to the GOsa mailing list E<lt>gosa-devel@oss.gonicus.deE<gt> or to L<https://oss.gonicus.de/labs/gosa>

=head1 COPYRIGHT

This code is part of GOsa (L<http://www.gosa-project.org>)

Copyright (C) 2003-2008 GONICUS GmbH

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

=cut


1;
