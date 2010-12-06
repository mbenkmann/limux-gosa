<h3>{t}Policy settings{/t}</h3>

<table style="width:100%" summary="{t}Kerberos policy{/t}">
 <tr>
  <td class='right-border'>

   <table style="width:100%" summary="{t}Kerberos policy{/t}">
    <tr>
     <td>{t}Policy name{/t}{$must}</td>
     <td><input type="text" name="name" value="{$name}"></td>
    </tr>
    <tr>
     <td>{t}Minimum password length{/t}</td>
     <td><input type="text" name="PW_MIN_LENGTH" value="{$PW_MIN_LENGTH}"></td>
    </tr>
    <tr>
     <td>{t}Required different characters{/t}</td>
     <td><input type="text" name="PW_MIN_CLASSES" value="{$PW_MIN_CLASSES}"></td>
    </tr>
    <tr>
     <td>{t}Password history size{/t}</td>
     <td><input type="text" name="PW_HISTORY_NUM" value="{$PW_HISTORY_NUM}"></td>
    </tr>
   </table>

  </td>
  <td>

   <table style="width:100%" summary="{t}Kerberos policy{/t}">
    <tr>
     <td>
       <td>{t}Minimum password lifetime{/t}</td>
       <td><input type="text" name="PW_MIN_LIFE" value="{$PW_MIN_LIFE}">&nbsp;{t}seconds{/t}</td>
     </tr>
     <tr>
       <td>{t}Password lifetime{/t}</td>
       <td><input type="text" name="PW_MAX_LIFE" value="{$PW_MAX_LIFE}">&nbsp;{t}seconds{/t}</td>
     </tr>
   </table>

  </td>
 </tr>
 <tr>
  <td colspan="2">
   <br>
   {$POLICY_REFCNT}
  </td>
 </tr>
</table>

<input type="hidden" name="Policy_Posted" value="1">
<hr>

<div class="plugin-actions">
 <button type='submit' name='save_policy'>{msgPool type=okButton}</button>
 <button type='submit' name='cancel_policy'>{msgPool type=cancelButton}</button>
</div>
