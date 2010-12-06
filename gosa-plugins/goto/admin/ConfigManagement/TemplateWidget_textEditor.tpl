{if $write_protect}
  {t}The text is write protected due to its encoding. Editing may break it!{/t}
  <br>
  <button type='submit' name='editAnyway'>{t}Edit anyway{/t}</button>
{/if}
<textarea {if $write_protect} disabled {/if} {if !$write_protect} name="{$postName}" {/if}
    style="width:100%;height:300px;" id="{$postName}"
    rows="20" cols="120">{$value}</textarea>
<div>
  <input type="file" name="ImportFile">&nbsp;
  <button type='submit' name='ImportUpload'>{t}Import text{/t}</button>
</div>
