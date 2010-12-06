
<table summary="{t}Groupware shared folder{/t}" width="100%">
    <tr>
        <td style='width:50%; vertical-align: top;'>
            <h3>{t}Groupware shared folder{/t}</h3>

            {render acl=$folderListACL}
                {t}Edit folder list{/t}&nbsp;<button name='configureFolder'>{msgPool type=editButton}</button>
            {/render}
        </td>
        <td style='width:50%; vertical-align: top; padding-left:5px;'>
        </td>
    </tr>
</table>

<input type="hidden" name="GroupwareSharedFolder_posted" value="1">
