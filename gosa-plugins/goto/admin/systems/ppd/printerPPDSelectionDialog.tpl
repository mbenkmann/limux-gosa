<h3>{t}Select objects to add{/t}</h3>
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
<hr>
<div class="plugin-actions">
 <input type="text" name="ppdSearch" title="{t}PPD search field{/t}" value="{$ppdFilter}" />
 <button type="submit" name="SearchPPD">{if $mode eq "ppd"}{t}Search for printer model{/t}{else}{t}Search for printer manufacturer{/t}{/if}</button>
 <button type='submit' name='ClosePPDSelection'>{t}Close{/t}</button>
</div>
