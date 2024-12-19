.PHONY: all mac linux windows clean test

PROTOC=protoc
GOOS=linux
GOARCH?='$(ARCH)'
LDFLAGS='-w -extldflags "-static"'

TAGS=netgo

.PHONY: all
all: test mac linux windows

.PHONY: mac
mac:
	GOOS=darwin GOARCH=$(GOARCH) CGO_ENABLED=0 go build -mod vendor -ldflags $(LDFLAGS) -tags $(TAGS) -o build/cloudcutter ./cmd/cloudcutter
	
.PHONY: linux
linux:
	GOOS=linux GOARCH=$(GOARCH) CGO_ENABLED=0 go build -mod vendor -ldflags $(LDFLAGS) -tags $(TAGS) -o build/cloudcutter ./cmd/cloudcutter
	
.PHONY: windows
windows:
	GOOS=windows GOARCH=$(GOARCH) CGO_ENABLED=0 go build -mod vendor -ldflags $(LDFLAGS) -tags $(TAGS) -o build/cloudcutter ./cmd/cloudcutter


.PHONY: test-unit
test-unit:
	go test -coverprofile=cover.out -mod vendor ./...
	# vet requires gcc to be installed.
	CGO_ENABLED=1 go test -mod vendor -race ./...

.PHONY: cover
cover: test-unit
	go tool cover -html=cover.out

.PHONY: clean
clean:
	go clean ./...
	rm -rf ./build

.PHONY: install-tools
install-tools:
	@echo Installing tools from _tools.go
	@cat ./tools/tools.go | grep _ | awk -F'"' '{print $$2}' | xargs -tI % go install %

.PHONY: codecheck
codecheck:
	golint ./cmd/...
	golint ./internal/...

	gofmt -s -w ./cmd/
	gofmt -s -w ./internal/

	go vet ./cmd/...
	go vet ./internal/...


.PHONY: test
test: test-unit 
