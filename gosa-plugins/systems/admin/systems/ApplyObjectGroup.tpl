<div style="font-size: 18px;">
{if !$is_incoming}
    {t}Apply template to{/t} {$SystemTypeName}
{/if}
</div>
<br>
<p class="seperator">
{if $is_incoming}
{t}The selected system(s) do not have an assigned type. Please choose a type and a template to be applied.{/t}
{else}
{t}This dialog gives you the possibility to choose a template, which will be applied to the selected target(s).{/t}
{/if}
<br>
<br>
</p>

<p class="seperator">
<br>
 <b>{t}Select object group{/t}</b>
<br>
<br>
</p>


<!-- Outer table -->
<table summary="" style='width:100%'>
    <tr>
{if $is_incoming}
        <td style='width:49%'>
        {include file={$devicetype_selection_tpl}}
        </td>
{/if}
        <td>
        <!-- Right side of the table -->
        {include file={$ogroup_selection_tpl}}
        </td>
    </tr>
</table>    

{if $reinstall_allowed && !$is_incoming}
<br>
<div align="left">
<input type="checkbox" name="trigger_reinstall" value="1">
{if $is_incoming}
{t}Trigger installation of the selected system(s){/t}
{else}
{t}Trigger reinstallation of the selected system(s){/t}
{/if}
</div>
{/if}


<hr>
{if !$is_incoming}
<input type="hidden"  name="SystemType" value="{$SystemType}">
{else}
<input type="hidden" name="trigger_reinstall" value="1">
{/if}

{if $is_incoming || $always_inherit}
<input type="hidden" name="inherit_attributes" value="1">
{/if}

<p style="text-align:right">
    <button type="submit" {if $apply_disabled}disabled{/if} name="ApplyOgroup">{t}Apply{/t}</button>&nbsp
    <button type="submit" name="edit_cancel">{msgPool type='cancelButton'}</button>
</p>

