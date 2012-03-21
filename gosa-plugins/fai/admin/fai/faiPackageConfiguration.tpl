<h3>{$headline}</h3>
{$Config}
<hr>
<div class="plugin-actions">
{if $writable}
 <button type='submit' name='SaveObjectConfig'>{msgPool type=applyButton}</button>
{/if}
 <button type='submit' name='CancelObjectConfig'>{msgPool type=cancelButton}</button>
</div>
