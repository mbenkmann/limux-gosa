BINARIES=run-tests go-susi encrypt decrypt sibridge

susi:
	main/makeversion
	go build main/go-susi.go

all: man
	main/makeversion
	go build main/run-tests.go
	go build main/go-susi.go
	go build main/encrypt.go
	go build main/decrypt.go
	go build main/sibridge.go
	strip $(BINARIES)
	ln -snf go-susi gosa-si-server

.PHONY: man
man: VERSION=$(shell main/makeversion && sed -n 's/.*Version.*=.*"\([^"]*\)".*/\1/p' config/version.go)
man:
	xsltproc >doc/go-susi.1 --stringparam name GO-SUSI --stringparam section 1 --stringparam start_id "id.jes9sqqlstzn" --stringparam stop_id "id.sj4np8e9wdb8" --stringparam version "$(VERSION)" --novalid --nonet doc/manpage.xsl doc/go-susi-manual.xhtml
	xsltproc >doc/gosa-si-server.1 --stringparam name GOSA-SI-SERVER --stringparam section 1 --stringparam start_id "id.anooi87iwmip" --stringparam stop_id "id.l8s0fislnev7" --stringparam version "$(VERSION)" --novalid --nonet doc/manpage.xsl doc/go-susi-manual.xhtml
	xsltproc >doc/sibridge.1 --stringparam name SIBRIDGE --stringparam section 1 --stringparam start_id "id.ejujf2q7dp37" --stringparam stop_id "id.anooi87iwmip" --stringparam version "$(VERSION)" --novalid --nonet doc/manpage.xsl doc/go-susi-manual.xhtml
	xsltproc >doc/server.conf.5 --stringparam name SERVER.CONF --stringparam section 5 --stringparam start_id "id.ghm1et7cq23q" --stringparam stop_id "id.3yfqzfegrde0" --stringparam version "$(VERSION)" --novalid --nonet doc/manpage.xsl doc/go-susi-manual.xhtml
	xsltproc >doc/gosa-si-jobs.5 --stringparam name GOSA-SI-JOBS --stringparam section 5 --stringparam start_id "id.sj4np8e9wdb8" --stringparam stop_id "id.8rs4yx8or7cf" --stringparam version "$(VERSION)" --novalid --nonet doc/manpage.xsl doc/go-susi-manual.xhtml
	xsltproc >doc/gosa-si-s2s.5 --stringparam name GOSA-SI-S2S --stringparam section 5 --stringparam start_id "id.8rs4yx8or7cf" --stringparam stop_id "id.lcwii6oc1qwv" --stringparam version "$(VERSION)" --novalid --nonet doc/manpage.xsl doc/go-susi-manual.xhtml
	xsltproc >doc/gosa-si-client.5 --stringparam name GOSA-SI-CLIENT --stringparam section 5 --stringparam start_id "id.lcwii6oc1qwv" --stringparam stop_id "id.qz602c68q3ci" --stringparam version "$(VERSION)" --novalid --nonet doc/manpage.xsl doc/go-susi-manual.xhtml
	xsltproc >doc/gosa-si-fai.5 --stringparam name GOSA-SI-FAI --stringparam section 5 --stringparam start_id "id.qz602c68q3ci" --stringparam stop_id "id.vafpk0dntk2q" --stringparam version "$(VERSION)" --novalid --nonet doc/manpage.xsl doc/go-susi-manual.xhtml
	xsltproc >doc/gosa-si-query.5 --stringparam name GOSA-SI-QUERY --stringparam section 5 --stringparam start_id "id.vafpk0dntk2q" --stringparam stop_id "id.5uju4li1h33b" --stringparam version "$(VERSION)" --novalid --nonet doc/manpage.xsl doc/go-susi-manual.xhtml
	xsltproc >doc/gosa-si-misc.5 --stringparam name GOSA-SI-MISC --stringparam section 5 --stringparam start_id "id.5uju4li1h33b" --stringparam stop_id "id.ys29afdlp9ml" --stringparam version "$(VERSION)" --novalid --nonet doc/manpage.xsl doc/go-susi-manual.xhtml
	xsltproc >doc/gosa-si-deprecated.5 --stringparam name GOSA-SI-DEPRECATED --stringparam section 5 --stringparam start_id "id.ys29afdlp9ml" --stringparam stop_id "id.qm42onlc5h25" --stringparam version "$(VERSION)" --novalid --nonet doc/manpage.xsl doc/go-susi-manual.xhtml

.PHONY: doc
doc:
	wget -nv "https://docs.google.com/document/d/17_j8s2-PBVJLaQzmjrdeh6CVWkzP_viA3Uo7AtFmt8c/export?format=pdf" -O doc/go-susi-manual.pdf
	wget -nv "https://docs.google.com/document/d/17_j8s2-PBVJLaQzmjrdeh6CVWkzP_viA3Uo7AtFmt8c/export?format=odt" -O doc/go-susi-manual.odt
	wget -nv "https://docs.google.com/document/d/17_j8s2-PBVJLaQzmjrdeh6CVWkzP_viA3Uo7AtFmt8c/export?format=html" -O - | tidy -quiet -numeric -asxml -indent -o doc/go-susi-manual.xhtml 2>&1 | { grep -v Warning || true ; }
	LC_ALL=C pretty-xml/pretty-xsl doc/manpage.pxsl >doc/manpage.xsl
	
frankensusi:
	main/frankensusi $$(find . /srv/www -name "gosa-si-server_*_i386.deb" -print)

debug:
	rm -f run-tests
	go build -a -gcflags '-N -l' main/run-tests.go

test:
	main/makeversion
	go build main/run-tests.go
	go build main/go-susi.go
	./run-tests --unit --system=./go-susi

almostclean:
	rm -f $(BINARIES) gosa-si-server go-susi_*.deb
	rm -f testdata/ldif/c=de/o=go-susi/ou=incoming/cn=*.ldif
	rm -f testdata/ldif/c=de/o=go-susi/ou=systems/ou=workstations/cn=_aa-00-bb-11-cc-99_.ldif
	rm -f testdata/ldif/c=de/o=go-susi/ou=systems/ou=workstations/cn=mrhyde.ldif
	rm -f deb/go-susi*.orig.tar.gz deb/go-susi*.deb deb/go-susi*.dsc
	rm -f deb/go-susi*.changes deb/go-susi*.diff.gz deb/go-susi-?*.?*.?*/.hg*
	rm -rf deb/go-susi-?*.?*.?*/*
	rm -f manpage.xsl
	test -d deb/go-susi-?*.?*.?*/ && rmdir deb/go-susi-?*.?*.?*/ || true
	test -d deb && rmdir deb || true

clean:  almostclean
	hg revert --no-backup testdata/ldif

deb:
	main/makedebsource
	cd deb/go-susi-*/ && dpkg-buildpackage -rfakeroot -sa -us -uc
