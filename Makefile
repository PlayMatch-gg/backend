.PHONY: db-up db-down swag-gen run dev clean format lint

# Start all docker-compose services (PostgreSQL and Adminer)
db-up:
	docker-compose up -d

# Stop all docker-compose services
db-down:
	docker-compose down

# Generate Swagger documentation
swag-gen:
	@echo "Generating Swagger documentation..."
	go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/server/main.go

# Run the Go application
run: swag-gen
	@echo "Running Go application..."
	go run ./cmd/server

# Development mode: generate swagger and run the app
dev: swag-gen
	@echo "Starting development server..."
	go run ./cmd/server

# Format the code
format:
	@echo "Formatting code..."
	go run golang.org/x/tools/cmd/goimports@latest -w .

# Lint the code
lint: format
	@echo "Linting code..."
	go vet ./...

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	rm -rf docs
	go clean -modcache
	go mod tidy
