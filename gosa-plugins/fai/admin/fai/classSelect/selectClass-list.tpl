
<div id="mainlist">
 <div class="mainlist-header">
  <p>
   {$HEADLINE}&nbsp;
   {$SIZELIMIT}
  </p>
  <div class="mainlist-nav">
   
   <table summary="{$HEADLINE}">
    <tr>
     <td>
      {$RELOAD}
     </td>
     <td class="left-border">
      {$FILTER}
     </td>
    </tr>
   </table>
  </div>
 </div>
 {$LIST}
</div>
<div class="clear">
</div>
<div class="plugin-actions">
 <button type=submit name="classSelect_save">{msgPool type=okButton}</button>
 <button type=submit name="classSelect_cancel">{msgPool type=cancelButton}</button>
</div>
