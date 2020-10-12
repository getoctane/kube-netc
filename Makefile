.DEFAULT_GOAL := build

IMAGENAME := kube-netc
LD_FLAGS:=-ldflags="-w -s"
BUILD_ARGS := -tags="linux_bpf"
REGISTRY := us.gcr.io/octane-public

IMAGEVERSION ?= latest

recv:
	go build -o recv $(BUILD_ARGS) examples/recv.go

promserv:
	go build -o promserv $(BUILD_ARGS) examples/promserv.go

bps:
	go build -o bps $(BUILD_ARGS) examples/bps.go

bpf:
	go build -o bpf $(BUILD_ARGS) examples/bpf.go

tests:
	go test $(BUILD_ARGS) ./pkg/tracker

build:
	go build $(BUILD_ARGS) -o main main.go

buildBinForDocker:
	GOARCH=amd64 CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo $(LD_FLAGS) $(BUILD_ARGS) -o main .

build-docker:
	docker build -t $(IMAGENAME) -f container/Dockerfile .

run-docker:
	docker run --name kube-netc-server --rm -v /sys/kernel/debug:/sys/kernel/debug -v /sys/fs/cgroup:/sys/fs/cgroup -v /sys/fs/bpf:/sys/fs/bpf --privileged $(IMAGENAME)

push-docker:
	docker build -t $(REGISTRY)/kube-netc:$(IMAGEVERSION) . -f container/Dockerfile
	docker push $(REGISTRY)/kube-netc:$(IMAGEVERSION)

run: build-docker run-docker

lint:
	$(GOPATH)/bin/golangci-lint run ./pkg/tracker/...
	$(GOPATH)/bin/golangci-lint run ./pkg/collector/...
	$(GOPATH)/bin/golangci-lint run ./pkg/cluster/...
	$(GOPATH)/bin/golangci-lint run main.go

format:
	gofmt -w -s ./pkg/tracker
	gofmt -w -s ./pkg/collector
	gofmt -w -s ./pkg/cluster
	gofmt -w -s ./examples
	gofmt -w -s main.go

check: tests build clean lint format

clean:
	go clean
	rm -f recv promserv bps main bpf
