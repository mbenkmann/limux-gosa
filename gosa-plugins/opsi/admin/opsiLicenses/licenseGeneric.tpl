{if !$init_successfull}

  <br>
    <b>{msgPool type=siError}</b><br>
    {t}Check if the GOsa support daemon (gosa-si) is running.{/t}&nbsp;
    <button type='submit' name='retry_init'>{t}Retry{/t}</button>
  <br>
  <hr>
{else}

<h3>{t}License{/t}</h3>

<table style='width:100%' summary="{t}License settings{/t}">
  <tr>
    <td style='width:50%;'  class='right-border'>        


        <table  summary="{t}Generic settings{/t}">
          <tr>
            <td>
              {t}Name{/t}{$must}
            </td>
            <td>
              {if $initially_was_account}
                <input type='text' name='dummy12' disabled value='{$cn}'>
              {else}
{render acl=$cnACL}
                <input type='text' name='cn' value='{$cn}'>
{/render}
              {/if}
            </td>
          </tr>
          <tr>
            <td>
              {t}Partner{/t}
            </td>
            <td>
{render acl=$partnerACL}
              <input type='text' name='partner' value='{$partner}'>
{/render}
            </td>
          </tr>
          <tr>
            <td>
              {t}Description{/t}
            </td>
            <td>
{render acl=$descriptionACL}
              <input type='text' name='description' value='{$description}'>
{/render}
            </td>
          </tr>
          <tr>
            <td>
              {t}License model{/t}
            </td>
            <td>
{render acl=$licenseModelACL}
              {if $initially_was_account}
                <select name='dummy223' disabled size=1>
                  {html_options options=$licenseModels values=$licenseModels selected=$licenseModel}
                </select>
              {else}
                <select name='licenseModel' onChange='document.mainform.submit();' size=1>
                  {html_options options=$licenseModels values=$licenseModels selected=$licenseModel}
                </select>
              {/if}
{/render}
            </td>
          </tr>
        </table>

    </td>
    <td>
        <table  summary="{t}Additional settings{/t}">
          <tr>
            <td>
              {t}Conclusion date{/t}
            </td>
            <td style='width:300px;'>
{render acl=$conclusionDateACL}
              <input type="text" id="date1" name="conclusionDate" value="{$conclusionDate}" class="date" />
              {if $conclusionDateWriteable}
              {literal}
              <script type="text/javascript">
                  /*<[CDATA[*/
                  var datepicker    = new DatePicker({
                  keepFieldEmpty : true,
                  enableCloseEffect : false,
                  enableShowEffect : false,
                  relative    : 'date1',
                  language    : '{/literal}{$lang}{literal}'
                  });
                  /*]]>*/
              </script>
              {/literal}
              {/if}
{/render}
            </td>
          </tr>
          <tr>
            <td>
              {t}Expiration date{/t}
            </td>
            <td style='width:300px;'>
{render acl=$expirationDateACL}
              <input type="text" id="date2" name="expirationDate" value="{$expirationDate}" class="date" />
              {if $expirationDateWriteable}
              {literal}
              <script type="text/javascript">
                  /*<[CDATA[*/
                  var datepicker    = new DatePicker({
                  keepFieldEmpty : true,
                  enableCloseEffect : false,
                  enableShowEffect : false,
                  relative    : 'date2',
                  language    : '{/literal}{$lang}{literal}'
                  });
                  /*]]>*/
              </script>
              {/literal}
              {/if}
{/render}
            </td>
          </tr>
          <tr>
            <td>
              {t}Notification date{/t}
            </td>
            <td style='width:300px;'>
{render acl=$notificationDateACL}
              <input type="text" id="date3" name="notificationDate" value="{$notificationDate}" class="date" >
              {if $notificationDateWriteable}
              {literal}
              <script type="text/javascript">
                  /*<[CDATA[*/
                  var datepicker    = new DatePicker({
                  keepFieldEmpty : true,
                  enableCloseEffect : false,
                  enableShowEffect : false,
                  relative    : 'date3',
                  language    : '{/literal}{$lang}{literal}'
                  });
                  /*]]>*/
              </script>
              {/literal}
              {/if}
{/render}
            </td>
          </tr>
        </table>
 
    </td> 
  </tr>
</table>

<hr>
<h3>{t}License key settings{/t}</h3>

<table width="100%" summary="{t}License key settings{/t}">
  <tr>
    <td style='width:50%;' class='right-border'>
        <table summary="{t}License key settings{/t}">
          <tr>
            <td>
              {t}License key{/t}{$must}
            </td>
            <td>
{render acl=$licenseKeyACL}
              <input type='text' name='licenseKey' value='{$licenseKey}'>
{/render}
            </td>
          </tr>
          {if $licenseModel == "VOLUME"}
          <tr>
            <td>
              {t}Maximum installations{/t}
            </td>
            <td>
{render acl=$maximumInstallationsACL}
              <input type='text' name='maximumInstallations' value='{$maximumInstallations}'>
{/render}
            </td>
          </tr>
          {/if}
          {if $licenseModel == "OEM"}
          <tr>
            <td>
              {t}Reserved for{/t}
            </td>
            <td>
{render acl=$boundToHostACL}
              <select name='boundToHost' size=1>
                <option value="">{t}none{/t}</option>
                {html_options options=$hosts selected=$boundToHost}
              </select>
{/render}
            </td>
          </tr>
          {/if}
        </table>
 
    </td> 
    <td class='right-border'>   
              <b>{t}Used by{/t}</b><br>
{render acl=$usedByHostACL}
              <select   disabled
                name='selectedUsedHosts[]' multiple size=4 style='width:100%;'>
                {html_options options=$usedByHost}
              </select><br>
{/render}
    </td> 
  </tr>
</table>
<input name='opsiLicensesPosted' value='1' type='hidden'>
{/if}
