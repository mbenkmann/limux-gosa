{if !$init_successfull}
<br>
<b>{msgPool type=siError}</b><br>
{t}Check if the GOsa support daemon (gosa-si) is running.{/t}&nbsp;
<button type='submit' name='retry_init'>{t}Retry{/t}</button>

<br>
<br>
{else}


<table width="100%" summary="{t}License usage{/t}">
  <tr>
    <td style='width: 50%; padding-right:5px; ' class='right-border'>        <h3>{t}Reserved for{/t}</h3>
{render acl=$boundToHostACL}
        {$licenseReserved}
{/render}
{render acl=$boundToHostACL}
        <select name='availableLicense' size=1>
{/render}
          {html_options options=$availableLicenses}
        </select>
{render acl=$boundToHostACL}
        <button type='submit' name='addReservation'>{msgPool type=addButton}</button>

{/render}
    </td>
    <td>        <h3>{t}Licenses used{/t}</h3>
{render acl=$boundToHostACL}
        {$licenseUses}
{/render}
    </td>
  </tr>
</table>

<input name='opsiLicenseUsagePosted' value='1' type='hidden'>
{/if}
