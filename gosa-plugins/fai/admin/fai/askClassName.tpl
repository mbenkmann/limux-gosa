
<div style="font-size: 18px;">
 {$headline}
</div>
<br>
<p class="seperator">{t}Adding a new class to the FAI management, requires a class name. You have to specify a unique class name for unique types of FAI classes, while you can use the same class name for different types of FAI classes. In the last case, FAI will automatically enclose all these different class types to one unique class name.{/t}
 <br>
 <br>
</p>
<p class="seperator">
 <br>
 <b>{t}Please use one of the following methods to choose the name for the new FAI class.{/t}</b>
 <br>
 <br>
</p>
<br>
<table summary="{t}FAI class creator{/t}" style='width:100%' >
 <tr>
  <td style='width:49%'>
   <table summary="{t}Class settings{/t}">
    <tr>
     <td>
      <input type=radio name="classSelector" id="classSelector1" value="1" checked>
     </td>
     <td><h3><label for='classSelector1'>{t}Enter FAI class name manually{/t}</label></h3>
     </td>
    </tr>
    <tr>
     <td>&nbsp;
     </td>
     <td>{t}Class name{/t}&nbsp;
      <input type="text"	 name="UseTextInputName" value="{$ClassName}" style="width:120px;">
     </td>
    </tr>
   </table>
   <br>
   <br>
  </td>
  <td class='left-border'>
  </td>
  <td>
   
   <table summary="{t}Class name selector{/t}" {$grey}>
    <tr>
     <td>
      <input type=radio name="classSelector" value="2" id="classSelector2" {$ClassNamesAvailable}>
     </td>
     <td>
      <h1 {$grey}><label for='classSelector2'>{t}Choose FAI class name from a list of existing classes{/t}</label></h1>
     </td>
    </tr>
    <tr>
     <td>&nbsp;
     </td>
     <td>{t}Class name{/t}&nbsp;
      
      <select name="SelectedClass" title="{t}Choose class name{/t}" style="width:120px;" {$ClassNamesAvailable}size=1>
       {html_options options=$ClassNames}
      </select>
     </td>
    </tr>
   </table>
   <br>
   <br>
  </td>
 </tr>
</table>
<hr>
<p style="text-align:right">
 <button type='submit' name='edit_continue'>{t}Continue{/t}</button>&nbsp;
 <button type='submit' name='edit_cancel'>
 {msgPool type='cancelButton'}</button>
</p><!-- Place cursor -->
<script language="JavaScript" type="text/javascript"><!--	focus_field('UseTextInputName');	--></script>
