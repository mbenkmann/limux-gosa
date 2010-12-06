
{if $isNew}

<h3>{t}Create folder{/t}</h3>
<table>
    <tr>
        <td>{t}Name{/t}:&nbsp;</td>
        <td><input name='folderName' value="{$folderItem.name}" type="text"></td>
    </tr>
</table>

{else}

<h3>{t}Edit folder{/t}</h3>
<table>
    <tr>
        <td>{t}Name{/t}:&nbsp;</td>
        <td>{$folderName}</td>
    </tr>
    <tr>
        <td>{t}Path{/t}:&nbsp;</td>
        <td>{$folderPath}</td>
    </tr>
</table>
{/if}

<hr>

<h3>{t}Permissions{/t}</h3>

<table>
    <tr>
        <td style='width:100px;'>{t}Type{/t}</td>
        <td style='width:180px;'>{t}Name{/t}</td>
        <td style='width:180px;'>{t}Permission{/t}</td>
    </tr>
    {foreach from=$folderItem.acls item=item key=key}
        <tr>
            <td>{$item.type}</td>
            <td><input type='text' name="permission_{$key}_name" value="{$item.name}"></td>
            <td>
                {if $permissionCnt == 0 || !isset($permissions[$item.acl])}
                    <input type='text' name="permission_{$key}_acl" value="{$item.acl}">
                {else}
                    <select name="permission_{$key}_acl" size=1>
                        {html_options options=$permissions selected=$item.acl}
                    </select>
                {/if}
            </td>
            <td><button name="permission_{$key}_del">{msgPool type=delButton}</button></td>
        </tr>
    {/foreach}
    <tr>
        <td></td>
        <td></td>
        <td></td>
        <td><button name="permission_add">{msgPool type=addButton}</button></td>
    </tr>
</table>

<hr>
<div class='plugin-actions'>
    <button name="FolderEditDialog_ok">{msgPool type='okButton'}</button>
    <button name="FolderEditDialog_cancel">{msgPool type='cancelButton'}</button>
</div>
