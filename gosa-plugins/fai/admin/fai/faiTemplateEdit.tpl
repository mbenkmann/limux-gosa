
<h3>Template entry
</h3>

{if !$mb_extension}
 {msgPool type=missingext param1='multi byte'}
 <p class='seperator'>
  <div style='text-align:right;'>
   <button type='submit' name='templateEditCancel'>
   {msgPool type=cancelButton}</button>
  </div>
 </p>
 {else}
 
 {if $write_protect}{t}This FAI template is write protected. Editing may break it!{/t}
  <br>
  <button type='submit' name='editAnyway'>{t}Edit anyway{/t}</button>
  
 {/if}
 
 
 <textarea {if $write_protect} disabled {/if}  style='width:100%; height: 350px;' 
     {if !$write_protect}name="templateValue"{/if}>{$templateValue}</textarea>

 <div class="plugin-actions">
  <button type='submit' name='templateEditSave'>{msgPool type=okButton}</button>&nbsp;
  <button type='submit' name='templateEditCancel'>{msgPool type=cancelButton}</button>
 </div>
 
{/if}
