quick:
	go build -o datadt

build: quick

build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 make build -o datadt