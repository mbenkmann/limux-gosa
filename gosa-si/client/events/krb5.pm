package krb5;


use strict;
use warnings;

use Authen::Krb5;
use Authen::Krb5::Admin qw(:constants);
use GOsaSI::GosaSupportDaemon;

use Exporter;

our @ISA = qw(Exporter);

my @events = (
    "get_events",
    "krb5_list_principals",  
    "krb5_list_policies",
    "krb5_get_principal",
    "krb5_create_principal",
    "krb5_modify_principal",
    "krb5_del_principal",
    "krb5_get_policy",
    "krb5_create_policy",
    "krb5_modify_policy",
    "krb5_del_policy",
    "krb5_set_password",
    );
    
our @EXPORT = @events;

BEGIN {}

END {}

### Start ######################################################################

Authen::Krb5::init_context;
Authen::Krb5::init_ets;

my $krb_admin;
my $krb_password;

my %cfg_defaults = (
"krb5" => {
   "admin" => [\$krb_admin, ""],
   "password" => [\$krb_password, ""],
   },
);
# why not using the main::read_configfile, the code seems exactly the same
&krb5_read_configfile($main::cfg_file, %cfg_defaults);


sub krb5_read_configfile {
    my ($cfg_file, %cfg_defaults) = @_;
    my $cfg;

    if( defined( $cfg_file) && ( (-s $cfg_file) > 0 )) {
        if( -r $cfg_file ) {
            $cfg = Config::IniFiles->new( -file => $cfg_file );
        } else {
            &main::daemon_log("ERROR: krb5.pm couldn't read config file!", 1);
        }
    } else {
        $cfg = Config::IniFiles->new() ;
    }
    foreach my $section (keys %cfg_defaults) {
        foreach my $param (keys %{$cfg_defaults{ $section }}) {
            my $pinfo = $cfg_defaults{ $section }{ $param };
            ${@$pinfo[0]} = $cfg->val( $section, $param, @$pinfo[1] );
        }
    }
}


sub get_events { return \@events; }


sub krb5_list_principals {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];

    # build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    # Authenticate
    my $kadm5 = Authen::Krb5::Admin->init_with_password($krb_admin, $krb_password);
    if (not defined $kadm5){
      &add_content2xml_hash($out_hash, "error", "Cannot connect to kadmin server");
    } else {
      my @principals= $kadm5->get_principals() or &add_content2xml_hash($out_hash, "error", Authen::Krb5::Admin::error);
      for my $principal (@principals) {
        &add_content2xml_hash($out_hash, "principal", $principal);
      }
    }

    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # return message
    return &create_xml_string($out_hash);
}


sub krb5_create_principal {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];

    # build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    # Sanity check
    if (not defined @{$msg_hash->{'principal'}}[0]){
      &add_content2xml_hash($out_hash, "error", "No principal specified");
      return &create_xml_string($out_hash);
    }

    # Authenticate
    my $kadm5 = Authen::Krb5::Admin->init_with_password($krb_admin, $krb_password);
    my $principal;
    if (not defined $kadm5){
      &add_content2xml_hash($out_hash, "error", "Cannot connect to kadmin server");
    } else {
      $principal= Authen::Krb5::parse_name(@{$msg_hash->{'principal'}}[0]);
      if(not defined $principal) {
        &add_content2xml_hash($out_hash, "error", "Illegal principal name");
      } else {
        if ( $kadm5->get_principal($principal)){
          &add_content2xml_hash($out_hash, "error", "Principal exists");
          if (defined $forward_to_gosa) {
              &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
          }
          return &create_xml_string($out_hash);
        }

        my $princ= Authen::Krb5::Admin::Principal->new;
        foreach ('mask', 'attributes', 'aux_attributes', 'max_life', 'max_renewable_life', 
                 'policy', 'princ_expire_time', 'pw_expiration'){

          if (defined @{$msg_hash->{$_}}[0]){
            $princ->$_(@{$msg_hash->{$_}}[0]);
          }
        }

        $princ->principal($principal);
        $kadm5->create_principal($princ, join '', map { chr rand(255) + 1 } 1..256) or &add_content2xml_hash($out_hash, "error", Authen::Krb5::Admin::error);

        # Directly randomize key
        $kadm5->randkey_principal($principal);
      }
    }

    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # return message
    return &create_xml_string($out_hash);
}


sub krb5_modify_principal {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];

    # build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    # Sanity check
    if (not defined @{$msg_hash->{'principal'}}[0]){
        &add_content2xml_hash($out_hash, "error", "No principal specified");
        if (defined $forward_to_gosa) {
            &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
        }
        return &create_xml_string($out_hash);
    }

    # Authenticate
    my $kadm5 = Authen::Krb5::Admin->init_with_password($krb_admin, $krb_password);
    my $principal;
    if (not defined $kadm5){
      &add_content2xml_hash($out_hash, "error", "Cannot connect to kadmin server");
    } else {
      $principal= Authen::Krb5::parse_name(@{$msg_hash->{'principal'}}[0]);
      if(not defined $principal) {
        &add_content2xml_hash($out_hash, "error", "Illegal principal name");
      } else {
        if (not $kadm5->get_principal($principal)){
          &add_content2xml_hash($out_hash, "error", "Principal does not exists");
          if (defined $forward_to_gosa) {
              &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
          }
          return &create_xml_string($out_hash);
        }

        my $princ= Authen::Krb5::Admin::Principal->new;
        foreach ('mask', 'attributes', 'aux_attributes', 'max_life', 'max_renewable_life', 
                 'policy', 'princ_expire_time', 'pw_expiration'){

          if (defined @{$msg_hash->{$_}}[0]){
            $princ->$_(@{$msg_hash->{$_}}[0]);
          }
        }

        $princ->principal($principal);
        $kadm5->modify_principal($princ) or &add_content2xml_hash($out_hash, "error", Authen::Krb5::Admin::error);
      }
    }

    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # return message
    return &create_xml_string($out_hash);
}


sub krb5_get_principal {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];

    # build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    # Sanity check
    if (not defined @{$msg_hash->{'principal'}}[0]){
      &add_content2xml_hash($out_hash, "error", "No principal specified");
      if (defined $forward_to_gosa) {
          &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
      }
      return &create_xml_string($out_hash);
    }

    # Authenticate
    my $kadm5 = Authen::Krb5::Admin->init_with_password($krb_admin, $krb_password);
    my $principal;
    if (not defined $kadm5){
      &add_content2xml_hash($out_hash, "error", "Cannot connect to kadmin server");
    } else {
      $principal= Authen::Krb5::parse_name(@{$msg_hash->{'principal'}}[0]);
      if(not defined $principal) {
        &add_content2xml_hash($out_hash, "error", "Illegal principal name");
      } else {
        my $data= $kadm5->get_principal($principal) or &add_content2xml_hash($out_hash, "error", Authen::Krb5::Admin::error);
        &add_content2xml_hash($out_hash, "principal", @{$msg_hash->{'principal'}}[0]);
        &add_content2xml_hash($out_hash, "mask", $data->mask);
        &add_content2xml_hash($out_hash, "attributes", $data->attributes);
        &add_content2xml_hash($out_hash, "kvno", $data->kvno);
        &add_content2xml_hash($out_hash, "max_life", $data->max_life);
        &add_content2xml_hash($out_hash, "max_renewable_life", $data->max_renewable_life);
        &add_content2xml_hash($out_hash, "aux_attributes", $data->aux_attributes);
        &add_content2xml_hash($out_hash, "policy", $data->policy);
        &add_content2xml_hash($out_hash, "fail_auth_count", $data->fail_auth_count);
        &add_content2xml_hash($out_hash, "last_failed", $data->last_failed);
        &add_content2xml_hash($out_hash, "last_pwd_change", $data->last_pwd_change);
        &add_content2xml_hash($out_hash, "last_success", $data->last_success);
        &add_content2xml_hash($out_hash, "mod_date", $data->mod_date);
        &add_content2xml_hash($out_hash, "princ_expire_time", $data->princ_expire_time);
        &add_content2xml_hash($out_hash, "pw_expiration", $data->pw_expiration);
      }
    }

    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # return message
    return &create_xml_string($out_hash);
}


sub krb5_del_principal {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];

    # build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    # Sanity check
    if (not defined @{$msg_hash->{'principal'}}[0]){
        if (defined $forward_to_gosa) {
            &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
        }
        &add_content2xml_hash($out_hash, "error", "No principal specified");
        return &create_xml_string($out_hash);
    }

    # Authenticate
    my $kadm5 = Authen::Krb5::Admin->init_with_password($krb_admin, $krb_password);
    my $principal;
    if (not defined $kadm5){
      &add_content2xml_hash($out_hash, "error", "Cannot connect to kadmin server");
    } else {
      $principal= Authen::Krb5::parse_name(@{$msg_hash->{'principal'}}[0]);
      if(not defined $principal) {
        &add_content2xml_hash($out_hash, "error", "Illegal principal name");
      } else {
        $kadm5->delete_principal($principal) or &add_content2xml_hash($out_hash, "error", Authen::Krb5::Admin::error);
      }
    }

    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # return message
    return &create_xml_string($out_hash);
}


sub krb5_list_policies {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];

    # build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    # Authenticate
    my $kadm5 = Authen::Krb5::Admin->init_with_password($krb_admin, $krb_password);
    if (not defined $kadm5){
      &add_content2xml_hash($out_hash, "error", "Cannot connect to kadmin server");
    } else {
      my @policies= $kadm5->get_policies(); # or &add_content2xml_hash($out_hash, "error", Authen::Krb5::Admin::error);
      for my $policy (@policies) {
        &add_content2xml_hash($out_hash, "policy", $policy);
      }
    }

    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];
    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # return message
    return &create_xml_string($out_hash);
}


sub krb5_get_policy {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];

    # build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    # Sanity check
    if (not defined @{$msg_hash->{'policy'}}[0]){
        &add_content2xml_hash($out_hash, "error", "No policy specified");
        if (defined $forward_to_gosa) {
            &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
        }
        return &create_xml_string($out_hash);
    }

    # Authenticate
    my $kadm5 = Authen::Krb5::Admin->init_with_password($krb_admin, $krb_password);
    my $principal;
    if (not defined $kadm5){
      &add_content2xml_hash($out_hash, "error", "Cannot connect to kadmin server");
    } else {
      my $data= $kadm5->get_policy(@{$msg_hash->{'policy'}}[0]) or &add_content2xml_hash($out_hash, "error", Authen::Krb5::Admin::error);
      &add_content2xml_hash($out_hash, "name", $data->name);
      &add_content2xml_hash($out_hash, "mask", $data->mask);
      &add_content2xml_hash($out_hash, "pw_history_num", $data->pw_history_num);
      &add_content2xml_hash($out_hash, "pw_max_life", $data->pw_max_life);
      &add_content2xml_hash($out_hash, "pw_min_classes", $data->pw_min_classes);
      &add_content2xml_hash($out_hash, "pw_min_length", $data->pw_min_length);
      &add_content2xml_hash($out_hash, "pw_min_life", $data->pw_min_life);
      &add_content2xml_hash($out_hash, "policy_refcnt", $data->policy_refcnt);
    }

    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # return message
    return &create_xml_string($out_hash);
}


sub krb5_create_policy {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];

    # Build return message
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    # Sanity check
    if (not defined @{$msg_hash->{'policy'}}[0]){
        &add_content2xml_hash($out_hash, "error", "No policy specified");
        if (defined $forward_to_gosa) {
            &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
        }

        return &create_xml_string($out_hash);
    }
    &add_content2xml_hash($msg_hash, "name", @{$msg_hash->{'policy'}}[0]);

    # Authenticate
    my $kadm5 = Authen::Krb5::Admin->init_with_password($krb_admin, $krb_password);
    if (not defined $kadm5){
        &add_content2xml_hash($out_hash, "error", "Cannot connect to kadmin server");
    } else {
        if ( $kadm5->get_policy(@{$msg_hash->{'policy'}}[0])) {
            &add_content2xml_hash($out_hash, "error", "Policy exists");
            if (defined $forward_to_gosa) {
                &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
            }
            return &create_xml_string($out_hash);
        }

      my $pol = Authen::Krb5::Admin::Policy->new;

      # Move information from xml message to modifyer
      foreach ('name', 'mask', 'pw_history_num', 'pw_max_life', 'pw_min_classes',
                'pw_min_length', 'pw_min_life'){

        if (defined @{$msg_hash->{$_}}[0]){
          $pol->$_(@{$msg_hash->{$_}}[0]);
        }
      }

      # Create info
      $kadm5->create_policy($pol) or &add_content2xml_hash($out_hash, "error", Authen::Krb5::Admin::error);
    }

    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # build return message with twisted target and source
    my $out_msg = &create_xml_string($out_hash);

    # return message
    return $out_msg;
}


sub krb5_modify_policy {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];

    # Build return message
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    # Sanity check
    if (not defined @{$msg_hash->{'policy'}}[0]){
        if (defined $forward_to_gosa) {
            &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
        }
        &add_content2xml_hash($out_hash, "error", "No policy specified");
        return &create_xml_string($out_hash);
    }
    &add_content2xml_hash($msg_hash, "name", @{$msg_hash->{'policy'}}[0]);

    # Authenticate
    my $kadm5 = Authen::Krb5::Admin->init_with_password($krb_admin, $krb_password);
    if (not defined $kadm5){
      &add_content2xml_hash($out_hash, "error", "Cannot connect to kadmin server");
    } else {
      my $pol = Authen::Krb5::Admin::Policy->new;

      # Move information from xml message to modifyer
      foreach ('name', 'mask', 'pw_history_num', 'pw_max_life', 'pw_min_classes',
                'pw_min_length', 'pw_min_life'){

        if (defined @{$msg_hash->{$_}}[0]){
          $pol->$_(@{$msg_hash->{$_}}[0]);
        }
      }

      # Create info
      $kadm5->modify_policy($pol) or &add_content2xml_hash($out_hash, "error", Authen::Krb5::Admin::error);
    }

    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # build return message with twisted target and source
    my $out_msg = &create_xml_string($out_hash);

    # return message
    return $out_msg;
}


sub krb5_del_policy {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];

    # build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    # Sanity check
    if (not defined @{$msg_hash->{'policy'}}[0]){
        if (defined $forward_to_gosa) {
            &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
        }
        &add_content2xml_hash($out_hash, "error", "No policy specified");
        return &create_xml_string($out_hash);
    }

    # Authenticate
    my $kadm5 = Authen::Krb5::Admin->init_with_password($krb_admin, $krb_password);
    my $policy= @{$msg_hash->{'policy'}}[0];
    if (not defined $kadm5){
      &add_content2xml_hash($out_hash, "error", "Cannot connect to kadmin server");
    } else {
      $kadm5->delete_policy($policy) or &add_content2xml_hash($out_hash, "error", Authen::Krb5::Admin::error);
    }

    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # return message
    return &create_xml_string($out_hash);
}

sub krb5_set_password {
    my ($msg, $msg_hash) = @_;
    my $header = @{$msg_hash->{'header'}}[0];
    my $source = @{$msg_hash->{'source'}}[0];
    my $target = @{$msg_hash->{'target'}}[0];
    my $session_id = @{$msg_hash->{'session_id'}}[0];
    my $forward_to_gosa = @{$msg_hash->{'forward_to_gosa'}}[0];

    # build return message with twisted target and source
    my $out_hash = &main::create_xml_hash("answer_$header", $target, $source);
    &add_content2xml_hash($out_hash, "session_id", $session_id);

    # Sanity check
    if (not defined @{$msg_hash->{'principal'}}[0]){
        if (defined $forward_to_gosa) {
            &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
        }

        &add_content2xml_hash($out_hash, "error", "No principal specified");
        return &create_xml_string($out_hash);
    }
    if (not defined @{$msg_hash->{'password'}}[0]){
        if (defined $forward_to_gosa) {
            &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
        }

        &add_content2xml_hash($out_hash, "error", "No password specified");
        return &create_xml_string($out_hash);
    }

    # Authenticate
    my $kadm5 = Authen::Krb5::Admin->init_with_password($krb_admin, $krb_password);
    my $principal;
    if (not defined $kadm5){
      &add_content2xml_hash($out_hash, "error", "Cannot connect to kadmin server");
    } 

    $principal= Authen::Krb5::parse_name(@{$msg_hash->{'principal'}}[0]);
    if(not defined $principal) {
      &add_content2xml_hash($out_hash, "error", "Illegal principal name");
    } else {
      $kadm5->chpass_principal($principal, @{$msg_hash->{'password'}}[0]) or &add_content2xml_hash($out_hash, "error", Authen::Krb5::Admin::error);
    }

    if (defined $forward_to_gosa) {
        &add_content2xml_hash($out_hash, "forward_to_gosa", $forward_to_gosa);
    }

    # return message
    return &create_xml_string($out_hash);
}
1;
