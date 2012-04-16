{t}The members checked in the list below will inherit all settings from this object group{/t}:

{$list}

<script>
  var check_all = $('member-checkbox-all');
  Event.observe(check_all, 'click', function () {
    var checkboxes = $$('.member-checkbox');
    for (var i=0; i < checkboxes.length; i++) {
      checkboxes[i].checked = check_all.checked;
    };
  });
</script>
