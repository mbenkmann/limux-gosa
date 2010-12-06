{if !$init_successfull}

  <br>
    <b>{msgPool type=siError}</b><br>
    {t}Check if the GOsa support daemon (gosa-si) is running.{/t}&nbsp;
    <button type='submit' name='retry_init'>{t}Retry{/t}</button>
  <br>
  <br>

{else}

<table width="100%" summary="{t}Lincense settings{/t}">
  <tr> 
    <td>

        <!-- GENERIC -->
        <h3>{t}Generic{/t}</h3>
        <table summary="{t}Generic settings{/t}">
          <tr> 
            <td>{t}Name{/t}</td>
            <td>
              {if $initially_was_account}
                <input type='text' value='{$cn}' disabled>
              {else}
{render acl=$cnACL}
              <input type='text' value='{$cn}' name='cn'>
{/render}
              {/if}
            </td>
          </tr>
          <tr> 
            <td>{t}Description{/t}</td>
            <td>
{render acl=$descriptionACL}
              <input type='text' value='{$description}' name='description'>
{/render}
            </td>
          </tr>
        </table>

    </td>
    <td style='width:50%; padding: 5px;' class='left-border'>        <!-- LICENSES -->
      <h3>{t}Licenses{/t}</h3>
      {$licenses}
{render acl=$licensesACL}
              <button type='submit' name='addLicense'>{msgPool type=addButton}</button>

{/render}
    </td>
  </tr>
  <tr> 
    <td colspan="2">
      <hr>
    </td>
  </tr>
  <tr>
    <td style='width:50%'>
        <h3>{t}Applications{/t}</h3>
{render acl=$productIdsACL}
              <select name='productIds[]' multiple size="6" style="width:100%;">
                {html_options options=$productIds}
              </select><br>
{/render}
{render acl=$productIdsACL}
              <select name='availableProduct' size=1>
                {html_options options=$availableProductIds}
              </select>
{/render}
{render acl=$productIdsACL}
              <button type='submit' name='addProduct'>{msgPool type=addButton}</button>

{/render}
{render acl=$productIdsACL}
              <button type='submit' name='removeProduct'>{msgPool type=delButton}</button>

{/render}
    </td>
    <td style='padding: 5px;' class='left-border'>        <!-- SOFTWARE -->
        <h3>{t}Windows software IDs{/t}</h3>
{render acl=$windowsSoftwareIdsACL}
              <select name='softwareIds[]' multiple size="6" style="width:100%;">
                {html_options options=$softwareIds}
              </select>
{/render}
    </td>
  </tr>
</table>
<input name='opsiLicensePoolPosted' value='1' type='hidden'>
{/if}
