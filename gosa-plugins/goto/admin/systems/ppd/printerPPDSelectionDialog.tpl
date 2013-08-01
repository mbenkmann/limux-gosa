<h3>{t}Select objects to add{/t}</h3>
 <input type="text" name="txt_ppdSearch" title="{t}PPD search field{/t}" value="{$ppdFilter}" />
 <button type="submit" name="btn_ppdSearch">{t}Search{/t}</button>

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
 <button type='submit' name='ClosePPDSelection'>{t}Close{/t}</button>
</div>
