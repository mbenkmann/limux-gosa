<body>

<div class='logout-box'>
 <h2>{t}Existing session!{/t}</h2>

 <p>
 {t}You have an existing session. Using multiple windows or tabs for a single GOsa session results in data corruption!{/t}
 </p>
 
 <p>
 {t}If you are sure you closed all other GOsa windows/tabs, you can continue using the session.{/t}
 </p>

 <hr>

 <div class='plugin-actions'>
  <center>
   <button type="submit" name="dummy" id="dummy" onclick="window.location='main.php';">{t}Force use of existing session{/t}</button>
  </center>
 </div>
</div>


</body>
</html>
