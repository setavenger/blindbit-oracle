# Blindbit Oracle Justfile
# Default recipe (runs when you just type `just`)
default:
    @just build

init:
    #!/usr/bin/env sh
    if [ ! -d ~/.blindbit-oracle ]; then
        mkdir -p ~/.blindbit-oracle
        cp blindbit.example.toml ~/.blindbit-oracle/blindbit.toml
        echo "Created ~/.blindbit-oracle/ and copied config file"
    else
        echo "Directory ~/.blindbit-oracle already exists, skipping init"
    fi

# Build the application
build:
    go build -o blindbit-oracle ./src

# Run the application with development config
run:
    go run ./src

# Run tests
test:
    go test ./...

# Run tests with verbose output
test-verbose:
    go test -v ./...

# Run tests with coverage
test-coverage:
    go test -cover ./...

# Clean build artifacts
clean:
    rm -f blindbit-oracle

clean-config:
    rm -rf ~/.blindbit-oracle/blindbit.toml

# Run linter
lint:
    golangci-lint run

# Format code
fmt:
    go fmt ./...

# Run all checks (test, lint, format)
check: test lint
    @echo "All checks passed!"

# Development workflow: format, test, build
dev: fmt test build
    @echo "Development build complete!"

# Show current configuration
show-config:
    #!/usr/bin/env sh
    if [ -f ~/.blindbit-oracle/blindbit.toml ]; then
        cat ~/.blindbit-oracle/blindbit.toml
    else
        echo "Config file not found at ~/.blindbit-oracle/blindbit.toml"
        echo "Run 'just init' to create it"
    fi

# Docker commands
docker-build:
    docker build -t blindbit-oracle .

docker-run:
    docker run -p 8000:8000 -v $(pwd)/data:/data blindbit-oracle

# Nix Docker
nix-docker:
    nix build .#docker
    docker load < result

# Help with common tasks
help:
    @echo "Blindbit Oracle Development Commands:"
    @echo ""
    @echo "Core commands:"
    @echo "  just build       - Build the application"
    @echo "  just test        - Run tests"
    @echo "  just run         - Start server"
    @echo ""
    @echo "Development:"
    @echo "  just init        - Setup default config file (Start here)"
    @echo "  just check       - Run tests and linting"
    @echo "  just show-config - Display current configuration"
    @echo ""
    @echo "Use 'just --list' to see all available commands"
