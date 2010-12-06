<?php

/*! \brief  A template engine used to render dynamic template
 *           for the GOsa-devices.
 */
class TemplateEngine
{
    private $config;
    private $data = array();    
    private $template = "";
    private $itemType = '';
    private $widgets = array();


    /*! \brief  Constructs the template engine. 
     *  @param  Config  The GOsa configuration object.
     */
    function __construct($config)
    {
        $this->config = &$config;
    }


    /*! \brief  Load/Sets the instruction-set to use for the current
     *           device configuration.
     *          A device configruation tells us what options 
     *           an item can have and what children.
     *  @param  Array   The instruction set to use.
     */
    function load($array)
    {
        $this->data = $array;
    }


    /*! \brief  Set the template which will be used to generate 
     *           the HTML content for this configuration session.
     *  @param  String  A template filename.
     */
    function setTemplate($tmpl)
    {
        $this->template = $tmpl;
    }


    /*! \brief  Returns the list of widgets which are currently used 
     *           by the template engine to render the plugin.
     *  @return Array  A list of widgets.
     */
    function getWidgets()
    {
        return($this->widgets);
    }     


    /*! \brief  Sets the current item type we want to render
     *          E.g.  'KickstartTemplate' and the corresponding values.
     *
     *  @param  String  The name of the item we want to render now.
     *  @param  Array   The initial value.
     */
    function setValues($name, $values)
    {
        // Set the current item type and reset the widget list.
        $this->itemType = $name;
        $this->widgets = array();

        // Do nothing if something seems to be wrong. 
        if(!isset($this->data[$this->itemType])){
            echo "Undefined item type '{$name}'!<br>";
            return;
        }

        // Get the options provided by the item and create widget for them.
        $data = $this->data[$this->itemType];
        if(isset($data['options']) && count($data['options'])){
            foreach($data['options'] as $name => $item){
                $widgetClassName = "TemplateWidget_{$item['type']}";

                // Check if the widget is available, if it is not, use a default (string).
                if(!class_available($widgetClassName)){
                    echo "Unknown widget class {$widgetClassName}! Falling back to default widget.<br>";
                    $widgetClassName = "TemplateWidget_string";
                }

                // Prepare the value for the widget 
                $value = $values[$name];
                $syntax = (isset($item['syntax']))? $item['syntax']: "";
                $providedValues = (isset($item['values']))? $item['values']: array();
   
                // Create the new widget.
                $this->widgets[$name] = new $widgetClassName($this->config, $name, 
                        $value,
                        $item['description'],
                        $syntax,
                        $item['required'],
                        $item['type'],
                        $item['display'],
                        $providedValues);
            }
        }
    }

    
    function execute(){
        return($this->render());
    }
    

    /*! \brief  Creates the HTML content for the given list of widgets
     *  @return String  The HTML content.
     */
    function render()
    {
        $smarty = get_smarty();
        $smarty->assign("type", $this->itemType);

        // Tell smarty the HTML-content for each widget and the name that should be
        //  displayed.
        foreach($this->widgets as $widget){
            $smarty->assign($widget->getName(), $widget->render());
            $smarty->assign($widget->getName()."Name", $widget->getDisplayName());
        }
        $template = $smarty->fetch(get_template_path("goto/Config/{$this->template}", TRUE));

        $smarty->assign('template', $template);
        return($smarty->fetch(get_template_path("goto/Config/TemplateEngine.tpl", TRUE)));
    }


    /*! \brief  Keep track of posted values.
     */
    function save_object()
    {
        foreach($this->widgets as $widget){
            $widget->save_object();
        }
    }


    /*! \brief  Check widget values and return a list of errors.
     */
    function check()
    {
        $msgs = array();
        foreach($this->widgets as $widget){
            $msgs = array_merge($msgs, $widget->check());
        }
        return($msgs);
    }
}


?>
