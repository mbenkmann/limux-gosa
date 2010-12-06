
{if $mode == "remove"}
    <b>{t}Please select the objects you want to remove:{/t}</b>
{elseif $mode == "edit"}
    <b>{t}Select the object you want to edit:{/t}</b>
{elseif $mode == "copy"}
    <b>{t}Select the object you want to copy:{/t}</b>
{/if}

<hr>

<table summary="{t}FAI group selection{/t}">
    {foreach from=$FAI_group item=item key=key}
        <tr>
            <td>
                {if $item.freezed}
                    {image path="images/lists/locked.png"}
                {else}

                    {if $mode == "remove" || $mode == "copy"}
                        <input id='{$mode}_selected_{$key}' type='checkbox' name='{$mode}_{$key}' {if $item.selected} checked {/if}>
                    {elseif $mode == "edit"}
                        <input id='{$mode}_selected_{$key}' type='radio' name='{$mode}_selected' 
                            value='{$key}' {if $item.selected} checked {/if}>
                    {/if}

                {/if}
            </td>
            <td>
                {image path="{$types.$key.IMG}" title="{$types.$key.NAME}"}
            </td>
            <td style='width:150px;'>
                <LABEL for='{$mode}_selected_{$key}'>
                    {$types.$key.NAME}
                </LABEL>    
            </td>
            <td style='width:80px;'>
                {if $item.freezed}
                    <LABEL for='{$mode}_selected_{$key}'>
                        <i>({t}Frozen{/t})</i>
                    </LABEL>    
                {/if}
            </td>
            <td>
                <LABEL for='{$mode}_selected_{$key}'>
                    <i>({$item.description.0})</i>
                </LABEL>    
            </td>
        </tr>
    {/foreach}
</table>


<br>
<input type='hidden' value='faiGroupHandle' name='faiGroupHandle'>
<hr>
<div class="plugin-actions">
 <button type='submit' name='faiGroupHandle_apply'>{msgPool type=applyButton}</button>&nbsp;
 <button type='submit' name='faiGroupHandle_cancel'>{msgPool type=cancelButton}</button>
</div>
