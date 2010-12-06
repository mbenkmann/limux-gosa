
<div style="font-size:18px;">
 {image path="images/warning.png" align="top"}&nbsp;{t}Warning{/t}
</div>
<p>
 {$info}{t}This includes all account data, system access, etc. for this branch. Please double check if your really want to do this since there is no way for GOsa to get your data back.{/t}
</p>
<p>{t}So - if you're sure - press 'Delete' to continue or 'Cancel' to abort.{/t}
</p>
<input type='hidden' name='release_hidden' value='{$release_hidden}'>
<hr>
<div class="plugin-actions">
 <button type='submit' name='delete_branch_confirm'>
 {msgPool type=delButton}</button>
 <button type='submit' name='delete_cancel'>
 {msgPool type=cancelButton}</button>
</div>