<div style="font-size: 18px;">
	{$headline}
</div>
<br>
<p class="seperator">
{t}This dialog gives you the possibility to select an optional bundle of predefined settings to be inherited.{/t}
<br>
<br>
</p>

<p class="seperator">
<br>
 <b>{t}Select object group{/t}
<br>
<br>
</p>
<table summary="" style='width:100%'>
 <tr>
  <td style='width:49%'>
   <table summary="">
   <tr>
      <td>
      {t}Choose an object group as template{/t}&nbsp;
      <select name="SelectedOgroup" title="{t}Select object group{/t}" style="width:120px;">
      {html_options options=$ogroups}
     </td>
    </tr>
  </table>
  <hr>
  <p style="text-align:right">
  <button type="submit" name="edit_continue">{t}Continue{/t}</button>&nbsp
  <button type="submit" name="edit_cancel">{msgPool type='cancelButton'}</button>
  </p>

