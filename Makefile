.PHONY: default all cmd rr diode1 diode2 bjt clean

default: all
all: cmd rr diode1 diode2 bjt

BINARY_DIR := bin

cmd:
	go build -o $(BINARY_DIR)/ ./$@

rr:
	go build -o $(BINARY_DIR)/ ./examples/$@

diode1:
	go build -o $(BINARY_DIR)/ ./examples/$@

diode2:
	go build -o $(BINARY_DIR)/ ./examples/$@

bjt:
	go build -o $(BINARY_DIR)/ ./examples/$@

clean:
	rm -rf $(BINARY_DIR)/*.exe
	rm -rf $(BINARY_DIR)/*.log
