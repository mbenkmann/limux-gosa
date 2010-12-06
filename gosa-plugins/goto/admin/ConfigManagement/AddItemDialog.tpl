
<h3>{t}Add item{/t}</h3>

{t}Please specify a name for the item to add. This name has to be unique within the item configuration.{/t}
<br>

<hr>

<p>
 <b>{$itemCfg.name}</b>&nbsp;-&nbsp; {$itemCfg.description}
</p>

{t}Name{/t}:&nbsp;<input type='text' name='itemName' value="{$itemName}">

<hr>

<div class='plugin-actions'>
    <button name='saveItemAdd'>{msgPool type=okButton}</button>
    <button name='cancelItemAdd'>{msgPool type=cancelButton}</button>
</div>

