#!/usr/bin/perl 
#===============================================================================
#
#         FILE:  deploy-gosa-si.pl
#
#        USAGE:  ./deploy-gosa-si.pl 
#
#  DESCRIPTION:  
#
#      OPTIONS:  ---
# REQUIREMENTS:  ---
#         BUGS:  ---
#        NOTES:  ---
#       AUTHOR:   (), <>
#      COMPANY:  
#      VERSION:  1.0
#      CREATED:  22.04.2008 11:28:43 CEST
#     REVISION:  ---
#===============================================================================

use strict;
use warnings;
use File::Spec;
use Data::Dumper;

my $test_path = File::Spec->rel2abs(File::Spec->curdir());
my @gosa_dir = File::Spec->splitdir($test_path);
pop(@gosa_dir);
my $gosa_path = File::Spec->catdir(@gosa_dir);

my %copies = (
        "/usr/sbin/gosa-si-server" => "gosa-si-server",
        "/usr/sbin/gosa-si-client" => "gosa-si-client",

        "/usr/lib/gosa-si/modules/GosaPackages.pm"    => "modules/GosaPackages.pm",
        "/usr/lib/gosa-si/modules/ClientPackages.pm"  => "modules/ClientPackages.pm",
        "/usr/lib/gosa-si/modules/ServerPackages.pm"  => "modules/ServerPackages.pm",
        "/usr/lib/gosa-si/modules/ArpHandler.pm"      => "modules/ArpHandler.pm",
        
        "/usr/share/perl5/GOSA/DBsqlite.pm"            => "modules/DBsqlite.pm",
        "/usr/share/perl5/GOSA/DBmysql.pm"             => "modules/DBmysql.pm",
        "/usr/share/perl5/GOSA/GosaSupportDaemon.pm"   => "modules/GosaSupportDaemon.pm",
        
        "/usr/lib/gosa-si/server/ClientPackages/clMessages.pm"  => "server/events/clMessages.pm",
        "/usr/lib/gosa-si/server/ClientPackages/siTriggered.pm" => "server/events/siTriggered.pm",
        
        "/usr/lib/gosa-si/server/GosaPackages/databases.pm"        => "server/events/databases.pm",
        "/usr/lib/gosa-si/server/GosaPackages/gosaTriggered.pm"    => "server/events/gosaTriggered.pm",
        "/usr/lib/gosa-si/server/GosaPackages/logHandling.pm"      => "server/events/logHandling.pm",
        "/usr/lib/gosa-si/server/GosaPackages/mailqueue_com.pm"    => "server/events/mailqueue_com.pm",
        "/usr/lib/gosa-si/server/GosaPackages/opsi_com.pm"         => "server/events/opsi_com.pm",
        
        "/usr/lib/gosa-si/server/ServerPackages/opsi_com.pm"           => "server/events/opsi_com.pm",
        "/usr/lib/gosa-si/server/ServerPackages/databases.pm"          => "server/events/databases.pm" ,
        "/usr/lib/gosa-si/server/ServerPackages/gosaTriggered.pm"      => "server/events/gosaTriggered.pm" ,
        "/usr/lib/gosa-si/server/ServerPackages/logHandling.pm"        => "server/events/logHandling.pm",
        "/usr/lib/gosa-si/server/ServerPackages/mailqueue_com.pm"      => "server/events/mailqueue_com.pm" ,
        "/usr/lib/gosa-si/server/ServerPackages/server_server_com.pm"  => "server/events/server_server_com.pm" ,
        
        "/usr/lib/gosa-si/client/events/corefunctions.pm"  => "client/events/corefunctions.pm",
        "/usr/lib/gosa-si/client/events/dak.pm"            => "client/events/dak.pm" ,
        "/usr/lib/gosa-si/client/events/gosaTriggered.pm"  => "client/events/gosaTriggered.pm",
        "/usr/lib/gosa-si/client/events/installation.pm"   => "client/events/installation.pm",
        "/usr/lib/gosa-si/client/events/mailqueue.pm"      => "client/events/mailqueue.pm",
        "/usr/lib/gosa-si/client/events/load_reporter.pm"  => "client/events/load_reporter.pm",
);

while( my($new_file, $file_name) = each %copies ) {
    #print STDERR "copy ../$file_name to $new_file\n"; 
    #system("cp ../$file_name $new_file"); 
    
    my $del_cmd = "rm -rf $new_file"; 
    print STDERR "$del_cmd\n";
    system($del_cmd);
    
    my $abs_file = File::Spec->catfile($gosa_path, $file_name);
    my $ln_cmd = "ln -s $abs_file $new_file"; 
    print STDERR "$ln_cmd\n"; 
    system($ln_cmd);
    
    print STDERR "\n"; 
}


