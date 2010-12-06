
<h3>{t}Installation type{/t}</h3>
<table>
    <tr>
        <td><LABEL for="installBootstrapMethod">{t}Bootstrap method{/t}</LABEL></td>
        <td>
            <select name="installBootstrapMethod" size=1>
                {html_options options=$installBootstrapMethodList 
                selected=$installBootstrapMethod}
            </select>
        </td>
    </tr>
    <tr>
        <td><LABEL for="installConfigManagement">{t}Config management{/t}</LABEL></td>
        <td>
            <select name="installConfigManagement" size=1
                onChange='document.mainform.submit();'>
                {html_options options=$installConfigManagementList 
                selected=$installConfigManagement}
            </select>
        </td>
    </tr>
</table>
<hr>
<table style='width:100%;'>
    <tr>
        <td style='width:50%; vertical-align: top;'>
            <h3>{t}Bootstrap settings{/t}</h3>
    
            <table>
                <tr>
                    <td><LABEL for="installMirrorDN">{t}Mirror{/t}</LABEL></td>
                    <td>
                        <select name="installMirrorDN" size=1>
                            {html_options options=$installMirrorDNList 
                            selected=$installMirrorDN}
                        </select>
                    </td>
                </tr>
                <tr>
                    <td><LABEL for="installTemplateDN">{t}Kickstart template{/t}</LABEL></td>
                    <td>
                        <select name="installTemplateDN" size=1>
                            {html_options options=$installTemplateDNList 
                            selected=$installTemplateDN}
                        </select>
                    </td>
                </tr>
                <tr>
                    <td><LABEL for="installKernelPackage">{t}Kernel package{/t}</LABEL></td>
                    <td>
                        <select name="installKernelPackage" size=1>
                            {html_options values=$installKernelPackageList output=$installKernelPackageList 
                            selected=$installKernelPackage}
                        </select>
                    </td>
                </tr>
            </table>
        </td>
        <td style='width:50%; vertical-align: top;padding-left:5px;' class='left-border'>
            <h3>{t}Login{/t}</h3>

            <table>
                <tr>
                    <td>
                        <input type='checkbox' value='1' {if $installRootEnabled} checked {/if}
                            onClick="changeState('setKickstartRootPasswordHash');"
                            name="installRootEnabled" id="installRootEnabled" 
                           >
                        <LABEL for="installRootEnabled">{t}Use root user{/t}</LABEL>&nbsp;
                        <button name='setKickstartRootPasswordHash'
                            {if !$installRootEnabled} disabled {/if}
                            id="setKickstartRootPasswordHash">{t}Set password{/t}</button>
                    </td>
                </tr>
            </table>
        </td>
    </tr>
    <tr>
        <td colspan=2><hr></td>
    </tr>
    <tr>
        <td style='width:50%; vertical-align: top;'>
            <h3>{t}Locale{/t}</h3>

            <table>
                <tr>
                    <td><LABEL for="installKeyboardlayout">{t}Keyboard layout{/t}</LABEL></td>
                    <td>
                        <select name="installKeyboardlayout" size=1>
                            {html_options values=$installKeyboardlayoutList output=$installKeyboardlayoutList 
                            selected=$installKeyboardlayout}
                        </select>
                    </td>
                </tr>
                <tr>
                    <td><LABEL for="installSystemLocale">{t}System locale{/t}</LABEL></td>
                    <td>
                        <select name="installSystemLocale" size=1>
                            {html_options values=$installSystemLocaleList output=$installSystemLocaleList 
                            selected=$installSystemLocale}
                        </select>
                    </td>
                </tr>
            </table>
        </td>
        <td style='width:50%; vertical-align: top;padding-left:5px;' class='left-border'>
            <h3>{t}Time{/t}</h3>

            <table>
                <tr>
                    <td>
                        <input type='checkbox' name="installTimeUTC" id="installTimeUTC" 
                        {if $installTimeUTC} checked {/if}>
                        <LABEL for="installTimeUTC">{t}Use UTC{/t}</LABEL>
                    </td>
                </tr>
                <tr>
                    <td><LABEL for="installTimezone">{t}Timezone{/t}</LABEL></td>
                    <td>
                        <select size='1' name="installTimezone" id="installTimezone">
                            {html_options values=$timezones options=$timezones selected=$installTimezone}
                        </select>
                    </td>
                </tr>
                <tr>    
                    <td colspan=2>
                        {t}NTP server{/t}
                        {$installNTPServerList}
                        <input type='text' name="installNTPServer_Input">
                        <button name='installNTPServer_Add'>{msgPool type=addButton}</button>
                    </td>
                </tr>
            </table>
        </td>
    </tr>
</table>

<hr>
<h3>{t}Partition table{/t}</h3>
<table>
    <tr>
        <td><LABEL for="installPartitionTable">{t}Partition table{/t}</LABEL></td>
        <td>
            <input type='text' name="installPartitionTable" id="installPartitionTable" value="{$installPartitionTable}">
        </td>
    </tr>
</table>

<input type='hidden' name='InstallRecipePosted' value=1>
