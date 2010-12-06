<div id="mainlist">

  <div class="mainlist-header">
   <p>{$HEADLINE}&nbsp;{$SIZELIMIT}</p>
   <div class="mainlist-nav">
    <table summary="{$HEADLINE}">
     <tr>
      <td>{$RELOAD}</td>
      <td class="left-border">{t}Basis{/t}: {$RELEASE}</td>
      <td class="left-border">{$ACTIONS}</td>
      <td class="left-border">{$FILTER}</td>
     </tr>
    </table>
   </div>
  </div>

  {$LIST}
</div>

<div class="clear"></div>

{if $SHOW_BUTTONS}
<hr>
<div class='plugin-actions'>
    <button name='FolderWidget_ok'>{msgPool type=okButton}</button>
    <button name='FolderWidget_cancel'>{msgPool type=cancelButton}</button>
</div>
{/if}
