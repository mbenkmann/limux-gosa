BINARIES=run-tests go-susi encrypt decrypt sibridge

all:
	main/makeversion
	go build main/run-tests.go
	go build main/go-susi.go
	go build main/encrypt.go
	go build main/decrypt.go
	go build main/sibridge.go
	strip $(BINARIES)
	ln -snf go-susi gosa-si-server

frankensusi:
	main/frankensusi $$(find . /srv/www -name "gosa-si-server_*_i386.deb" -print)

debug:
	rm -f run-tests
	go build -a -gcflags '-N -l' main/run-tests.go

test: all
	./run-tests --unit --system=./go-susi

almostclean:
	rm -f $(BINARIES) gosa-si-server go-susi_*.deb
	rm -f testdata/ldif/c=de/o=go-susi/ou=incoming/cn=*.ldif
	rm -f testdata/ldif/c=de/o=go-susi/ou=systems/ou=workstations/cn=system-aa-00-bb-11-cc-99.ldif
	rm -f testdata/ldif/c=de/o=go-susi/ou=systems/ou=workstations/cn=mrhyde.ldif
	rm -f deb/go-susi*.orig.tar.gz deb/go-susi*.deb deb/go-susi*.dsc
	rm -f deb/go-susi*.changes deb/go-susi*.diff.gz deb/go-susi-?*.?*.?*/.hg*
	rm -rf deb/go-susi-?*.?*.?*/*
	test -d deb/go-susi-?*.?*.?*/ && rmdir deb/go-susi-?*.?*.?*/ || true
	test -d deb && rmdir deb || true

clean:  almostclean
	hg revert --no-backup testdata/ldif

deb:
	main/makedebsource
	cd deb/go-susi-*/ && dpkg-buildpackage -rfakeroot -sa -us -uc
