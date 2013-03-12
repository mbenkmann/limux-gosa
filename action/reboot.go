package action

import (
         "../db"
         "../xml"
         "../util"
         "../config"
         "../message"
       )

func Reboot(job *xml.Hash) bool {
  client := db.ClientWithMAC(job.Text("macaddress"))
  if client == nil {
    util.Log(0, "ERROR! Client with MAC %v unknown. Cannot execute job: %v", job.Text("macaddress"), job)
    return false
  } 
  
  client_addr := client.Text("client")
  util.Log(1, "INFO! Sending trigger_action_reboot to %v", client_addr)
  trigger_action_reboot := "<xml><header>trigger_action_reboot</header><trigger_action_reboot></trigger_action_reboot><source>"+config.ServerSourceAddress+"</source><target>"+client_addr+"</target></xml>"
  message.Client(client_addr).Tell(trigger_action_reboot, config.LocalClientMessageTTL)
  
  // In case the machine is sleeping, wake it up, because user
  // expects machine to be on after a reboot.
  // NOTE: Other jobs call Reboot() and expect the Wake().
  Wake(job)
  return true
}
