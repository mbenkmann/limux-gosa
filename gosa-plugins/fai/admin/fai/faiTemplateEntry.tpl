
<input type="hidden" name="SubObjectFormSubmitted" value="1">
<h3>{t}Generic{/t}
</h3>
<table width="100%" summary="{t}FAI template entry{/t}">
 <tr>
  <td style='width:50%;padding-right:10px;' class='right-border'>
   <table style='width:100%;' summary="{t}Generic settings{/t}">
    <tr>
     <td>{t}File name{/t}{$must}&nbsp;</td>
     <td style='width:100%;'>
      {render acl=$cnACL}
       <input value="{$templateFile}" type='text' name="templateFile">
      {/render}
     </td>
    </tr>
    <tr>
     <td>
        <LABEL for="templatePath">{t}Destination path{/t}{$must}&nbsp;</LABEL></td>
     <td>
      {render acl=$FAItemplatePathACL}
       <input type="text" name="templatePath" value="{$templatePath}" id="templatePath" >
      {/render}
     </td>
    </tr>
   </table>
  </td>
  <td>{t}Description{/t}
   {render acl=$descriptionACL}
    <input value="{$description}" name="description" type='text'>
   {/render}
  </td>
 </tr>
</table>
<hr>
<table width="100%" summary="{t}Template attributes{/t}">
 <tr>
  <td colspan=2>
   <h3>{t}Template attributes{/t}</h3>
  </td>
 </tr>
 <tr>
  <td style='width:50%;' class='right-border'>
   <table summary="{t}Template file status{/t}">
    <tr>
     <td>{t}File{/t}{$must}:&nbsp;
      {$status}
      
      {if $bStatus}
        {image path='images/save.png' action='getFAItemplate' title='{t}Save template{/t}...'}
        {image path='images/lists/edit.png' action='editFAItemplate' title='{t}Edit template{/t}...'}
      {/if}
     </td>
    </tr>
    
    {if $bStatus}
     <tr>
      <td>{t}Full path{/t}:&nbsp;<i>
       {$FAItemplatePath}</i>
      </td>
     </tr>
     
    {/if}
    <tr>
     <td class='center'>
      {render acl=$FAItemplateFileACL}
       <input type="file" name="FAItemplateFile" value="" id="FAItemplateFile">
      {/render}
      {render acl=$FAItemplateFileACL}
       <button type='submit' name='TmpFileUpload'>{t}Upload{/t}</button>
      {/render}
     </td>
    </tr>
   </table>
  </td>
  <td>
   <table summary="{t}File attributes{/t}">
    <tr>
     <td><LABEL for="user">{t}Owner{/t}{$must}&nbsp;</LABEL>
     </td>
     <td>
      {render acl=$FAIownerACL}
       <input type="text" name="user" value="{$user}" id="user" size="15">
      {/render}
     </td>
    </tr>
    <tr>
     <td><LABEL for="group">{t}Group{/t}{$must}&nbsp;</LABEL>
     </td>
     <td>
      {render acl=$FAIownerACL}
       <input type="text" name="group" value="{$group}" id="group" size="15">
      {/render}
      <br>
      <br>
     </td>
    </tr>
    <tr>
     <td>{t}Access{/t}{$must}&nbsp;
     </td>
     <td>
      <table summary="{t}File permissions{/t}"><colgroup width="55" span="3"></colgroup>
       <tr><th>{t}Class{/t}</th><th>{t}Read{/t}</th><th>{t}Write{/t}</th><th>{t}Execute{/t}</th><th>&nbsp;</th><th>{t}Special{/t}</th><th>&nbsp;</th>
       </tr>
       <tr>
        <td>{t}User{/t}
        </td>
        {render acl=$FAImodeACL}
         <td align="center">
          <input type="checkbox" name="u4" value="4" {$u4}>
         </td>
        {/render}
        {render acl=$FAImodeACL}
         <td align="center">
          <input type="checkbox" name="u2" value="2" {$u2}>
         </td>
        {/render}
        {render acl=$FAImodeACL}
         <td align="center">
          <input type="checkbox" name="u1" value="1" {$u1}>
         </td>
        {/render}
        <td>&nbsp;
        </td>
        {render acl=$FAImodeACL}
         <td align="center">
          <input type="checkbox" name="s4" value="4" {$s4}>
         </td>
        {/render}
        <td>(SUID)
        </td>
       </tr>
       <tr>
        <td>{t}Group{/t}
        </td>
        {render acl=$FAImodeACL}
         <td align="center">
          <input type="checkbox" name="g4" value="4" {$g4}>
         </td>
        {/render}
        {render acl=$FAImodeACL}
         <td align="center">
          <input type="checkbox" name="g2" value="2" {$g2}>
         </td>
        {/render}
        {render acl=$FAImodeACL}
         <td align="center">
          <input type="checkbox" name="g1" value="1" {$g1}>
         </td>
        {/render}
        <td>&nbsp;
        </td>
        {render acl=$FAImodeACL}
         <td align="center">
          <input type="checkbox" name="s2" value="2" {$s2}>
         </td>
        {/render}
        <td>(SGID)
        </td>
       </tr>
       <tr>
        <td>{t}Others{/t}
        </td>
        {render acl=$FAImodeACL}
         <td align="center">
          <input type="checkbox" name="o4" value="4" {$o4}>
         </td>
        {/render}
        {render acl=$FAImodeACL}
         <td align="center">
          <input type="checkbox" name="o2" value="2" {$o2}>
         </td>
        {/render}
        {render acl=$FAImodeACL}
         <td align="center">
          <input type="checkbox" name="o1" value="1" {$o1}>
         </td>
        {/render}
        <td>&nbsp;
        </td>
        {render acl=$FAImodeACL}
         <td align="center">
          <input type="checkbox" name="s1" value="1" {$s1}>
         </td>
        {/render}
        <td>({t}sticky{/t})
        </td>
       </tr>
      </table>
     </td>
    </tr>
   </table>
  </td>
 </tr>
 <tr>
  <td colspan=2>
   <br>
   <hr>
   <br>
   <div class="plugin-actions">
    
    {if !$freeze}
     <button type='submit' name='SaveSubObject'>
     {msgPool type=applyButton}</button>&nbsp;
     
    {/if}
    <button type='submit' name='CancelSubObject'>
    {msgPool type=cancelButton}</button>
   </div>
  </td>
 </tr>
</table>
<input type='hidden' name='FAItemplateEntryPosted' value='1'><!-- Place cursor -->
<script language="JavaScript" type="text/javascript"><!-- // First input field on page	focus_field('cn','description');  --></script>
