.PHONY: default all spice rr diode1 diode2 bjt clean

default: all
all: spice rr diode1 diode2 bjt

BINARY_DIR := bin

spice:
	go build -o $(BINARY_DIR)/ ./cmd/$@

rr:
	go build -o $(BINARY_DIR)/ ./cmd/examples/$@

diode1:
	go build -o $(BINARY_DIR)/ ./cmd/examples/$@

diode2:
	go build -o $(BINARY_DIR)/ ./cmd/examples/$@

bjt:
	go build -o $(BINARY_DIR)/ ./cmd/examples/$@

clean:
	rm -rf $(BINARY_DIR)/*.exe
	rm -rf $(BINARY_DIR)/*.log
