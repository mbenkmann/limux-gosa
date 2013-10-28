<?php
/*
 * This code is part of GOsa (http://www.gosa-project.org)
 * Copyright (C) 2003-2010 GONICUS GmbH
 *
 * ID: $$Id$$
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

define ('LIST_NORMAL', 0);
define ('LIST_MARKED', 1);
define ('LIST_DISABLED', 2);

class sortableListing {
    private $header= null;
    private $colspecs= null;
    private $reorderable= true;
    private $width= "400px";
    private $height= "100px";
    private $cssclass= "";
    private $id;

    private $sortingEnabled= true;
    private $data= array();
    private $keys= array();
    private $modes= array();
    private $actions= array();
    private $displayData= array();
    private $columns= 0;
    private $deleteable= false;
    private $editable= false;
    private $massdelete= false;
    private $colorAlternate= false;
    private $instantDelete= true;
    private $action;
    private $mapping;
    private $current_mapping;
    private $active_index= array();
    private $scrollPosition= 0;
    private $primarySortColumn= 0;
    private $additionalSortColumns = array();
    private $sortDirection= array();

    private $acl= "";
    private $modified= false;


    public function sortableListing($data= array(), $displayData= null, $reorderable= false)
    {
        global $config;

        // Save data to display
        $this->setListData($data, $displayData);

        // Get list of used IDs 
        if(!session::is_set('sortableListing_USED_IDS')){
            session::set('sortableListing_USED_IDS',array());
        }
        $usedIds = session::get('sortableListing_USED_IDS');

        // Generate instance wide unique ID
        $id = "";
        while($id == "" || in_array_strict($id, $usedIds)){

            // Wait 1 msec to ensure that we definately get a new id
            if($id != "") usleep(1);
            $tmp= gettimeofday();
            $id =  'l'.md5(microtime().$tmp['sec']);
        }

        // Only keep the last 10 list IDsi
        $usedIds = array_slice($usedIds, count($usedIds) -10, 10);
        $usedIds[] = $id;
        session::set('sortableListing_USED_IDS',$usedIds);
        $this->id = $id;

        // Set reorderable flag
        $this->reorderable= $reorderable;
        if (!$reorderable) {
            $this->sortData();
        }
    }

    public function setReorderable($bool)
    {
        $this->reorderable= $bool;
    }

    public function setDefaultSortColumn($id)
    {
        $this->primarySortColumn = $id;
    }

    public function setAdditionalSortColumns($primarySortColumn, array $additionalSortColumns)
    {
        $this->additionalSortColumns[$primarySortColumn] = $additionalSortColumns;
    }

    /*
     * 
     * Examples
     * DatenARray ($data)
     * @param: array( arbitrary object, arbitrary object)
     * Datenarray will be manipulated by add, del and sort operations. According to this it will be returned from this widget.
     * The index of a data entry must correspond to the entry of the "display array" following.
     * DisplayArray ($displyData)
     * @param: array("eins" array( "data"=> array("Uno", "2", "x" ) , "zwei" array( "data"=> array("Due", "3", "y" ))) ;
     * label pointing on a list of columns that will be shown in the list.
     */
    public function setListData($data, $displayData= null)
    {
        // Save data to display
        $this->setData($data);
        if (!$displayData) {
            $displayData= array();
            foreach ($data as $key => $value) {
                $displayData[$key]= array("data" => array($value));
            }
        }
        $this->setDisplayData($displayData);
    }

    //setting flat data 
    private function setData($data)
    {
        $this->data= $data;
    }

    // collecting the display data -
    private function setDisplayData($data)
    {
        if (!is_array($data)) {
            trigger_error ("sortableList needs an array as data!");
        }

        // Transfer information
        $this->displayData= array();
        $this->modes= array();
        $this->actions = array();
        $this->mapping= array();
        foreach ($data as $key => $value) {
            $this->displayData[]= $value['data'];
            if (isset($value['mode'])) {
                $this->modes[]= $value['mode'];
            }
            // Allow overwriting allowed actions of a single entry
            if (isset($value['actions'])) {
                $this->actions[] = $value['actions'];
            }
        }
        $this->keys= array_keys($data);

        // Create initial mapping
        if(count($this->keys)){
            $this->mapping= range(0, abs(count($this->keys)-1));
        }
        $this->current_mapping= $this->mapping;

        // Find the number of coluns
        reset($this->displayData);
        $first= current($this->displayData);
        if (is_array($first)) {
            $this->columns= count($first);
        } else {
            $this->columns= 1;
        }

        // Preset sort orders to 'down'
        for ($column= 0; $column<$this->columns; $column++) {
            if(!isset($this->sortDirection[$column])){
                $this->sortDirection[$column]= true;
            }
        }
    }


    public function setWidth($width)
    {
        $this->width= $width;
    }


    public function setInstantDelete($flag)
    {
        $this->instantDelete= $flag;
    }


    public function setColorAlternate($flag)
    {
        $this->colorAlternate= $flag;
    }


    public function setEditable($flag)
    {
        $this->editable= $flag;
    }


    public function setDeleteable($flag)
    {
        $this->deleteable= $flag;
    }


    public function setMassdelete($flag)
    {
        $this->massdelete= $flag;
    }


    public function setHeight($height)
    {
        $this->height= $height;
    }


    public function setCssClass($css)
    {
        $this->cssclass= $css;
    }


    public function setHeader($header)
    {
        $this->header= $header;
    }


    public function setColspecs($specs)
    {
        $this->colspecs= $specs;
    }


    public function render()
    {	
        $result= "<div class='sortableListContainer' id='scroll_".$this->id."' style='min-width:".$this->width.";height: ".$this->height."'>\n";
        $result.= "<table summary='"._("Sortable list")."' border='0' cellpadding='0' cellspacing='0' width='100%' style='width:100%' ".(!empty($this->cssclass)?" class='".$this->cssclass."'":"").">\n";
        $action_width= 0;

        // Define images based on list attributes
        $edit_img_disabled = $this->editable?image("images/lists/edit-grey.png"):"";
        $edit_img_enabled  = $this->editable?image('images/lists/edit.png', "%ID", _("Edit this entry")):"";
        $delete_img_disabled = $this->deleteable?image('images/lists/trash-grey.png'):"";
        $delete_img_enabled = $this->deleteable?image('images/lists/trash.png', "%ID", _("Delete this entry")):"";

        // Define default images based on ACL
        if (strpos($this->acl, 'w') === false) {
            $edit_image= $edit_img_disabled;
        } else {
            $edit_image= $edit_img_enabled;
        }
        if (strpos($this->acl, 'w') === false) {
            $delete_image= $delete_img_disabled;
            $massdelete_enabled= false;
        } else {
            $delete_image= $delete_img_enabled;
            $massdelete_enabled= true;
        }

        // Do we need colspecs?
        $action_width= ($this->editable?30:0) + ($this->deleteable?30:0);
        if ($this->colspecs) {
            $result.= " <colgroup>\n";
            for ($i= 0; $i<$this->columns; $i++) {
                if(isset($this->colspecs[$i]) && $this->colspecs[$i] != '*'){
                    $result.= "  <col style='width:".($this->colspecs[$i])."'>\n";
                }else{
                    $result.= "  <col>\n";
                }
            }

            // Extend by another column if we've actions specified
            if ($action_width) {
                $result.= "  <col style='width:".$action_width."px' >\n";
            }
            $result.= " </colgroup>\n";
        }

        // Do we need a header?
        if ($this->header) {
            $result.= " <thead>\n  <tr>\n";
            $first= " style='border-left:0'";
            if ($this->massdelete && $massdelete_enabled)
                $this->header[0]="<input type='checkbox' id='select_all' name='select_all' title='"._("Select all")."' onClick='toggle_all_(\"listing_selected_[0-9]*\$\",\"select_all\");' />";
            for ($i= 0; $i<$this->columns; $i++) {
                $link= "href='?plug=".$_GET['plug']."&amp;PID=".$this->id."&amp;act=SORT_$i'";
                $sorter= "";
                if ($i == $this->primarySortColumn){
                    $sorter= "&nbsp;".image("images/lists/sort-".($this->sortDirection[$i]?"up":"down").".png", null, $this->sortDirection[$i]?_("Sort ascending"):_("Sort descending"));
                }

                if($this->massdelete && $massdelete_enabled && $i==0) {
                    $result.="<th$first>{$this->header[0]}</th>";
                } elseif ($this->reorderable) {
                    $result.= "   <th$first>".(isset($this->header[$i])?$this->header[$i]:"")."</th>";
                } else {
                    $result.= "   <th$first><a $link>".(isset($this->header[$i])?$this->header[$i]:"")."</a>$sorter</th>";
                }
                $first= "";
            }
            if ($action_width) {
                $result.= "<th>&nbsp;</th>";
            }
            $result.= "\n  </tr>\n </thead>\n";
        }

        // Render table body if we've read permission
        $result.= " <tbody id='".$this->id."'>\n";
        $reorderable= $this->reorderable?"":" style='cursor:default'";

        if (strpos($this->acl, 'r') !== false) {
            foreach ($this->mapping as $nr => $row) {
                $editable= $this->editable?" onClick='$(\"edit_".$this->id."_$nr\").click()'":"";

                $id= "";
                if (isset($this->modes[$row])) {
                    switch ($this->modes[$row]) {
                        case LIST_DISABLED:
                            $id= " sortableListItemDisabled";
                            $editable= "";
                            break;
                        case LIST_MARKED:
                            $id= " sortableListItemMarked";
                            break;
                    }
                }

                // If an action is defined for this row, use it instead of
                // the default list action
                if (isset($this->actions[$row])) {
                    if (isset($this->actions[$row]['edit'])) {
                         $edit_image = ($this->actions[$row]['edit'])?$edit_img_enabled:$edit_img_disabled;
                   }
                    if (isset($this->actions[$row]['delete'])) {
                        $delete_image = ($this->actions[$row]['delete'])?$delete_img_enabled:$delete_img_disabled;
                    }
                }

                $result.= "  <tr class='sortableListItem".((($nr&1)||!$this->colorAlternate)?'':'Odd')."$id' id='item_".$this->id."_$nr'$reorderable>\n";
                $first= " style='border:0'";

                if ($this->massdelete) {
                    if ($massdelete_enabled)
                        $this->displayData[$row][0]="<input type=\"checkbox\" id=\"listing_selected_$nr\" name=\"massdelete[]\" value=\"{$this->id}_$nr\" />";
                    else
                        $this->displayData[$row][0]="";
                }

                foreach ($this->displayData[$row] as $column) {

                    // Do NOT use the onClick statement for columns that contain links or buttons.
                    if(preg_match("<.*type=.submit..*>", $column) || preg_match("<a.*href=.*>", $column)){
                        $result.= "   <td$first>".$column."</td>\n";
                    }else{
                        $result.= "   <td$editable$first>".$column."</td>\n";
                    }
                    $first= "";
                }

                if ($action_width) {
                    $result.= "<td>".str_replace('%ID', "edit_".$this->id."_$nr", $edit_image).
                        str_replace('%ID', "del_".$this->id."_$nr", $delete_image)."</td>";
                }

                $result.= "  </tr>\n";
            }
        }

        // Add spacer
        $result.= "  <tr class='sortableListItemFill' style='height:100%'><td style='border:0'></td>";
        $num= $action_width?$this->columns:$this->columns-1;
        for ($i= 0; $i<$num; $i++) {
            $result.= "<td class='sortableListItemFill'></td>";
        }
        $result.= "</tr>\n";

        $result.= " </tbody>\n</table>\n</div>\n";
#    $result.= " <input type='hidden' name='PID' value='".$this->id."' id='PID'>\n";
        $result.= " <input type='hidden' name='position_".$this->id."' id='position_".$this->id."'>\n";
        $result.= " <input type='hidden' name='reorder_".$this->id."' id='reorder_".$this->id."'>\n";
        if($this->massdelete && $massdelete_enabled)
            $result.= "<button name='massdelete_active' value=\"removeme\">".msgPool::delButton()."</button>\n";

        // Append script stuff if needed
        $result.= '<script type="text/javascript" language="javascript">';
        if ($this->reorderable) {
            $result.= ' function updateOrder(){';
                $result.= '    var ampcharcode= \'%26\';';
                $result.= '    var serializeOpts = Sortable.serialize(\''.$this->id.'\')+"='.$this->id.'";';
                $result.= '    $("reorder_'.$this->id.'").value= serializeOpts;';
                $result.= '    document.mainform.submit();';
                $result.= ' }';
                $result.= 'Position.includeScrollOffsets = true;';
                $result.= ' Sortable.create(\''.$this->id.'\',{tag:\'tr\', ghosting:false, constraint:\'vertical\', scroll:\'scroll_'.$this->id.'\',onUpdate : updateOrder});';
        }
        $result.= '$("scroll_'.$this->id.'").scrollTop= '.$this->scrollPosition.';';
        $result.= 'var box = $("scroll_'.$this->id.'").onscroll= function() {$("position_'.$this->id.'").value= this.scrollTop;}';
        $result.= '</script>';

        return $result;
    }


    public function update()
    {

        // Filter GET with "act" attributes
        if (!$this->reorderable){
            if(isset($_GET['act']) && isset($_GET['PID']) && $this->id == $_GET['PID']) {

                $key= validate($_GET['act']);
                if (preg_match('/^SORT_([0-9]+)$/', $key, $match)) {

                    // Switch to new column or invert search order?
                    $column= (int) $match[1];
                    if ($this->primarySortColumn != $column) {
                        $this->primarySortColumn= $column;
                    } else {
                        $this->sortDirection[$column]= !$this->sortDirection[$column];
                    }

                }
            }

            // Update mapping according to sort parameters
            $this->sortData();
        }
    }


    public function save_object()
    {
        // Do not do anything if this is not our PID, or there's even no PID available...
        if(isset($_REQUEST['PID']) && $_REQUEST['PID'] != $this->id) {
            return;
        }

        // Do not do anything if we're not posted - or have no permission
        if (strpos($this->acl, 'w') !== false && isset($_POST['reorder_'.$this->id])){

            if (isset($_POST['position_'.$this->id]) && is_numeric($_POST['position_'.$this->id])) {
                $this->scrollPosition= get_post('position_'.$this->id);
            }

            // Move requested?
            $move= get_post('reorder_'.$this->id);
            if ($move != "") {
                preg_match_all('/=([0-9]+)[&=]/', $move, $matches);
                $this->action= "reorder";
                $tmp= array();
                foreach ($matches[1] as $id => $row) {
                    $tmp[$id]= $this->mapping[$row];
                }
                $this->mapping= $tmp;
                $this->current_mapping= $matches[1];
                $this->modified= true;
                return;
            }
        }

        $this->action = "";
        if (strpos($this->acl, 'd') !== false){
            foreach ($_POST as $key => $value) {
                $value = get_post($key);
            }
        }

        if(isset($_POST["massdelete_active"]) && isset($_POST["massdelete"]) && $_POST["massdelete_active"]!="") {
            $t= get_post("massdelete");
            $targets= array();
            foreach ($t as $id) {
                if (preg_match('/^'.$this->id.'_([0-9]+)$/', $id, $matches))
                    $_POST["del_".$this->id."_".$matches[1]]= true;
            }
        }

        $this->active_index= array();

        foreach ($_POST as $key => $value) {
            $value = get_post($key);

            // Delete requested?
            if (preg_match('/^del_'.$this->id.'_([0-9]+)$/', $key, $matches)) {
                if(!isset($this->mapping[$matches[1]])) return;

                $me= $this->mapping[$matches[1]];

                // Ignore request if mode requests it
                if (isset($this->modes[$me]) && $this->modes[$me] == LIST_DISABLED) {
                    $me= null;
                    continue;
                }
    
                // Allowed to delete?
                $allow_delete = strpos($this->acl, 'w');
                if (isset($this->actions[$me]['delete'])) {
                    $this->actions[$me]['delete'];
                }

                // Set action
                $this->action= "delete";

                // Remove value if requested
                if ($this->instantDelete) {
                    $this->deleteEntry($me);
                }
                
                $this->active_index[]=$me;
            }


            // Edit requested?
            if (preg_match('/^edit_'.$this->id.'_([0-9]+)$/', $key, $matches)) {
                if(!isset($this->mapping[$matches[1]])) return;

                $this->active_index= $this->mapping[$matches[1]];

                // Ignore request if mode requests it
                if (isset($this->modes[$this->active_index]) && $this->modes[$this->active_index] == LIST_DISABLED) {
                    $this->active_index= null;
                    continue;
                }

                // Allowed to edit?
                $allow_edit = strpos($this->acl, 'w');
                if (isset($this->actions[$this->active_index]['edit'])) {
                    $allow_edit = $this->actions[$this->active_index]['edit'];
                }

                $this->action= $allow_edit?"edit":"";
            }
        }
    }


    public function getAction()
    {
        // Do not do anything if we're not posted
        if(!isset($_POST['reorder_'.$this->id])) {
            return;
        }

        // For reordering, return current mapping
        if ($this->action == 'reorder') {
            return array("targets" => $this->current_mapping, "mapping" => $this->mapping, "action" => $this->action);
        }

        if(!is_array($this->active_index))
            $this->active_index=array($this->active_index);
        // Edit and delete
        $result= array("targets" => $this->active_index, "action" => $this->action);

        return $result;
    }


    public function deleteEntry($id)
    {
        // Remove mapping
        $index= array_search($id, $this->mapping);
        if ($index !== false) {
            $target = $this->mapping[$index];
            $key = $this->keys[$target];

            unset($this->mapping[$index]);
            if(isset($this->displayData[$target])){
                unset($this->displayData[$target]);
                unset($this->data[$key]);
                unset($this->keys[$target]);
            }
            $this->mapping= array_values($this->mapping);
            $this->modified= true;
        }
    }


    public function getMaintainedData()
    {
        $tmp= array();
        foreach ($this->mapping as $src => $dst) {
            $realKey  = $this->keys[$dst];
            $tmp[$realKey] = $this->data[$realKey];
        }
        return $tmp;
    }


    public function isModified()
    {
        return $this->modified;
    }


    public function setAcl($acl)
    {
        $this->acl= $acl;
    }


    public function sortingEnabled($bool = TRUE)
    {
        $this->sortingEnabled= $bool;
    }
    
    protected function removeValueFromArray(array $array, $value) {
      $filterFunction = function ($x) use ($value) { return $x !== $value; };
      
      return array_filter($array, $filterFunction);
    }

    protected function getAdditionalSortColumns() {
        if (isset($this->additionalSortColumns[$this->primarySortColumn])) {
            return $this->additionalSortColumns[$this->primarySortColumn];
        } else {
            $allColumns = range(0, $this->columns - 1);
            $columnsWithoutPrimarySortColumn = $this->removeValueFromArray($allColumns, $this->primarySortColumn);
            return $columnsWithoutPrimarySortColumn;
        }
    }

    public function sortData()
    {
        if (!$this->sortingEnabled || empty($this->data)) return;

        $columnsSortPriority = array_merge(array($this->primarySortColumn), $this->getAdditionalSortColumns());
        $reverseSort = !$this->sortDirection[$this->primarySortColumn];

        $comparatorFunction = function($a, $b) use ($columnsSortPriority, $reverseSort) {
            foreach ($columnsSortPriority as $column) {
                if ($reverseSort) {
                    $comparisonResult = strcasecmp($b[$column], $a[$column]);
                } else {
                    $comparisonResult = strcasecmp($a[$column], $b[$column]);
                }
                if ($comparisonResult != 0) {
                    // Found a difference
                    break;
                }
            }

            return $comparisonResult;
        };

        $sortableData = $this->displayData;
        uasort($sortableData, $comparatorFunction);
        $this->mapping = array_keys($sortableData);
      }


    public function addEntry($entry, $displayEntry= null, $key= null)
    {
        // Only add if not already there
        if (!$key) {
            if (in_array_strict($entry, $this->data)) {
                return;
            }
        } else {
            if (isset($this->data[$key])) {
                return;
            }
        }

        // Prefill with default value if not specified
        if (!$displayEntry) {
            $displayEntry= array('data' => array($entry));
        }

        // Append to data and mapping
        $this->displayData[]= $displayEntry['data'];
        $this->mapping[]= max(array_keys($this->displayData));
        $this->modified= true;
        if ($key) {
            $this->data[$key]= $entry;
            $this->keys[]= $key;
        } else {
            $this->data[]= $entry;
            $this->keys[]= max(array_keys($this->displayData));
        }
        // Find the number of coluns
        reset($this->displayData);
        $first= current($this->displayData);
        if (is_array($first)) {
            $this->columns= count($first);
        } else {
            $this->columns= 1;
        }

        // Preset sort orders to 'down'
        for ($column= 0; $column<$this->columns; $column++) {
            if(!isset($this->sortDirection[$column])){
                $this->sortDirection[$column]= true;
            }
        }


        // Sort data after we've added stuff
        $this->sortData();
    }


    public function getKey($index) {
        return isset($this->keys[$index])?$this->keys[$index]:null;
    }

    public function getData($index) {
        $realkey = $this->keys[$index];
        return($this->data[$realkey]);
    }
}
