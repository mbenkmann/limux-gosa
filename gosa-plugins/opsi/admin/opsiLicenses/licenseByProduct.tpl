{if !$init_successfull}
<br>
<b>{msgPool type=siError}</b><br>
{t}Check if the GOsa support daemon (gosa-si) is running.{/t}&nbsp;
<button type='submit' name='retry_init'>{t}Retry{/t}</button>

<br>
<br>
{else}

<!-- GENERIC -->
<h3>{t}License usage{/t}</h3>

{$licenseUses}

<input name='opsiLicenseUsagePosted' value='1' type='hidden'>
{/if}

<hr>
<div style='width:100%; text-align: right; padding:3px;'>
  <button type='submit' name='save_properties'>{msgPool type=saveButton}</button>

  &nbsp;
  <button type='submit' name='cancel_properties'>{msgPool type=cancelButton}</button>

</div>

