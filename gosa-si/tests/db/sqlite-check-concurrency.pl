#!/usr/bin/perl -W

package GOSA::DBsqlite;
use strict;
use warnings;

use DBI;
use Data::Dumper;
use Time::HiRes qw(usleep);
use Fcntl ':flock';
use threads;


sub daemon_log {}

my %threads;
# Count of threads, if > 1 it corrupts the db
my $count= 10;
my $db_name= "./test.sqlite";
my $lock = $db_name.".si.lock";

#if(stat($lock)) {
#	unlink($lock);
#}
#
#if(stat($db_name)) {
#	unlink($db_name)
#}

for(my $i=0;$i<$count;$i++) {
	$threads{$i}= threads->create(\&check_database);
}

foreach my $thread (threads->list()) {
	$thread->join();
}

sub check_database {
	$threads{threads->self->tid()}= GOSA::DBsqlite->new($db_name);
	threads->yield();
	$threads{threads->self->tid()}->run_test("test");

	return;
}

sub new {
	my $class = shift;
	my $db_name = shift;

	my $self = {dbh=>undef,db_name=>undef,db_lock=>undef,db_lock_handle=>undef};
	my $dbh = DBI->connect("dbi:SQLite:dbname=$db_name", "", "", {RaiseError => 1, AutoCommit => 1});
	$self->{dbh} = $dbh;
	$self->{db_name} = $db_name;
	$self->{db_lock} = $lock;
	bless($self,$class);
	return($self);
}

sub lock {
	my $self = shift;
	open($self->{db_lock_handle}, ">>".($self->{db_lock})) unless ref $self->{db_lock_handle};
	flock($self->{db_lock_handle},LOCK_EX);
	seek($self->{db_lock_handle}, 0, 2);
}


sub unlock {
	my $self = shift;
	flock($self->{db_lock_handle},LOCK_UN);
}

sub run_test {
	my $self= shift;
	my $table_name= shift;
	my $sql= "CREATE TABLE IF NOT EXISTS $table_name (id INTEGER PRIMARY KEY, status VARCHAR(255) DEFAULT 'none')";
	$self->lock();
	eval {
		$self->{dbh}->do($sql);
	};
	if($@) {
		print STDERR Dumper($@);
	}
	$self->unlock();

	for(my $i=0;$i<100;$i++) {
		$sql= "INSERT INTO $table_name (id, status) VALUES (null, 'test $i')";
		$self->lock();
		eval {
			$self->{dbh}->do($sql);
		};
		if($@) {
			print STDERR Dumper($@);
		}
		$self->unlock();
	}
}
