#!/usr/bin/perl -w


# Copyright (C) 2005 Guillaume Delecourt <guillaume.delecourt@opensides.be>
# Copyright (C) 2005 Vincent Senave <vincent.senave@opensides.be>
# Copyright (C) 2005-2009 Benoit Mortier <benoit.mortier@opensides.be>
#
#
# This program is free software; you can redistribute it and/or modify
# it under the terms of the GNU General Public License as published by
# the Free Software Foundation; either version 2 of the License, or
# (at your option) any later version.
#
# This program is distributed in the hope that it will be useful,
# but WITHOUT ANY WARRANTY; without even the implied warranty of
# MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
# GNU General Public License for more details.
#
# You should have received a copy of the GNU General Public License
# along with this program; if not, write to the Free Software
# Foundation, Inc., 59 Temple Place - Suite 330, Boston, MA 02111-1307, USA.
#
#

use Net::LDAP;
use Getopt::Std;
use Net::LDAP::Schema;
use Net::LDAP::LDIF;
use Data::Dumper;
use MIME::Lite;
use Sys::Syslog;
use Switch; 
use strict;

# Variables a config

my $admindef="admin";

my $cgi_file="cgi.cfg";
my $contacts_file="contacts.cfg";
my $contacts_groups_file="contactgroups.cfg";

my $TS_FILE='/tmp/gosa_timestamp';
my %Options;
my $nb_user=0;
my $nb_groupe=0;

my ($i,$file,$ldap,@nagiosmail,
	$line,$text,$mesg,$entry,$userlist1,$userlist2,$userlist3,$userlist4,
	$userlist5,$userlist6,$userlist7,$msg,@groupname,@groupmembers,@contactlias,
	@groupdescription,@servicenotificationoptions,@servicenotificationperiod,
	@hostnotificationoptions,@hostnotificationperiod,$stdout,
	$usercontact,$members,@contactname,@nagiosalias,$j,@entries
);

# The connexion parameters are in gosa_bind.conf
my $gosa_bind_conf="/etc/gosa/gosa_bind.conf";
my $gosa_ldap_conf="/etc/gosa/nagios_ldap.conf";
my %config_bind = &read_conf($gosa_bind_conf);
my %config = &read_conf($gosa_ldap_conf);

my $peopleou=$config{peopleou};
my $groupeou=$config{groupeou};
my $base=$config{base};
my $scope=$config{scope};# par defaut
my $server=$config{server};

my $admin=$config_bind{masterDN};
my $password=$config_bind{masterPw};


	$stdout.="\n\nSearch new Nagios attribute in user list\n";
	$stdout.="-"x55;$stdout.="\n";
	#my $ts = getTS;

# 	$ldap = &anonBind;
# 	$mesg = $ldap->search(
# 	base => $LDAP_BASE,
# 	filter => "(&(modifyTimestamp>=$ts)(!(objectClass=gosaUserTemplate)))"
# 	);

	# Put timestamp to file
	#putTS;

	# Work if changes is present
	#if($mesg->count > 0)
	#{
	#$stdout.="Processing records modified after $ts\n\n";
	$ldap = Net::LDAP->new($server);
	$mesg = $ldap->bind($admin,password=>$password) or syslog('error',$mesg->err) && print $mesg->code && die $mesg->error;

	

	#Part of the ObjectClass NAgios Contact
	$mesg = $ldap->search(filter=>"(&(objectClass~=nagiosContact))", base=>$peopleou,scope=>$scope);
	@entries = $mesg->entries;
	$i=0;
	foreach $entry (@entries) {
	$stdout.="\nContact $i : \nName\t\t\t";$contactname[$i]=$entry->get_value('uid');$stdout.=$contactname[$i];
	$stdout.="\n\n\tmail:\t\t\t\t";$nagiosmail[$i]=$entry->get_value('NagiosMail');$stdout.=$nagiosmail[$i];
	$stdout.="\n\talias:\t\t\t\t";$nagiosalias[$i]=$entry->get_value('NagiosAlias');$stdout.=$nagiosalias[$i];
	$stdout.="\n\tHostNotificationPeriod:\t\t";$hostnotificationperiod[$i]=$entry->get_value('HostNotificationPeriod');$stdout.=$hostnotificationperiod[$i];
	$stdout.="\n\tServiceNotificationPeriod:\t";$servicenotificationperiod[$i]=$entry->get_value('ServiceNotificationPeriod');$stdout.=$servicenotificationperiod[$i];
	$stdout.="\n\tHostNotificationOptions:\t";$hostnotificationoptions[$i]=$entry->get_value('HostNotificationOptions');$stdout.=$hostnotificationoptions[$i];
	$stdout.="\n\tServiceNotificationOptions:\t";$servicenotificationoptions[$i]=$entry->get_value('ServiceNotificationOptions');$stdout.=$servicenotificationoptions[$i];
	$stdout.="\n"." "x15;$stdout.="-"x20;$stdout.=" "x 15;				
	$usercontact.=$entry->get_value('uid')."  ,";
	$i++;
	}
	$nb_user=$i;
		
		
	#Part of the ObjectClass NAgios Group
	$mesg = $ldap->search(filter=>"(&(objectClass~=nagiosContactGroup))", base=>$groupeou,scope=>$scope);
	@entries = $mesg->entries;
	$i=0;
	foreach $entry (@entries) {
	$stdout.="\nGroupe $i : \nName\t\t";$groupname[$i]=$entry->get_value('cn');$stdout.=$groupname[$i];

	$stdout.="\n\n\talias:\t\t";
	$groupdescription[$i]=$entry->get_value('description');

	if(defined($groupdescription[$i])) { 
		$stdout.=$groupdescription[$i];
	} else { 
		# We need a valid description entry, so we'll just use the groupname
		$stdout.=$groupname[$i];
 	}

	$stdout.="\n\tmembers:\t";
	$j=0;
	foreach $members($entry->get_value('memberUid'))
	{
	$stdout.=$members." ";
	$groupmembers[$i][$j]=$members;
	$j++;
	}
	$stdout.="\n"." "x15;$stdout.="-"x20;$stdout.=" "x 15;			
	$i++;
	}
	$nb_groupe=$i;

	#Part of the ObjectClass NagiosAuth
	$stdout.="\n\n\n\n\nAuthorization for the different Information in Nagios\n"."-" x 53;$stdout.="\n";
	$mesg = $ldap->search(filter=>"(&(objectClass~=nagiosAuth)(AuthorizedSystemInformation~=checked))", base=>$peopleou,scope=>$scope);
	@entries = $mesg->entries;
	$stdout.="\nSystem infos :\t\t";
	foreach $entry (@entries) {
	$stdout.= $entry->get_value('uid')."\t";
	$userlist1.=$entry->get_value('uid').",";
	}
	$userlist1.=$admindef;

	$mesg = $ldap->search(filter=>"(&(objectClass~=nagiosAuth)(AuthorizedConfigurationInformation~=checked))", base=>$peopleou,scope=>$scope);
	@entries = $mesg->entries;
	$stdout.="\nConfiguration infos :\t";
	foreach $entry (@entries) {
	$stdout.= $entry->get_value('uid')."\t";
	$userlist2.=$entry->get_value('uid').",";
	}
	$userlist2.=$admindef;

	$mesg = $ldap->search(filter=>"(&(objectClass~=nagiosAuth)(AuthorizedSystemCommands~=checked))", base=>$peopleou,scope=>$scope);
	@entries = $mesg->entries;
	$stdout.="\nSystem commands : \t";
	foreach $entry (@entries) {
	$stdout.= $entry->get_value('uid')."\t";
	$userlist3.=$entry->get_value('uid').",";
	}
	$userlist3.=$admindef;

	$mesg = $ldap->search(filter=>"(&(objectClass~=nagiosAuth)(AuthorizedAllServices~=checked))", base=>$peopleou,scope=>$scope);
	@entries = $mesg->entries;
	$stdout.="\nAll services :\t\t";
	foreach $entry (@entries) {
	$stdout.= $entry->get_value('uid')."\t";
	$userlist4.=$entry->get_value('uid').",";
	}
	$userlist4.=$admindef;

	$mesg = $ldap->search(filter=>"(&(objectClass~=nagiosAuth)(AuthorizedAllHosts~=checked))", base=>$peopleou,scope=>$scope);
	@entries = $mesg->entries;
	$stdout.="\nAll hosts :\t\t";
	foreach $entry (@entries) {
	$stdout.= $entry->get_value('uid')."\t";
	$userlist5.=$entry->get_value('uid').",";
	}
	$userlist5.=$admindef;


	$mesg = $ldap->search(filter=>"(&(objectClass~=nagiosAuth)(AuthorizedAllServiceCommands~=checked))", base=>$peopleou,scope=>$scope);
	@entries = $mesg->entries;
	$stdout.="\nAll services commands :\t";
	foreach $entry (@entries) {
	$stdout.= $entry->get_value('uid')."\t";
	$userlist6.=$entry->get_value('uid').",";
	}
	$userlist6.=$admindef;

	$mesg = $ldap->search(filter=>"(&(objectClass~=nagiosAuth)(AuthorizedAllHostCommands~=checked))",base=>$peopleou,scope=>$scope);
	@entries = $mesg->entries;
	$stdout.="\nAll host commands :\t";
	foreach $entry (@entries) {
	$stdout.= $entry->get_value('uid')."\t";
	$userlist7.=$entry->get_value('uid').",";
	}
	$userlist7.=$admindef;


	&modiffile_cgi($cgi_file);
	&modiffile_contact($contacts_file);
	&modiffile_group($contacts_groups_file);
	
	$ldap->unbind;
	$stdout.="\n";
	switch($config{stdout})
	{
	case "mail"	{&mail()}
	case "log"	{&writelog()}
	case "normal"	{print $stdout}
	}
	exit(0);

sub modiffile_contact()
{
	$file=$_[0];
	my $text="";
	open(FH,"$file") || die "Can't open file $file";
	$stdout.="\n\n"; $stdout.=" "x10;$stdout.="-"x25;$stdout.=" "x10;
	$stdout.="\n\n$nb_user user(s) added in file $file\n";
	for($i=0;$i<$nb_user;$i++)
	{
		$text.="\n\ndefine contact{\n";
		$text.="\n\tcontact_name \t\t\t".$contactname[$i];
		$text.="\n\talias \t\t\t\t".$nagiosalias[$i];
		$text.="\n\thost_notification_period \t".$hostnotificationperiod[$i];
		$text.="\n\thost_notification_options \t".$hostnotificationoptions[$i];
		$text.="\n\tservice_notification_period \t".$servicenotificationperiod[$i];
		$text.="\n\tservice_notification_options \t".$servicenotificationoptions[$i];
		$text.="\n\tservice_notification_commands \t".$config{service_notification_commands};
		$text.="\n\thost_notification_commands \t".$config{host_notification_commands};
		$text.="\n\temail \t\t\t\t".$nagiosmail[$i];
		$text.="\n}\n\n";
	}
	close(FH);
	open(FH,"> $file") || die "Can't open file $file";
	print  FH "$text";
	close(FH);
	
}

sub modiffile_group()
{
	$file=$_[0];
	$text="";
	$j=0;
	$i=0;
	open(FH,"$file") || die "Can't open $file";
	$stdout.="\n\n"; $stdout.=" "x10;$stdout.="-"x25;$stdout.=" "x10;
	$stdout.="\n\n$nb_groupe group(s) added in file $file\n";
	for($i=0;$i<$nb_groupe;$i++)
	{
		$text.="\n\ndefine contactgroup{\n";
		$text.="\n\tcontactgroup_name \t".$groupname[$i];
		if(defined($groupdescription[$i])) {
			$text.="\n\talias \t\t\t".$groupdescription[$i];
		} else { 
 			# We need a valid alias entry, so we'll just use the groupname
			$text.="\n\talias \t\t\t".$groupname[$i]; 
 		}
		$text.="\n\tmembers \t\t";
		while(defined($groupmembers[$i][$j]))
		{
			$text.=$groupmembers[$i][$j];
 			$j++;
			
			if(defined($groupmembers[$i][$j])) { 
 				$text.=","; 
 			}
		}
		$text.="\n}\n\n";
	}
	
	close(FH);
	open(FH,"> $file") || die "Can't open file $file";
	print FH "$text";
	close(FH);
	
}

sub modiffile_cgi()
{
	$file=$_[0];
	$text="";
	open(FH,"$file") || die "Can't open file $file";
	while(<FH>)
	{	
		$line=$_;
		#$stdout.="$line";
		if($line =~ s/^(authorized_for_system_information=).*$/$1$userlist1/){$text.=$line;}
		elsif($line =~ s/^(authorized_for_configuration_information=).*$/$1$userlist2/){$text.=$line;}
		elsif($line =~ s/^(authorized_for_system_commands=).*$/$1$userlist3/){$text.=$line;}
		elsif($line =~ s/^(authorized_for_all_services=).*$/$1$userlist4/){$text.=$line;}
		elsif($line =~ s/^(authorized_for_all_hosts=).*$/$1$userlist5/){$text.=$line;}
		elsif($line =~ s/^(authorized_for_all_service_commands=).*$/$1$userlist6/){$text.=$line;}
		elsif($line =~ s/^(authorized_for_all_host_commands=).*$/$1$userlist7/){$text.=$line;}
		else {$text.=$line};
	}
	close(FH);
	open(FH,"> $file") || die "Can't open file $file";
	print FH "$text";
	close(FH);
	
}

sub read_conf()
{
        my %conf;
        open (CONFIGFILE, "$_[0]") || die "Can't open $_[0] for reading !\n";
        while (<CONFIGFILE>) {
                chomp($_);
                ## throw away comments
                next if ( /^\s*#/ || /^\s*$/ || /^\s*\;/);
                ## check for a param = value
                my ($parameter,$value)=read_parameter($_);
                $value = &subst_configvar($value,\%conf);
                $conf{$parameter}=$value;
          }
        close (CONFIGFILE);
        return(%conf);
}




sub read_parameter
{
        my $line=shift;
        ## check for a param = value
        if ($_=~/=/) {
          my ($param,$val);
          if ($_=~/"/) {
                #my ($param,$val) = ($_=~/(.*)\s*=\s*"(.*)"/);
                ($param,$val) = /\s*(.*?)\s*=\s*"(.*)"/;
          } elsif ($_=~/'/) {
                ($param,$val) = /\s*(.*?)\s*=\s*'(.*)'/;
          } else {
                ($param,$val) = /\s*(.*?)\s*=\s*(.*)/;
          }
          return ($param,$val);
        }
}

sub subst_configvar
{
        my $value = shift;
        my $vars = shift;

        $value =~ s/\$\{([^}]+)\}/$vars->{$1} ? $vars->{$1} : $1/eg;
        return $value;
}

sub mail
{

if($config{email}eq ""){$config{email}="root"}

$msg = MIME::Lite->new(
             From     => 'monperl@opensides.be',
             To       => $config{email},
             Subject  => "Plugin Nagios Gosa",
             Data     => $stdout
             );


$msg->send;
}

sub writelog
{
	open(F, "> $config{logfile}");
	print F $stdout;
	close(F);
}

# Read timestamp
sub getTS
{
	open(F, "< $TS_FILE");
	my $ts = <F>;
	chop $ts;
	$ts ||= "19700101000000Z";
	return $ts;
}

# save timestamp
sub putTS
{
	my $ts = `date -u '+%Y%m%d%H%M%SZ'`;
	open(F, "> $TS_FILE");
	$stdout.= F $ts;
}

#connexion anonyme
sub anonBind
{
	my $ldap = Net::LDAP->new( $server);
	my $mesg = $ldap->bind();
	$mesg->code && die $mesg->error;
	return $ldap;
}
