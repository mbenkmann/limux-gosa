
<table width="100%" summary="{t}FAI partition table{/t}">
 <tr>
  <td width="50%" valign="top">
   <h3>{t}Generic{/t}
   </h3>
   <table summary="{t}Generic settings{/t}" cellspacing="4">
    <tr>
     <td><LABEL for="cn">{t}Name{/t}
      {$must}</LABEL>
     </td>
     <td>
      {render acl=$cnACL}
       <input type='text' value="{$cn}" size="45" maxlength="80" id='cn' disabled >
      {/render}
     </td>
    </tr>
    <tr>
     <td><LABEL for="description">{t}Description{/t}</LABEL>
     </td>
     <td>
      {render acl=$descriptionACL}
       <input type='text' value="{$description}" size="45" maxlength="80" name="description" id="description">
      {/render}
     </td>
    </tr>
   </table>
   <hr>
   <p>
    <input type="checkbox" name="mode" value="1" {$mode} {$lockmode} id='setup-storage'
      onClick="changeState('AddRaid'); changeState('AddVolgroup');">
    <label for='setup-storage'>{t}Use 'setup-storage' to partition the disk{/t}</label>
   </p>
  </td>
  <td class='left-border'>&nbsp;
  </td>
  <td>
   <h3><LABEL for="SubObject">{t}Discs{/t}</LABEL>
   </h3>
   {$Entry_listing}
   
   {if $sub_object_is_addable}
    <button type='submit' name='AddDisk' title="{t}Add disk{/t}">{t}Add disk{/t}</button>
    <button {$storage_mode} {$addraid} type='submit' name='AddRaid' id="AddRaid" title="{t}Add RAID{/t}">{t}Add RAID{/t}</button>
    <button {$storage_mode} type='submit' name='AddVolgroup' id="AddVolgroup" title="{t}Add volume group{/t}">{t}Add volume group{/t}</button>
    {else}
    <button type='button' disabled name='AddDisk' title="{t}Add disk{/t}">{t}Add disk{/t}</button>
    <button type='button' disabled name='AddRaid' title="{t}Add RAID{/t}">{t}Add RAID{/t}</button>
    <button type='button' disabled name='AddVolgroup' title="{t}Add volume group{/t}">{t}Add volume group{/t}</button>
    
   {/if}
  </td>
 </tr>
</table>
<input type='hidden' name='FAIpartitionTablePosted' value='1'><!-- Place cursor -->
<script language="JavaScript" type="text/javascript"><!-- // First input field on page	focus_field('cn','description');  --></script>
