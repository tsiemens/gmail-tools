CURDIR=$(shell pwd)
export GOPATH=$(shell echo $$(readlink -f $(CURDIR)/../../../..))

PLUGINSDIR=$(HOME)/.gmailcli/plugins

build:
	mkdir -p bld
	go build -buildmode=plugin -o bld/pluginscore.so ./pluginscore
	mkdir -p $(PLUGINSDIR)
	test -L $(PLUGINSDIR)/pluginscore.so || \
		ln -s $(CURDIR)/bld/pluginscore.so $(PLUGINSDIR)/pluginscore.so
	go build -o bld/gmailcli main.go

getdeps:
	go get -u google.golang.org/api/gmail/v1
	go get -u golang.org/x/oauth2/...
	go get -u github.com/spf13/cobra/cobra
	go get -u github.com/google/shlex
	go get -u github.com/tsiemens/go-concurrentMap
	go get -u github.com/golang-collections/collections
	go get -u gopkg.in/yaml.v2

clean:
	rm bld/gmailcli

test:
	go test ./filter
	go test ./test

.PHONY: clean test
