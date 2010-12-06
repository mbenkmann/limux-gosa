{if $rpcError}
    <h3>{t}Error{/t}</h3>
    {msgPool type=rpcError p1=$error}
    <button name='retryInit'>{t}Retry{/t}</button>
{elseif $initFailed}
    <h3>{t}Communication failed{/t}</h3>
    {msgPool type=rpcError p1=$error}
    <button name='retryInit'>{t}Retry{/t}</button>
{elseif $invalidInstallMethod}
    <h3>{t}Configuration error{/t}</h3>
    {msgPool type=rpcError p1=$error}
{/if}
