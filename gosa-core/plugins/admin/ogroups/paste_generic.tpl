<table width='100%' summary="{t}Paste object group{/t}">
	<tr>
		<td width='120'>
			<LABEL for="cn">{t}Group name{/t}</LABEL>{$must}
		</td>
		<td>
			<input type='text' id='cn' name='cn' value='{$cn}' size='40' title='{t}Please enter the new object group name{/t}'> 
		</td>
	</tr>
</table>
<br>
{if $copyMembers}
<input class="center" type="checkbox" value="1" checked name="copyMembers" id="copyMembers">{t}Inherit Members{/t}<br>
<br>
{t}Note that settings are copied even if you choose to not inherit members. Those settings will be lost in case different type members get added to the group and will only appear once a new member of the current type is added.{/t}
{else}
{t}Note that only settings are copied and the new object group will have no members initially. Those settings will be lost in case different type members get added to the group and will only appear once a new member of the current type is added.{/t}
{/if}

<script language="JavaScript" type="text/javascript">
	focus_field('cn');
</script>
