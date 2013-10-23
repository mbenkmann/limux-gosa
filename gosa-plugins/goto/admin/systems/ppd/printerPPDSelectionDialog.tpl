<h3>{t}Select objects to add{/t}</h3>
<select size="1" name="listVendor" style="width: 160px">
       {if $mode eq "vnd"}
           <option value=".." selected>{t}Generic{/t}</option>
       {/if}
       {if isset($ListVendor[".."]) eq ".."}
           <option value=".." >{t}Generic{/t}</option>
       {else}
           {html_options options=$ListVendor values=$ListVendor}
       {/if}
      </select>
 <input type="text" name="txt_ppdSearch" value="{$ppdFilter}" style="width: 200px"/>
 <button type="submit" name="btn_ppdSearch">{t}Search{/t}</button>
 <button type="submit" name="btn_reset">{t}Reset{/t}</button>

{$List}
{literal}
<script type="text/javascript" name="javascript">
/* <!-- */
$$(".sortableListItem").each(function(e){
       e.firstDescendant().setStyle({cursor:"pointer"});
} );
/* --> */
</script>
{/literal}
<div class="plugin-actions">
 <button type='submit' name='ClosePPDSelection'>{t}Cancel{/t}</button>
</div>
