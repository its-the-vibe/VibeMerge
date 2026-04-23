.PHONY: build test lint clean tidy

BINARY_NAME=vibemerge

build:
	go build -o $(BINARY_NAME) .

test:
	go test -v ./...

lint:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BINARY_NAME)
