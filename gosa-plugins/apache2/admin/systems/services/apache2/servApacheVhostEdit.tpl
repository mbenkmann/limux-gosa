<h3>{t}Generic{/t}</h3>

			<table summary="{t}Apache Virutal Host{/t}">
				<tr>
					<td>{t}Virtual host name{/t}{$must}
					</td>
					<td><input type="text" name="apacheServerName" value="{$apacheServerName}" {if $NotNew} disabled {/if}>
					</td>
				</tr>
				<tr>
					<td>{t}Document root{/t}{$must}
					</td>
					<td><input type="text" name="apacheDocumentRoot" value="{$apacheDocumentRoot}">
					</td>
				</tr>
				<tr>
					<td>{t}Administrator mail address{/t}{$must}
					</td>
					<td><input type="text" name="apacheServerAdmin" value="{$apacheServerAdmin}">
					</td>
				</tr>
			</table>

<hr>

<table summary="{t}Server settings{/t}" width="100%">
  <tr>
    <td style='width:50%;' class='right-border'>

  		<h3>{t}Server Alias{/t}</h3>

 			{$apacheServerAlias}
 			<table width="100%" summary="{t}Server Alias{/t}">
 				<tr>
 					<td style='width:30%;'>

 						<h3>{t}URL Alias{/t}</h3>
 					</td>
 					<td>
 						<h3>{t}Directory Path{/t}</h3>
 					</td>
 				</tr>
 				<tr>
 					<td style='width:30%;'>

 						<input type="text" 		name="StrSAAlias" value="">
 					</td>
 					<td>
 						<input type="text" 		name="StrSADir" value="">
 						<button type='submit' name='AddSARecord'>{t}Add{/t}</button>

 					</td>
 				</tr>
   		</table>

		</td>
		<td style='width:50%;' class='right-border'>

			<h3>{t}Script Alias{/t}</h3>
  		{$apacheScriptAlias}

      <table width="100%" summary="{t}Script Alias{/t}">
        <tr>
          <td style='width:30%;'>
            <h3>{t}Alias Directory{/t}</h3>
          </td>
          <td>
            <h3>{t}Script Directory{/t}</h3>
          </td>
        </tr>
        <tr>
          <td style='width:30%;'>

            <input type="text" 		name="StrSCAlias" value="">
          </td>
          <td>
            <input type="text" 		name="StrSCDir" value="">
            <button type='submit' name='AddSCRecord'>{t}Add{/t}</button>

          </td>
        </tr>
      </table>

		</td>
	</tr>
</table>
<div style="text-align:right;" align="right">
	<p>
		<button type='submit' name='SaveVhostChanges'>{t}Save{/t}</button>

		<button type='submit' name='CancelVhostChanges'>{t}Cancel{/t}</button>

	</p>
</div>
<script language="JavaScript" type="text/javascript">
  <!-- // First input field on page
  document.mainform.apacheServerName.focus();
  -->
</script>
