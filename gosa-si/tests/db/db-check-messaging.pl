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
use GOSA::DBmysql;
use Data::Dumper;

print "START\n";
my $table_name = "messaging";

print "\n############################################################\n";
print "$table_name\n";

my $dbh = GOSA::DBmysql->new('gosa_si', '127.0.0.1', 'gosa_si', 'gosa');

my $col_names = $dbh->get_table_columns($table_name);
print join(', ', @{ $col_names } )."\n" ;

my $answer = $dbh->show_table($table_name);
print $answer."\n";


print "\nFINISH\n";
