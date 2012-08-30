/* 
Copyright (c) 2012 Landeshauptstadt München
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
         "../xml"
       )

/* gosa_query_jobdb

Relevant gosa-si-server files
/usr/lib/gosa-si/server/ServerPackages/databases.pm
/usr/lib/gosa-si/server/GosaPackages/databases.pm   (identical)

Example message:
      <xml>
        <header>gosa_query_jobdb</header>
        <target>GOSA</target>
        <source>GOSA</source>
        <where>
            <clause>
                <connector>or</connector>
                <phrase>
                    <operator>eq</operator>
                    <macaddress>00:1d:60:7e:9b:f6</macaddress>
                </phrase>
            </clause>
        </where>
      </xml>

Example replies:
      <xml>
        <header>query_jobdb</header>
        <source>172.16.2.143:20081</source>
        <target>GOSA</target>
        <session_id>2427</session_id>
      </xml>

      <xml>
        <header>query_jobdb</header>
        <source>172.16.2.143:20081</source>
        <target>GOSA</target>
        <answer1>
            <plainname>grisham</plainname>
            <progress>none</progress>
            <status>waiting</status>
            <siserver>localhost</siserver>
            <modified>0</modified>
            <targettag>00:0c:29:50:a3:52</targettag>
            <macaddress>00:0c:29:50:a3:52</macaddress>
            <timestamp>20120824131849</timestamp>
            <id>1</id>
            <headertag>trigger_action_reinstall</headertag>
            <result>none</result>
            <xmlmessage>PHhtbD48aGVhZGVyPmpvYl90cmlnZ2VyX2FjdGlvbl9yZWluc3RhbGw8L2hlYWRlcj48c291cmNl
PkdPU0E8L3NvdXJjZT48dGFyZ2V0PjAwOjBjOjI5OjUwOmEzOjUyPC90YXJnZXQ+PHRpbWVzdGFt
cD4yMDEyMDgyNDEzMTg0OTwvdGltZXN0YW1wPjxtYWNhZGRyZXNzPjAwOjBjOjI5OjUwOmEzOjUy
PC9tYWNhZGRyZXNzPjwveG1sPg==
</xmlmessage>
        </answer1>
        <answer2>
            <plainname>grisham</plainname>
            <progress>none</progress>
            <status>waiting</status>
            <siserver>localhost</siserver>
            <modified>0</modified>
            <targettag>00:0c:29:50:a3:52</targettag>
            <macaddress>00:0c:29:50:a3:52</macaddress>
            <timestamp>20120824143205</timestamp>
            <id>2</id>
            <headertag>trigger_action_reboot</headertag>
            <result>none</result>
            <xmlmessage>PHhtbD48aGVhZGVyPmpvYl90cmlnZ2VyX2FjdGlvbl9yZWJvb3Q8L2hlYWRlcj48c291cmNlPkdP
U0E8L3NvdXJjZT48dGFyZ2V0PjAwOjBjOjI5OjUwOmEzOjUyPC90YXJnZXQ+PHRpbWVzdGFtcD4y
MDEyMDgyNDE0MzIwNTwvdGltZXN0YW1wPjxwZXJpb2RpYz43X2RheXM8L3BlcmlvZGljPjxtYWNh
ZGRyZXNzPjAwOjBjOjI5OjUwOmEzOjUyPC9tYWNhZGRyZXNzPjwveG1sPg==
</xmlmessage>
        </answer2>
        <session_id>5288</session_id>
      </xml>

      where the base64-encoded <xmlmessage> are:
      <xml>
        <header>job_trigger_action_reinstall</header>
        <source>GOSA</source>
        <target>00:0c:29:50:a3:52</target>
        <timestamp>20120824131849</timestamp>
        <macaddress>00:0c:29:50:a3:52</macaddress>
      </xml>
      
      and
      
      <xml>
        <header>job_trigger_action_reboot</header>
        <source>GOSA</source>
        <target>00:0c:29:50:a3:52</target>
        <timestamp>20120824143205</timestamp>
        <periodic>7_days</periodic>
        <macaddress>00:0c:29:50:a3:52</macaddress>
      </xml>
      
      
      Note that <periodic>7_days</periodic> is only in the embedded <xmlmessage>, 
      not in the <answer2> wrapper.

Explanation:
      Returns all entries from the jobdb that match a given filter. 
      
      The message has the following general structure:
      
      <xml>
        <header>gosa_query_jobdb</header>
        <target>GOSA</target>
        <source>GOSA</source>
        <where>
            <clause>
                <connector>or</connector>
                <phrase>
                    <operator>eq</operator>
                    <macaddress>00:1d:60:7e:9b:f6</macaddress>
                </phrase>
            </clause>
        </where>
      </xml>
      
      The tags have the following meaning:
      
      <source>,<target> "GOSA"
      <where>           (exactly 1) The filter that selects the jobs to return.
                        The parsing of this filter is done in
                        /usr/share/perl5/GOSA/GosaSupportDaemon.pm:get_where_statement()
      <clause>          (0 or more) A filter condition. 
                        All <clause> filter conditions within <where> are ANDed.
      <connector>       (0 or 1) If not provided, the default connector is "AND"
                        All <phrase> filter conditions within a <clause> are combined
                        by this operator like this:
                           P1 c P2 c P3 c ... Pn
                        where Pi are the phrase filters and c is the connector.
                        In theory anything that results in valid SQL
                        is possible here, but I guess only "AND" and "OR" are actually
                        used. The connector string is case-insensitive because gosa-si-server
                        ucases it.
      <phrase>          (0 or more) A single primitive filter condition. In addition to
                        one <operator> element (which is optional and will be assumed to
                        be "eq" if not present) a <phrase> must contain exactly one
                        other element. The element's name specifies the column name
                        in the database and the element's text content the value to
                        compare against. The comparison is performed according to <operator>.
                        In the case of the jobdb, the valid elements inside <phrase> are
                        <id>, <timestamp>, <status>, <result>, <progress>, <headertag>,
                        <targettag>, <xmlmessage>, <macaddress>, <plainname>, 
                        <siserver> and <modified>.
      <operator>        The comparison operator for the <phrase>. Permitted operators are
                        "eq", "ne", "ge", "gt", "le", "lt" with their obvious meanings, and
                        "like" which performs a case-insensitive match against a pattern
                        that may include "%" to match any sequence of 0 or more characters
                        and "_" to match exactly one character. A literal "%" or "_" cannot
                        be embedded in such a pattern.
      
      
      The reply has the following general structure:
      
      <xml>
        <header>query_jobdb</header>
        <source>172.16.2.143:20081</source>
        <target>GOSA</target>
        <session_id>2427</session_id>
        <answer1>...</answer1>
        <answer2>...</answer2>
        ...
      </xml>
      
      If there are no answers that match the query, there are no <answerX> tags in
      the reply but the rest remains the same. The tags have the following meaning:
      
      <source>  The gosa-si server whose jobdb results are contained in the query.
      <target>  "GOSA"
      <session_id> ??? A POE session ID. Could be used by one gosa-si-server to
                   forward a request to another so that the other can reply 
                   asynchronously via a different TCP connection and the receiving 
                   gosa-si-server will know to which POE session the reply belongs,
                   so that it can forward the reply over the correct connection
                   (which is tied to the POE session).
      <plainname>  The name (without domain) of the host affected by the job.
      <progress>   Possible values:
                    "none"  The job has not started yet.
                    ????
      <status>     Possible values: 
                    "waiting"
      <siserver>   ???  Which gosa-si-server is responsible for triggering the job.
                   Possible values:
                     "localhost"  ??? This job belongs to the gosa-si-server whose jobdb it is in.
      <modified>   ???
      <targettag>  ???  Maybe the way the target should be identified. So far I've only seen
                        the target's MAC address repeated here.
      <macaddress> The affected host's MAC address.
      <timestamp>  When the job should be executed. Format: YYYYMMDDHHMMSS
      <id>         The answer's id. <answerXX> has <id>XX</id>
      <headertag>  Identifies the type of job.
                   Possible values:
                      "trigger_action_reboot"
      <result>     Possible values:
                    "none"  ??? No result expected
      <xmlmessage> base64-encoded <xml>...</xml> message that will be sent out to the target
                   when the job's time has come.

GOSA notes:      
      AFAICT GOSA doesn't care about the reply's <source>, <target> and <session_id>.
      GOsa sends these requests in 
      include/class_gosaSupportDaemon.inc:get_queued_entries()
        called by
      plugins/addons/goto/class_filterGotoEvents.inc:query()

go-susi notes:


*/
func gosa_query_jobdb(xml *xml.Hash) string {
  return ""
}
