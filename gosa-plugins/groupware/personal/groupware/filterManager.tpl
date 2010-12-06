<h3>{t}Available filter rules {/t}</h3>

{$list}

{render acl=$acl}
    <button name='addFilter'>{msgPool type='addButton'}</button>
{/render}

<hr>
<div class="plugin-actions">
    {render acl=$acl}
        <button name='filterManager_ok'>{msgPool type='okButton'}</button>
    {/render}
    <button name='filterManager_cancel'>{msgPool type='cancelButton'}</button>
</div>
