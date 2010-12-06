<?php
/*
 * This code is part of GOsa (http://www.gosa-project.org)
 * Copyright (C) 2003-2008 GONICUS GmbH
 *
 * ID: $$Id: class_target_list.inc 9850 2008-03-14 14:15:12Z hickert $$
 *
 * This program is free software; you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation; either version 2 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program; if not, write to the Free Software
 * Foundation, Inc., 59 Temple Place, Suite 330, Boston, MA  02111-1307  USA
 */

class classSelect extends management
{
  // Tab definition 
  protected $skipFooter = TRUE;
  protected $skipHeader = TRUE;

  var $plHeadline = "Class selection";

  function __construct($config,$ui)
  {
    $this->config = $config;
    $this->ui = $ui;

    // Build filter
    if (session::global_is_set(get_class($this)."_filter")){
      $filter= session::global_get(get_class($this)."_filter");
    } else {
      $filter = new filter(get_template_path("selectClass-filter.xml", true, dirname(__FILE__)));
      $filter->setObjectStorage($this->storagePoints);
    }
    $this->setFilter($filter);

    // Build headpage
    $headpage = new listing(get_template_path("selectClass-list.xml", true, dirname(__FILE__)));
    $headpage->registerElementFilter("filterProperties", "classSelect::filterProperties");
    $headpage->setFilter($filter);
    parent::__construct($config, $ui, "FAI class", $headpage);
  }

  static function filterProperties($row, $classes)
  {
    /* Create different picture for each object type */
    $objTypes['FAIhook']            = image('plugins/fai/images/fai_hook.png');
    $objTypes['FAItemplate']        = image('plugins/fai/images/fai_template.png');
    $objTypes['FAIscript']          = image('plugins/fai/images/fai_script.png');
    $objTypes['FAIvariable']        = image('plugins/fai/images/fai_variable.png');
    $objTypes['FAIpackageList']     = image('plugins/fai/images/fai_packages.png');
    $objTypes['FAIpartitionTable']  = image('plugins/fai/images/fai_partitionTable.png');

    $str = "";
    foreach($objTypes as $class => $img){
      if(in_array($class, $classes)){
        $str .= $img;
      }else{
        $str .= image('images/empty.png');
      }
    }
    return($str);
  }

  function save()
  {
    $act = $this->detectPostActions();
    $headpage = $this->getHeadpage();
    if(!isset($act['targets'])) return(array());
    $ret = array();
    foreach($act['targets'] as $dn){
      $ret[] = $headpage->getEntry($dn);
    }
    return($ret);
  }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
