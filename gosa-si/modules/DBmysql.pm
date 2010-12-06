package GOsaSI::DBmysql;

use strict;
use warnings;

use Data::Dumper;
use Time::HiRes qw(usleep);
use GOsaSI::GosaSupportDaemon;

use DBI;

my $col_names = {};

sub new {
    my $class = shift;
    my $database = shift;
    my $host = shift;
    my $username = shift;
    my $password = shift;

    my $self = {dbh=>undef};
    my $dbh = DBI->connect("dbi:mysql:database=$database;host=$host", $username, $password,{ RaiseError => 1, AutoCommit => 1 });
		$dbh->{mysql_auto_reconnect} = 1;
    $self->{dbh} = $dbh;
    bless($self,$class);

    return($self);
}


sub create_table {
	my $self = shift;
	my $table_name = shift;
	my $col_names_ref = shift;
	my $recreate_table = shift || 0;
	my @col_names;
	my $col_names_string = join(", ", @$col_names_ref);

	if($recreate_table) {
		$self->{dbh}->do("DROP TABLE $table_name");
	}
	my $sql_statement = "CREATE TABLE IF NOT EXISTS $table_name ( $col_names_string ) ENGINE=INNODB"; 
	# &main::daemon_log("DEBUG: $sql_statement");
	eval {
		$self->{dbh}->do($sql_statement);
	};
	if($@) {
		&main::daemon_log("ERROR: $sql_statement failed with $@", 1);
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
		eval {
			# &main::daemon_log("DEBUG: $sql_statement");
			my $sth = $self->{dbh}->prepare($sql_statement);
			$sth->execute;
			$res = @{ $sth->fetchall_arrayref() };
			$sth->finish;
		};
		if($@) {
			&main::daemon_log("ERROR: $sql_statement failed with $@", 1);
		}

	}

	# primkey is unique or no primkey specified -> run insert
	if ($res == 0) {
		# fetch column names of table
		my $col_names = &get_table_columns($self, $table);

		#my $create_id=0;
		#foreach my $col_name (@{$col_names}) {
		#	#if($col_name eq "id" && (! exists $arg->{$col_name})) {
		#		#&main::daemon_log("0 DEBUG: id field found without value! Creating autoincrement statement!", 7);
		#		$create_id=1;
		#	}
		#}

		# assign values to column name variables
		my @col_list;
		my @val_list;
		foreach my $col_name (@{$col_names}) {
			# use function parameter for column values
			if (exists $arg->{$col_name}) {
				push(@col_list, $col_name);
				push(@val_list, "'".$arg->{$col_name}."'");
			}
		}    

		my $sql_statement;
		#if($create_id==1) {
		#	$sql_statement = "INSERT INTO $table (id, ".join(", ", @col_list).") VALUES ((select coalesce(max(id),0)+1), ".join(", ", @val_list).")";
		#} else {
			$sql_statement = "INSERT INTO $table (".join(", ", @col_list).") VALUES (".join(", ", @val_list).")";
		#}
		my $db_res;
		# &main::daemon_log("DEBUG: $sql_statement",1);
		eval {
			$db_res = $self->{dbh}->do($sql_statement);
		};
		if($@) {
			&main::daemon_log("ERROR: $sql_statement failed with $@", 1);
		}

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
    my $db_answer= &exec_statement($self, $sql); 
    return $db_answer;
}


sub del_dbentry {
    my ($self, $sql)= @_;
    my $db_res= &exec_statement($self, $sql);
    return $db_res;
}


sub get_table_columns {
    my $self = shift;
    my $table = shift;
	my @column_names;

	my @res;
	eval {
		my $sth = $self->{dbh}->prepare("describe $table") or &main::daemon_log("ERROR: Preparation of statement 'describe $table' failed!", 1);
		$sth->execute or &main::daemon_log("ERROR: Execution of statement 'describe $table' failed!", 1);
		@res = @{ $sth->fetchall_arrayref() };
		$sth->finish or &main::daemon_log("ERROR: Finishing the statement handle failed!", 1);
	};
	if($@) {
		&main::daemon_log("ERROR: describe ('$table') failed with $@", 1);
	}

	foreach my $column (@res) {
		push(@column_names, @$column[0]);
	}

	return \@column_names;
}


sub select_dbentry {
    my ($self, $sql)= @_;
    my $error= 0;
    my $answer= {};
    my $db_answer= &exec_statement($self, $sql); 
    my @column_list;

    # fetch column list of db and create a hash with column_name->column_value of the select query
    $sql =~ /SELECT ([\S\s]*?) FROM ([\S]*?)( |$)/g;
    my $selected_cols = $1;
    my $table = $2;

    # all columns are used for creating answer
    if ($selected_cols eq '*') {
        @column_list = @{ &get_table_columns($self, $table) };    

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
    my $table_name = shift;

    my $sql_statement= "SELECT * FROM $table_name ORDER BY timestamp";
    my $res= &exec_statement($self, $sql_statement);
    my @answer;
    foreach my $hit (@{$res}) {
        push(@answer, "hit: ".join(', ', @{$hit}));
    }

    return join("\n", @answer);
}


sub exec_statement {
	my $self = shift;
	my $sql_statement = shift;
	my $sth;
	my @db_answer;

	# print STDERR Dumper($sql_statement);
#	eval {
		if($sql_statement =~ /^SELECT/i) {
			$sth = $self->{dbh}->prepare($sql_statement) or &main::daemon_log("0 ERROR: Preparation of statement '$sql_statement' failed!", 1);
			$sth->execute or &main::daemon_log("0 ERROR: Execution of statement '$sql_statement' failed!", 1);
			if($sth->rows > 0) {
				@db_answer = @{ $sth->fetchall_arrayref() } or &main::daemon_log("0 ERROR: Fetch() failed!", 1);
				# print STDERR Dumper(@db_answer);
			}
			$sth->finish or &main::daemon_log("0 ERROR: Finishing the statement handle failed!", 1);
		} else {
			$self->{dbh}->do($sql_statement);
		}
#	};
#	if($@) {
#		&main::daemon_log("0 ERROR: '$sql_statement' failed with '$@'", 1);
#	}
	# TODO : maybe an error handling and an erro feedback to invoking function
	my $error = $self->{dbh}->err;
	if ($error) {
		&main::daemon_log("0 ERROR: ".@$self->{dbh}->errstr, 1);
	}

	return \@db_answer;
}


sub exec_statementlist {
	my $self = shift;
	my $sql_list = shift;
	my @db_answer;

	foreach my $sql (@$sql_list) {
		if(defined($sql) && length($sql) > 0) {
			# &main::daemon_log("DEBUG: $sql");
			eval {
				if($sql =~ /^SELECT/i) {
					my $sth = $self->{dbh}->prepare($sql);
					# &main::daemon_log("DEBUG: ".$sth->execute);
					if($sth->rows > 0) {
						my @answer = @{$sth->fetchall_arrayref()};
						push @db_answer, @answer;
					}
					$sth->finish;
				} else {
					$self->{dbh}->do($sql);
				}
			};
			if($@) {
				&main::daemon_log("ERROR: $sql failed with $@", 1);
			}
		} else {
			next;
		}
	}

	return \@db_answer;
}


sub count_dbentries {
    my ($self, $table)= @_;
    my $error= 0;
    my $answer= -1;
    
    my $sql_statement= "SELECT * FROM $table";
    my $db_answer= &select_dbentry($self, $sql_statement); 

    my $count = keys(%{$db_answer});
    return $count;
}


sub move_table {
	my ($self, $from, $to) = @_;

	my $sql_statement_drop = "DROP TABLE IF EXISTS $to";
	my $sql_statement_alter = "ALTER TABLE $from RENAME TO $to";

	eval {
		$self->{dbh}->do($sql_statement_drop);
		$self->{dbh}->do($sql_statement_alter);
	};

	if($@) {
		&main::daemon_log("ERROR: $sql_statement_drop failed with $@", 1);
	}

	return;
} 


1;
