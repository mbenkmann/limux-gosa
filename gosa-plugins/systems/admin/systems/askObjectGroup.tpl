<div style="font-size: 18px;">
	{$headline}
</div>
<br>
<p class="seperator">
{t}This dialog gives you the possibility to select an optional bundle of predefined settings to be inherited.{/t}
<br>
<br>
</p>

<table summary="" style='width:100%'>
 <tr>
  <td style='width:49%'>
   <table summary="">
   <tr>
      <td>
      <b>{t}Choose an object group as template{/t}&nbsp;</b>
      <select name="SelectedOgroup" title="{t}Select object group{/t}" style="width:220px;">
      {html_options options=$ogroups}
     </td>
    </tr>
  </table>
  <br>
  <hr>
  <p style="text-align:right">
  <button type="submit" name="edit_continue">{t}Continue{/t}</button>&nbsp
  <button type="submit" name="edit_cancel">{msgPool type='cancelButton'}</button>
  </p>

