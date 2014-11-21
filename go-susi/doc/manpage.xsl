<?xml version="1.0" encoding="utf-8"?>
<!DOCTYPE xsl:stylesheet [ 
<!ENTITY nbsp   "&#160;">
<!ENTITY copy   "&#169;">  <!-- copyright sign, U+00A9 ISOnum -->
<!ENTITY ensp   "&#8194;"> <!-- en space, U+2002 ISOpub -->
<!ENTITY thinsp "&#8201;"> <!-- thin space, U+2009 ISOpub -->
<!ENTITY and  "&#8743;">   <!-- logical and = wedge, U+2227 ISOtech -->
<!ENTITY neq   "&#8800;">
<!ENTITY leq   "&#8804;">
]>

<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform"
 xmlns="http://www.w3.org/1999/xhtml"
 xmlns:str="http://exslt.org/strings"
 xmlns:h="http://www.w3.org/1999/xhtml"
 xmlns:set="http://exslt.org/sets"

><xsl:output
 method="text"
 encoding="utf-8"
 indent="no"
 media-type="text/plain"
></xsl:output>


<xsl:param name="name" select="'NAME'"></xsl:param>


<xsl:param name="section" select="'1'"></xsl:param>


<xsl:param name="version" select="'4.5.6'"></xsl:param>


<xsl:param name="start_id" select="'id.jes9sqqlstzn'"></xsl:param>


<xsl:param name="stop_id" select="'id.sj4np8e9wdb8'"></xsl:param>

<xsl:variable name="css" select="normalize-space(/h:html/h:head/h:style)"></xsl:variable>
<xsl:param name="ltr" select="string(//h:a[@id='id.hjmnt8awkh8s']/following-sibling::*[1]/@class)"></xsl:param>
<xsl:param name="mono" select="string(//h:a[@id='id.w63fd9fui69z']/following-sibling::*[1]/child::h:span[1]/@class)"></xsl:param>
<xsl:variable name="btemp" select="str:split(substring-before($css,'{font-weight:bold}'),'.')"></xsl:variable>
<xsl:param name="bold" select="string($btemp[count($btemp)])"></xsl:param>
<xsl:variable name="itemp" select="str:split(substring-before($css,'{font-style:italic}'),'.')"></xsl:variable>
<xsl:param name="italic" select="string($itemp[count($itemp)])"></xsl:param>
<xsl:param name="monobold" select="string(//h:a[@id='id.7rmu3qjxnnyy']/following-sibling::*[1]/child::h:span[1]/@class)"></xsl:param>
<xsl:param name="indent" select="string(//h:a[@id='id.f852w3yjg198']/following-sibling::*[1]/@class)"></xsl:param>
<xsl:param name="indentmore" select="string(//h:a[@id='id.3uhjxiaklp5d']/following-sibling::*[1]/@class)"></xsl:param>
<xsl:param name="indent3" select="string(//h:a[@id='id.i03t6xrtz85y']/following-sibling::*[1]/@class)"></xsl:param>
<xsl:param name="indent4" select="string(//h:a[@id='id.vbpk5t3411tq']/following-sibling::*[1]/@class)"></xsl:param>
<xsl:variable name="ctemp" select="str:split(substring-before($css,'{text-align:center}'),'.')"></xsl:variable>
<xsl:param name="center" select="string($ctemp[count($ctemp)])"></xsl:param>
<xsl:param name="indentedbull" select="string(//h:a[@id='id.fhvqu6w21q56']/following-sibling::h:ol[1]/@class)"></xsl:param>












<xsl:template match="*" priority="-10"
><xsl:apply-templates/> 
</xsl:template>




<xsl:template match="text()" priority="-7"
><xsl:value-of select="str:replace(str:replace(str:replace(str:replace(normalize-space(.),'\','\[char92]'),'&quot;','\[char34]'),'.','\[char46]'),'-','\-')"></xsl:value-of> 
</xsl:template>


<xsl:template match="text()[normalize-space(.)='']" priority="-6"
><xsl:value-of select="' '"></xsl:value-of> 
</xsl:template>


<xsl:template match="*[local-name(.)='br']"><xsl:text>\p </xsl:text> </xsl:template>














<xsl:template match="*[local-name(.)='html']"














><xsl:value-of select="concat('.TH ',$name,' ',$section,' &quot;VERSION ',$version,'&quot;',' ','&quot;(C) MATTHIAS S. BENKMANN&quot;',' ','&quot;GO-SUSI OPERATOR',&quot;'&quot;,'S MANUAL&quot;')"></xsl:value-of>
  <xsl:text>&#10;</xsl:text>
  <xsl:text>.nh&#10;</xsl:text> 
  <xsl:text>.ad l</xsl:text> 
  
  



  <xsl:text>.PD 1m</xsl:text> 
  <xsl:apply-templates/> 
  
  <xsl:text>&#10;.PD 1m&#10;.SH SEE ALSO</xsl:text>
  <xsl:text>&#10;.BR &quot;go-susi&quot; &quot;(1)        -  main description, API basics, XML message basics&quot;</xsl:text>
  <xsl:text>&#10;.br</xsl:text>
  <xsl:text>&#10;.BR &quot;gosa-si-server&quot; &quot;(1) -  the preferred way to launch go-susi&quot;</xsl:text>
  <xsl:text>&#10;.br</xsl:text>
  <xsl:text>&#10;.BR &quot;sibridge&quot; &quot;(1)       -  remote control for an si-server&quot;</xsl:text>
  <xsl:text>&#10;.br</xsl:text>
  <xsl:text>&#10;.BR &quot;initrd_autopack&quot; &quot;(5)-  a godsend for developers of initrd.img&quot;</xsl:text>
  <xsl:text>&#10;.br</xsl:text>
  <xsl:text>&#10;.BR &quot;generate_package_list&quot; &quot;(5) - Debian repository scanner (package-list-hook)&quot;</xsl:text>
  <xsl:text>&#10;.br</xsl:text>
  <xsl:text>&#10;.BR &quot;server.conf&quot; &quot;(5)    -  configuration file&quot;</xsl:text>
  <xsl:text>&#10;.br</xsl:text>
  <xsl:text>&#10;.BR &quot;gosa-si-jobs&quot; &quot;(5)   -  jobs database, job-related messages&quot;</xsl:text>
  <xsl:text>&#10;.br</xsl:text>
  <xsl:text>&#10;.BR &quot;gosa-si-s2s&quot; &quot;(5)    -  server-server communication&quot;</xsl:text>
  <xsl:text>&#10;.br</xsl:text>
  <xsl:text>&#10;.BR &quot;gosa-si-client&quot; &quot;(5) -  client registration, new clients, job triggers&quot;</xsl:text>
  <xsl:text>&#10;.br</xsl:text>
  <xsl:text>&#10;.BR &quot;gosa-si-fai&quot; &quot;(5)    -  FAI installation and update&quot;</xsl:text>
  <xsl:text>&#10;.br</xsl:text>
  <xsl:text>&#10;.BR &quot;gosa-si-query&quot; &quot;(5)  -  query releases, kernels, packages, FAI logs&quot;</xsl:text>
  <xsl:text>&#10;.br</xsl:text>
  <xsl:text>&#10;.BR &quot;gosa-si-misc&quot; &quot;(5)   -  miscellaneous messages&quot;</xsl:text>
  <xsl:text>&#10;.br</xsl:text>
  <xsl:text>&#10;.BR &quot;gosa-si-deprecated&quot; &quot;(5) - messages not supported by go-susi&quot;</xsl:text>
</xsl:template>


<xsl:template match="*[local-name(.)='head']"></xsl:template>


<xsl:template match="*[local-name(.)='body']"
><xsl:apply-templates select="set:intersection(child::*[@id=$start_id]/following-sibling::*, child::*[@id=$stop_id]/preceding-sibling::*)"></xsl:apply-templates>
</xsl:template>


<xsl:template match="*[local-name(.)='h1']"
><xsl:text>&#10;.PD 1m&#10;.SH </xsl:text> 
  <xsl:value-of select="translate(normalize-space(.),'abcdefghijklmnopqrstuvwxyz','ABCDEFGHIJKLMNOPQRSTUVWXYZ')"></xsl:value-of>
</xsl:template>


<xsl:template match="*[local-name(.)='h2']"
><xsl:text>&#10;.PD 1m&#10;.SS </xsl:text>
  <xsl:value-of select="normalize-space(.)"></xsl:value-of>
</xsl:template>


<xsl:template match="*[local-name(.)='h3']"
><xsl:text>&#10;.PD 1m&#10;.SS </xsl:text>
  <xsl:value-of select="normalize-space(.)"></xsl:value-of>
</xsl:template>


<xsl:template match="*[local-name(.)='p']" priority="1"
><xsl:variable name="prev" select="normalize-space(preceding-sibling::h:p[1]/attribute::class)"></xsl:variable>
  <xsl:variable name="prevspan" select="normalize-space(preceding-sibling::h:p[1]/child::*[1]/attribute::class)"></xsl:variable>
  <xsl:variable name="myspan" select="normalize-space(child::*[1]/attribute::class)"></xsl:variable>
  
  <xsl:choose

><xsl:when test="@class=$indent"
><xsl:if test="$prev!=$ltr and $prev!=$indent"><xsl:text>&#10;.PD 1m</xsl:text> </xsl:if>
          <xsl:if test="$prev=$ltr or $prev=$indent"><xsl:text>&#10;.PD 0</xsl:text> </xsl:if>
          <xsl:if test="$prevspan=$monobold and $myspan=$monobold"><xsl:text>&#10;.PD 0</xsl:text> </xsl:if>
          <xsl:text>&#10;.IP &quot;&quot; 4</xsl:text>
    </xsl:when>
    
    
    <xsl:when test="@class=$indentmore"
><xsl:text>&#10;.IP &quot;&quot; 8</xsl:text>
    </xsl:when>
    
    
    <xsl:when test="@class=$indent3"
><xsl:text>&#10;.IP &quot;&quot; 12</xsl:text>
    </xsl:when>
    
    
    <xsl:when test="@class=$indent4"
><xsl:text>&#10;.IP &quot;&quot; 16</xsl:text>
    </xsl:when>
    
    
    <xsl:when test="@class=concat($center,' ',$ltr) or @class=concat($ltr,' ',$center)"
><xsl:text>&#10;.PD 1m&#10;.IP &quot;&quot; 16</xsl:text>
    </xsl:when>
    
    
    <xsl:otherwise
><xsl:if test="$prevspan=$monobold and $myspan=$monobold"><xsl:text>&#10;.PD 0</xsl:text> </xsl:if>
          <xsl:if test="$prevspan!=$monobold or $myspan!=$monobold"><xsl:text>&#10;.PD 1m</xsl:text> </xsl:if>
          <xsl:text>&#10;.P</xsl:text>
    </xsl:otherwise>
  </xsl:choose>
  <xsl:apply-templates/>
</xsl:template>


<xsl:template match="*[local-name(.)='span']"
><xsl:choose

><xsl:when test="@class=$mono and (count(preceding-sibling::node())>0 or count(following-sibling::node())>0)"
><xsl:text>&#10;.IR &quot;</xsl:text> <xsl:apply-templates/> <xsl:text>&quot;</xsl:text>
    </xsl:when>

    
    <xsl:when test="@class=$mono and (count(preceding-sibling::node())=0 and count(following-sibling::node())=0)"
><xsl:text>&#10;.PD 0&#10;.BR &quot;</xsl:text> <xsl:apply-templates/> <xsl:text>&quot;</xsl:text>
    </xsl:when>
    
    
    <xsl:when test="@class=$italic and (count(preceding-sibling::node())>0 or count(following-sibling::node())>0)"
><xsl:text>&#10;.IR &quot;</xsl:text> <xsl:apply-templates/> <xsl:text>&quot;</xsl:text>
    </xsl:when>

    
    <xsl:when test="@class=$italic and (count(preceding-sibling::node())=0 and count(following-sibling::node())=0)"
><xsl:text>&#10;.PD 0&#10;.BR &quot;</xsl:text> <xsl:apply-templates/> <xsl:text>&quot;</xsl:text>
    </xsl:when>
    
    
    <xsl:when test="@class=$monobold and (count(preceding-sibling::node())>0 or count(following-sibling::node())>0)"
><xsl:text>&#10;.BR &quot;</xsl:text> <xsl:apply-templates/> <xsl:text>&quot;</xsl:text>
    </xsl:when>

    
    <xsl:when test="@class=$monobold and (count(preceding-sibling::node())=0 and count(following-sibling::node())=0)"
><xsl:text>&#10;.PD 0&#10;.BR &quot;</xsl:text> <xsl:apply-templates/> <xsl:text>&quot;</xsl:text>
    </xsl:when>
    
    
    <xsl:when test="@class=$bold and (count(preceding-sibling::node())>0 or count(following-sibling::node())>0)"
><xsl:text>&#10;.BR &quot;</xsl:text> <xsl:apply-templates/> <xsl:text>&quot;</xsl:text>
    </xsl:when>

    
    <xsl:when test="@class=$bold and (count(preceding-sibling::node())=0 and count(following-sibling::node())=0)"
><xsl:text>&#10;.PD 0&#10;.BR &quot;</xsl:text> <xsl:apply-templates/> <xsl:text>&quot;</xsl:text>
    </xsl:when>

    
    <xsl:otherwise
><xsl:text>&#10;.R </xsl:text> <xsl:apply-templates/>
    </xsl:otherwise>
  </xsl:choose>
</xsl:template>


<xsl:template match="*[local-name(.)='ol']"

><xsl:choose
><xsl:when test="@class=$indentedbull"
><xsl:text>&#10;.RS</xsl:text>
          <xsl:apply-templates/> 
          <xsl:text>&#10;.RE</xsl:text>
    </xsl:when>
    <xsl:otherwise
><xsl:apply-templates/>
    </xsl:otherwise>
  </xsl:choose>
</xsl:template>
<xsl:template match="*[local-name(.)='li']"><xsl:text>&#10;.TP 4&#10;.B \[bu]</xsl:text>  <xsl:apply-templates/> </xsl:template>


</xsl:stylesheet>
