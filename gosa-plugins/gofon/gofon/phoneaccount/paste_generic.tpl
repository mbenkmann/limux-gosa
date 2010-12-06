<h3>{t}Phone settings{/t}</h3>
<table summary="{t}Phone numbers{/t}" style="width:100%" cellspacing=0>
 <tr>
  <td class='right-border'>
   <h3>{t}Phone numbers{/t}</h3>
  </td>
 </tr>
 <tr>
  <td class='right-border'>
   <select style="width:100%;" name="phonenumber_list[]" size=7 multiple>
    {html_options options=$phoneNumbers}
     <option disabled>&nbsp; </option>
   </select>
   <br>
   <input type='text' name="phonenumber" size=20 align=middle maxlength=60 value="">
   <button type='submit' name='add_phonenumber'>{msgPool type=addButton}</button> 

   <button type='submit' name='delete_phonenumber'>{msgPool type=delButton}</button>

  </td>
  <td style='width:50%;'>   <table summary="" style="width:100%" border=0>
    <tr>
     <td>      <h3>{t}Telephone hardware{/t}</h3>
      <table summary="{t}Telephone{/t}" border=0>
       <tr>
        <td>
         <label for="goFonVoicemailPIN">{t}Voice mail PIN{/t}{$must}</label>
        </td>
        <td>
         <input type="password" id="goFonVoicemailPIN" name="goFonVoicemailPI" value="{$goFonVoicemailPIN}">
        </td>
       </tr>
       <tr>
        <td>
         <label for="goFonPIN">{t}Phone PIN{/t}{$must}</label>
        </td>
        <td>
         <input type="password" id="goFonPIN" name="goFonPIN" value="{$goFonPIN}">
        </td>
       </tr>
      </table>
     </td>
    </tr>
   </table>
  </td>
 </tr>
</table>
<input type="hidden" name="phoneTab" value="phoneTab">
<br>
<br>
