
<input type="hidden" name="SubObjectFormSubmitted" value="1">
<table width="100%" summary="{t}FAI variable entry{/t}">
 <tr>
  <td valign="top" width="50%">
   <h3>{t}Generic{/t}
   </h3>
   <table summary="{t}Generic settings{/t}">
    <tr>
     <td>{t}Name{/t}
      {$must}&nbsp;
     </td>
     <td>
      {render acl=$cnACL}
       <input type='text' value="{$cn}" size="45" maxlength="80" name="cn">
      {/render}
     </td>
    </tr>
    <tr>
     <td>{t}Description{/t}&nbsp;
     </td>
     <td>
      {render acl=$descriptionACL}
       <input type='text' value="{$description}" size="45" maxlength="80" name="description">
      {/render}
     </td>
    </tr>
   </table>
  </td>
  <td class='left-border'>&nbsp;
  </td>
  <td valign="top">
   <h3>{t}Variable attributes{/t}
   </h3>
   <table  summary="{t}Variable attributes{/t}" width="100%">
    <tr>
     <td><LABEL for="Content">{t}Variable content{/t}
      {$must}&nbsp;</LABEL>
     </td>
     <td>
      {render acl=$FAIvariableContentACL}
       <input type="text" name="FAIvariableContent" value="{$FAIvariableContent}" id="Content" style="width:250px;">
      {/render}
     </td>
    </tr>
   </table>
  </td>
 </tr>
</table>
<hr>
<br>
<div class="plugin-actions">
 
 {if !$freeze}
  <button type='submit' name='SaveSubObject'>
  {msgPool type=applyButton}</button>&nbsp;
  
 {/if}
 <button type='submit' name='CancelSubObject'>
 {msgPool type=cancelButton}</button>
</div><!-- Place cursor -->
<script language="JavaScript" type="text/javascript"><!-- // First input field on page	focus_field('cn','description');  --></script>