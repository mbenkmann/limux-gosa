/*
Copyright (c) 2012 Matthias S. Benkmann

This program is free software; you can redistribute it and/or
modify it under the terms of the GNU General Public License
as published by the Free Software Foundation; either version 2
of the License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.

You should have received a copy of the GNU General Public License
along with this program; if not, write to the Free Software
Foundation, Inc., 51 Franklin Street, Fifth Floor, Boston, 
MA  02110-1301, USA.
*/

// The handling code for the many messages passed around between 
// GOsa, gosa-si and go-susi.
package message

import ( 
         "sync"
         "time"
         "strings"
         
         "../db"
         "../xml"
         "github.com/mbenkmann/golib/util"
         "github.com/mbenkmann/golib/bytes"
         "../config"
         "../security"
       )

// Returns an XML string to return to GOsa if there was an error processing
// a message from GOsa. msg is an error message (will be formatted by Sprintf())
func ErrorReplyXML(msg interface{}) *xml.Hash {
  // Use an XML hash so that msg will be properly escaped if it contains e.g. "<"
  x := xml.NewHash("xml","header","answer")
  x.Add("error_string",msg)
  x.Add("source", config.ServerSourceAddress)
  x.Add("target", "GOSA")
  x.Add("answer1","1")
  return x
}

// Returns ErrorReplyXML(msg).String()
func ErrorReply(msg interface{}) string {
  return ErrorReplyXML(msg).String()
}

// Returns ErrorReply(msg) within a bytes.Buffer THAT NEEDS TO BE FREED BY THE CALLER!!
func ErrorReplyBuffer(msg interface{}) *bytes.Buffer {
  var b bytes.Buffer
  ErrorReplyXML(msg).WriteTo(&b)
  return &b
}

func handleServerMessage(bit bool, name string) bool {
  if !config.RunServer {
    util.Log(0, "WARNING! Ignoring server message because operating in client-only mode")
    return false
  } else {
    if !bit {
      util.Log(0, "WARNING! [SECURITY] Ignoring message because access control bit \"%s\" is not set in certificate", name)
      return false
    }
  }
  return true
}

// Takes a possibly encrypted message in buf and processes it, returning a reply.
// context is the security context.
// Returns: 
//  buffer containing the reply to return (MUST BE FREED BY CALLER VIA Reset()!)
//  disconnect == true if connection should be terminated due to error
//
// NOTE: buf IS NOT FREED BY THIS FUNCTION BUT ITS CONTENTS ARE CHANGED!
func ProcessEncryptedMessage(buf *bytes.Buffer, context *security.Context) (reply *bytes.Buffer, disconnect bool) {
  if buf.Len() > 4096 {
    util.Log(2, "DEBUG! Processing LONG message: (truncated)%v\n.\n.\n.\n%v", string(buf.Bytes()[0:2048]), string(buf.Bytes()[buf.Len()-2048:]))
  } else {
    util.Log(2, "DEBUG! Processing message: %v", buf.String())
  }

  for attempt := 0 ; attempt < 4; attempt++ {
    if attempt != 0 && config.TLSRequired {
      util.Log(1, "INFO! [SECURITY] TLS-only mode => Decryption with old protocol will not be attempted")
      //NOTE: This prevents the last ditch attempt to decrypt with all known
      //      server and client keys. This attempt might still have produced a
      //      result in case the connecting party is pre-TLS and we happen to
      //      have its key in our database (from a time before our server was
      //      configured to be TLS-only). However if the admin configured our
      //      server to be TLS-only (by not putting any keys
      //      in the config) we assume that he does not want pre-TLS
      //      parties to connect.
      break
    }
    var keys_to_try []string
    
    switch attempt {
      case 0: keys_to_try = config.ModuleKeys
      case 1: host := context.PeerID.IP.String()
              {
                keys_to_try = append(db.ServerKeys(host), db.ClientKeys(host)...)
                if host == "127.0.0.1" { // make sure we find the key even if registered under our external IP address
                  keys_to_try = append(db.ServerKeys(config.IP), db.ClientKeys(config.IP)...)
                }
              }
      case 2: util.Log(1, "INFO! Last resort attempt to decrypt message from %v with all server keys", context.PeerID.IP)
              keys_to_try = db.ServerKeysForAllServers()
      case 3: util.Log(1, "INFO! Last resort attempt to decrypt message from %v with all client keys", context.PeerID.IP)
              keys_to_try = db.ClientKeysForAllClients()
    }
    
    for _, key := range keys_to_try {
      if security.GosaDecryptBuffer(buf, key) {
        if buf.Len() > 4096 {
          util.Log(2, "DEBUG! Decrypted LONG message from %v with key %v: (truncated)%v\n.\n.\n.\n%v", context.PeerID.IP, key, string(buf.Bytes()[0:2048]), string(buf.Bytes()[buf.Len()-2048:]))
        } else {
          util.Log(2, "DEBUG! Decrypted message from %v with key %v: %v", context.PeerID.IP, key, buf.String())
        }
        
        // special case for CLMSG_save_fai_log because this kind of message
        // is so large and parsing it to XML doesn't really gain us anything.
        if buf.Contains("<CLMSG_save_fai_log>") {
          if handleServerMessage(true,"") {
            clmsg_save_fai_log(buf, context)
          }
          return &bytes.Buffer{}, false
        }
        
        xml, err := xml.StringToHash(buf.String())
        if err != nil {
          util.Log(0,"ERROR! %v", err)
          return ErrorReplyBuffer(err), true
        } 
        
        // At this point we have successfully decrypted and parsed the message
        return ProcessXMLMessage(xml, context, key)
      }
    }
  }
  
  // This part is only reached if none of the keys opened the message
  util.Log(0, "ERROR! Could not decrypt message from %v", context.PeerID.IP)
  
  // Maybe we got out of sync with the sender's encryption key 
  // (e.g. by missing a new_key message). Try to re-establish communcation.
  ip := context.PeerID.IP.To4()
  if ip == nil {
    util.Log(0, "ERROR! Cannot convert sender address to IPv4 address: %v", context.PeerID.IP)
  } else {
    go tryToReestablishCommunicationWith(ip.String())
  }
  
  return ErrorReplyBuffer("Could not decrypt message"), true
}

var mapIP2ReestablishDelay = map[string]time.Duration{}
var mapIP2ReestablishDelay_mutex sync.Mutex

// Tries to re-establish communication with a client/server at the given IP,
// by 
//   1) sending here_i_am to the server where we are registered. We do this
//      even if config.RunServer (i.e. we are registered at ourselves) because
//      this will trigger new_foreign_client messages sent to peers so that other
//      servers that may believe they own us correct their data.
//   2) sending (if config.RunServer) new_server messages to all known servers
//      we find for the IP in our servers database.
//   3) if config.RunServer and in 2) we did not find a server at that IP,
//      maybe it's a client that thinks we are its server. Send "deregistered" to
//      all ClientPorts in that case to cause re-registration.
func tryToReestablishCommunicationWith(ip string) {
  // Wait a little to limit the rate of spam wars between
  // 2 machines that can't re-establish communication (e.g. because of changed
  // keys in server.conf).
  mapIP2ReestablishDelay_mutex.Lock()
  var delay time.Duration
  var ok bool
  if delay, ok = mapIP2ReestablishDelay[ip]; !ok {
    delay = 1*time.Minute
  }
  mapIP2ReestablishDelay[ip] = 2*delay;
  mapIP2ReestablishDelay_mutex.Unlock()
  
  // if the delay exceeds 24h this means that we got multiple
  // reestablish requests while we're still waiting to begin one
  // in that case, bail out.
  if delay > 24*time.Hour { return }

  util.Log(0, "WARNING! Will try to re-establish communication with %v after waiting %v", ip, delay)
  time.Sleep(delay)
  
  // if we actually completed a 10h wait, reset the timer to 1 minute
  if delay >= 10*time.Hour {
    mapIP2ReestablishDelay_mutex.Lock()
    mapIP2ReestablishDelay[ip] = 1*time.Minute;
    mapIP2ReestablishDelay_mutex.Unlock()
  }
  
  util.Log(0, "WARNING! Will try to re-establish communication with %v", ip)
  ConfirmRegistration() // 1)
  
  ip, err := util.Resolve(ip, config.IP)
  if err != nil {
    util.Log(0, "ERROR! Resolve(): %v", err)
  }
  
  if config.RunServer { // 2)
    sendmuell := true
    for _, server := range db.ServerAddresses() {
      if strings.HasPrefix(server, ip) {
        sendmuell = false
        srv := server
        go util.WithPanicHandler(func(){ Send_new_server("new_server", srv) })
      }
    }
    
    if sendmuell {
      for _, port := range config.ClientPorts {
        addr := ip + ":" + port
        if addr != config.ServerSourceAddress { // never send "deregistered" to our own server
          dereg :=  "<xml><header>deregistered</header><source>"+config.ServerSourceAddress+"</source><target>"+addr+"</target></xml>"
          go security.SendLnTo(addr, dereg, "", false)
        }
      }
    }
  }
}

// Arguments
//   xml: the message
//   context: the security context
//   key: the key that successfully decrypted the message
// Returns:
//   reply: buffer containing the reply to return
//   disconnect: true if connection should be terminated due to error
func ProcessXMLMessage(xml *xml.Hash, context *security.Context, key string) (reply *bytes.Buffer, disconnect bool) {
  if !context.TLS && key == "dummy-key" && xml.Text("header") != "gosa_ping" {
    util.Log(0, "ERROR! Rejecting non-ping message encrypted with dummy-key or not at all")
    return ErrorReplyBuffer("ERROR! Rejecting non-ping message encrypted with dummy-key or not at all"),true
  }
  reply = &bytes.Buffer{}
  disconnect = false
  
  is_client_message := true
  switch xml.Text("header") {
    case "new_ldap_config":     new_ldap_config(xml)
    case "new_ntp_config":      new_foo_config(xml)
    case "registered":          registered(xml)
    case "deregistered":        deregistered(xml)
    case "usr_msg":             usr_msg(xml)
    case "set_activated_for_installation": set_activated_for_installation(xml)
    case "detect_hardware":     detect_hardware(xml)
    case "sistats":             if !context.Access.Query.QueryAll {
                                  util.Log(0, "WARNING! [SECURITY] Ignoring \"sistats\" query because access control bit \"queryAll\" is not set in certificate")
                                } else {
                                  sistats().WriteTo(reply)
                                }
    case "panic":               if !context.Access.Misc.Debug {
                                  util.Log(0, "WARNING! [SECURITY] Ignoring \"panic\" command because access control bit \"debug\" is not set in certificate")
                                } else {
                                  go func(){panic("Panic by user request")}()
                                }
    case "trigger_action_halt",      // "Anhalten"
         "trigger_action_localboot", // "Erzwinge lokalen Start"
         "trigger_action_reboot",    // "Neustarten"
         "trigger_action_faireboot", // "Job abbrechen"
         "trigger_action_update",    // "Aktualisieren"
         "trigger_action_reinstall", // "Neuinstallation"
         "trigger_action_audit",     // "Auditieren"
         "trigger_action_instant_update": trigger_action_foo(xml)
  default:                      
    is_client_message = false
  }
  
  is_server_message := !is_client_message
  if is_server_message {
    switch xml.Text("header") {
      case "gosa_ping":                if handleServerMessage(context.Access.Query.QueryAll, "queryAll") {
                                         reply.WriteString(gosa_ping(xml))
                                         disconnect = true
                                       }
      case "gosa_query_jobdb":         if handleServerMessage(context.Access.Query.QueryJobs||context.Access.Query.QueryAll,"queryJobs") { gosa_query_jobdb(xml, context).WriteTo(reply) }
      case "gosa_query_fai_server":    if handleServerMessage(context.Access.Query.QueryAll,"queryAll") { gosa_query_fai_server(xml, context).WriteTo(reply) }
      case "gosa_query_fai_release":   if handleServerMessage(context.Access.Query.QueryAll,"queryAll") { gosa_query_fai_release(xml, context).WriteTo(reply) }
      case "gosa_query_packages_list": if handleServerMessage(context.Access.Query.QueryAll,"queryAll") { 
                                         // result can be very large, so make sure
                                         // memory is free'd immediately instead of
                                         // waiting for GC
                                         pkg := gosa_query_packages_list(xml, context)
                                         pkg.WriteTo(reply)
                                         pkg.Destroy()
                                       }
      case "gosa_query_audit":         if handleServerMessage(context.Access.Query.QueryAll,"queryAll") { 
                                         // result can be very large, so make sure
                                         // memory is free'd immediately instead of
                                         // waiting for GC
                                         audit := gosa_query_audit(xml, context)
                                         audit.WriteTo(reply)
                                         audit.Destroy()
                                       }
      case "gosa_query_audit_aggregate":if handleServerMessage(context.Access.Query.QueryAll,"queryAll") { 
                                         // result can be very large, so make sure
                                         // memory is free'd immediately instead of
                                         // waiting for GC
                                         audit := gosa_query_audit_aggregate(xml, context)
                                         audit.WriteTo(reply)
                                         audit.Destroy()
                                       }
      case "gosa_show_log_by_mac":     if handleServerMessage(context.Access.Query.QueryAll,"queryAll") { gosa_show_log_by_mac(xml).WriteTo(reply) }
      case "gosa_show_log_files_by_date_and_mac": 
                                       if handleServerMessage(context.Access.Query.QueryAll,"queryAll") { 
                                         gosa_show_log_files_by_date_and_mac(xml).WriteTo(reply)
                                       }
      case "gosa_get_log_file_by_date_and_mac":   
                                       if handleServerMessage(context.Access.Query.QueryAll,"queryAll") { 
                                         gosa_get_log_file_by_date_and_mac(xml).WriteTo(reply)
                                       }
      case "gosa_get_available_kernel":   
                                       if handleServerMessage(context.Access.Query.QueryAll,"queryAll") {
                                         gosa_get_available_kernel(xml,context).WriteTo(reply)
                                       }
      case "new_server":          if handleServerMessage(context.Access.Misc.Peer,"peer") { new_server(xml) }
      case "confirm_new_server":  if handleServerMessage(context.Access.Misc.Peer,"peer") { confirm_new_server(xml) }
      case "foreign_job_updates": if handleServerMessage(context.Access.Misc.Peer,"peer") { foreign_job_updates(xml) }
      case "new_foreign_client":  if handleServerMessage(true,"") { new_foreign_client(xml) }
      case "information_sharing": if handleServerMessage(true,"") { information_sharing(xml) }
      case "here_i_am":           if handleServerMessage(true,"") { here_i_am(xml) }
      case "new_key":             if handleServerMessage(true,"") { new_key(xml) }
      case "detected_hardware":   if handleServerMessage(true,"") { detected_hardware(xml) }
      case "CLMSG_CURRENTLY_LOGGED_IN": if handleServerMessage(true,"") { clmsg_currently_logged_in(xml) }
      case "CLMSG_LOGIN":         if handleServerMessage(true,"") { clmsg_login(xml) }
      case "CLMSG_LOGOUT":        if handleServerMessage(true,"") {clmsg_logout(xml) }
      case "CLMSG_PROGRESS":      if handleServerMessage(true,"") {clmsg_progress(xml) }
      case "CLMSG_TASKDIE":       if handleServerMessage(true,"") {clmsg_taskdie(xml) }
      case "CLMSG_GOTOACTIVATION":if handleServerMessage(true,"") {clmsg_gotoactivation(xml) }
      case "CLMSG_HOOK",
           "CLMSG_TASKBEGIN",
           "CLMSG_check",
           "CLMSG_TASKEND":
                                  util.Log(2, "DEBUG! ProcessXMLMessage: '%v'\n=======start FAI message=======\n%v\n=======end FAI message=======", xml.Text("header"), xml.String())
      case "job_set_activated_for_installation",
           "gosa_set_activated_for_installation":
                                  if handleServerMessage(context.Access.Jobs.Unlock||context.Access.Jobs.JobsAll,"unlock") { gosa_set_activated_for_installation(xml) }
      case "gosa_trigger_action_lock":      // "Sperre"
                                  if handleServerMessage(context.Access.Jobs.Lock||context.Access.Jobs.JobsAll,"lock") {
                                    gosa_trigger_action(xml).WriteTo(reply)
                                  }
      case "gosa_trigger_action_reboot",    // "Neustarten"
           "gosa_trigger_action_halt":      // "Anhalten"
                                  if handleServerMessage(context.Access.Jobs.Shutdown||context.Access.Jobs.JobsAll,"shutdown") {
                                    gosa_trigger_action(xml).WriteTo(reply)
                                  }
      case "gosa_trigger_action_localboot", // "Erzwinge lokalen Start"
           "gosa_trigger_action_faireboot": // "Job abbrechen"
                                  if handleServerMessage(context.Access.Jobs.Abort||context.Access.Jobs.JobsAll,"abort") {
                                    gosa_trigger_action(xml).WriteTo(reply)
                                  }
      case "gosa_trigger_action_activate":  // "Sperre aufheben"
                                  if handleServerMessage(context.Access.Jobs.Unlock||context.Access.Jobs.JobsAll,"unlock") {
                                    gosa_trigger_action(xml).WriteTo(reply)
                                  }
      case "gosa_trigger_action_update":    // "Aktualisieren"
                                  if handleServerMessage(context.Access.Jobs.Update||context.Access.Jobs.JobsAll,"update") {
                                    gosa_trigger_action(xml).WriteTo(reply)
                                  }
      case "gosa_trigger_action_reinstall": // "Neuinstallation"
                                  if handleServerMessage(context.Access.Jobs.Install||context.Access.Jobs.JobsAll,"install") {
                                    gosa_trigger_action(xml).WriteTo(reply)
                                  }
      case "gosa_trigger_action_wake":      // "Aufwecken"
                                  if handleServerMessage(context.Access.Jobs.Wake||context.Access.Jobs.JobsAll,"wake") {
                                    gosa_trigger_action(xml).WriteTo(reply)
                                  }
      case "gosa_trigger_action_audit":      // "Auditieren"
                                  if handleServerMessage(context.Access.Jobs.Audit||context.Access.Jobs.JobsAll,"audit") {
                                    gosa_trigger_action(xml).WriteTo(reply)
                                  }
      case "job_trigger_action_lock":      // "Sperre"
                                  if handleServerMessage(context.Access.Jobs.Lock||context.Access.Jobs.JobsAll,"lock") {
                                    job_trigger_action(xml).WriteTo(reply)
                                  }
      case "job_trigger_action_halt",      // "Anhalten"
           "job_trigger_action_reboot":    // "Neustarten"
                                  if handleServerMessage(context.Access.Jobs.Shutdown||context.Access.Jobs.JobsAll,"shutdown") {
                                    job_trigger_action(xml).WriteTo(reply)
                                  }
      case "job_trigger_action_localboot", // "Erzwinge lokalen Start"
           "job_trigger_action_faireboot": // "Job abbrechen"
                                  if handleServerMessage(context.Access.Jobs.Abort||context.Access.Jobs.JobsAll,"abort") {
                                    job_trigger_action(xml).WriteTo(reply)
                                  }
      case "job_trigger_action_activate":  // "Sperre aufheben"
                                  if handleServerMessage(context.Access.Jobs.Unlock||context.Access.Jobs.JobsAll,"unlock") {
                                    job_trigger_action(xml).WriteTo(reply)
                                  }
      case "job_trigger_action_update":    // "Aktualisieren"
                                  if handleServerMessage(context.Access.Jobs.Update||context.Access.Jobs.JobsAll,"update") {
                                    job_trigger_action(xml).WriteTo(reply)
                                  }
      case "job_trigger_action_reinstall": // "Neuinstallation"
                                  if handleServerMessage(context.Access.Jobs.Install||context.Access.Jobs.JobsAll,"install") {
                                    job_trigger_action(xml).WriteTo(reply)
                                  }
      case "job_trigger_action_wake":      // "Aufwecken"
                                  if handleServerMessage(context.Access.Jobs.Wake||context.Access.Jobs.JobsAll,"wake") {
                                    job_trigger_action(xml).WriteTo(reply)
                                  }
      case "job_trigger_action_audit":      // "Auditieren"
                                  if handleServerMessage(context.Access.Jobs.Audit||context.Access.Jobs.JobsAll,"audit") {
                                    job_trigger_action(xml).WriteTo(reply)
                                  }
      case "gosa_trigger_activate_new",
           "job_trigger_activate_new":
                                  if handleServerMessage(context.Access.Jobs.NewSys||context.Access.Jobs.JobsAll,"newSys") {
                                    job_trigger_activate_new(xml).WriteTo(reply)
                                  }
      case "gosa_send_user_msg",
           "job_send_user_msg":   if handleServerMessage(context.Access.Jobs.UserMsg||context.Access.Jobs.JobsAll,"userMsg") { job_send_user_msg(xml).WriteTo(reply) }
      case "trigger_wake":        if handleServerMessage(context.Access.Misc.Wake,"wake") {
                                    trigger_wake(xml)
                                  }
      
      case "gosa_delete_jobdb_entry":
                                  if handleServerMessage(context.Access.Jobs.ModifyJobs,"modifyJobs") { gosa_delete_jobdb_entry(xml).WriteTo(reply) }
      case "gosa_update_status_jobdb_entry":
                                  if handleServerMessage(context.Access.Jobs.ModifyJobs,"modifyJobs") { gosa_update_status_jobdb_entry(xml).WriteTo(reply) }
    default:
          is_server_message = false
    }
  }
  
  if !is_server_message && !is_client_message {
    xml.SetText() // clean up excess top-level whitespace before logging
    util.Log(0, "ERROR! ProcessXMLMessage: Unknown message type '%v'\n=======start message=======\n%v\n=======end message=======", xml.Text("header"), xml.String())
    ErrorReplyXML("Unknown message type").WriteTo(reply)
  }
  
  disconnect = disconnect || reply.Contains("<error_string>")
  if key != "dummy-key" {
    security.GosaEncryptBuffer(reply, key)
  }
  return
}




