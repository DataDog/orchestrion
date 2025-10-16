# End-to-End (E2E) Tests

End-to-end tests verify complete orchestrion workflows in realistic scenarios.

## Framework

```
test/e2e/
├── helpers.go          # Common test utilities
├── pgo_test.go         # PGO test
├── testdata/           # Test applications
│   └── pgo/            # PGO sample app
│       ├── main.go
│       ├── go.mod
│       └── orchestrion.tool.go
└── [your]_test.go      # Add new test files here
```

All tests use the root `github.com/DataDog/orchestrion` module - no separate modules needed!

## Running Tests

### All e2e tests

```bash
make test-e2e
```

### Specific test case

```bash
cd test/e2e/pgo
go test -tags=e2e -v .
```

**Important:** E2E tests require the `-tags=e2e` flag and won't run with regular `go test ./...`

## Adding a New Test Case

1. **Create directory and module:**

   ```bash
   mkdir test/e2e/my-test
   cd test/e2e/my-test
   go mod init github.com/DataDog/orchestrion/test/e2e/my-test
   ```

2. **Add dependencies:**

   ```bash
   go mod edit -require=github.com/DataDog/orchestrion/test/e2e@v0.0.0
   go mod edit -replace=github.com/DataDog/orchestrion/test/e2e=..
   go mod edit -replace=github.com/DataDog/orchestrion=../../..
   go mod tidy
   ```

3. **Create test file** (`my_test_test.go`):

   ```go
   //go:build e2e

   package main_test

   import (
       "testing"
       helpers "github.com/DataDog/orchestrion/test/e2e"
   )

   func TestMyTest(t *testing.T) {
       if testing.Short() {
           t.Skip("Skipping e2e test in short mode")
       }

       orchestrionBin := helpers.FindOrchestrionBinary(t)
       workDir := helpers.CreateWorkDir(t, ".")

       // Your test logic using helpers
   }
   ```

4. **Run it:**

   ```bash
   go test -tags=e2e -v .
   ```

That's it! The test will automatically run in CI.

## Helper Functions

See `helpers.go` for utilities:

- `FindOrchestrionBinary(t)` - Locates or builds orchestrion
- `CreateWorkDir(t, dir)` - Creates temp directory with test files
- `RunAndLog(t, cmd, logPath, log)` - Runs command, logs output, checks for errors
- `Logger(t, logFile)` - Creates logging function
- `CopyDir(t, src, dst)` - Recursively copies directories
- `WaitForCommandWithTimeout(t, cmd, timeout)` - Runs with timeout
