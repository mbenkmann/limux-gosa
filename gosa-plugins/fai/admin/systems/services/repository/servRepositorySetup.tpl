<h3>{t}Repository{/t}</h3>

<table width="100%" summary='{t}FAI Repository{/t}'>
	<tr>
		<td class='right-border'>

			<table summary='{t}Generic settings{/t}'>
				<tr>
					<td>{t}Parent server{/t}
					</td>
					<td>
{render acl=$ParentServerACL}
						<select name="ParentServer" size=1>
							{html_options options=$ParentServers values=$ParentServerKeys selected=$ParentServer} 
						</select>
{/render}
					</td>
				</tr>
				<tr>
					<td>{t}Release{/t}
					</td>
					<td>
{render acl=$ReleaseACL}
						<input type="text" value="{$Release}" name="Release">
{/render}
					</td>
				</tr>
				<tr>
					<td>{t}URL{/t}
					</td>
					<td>
{render acl=$UrlACL}
						<input type="text" size="40" value="{$Url}" name="Url">
{/render}
					</td>
				</tr>
			</table>
		</td>
		<td>
			{t}Sections{/t}<br>
{render acl=$SectionACL}
			{$Sections}
{/render}
{render acl=$SectionACL}
			<input type="text" 	name="SectionName" value="" style='width:100%;'>
{/render}
{render acl=$SectionACL}
			<button type='submit' name='AddSection'>{msgPool type=addButton}</button>

{/render}
		</td>
	</tr>
</table>
<input type='hidden' name='servRepositorySetup_Posted' value='1'>

<hr>
<div class="plugin-actions">
  <button type='submit' name='repository_setup_save'>{msgPool type=applyButton}</button>
  <button type='submit' name='repository_setup_cancel'>{msgPool type=cancelButton}</button>
</div>

