
<table style='width:100%; ' summary="{t}Summary of used FAI classes{/t}">
 <tr>
  <td>
   <h3>{t}FAI object tree{/t}
   </h3>
  </td>
 </tr>
 
 {if $readable}
  <tr>
   <td>
    {image path="images/lists/reload.png" action="reloadList"}{t}Reload class and release configuration from parent object.{/t}
   </td>
  </tr>
  <tr>
   <td>
    {$objectList}
   </td>
  </tr>
  {else}
  <tr>
   <td>
    <h3>{t}You are not allowed to view the FAI summary.{/t}
    </h3>
   </td>
  </tr>
  
 {/if}
</table>
