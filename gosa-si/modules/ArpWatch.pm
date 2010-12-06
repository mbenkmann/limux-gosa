#!/usr/bin/perl
package POE::Component::ArpWatch;

use strict;
use warnings;

BEGIN{
	eval('use POE');
	eval('use POE::Component::Pcap');
	eval('use NetPacket::Ethernet qw( :types )');
	eval('use NetPacket::ARP qw( :opcodes )');
}

END{
}

## Map arp opcode #'s to strings
my %arp_opcodes = (
		   NetPacket::ARP::ARP_OPCODE_REQUEST, 'ARP Request',
		   NetPacket::ARP::ARP_OPCODE_REPLY, 'ARP Reply',
		   NetPacket::ARP::RARP_OPCODE_REQUEST, 'RARP Request',
		   NetPacket::ARP::RARP_OPCODE_REPLY, 'RARP Reply',
		  );

##
## POE::Component::ArpWatch->spawn(
##				   [ Alias => 'arp_watch' ],
##				   [ Device => 'eth0' ],
##				   [ Dispatch => dispatch_state ],
##				   [ Session => dispatch_session ],
##				  )
##
sub spawn {
  my $class = shift;
  my %args = @_;

  $args{ Alias } ||= 'arp_watch';

  POE::Session->create( 
		       inline_states => {
					 _start => \&_start,
#					 _signal => \&_signal,
					 _stop => \&_stop,,
					 _dispatch => \&_dispatch,
					 set_dispatch => \&set_dispatch,
					 run => \&run,
					 pause => \&pause,
					 shutdown => \&shutdown,
					},
		       args => [
				$args{ Alias },   # ARG0
				$args{ Device },  # ARG1
				$args{ Dispatch },# ARG2
				$args{ Session }, # ARG3
			       ],
		      );

  return $args{ Alias };
}

sub _start {
  my ($kernel, $heap, $session, 
      $alias, $device, $target_state, $target_session ) 
    = @_[ KERNEL, HEAP, SESSION, ARG0..ARG3 ];

  POE::Component::Pcap->spawn( 
			      Alias => $alias . '_pcap',
			      Device => $device, 
			      Filter => 'arp',
			      Dispatch => '_dispatch',
			      Session => $session,
			     );

  $heap->{'pcap_session'} = $kernel->alias_resolve( $alias . '_pcap' );

  ## Set alias for ourselves and remember it
  $kernel->alias_set( $alias );
  $heap->{Alias} = $alias;

  ## Set dispatch target session and state if it was given
  if( defined( $target_session ) ) {
    $heap->{'target_session'} = $target_session;
    $heap->{'target_state'} = $target_state;
  }
}

sub set_dispatch {
  my( $heap, $sender, $target ) = @_[ HEAP, SENDER, ARG0 ];

  if( defined $target ) {
    ## Remember whome to forward results to
    $heap->{'target_session'} = $sender;
    $heap->{'target_state'} = $target;
  } else {
    ## Clear target
    delete $heap->{'target_session'};
    delete $heap->{'target_state'};
  }
}

sub run {
  $_[KERNEL]->post( $_[HEAP]->{'pcap_session'} => 'run' );
}

sub pause {
  $_[KERNEL]->post( $_[HEAP]->{'pcap_session'} => 'pause' );
}

sub _dispatch {
  my( $kernel, $heap, $packets ) =
    @_[ KERNEL, HEAP, ARG0 ];

  if( exists $heap->{'target_session'} ) {
    $kernel->post( $heap->{'target_session'}, 
		   $heap->{'target_state'}, 
		   _process_packet( @{ $_ } ) ) foreach( @{$packets} );
  }
}

sub _signal {
  # print "Got signal ", $_[ARG0], "\n";

  $_[KERNEL]->post( pcap => 'shutdown' );

  return 1
}

sub shutdown {
  my ( $kernel, $heap, $session, $sender ) 
    = @_[ KERNEL, HEAP, SESSION, SENDER ];
  my $alias = $heap->{Alias};

#  print "In shutdown for sid ", $session->ID, ", alias $alias\n"; 
#  print "shutdown by ", $sender->ID, "\n";

  $kernel->post( $heap->{'pcap_session'} => 'shutdown' );

  $kernel->alias_remove( $alias );

#  print "Out shutdown for sid ", $session->ID, ", alias $alias\n"; 
}

sub _stop {
  my ( $kernel, $heap, $session ) = @_[ KERNEL, HEAP, SESSION ];
  my $alias = $heap->{Alias};

#  print "In state_stop for sid ", $session->ID, ", alias $alias\n"; 

#  print "Out state_stop for sid ", $session->ID, ", alias $alias\n"; 
}

sub _process_packet {
  my( $hdr, $pkt ) = @_;

  my $arp = 
    NetPacket::ARP->decode( NetPacket::Ethernet->decode($pkt)->{data} );

  ## Return hashref with apropriate fields
  return { 
	  type => $arp_opcodes{ $arp->{opcode} },
	  tv_sec => $hdr->{tv_sec},
	  tv_usec => $hdr->{tv_usec},
	  source_haddr => _phys( $arp->{sha} ), 
	  source_ipaddr => _ipaddr( $arp->{spa} ),
	  target_haddr => _phys( $arp->{tha} ),
	  target_ipaddr => _ipaddr( $arp->{tpa} ),
	 }
}

## Pretty printing subs for addresses
sub _ipaddr { join( ".", unpack( "C4", pack( "N", oct( "0x". shift ) ) ) ) }
sub _phys { join( ":", grep {length} split( /(..)/, shift ) ) }

# vim:ts=4:shiftwidth:expandtab
1;

__END__
