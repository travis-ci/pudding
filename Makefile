PACKAGE := github.com/travis-ci/pudding
SUBPACKAGES := \
	$(PACKAGE)/cmd/pudding-server \
	$(PACKAGE)/cmd/pudding-workers \
	$(PACKAGE)/db \
	$(PACKAGE)/server \
	$(PACKAGE)/server/jsonapi \
	$(PACKAGE)/server/negroniraven \
	$(PACKAGE)/workers

VERSION_VAR := $(PACKAGE)/pudding.VersionString
VERSION_VALUE ?= $(shell git describe --always --dirty --tags 2>/dev/null)
REV_VAR := $(PACKAGE)/pudding.RevisionString
REV_VALUE ?= $(shell git rev-parse --sq HEAD 2>/dev/null || echo "'???'")
GENERATED_VAR := $(PACKAGE)/pudding.GeneratedString
GENERATED_VALUE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%S%z')

FIND ?= find
GO ?= go
DEPPY ?= deppy
GOPATH := $(shell echo $${GOPATH%%:*})
GOBUILD_LDFLAGS ?= -ldflags "\
	-X $(VERSION_VAR)='$(VERSION_VALUE)' \
	-X $(REV_VAR)=$(REV_VALUE) \
	-X $(GENERATED_VAR)='$(GENERATED_VALUE)' \
"
GOBUILD_FLAGS ?= -x

PORT ?= 42151
export PORT

COVERPROFILES := \
	db-coverage.coverprofile \
	server-coverage.coverprofile \
	server-jsonapi-coverage.coverprofile \
	server-negroniraven-coverage.coverprofile \
	workers-coverage.coverprofile

%-coverage.coverprofile:
	$(GO) test -covermode=count -coverprofile=$@ \
		$(GOBUILD_LDFLAGS) $(PACKAGE)/$(subst -,/,$(subst -coverage.coverprofile,,$@))

.PHONY: all
all: clean deps test

.PHONY: buildpack
buildpack:
	@$(MAKE) build \
		GOBUILD_FLAGS= \
		REV_VALUE="'$(shell git log -1 --format='%H')'" \
		VERSION_VALUE=buildpack-$(STACK)-$(USER)-$(DYNO)

.PHONY: test
test: build fmtpolice test-deps coverage.html

.PHONY: test-deps
test-deps:
	$(GO) test -i $(GOBUILD_LDFLAGS) $(PACKAGE) $(SUBPACKAGES)

# .PHONY: test-race
# test-race:
# 	$(GO) test -race $(GOBUILD_LDFLAGS) $(PACKAGE) $(SUBPACKAGES)

coverage.html: coverage.coverprofile
	$(GO) tool cover -html=$^ -o $@

coverage.coverprofile: $(COVERPROFILES)
	./bin/fold-coverprofiles $^ > $@
	$(GO) tool cover -func=$@

.PHONY: build
build:
	$(GO) install $(GOBUILD_FLAGS) $(GOBUILD_LDFLAGS) $(PACKAGE) $(SUBPACKAGES)

.PHONY: deps
deps:
	$(GO) get -t $(GOBUILD_FLAGS) $(GOBUILD_LDFLAGS) $(PACKAGE) $(SUBPACKAGES)

.PHONY: clean
clean:
	./bin/clean

.PHONY: annotations
annotations:
	@git grep -E '(TODO|FIXME|XXX):' | grep -v Makefile

.PHONY: save
save:
	$(DEPPY) save ./...

.PHONY: fmtpolice
fmtpolice:
	./bin/fmtpolice

lintall:
	./bin/lintall
