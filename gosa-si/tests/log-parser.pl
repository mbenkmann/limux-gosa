#!/usr/bin/perl 
#===============================================================================
#
#         FILE:  log-parser.pl
#
#        USAGE:  ./log-parser.pl 
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
#      CREATED:  13.03.2008 14:51:03 CET
#     REVISION:  ---
#===============================================================================

use strict;
use warnings;
use Getopt::Long;
use Data::Dumper;

my $pattern;
my $log_file = "/var/log/gosa-si-server.log"; 


### MAIN ######################################################################

GetOptions(
		"p|pattern=s" => \$pattern,
		);

open(FILE, "<$log_file") or die "\ncan not open log-file '$log_file'\n"; 
my @lines;
my $messages = {};

# Read lines
while ( my $line = <FILE>){
    chomp($line);
    
	# start of a new message, plot saved log lines
	if ($line =~ /$pattern/ ) {
        print "$line\n";
    }   

}

close FILE;
