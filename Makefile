.PHONY: build install

build:
	go build -o lr .

install: build
	sudo install -m 755 lr /opt/homebrew/bin/lr
