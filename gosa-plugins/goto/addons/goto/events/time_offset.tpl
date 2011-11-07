<p>&nbsp;</p>
<b>{t}Extended settings for delayed operations{/t}</b>
<table cellspacing="0" cellpadding="0">
    <tr>
        <td>&nbsp;</td>
        <td>&nbsp;</td>
    </tr>
	<tr>
		<td style="width: 175px;">
            {t}Time offset in minutes{/t}
        </td>
        <td>
            <select name="time_offset">
            {html_options options=$offset_minutes values=$offset_minutes selected=$time_offset}
            </select>
        </td>
    </tr>
    <tr>
        <td>
            {t}Concurrent operations{/t}
        </td>
        <td>
            <select name="concurrent_operations">
            {html_options options=$offset_operations values=$offset_operations selected=$concurrent_operations}
            </select>
        </td>
    </tr>
</table>

