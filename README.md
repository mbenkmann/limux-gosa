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

go-susi
=======
go-susi is a replacement for gosa-si-server and gosa-si-client. go-susi has been written from scratch and has none of the performance and quality issues of its predecessors.

Complete documentation of go-susi and the XML-based protocol GOsa² uses to communicate with it can be found in the
[Operator's Manual](https://docs.google.com/document/d/17_j8s2-PBVJLaQzmjrdeh6CVWkzP_viA3Uo7AtFmt8c/view) and the manpages included with the Debian packages (start reading at go-susi(1)).


Status
======

GOsa² 2.8 works as well or better than GOsa² 2.7 in all respects. go-susi works unequivocally better than gosa-si-server/client. GOsa² 2.7 installations can be upgraded to GOsa² 2.8 without restrictions.

gosa-quickstart is still in its infancy. It has the following limitations compared to the description above:
* only works on Ubuntu Trusty. Debian Jessie support will come later.
* No DNS or DHCP included. You need to set up both services yourself.
* Steps can not bet disabled or executed separately. The script will always set up everything on the machine where it is run.
* OS installation is not implemented. But you can already create and edit server and workstation objects in GOsa².
