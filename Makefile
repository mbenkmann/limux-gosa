BINARIES=run-tests go-susi encrypt decrypt

all:
	for f in $(BINARIES) ; do echo Building $$f...; go build main/$$f.go ; done
	strip $(BINARIES)
	ln -snf go-susi gosa-si-server

debug:
	rm -f run-tests
	go build -a -gcflags '-N -l' main/run-tests.go

test: all
	./run-tests --unit --system=./go-susi

clean:
	rm -f $(BINARIES) gosa-si-server
