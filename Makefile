build: gen
	go build -o wukuard

build-linux-amd64: gen
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o wukuard-linux-amd64

gen:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative grpc/wukuard.proto
