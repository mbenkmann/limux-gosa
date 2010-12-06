

<table width="100%">
    <tr>
        <td style='width:50%;'>

            <h3>{t}Device{/t}</h3>
            <table>
                <tr>
                    <td><LABEL for='name'>{t}Name{/t}</LABEL></td>
                    <td>
                        {render acl=$cnACL}
                            <input type="text" name="cn" value="{$cn}" id="cn" value="{$cn}">
                        {/render}
                    </td>
                </tr>
                <tr>
                    <td><LABEL for='description'>{t}Description{/t}</LABEL></td>
                    <td>
                        {render acl=$descriptionACL}
                            <input type="text" name="description" value="{$description}" id="description" value="{$description}">
                        {/render}
                    </td>
                </tr>
                <tr>
                    <td>
                        <div style="height:10px;"> </div>
                        {t}Base{/t}
                    </td>
                    <td>
                        <div style="height:10px;"> </div>
                        {render acl=$baseACL}
                            {$base}
                        {/render}
                    </td>
                </tr>
            </table>

            <hr>
            <h3>{t}Registration{/t}</h3>
            <table>
                <tr>
                    <td><LABEL for='deviceType'>{t}Type{/t}</LABEL>
                    </td>
                    <td>
                        {render acl=$deviceTypeACL}
                            <input type="text" name="deviceType" value="{$deviceType}" id="deviceType" value="{$deviceType}">
                        {/render}
                    </td>
                </tr>
                <tr>
                    <td><LABEL for='deviceUUID'>{t}Device UUID{/t}</LABEL></td>
                    <td> 
                        {render acl=$deviceUUIDACL}
                            <input type="text" name="deviceUUID" value="{$deviceUUID}" id="deviceUUID" value="{$deviceUUID}">
                        {/render}
                        {render acl=$deviceUUIDACL}
                            {image path="images/lists/reload.png" action="reloadUUID"}
                        {/render}
                    </td>
                </tr>
                <tr>
                    <td><LABEL for='deviceStatus'>{t}Status{/t}</LABEL>
                    </td>
                    <td>
                        {render acl=$deviceStatusACL}
                            <input type="text" name="deviceStatus" value="{$deviceStatus}" id="deviceStatus" value="{$deviceStatus}">
                        {/render}
                    </td>
                </tr>
            </table>
        </td>
        <td class='left-border' style='padding-left:5px;' rowspan=2>

            <h3>{t}Orgaizational data{/t}</h3>
            <table>
                <tr>
                    <td><LABEL for='ou'>{t}Organizational Unit{/t}</LABEL></td>
                    <td>
                        {render acl=$ouACL}
                            <input type="text" name="ou" value="{$ou}" id="ou" value="{$ou}">
                        {/render}
                    </td>
                </tr>
                <tr>
                    <td><LABEL for='o'>{t}Organization{/t}</LABEL></td>
                    <td>
                        {render acl=$oACL}
                            <input type="text" name="o" value="{$o}" id="o" value="{$o}">
                        {/render}
                    </td>
                </tr>
                <tr>
                    <td><LABEL for='l'>{t}Location{/t}</LABEL></td>
                    <td>
                        {render acl=$lACL}
                            <input type="text" name="l" value="{$l}" id="l" value="{$l}">
                        {/render}
                    </td>
                </tr>
                <tr>
                    <td><LABEL for='serialNumber'>{t}Serial number{/t}</LABEL></td>
                    <td>
                        {render acl=$serialNumberACL}
                            <input type="text" name="serialNumber" value="{$serialNumber}" id="serialNumber" value="{$serialNumber}">
                        {/render}
                    </td>
                </tr>
<!--
                <tr>
                    <td><LABEL for='seeAlso'>{t}See also{/t}</LABEL></td>
                    <td>
                        {render acl=$seeAlsoACL}
                            <input type="text" name="seeAlso" value="{$seeAlso}" id="seeAlso" value="{$seeAlso}">
                        {/render}
                    </td>
                </tr>
-->
<!--
                <tr>
                    <td><LABEL for='owner'>{t}Owner{/t}</LABEL></td>
                    <td>
                        {render acl=$ownerACL}
                            <input type="text" name="owner" value="{$owner_name}" id="owner" value="{$owner_name}" 
                            title="{$owner}" disabled style="width:120px;">
                        {/render}

                        {image path="images/lists/edit.png" action="editOwner" acl=$ownerACL}
                        {if $owner!=""}
                            {image path="images/info_small.png" title="{$owner}" acl=$ownerACL}
                            {image path="images/lists/trash.png" action="removeOwner" acl=$ownerACL}
                        {/if}
                 </td>
                </tr>
-->
                <tr>
                    <td><LABEL for='manager'>{t}Manager{/t}</LABEL>
                    </td>
                    <td>
                        {render acl=$managerACL}
                            <input type="text" name="manager" value="{$manager_name}" id="manager" value="{$manager_name}" 
                            title="{$manager}" disabled style="width:120px;">
                        {/render}

                        {image path="images/lists/edit.png" action="editManager" acl=$managerACL}
                        {if $manager!=""}
                            {image path="images/info_small.png" title="{$manager}" acl=$managerACL}
                            {image path="images/lists/trash.png" action="removeManager" acl=$managerACL}
                        {/if}
                 </td>
                </tr>
            </table>
        </td>
    </tr>
</table>
            <hr>
            <h3>{t}Network settings{/t}</h3>
            <table>
                <tr>
                    <td><LABEL for='ipHostNumber'>{t}IP address{/t}</LABEL>
                    </td>
                    <td>
                        {render acl=$ipHostNumberACL}
                            <input type="text" name="ipHostNumber" value="{$ipHostNumber}" id="ipHostNumber" value="{$ipHostNumber}">
                        {/render}
                    </td>
                </tr>
                <tr>
                    <td><LABEL for='macAddress'>{t}MAC address{/t}</LABEL>
                    </td>
                    <td>
                        {render acl=$macAddressACL}
                            <input type="text" name="macAddress" value="{$macAddress}" id="macAddress" value="{$macAddress}">
                        {/render}
                    </td>
                </tr>
            </table>
