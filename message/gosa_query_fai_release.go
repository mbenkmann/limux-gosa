/*
Copyright (c) 2013 Landeshauptstadt München
Author: Matthias S. Benkmann

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
         "time"
         "strconv"
         "strings"
         
         "../xml"
         "../util"
         "../config"
       )

// Handles the message "gosa_query_fai_release".
//  xmlmsg: the decrypted and parsed message
// Returns:
//  unencrypted reply
func gosa_query_fai_release(xmlmsg *xml.Hash) string {
  release := xmlmsg.String()
  release = strings.SplitN(release, "</fai_release>", 2)[0]
  release = strings.SplitN(release, "<fai_release>", 2)[1]
  
  reply := xml.NewHash("xml","header","query_fai_release")
  reply.Add("source", config.ServerSourceAddress)
  reply.Add("target", "GOSA")
  reply.Add("session_id","1")
  
  var count uint64 = 1
  for _, class := range []string{"ACPIOFF", "BALAPTOP", "BC-VL-faimond-check", "BC-ZF-ARCOR-IP", "BC-ZF-HDENC", "BC-ZF-Telearbeit-Forcelock", "BC-ZF-Telearbeit-online", "BC-ZF-USBENC", "BC-ZF-VPNC", "BC-ZF-gosa-si-kein-dnslookup", "DEPOTSERVER", "DKTEST02052012", "FF-TEST", "FF-UPDATES", "FIXES", "HARDENING", "HARDENING_SERVER", "I810-BEAMER-FIX", "ISDN", "LAST", "LUKS_GRUB_PW", "Modul_Bezirksausschuss", "Modul_Stadtrat", "Modul_Standard", "Modul_Telearbeit_DSL", "Modul_Telearbeit_Offline", "NOLAPIC", "NOTEBOOK", "PLUS", "PPDS", "PROPOSED_PLUS", "PROPOSED_SECURITY_UPDATES", "SECURITY_UPDATES", "TESTDK02052012", "UPDATES_INSTALL", "Verteilserver-Produktiv", "_ADD_XUMIL_USER", "_ASTEC", "_BC-VL-Kartenleser", "_BC-VL-reset-etc-hosts", "_BNK_DENKBRETT", "_BNK_GRISHAM", "_BNK_HYDRA_ZWINGEND_NOETIG", "_BNK_NOVA_ZWINGEND_NOETIG", "_BNK_PART_ALT_MIT_PRESERVE", "_BNK_PART_ALT_OHNE_PRESERVE", "_BNK_PART_GPT", "_BNK_PART_NEU_MIT_PRESERVE", "_BNK_PART_NEU_OHNE_PRESERVE", "_BNK_TEST", "_BNK_UPDATES_REPOSITORY", "_DEPOTSERVER_deknos_plophos", "_DKNS", "_DKN_PL_AKON", "_ENTWICKLUNG-BC", "_ENTWICKLUNG-BC2", "_ENTWICKLUNG-VTS", "_FIT", "_FIT123", "_FIT2", "_FIT_Firstboot_check", "_FIT_GOSA_SI_CLIENT", "_FIT_GOSA_SI_CLUSTER", "_FIT_HDENC", "_FIT_PAKETE", "_FIT_SAVEPART", "_FIT_TP118_test", "_FIT_UCF_OVERRIDE", "_FIT_VOrTEST", "_FIT_repo", "_FOS", "_ForsterTest", "_Forster_Test_Java", "_Forster_Umbenennungstest", "_GLG", "_GLG_QEMU", "_GRACA_AUTODETECT", "_KRB5", "_LIBATA_NOACPI", "_LOGFILES_ZUVERLAESSIG_SPEICHERN", "_LOGSPAMMER", "_LUKS_PW", "_LiMuxClient", "_MAF_41PLUS", "_MINIMUENCHEN", "_Modul_Standard_DEKNOS", "_Modul_Standard_deknos_plophos1", "_NOUVEAU", "_PLOPHOS", "_PLOPHOS_FIT", "_PLOPHOS_FIT2", "_PLO_AKON", "_TELE_GRUB_PW", "_TELE_GRUB_PW_test", "_TELE_LUKS_GRUB_PW", "_TEST40", "_TEST_8954", "_UPDATES_VERTEILSERVER", "_VERTEILSERVER_FIXES", "_VERTEILSERVER_UPDATES", "_VboxVirtualisierung", "_Verteilserver-Entwicklung", "_Verteilserver-Entwicklung_deknos", "_WAIT", "__VboxVirtualisierung", "_basismodul_deknos_plophos1", "_basismodul_minimalist", "_basismodul_trinitytest", "_blubb", "_blubbtest", "_cheeseburger2", "_cheeseburger2profile", "_dkns_paket", "_dkns_skripte", "_fit_grep", "_fit_printer", "_fit_tp118_basismodul", "_fos_eigene_Paketliste", "_fos_ext4", "_fos_partitionstabelle_neu", "_gevtest", "_jmg-test2", "_mini", "_mstest123", "_set-hwclock", "_test-jmg", "_test8296", "_test8372", "_test_GRUB_PW", "_test_anzeige", "_test_bauadobe", "_test_biblfix", "_test_biblinfom", "_test_ff10", "_test_mk", "_test_opengl", "_test_printermanager", "_test_repo_haftmann", "_test_reset", "_test_umbruch", "_test_umbruch1", "_test_umbruch1_kopie", "_test_umbruch1b", "_test_upd", "_test_updta", "_test_xnview", "_testffbibl", "_testfont", "_testms23131123423", "_testmskino", "_testvm", "_tzdata-fehlendeparameter", "_wanderer", "basismodul" } {
    answer := xml.NewHash("answer"+strconv.FormatUint(count, 10))
    answer.Add("class", class)
    answer.Add("timestamp", util.MakeTimestamp(time.Now()))
    answer.Add("fai_release", release)
    answer.Add("type", "FAIprofile")
    answer.Add("state")
  
    reply.AddWithOwnership(answer)
    count++
  }
  
  return reply.String()
}
