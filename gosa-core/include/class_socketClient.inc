<?php
/*
 * This code is part of GOsa (http://www.gosa-project.org)
 * Copyright (C) 2003-2008 GONICUS GmbH
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

class Socket_Client
{
  private $host 	= "";
  private $port 	= "";
  private $timeout= "";
  private $context = NULL;
  private $errno	= "";
  private $errstr	= "";
  private $b_data_send = FALSE;
  private $b_use_tls = FALSE;
  private $b_tls_hands_shaken = FALSE;
  private $handle	= NULL;
  private $bytes_read = 0;
  private $error = "";
  private $is_error   = FALSE;
  private $b_encrypt  = FALSE;

  /* Crypto information */
  private $td= NULL;
  private $ckey= "";
  private $ks;
  private $iv;


  public function __construct($host, $port, $connect = TRUE, $timeout = 3){
    $this->host= $host;
    $this->port= $port;
    $this->timeout= $timeout;
    $opts = array(
               'ssl'=>array(
                 'verify_peer_name'=>FALSE,
                 'allow_self_signed'=>FALSE,
                 'capture_peer_cert'=>TRUE
               )
            );
    $this->context= stream_context_create($opts);
    $this->reset_error();

    /* Connect if needed */
    if($connect){
      $this->open();
    }
  }

  // $key argument is either:
  //   string: old-style encryption password
  //   array: TLS-parameters "ca-certificate", "certificate" and "keyfile" where
  //          each maps to a file path of the respective PEM-encoded certificate/key.
  public function setEncryptionKey($key)
  {
    if (is_array($key)) {
      stream_context_set_option($this->context, 'ssl', 'cafile', $key['ca-certificate']);
      stream_context_set_option($this->context, 'ssl', 'local_cert', $key['certificate']);
      stream_context_set_option($this->context, 'ssl', 'local_pk', $key['keyfile']);
      if($this->handle){
        stream_context_set_option($this->handle, 'ssl', 'cafile', $key['ca-certificate']);
        stream_context_set_option($this->handle, 'ssl', 'local_cert', $key['certificate']);
        stream_context_set_option($this->handle, 'ssl', 'local_pk', $key['keyfile']);
      }
      
      $this->b_encrypt = FALSE; // TLS is already encrypted. Don't double-encrypt.
      $this->b_use_tls = TRUE;
      
    } else {
      $this->b_use_tls = FALSE;
      
      if (is_null($this->td)) {
        /* Open the cipher */
        $this->td = mcrypt_module_open('rijndael-128', '', 'cbc', '');
    
        /* Create the IV and determine the keysize length */
        $this->iv = substr(md5('GONICUS GmbH'),0, mcrypt_enc_get_iv_size($this->td));
        $this->ks = mcrypt_enc_get_key_size($this->td);
      }
      
      if(!function_exists("mcrypt_get_iv_size")){
        $this->set_error(msgPool::missingext("mcrypt"));
        $this->ckey = "";
        $this->b_encrypt = FALSE;
      }

      if ($this->connected()){
        $this->ckey = substr(md5($key), 0, $this->ks);
        $this->b_encrypt = TRUE;
      }
    }
    return($this->b_encrypt);
  }


  private function encrypt($data)
  {
    if($this->b_encrypt){
      mcrypt_generic_init($this->td, $this->ckey, $this->iv);
      $data = base64_encode(mcrypt_generic($this->td, $data));
    } else {
      // If we do not encrypt, we need to get rid of embedded newlines
      // because newline terminates the message
      $data = str_replace("\n", "", $data);
    }
    return($data);
  }


  private function decrypt($data)
  {
    /* decrypt data */
    if($this->b_encrypt && strlen($data)){
      $data = base64_decode($data);
      mcrypt_generic_init($this->td, $this->ckey, $this->iv);
      $data = mdecrypt_generic($this->td, $data);
    }
    return($data);
  }


  public function connected()
  {
    return ($this->handle == TRUE);
  }


  public function open()
  {
    $this->reset_error();
    $this->b_tls_hands_shaken = FALSE;
    $this->handle = @stream_socket_client("tcp://{$this->host}:{$this->port}", $this->errno, $this->errstr, $this->timeout, STREAM_CLIENT_CONNECT, $this->context);
    if(!$this->handle){
      $this->handle = NULL;
      $this->set_error(sprintf(_("Socket connection to %s:%s failed: %s"), bold($this->host), bold($this->port), $this->errstr));
    }else{
      $this->b_data_send = TRUE;
    }
  }


  public function set_error($str)
  {
    $this->is_error =TRUE;
    $this->error=$str;
  }


  public function reset_error()
  {
    $this->is_error =FALSE;
    $this->error = "";
  }


  public function is_error()
  {
    return($this->is_error);
  }


  public function get_error()
  {
    return $this->error;
  }


  public function write($data){
    if($this->handle){
      if ($this->b_use_tls && !$this->b_tls_hands_shaken) {
        fputs($this->handle, "STARTTLS\n");
        $result = stream_socket_enable_crypto($this->handle, TRUE, STREAM_CRYPTO_METHOD_TLS_CLIENT);
        if ($result !== TRUE) {
          $this->set_error('TLS handshake failed');
          $this->b_data_send = FALSE;
          return $this->b_data_send;
        }
        $this->b_tls_hands_shaken = TRUE;
      }
      $data = $this->encrypt($data);
      fputs($this->handle, $data."\n");
      $this->b_data_send = TRUE;
    }else{
      $this->b_data_send = FALSE;
    }

    return $this->b_data_send;
  }


  public function read()
  {
    // Output the request results
    $this->reset_error();
    $str = "";
    $data = "test";
    socket_set_timeout($this->handle,$this->timeout);			
    stream_set_blocking($this->handle,0);
    $start = microtime(TRUE);

    /* Read while 
     * nothing was read yet
     * the timelimit reached
     * there is not data left on the socket.
     */
    while(TRUE){
      usleep(10000);
      $data = fread($this->handle, 1024000);
      if($data && strlen($data)>0) {
        $str .= $data;
      } else {
        if(strlen($str) != 0){
          
          /* We got <xml> but </xml> is still missing, keep on reading */
          if(preg_match("/<\/xml>/",$this->decrypt($str))){
            break;
          }
        }
      }
      if((microtime(TRUE) - $start) > $this->timeout ){     
        $this->set_error(sprintf(_("Socket timeout of %s seconds reached!"), bold($this->timeout)));     
        break;
      }
    }
    $this->bytes_read = strlen($str);
    $this->b_data_send = FALSE;
    $str = $this->decrypt($str);
    return($str);	
  }


  public function bytes_read()
  {
    return $this->bytes_read;
  }


  public function close()
  {
    if($this->handle){
      fclose($this->handle);
    }

    /* Terminate decryption handle. Module is closed automatically at end of script. */
    if ($this->td) {
      mcrypt_generic_deinit($this->td);
    }
  }
}
// vim:tabstop=2:expandtab:shiftwidth=2:filetype=php:syntax:ruler:
?>
