<?php

class filterFolderWidget {

    static function query($base, $scope, $filter, $attributes, $category, $objectStorage= "")
    {
        $data = session::get('FolderWidgetListing::folderList');

        // Prepare filter, strip out regex regex related chars, except for *
        $filter = preg_replace('/\*/','____STAR____', $filter);
        $filter = preg_quote($filter,'/');
        $filter = preg_replace('/____STAR____/','.*', $filter);

        // Detect starting position in folder tree.
        $items = $data['recursive'];
        $parent = NULL;
        foreach($data['linear'] as $item){
            if($item['base'] == $base){
                $items = $item['children'];
                $parent = $item['parentPath'];
                break;
            }
        }

        // Add entries
        $ret = array();
        if(isset($data['linear'][$parent]) && !empty($parent)){
            $item = $data['linear'][$parent];
            $item['name'] = '..';
            filterFolderWidget::addEntry($ret, $item, FALSE, $filter);
        }
        foreach($items as $item){
            filterFolderWidget::addEntry($ret, $item, $scope == 'sub', $filter);
        }
    
        return($ret);
    }

    static function addEntry(&$ret, $item, $recursive = FALSE, $filter)
    {
        if(preg_match("/".$filter."/",$item['name']) && $item['status'] != 'removed'){
            $entry = array();
            $entry['dn'] = $item['base'];
            $entry[] = 'dn';
            $entry['cn'] = array($item['name'], 'count' => 1);
            $entry[] = 'cn';
            $entry['folderType'] = array($item['folderType'], 'count' => 1);
            $entry[] = 'folderType';
            $entry['objectClass'] = array('FAKE_OC_Groupware_'.$item['folderType'],'count' => 1);
            $entry[] = 'objectClass';
            $entry['count'] = 4;
            $ret[] = $entry;
        }

        if($recursive && isset($item['children']) && count($item['children'])){
            foreach($item['children'] as $item){
                filterFolderWidget::addEntry($ret, $item, $recursive,$filter);
            }
        }
    }
}
?>
