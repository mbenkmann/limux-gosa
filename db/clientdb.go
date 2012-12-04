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

package db

import "../xml"


/*
NOTE: Test cases for clientdb:

- send a job_trigger_action_wake for a client that is not in clientdb and
  can't otherwise be resolved. Check that go-susi sends the test server a
  "trigger_wake" message asking it to help with waking the client.

- send a new_foreign_client message for the client from the previous test.
  Now check that go-susi no longer sends trigger_wake when asked to
  wake up the client (because now go-susi knows the client)

- try the above 2 tests with new_server message and its <client> element

- try the 2 tests with confirm_new_server's <client> element

- try the 2 tests with a here_i_am message


*/




// Returns the entry from the clientdb 
// (format: <xml><client>172.16.2.52:20083</client><macaddress>...</xml>) of
// the client with the given MAC address, or nil if the client is not in
// the clientdb.
func ClientWithMAC(macaddress string) *xml.Hash { return nil }
