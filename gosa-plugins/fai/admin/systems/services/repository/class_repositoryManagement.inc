<?php

class repositoryManagement extends management
{

  protected $skipFooter = TRUE;
  protected $skipHeader = TRUE;
  protected $parent     = NULL;

  function __construct($config, $parent)
  {
    $this->ui     = get_userinfo();
    $this->parent = $parent;

    // Set storage points - We do not have any - We just create a fake list which lists all services
    $this->storagePoints = array("");

    // Build filter
    if (session::global_is_set(get_class($this)."_filter")){
      $filter= session::global_get(get_class($this)."_filter");
    } else {
      $filter = new filter(get_template_path("services/repository/repository-filter.xml", true));
      $filter->setObjectStorage($this->storagePoints);
    }
    $this->setFilter($filter);

    // Build headpage
    $headpage = new listing(get_template_path("services/repository/repository-list.xml", true));
    $headpage->registerElementFilter("filterSections", "repositoryManagement::filterSections");
    $headpage->setFilter($filter);

    parent::__construct($config, $this->ui, "repositories", $headpage);
    $this->registerAction('edit',    'forwardToParent');
    $this->registerAction('remove',  'forwardToParent');
    $this->registerAction('new',     'forwardToParent');
  }


  /*! \brief    Forwards list action to the parent management class 
   */
  function forwardToParent($action, $target, $all)
  {
    if(!empty($action)){
      $tmp = $action."Entry";
      $this->parent->$tmp($action,$target,$all);  
    }
  }


  /*! \brief    Makes section array printable for the overview list.
   */
  static function filterSections($a,$b)
  {
    if(!$b) return('&nbsp;');
    return(implode($b,', '));
  }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
