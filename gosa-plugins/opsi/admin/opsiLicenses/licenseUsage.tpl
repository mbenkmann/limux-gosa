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
