package GOsaSI::GosaSupportDaemon;

use strict;
use warnings;

use IO::Socket::INET;
use Crypt::Rijndael;
use Digest::MD5  qw(md5 md5_hex md5_base64);
use MIME::Base64;
use XML::Quote qw(:all);
use XML::Simple;
use Data::Dumper;
use Net::DNS;
use Net::ARP;

use DateTime;
use Exporter;

our @ISA = qw(Exporter);

my @functions = (
    "create_passwd",
    "create_xml_hash",
    "createXmlHash",
    "myXmlHashToString",
    "get_content_from_xml_hash",
    "add_content2xml_hash",
    "create_xml_string",
    "transform_msg2hash",
    "get_time",
    "get_utc_time",
    "build_msg",
    "db_res2xml",
    "db_res2si_msg",
    "get_where_statement",
    "get_select_statement",
    "get_update_statement",
    "get_limit_statement",
    "get_orderby_statement",
    "get_dns_domains",
    "get_server_addresses",
    "get_logged_in_users",
    "import_events",
    "del_doubles",
    "get_ip",
    "get_interface_for_ip",
    "get_interfaces",
    "get_mac_for_interface",
    "get_local_ip_for_remote_ip",
    "is_local",
    "run_as",
    "inform_all_other_si_server",
    "read_configfile",
    "check_opsi_res",
    "calc_timestamp",
    "opsi_callobj2string",
    );
    
our @EXPORT = @functions;

my $op_hash = {
    'eq' => '=',
    'ne' => '!=',
    'ge' => '>=',
    'gt' => '>',
    'le' => '<=',
    'lt' => '<',
    'like' => ' LIKE ',
};


BEGIN {}

END {}

### Start ######################################################################

our $xml = new XML::Simple();

sub daemon_log {
    my ($msg, $level) = @_ ;
    &main::daemon_log($msg, $level);
    return;
}


sub create_passwd {
    my $new_passwd = "";
    for(my $i=0; $i<31; $i++) {
        $new_passwd .= ("a".."z","A".."Z",0..9)[int(rand(62))]
    }

    return $new_passwd;
}


sub del_doubles { 
    my %all; 
    $all{$_}=0 for @_; 
    return (keys %all); 
}


#===  FUNCTION  ================================================================
#         NAME:  create_xml_hash
#   PARAMETERS:  header - string - message header (required)
#                source - string - where the message come from (required)
#                target - string - where the message should go to (required)
#                [header_value] - string - something usefull (optional)
#      RETURNS:  hash - hash - nomen est omen
#  DESCRIPTION:  creates a key-value hash, all values are stored in a array
#===============================================================================
sub create_xml_hash {
    my ($header, $source, $target, $header_value) = @_;
    my $hash = {
            header => [$header],
            source => [$source],
            target => [$target],
            $header => [$header_value],
    };
    return $hash
}

sub createXmlHash {
	my ($header, $source, $target) = @_;
	return { header=>$header, source=>$source, target=>$target};
}

sub _transformHashToString {
	my ($hash) = @_;
	my $s = "";

	while (my ($tag, $content) = each(%$hash)) {

		if (ref $content eq "HASH") {
			$s .= "<$tag>".&_transformHashToString($content)."</$tag>";
		} elsif ( ref $content eq "ARRAY") {
			$s .= &_transformArrayToString($tag, $content);
		} else {
			$content = defined $content ? $content : "";
			$s .= "<$tag>".&xml_quote($content)."</$tag>";
		}
	}
	return $s;
}

sub _transformArrayToString {
	my ($tag, $contentArray) = @_;
	my $s = "";
	foreach my $content (@$contentArray) {
		if (ref $content eq "HASH") {
			$s .= "<$tag>".&_transformHashToString($content)."</$tag>";
		} else {
			$content = defined $content ? $content : "";
			$s .= "<$tag>".&xml_quote($content)."</$tag>";
		}
	}
	return $s;
}


#===  FUNCTION  ================================================================
#         NAME:  myXmlHashToString
#   PARAMETERS:  xml_hash - hash - hash from function createXmlHash
#      RETURNS:  xml_string - string - xml string representation of the hash
#  DESCRIPTION:  Transforms the given hash to a xml wellformed string. I.e.:
#	             {
#                   'header' => 'a'
#                   'source' => 'c',
#                   'target' => 'b',
#                   'hit' => [ '1',
#                              '2',
#                              { 
#                                'hit31' => 'ABC',
#                   	         'hit32' => 'XYZ'
#                              }
#                            ],
#                   'res0' => {
#                      'res1' => {
#                         'res2' => 'result'
#                      }
#                   },
#             	 };
#           
#				 will be transformed to 
#				 <xml>
#					<header>a</header>
#					<source>c</source>
#					<target>b</target>
#					<hit>1</hit>
#					<hit>2</hit>
#					<hit>
#						<hit31>ABC</hit31>
#						<hit32>XYZ</hit32>
#					</hit>
#					<res0>
#						<res1>
#							<res2>result</res2>
#						</res1>
#					</res0>
#				</xml>
#
#===============================================================================
sub myXmlHashToString {
	my ($hash) = @_;
	return "<xml>".&_transformHashToString($hash)."</xml>";
}


#===  FUNCTION  ================================================================
#         NAME:  create_xml_string
#   PARAMETERS:  xml_hash - hash - hash from function create_xml_hash
#      RETURNS:  xml_string - string - xml string representation of the hash
#  DESCRIPTION:  transform the hash to a string using XML::Simple module
#===============================================================================
sub create_xml_string {
    my ($xml_hash) = @_ ;
    my $xml_string = $xml->XMLout($xml_hash, RootName => 'xml');
    #$xml_string =~ s/[\n]+//g;
    #daemon_log("create_xml_string:",7);
    #daemon_log("$xml_string\n", 7);
    return $xml_string;
}


sub transform_msg2hash {
    my ($msg) = @_ ;
    my $hash = $xml->XMLin($msg, ForceArray=>1);
    
    # xml tags without a content are created as an empty hash
    # substitute it with an empty list
    eval {
        while( my ($xml_tag, $xml_content) = each %{ $hash } ) {
            if( 1 == @{ $xml_content } ) {
                # there is only one element in xml_content list ...
                my $element = @{ $xml_content }[0];
                if( ref($element) eq "HASH" ) {
                    # and this element is an hash ...
                    my $len_element = keys %{ $element };
                    if( $len_element == 0 ) {
                        # and this hash is empty, then substitute the xml_content
                        # with an empty string in list
                        $hash->{$xml_tag} = [ "none" ];
                    }
                }
            }
        }
    };
    if( $@ ) {  
        $hash = undef;
    }

    return $hash;
}


#===  FUNCTION  ================================================================
#         NAME:  add_content2xml_hash
#   PARAMETERS:  xml_ref - ref - reference to a hash from function create_xml_hash
#                element - string - key for the hash
#                content - string - value for the hash
#      RETURNS:  nothing
#  DESCRIPTION:  add key-value pair to xml_ref, if key alread exists, 
#                then append value to list
#===============================================================================
sub add_content2xml_hash {
    my ($xml_ref, $element, $content) = @_;
    if(not exists $$xml_ref{$element} ) {
        $$xml_ref{$element} = [];
    }
    my $tmp = $$xml_ref{$element};
    push(@$tmp, $content);
    return;
}


sub get_time {
	my ($seconds, $minutes, $hours, $monthday, $month,
		$year, $weekday, $yearday, $sommertime) = localtime;
	$hours = $hours < 10 ? $hours = "0".$hours : $hours;
	$minutes = $minutes < 10 ? $minutes = "0".$minutes : $minutes;
	$seconds = $seconds < 10 ? $seconds = "0".$seconds : $seconds;
	$month+=1;
	$month = $month < 10 ? $month = "0".$month : $month;
	$monthday = $monthday < 10 ? $monthday = "0".$monthday : $monthday;
	$year+=1900;
	return "$year$month$monthday$hours$minutes$seconds";
}


sub get_utc_time {
    my $utc_time = qx(date --utc +%Y%m%d%H%M%S);
    $utc_time =~ s/\s$//;
    return $utc_time;
}


#===  FUNCTION  ================================================================
#         NAME: build_msg
#  DESCRIPTION: Send a message to a destination
#   PARAMETERS: [header] Name of the header
#               [from]   sender ip
#               [to]     recipient ip
#               [data]   Hash containing additional attributes for the xml
#                        package
#      RETURNS:  nothing
#===============================================================================
sub build_msg ($$$$) {
	my ($header, $from, $to, $data) = @_;

    # data is of form, i.e.
    # %data= ('ip' => $address, 'mac' => $mac);

	my $out_hash = &create_xml_hash($header, $from, $to);

	while ( my ($key, $value) = each(%$data) ) {
		if(ref($value) eq 'ARRAY'){
			map(&add_content2xml_hash($out_hash, $key, $_), @$value);
		} else {
			&add_content2xml_hash($out_hash, $key, $value);
		}
	}
    my $out_msg = &create_xml_string($out_hash);
    return $out_msg;
}


sub db_res2xml {
    my ($db_res) = @_ ;
    my $xml = "";

    my $len_db_res= keys %{$db_res};
    for( my $i= 1; $i<= $len_db_res; $i++ ) {
        $xml .= "\n<answer$i>";
        my $hash= $db_res->{$i};
        while ( my ($column_name, $column_value) = each %{$hash} ) {
            $xml .= "<$column_name>";
            my $xml_content;
            if( $column_name eq "xmlmessage" ) {
                $xml_content = &encode_base64($column_value);
            } else {
                $xml_content = defined $column_value ? $column_value : "";
            }
            $xml .= $xml_content;
            $xml .= "</$column_name>"; 
        }
        $xml .= "</answer$i>";

    }

    return $xml;
}


sub db_res2si_msg {
    my ($db_res, $header, $target, $source) = @_;

    my $si_msg = "<xml>";
    $si_msg .= "<header>$header</header>";
    $si_msg .= "<source>$source</source>";
    $si_msg .= "<target>$target</target>";
    $si_msg .= &db_res2xml;
    $si_msg .= "</xml>";
}


sub get_where_statement {
    my ($msg, $msg_hash) = @_;
    my $error= 0;
    
    my $clause_str= "";
    if( (not exists $msg_hash->{'where'}) || (not exists @{$msg_hash->{'where'}}[0]->{'clause'}) ) { 
        $error++; 
    }

    if( $error == 0 ) {
        my @clause_l;
        my @where = @{@{$msg_hash->{'where'}}[0]->{'clause'}};
        foreach my $clause (@where) {
            my $connector = $clause->{'connector'}[0];
            if( not defined $connector ) { $connector = "AND"; }
            $connector = uc($connector);
            delete($clause->{'connector'});

            my @phrase_l ;
            foreach my $phrase (@{$clause->{'phrase'}}) {
                my $operator = "=";
                if( exists $phrase->{'operator'} ) {
                    my $op = $op_hash->{$phrase->{'operator'}[0]};
                    if( not defined $op ) {
                        &main::daemon_log("ERROR: Can not translate operator '$operator' in where-".
                                "statement to sql valid syntax. Please use 'eq', ".
                                "'ne', 'ge', 'gt', 'le', 'lt' in xml message\n", 1);
                        &main::daemon_log($msg, 8);
                        $op = "=";
                    }
                    $operator = $op;
                    delete($phrase->{'operator'});
                }

                my @xml_tags = keys %{$phrase};
                my $tag = $xml_tags[0];
                my $val = $phrase->{$tag}[0];
                if( ref($val) eq "HASH" ) { next; }  # empty xml-tags should not appear in where statement

				# integer columns do not have to have single quotes besides the value
				if ($tag eq "id") {
						push(@phrase_l, "$tag$operator$val");
				} else {
						push(@phrase_l, "$tag$operator'$val'");
				}
            }

            if (not 0 == @phrase_l) {
                my $clause_str .= join(" $connector ", @phrase_l);
                push(@clause_l, "($clause_str)");
            }
        }

        if( not 0 == @clause_l ) {
            $clause_str = join(" AND ", @clause_l);
            $clause_str = "WHERE ($clause_str) ";
        }
    }

    return $clause_str;
}

sub get_select_statement {
    my ($msg, $msg_hash)= @_;
    my $select = "*";
    if( exists $msg_hash->{'select'} ) {
        my $select_l = \@{$msg_hash->{'select'}};
        $select = join(', ', @{$select_l});
    }
    return $select;
}


sub get_update_statement {
    my ($msg, $msg_hash) = @_;
    my $error= 0;
    my $update_str= "";
    my @update_l; 

    if( not exists $msg_hash->{'update'} ) { $error++; };

    if( $error == 0 ) {
        my $update= @{$msg_hash->{'update'}}[0];
        while( my ($tag, $val) = each %{$update} ) {
            my $val= @{$update->{$tag}}[0];
            push(@update_l, "$tag='$val'");
        }
        if( 0 == @update_l ) { $error++; };   
    }

    if( $error == 0 ) { 
        $update_str= join(', ', @update_l);
        $update_str= "SET $update_str ";
    }

    return $update_str;
}

sub get_limit_statement {
    my ($msg, $msg_hash)= @_; 
    my $error= 0;
    my $limit_str = "";
    my ($from, $to);

    if( not exists $msg_hash->{'limit'} ) { $error++; };

    if( $error == 0 ) {
        eval {
            my $limit= @{$msg_hash->{'limit'}}[0];
            $from= @{$limit->{'from'}}[0];
            $to= @{$limit->{'to'}}[0];
        };
        if( $@ ) {
            $error++;
        }
    }

    if( $error == 0 ) {
        $limit_str= "LIMIT $from, $to";
    }   
    
    return $limit_str;
}

sub get_orderby_statement {
    my ($msg, $msg_hash)= @_;
    my $error= 0;
    my $order_str= "";
    my $order;
    
    if( not exists $msg_hash->{'orderby'} ) { $error++; };

    if( $error == 0) {
        eval {
            $order= @{$msg_hash->{'orderby'}}[0];
        };
        if( $@ ) {
            $error++;
        }
    }

    if( $error == 0 ) {
        $order_str= "ORDER BY $order";   
    }
    
    return $order_str;
}

sub get_dns_domains() {
        my $line;
        my @searches;
        open(my $RESOLV, "<", "/etc/resolv.conf") or return @searches;
        while(<$RESOLV>){
                $line= $_;
                chomp $line;
                $line =~ s/^\s+//;
                $line =~ s/\s+$//;
                $line =~ s/\s+/ /;
                if ($line =~ /^domain (.*)$/ ){
                        push(@searches, $1);
                } elsif ($line =~ /^search (.*)$/ ){
                        push(@searches, split(/ /, $1));
                }
        }
        close($RESOLV);

        my %tmp = map { $_ => 1 } @searches;
        @searches = sort keys %tmp;

        return @searches;
}


sub get_server_addresses {
    my $domain= shift;
    my @result;
    my $error_string;

    my $error = 0;
    my $res   = Net::DNS::Resolver->new;
    my $query = $res->send("_gosa-si._tcp.".$domain, "SRV");
    my @hits;

    if ($query) {
        foreach my $rr ($query->answer) {
            push(@hits, $rr->target.":".$rr->port);
        }
    }
    else {
        $error_string = "determination of '_gosa-si._tcp' server in domain '$domain' failed: ".$res->errorstring;
        $error++;
    }

    if( $error == 0 ) {
        foreach my $hit (@hits) {
            my ($hit_name, $hit_port) = split(/:/, $hit);
            chomp($hit_name);
            chomp($hit_port);

            my $address_query = $res->send($hit_name);
            if( 1 == length($address_query->answer) ) {
                foreach my $rr ($address_query->answer) {
                    push(@result, $rr->address.":".$hit_port);
                }
            }
        }
    }

    return \@result, $error_string;
}


sub get_logged_in_users {
    my $result = qx(/usr/bin/w -hs);
    my @res_lines;

    if( defined $result ) { 
        chomp($result);
        @res_lines = split("\n", $result);
    }

    my @logged_in_user_list;
    foreach my $line (@res_lines) {
        chomp($line);
        my @line_parts = split(/\s+/, $line); 
        push(@logged_in_user_list, $line_parts[0]);
    }

    return @logged_in_user_list;

}


sub import_events {
    my ($event_dir) = @_;
    my $event_hash;
    my $error = 0;
    my @result = ();
    if (not -e $event_dir) {
        $error++;
        push(@result, "cannot find directory or directory is not readable: $event_dir");   
    }

    my $DIR;
    if ($error == 0) {
        opendir ($DIR, $event_dir) or do { 
            $error++;
            push(@result, "cannot open directory '$event_dir' for reading: $!\n");
        }
    }

    if ($error == 0) {
        while (defined (my $event = readdir ($DIR))) {
            if( $event eq "." || $event eq ".." || ($event =~ /^\.pm$/)) { next; }  

			# Check config file to exclude disabled event plugins (i.e. Opsi)
			if ($event eq "opsi_com.pm" &&  $main::opsi_enabled ne "true")  { 
				&main::daemon_log("0 WARNING: opsi-module is installed but not enabled in config file, please set under section '[OPSI]': 'enabled=true'", 3);	
				next; 
			}

            # try to import event module
            eval{ require $event; };
            if( $@ ) {
                $error++;
                #push(@result, "import of event module '$event' failed: $@");
                #next;
                
                &main::daemon_log("ERROR: Import of event module '$event' failed: $@",1);
                exit(1);
            }

            # fetch all single events
            $event =~ /(\S*?).pm$/;
            my $event_module = $1;
            my $events_l = eval( $1."::get_events()") ;
            foreach my $event_name (@{$events_l}) {
                $event_hash->{$event_module}->{$event_name} = "";
            }
            my $events_string = join( ", ", @{$events_l});
            push(@result, "import of event module '$event' succeed: $events_string");
        }
        
        close $DIR;
    }

    return ($error, \@result, $event_hash);

}


#===  FUNCTION  ================================================================
#         NAME:  get_ip 
#   PARAMETERS:  interface name (i.e. eth0)
#      RETURNS:  (ip address) 
#  DESCRIPTION:  Uses ioctl to get ip address directly from system.
#===============================================================================
sub get_ip {
	my $ifreq= shift;
	my $result= "";
	my $SIOCGIFADDR= 0x8915;       # man 2 ioctl_list
	my $proto= getprotobyname('ip');

	socket SOCKET, PF_INET, SOCK_DGRAM, $proto
		or die "socket: $!";

	if(ioctl SOCKET, $SIOCGIFADDR, $ifreq) {
		my ($if, $sin)    = unpack 'a16 a16', $ifreq;
		my ($port, $addr) = sockaddr_in $sin;
		my $ip            = inet_ntoa $addr;

		if ($ip && length($ip) > 0) {
			$result = $ip;
		}
	}

	return $result;
}


#===  FUNCTION  ================================================================
#         NAME:  get_interface_for_ip
#   PARAMETERS:  ip address (i.e. 192.168.0.1)
#      RETURNS:  array: list of interfaces if ip=0.0.0.0, matching interface if found, undef else
#  DESCRIPTION:  Uses proc fs (/proc/net/dev) to get list of interfaces.
#===============================================================================
sub get_interface_for_ip {
	my $result;
	my $ip= shift;

	if($ip =~ /^[a-z]/i) {
		my $ip_address = inet_ntoa(scalar gethostbyname($ip));
		if(defined($ip_address) && $ip_address =~ /^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}/) {
			# Write ip address to $source variable
			$ip = $ip_address;
		}
	}

	if ($ip && length($ip) > 0) {
		my @ifs= &get_interfaces();
		if($ip eq "0.0.0.0") {
			$result = "all";
		} else {
			foreach (@ifs) {
				my $if=$_;
				if(&get_ip($if) eq $ip) {
					$result = $if;
				}
			}	
		}
	}	
	return $result;
}

#===  FUNCTION  ================================================================
#         NAME:  get_interfaces 
#   PARAMETERS:  none
#      RETURNS:  (list of interfaces) 
#  DESCRIPTION:  Uses proc fs (/proc/net/dev) to get list of interfaces.
#===============================================================================
sub get_interfaces {
	my @result;
	my $PROC_NET_DEV= ('/proc/net/dev');

	open(my $FD_PROC_NET_DEV, "<", "$PROC_NET_DEV")
		or die "Could not open $PROC_NET_DEV";

	my @ifs = <$FD_PROC_NET_DEV>;

	close($FD_PROC_NET_DEV);

	# Eat first two line
	shift @ifs;
	shift @ifs;

	chomp @ifs;
	foreach my $line(@ifs) {
		my $if= (split /:/, $line)[0];
		$if =~ s/^\s+//;
		push @result, $if;
	}

	return @result;
}

sub get_local_ip_for_remote_ip {
	my $remote_ip= shift;
	my $result="0.0.0.0";

    if($remote_ip =~ /^(\d\d?\d?\.){3}\d\d?\d?$/) {
        my $PROC_NET_ROUTE= ('/proc/net/route');

        open(my $FD_PROC_NET_ROUTE, "<", "$PROC_NET_ROUTE")
            or die "Could not open $PROC_NET_ROUTE";

        my @ifs = <$FD_PROC_NET_ROUTE>;

        close($FD_PROC_NET_ROUTE);

        # Eat header line
        shift @ifs;
        chomp @ifs;
        my $iffallback = ''; 

        # linux-vserver might have * as Iface due to hidden interfaces, set a default 
        foreach my $line(@ifs) { 
            my ($Iface,$Destination,$Gateway,$Flags,$RefCnt,$Use,$Metric,$Mask,$MTU,$Window,$IRTT)=split(/\s/, $line); 
            if ($Iface =~ m/^[^\*]+$/) { 
                 $iffallback = $Iface; 
            } 
        }
 
        foreach my $line(@ifs) {
            my ($Iface,$Destination,$Gateway,$Flags,$RefCnt,$Use,$Metric,$Mask,$MTU,$Window,$IRTT)=split(/\s/, $line);
            my $destination;
            my $mask;
            my ($d,$c,$b,$a)=unpack('a2 a2 a2 a2', $Destination);
            if ($Iface =~ m/^[^\*]+$/) { 
                 $iffallback = $Iface;
            } 
            $destination= sprintf("%d.%d.%d.%d", hex($a), hex($b), hex($c), hex($d));
            ($d,$c,$b,$a)=unpack('a2 a2 a2 a2', $Mask);
            $mask= sprintf("%d.%d.%d.%d", hex($a), hex($b), hex($c), hex($d));
            if(new NetAddr::IP($remote_ip)->within(new NetAddr::IP($destination, $mask))) {
                # destination matches route, save mac and exit
                #$result= &get_ip($Iface);

                if ($Iface =~ m/^\*$/ ) { 
                    $result= &get_ip($iffallback);    
                } else { 
                    $result= &get_ip($Iface); 
                } 
                last;
            }
        }
    } 

	return $result;
}


sub get_mac_for_interface {
	my $ifreq= shift;
	my $result;
	if ($ifreq && length($ifreq) > 0) {
		if($ifreq eq "all") {
			$result = "00:00:00:00:00:00";
		} else {
        $result = Net::ARP::get_mac($ifreq);
		}
	}
	return $result;
}


#===  FUNCTION  ================================================================
#         NAME:  is_local
#   PARAMETERS:  Server Address
#      RETURNS:  true if Server Address is on this host, false otherwise
#  DESCRIPTION:  Checks all interface addresses, stops on first match
#===============================================================================
sub is_local {
    my $server_address = shift || "";
    my $result = 0;

    my $server_ip = $1 if $server_address =~ /^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}):\d{1,6}$/;

    if(defined($server_ip) && length($server_ip) > 0) {
        foreach my $interface(&get_interfaces()) {
            my $ip_address= &get_ip($interface);
            if($ip_address eq $server_ip) {
                $result = 1;
                last;
            }
        }
    }

    return $result;
}


#===  FUNCTION  ================================================================
#         NAME:  run_as
#   PARAMETERS:  uid, command
#      RETURNS:  hash with keys 'resultCode' = resultCode of command and 
#                'output' = program output
#  DESCRIPTION:  Runs command as uid using the sudo utility.
#===============================================================================
sub run_as {
	my ($uid, $command) = @_;
	my $sudo_cmd = `which sudo`;
	chomp($sudo_cmd);
	if(! -x $sudo_cmd) {
		&main::daemon_log("ERROR: The sudo utility is not available! Please fix this!");
	}
	my $cmd_line= "$sudo_cmd su - $uid -c '$command'";
	open(my $PIPE, "$cmd_line |");
	my $result = {'command' => $cmd_line};
	push @{$result->{'output'}}, <$PIPE>;
	close($PIPE);
	my $exit_value = $? >> 8;
	$result->{'resultCode'} = $exit_value;
	return $result;
}


#===  FUNCTION  ================================================================
#         NAME:  inform_other_si_server
#   PARAMETERS:  message
#      RETURNS:  nothing
#  DESCRIPTION:  Sends message to all other SI-server found in known_server_db. 
#===============================================================================
sub inform_all_other_si_server {
    my ($msg) = @_;

    # determine all other si-server from known_server_db
    my $sql_statement= "SELECT * FROM $main::known_server_tn";
    my $res = $main::known_server_db->select_dbentry( $sql_statement ); 

    while( my ($hit_num, $hit) = each %$res ) {    
        my $act_target_address = $hit->{hostname};
        my $act_target_key = $hit->{hostkey};

        # determine the source address corresponding to the actual target address
        my ($act_target_ip, $act_target_port) = split(/:/, $act_target_address);
        my $act_source_address = &main::get_local_ip_for_remote_ip($act_target_ip).":$act_target_port";

        # fill into message the correct target and source addresses
        my $act_msg = $msg;
        $act_msg =~ s/<target>\w*<\/target>/<target>$act_target_address<\/target>/g;
        $act_msg =~ s/<source>\w*<\/source>/<source>$act_source_address<\/source>/g;

        # send message to the target
        &main::send_msg_to_target($act_msg, $act_target_address, $act_target_key, "foreign_job_updates" , "J");
    }

    return;
}


sub read_configfile {
    my ($cfg_file, %cfg_defaults) = @_ ;
    my $cfg;
    if( defined( $cfg_file) && ( (-s $cfg_file) > 0 )) {
        if( -r $cfg_file ) {
            $cfg = Config::IniFiles->new( -file => $cfg_file, -nocase => 1 );
        } else {
            print STDERR "Couldn't read config file!";
        }
    } else {
        $cfg = Config::IniFiles->new() ;
    }
    foreach my $section (keys %cfg_defaults) {
        foreach my $param (keys %{$cfg_defaults{ $section }}) {
            my $pinfo = $cfg_defaults{ $section }{ $param };
           ${@$pinfo[ 0 ]} = $cfg->val( $section, $param, @$pinfo[ 1 ] );
        }
    }
}


sub check_opsi_res {
    my $res= shift;

    if($res) {
        if ($res->is_error) {
            my $error_string;
            if (ref $res->error_message eq "HASH") { 
				# for different versions
                $error_string = $res->error_message->{'message'}; 
				$_ = $res->error_message->{'message'};
            } else { 
				# for different versions
                $error_string = $res->error_message; 
				$_ = $res->error_message;
            }
            return 1, $error_string;
        }
    } else {
		# for different versions
		$_ = $main::opsi_client->status_line;
        return 1, $main::opsi_client->status_line;
    }
    return 0;
}

sub calc_timestamp {
    my ($timestamp, $operation, $value, $entity) = @_ ;
	$entity = defined $entity ? $entity : "seconds";
    my $res_timestamp = 0;
    
    $value = int($value);
    $timestamp = int($timestamp);
    $timestamp =~ /(\d{4})(\d\d)(\d\d)(\d\d)(\d\d)(\d\d)/;
    my $dt = DateTime->new( year   => $1,
            month  => $2,
            day    => $3,
            hour   => $4,
            minute => $5,
            second => $6,
            );

    if ($operation eq "plus" || $operation eq "+") {
        $dt->add($entity => $value);
        $res_timestamp = $dt->ymd('').$dt->hms('');
    }

    if ($operation eq "minus" || $operation eq "-") {
        $dt->subtract($entity => $value);
        $res_timestamp = $dt->ymd('').$dt->hms('');
    }

    return $res_timestamp;
}

sub opsi_callobj2string {
    my ($callobj) = @_;
    my @callobj_string;
    while(my ($key, $value) = each(%$callobj)) {
        my $value_string = "";
        if (ref($value) eq "ARRAY") {
            $value_string = join(",", @$value);
        } else {
            $value_string = $value;
        }
        push(@callobj_string, "$key=$value_string")
    }
    return join(", ", @callobj_string);
}

1;
