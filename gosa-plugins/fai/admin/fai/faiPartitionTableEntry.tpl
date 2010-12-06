
<input type="hidden" name="TableEntryFrameSubmitted" value="1">
<h3>{t}Device{/t}
</h3>
<table style='width:100%' summary="{t}FAI partition table entry{/t}">
 <tr>
  <td style='width:50%;' class='right-border'><LABEL for="DISKcn">{t}Name{/t}</LABEL>
   {$must}&nbsp;
   {render acl=$DISKcnACL}
    <input type='text' value="{$DISKcn}" size="45" maxlength="80" name="DISKcn" id="DISKcn">
   {/render}
  </td>
  <td><LABEL for="DISKdescription">&nbsp;{t}Description{/t}</LABEL>&nbsp;
   {render acl=$DISKdescriptionACL}
    <input type='text' value="{$DISKdescription}" size="45" maxlength="80" name="DISKdescription" id="DISKdescription">
   {/render}
  </td>
 </tr>
</table>
<br>
<hr>
<br>
<h3>{t}Partition entries{/t}
</h3>
{$setup}
<br>

{if !$freeze}
 
 {if $sub_object_is_createable}
  <button type='submit' name='AddPartition'>{t}Add partition{/t}</button>
  {else}
  <button type='submit' name='restricted'>{t}Add partition{/t}</button>
  
 {/if}
 
{/if}
<br>
<br>
<hr>
<br>
<div class="plugin-actions">
 
 {if !$freeze}
  <button type='submit' name='SaveDisk'>
  {msgPool type=saveButton}</button>
  
 {/if}
 <button type='submit' name='CancelDisk'>
 {msgPool type=cancelButton}</button>
</div><!-- Place cursor -->
<script language="JavaScript" type="text/javascript"><!-- // First input field on page	focus_field('DISK_cn');  --></script>
