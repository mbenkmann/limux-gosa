<h3>{t}Copy & paste wizard{/t}</h3>

<b>{$message}</b>
<br>
<br>
{if $Complete == false}
	{t}Some values need to be unique in the complete directory while some combinations make no sense. Please edit the values below to fulfill the policies.{/t}
	<br>
{t}Remember that some properties like taken snapshots will not be copied!{/t}&nbsp;
{t}Or if you copy or cut an entry within GOsa and delete the source object, you may get errors while pasting this object again!{/t}

	<hr>
	<br>
	{$AttributesToFix}
	{if $SubDialog == false}
	<br>

	<div style='text-align:right;width:100%;'>
		<button type='submit' name='PerformCopyPaste'>{t}Save{/t}</button>
	    {if $type == "modified"}
		    <button type='submit' name='abort_current_cut-copy_operation'>{t}Cancel{/t}</button>
    	{/if}
		<button type='submit' name='abort_all_cut-copy_operations'>{t}Cancel all{/t}</button>
	</div>
{if $EnableCSV == true}
<hr>
<h3>{t}CSV Import{/t}</h3>
	{t}The CSV Import feature allows you to create multiple copies of the above object with one or more attributes replaced with data from a CSV file. To use this feature, create a CSV file like the following:{/t}
<pre>
{$attributes}
</pre>
<div>
{t}The first line lists all attributes you want to change. The above example contains all possible attributes. Do not list all of them in your file. Only list those you actually want to change.{/t}
</div>
<div>
{t}The remaining lines of the CSV file contain the override values for the listed attributes. Each line will result in a new copy of the object with the override values used instead of the original values.{/t}
</div>
<hr>
	<div style='text-align:right;width:100%;'>
		<input id="csv_file" name="csv_file" type="file" size="20" maxlength="255" accept="text/csv">
		<button type='submit' name='CSVImport'>{t}CSV Import{/t}</button>
	</div>
{/if}
	{/if}
{else}
	<hr>
	<h3>{t}Operation complete{/t}</h3>
	<div style='text-align:right;width:100%;'>
		<button type='submit' name='Back'>{t}Finish{/t}</button>
	</div>
{/if}
