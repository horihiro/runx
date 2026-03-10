.PHONY: build build-linux build-windows build-mac clean install

# Default target
build:
	go build -o runx main.go

# Linux build
build-linux:
	GOOS=linux GOARCH=amd64 go build -o runx-linux-amd64 main.go

# Windows build
build-windows:
	GOOS=windows GOARCH=amd64 go build -o runx-windows-amd64.exe main.go

# macOS build (Intel)
build-mac-amd64:
	GOOS=darwin GOARCH=amd64 go build -o runx-darwin-amd64 main.go

# macOS build (Apple Silicon)
build-mac-arm64:
	GOOS=darwin GOARCH=arm64 go build -o runx-darwin-arm64 main.go

# Build all platforms
build-all: build-linux build-windows build-mac-amd64 build-mac-arm64

# Clean up
clean:
	rm -f runx runx-* *.exe
