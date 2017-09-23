export GOPATH=$(shell echo $$(readlink -f $$(pwd)/../../../..))

build:
	mkdir -p bld
	go build -o bld/gmailcli main/gmailcli.go

getdeps:
	go get -u google.golang.org/api/gmail/v1
	go get -u golang.org/x/oauth2/...

clean:
	rm bld/gmailcli

.PHONY: clean
