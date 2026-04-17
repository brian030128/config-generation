.PHONY: build run test test-db test-bdd test-verbose clean

# Build the server binary
build:
	go build -o bin/server .

# Run the server (requires DATABASE_URL and JWT_SECRET)
run:
	go run .

# Run all tests
test:
	go test ./...

# Run only database integration tests
test-db:
	go test ./dbtest/

# Run only BDD tests
test-bdd:
	go test ./bddtest/

# Run all tests with verbose output
test-verbose:
	go test -v -count=1 ./...

# Remove build artifacts
clean:
	rm -rf bin/
