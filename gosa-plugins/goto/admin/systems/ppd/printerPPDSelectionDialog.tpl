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
 <button type='submit' name='ClosePPDSelection'>{t}Close{/t}</button>
</div>
