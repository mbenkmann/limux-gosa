
<div id="mainlist">
 <div class="mainlist-header">
  <p>{t}Mail queue{/t}
  </p>
  <div class="mainlist-nav">
   <table summary="{t}Filter{/t}" style="width: 100%;"      id="t_scrolltable" cellpadding="0" cellspacing="0">
    <tr>
     <td>{t}Search on{/t}
      <select size="1" name="p_server" title="{t}Select a server{/t}" onchange="mainform.submit()">
       {html_options values=$p_serverKeys output=$p_servers selected=$p_server}
      </select>
     </td>
     <td>{t}Search for{/t}
      <input type='text' name="search_for" size=25 maxlength=60        
        value="{$search_for}" title="{t}Enter user name to search for{/t}"        
        onChange="mainform.submit()">
     </td>
     <td>{t}within the last{/t}&nbsp;
      <select size="1" name="p_time" onchange="mainform.submit()">
       {html_options values=$p_timeKeys output=$p_times selected=$p_time}
      </select>
     </td>
     <td>
      <button type='submit' name='search'>{t}Search{/t}</button>
     </td>
     <td>
      {if $delAll_W}
       <input name="all_del"  src="images/lists/trash.png"							
        value="{t}Remove all messages{/t}" type="image" 					
        title="{t}Remove all messages from selected servers queue{/t}">
      {/if}
      {if $holdAll_W}
       <input name="all_hold" src="plugins/mail/images/mailq_hold.png"					
        value="{t}Hold all messages{/t}" type="image"					
        title="{t}Hold all messages in selected servers queue{/t}">
      {/if}
      {if $unholdAll_W}
       <input name="all_unhold" src="plugins/mail/images/mailq_unhold.png"							
        value="{t}Release all messages{/t}" 	type="image"					
        title="{t}Release all messages in selected servers queue{/t}">
      {/if}
      {if $requeueAll_W}
       <input name="all_requeue" src="images/lists/reload.png"							
        value="{t}Re-queue all messages{/t}" type="image"					
        title="{t}Re-queue all messages in selected servers queue{/t}">
      {/if}
     </td>
    </tr>
   </table>
  </div>
 </div>
</div>

<br>

{if !$query_allowed}
<b>{msgPool type=permView}</b>

{else}

 {if $all_ok != true}

  <b>{t}Search returned no results{/t}...</b>

 {else}

  <div class="listContainer" id="d_scrollbody" style="min-height: 475px; height: 444px;">
   <table summary="{t}Phone reports{/t}" style="width:100%;" cellpadding="0" cellspacing="0">
    <thead class="fixedListHeader listHeaderFormat">
     <tr>
      <td class='listheader'>
       <input type='checkbox' id='select_all' name='select_all' 
          title='"._("Select all")."' onClick="toggle_all_('^selected_.*$','select_all');">
      </td> 
      <td class='listheader'><a href="{$plug}&amp;sort=MailID">{t}ID{/t}{if $OrderBy == "MailID"} {$SortType}{/if}</a></td>
      <td class='listheader'><a href="{$plug}&amp;sort=Server">{t}Server{/t}{if $OrderBy == "Server"}{$SortType}{/if}</a></td>
      <td class='listheader'><a href="{$plug}&amp;sort=Size">{t}Size{/t}{if $OrderBy == "Size"} {$SortType}{/if}</a></td>
      <td class='listheader'><a href="{$plug}&amp;sort=Arrival">{t}Arrival{/t}{if $OrderBy == "Arrival"}{$SortType}{/if}</a></td>
      <td class='listheader'><a href="{$plug}&amp;sort=Sender">{t}Sender{/t}{if $OrderBy == "Sender"}{$SortType}{/if}</a></td>
      <td class='listheader'><a href="{$plug}&amp;sort=Recipient">{t}Recipient{/t}{if $OrderBy == "Recipient"}{$SortType}{/if}</a></td>
      <td class='listheader'><a href="{$plug}&amp;sort=Status">{t}Status{/t}{if $OrderBy == "Status"}{$SortType}{/if}</a></td>
      <td class='listheader'>&nbsp;</td>
     </tr>
    </thead>
    <tbody class="listScrollContent listBodyFormat" id="t_nscrollbody">


     {foreach from=$entries item=val key=key}
      <tr>
       <td class="list0">
        <input id="selected_{$entries[$key].MailID}" type='checkbox' 
         name='selected_{$entries[$key].MailID}_{$entries[$key].Server}' class='center'>
       </td>
       <td class="list0">
        {if $entries[$key].Active == true}
         {image path="plugins/mail/images/mailq_active.png"}
        {/if}
        {$entries[$key].MailID}
       </td>
       <td class="list0">{$entries[$key].ServerName}</td>
       <td class="list0">{$entries[$key].Size}</td>
       <td class="list0">{$entries[$key].Arrival|date_format:"%d.%m.%Y %H:%M:%S"}</td>
       <td class="list0">{$entries[$key].Sender}</td>
       <td class="list0">{$entries[$key].Recipient}</td>
       <td class="list0">{$entries[$key].Status}</td>
       <td class="list0" style='border-right: 0pt none;'>
        {if $del_W}
         {image action="del__{$entries[$key].MailID}__{$entries[$key].Server}" 
           path="images/lists/trash.png" title="{t}Delete this message{/t}"}
        {else}
         {image path="images/empty.png"}
        {/if}
        
        {if $entries[$key].Hold == true}
         {if $unhold_W}
          {image action="unhold__{$entries[$key].MailID}__{$entries[$key].Server}" 
            path="plugins/mail/images/mailq_unhold.png" title="{t}Release message{/t}"}
          {else}
           {image path="images/empty.png"}
          {/if}
         {else}
          {if $hold_W}
           {image action="hold__{$entries[$key].MailID}__{$entries[$key].Server}" 
             path="plugins/mail/images/mailq_hold.png" title="{t}Hold message{/t}"}
          {else}
           {image path="images/empty.png"}
          {/if}
         {/if}
        
         {if $requeue_W}
          {image action="requeue__{$entries[$key].MailID}__{$entries[$key].Server}" 
            path="images/lists/reload.png" title="{t}Re-queue this message{/t}"}
         {else}
          {image path="images/empty.png"}
         {/if}
        
         {if $header_W}
          {image action="header__{$entries[$key].MailID}__{$entries[$key].Server}" 
            path="plugins/mail/images/mailq_header.png" title="{t}Display header of this message{/t}"}
         {else}
          {image path="images/empty.png"}
         {/if}
       </td>
      </tr>
     {/foreach}
     <tr>
      <td class="list0">&nbsp;</td>
      <td class="list0">&nbsp;</td>
      <td class="list0">&nbsp;</td>
      <td class="list0">&nbsp;</td>
      <td class="list0">&nbsp;</td>
      <td class="list0">&nbsp;</td>
      <td class="list0" style='border-right: 0pt none;'>
</td>
     </tr>
    </tbody>
   </table>
   <table style='width:100%; text-align:center;' summary="{t}Page selector{/t}">
    <tr>
     <td>{$range_selector}</td>
    </tr>
   </table>
  </div>
  <hr>
 {/if}
{/if}
