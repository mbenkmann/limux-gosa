
<table width="100%" summary="{t}FAI script{/t}">
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
       <input type='text' value="{$cn}"  maxlength="80" size="45" disabled id="cn">
      {/render}
     </td>
    </tr>
    <tr>
     <td><LABEL for="description">{t}Description{/t}</LABEL>
     </td>
     <td>
      {render acl=$descriptionACL}
       <input type='text' size="45" maxlength="80" value="{$description}" name="description" id="description" >
      {/render}
     </td>
    </tr>
   </table>
  </td>
 </tr>
</table>
<hr>
<table width="100%" summary="{t}List of scripts{/t}">
 <tr>
  <td>
   <h3>{t}List of scripts{/t}
   </h3>
   {$Entry_listing}
   <hr>
   
   {if $sub_object_is_addable}
    <button type='submit' name='AddSubObject' title="{msgPool type=addButton}">
    {msgPool type=addButton}</button>
    {else}
    <button type='submit' name='Dummy2' title="{msgPool type=addButton}">
    {msgPool type=addButton}</button>
    
   {/if}
  </td>
 </tr>
</table>
<input type="hidden" value="1" name="FAIscript_posted" ><!-- Place cursor -->
<script language="JavaScript" type="text/javascript"><!--	focus_field("cn","description");--></script>