EXECS   := $(wildcard cmd/*)
TARGETS := ${EXECS:cmd/%=%}

TESTA   := ${shell go list ./... | grep -v /cmd/ }

BRANCH   := ${shell git branch --show-current}
REVCNT   := ${shell git rev-list --count $(BRANCH) --}
REVHASH  := ${shell git log -1 --format="%h"}

GITTAG   := ${shell git tag --points-at HEAD}
ifeq ($(strip $(GITTAG)),)
  RELEASE=untagged
else
  RELEASE=$(GITTAG)
endif

LDFLAGS  := -s -w -X main.version=${BRANCH}.${REVCNT}.${REVHASH} -X main.release=${RELEASE}

all: check clean build

check: gen lint test

cover:
	go test -coverprofile=cover.out ${TESTA} && \
	go tool cover -func=cover.out

gen:
	go generate ./...

lint:
	golangci-lint run ./...

test:
	go test -count 1 ${TESTA}

clean:
	rm -rf bin/*

build: ${TARGETS}
	@echo ":: Done"

${TARGETS}:
	@echo ":: Building $@"
	#CGO_ENABLED=0 go build -ldflags '${LDFLAGS}' -o bin/$@ cmd/$@/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags '${LDFLAGS}' -o bin/$@_linux-amd64_${RELEASE} cmd/$@/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags '${LDFLAGS}' -o bin/$@_linux-arm64_${RELEASE} cmd/$@/main.go

.PHONY: all check cover gen lint test clean build

