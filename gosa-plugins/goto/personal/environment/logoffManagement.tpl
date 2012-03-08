

    <input type="hidden" name="dialogissubmitted" value="1">

    <table summary="{t}Log off script management{/t}" width="100%">
    	<tr>
			<td class='right-border'>

					<table summary="{t}Log off script settings{/t}">
						<tr>
							<td><LABEL for="LogoffName">{t}Script name{/t}</LABEL>
							</td>
							<td>
								<input type="text" size=20 value="{$LogoffName}" name="LogoffName" {$LogoffNameACL} id="LogoffName">
							</td>
						</tr>
						<tr>
							<td><LABEL for="LogoffDescription">{t}Description{/t}</LABEL>
							</td>
							<td>
								<input type="text" size=40 value="{$LogoffDescription}" name="LogoffDescription" id="LogoffDescription"> 
							</td>
						</tr>
						<tr>
							<td><LABEL for="LogoffPriority">{t}Priority{/t}</LABEL>
							</td><td>
				            	<select name="LogoffPriority" id="LogoffPriority" size=1>
                					{html_options values=$LogoffPriorityKeys output=$LogoffPrioritys selected=$LogoffPriority}
                				</select>
							</td>
						</tr>
					</table>
			</td>
			<td>

					<table summary="{t}Log off script flags{/t}">
						<tr>
							<td>
								<input type="checkbox" value="L" name="LogoffLast" {$LogoffLastCHK} id="LogoffLast">
							<LABEL for="LogoffLast">{t}Last script{/t}</LABEL>
							</td>
						</tr>
						<tr>
							<td>
								<input type="checkbox" value="O" name="LogoffOverload" {$LogoffOverloadCHK} id="LogoffOverload">
								<LABEL for="LogoffOverload">{t}Script can be replaced by user{/t}</LABEL>
							</td>
						</tr>
					</table>
			</td>
		</tr>
	</table>
	
  <hr>

	<h3>{t}Script{/t}</h3>
	<table width="100%" summary="{t}Log off script{/t}">
		<tr>
			<td>
				<textarea style="width:100%;height:400px;" name="LogoffData">{$LogoffData}</textarea>
			</td>
		</tr>
		<tr>
			<td>
				<input type="file" name="importFile" id="importFile">
				<button type='submit' name='StartImport'>{t}Import script{/t}</button>
				{$DownMe}
			</td>
		</tr>
	</table>

  <hr>
  <div class="plugin-actions">
    <button type='submit' name='LogoffSave'>{msgPool type=applyButton}</button>
    <button type='submit' name='LogoffCancel'>{msgPool type=cancelButton}</button>
  </div>

<script language="JavaScript" type="text/javascript">
  <!-- // First input field on page
	focus_field('LogoffName');
  -->
</script>

