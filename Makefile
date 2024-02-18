BINDIR := $(CURDIR)/bin

test:
	go test ./... -shuffle=on -race

lint:
	go vet  ./...

test.cover:
	go test -race -shuffle=on -coverprofile=coverage.txt -covermode=atomic ./...

# For local environment
cov:
	go test -cover -coverprofile=cover.out
	go tool cover -html=cover.out -o cover.html
