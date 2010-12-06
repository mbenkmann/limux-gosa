<h3>{t}Kerberos kadmin access{/t}</h3>

{t}Kerberos Realm{/t}{$must}
 {render acl=$goKrbRealmACL}
  <input type='text' name="goKrbRealm" id="goKrbRealm" size=30 maxlength=60  value="{$goKrbRealm}">
 {/render}

{if $MIT_KRB}
 <h3>{t}Policies{/t}</h3>
 {render acl=$goKrbPolicyACL}
  {$list}
 {/render}
 <button type='submit' name='policy_add'>{msgPool type=addButton}</button>
{/if}


<hr>
<div class="plugin-actions">
 <button type='submit' name='SaveService'>{msgPool type=saveButton}</button>
 <button type='submit' name='CancelService'>{msgPool type=cancelButton}</button>
</div>
<input type="hidden" name="goKrbServerPosted" value="1">
