<h3>{t}VoIP database information{/t}</h3>
  <table summary="{t}VoIP database information{/t}">
    <tr>
     <td>{t}Asterisk DB user{/t}{$must}</td>
     <td>
{render acl=$goFonAdminACL}
      <input type='text' name="goFonAdmin" size=30 maxlength=60 id="goFonAdmin" value="{$goFonAdmin}">
{/render}
     </td>
    </tr>
    <tr>
     <td>{t}Password{/t}{$must}</td>
     <td>
{render acl=$goFonPasswordACL}
      <input type=password name="goFonPassword" id="goFonPassword" size=30 maxlength=60 value="{$goFonPassword}">
{/render}
     </td>
    </tr>
    <tr>
     <td>{t}Country dial prefix{/t}{$must}</td>
     <td>
{render acl=$goFonCountryCodeACL}
      <input type='text' name="goFonCountryCode" size=10 maxlength=30 id="goFonCountryCode" value="{$goFonCountryCode}">
{/render}
     </td>
    </tr>
    <tr>
     <td>{t}Local dial prefix{/t}{$must}</td>
     <td>
{render acl=$goFonAreaCodeACL}
      <input type='text' name="goFonAreaCode" size=10 maxlength=30 id="goFonAreaCode" value="{$goFonAreaCode}">
{/render}
     </td>
    </tr>
   </table>

<hr>
<div class="plugin-actions">
 <button type='submit' name='SaveService'>{msgPool type=saveButton}</button>
 <button type='submit' name='CancelService'>{msgPool type=cancelButton}</button>
</div>
<input type="hidden" name="goFonServerPosted" value="1">
