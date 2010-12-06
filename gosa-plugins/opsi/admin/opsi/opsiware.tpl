

{if $init_failed}
 <h3>{t}Information{/t}
 </h3><font style='color: #FF0000;'>
 {msgPool type=siError p=$message}</font>
 <button type='submit' name='reinit'>{t}Retry{/t}</button>
 {else}
 
 {if $type == 0}
  <h3>{t}Hardware information{/t}
  </h3>
  {else}
  <h3>{t}Software information{/t}
  </h3>
  
 {/if}{foreach from=$info item=item key=key}
 <div style='background-color: #E8E8E8; width: 100%; border: 2px dotted #CCCCCC;'>
  <h3>{t}Device{/t}
   {$key+1}
  </h3>{foreach from=$item key=name item=value}
  <div style="text-transform:lowercase;width:30%; float: left; ">
   {$name}:&nbsp;
  </div>
  <div style="width:70%; float: right;background-color: #DADADA;">
   {$value.0.VALUE}&nbsp;
  </div>
  <div style='clear: both;'>
  </div>{/foreach}
 </div>
 <br>{/foreach}
 
{/if}