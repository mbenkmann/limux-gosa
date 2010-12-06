<h3>{t}Apache virtual hosts{/t}</h3>
<table summary="" width="100%">
<tr>
	<td style='width:100%;'>
		{$VhostList}
		{render acl=$VirtualHostsACL}
		  <button type='submit' name='AddVhost'>{t}Add{/t}</button>
		{/render}
	</td>
</tr>
</table>

<script language="JavaScript" type="text/javascript">
  <!-- // First input field on page
  document.mainform.AddVhost.focus();
  -->
</script>

<input type="hidden" name="servapache" value="1">
<hr>
<div class="plugin-actions">
  <button type='submit' name='SaveService'>{msgPool type=saveButton}</button>
  <button type='submit' name='CancelService'>{msgPool type=cancelButton}</button>
</div>

