.PHONY: db-up db-down swag-gen run dev clean

# Start PostgreSQL database
db-up:
	docker-compose up -d db

# Stop PostgreSQL database
db-down:
	docker-compose down db

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

# Clean generated files
clean:
	@echo "Cleaning generated files..."
	rm -rf docs
	go clean -modcache
	go mod tidy
