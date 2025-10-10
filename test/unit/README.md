# Herman Unit Tests

Directory-based regression test suite for Herman dependency resolution.

## Test Structure

Each test case is a directory containing:
```
testcase-name/
├── app.json                 # Input: launcher configuration
├── dependencies.json        # Input: dependency list
├── expected-default.nix     # Expected: generated Nix derivation
└── expected-flake.nix       # Expected: generated flake wrapper
```

## Running Tests

### Via Shell Script (Direct)
```bash
./test/unit/run_tests.sh
```

### Via Go Test (Recommended)
```bash
cd src && go test -v -run TestGenerateRegressions
```

### Run All Go Tests
```bash
cd src && go test -v
```

## Current Test Cases

### testcase-simple
- **Purpose**: Basic public Maven dependencies
- **Tests**:
  - Public Maven repo resolution
  - Path-based artifact URLs
  - SHA256 hash handling

### testcase-mixed-repos
- **Purpose**: Multiple repository sources
- **Tests**:
  - Mixed repo types (public_maven, locus, default "repo")
  - Artifacts with explicit URLs
  - Artifacts without paths (name-based construction)
  - Repo override from repo.properties

## Adding New Test Cases

1. **Create test directory**:
   ```bash
   mkdir test/unit/testcase-mytest
   ```

2. **Create input files**:
   ```bash
   # Create app.json
   cat > test/unit/testcase-mytest/app.json <<EOF
   {
     "mainClass": "com.example.MyApp",
     "organization": "com.example",
     "artifact": "myapp",
     "branch": "master"
   }
   EOF

   # Create dependencies.json (see existing examples)
   vim test/unit/testcase-mytest/dependencies.json
   ```

3. **Generate expected outputs**:
   ```bash
   ./result/bin/herman generate \
     test/unit/testcase-mytest/app.json \
     test/unit/testcase-mytest/output \
     --dependencies-json test/unit/testcase-mytest/dependencies.json

   # Copy to expected files
   cp test/unit/testcase-mytest/output/default.nix \
      test/unit/testcase-mytest/expected-default.nix
   cp test/unit/testcase-mytest/output/flake.nix \
      test/unit/testcase-mytest/expected-flake.nix

   # Clean up
   rm -rf test/unit/testcase-mytest/output
   ```

4. **Run tests**:
   ```bash
   ./test/unit/run_tests.sh
   ```

The new test will be automatically discovered and run!

## Regenerating Expected Outputs

If you intentionally change Herman's output format:

```bash
for testcase in test/unit/testcase-*/; do
    name=$(basename "$testcase")
    echo "Regenerating $name..."

    ./result/bin/herman generate \
      "$testcase/app.json" \
      "$testcase/output" \
      --dependencies-json "$testcase/dependencies.json"

    cp "$testcase/output/default.nix" "$testcase/expected-default.nix"
    cp "$testcase/output/flake.nix" "$testcase/expected-flake.nix"
    rm -rf "$testcase/output"
done

# Verify
./test/unit/run_tests.sh
```

## Test Output

### Successful Run
```
Herman Test Suite
=================
Using herman: /path/to/herman

Testing testcase-mixed-repos... ✓ PASSED
Testing testcase-simple... ✓ PASSED

=================
Test Results:
  Passed: 2
  Failed: 0

All tests passed!
```

### Failed Test
```
Testing testcase-simple... ✗ FAILED (default.nix mismatch)
  Diff:
    --- expected-default.nix
    +++ actual output
    @@ -25,7 +25,7 @@
    -    version = "1.0.0-test";
    +    version = "1.0.0-prod";
```

## Benefits

- **Easy to add**: Just create a directory with files, no code changes needed
- **Visual diffs**: See exact differences in generated files
- **Regression safe**: Captures exact expected output
- **Self-documenting**: Each test case is its own example
- **CI/CD ready**: Exits with non-zero code on failure
- **Fast**: No network calls, runs in < 1 second

## Use Cases

1. **Development**: Quick iteration without network dependencies
2. **CI/CD**: Fast, reliable tests in pipelines
3. **Regression testing**: Verify changes don't break existing behavior
4. **Edge cases**: Test corner cases and unusual configurations
5. **Documentation**: Living examples of dependencies.json format

## Test Ideas

Consider adding test cases for:
- Empty dependencies list
- Single dependency
- Dependencies with no SHA256 (error case)
- Invalid repo names (error case)
- URL-only artifacts (no repo/path)
- Path-only artifacts (no URL)
- Mixed artifact types (jar + pom + sources)
- Special characters in artifact names
- Very long dependency lists (performance)

## Debugging Failed Tests

1. **Run with verbose output**:
   ```bash
   HERMAN=/path/to/herman ./test/unit/run_tests.sh
   ```

2. **Run single test case manually**:
   ```bash
   ./result/bin/herman generate \
     test/unit/testcase-simple/app.json \
     /tmp/test-output \
     --dependencies-json test/unit/testcase-simple/dependencies.json \
     --trace

   diff -u test/unit/testcase-simple/expected-default.nix \
           /tmp/test-output/default.nix
   ```

3. **Check test runner**:
   ```bash
   bash -x ./test/unit/run_tests.sh
   ```
