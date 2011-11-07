<div style="font-size: 18px;">
{t}New debconf configuration{/t}
</div>
<br>
<p class="seperator">
{t}In this dialog you can add a new debconf configuration.{/t}
<br>
<br>
<br>
<table summary="" style='width:100%'>
 <tr>
  <td style='width:49%'>
   <table summary="">
    <tr>
     <td> 
        <b>{t}Package{/t}</b>
     </td>
     <td>
        <b>{t}Variable{/t}</b>
     </td>
     <td>
        <b>{t}Type{/t}</b>
     </td>
     <td>
        <b>{t}Value{/t}</b>
     </td>
   </tr>
   <tr>
      <td style="width:180px";>
        <select name="Package" title="{t}Select package{/t}" style="width:150px;">
        {html_options options=$packages selected=$package}
        </select>
      </td>
      <td style="width: 300px;">
        <input type="text" style="width:280px" name="FAIvariable" value="{$variable}">
      </td>
      <td style="width: 120px;">
        <select name="FAIvariableType" title="{t}Select type{/t}" style="width: 100px;">
        {html_options options=$variable_types selected=$variable_type}
        </select>
      </td>
      <td style="width:300px;">
        <input type="text" name="FAIvariableContent" title="{t}Value{/t}" value="{$content}" style="width:280px;">
      </td>
   </tr>
   </table>
   <br>
   <br>
  </td>
 </tr>
</table>
</table>
<br>
<div align="right">
    <input type="submit" name="save_AddDebconf" value="{msgPool type=applyButton}">&nbsp;
    <input type="submit" name="cancel_AddDeconf" value="{msgPool type=cancelButton}">
</div>
<!-- Place cursor -->
<script language="JavaScript" type="text/javascript">
	<!--
	focus_field('SelectedPackage');
	-->
</script>
