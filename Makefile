.PHONY: all

all: build

fmt:
	gofmt -s -w .
	go mod tidy

build: grpc fmt
	mkdir -p bin
	go build -ldflags "-s -w" \
		-o bin/svarog_messenger.bin \
		messenger/main/messenger_main.go
	go build -ldflags "-s -w" \
		-o bin/svarog_peer.bin \
		peer/main/peer_main.go
	cp scripts/deploy.sh bin/deploy.sh
	cp scripts/deploy_tee.sh bin/deploy_tee.sh
	chmod +x bin/*.sh

proto: grpc
grpc:
	mkdir -p pb
	protoc svarog.proto \
		--go_out=pb \
		--go-grpc_out=pb \
		--go_opt=paths=source_relative \
		--go-grpc_opt=paths=source_relative