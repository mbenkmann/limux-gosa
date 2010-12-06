
<table width="100%" summary="{t}FAI profile{/t}">
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
       <input type='text' value="{$cn}" size="45" maxlength="80" id="cn" name="cn">
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
  <td class='left-border'>
   <table summary="{t}FAI classes{/t}" width="100%">
    <tr>
     <td>
      <h3>{t}FAI classes{/t}
      </h3>
      <!--						<select name="FAIclass" title="{t}Choose a priority{/t}" size="20" style="width:100%;" multiple>
       {html_options values=$FAIclasses output=$FAIclasses}
      </select>-->
      {$listing}
      <br>
      {render acl=$FAIclassACL}
       <button type='submit' name='AddClass'>
       {msgPool type=addButton}</button>
      {/render}
      <input type="hidden" name="edit_helper">
     </td>
    </tr>
   </table>
  </td>
 </tr>
</table><!-- Place cursor -->
<script language="JavaScript" type="text/javascript"><!-- // First input field on page	focus_field('cn','description');  --></script>