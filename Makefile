BINARIES=run-tests go-susi encrypt decrypt

all:
	go build main/run-tests.go
	go build main/go-susi.go
	go build main/encrypt.go
	go build main/decrypt.go
	strip $(BINARIES)
	ln -snf go-susi gosa-si-server

debug:
	rm -f run-tests
	go build -a -gcflags '-N -l' main/run-tests.go

test: all
	./run-tests --unit --system=./go-susi

clean:
	rm -f $(BINARIES) gosa-si-server
