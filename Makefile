BINARIES=run-tests go-susi encrypt decrypt

all: $(BINARIES)

debug:
	rm -f run-tests
	go build -a -gcflags '-N -l' main/run-tests.go

%: main/%.go
	go build $<
	strip $@

clean:
	rm -f $(BINARIES)
			