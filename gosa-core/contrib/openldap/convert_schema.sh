#!/bin/sh
for i in "$@" ; do
  case "$i" in
    *.schema)
    ;;
    *) echo "$i does not have .schema extension => Skip"
       continue
    ;;
  esac

  test -f "${i%.schema}.ldif" && { echo "${i%.schema}.ldif exists => Skip" ; continue; }

  echo "Converting ${i} => ${i%.schema}.ldif"

  sed -rze '
s/\n[\t ]+/ /g
s/\n([\t ]*\n)+/\n#\n/g
s/\nattributetype[ \t]+/\nolcAttributeTypes: /g
s/\nobjectclass[ \t]+/\nolcObjectClasses: /g
s/\nolc/\ndn: cn=<inserthere>,cn=schema,cn=config\nobjectClass: olcSchemaConfig\ncn: <inserthere>\nolc/
' "$i" | sed "s/<inserthere>/${i%.schema}/g" >"${i%.schema}.ldif"
done 
