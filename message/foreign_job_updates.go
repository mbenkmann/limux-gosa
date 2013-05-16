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

package message

import (
         "regexp"
         "strings"
         
         "../db"
         "../xml"
         "../util"
         "../config"
       )

const re_1xx = "(1([0-9]?[0-9]?))"
const re_2xx = "(2([6-9]|([0-4][0-9]?)|(5[0-5]?))?)"
const re_xx  = "([3-9][0-9]?)"
const re_port = "(0|([1-6][0-9]{0,4})|([7-9][0-9]{0,3}))"
const ip_part = "(0|"+re_1xx+"|"+re_2xx+"|"+re_xx+")"
var addressRegexp = regexp.MustCompile("^"+ip_part+"([.]"+ip_part+"){3}:"+re_port+"$")

// Handles the message "foreign_job_updates".
//  xmlmsg: the decrypted and parsed message.
func foreign_job_updates(xmlmsg *xml.Hash) {
  source := xmlmsg.Text("source")
  sync   := xmlmsg.Text("sync")
  
  if !addressRegexp.MatchString(source) {
    // We could try name lookup here, but non-numeric <source> fields
    // don't occur in the wild. So we just bail out with a message.
    util.Log(0, "ERROR! <source>%v</source> is not in IP:PORT format", source)
    return
  }
  
  // If the message is a complete copy of the sender's jobdb,
  // clear out all old job data, before processing the message.
  if sync == "all" {
    db.JobsRemoveForeign(xml.FilterSimple("siserver",source))
  }
  
  // The key set is the set of peers for which to call
  // SyncIfNotGoSusi().
  syncIfNotGoSusi := make(map[string]bool)
  
  for child := xmlmsg.FirstChild(); child != nil; child = child.Next() {
  
    if !strings.HasPrefix(child.Element().Name(), "answer") { continue }
  
    answer := child.Element() 
    {
      job := answer.Clone()
      job.Rename("job")
      
      if job.Text("siserver") == "localhost" {
        job.First("siserver").SetText(source)
      }
      siserver := job.Text("siserver")
      
      xmlmess := job.First("xmlmessage")
      if xmlmess == nil {
        util.Log(0, "ERROR! <xmlmessage> missing from job descriptor")
        // go-susi doesn't need xmlmessage. We just add an empty one.
        // It would be nicer to generate a proper xmlmessage from the
        // job, but I'm too lazy to code this right now. This case doesn't
        // occur in the wild.
        job.Add("xmlmessage","") 
      } else 
      {
        // remove all whitespace from xmlmessage
        // This works around gosa-si's behaviour of introducing whitespace
        // which breaks base64 decoding.
        xmlmess.SetText(strings.Join(strings.Fields(xmlmess.Text()),""))
      }
      
      if !addressRegexp.MatchString(siserver) {
        // We could try name lookup here, but non-numeric <siserver> fields
        // don't occur in the wild. So we just bail out with a message.
        util.Log(0, "ERROR! <siserver>%v</siserver> is not in IP:PORT format", siserver)
        return
      }
      
      headertag := job.Text("headertag")
      macaddress := job.Text("macaddress")

/************************************************************************************
             Case 1: The updated job belongs to us
*************************************************************************************/
/*1*/ if siserver == config.ServerSourceAddress {
        var filter xml.HashFilter
        
/*1.1*/ if Peer(source).IsGoSusi() { // Message is from a go-susi, so <id> is meaningful.
          filter = xml.FilterSimple("id", job.Text("id"), "siserver", config.ServerSourceAddress)
/*1.2*/ } else {
          // The <id> field is the id of the job in the sending server's database
          // which is not meaningful to us. So the best we can do is select all
          // local jobs which match the headertag/macaddress combination.
          filter = xml.FilterSimple("siserver", config.ServerSourceAddress, "headertag",headertag,"macaddress",macaddress)
          
          // We also schedule a full sync to make sure the sender gets the actual change
          // as performed and not what it believes the change to be.
          // This won't fix any misinformation on other gosa-si servers that blindly
          // believe 3rd party information, but in the typical case of one gosa-si on
          // the production server and one go-susi on the test server this works perfectly.
          syncIfNotGoSusi[source] = true
        }
/*1.1 + 1.2*/
        
        // A foreign server can't know if a local job is done or not, so if
        // it sends us a "done" status for a local job it can only mean that
        // the job should be cancelled. Make sure that this works properly for
        // periodic jobs even if the sender doesn't support <periodic> 
        // (gosa-si versions older than 2.7)
        if job.Text("status") == "done" { 
          job.FirstOrAdd("periodic").SetText("none") 
        }
        db.JobsModifyLocal(filter, job)

/************************************************************************************
             Case 2: The updated job belongs to the sender
*************************************************************************************/        
/*2*/ } else if siserver == source {
        // If the job is in status "processing", cancel all local jobs for the same
        // MAC in status "processing", because only one job can be processing at
        // any time.
        // NOTE: I'm not sure if clearing <periodic> is the right thing to do
        // in this case. I've chosen to do it like this because I'm wary of
        // situations where 2 servers might end up having the same periodic job.
        // I think a lost periodic job is better than too many.
        if job.Text("status") == "processing" {
          filter := xml.FilterSimple("siserver", config.ServerSourceAddress, "macaddress", macaddress, "status", "processing")
          db.JobsRemoveLocal(filter, true)
        }
        
        // Because the job belongs to the sender, the <id> field corresponds to
        // the <original_id> we have in our database, so we can select the
        // job with precision.
        filter := xml.FilterSimple("original_id", job.Text("id"))
          
        db.JobsAddOrModifyForeign(filter, job)

/************************************************************************************
             Case 3: The updated job belongs to a 3rd party peer
*************************************************************************************/          
/*3*/ } else {
        // We don't trust Chinese whispers, so we don't use the job information
        // directly. Instead we schedule a query of the affected 3rd party's
        // jobdb. This needs to be done with a delay (part of SyncIfNotGoSusi()
        // in peer_connection.go), because the 3rd party
        // may not even have received the foreign_job_updates affecting its job.
        // We only do this if the 3rd party is not (known to be) a go-susi.
        // In the case of a go-susi we can rely on it telling us about changes
        // reliably.
        syncIfNotGoSusi[siserver] = true

      }
    }
  }
  
  for siserver := range syncIfNotGoSusi {
    Peer(siserver).SyncIfNotGoSusi()
  }
  
  return
}
