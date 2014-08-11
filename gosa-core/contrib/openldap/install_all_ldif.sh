#!/bin/sh

# some schemas must be installed in a specific order
order="gofax gofon samba3 gosystem goto gosa-samba3 goserver goto-mime trust"

# install order-dependent ldifs first
for ldif in $order ; do
  test -f $ldif.ldif && ldapadd -Y EXTERNAL -H ldapi:/// -f $ldif.ldif
done

# now install all other ldifs
for ldif in *.ldif ; do
  for already_installed in $order ; do
    test "$ldif" = "$already_installed.ldif" && break
  done
  test "$ldif" = "$already_installed.ldif" && continue
  test -f $ldif && ldapadd -Y EXTERNAL -H ldapi:/// -f $ldif
done
