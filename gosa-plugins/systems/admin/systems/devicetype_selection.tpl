<table summary="{t}Target system type selection{/t}">
<tr>
    <td>
        {t}System type{/t}&nbsp;
        <select name="SystemType" title="{t}System type{/t}" style="width:120px;"
            onChange="document.mainform.submit();">
        {html_options values=$SystemTypeKeys output=$SystemTypes selected=$SystemType}
        </select>
    </td>
</tr>
</table>
