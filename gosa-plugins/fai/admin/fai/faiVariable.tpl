
<table summary="" width="100%">
 <tr>
  <td width="50%" valign="top">
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
       <input type='text' size="45" maxlength="80" value="{$description}" name="description" id="description">
      {/render}
     </td>
    </tr>
   </table>
  </td>
  <td class='left-border'>&nbsp;
  </td>
  <td>
   <h3><LABEL for="SubObject">{t}List of assigned variables{/t}</LABEL>
   </h3>
   <table summary="" width="100%">
    <tr>
     <td>
      <select name="SubObject[]" multiple  title="{t}Choose a variable to delete or edit{/t}" 							style="width:100%;" size="20" id="SubObject">
       {html_options options=$SubObjects}
      </select>
      <br>
      
      {if $is_createable}
       <button type='submit' name='AddSubObject' title="{msgPool type=addButton}">
       {msgPool type=addButton}</button>
       {else}
       <button type='submit' name='AddSubObject' title="{msgPool type=addButton}">
       {msgPool type=addButton}</button>
       
      {/if}
      <button type='submit' name='EditSubObject' title="{t}Edit{/t}">{t}Edit{/t}</button>
      
      {if $is_removeable}
       <button type='submit' name='DelSubObject' title="{msgPool type=delButton}">
       {msgPool type=delButton}</button>
       {else}
       <button type='submit' name='DelSubObject' title="{msgPool type=delButton}">
       {msgPool type=delButton}</button>
       
      {/if}
     </td>
    </tr>
   </table>
  </td>
 </tr>
</table>
<input type="hidden" value="1" name="FAIvariable_posted"><!-- Place cursor -->
<script language="JavaScript" type="text/javascript"><!-- // First input field on page	focus_field('cn','description');  --></script>