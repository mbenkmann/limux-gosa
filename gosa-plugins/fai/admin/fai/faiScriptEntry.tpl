<input type="hidden" name="SubObjectFormSubmitted" value="1">
<table width="100%" summary="{t}FAI script{/t}">
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
       <input type='text' size="45" maxlength="80" value="{$description}" name="description">
      {/render}
     </td>
    </tr>
   </table>
  </td>
  <td class='left-border'>&nbsp;
  </td>
  <td valign="top">
   <h3>{t}Script attributes{/t}
   </h3>
   <table summary="{t}Priority configuration{/t}" width="100%">
    <tr>
     <td><LABEL for="FAIpriority">{t}Priority{/t}</LABEL>
     </td>
     <td>
      {render acl=$FAIpriorityACL}
       <select id="FAIpriority" name="FAIpriority" title="{t}Choose a priority. Low values result in an earlier, high values in a later execution.{/t}" size=1>
        {html_options options=$FAIprioritys selected=$FAIpriority}
       </select>
      {/render}
     </td>
    </tr>
   </table>
  </td>
 </tr>
</table>
<hr>
<h3><LABEL for="FAIscript">{t}Script{/t}</LABEL></h3>

{if $write_protect}
  {t}This FAI script is write protected, due to its encoding. Editing may break it!{/t}
  <br>
  <button type='submit' name='editAnyway'>{t}Edit anyway{/t}</button>
{/if}

{render acl=$FAIscriptACL}
<textarea {if $write_protect} disabled {/if} {if !$write_protect} name="FAIscript" {/if} 
    style="width:100%;height:300px;" id="FAIscript"
    rows="20" cols="120">{$FAIscript}</textarea>
{/render}

<br>
<div>
 {render acl=$FAIscriptACL}
  <input type="file" name="ImportFile">&nbsp;
 {/render}
 {render acl=$FAIscriptACL}
  <button type='submit' name='ImportUpload'>{t}Import script{/t}</button>
 {/render}
 {render acl=$FAIscriptACL}
  {$DownMe}
 {/render}
</div>
<br>
<hr>
<div class="plugin-actions">
 
 {if !$freeze}
  <button type='submit' name='SaveSubObject'>
  {msgPool type=applyButton}</button>&nbsp;
  
 {/if}
 <button type='submit' name='CancelSubObject'>
 {msgPool type=cancelButton}</button>
</div><!-- Place cursor -->
<script language="JavaScript" type="text/javascript"><!-- // First input field on page	focus_field('cn','description');  --></script>
