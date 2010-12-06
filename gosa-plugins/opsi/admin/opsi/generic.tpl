
<h3>{t}OPSI host{/t}</h3>

{if $init_failed}<font style='color: #FF0000;'>

 {msgPool type=siError p=$message}</font>
 <button type='submit' name='reinit'>{t}Retry{/t}</button>

{else}

 <table style="width: 100%;" summary="{t}OPSI host{/t}">
  <tr>
   <td style="width:50%;" class="right-border">
    <table summary="{t}Generic{/t}">
     
     {if $standalone}
      <tr>
       <td>{t}Name{/t}{$must}</td>
       <td>
        {render acl=$hostIdACL}
         <input style='width:300px;' type='text' name='hostId' value='{$hostId}'>
        {/render}
       </td>
      </tr>

      {else}

      <tr>
       <td>{t}Name{/t}</td>
       <td>
        {render acl=$hostIdACL}
         <input style='width:300px;' type='text' disabled value="{$hostId}">
        {/render}
       </td>
      </tr>
      
     {/if}
     <tr>
      <td>{t}Net boot product{/t}</td>
      <td>

       {render acl=$netbootProductACL}
        <select name="opsi_netboot_product" onChange="document.mainform.submit();" size=1>
         {foreach from=$ANP item=item key=key}
          <option {if $key == $SNP} selected {/if} value="{$key}">{$key}</option>
         {/foreach}
        </select>
       {/render}
       
       {if $netboot_configurable}
        {image path="images/lists/edit.png" action="configure_netboot" title="{t}Configure product{/t}"}
       {/if}
      </td>
     </tr>
    </table>

   </td>
   <td>

    <table summary="{t}Generic{/t}">
     <tr>
      <td>{t}Description{/t}</td>
      <td>
       {render acl=$descriptionACL}
        <input type='text' name='description' value='{$description}'>
       {/render}
      </td>
     </tr>
     <tr>
      <td>{t}Notes{/t}</td>
      <td>
       {render acl=$descriptionACL}
        <input type='text' name='note' value='{$note}'>
       {/render}
      </td>
     </tr>
    </table>
   </td>
  </tr>
 </table>

 <hr>

 <table width="100%" summary="{t}Package settings{/t}">
  <tr>
   <td style="width:50%;" class="right-border">
    <h3>{t}Installed products{/t}</h3>
    {render acl=$localProductACL}
     {$divSLP}
    {/render}
   </td>
   <td style="width:50%;">
    <h3>{t}Available products{/t}</h3>
    {render acl=$localProductACL}
     {$divALP}
    {/render}
   </td>
  </tr>
 </table>
    
 {if $standalone}

  <hr> 

  <h3>{t}Action{/t}</h3>
  <select name='opsi_action' size=1>
   <option>&nbsp;</option>
    {if $is_installed}
     <option value="install">{t}Reinstall{/t}</option>
    {else}
     <option value="install">{t}Install{/t}</option>
    {/if}
    <option value="wake">{t}Wake{/t}</option>
   </select>

   {render acl=$triggerActionACL}
    <button type='submit' name='opsi_trigger_action'>{t}Execute{/t}</button>
   {/render}
 {/if}

 <hr>
 {$netconfig}
 <input type='hidden' name='opsiGeneric_posted' value='1'>
{/if}
