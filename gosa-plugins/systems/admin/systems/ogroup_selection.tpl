<table summary="{t}Object group selection{/t}">
    <tr>
        <td>
            {t}Choose an object group{/t}&nbsp;
            <select name="ObjectGroup" title="{t}Select object group{/t}" size="1" style="width: 250px;"
                onChange="document.mainform.submit();">
                <option value='none'>{t}none{/t}</option>
                {html_options values=$OgroupKeys output=$ogroups selected=$ObjectGroup}
            </select>

        </td>
    </tr>
    {if !$always_inherit and !$is_incoming}
    <tr>
        <td>
            <input type="checkbox" name="inherit_attributes" value="1">{t}Inherit attributes{/t}
        </td>
    </tr>
    {/if}
</table>
