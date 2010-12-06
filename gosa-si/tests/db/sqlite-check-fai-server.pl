#!/usr/bin/perl 
#===============================================================================
#
#         FILE:  DBD-SQlite.pl
#
#        USAGE:  ./DBD-SQlite.pl 
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
#      CREATED:  20.12.2007 08:54:52 CET
#     REVISION:  ---
#===============================================================================

use strict;
use warnings;
use GOSA::DBsqlite;
use Data::Dumper;

sub daemon_log {}
print "START\n";
my $res;
my $db_name;

$db_name = "/var/lib/gosa-si/fai_server.db";
if (-e $db_name) {
    print "\n############################################################\n";
    my $table_name = "fai_server";
    print "$db_name\n";
    print "$table_name\n";

    my $sqlite = GOSA::DBsqlite->new($db_name);
    my $col_names = $sqlite->get_table_columns($table_name);
    print join(', ', @{ $col_names } )."\n" ;
    my $answer = $sqlite->show_table($table_name);
    print $answer."\n";
}

print "\nFINISH\n";
