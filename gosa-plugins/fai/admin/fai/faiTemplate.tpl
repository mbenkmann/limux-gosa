
<table summary="" width="100%">
 <tr>
  <td style='width:50%' class='right-border'>
   <h3>{t}Generic{/t}
   </h3>
   <table summary="" cellspacing="4">
    <tr>
     <td><LABEL for="cn">{t}Name{/t}
      {$must}</LABEL>
     </td>
     <td>
      {render acl=$cnACL}
       <input type='text' value="{$cn}" size="45" maxlength="80" disabled id="cn">
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
  </td>
  <td style="width:50%">
   <h3>{t}List of template files{/t}
   </h3>
   {$Entry_listing}
   
   {if $sub_object_is_addable}
    <button type='submit' name='AddSubObject' title="{msgPool type=addButton}">
    {msgPool type=addButton}</button>
    {else}
    <button type='submit' name='AddSubObject' disabled title="{msgPool type=addButton}">
    {msgPool type=addButton}</button>
    
   {/if}
  </td>
 </tr>
</table>
<input type="hidden" value="1" name="FAItemplate_posted"><!-- Place cursor -->
<script language="JavaScript" type="text/javascript"><!-- // First input field on page	focus_field('cn','description');  --></script>
