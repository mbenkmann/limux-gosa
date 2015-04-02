GOsa² 2.8
=========

GOsa² is a web based adminstration tool for user accounts, groups, servers, clients, and many other things.
It integrates with FAI (fully automatic installation) and PXE (pre-boot execution environment) to provide unattended
installation and updates of operating systems derived from Debian (such as Ubuntu, Linux Mint).
Configuration data is stored in a database that supports LDAP (such as OpenLDAP).

GOsa² was developed by GONICUS GmbH. Development of the 2.x series has been been
discontinued by GONICUS in favor of the new 3.x series which is a major redesign.

GOsa² 2.8 continues development of the 2.x series where GONICUS left off. Aside from many
bug fixes and a complete rewrite of the horribly buggy gosa-si daemon, GOsa² 2.8 also adds all
of the backend components necessary for a turnkey solution to unattended installation of
Debian-based operating systems.

gosa-quickstart
===============

gosa-quickstart is a package that contains a script and configuration files to turn a plain Ubuntu or Debian server
installation into a complete GOsa²+FAI+LDAP+DNS+DHCP server for managing user accounts and unattended installations
of Debian and Ubuntu systems. It is the perfect starting point for your own GOsa²-managed infrastructure.

Individual parts of gosa-quickstart can be disabled or executed selectively via command line switches. This allows
you to use different machines for different functions (e.g. a dedicated DNS-server) or to omit setting up those
parts of the infrastructure that you already have (such as a DNS server).

