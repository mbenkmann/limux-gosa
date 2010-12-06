<h3>{t}Set root-user password{/t}</h3>

<hr>

<p>
 {t}Password{/t}: &nbsp;<input type="text" name="rootPassword" value="">
 <select name="passwordHash" size=1>
  {html_options options=$hashes selected=$hash}
 </select>
</p>

<hr>

<div class="plugin-actions">
    <button name="setPassword">{msgPool type=okButton}</button>
    <button name="cancelPassword">{msgPool type=cancelButton}</button>
</div>
