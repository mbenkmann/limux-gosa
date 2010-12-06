package server_server_com;

use strict;
use warnings;

use Data::Dumper;
use Time::HiRes qw( usleep);
use GOsaSI::GosaSupportDaemon;

use Exporter;
use Socket;

our @ISA = qw(Exporter);

my @events = (
    'information_sharing',
    'new_server',
    'confirm_new_server',
    'new_foreign_client',
    'trigger_wake',
    'foreign_job_updates',
    'confirm_usr_msg',
    );
    
our @EXPORT = @events;

BEGIN {}

END {}

### Start ######################################################################

sub get_events {
    return \@events;
}


sub information_sharing {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];

    # Handling of msg tag 'new_user'
    if (exists $msg_hash->{'new_user'}) {
        my $new_user_list = $msg_hash->{'new_user'};

        # Sanity check of new_user_list
        if (ref($new_user_list) eq 'HASH') {
            &main::daemon_log("$session_id ERROR: 'new_user'-tag in incoming msg has no content!", 1);

        } else {
			my @user_list;
            # Add each user to login_users_db
            foreach my $new_user_info (@$new_user_list) {
                my ($client, $user) = split(/;/, $new_user_info);
                my %add_hash = ( table=>$main::login_users_tn, 
                        primkey=> ['client', 'user'],
                        client=>$client,
                        user=>$user,
                        timestamp=>&get_time,
                        regserver=>$source,
                        ); 
                my ($res, $error_str) = $main::login_users_db->add_dbentry( \%add_hash );
                if ($res != 0)  
				{
                    &main::daemon_log("$session_id ERROR: cannot add entry to known_clients: $error_str", 1);
                }
				else
				{
					push(@user_list, "'$user' at '$client'");
				}
            }
			&main::daemon_log("$session_id INFO: server '$source' reports the following logged in user: ".join(", ", @user_list), 5);
        }
    }

    # Handling of msg tag 'user_db'
    if (exists $msg_hash->{'user_db'}) {
        my $user_db_list = $msg_hash->{'user_db'};

        # Sanity check of user_db_list
        if (ref($user_db_list) eq 'HASH') {
            &main::daemon_log("$session_id ERROR: 'user_db'-tag in incoming msg has no content!", 1);

        } else {
            # Delete all old login information
            my $sql = "DELETE FROM $main::login_users_tn WHERE regserver='$source'"; 
            my $res = $main::login_users_db->exec_statement($sql);

            # Add each user to login_users_db
			my @user_list;
            foreach my $user_db_info (@$user_db_list) {
                my ($client, $user) = split(/;/, $user_db_info);
                my %add_hash = ( table=>$main::login_users_tn, 
                        primkey=> ['client', 'user'],
                        client=>$client,
                        user=>$user,
                        timestamp=>&get_time,
                        regserver=>$source,
                        ); 
                my ($res, $error_str) = $main::login_users_db->add_dbentry( \%add_hash );
                if ($res != 0)  {
                    &main::daemon_log("$session_id ERROR: cannot add entry to known_clients: $error_str", 1);
                }
				else
				{
					push(@user_list, "'$user' at '$client'");
				}
            }
			&main::daemon_log("$session_id INFO: server '$source' reports the following logged in user: ".join(", ", @user_list), 5);
        }
    }

    return;
}

sub foreign_job_updates {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    
    my @act_keys = keys %$msg_hash;
    my @jobs;
    foreach my $key (@act_keys) {
        if ($key =~ /answer\d+/ ) { push(@jobs, $key); }
    }

    foreach my $foreign_job (@jobs) {

        # add job to job queue
        my $func_dic = {table=>$main::job_queue_tn,
            primkey=>['macaddress', 'headertag'],
            timestamp=>@{@{$msg_hash->{$foreign_job}}[0]->{'timestamp'}}[0],
            status=>@{@{$msg_hash->{$foreign_job}}[0]->{'status'}}[0],
            result=>@{@{$msg_hash->{$foreign_job}}[0]->{'result'}}[0],
            progress=>@{@{$msg_hash->{$foreign_job}}[0]->{'progress'}}[0],
            headertag=>@{@{$msg_hash->{$foreign_job}}[0]->{'headertag'}}[0],
            targettag=>@{@{$msg_hash->{$foreign_job}}[0]->{'targettag'}}[0],
            xmlmessage=>@{@{$msg_hash->{$foreign_job}}[0]->{'xmlmessage'}}[0],
            macaddress=>@{@{$msg_hash->{$foreign_job}}[0]->{'macaddress'}}[0],
            plainname=>@{@{$msg_hash->{$foreign_job}}[0]->{'plainname'}}[0],
            siserver=>$source,
            modified=>"0",
        };
        my $res = $main::job_db->add_dbentry($func_dic);
        if (not $res == 0) {
            &main::daemon_log("$session_id ERROR: ServerPackages: process_job_msg: $res", 1);
        } else {
            &main::daemon_log("$session_id INFO: ServerPackages: $header, job '".@{@{$msg_hash->{$foreign_job}}[0]->{'headertag'}}[0].
                    "' successfully added to job queue", 5);
        }
    }

    return;
}


sub new_server {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $key = @{$msg_hash->{'key'}}[0];
    my $mac = exists $msg_hash->{'macaddress'} ? @{$msg_hash->{'macaddress'}}[0] : "" ;
    my @clients = exists $msg_hash->{'client'} ? @{$msg_hash->{'client'}} : qw();
    my @loaded_modules = exists $msg_hash->{'loaded_modules'} ? @{$msg_hash->{'loaded_modules'}} : qw();

	# Ignor message if I'm already within a registration process for server $source
	my $check_statement = "SELECT * FROM $main::known_server_tn WHERE status='new_server' AND hostname='$source'"; 
	&main::daemon_log("$session_id DEBUG $check_statement", 7);
	my $check_res = $main::known_server_db->select_dbentry($check_statement);
	my $blocking_process = keys(%$check_res);
	if ($blocking_process)
	{
		return;
	}

    # Sanity check
    if (ref $key eq 'HASH') {
        &main::daemon_log("$session_id ERROR: 'new_server'-message from host '$source' contains no key!", 1);
        return;
    }
    # Add foreign server to known_server_db
	my $new_update_time = &calc_timestamp(&get_time(), 'plus', $main::foreign_servers_register_delay);
    my $func_dic = {table=>$main::known_server_tn,
        primkey=>['hostname'],
        hostname => $source,
        macaddress => $mac,
        status => "new_server",
        hostkey => $key,
        loaded_modules => join(',', @loaded_modules),
        timestamp=>&get_time(),
		update_time=>$new_update_time,
    };
    my $res = $main::known_server_db->add_dbentry($func_dic);
    if (not $res == 0) {
        &main::daemon_log("$session_id ERROR: server_server_com.pm: cannot add server to known_server_db: $res", 1);
    } else {
        &main::daemon_log("$session_id INFO: server_server_com.pm: server '$source' successfully added to known_server_db", 5);
    }

    # delete all entries at foreign_clients_db coresponding to this server
    my $del_sql = "DELETE FROM $main::foreign_clients_tn WHERE regserver='$source' ";
    my $del_res = $main::foreign_clients_db->exec_statement($del_sql);

    # add clients of foreign server to known_foreign_clients_db
    my @sql_list;
    foreach my $client (@clients) {
        my @client_details = split(/,/, $client);

        # workaround to avoid double entries in foreign_clients_db
        my $del_sql = "DELETE FROM $main::foreign_clients_tn WHERE hostname='".$client_details[0]."'";
        push(@sql_list, $del_sql);

        my $sql = "INSERT INTO $main::foreign_clients_tn VALUES ("
            ."'".$client_details[0]."',"   # hostname
            ."'".$client_details[1]."',"   # macaddress
            ."'".$source."',"              # regserver
            ."'".&get_time()."')";         # timestamp
        push(@sql_list, $sql);
    }
    if (@sql_list) {
		my $len = @sql_list;
		$len /= 2;
        &main::daemon_log("$session_id DEBUG: Inserting ".$len." entries to foreign_clients_db", 8);
        my $res = $main::foreign_clients_db->exec_statementlist(\@sql_list);
    }

    # fetch all registered clients
    my $client_sql = "SELECT * FROM $main::known_clients_tn"; 
    my $client_res = $main::known_clients_db->exec_statement($client_sql);


    # add already connected clients to registration message 
    my $myhash = &create_xml_hash('confirm_new_server', $main::server_address, $source);
    &add_content2xml_hash($myhash, 'key', $key);
    map(&add_content2xml_hash($myhash, 'client', @{$_}[0].",".@{$_}[4]), @$client_res);

    # add locally loaded gosa-si modules to registration message
    my $loaded_modules = {};
    while (my ($package, $pck_info) = each %$main::known_modules) {
        foreach my $act_module (keys(%{@$pck_info[2]})) {
            $loaded_modules->{$act_module} = ""; 
        }
    }
    map(&add_content2xml_hash($myhash, "loaded_modules", $_), keys(%$loaded_modules));

    # add macaddress to registration message
    my ($host_ip, $host_port) = split(/:/, $source);
    my $local_ip = &get_local_ip_for_remote_ip($host_ip);
    my $network_interface= &get_interface_for_ip($local_ip);
    my $host_mac = &get_mac_for_interface($network_interface);
    &add_content2xml_hash($myhash, 'macaddress', $host_mac);

    # build registration message and send it
    my $out_msg = &create_xml_string($myhash);
    my $error =  &main::send_msg_to_target($out_msg, $source, $main::ServerPackages_key, 'confirm_new_server', $session_id); 

    return;
}


sub confirm_new_server {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $key = @{$msg_hash->{'key'}}[0];
    my $mac = exists $msg_hash->{'macaddress'} ? @{$msg_hash->{'macaddress'}}[0] : "" ;
    my @clients = exists $msg_hash->{'client'} ? @{$msg_hash->{'client'}} : qw();
    my @loaded_modules = exists $msg_hash->{'loaded_modules'} ? @{$msg_hash->{'loaded_modules'}} : qw();

	my $new_update_time = &calc_timestamp(&get_time(), 'plus', $main::foreign_servers_register_delay);
    my $sql = "UPDATE $main::known_server_tn".
        " SET status='$header', hostkey='$key', loaded_modules='".join(",",@loaded_modules)."', macaddress='$mac', update_time='$new_update_time'".
        " WHERE hostname='$source'"; 
    my $res = $main::known_server_db->update_dbentry($sql);

    # add clients of foreign server to known_foreign_clients_db
    my @sql_list;
    foreach my $client (@clients) {
        my @client_details = split(/,/, $client);

        # workaround to avoid double entries in foreign_clients_db
        my $del_sql = "DELETE FROM $main::foreign_clients_tn WHERE hostname='".$client_details[0]."'";
        push(@sql_list, $del_sql);

        my $sql = "INSERT INTO $main::foreign_clients_tn VALUES ("
            ."'".$client_details[0]."',"   	# hostname
            ."'".$client_details[1]."',"   	# macaddress
            ."'".$source."',"              	# regserver
            ."'".&get_time()."')";			# timestamp
        push(@sql_list, $sql);
    }
    if (@sql_list) {
		my $len = @sql_list;
		$len /= 2;
        &main::daemon_log("$session_id DEBUG: Inserting ".$len." entries to foreign_clients_db", 8);
        my $res = $main::foreign_clients_db->exec_statementlist(\@sql_list);
    }


    return;
}


sub new_foreign_client {
    my ($msg, $msg_hash, $session_id) = @_ ;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $hostname = @{$msg_hash->{'client'}}[0];
    my $macaddress = @{$msg_hash->{'macaddress'}}[0];
	# if new client is known in known_clients_db
	my $check_sql = "SELECT * FROM $main::known_clients_tn WHERE (macaddress LIKE '$macaddress')"; 
	my $check_res = $main::known_clients_db->select_dbentry($check_sql);

	if( (keys(%$check_res) == 1) ) {
			my $host_key = $check_res->{1}->{'hostkey'};

			# check if new client is still alive
			my $client_hash = &create_xml_hash("ping", $main::server_address, $hostname);
			&add_content2xml_hash($client_hash, 'session_id', $session_id);
			my $client_msg = &create_xml_string($client_hash);
			my $error = &main::send_msg_to_target($client_msg, $hostname, $host_key, 'ping', $session_id);
			my $message_id;
			my $i = 0;
			while (1) {
					$i++;
					my $sql = "SELECT * FROM $main::incoming_tn WHERE headertag='answer_$session_id'";
					my $res = $main::incoming_db->exec_statement($sql);
					if (ref @$res[0] eq "ARRAY") {
							$message_id = @{@$res[0]}[0];
							last;
					}

					# do not run into a endless loop
					if ($i > 50) { last; }
					usleep(100000);
			}

			# client is alive
			# -> new_foreign_client will be ignored
			if (defined $message_id) {
				&main::daemon_log("$session_id ERROR: At new_foreign_clients: host '$hostname' is reported as a new foreign client, ".
								"but the host is still registered at this server. So, the new_foreign_client-msg will be ignored: $msg", 1);
			}
	}

	
	# new client is not found in known_clients_db or
	# new client is dead -> new_client-msg from foreign server is valid
	# -> client will be deleted from known_clients_db 
	# -> inserted to foreign_clients_db
	
	my $del_sql = "DELETE FROM $main::known_clients_tn WHERE (hostname='$hostname')";
	my $del_res = $main::known_clients_db->exec_statement($del_sql);
    my $func_dic = { table => $main::foreign_clients_tn,
        primkey => ['hostname'],
        hostname =>   $hostname,
        macaddress => $macaddress,
        regserver =>  $source,
        timestamp =>  &get_time(),
    };
    my $res = $main::foreign_clients_db->add_dbentry($func_dic);
    if (not $res == 0) {
        &main::daemon_log("$session_id ERROR: server_server_com.pm: cannot add server to foreign_clients_db: $res", 1);
    } else {
        &main::daemon_log("$session_id INFO: server_server_com.pm: client '$hostname' successfully added to foreign_clients_db", 5);
    }

    return;
}


sub trigger_wake {
    my ($msg, $msg_hash, $session_id) = @_ ;

    foreach (@{$msg_hash->{'macaddress'}}){
        &main::daemon_log("$session_id INFO: trigger wake for $_", 5);
        my $host    = $_;
        my $ipaddr  = '255.255.255.255';
        my $port    = getservbyname('discard', 'udp');
	if (not defined $port) {
		&main::daemon_log("$session_id ERROR: cannot determine port for wol $_: 'getservbyname('discard', 'udp')' failed!",1);
		next;
	}

        my ($raddr, $them, $proto);
        my ($hwaddr, $hwaddr_re, $pkt);

        # get the hardware address (ethernet address)
        $hwaddr_re = join(':', ('[0-9A-Fa-f]{1,2}') x 6);
        if ($host =~ m/^$hwaddr_re$/) {
          $hwaddr = $host;
        } else {
          &main::daemon_log("$session_id ERROR: trigger_wake called with non mac address", 1);
        }

        # Generate magic sequence
        foreach (split /:/, $hwaddr) {
                $pkt .= chr(hex($_));
        }
        $pkt = chr(0xFF) x 6 . $pkt x 16 . $main::wake_on_lan_passwd;

        # Allocate socket and send packet

        $raddr = gethostbyname($ipaddr);
	if (not defined $raddr) {
		&main::daemon_log("$session_id ERROR: cannot determine raddr for wol $_: 'gethostbyname($ipaddr)' failed!", 1);
		next;
	}

        $them = pack_sockaddr_in($port, $raddr);
        $proto = getprotobyname('udp');

        socket(S, AF_INET, SOCK_DGRAM, $proto) or die "socket : $!";
        setsockopt(S, SOL_SOCKET, SO_BROADCAST, 1) or die "setsockopt : $!";
        send(S, $pkt, 0, $them) or die "send : $!";
        close S;
    }

    return;
}


sub confirm_usr_msg {
    my ($msg, $msg_hash, $session_id) = @_ ;
    &clMessages::confirm_usr_msg($msg, $msg_hash, $session_id);
    return;
}


1;
