<h3>{t}Properties{/t}</h3>

<table summary="{t}Conference{/t}">
  <tr>
    <td>
    	<table summary="{t}Generic settings{/t}">
        <tr>
          <td>
            <LABEL for="cn">
              {t}Conference name{/t}
            </LABEL>
            {$must}
          </td>
          <td>
{render acl=$cnACL}
            <input type='text' id="cn" name="cn" size=25 maxlength=60 value="{$cn}" title="{t}Name of conference to create{/t}">
{/render}
          </td>
        </tr>
        <tr>
          <td>
            <LABEL for="goFonConferenceOption_D">
              {t}Type{/t}
            </LABEL>
            {$must}
          </td>
          <td>
{render acl=$goFonConferenceOptionDACL}
            <select id="goFonConferenceOption_D" size="1" name="goFonConferenceOption_D" 
              title="{t}Choose subtree to place conference in{/t}"> 
              {html_options options=$goFonConferenceOptions selected=$goFonConferenceOption_D}
            </select>
{/render} 
          </td>
        </tr>
        <tr>
          <td colspan="2">
            <br>
          </td>
        </tr>
        <tr>
          <td>
            <LABEL for="base">
              {t}Base{/t}
            </LABEL>
            {$must}
          </td>
          <td>

{render acl=$baseACL}
          {$base}
{/render}

          </td>
        </tr>
        <tr>
          <td><LABEL for="goFonHomeServer">{t}Home server{/t}</LABEL>{$must}</td>
          <td>
{render acl=$goFonHomeServerACL}
                <select name='goFonHomeServer' size=1>
                 {html_options options=$goFonHomeServers selected=$goFonHomeServer}
                </select>
{/render}
          </td>
        </tr>
        <tr>
          <td>{t}Language{/t}
          </td>
          <td>
{render acl=$languageACL}
            <select name="language" title="{t}Select language{/t}" size=1>
              {html_options options=$languages selected=$language}
            </select>
{/render}
          </td>
        </tr>
      </table>
    </td>
    <td class="left-border">
      &nbsp;
    </td>
    <td width="50%">
      <table summary="{t}Additional settings{/t}">
        <tr>
          <td>
            <LABEL for="description">
              {t}Description{/t}
            </LABEL>
          </td>
          <td>
{render acl=$descriptionACL}
            <input id="description" name="description" size=40 maxlength=80 type='text' 
              value="{$description}" title="{t}Descriptive text for department{/t}">
{/render} 
          </td>
        </tr>
        <tr>
          <td>
            {t}Lifetime (in days){/t}
          </td>
          <td>
{render acl=$goFonConferenceOptionLifetimeACL}
            <input type='text' name="goFonConferenceOptionLifetime" size=15 value="{$goFonConferenceOptionLifetime}">
{/render} 
          </td>
        </tr>
        <tr>
          <td>
            {t}Phone number{/t}
            {$must}
          </td>
          <td>
{render acl=$telephoneNumberACL}
            <input type='text' name="telephoneNumber" value="{$telephoneNumber}" size=15>
{/render} 
          </td>
        </tr>
      </table>

    </td>
  </tr>
  <tr>
    <td colspan=3 >
      <hr>
    </td>
  </tr>
  <tr>
    <td colspan=3>
      <h3>{t}Options{/t}</h3>
    </td>
  </tr>
  <tr>
    <td>

	    <table summary="{t}Options{/t}">
        <tr>
	        <td>
{render acl=$goFonConferenceOptionPACL}
		        <input type="checkbox" onclick="changeState('goFonPIN');" name="goFonConferenceOption_P" 
					    value="P" {$goFonConferenceOption_PCHK}> 
{/render} 
        	</td>
          <td>
		        {t}Preset PIN{/t}
	        </td>
	      </tr>
	      <tr>
	        <td>
	          &nbsp;
	        </td>
	        <td>
		        {t}PIN{/t} 
{render acl=$goFonPINACL}
            <input type='password' name="goFonPIN" id="goFonPIN" value="{$goFonPIN}" title="{$goFonPIN}">
{/render} 
        	</td>
	      </tr>
        <tr>
	        <td colspan=2>
{render acl=$goFonConferenceOptionrACL}
		        <input type="checkbox" onclick="changeState('goFonConferenceOptionFormat');" name="goFonConferenceOption_r" 
              value="r" {$goFonConferenceOption_rCHK}> 
{/render} 
    	      {t}Record conference{/t}
	        </td>
	      </tr>
	      <tr>
	        <td>
	          &nbsp;
	        </td>
	        <td>
	          {t}Sound file format{/t}&nbsp;
{render acl=$goFonConferenceOptionFormatACL}
	          <select id="goFonConferenceOptionFormat" size="1" name="goFonConferenceOptionFormat" 
              title="{t}Choose subtree to place department in{/t}"> 
	            {html_options options=$goFonConferenceOptionFormats selected=$goFonConferenceOptionFormat}
          	</select>
{/render} 
        	</td>
	      </tr>
	    </table>

    </td>
    <td class="left-border">
      &nbsp;
	  </td>
    <td>

      <table summary="{t}Flags{/t}">
        <tr>
          <td>
{render acl=$goFonConferenceOptionMACL}
            <input type="checkbox" name="goFonConferenceOption_M" value="M" {$goFonConferenceOption_MCHK}> 
{/render} 
            {t}Play music on hold{/t}
          </td>
        </tr>
        <tr>
          <td>
{render acl=$goFonConferenceOptionsACL}
            <input type="checkbox" name="goFonConferenceOption_s" value="s" {$goFonConferenceOption_sCHK}> 
{/render} 
            {t}Activate session menu{/t}
          </td>
        </tr>
        <tr>
          <td>
{render acl=$goFonConferenceOptioniACL}
            <input type="checkbox" name="goFonConferenceOption_i" value="i" {$goFonConferenceOption_iCHK}> 
{/render} 
            {t}Announce users joining or leaving the conference{/t}
         </td>
        </tr>
        <tr>
          <td>
{render acl=$goFonConferenceOptioncACL}
            <input type="checkbox" name="goFonConferenceOption_c" value="c" {$goFonConferenceOption_cCHK}>
{/render} 
            {t}Count users{/t}
          </td>
        </tr>
      </table>

    </td>
  </tr>
</table>

<input type='hidden' name='phoneConferenceGeneric' value='1'>

<!-- Place cursor -->
<script language="JavaScript" type="text/javascript">
  <!-- // First input field on page
	focus_field('cn');
  -->
</script>
<!-- 
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler: 
-->
