package GOsaSI::DBsqlite;

use strict;
use warnings;


use Time::HiRes qw(usleep);
use Data::Dumper;
use GOsaSI::GosaSupportDaemon;

use Fcntl qw/:DEFAULT :flock/; # import LOCK_* constants
use Carp;
use DBI;

our $col_names = {};

sub new {
	my $class = shift;
	my $db_name = shift;

	my $lock = $db_name.".si.lock";
	my $self = {dbh=>undef,db_name=>undef,db_lock=>undef,db_lock_handle=>undef};
	my $dbh = DBI->connect("dbi:SQLite:dbname=$db_name", "", "", {RaiseError => 1, AutoCommit => 1, PrintError => 0});
	
	$self->{dbh} = $dbh;
	$self->{db_name} = $db_name;
	$self->{db_lock} = $lock;
	bless($self,$class);

	my $sth = $self->{dbh}->prepare("pragma integrity_check");
	   $sth->execute();
	my @ret = $sth->fetchall_arrayref();
	   $sth->finish();
	if(length(@ret)==1 && $ret[0][0][0] eq 'ok') {
		&main::daemon_log("0 DEBUG: Database disk image '".$self->{db_name}."' is ok.", 74);
	} else {
		&main::daemon_log("0 ERROR: Database disk image '".$self->{db_name}."' is malformed, creating new database!", 1);
		$self->{dbh}->disconnect() or &main::daemon_log("0 ERROR: Could not disconnect from database '".$self->{db_name}."'!", 1);
		$self->{dbh}= undef;
		unlink($db_name);
	}
	return($self);
}


sub connect {
	my $self = shift;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::connect was called static! Argument was '$self'!", 1);
		return;
	}
		
	$self->{dbh} = DBI->connect("dbi:SQLite:dbname=".$self->{db_name}, "", "", {PrintError => 0, RaiseError => 1, AutoCommit => 1}) or 
	  &main::daemon_log("0 ERROR: Could not connect to database '".$self->{db_name}."'!", 1);

	return;
}


sub disconnect {
	my $self = shift;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::disconnect was called static! Argument was '$self'!", 1);
		return;
	}

	eval {
		$self->{dbh}->disconnect();
	};
  if($@) {
		&main::daemon_log("ERROR: Could not disconnect from database '".$self->{db_name}."'!", 1);
	}

	$self->{dbh}= undef;

	return;
}


sub lock {
	my $self = shift;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::lock was called static! Argument was '$self'!", 1);
		return;
	}

	if(not ref $self->{db_lock_handle} or not fileno $self->{db_lock_handle}) {
		sysopen($self->{db_lock_handle}, $self->{db_lock}, O_RDWR | O_CREAT, 0600) or &main::daemon_log("0 ERROR: Opening the database ".$self->{db_name}." failed with $!", 1);
	}
get_lock:
	my $lock_result = flock($self->{db_lock_handle}, LOCK_EX | LOCK_NB);
	if(not $lock_result) {
		&main::daemon_log("0 ERROR: Could not acquire lock for database ".$self->{db_name}, 1);
		usleep(250+rand(500));
		goto get_lock;
	} else {
		seek($self->{db_lock_handle}, 0, 2);
		&main::daemon_log("0 DEBUG: Acquired lock for database ".$self->{db_name}, 74);
		$self->connect();
	}
	return;
}


sub unlock {
	my $self = shift;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::unlock was called static! Argument was '$self'!", 1);
		return;
	}
	if(not ref $self->{db_lock_handle}) {
		&main::daemon_log("0 BIG ERROR: Lockfile for database ".$self->{db_name}."got closed within critical section!", 1);
	}
	flock($self->{db_lock_handle}, LOCK_UN);
	&main::daemon_log("0 DEBUG: Released lock for database ".$self->{db_name}, 74);
	$self->disconnect();
	return;
}


sub create_table {
	my $self = shift;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::create_table was called static! Statement was '$self'!", 1);
		return;
	}
	my $table_name = shift;
	my $col_names_ref = shift;
	my $index_names_ref = shift || undef;
	my @col_names;
	my @col_names_creation;
	foreach my $col_name (@$col_names_ref) {
		push(@col_names, $col_name);
	}
	$col_names->{ $table_name } = \@col_names;
	my $col_names_string = join(", ", @col_names);
	
	# Not activated yet
	# Check schema
	if($self->check_schema($table_name, $col_names_ref)) {
		$self->exec_statement("DROP TABLE $table_name");
		&main::daemon_log("WARNING: Schema of table $table_name has changed! Table will be recreated!", 3);
	}

	my $sql_statement = "CREATE TABLE IF NOT EXISTS $table_name ( $col_names_string )"; 
	my $res = $self->exec_statement($sql_statement);
	
	# Add indices
	if(defined($index_names_ref) and ref($index_names_ref) eq 'ARRAY') {
		foreach my $index_name (@$index_names_ref) {
			$self->exec_statement("CREATE ".(($index_name eq 'id')?'UNIQUE':'')." INDEX IF NOT EXISTS $index_name on $table_name ($index_name);");
		}
	}

	return 0;
}


sub check_schema {
	my $self = shift;
	my $table_name = shift;
	my $col_names_ref = shift;   # ['id INTEGER PRIMARY KEY', 'timestamp VARCHAR(14) DEFAULT \'none\'', ... ]
	my $col_names_length = @$col_names_ref;

	my $sql = "PRAGMA table_info($table_name)";
	my $res = $self->exec_statement($sql);   # [ ['0', 'id', 'INTEGER', '0', undef, '1' ], ['1', 'timestamp', 'VARCHAR(14)', '0', '\'none\'', '0'], ... ]
	my $db_table_length = @$res;

	# Tabel does not exists, so no differences
	if ($db_table_length == 0)
	{
		return 0;
	}



	# The number of columns is diffrent
	if ($col_names_length != $db_table_length) 
	{
		return 1;
	}

	# The column name and column type to not match
	for (my $i=0; $i < $db_table_length; $i++)
	{
		my @col_names_list = split(" ", @$col_names_ref[$i]);
		if (($col_names_list[0] ne @{@$res[$i]}[1]) || ($col_names_list[1] ne @{@$res[$i]}[2]))
		{
			return 1;
		}
	}


	return 0;
}



sub add_dbentry {
	my $self = shift;
	my $arg = shift;
	my $res = 0;   # default value

	# if dbh not specified, return errorflag 1
	my $table = $arg->{table};
	if( not defined $table ) {
		return 1 ;
	}

	# if timestamp is not provided, add timestamp   
	if( not exists $arg->{timestamp} ) {
		$arg->{timestamp} = &get_time;
	}

	# check primkey and run insert or update
	my $primkeys = $arg->{'primkey'};
	my $prim_statement="";
	if( 0 != @$primkeys ) {   # more than one primkey exist in list
		my @prim_list;
		foreach my $primkey (@$primkeys) {
			if( not exists $arg->{$primkey} ) {
				return (3, "primkey '$primkey' has no value for add_dbentry");
			}
			push(@prim_list, "$primkey='".$arg->{$primkey}."'");
		}
		$prim_statement = "WHERE ".join(" AND ", @prim_list);

		# check wether primkey is unique in table, otherwise return errorflag
		my $sql_statement = "SELECT * FROM $table $prim_statement";
		$res = @{ $self->exec_statement($sql_statement) };
	}

	# primkey is unique or no primkey specified -> run insert
	if ($res == 0) {
		# fetch column names of table
		my $col_names = &get_table_columns($self, $table);

		my $create_id=0;
		foreach my $col_name (@{$col_names}) {
			if($col_name eq "id" && (! exists $arg->{$col_name})) {
				$create_id=1;
			}
		}
		# assign values to column name variables
		my @col_list;
		my @val_list;
		foreach my $col_name (@{$col_names}) {
			# use function parameter for column values
			if (exists $arg->{$col_name}) {
				push(@col_list, "'".$col_name."'");
				push(@val_list, "'".$arg->{$col_name}."'");
			}
		}

		my $sql_statement;
		if($create_id==1) {
			$sql_statement = "INSERT INTO $table (id, ".join(", ", @col_list).") VALUES (null, ".join(", ", @val_list).")";
		} else {
			$sql_statement = "INSERT INTO $table (".join(", ", @col_list).") VALUES (".join(", ", @val_list).")";
		}
		my $db_res;
		my $success=0;
		$self->lock();
		eval {
			my $sth = $self->{dbh}->prepare($sql_statement);
			$db_res = $sth->execute();
			$sth->finish();
			&main::daemon_log("0 DEBUG: Execution of statement '$sql_statement' succeeded!", 74);
			$success = 1;
		};
		if($@) {
			eval {
				$self->{dbh}->do("ANALYZE");
				$self->{dbh}->do("VACUUM");
			};
		}
		if($success==0) {
			eval {
				my $sth = $self->{dbh}->prepare($sql_statement);
				$db_res = $sth->execute();
				$sth->finish();
				&main::daemon_log("0 DEBUG: Execution of statement '$sql_statement' succeeded!", 74);
				$success = 1;
			};
			if($@) {
				eval {
					$self->{dbh}->do("ANALYZE");
					$self->{dbh}->do("VACUUM");
				};
			}
		}
		if($success==0) {
			eval {
				my $sth = $self->{dbh}->prepare($sql_statement);
				$db_res = $sth->execute();
				$sth->finish();
				&main::daemon_log("0 DEBUG: Execution of statement '$sql_statement' succeeded!", 74);
				$success = 1;
			};
			if($@) {
				&main::daemon_log("0 ERROR: Execution of statement '$sql_statement' failed with $@", 1);
			}
		}
		$self->unlock();

		if( $db_res != 1 ) {
			return (4, $sql_statement);
		}

		# entry already exists -> run update
	} else  {
		my @update_l;
		while( my ($pram, $val) = each %{$arg} ) {
			if( $pram eq 'table' ) { next; }
			if( $pram eq 'primkey' ) { next; }
			push(@update_l, "$pram='$val'");
		}
		my $update_str= join(", ", @update_l);
		$update_str= " SET $update_str";

		my $sql_statement= "UPDATE $table $update_str $prim_statement";
		my $db_res = &update_dbentry($self, $sql_statement );
	}

	return 0;
}


sub update_dbentry {
	my ($self, $sql)= @_;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::update_dbentry was called static! Statement was '$self'!", 1);
		return;
	}
	my $db_answer= $self->exec_statement($sql); 
	return $db_answer;
}


sub del_dbentry {
	my ($self, $sql)= @_;;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::del_dbentry was called static! Statement was '$self'!", 1);
		return;
	}
	my $db_res= $self->exec_statement($sql);
	return $db_res;
}


sub get_table_columns {
	my $self = shift;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::get_table_columns was called static! Statement was '$self'!", 1);
		return;
	}
	my $table = shift;
	my @column_names;

	if(exists $col_names->{$table}) {
		foreach my $col_name (@{$col_names->{$table}}) {
			push @column_names, ($1) if $col_name =~ /^(.*?)\s.*$/;
		}
	} else {
		my @res;
		foreach my $column ( @{ $self->exec_statement ( "pragma table_info('$table')" ) } ) {
			push(@column_names, @$column[1]);
		}
	}

	return \@column_names;
}


sub select_dbentry {
	my ($self, $sql)= @_;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::select_dbentry was called static! Statement was '$self'!", 1);
		return;
	}
	my $error= 0;
	my $answer= {};
	my $db_answer= $self->exec_statement($sql); 
	my @column_list;

	# fetch column list of db and create a hash with column_name->column_value of the select query
	$sql =~ /SELECT ([\S\s]*?) FROM ([\S]*?)( |$)/g;
	my $selected_cols = $1;
	my $table = $2;

	# all columns are used for creating answer
	if ($selected_cols eq '*') {
		@column_list = @{ $self->get_table_columns($table) };    

		# specific columns are used for creating answer
	} else {
		# remove all blanks and split string to list of column names
		$selected_cols =~ s/ //g;          
		@column_list = split(/,/, $selected_cols);
	}

	# create answer
	my $hit_counter = 0;
	my $list_len = @column_list;
	foreach my $hit ( @{$db_answer} ){
		$hit_counter++;
		for ( my $i = 0; $i < $list_len; $i++) {
			$answer->{ $hit_counter }->{ $column_list[$i] } = @{ $hit }[$i];
		}
	}

	return $answer;  
}


sub show_table {
	my $self = shift;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::show_table was called static! Statement was '$self'!", 1);
		return;
	}
	my $table_name = shift;

	my $sql_statement= "SELECT * FROM $table_name ORDER BY timestamp";
	my $res= $self->exec_statement($sql_statement);
	my @answer;
	foreach my $hit (@{$res}) {
		push(@answer, "hit: ".join(', ', @{$hit}));
	}

	return join("\n", @answer);
}


sub exec_statement {
	my $self = shift;
	my $sql_statement = shift;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::exec_statement was called static! Statement was '$self'!", 1);
		return;
	}

	if(not defined($sql_statement) or length($sql_statement) == 0) {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::exec_statement was called with empty statement!", 1);
		return;
	}

	my @db_answer;
	my $success= 0;
	$self->lock();
	# Give three chances to the sqlite database
	# 1st chance
	eval {
		my $sth = $self->{dbh}->prepare($sql_statement);
		my $res = $sth->execute();
		@db_answer = @{$sth->fetchall_arrayref()};
		$sth->finish();
		$success=1;
		&main::daemon_log("0 DEBUG: $sql_statement succeeded.", 74);
	};
	if($@) {
		eval {
			$self->{dbh}->do("ANALYZE");
			$self->{dbh}->do("VACUUM");
			$self->{dbh}->do("pragma integrity_check");
		};
	}
	if($success) {
		$self->unlock();
		return \@db_answer ;
	}

	# 2nd chance
	eval {
		usleep(200);
		my $sth = $self->{dbh}->prepare($sql_statement);
		my $res = $sth->execute();
		@db_answer = @{$sth->fetchall_arrayref()};
		$sth->finish();
		$success=1;
		&main::daemon_log("0 DEBUG: $sql_statement succeeded.", 74);
	};
	if($@) {
		eval {
			$self->{dbh}->do("ANALYZE");
			$self->{dbh}->do("VACUUM");
			$self->{dbh}->do("pragma integrity_check");
		};
	}
	if($success) {
		$self->unlock();
		return \@db_answer ;
	}

	# 3rd chance
	eval {
		usleep(200);
		DBI->trace(6) if($main::verbose >= 7);
		my $sth = $self->{dbh}->prepare($sql_statement);
		my $res = $sth->execute();
		@db_answer = @{$sth->fetchall_arrayref()};
		$sth->finish();
		DBI->trace(0);
		&main::daemon_log("0 DEBUG: $sql_statement succeeded.", 74);
	};
	if($@) {
		DBI->trace(0);
		&main::daemon_log("ERROR: $sql_statement failed with $@", 1);
	}
	# TODO : maybe an error handling and an erro feedback to invoking function
	#my $error = @$self->{dbh}->err;
	#if ($error) {
	#	my $error_string = @$self->{dbh}->errstr;
	#}

	$self->unlock();
	return \@db_answer;
}


sub exec_statementlist {
	my $self = shift;
	my $sql_list = shift;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::exec_statementlist was called static!", 1);
		return;
	}
	my @db_answer;

	foreach my $sql_statement (@$sql_list) {
		if(defined($sql_statement) && length($sql_statement) > 0) {
			push @db_answer, $self->exec_statement($sql_statement);
		} else {
			next;
		}
	}

	return \@db_answer;
}


sub count_dbentries {
	my ($self, $table)= @_;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::count_dbentries was called static!", 1);
		return;
	}
	my $error= 0;
	my $count= -1;

	my $sql_statement= "SELECT count() FROM $table";
	my $db_answer= $self->select_dbentry($sql_statement); 
	if(defined($db_answer) && defined($db_answer->{1}) && defined($db_answer->{1}->{'count()'})) {
		$count = $db_answer->{1}->{'count()'};
	}

	return $count;
}


sub move_table {
	my ($self, $from, $to) = @_;
	if(not defined($self) or ref($self) ne 'GOsaSI::DBsqlite') {
		&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::move_table was called static!", 1);
		return;
	}

	my $sql_statement_drop = "DROP TABLE IF EXISTS $to";
	my $sql_statement_alter = "ALTER TABLE $from RENAME TO $to";
	my $success = 0;

	$self->lock();
	eval {
		$self->{dbh}->begin_work();
		$self->{dbh}->do($sql_statement_drop);
		$self->{dbh}->do($sql_statement_alter);
		$self->{dbh}->commit();
		$success = 1;
	};
	if($@) {
		$self->{dbh}->rollback();
		eval {
			$self->{dbh}->do("ANALYZE");
		};
		if($@) {
			&main::daemon_log("ERROR: 'ANALYZE' on database '".$self->{db_name}."' failed with $@", 1);
		}
		eval {
			$self->{dbh}->do("VACUUM");
		};
		if($@) {
			&main::daemon_log("ERROR: 'VACUUM' on database '".$self->{db_name}."' failed with $@", 1);
		}
	}

	if($success == 0) {
		eval {
			$self->{dbh}->begin_work();
			$self->{dbh}->do($sql_statement_drop);
			$self->{dbh}->do($sql_statement_alter);
			$self->{dbh}->commit();
			$success = 1;
		};
		if($@) {
			$self->{dbh}->rollback();
			eval {
				$self->{dbh}->do("ANALYZE");
			};
			if($@) {
				&main::daemon_log("ERROR: 'ANALYZE' on database '".$self->{db_name}."' failed with $@", 1);
			}
			eval {
				$self->{dbh}->do("VACUUM");
			};
			if($@) {
				&main::daemon_log("ERROR: 'VACUUM' on database '".$self->{db_name}."' failed with $@", 1);
			}
		}
	}
	
	if($success == 0) {
		eval {
			$self->{dbh}->begin_work();
			$self->{dbh}->do($sql_statement_drop);
			$self->{dbh}->do($sql_statement_alter);
			$self->{dbh}->commit();
			$success = 1;
		};
		if($@) {
			$self->{dbh}->rollback();
			&main::daemon_log("0 ERROR: GOsaSI::DBsqlite::move_table crashed! Operation failed with $@", 1);
		}
	}

	&main::daemon_log("0 INFO: GOsaSI::DBsqlite::move_table: Operation successful!", 7);
	$self->unlock();

	return;
} 


1;
