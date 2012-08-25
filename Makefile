BINARIES=encrypt decrypt go-susi

all: $(BINARIES)

%: main/%.go
	go build $<
	strip $@

clean:
	rm -f $(BINARIES)
			