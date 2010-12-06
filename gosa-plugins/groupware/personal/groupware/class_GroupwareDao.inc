<?php


/*! \brief Data Access Object for groupware
 */
class GroupwareDao{

    private $availableMethods;
    private $availableProperties;
    private $accountLocations;

    private $groupwarePluginRef = FALSE;
	
    private static $gwFeatures  = array(
            "primaryMail" => array(	"get"=>"gwAcctGetPrimaryMailAddress", 
                "save"=>"gwAcctSetPrimaryMailAddress"),
            "mailLocations" => array("get"=>"gwGetMailboxLocations"),
            "quotaSize"  =>array(  "get"=>"gwAcctGetQuota", 
                "save"=>"gwAcctSetQuota",
                "delete"=>"gwAcctSetQuota"),
            "mailFilter"  =>array(  "get"=>"gwAcctGetFilters", 
                "save"=>"gwAcctSetFilters",
                "delete"=>"gwAcctDelFilter"),
            "alternateAddresses"  =>array(  "get"=>"gwAcctGetAlternateMailAddresses", 
                "save"=>"gwAcctSetAlternateMailAddresses",
                "delete"=>"gwAcctDelAlternateMailAddress"),
            "forwardingAddresses"  =>array( "get"=>"gwAcctGetMailForwardAddresses", 
                "save"=>"gwAcctSetMailForwardAddresses",
                "delete"=>"gwAcctDelMailForwardAddress"),
            "vacationMessage"  =>array(  	"get"=>"gwAcctGetOutOfOfficeReply", 
                "save"=>"gwAcctSetOutOfOfficeReply",
                "delete"=>"gwAcctDelOutOfOfficeReply"),
            "mailBoxWarnLimit"  =>array(  	"get"=>"", 
                "save"=>"",
                "delete"=>""),
            "mailBoxSendSizelimit"  =>array(  "get"=>"gwAcctGetQuota", 
                    "save"=>"gwAcctSetQuota",
                    "delete"=>"gwAcctDelQuota"),
            "mailBoxHardSizelimit"  =>array(  "get"=>"gwAcctGetMailLimit", 
                    "save"=>"gwAcctSetMailLimit",
                    "delete"=>"gwAcctDelMailLimit"),
            "mailBoxAutomaticRemoval"  =>array(  "get"=>"", 
                    "save"=>"",
                    "delete"=>""),
            "localDeliveryOnly"  =>array(  "get"=>"", 
                    "save"=>"",
                    "delete"=>""),
            "dropOwnMails"  =>array(  "get"=>"",
                    "save"=>"",
                    "delete"=>""),
            "accountProperties" => array("get"=>"gwAcctGetProperties",
                    "save"=>"gwAcctSetProperties",
                    "delete"=>"gwAcctDelProperties"),
            //mailFolder is a composite feature - 
            "mailFolder" => array( "get"=>"gwFolderList",
                    "save"=>"gwFolderAdd",
                    "delete"=>"gwFolderDel"),
            "private_folderACLS" => array("get"=>"gwFolderGetMembers",
                    "save"=>"gwFolderSetMembers",
                    "delete"=>"gwFolderDelMember"),
            "mailLimit" => array("get"=>"gwAcctGetMailLimit",
                    "save"=>"gwAcctSetMailLimit"),
					
			//function for speedup
			"folderListWithMembers" => array(
						"get"=>"gwFolderListWithMembers"),
			"comprehensiveUser" => array(
                        "get"=>'gwAcctGetComprehensiveUser'),
			"folderACLs" 		=> array(
                        "get"=>'gwFolderGetMember',
						"save"=>'gwFolderSetMembers')
                );


    /*! \brief Constructor sets the connection to the rpc service 
     * 		initializes the class
     */
    function __construct(&$pluginRef)
    {
        $this->groupwarePluginRef = &$pluginRef;
        $this->init();
    }


    /*! \brief Gets the capabilities of the server
     *          builds an array with availbale features and knows how to call get, save, delete functions of 
     *          groupware rpc. 
     */
    public function init()
    {
        $this->availableMethods = $this->groupwarePluginRef->rpcExec('gwGetCapabilities');
    }


    /*! \brief Generic saving method for all features defined in $gwFeatures
     *          which are the available features.
     */
    public function save($feature, $uid, $valueArray = null)
    {
        $function = GroupwareDao::$gwFeatures[$feature]["save"];
        if(is_array($valueArray)){
            $valueArray = array_merge(array($function, $uid), $valueArray);
        }else{
            if(!empty($valueArray)){
				//add if single value
				$valueArray = array($function, $uid, $valueArray);
			}
			else{
				//omit otherwise
				$valueArray = array($function, $uid);
			}
        }
		//TODO: Remove the debug output
		//echo "----call_user_func_array:<pre>";
		//var_dump($valueArray);
		//echo "</pre>";
        $result =  call_user_func_array(array($this->groupwarePluginRef, 'rpcExec'), $valueArray);
        return $result;
    }


    /*! \brief generic get method for all features defined in $gwFeatures
     * 		which are the available features.
     */
    public function get($feature, $valueArray)
    {		
        $function = GroupwareDao::$gwFeatures[$feature]["get"];
        if(is_array($valueArray)){
            $valueArray = array_merge(array($function), $valueArray);
        }else{
            $valueArray = array($function, $valueArray);
        }

        $result =  call_user_func_array(array($this->groupwarePluginRef, 'rpcExec'), $valueArray);
        return $result;
    }


    /*! \brief generic delete function for all features defined in $gwFeatures
     * 		which are the available features.
     * 
     * NOT YET IMPLEMENTED
     */
    public function del($feature, $uid, $valueArray = "")
    {
		//just debugs
		echo "Feature:".$feature;
		echo "uid:".$uid;
		echo "array:".$valueArray;
	
        $function = GroupwareDao::$gwFeatures[$feature]["delete"];
        if(is_array($valueArray)){
			//merge if array
            $valueArray = array_merge(array($function, $uid), $valueArray);
        }else{
			if(!empty($valueArray)){
				//add if single value
				$valueArray = array($function, $uid, $valueArray);
			}
			else{
				//omit otherwise
				$valueArray = array($function, $uid);
			}
        }
		
        $result =  call_user_func_array(array($this->groupwarePluginRef, 'rpcExec'), $valueArray);
        return $result;
    }

    /*! \brief determine availability of features.
     * 
     * @param methodName
     * @return boolean
     */
    public function gwFeatureAvailable($methodName)
    {
        return(isset($this->availableMethods[$methodName]) && ($this->availableMethods[$methodName]));
    }

    /*! \brief comprehensive user info will dessolve into one groupware call later
     * 		right now it will get all data of the features that are available in the plugin.
     * @param int $uid
     * @return assoc array "feature" => "gwValue";   
     */
    public function getComprehensiverUser( $uid)
    {
		if($this->groupwarePluginRef->isFeatureEnabled("comprehensiveUser")){
			return $this->getComprehensiverUserCompact($uid);
		}
		else{
			return $this->getComprehensiverUserSeparate($uid);
		}
	}
	private function getComprehensiverUserCompact($uid){
		
		$resultArr = array();
		
		if($this->groupwarePluginRef->isFeatureEnabled("mailLocations")){
            $resultArr["mailLocations"] = $this->groupwarePluginRef->rpcExec('gwGetMailboxLocations');
        }
		$comp = $this->get("comprehensiveUser", array($uid));
		$resultArr["primaryMail"] = $comp["primaryMail"];
		$resultArr["alternateAddresses"] = $comp["alternateAddresses"];
		
		//This dao will only accept the first forwardingAddress.
		$resultArr["forwardingAddresses"] = array(key($comp["forwardingAddresses"]));           
	    var_dump($resultArr["forwardingAddresses"]);
					
		$resultArr["mailLocation"] = $comp["mailLocation"];
		//Quota things.
		$resultArr["quotaSize"] = $comp["quota"]["usage"];
		if(!empty($comp["quota"]["warn_limit"])){
			$resultArr["mailBoxWarnLimitEnabled"] = TRUE;
		}
		else{
			$resultArr["mailBoxWarnLimitEnabled"] = FALSE;
		}
		$resultArr["mailBoxWarnLimitValue"] = $comp["quota"]["warn_limit"];

		if(!empty($comp["quota"]["send_limit"])){
			$resultArr["mailBoxSendSizelimitEnabled"] = TRUE;
		}
		else{
			$resultArr["mailBoxSendSizelimitEnabled"] = FALSE;
		}
		
		$resultArr["mailBoxSendSizelimitValue"] = $comp["quota"]["send_limit"];
		if(!empty($comp["quota"]["hard_limit"])){
			$resultArr["mailBoxHardSizelimitEnabled"] = TRUE;
		}
		else{
			$resultArr["mailBoxHardSizelimitEnabled"] = FALSE;
		}
		$resultArr["mailBoxHardSizelimitValue"] = $comp["quota"]["hard_limit"];	

		//Mail Limits
		if(!empty($comp["mailLimits"]["receive"])){
			$resultArr["mailLimitReceiveEnabled"] = TRUE;
		}
		else{
			$resultArr["mailLimitReceiveEnabled"] = FALSE;
		}
		$resultArr["mailLimitReceiveValue"] = $comp["mailLimits"]["receive"];
		if(!empty($comp["mailLimits"]["send"])){
			$resultArr["mailLimitSendEnabled"] = TRUE;
		}
		else{
			$resultArr["mailLimitSendEnabled"] = FALSE;
		}
		$resultArr["mailLimitSendValue"] = $comp["mailLimits"]["send"];
		
		if(empty($comp["vacation"]["message"])){
			$resultArr["vacationEnabled"] = FALSE;
			$resultArr["vacationMessage"] = "";
		}
		else{
			$resultArr["vacationEnabled"] = TRUE;
			$resultArr["vacationMessage"] = $comp["vacation"]["message"];
		}
		
		return $resultArr;
	}	


    /* \brief   Returns a list of mail folders for the current folderPrefix. 
     *          Acl entries will be returned the GOsa-usable way.
     * @return  Array   A list of mail folder entries.
     */   
    function getListOfMailFolders()
    { 
        $prefix = $this->groupwarePluginRef->folderPrefix;
		$compFolders = $this->get("folderListWithMembers", array($prefix));
		return($this->translateGwAclsToGosaAcls($compFolders));
	}

	/* \brief   Returns a list of mail folders for the current folderPrefix. 
     *          Fetches every feature data chunk seperately
     * @return  Array   A list of mail folder entries.
     */   
	private function getComprehensiverUserSeparate($uid){
        if($this->groupwarePluginRef->isFeatureEnabled("primaryMail")){
            $resultArr["primaryMail"] = $this->get("primaryMail", array($uid));
        }

        if($this->groupwarePluginRef->isFeatureEnabled("alternateAddresses")){
            $resultArr["alternateAddresses"] = $this->get("alternateAddresses", array($uid));
        }

        if($this->groupwarePluginRef->isFeatureEnabled("forwardingAddresses")){
            $resultArr["forwardingAddresses"] = $this->get("forwardingAddresses", array($uid));		
        }

        // Location dropdownlist - only readable
        if($this->groupwarePluginRef->isFeatureEnabled("mailLocations")){
            $resultArr["mailLocations"] = $this->groupwarePluginRef->rpcExec('gwGetMailboxLocations');
            $resultArr["mailLocation"] = $this->groupwarePluginRef->rpcExec('gwAcctGetLocation',$uid);
        }

        // Load account folders.
        if($this->groupwarePluginRef->isFeatureEnabled("mailFolder")){

            // Get groupware folders for the current user.
            $prefix = $this->groupwarePluginRef->folderPrefix;
			
			$mFolder = $this->get("mailFolder", array($prefix));
			
			$newFolder = array();
			foreach($mFolder as $key =>$val){
				$newFolder[$val] = $this->get("private_folderACLS", array($val));
			}
			$resultArr["mailFolder"] = $this->translateGwAclsToGosaAcls($newFolder);

            // Add the required status field for each folder.
            foreach($resultArr["mailFolder"] as $id => $folder){
                $resultArr["mailFolder"][$id]['status'] = '';
            }
        }

        // Quota quotaUsage, quotaSize
        if($this->groupwarePluginRef->isFeatureEnabled("quotaSize") || 
                $this->groupwarePluginRef->isFeatureEnabled("quotaUsage") || 
                $this->groupwarePluginRef->isFeatureEnabled("mailBoxWarnLimit") || 
                $this->groupwarePluginRef->isFeatureEnabled("mailBoxSendSizelimit") || 
                $this->groupwarePluginRef->isFeatureEnabled("mailBoxHardSizelimit")){

            // example Array for Quota
            $quota = $this->get("quotaSize", array($uid));

            $resultArr["quotaSize"] = $quota["usage"];
            $resultArr["quotaSize"] = $quota["hold"];
            if($quota["warn_limit"] > 0){
                $resultArr["mailBoxWarnLimitEnabled"] = TRUE;
                $resultArr["mailBoxWarnLimitValue"] = $quota["warn_limit"];
            }else{
                $resultArr["mailBoxWarnLimitEnabled"] = FALSE;
            }
            if($quota["send_limit"] > 0){
                $resultArr["mailBoxSendSizelimitEnabled"] = TRUE;
                $resultArr["mailBoxSendSizelimitValue"] = $quota["send_limit"];
            }else{
                $resultArr["mailBoxSendSizelimitEnabled"] = FALSE; 
            }
            if($quota["hard_limit"] > 0){
                $resultArr["mailBoxHardSizelimitEnabled"] = TRUE;
                $resultArr["mailBoxHardSizelimitValue"] = $quota["hard_limit"];	
            }else{
                $resultArr["mailBoxHardSizelimitEnabled"] = FALSE;
            }
        }

        // Load account limitations 
        if($this->groupwarePluginRef->isFeatureEnabled("mailLimit")){
            $mailLimits = $this->get("mailLimit", array($uid));

            if($mailLimits["receive"] > 0){
                $resultArr["mailLimitReceiveEnabled"] = TRUE;
                $resultArr["mailLimitReceiveValue"] = $mailLimits["receive"];
            }else{
                $resultArr["mailLimitReceiveEnabled"] = FALSE;
            }
            if($mailLimits["send"] > 0){
                $resultArr["mailLimitSendEnabled"] = TRUE;
                $resultArr["mailLimitSendValue"] = $mailLimits["send"];
            }else{
                $resultArr["mailLimitSendEnabled"] = FALSE;
            }
        }		

        #FIXME Validate the vacation message handling, seems to be broken on the server side.
        if($this->groupwarePluginRef->isFeatureEnabled("vacationMessage")){
            $vacArray = $this->get("vacationMessage", array($uid));
            $resultArr["vacationMessage"] = $vacArray["message"];
            $resultArr["vacationEnabled"] = !empty($resultArr["vacationMessage"]);    
        }
        return $resultArr;
	
    }
	
	/* \brief   Translates Groupware folderAcls to Gosa widget Acls.
     * @return  Array   list of Groupware Members with rights to folders.
     */  
	private function translateGwAclsToGosaAcls($compFolders){
		$resultArr = array();
		foreach ($compFolders as $fName => $fMembers) {
			$resultArr[$fName]["name"] = preg_replace("/^.*\//","",$fName);
            $resultArr[$fName]["status"] = "";
			$resultArr[$fName]["acls"] = array();
			foreach($fMembers as $fMember => $fAcl){
				 $aclLine = array(
					"name" => $fMember, 
                    "type"=>"user"
					);
				if(empty($fAcl)){
					$aclLine["acl"] = GROUPWARE_RIGHTS_NONE;
				}
				else{
					$aclLine["acl"] = $fAcl;
					
				}
				$resultArr[$fName]["acls"][] = $aclLine;
			}
		}
		return $resultArr;
	}
}
?>
