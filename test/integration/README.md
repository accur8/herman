# Herman Test Setup

This directory contains test configurations and scripts for Herman.

## Unit Tests

Run the automated unit tests:

```bash
# From the project root
nix develop --command go test -v ./src
```

These tests verify:
- Reading launcher config JSON files
- Reading version files
- Parsing repo.properties files
- Multiple repository configurations

## Manual/Integration Testing

### Prerequisites

1. Build herman:
   ```bash
   cd ../..  # Go to project root
   go build -o herman ./src
   # or
   nix build
   ```

2. Create your repo configuration:
   ```bash
   mkdir -p ~/.a8
   cp repo.properties.example ~/.a8/repo.properties
   # Edit ~/.a8/repo.properties with actual credentials
   ```

### Quick Test

Run the test script:

```bash
./test-herman.sh
```

This will set up a test directory with a symlink and config file, and show you how to test.

### Manual Test

1. Create a test directory:
   ```bash
   mkdir -p /tmp/herman-test
   cd /tmp/herman-test
   ```

2. Copy the test config:
   ```bash
   cp /path/to/herman/test-setup/a8-codegen.json .
   ```

3. Create a symlink to herman:
   ```bash
   ln -s /path/to/herman/herman ./a8-codegen
   ```

4. Run it:
   ```bash
   ./a8-codegen --help
   ```

### What Happens

On first run:
1. Herman reads `a8-codegen.json`
2. Reads `~/.a8/repo.properties` to get repo URL and credentials
3. Calls the API endpoint: `https://locus2.accur8.net/api/nixBuildDescription`
4. Downloads the nix build files
5. Runs `nix-build` to create the application
6. Caches the result in `~/.a8/herman/builds/io.accur8/a8-versions_3/<version>/`
7. Executes the installed binary with your arguments

On subsequent runs:
- Uses the cached version
- Immediately executes the binary

### Expected Output

First run:
```
Fetching build description from https://locus2.accur8.net/repos/all...
Writing 3 files to /tmp/herman-build-xxxxx...
Running nix-build...
Installation complete: /nix/store/xxx-a8-versions_3/bin/launch
<output from a8-codegen>
```

Subsequent runs:
```
<output from a8-codegen>
```

### Cleanup

```bash
# Remove test directory
rm -rf /tmp/herman-test

# Remove cached installation
rm -rf ~/.a8/herman/builds/io.accur8/a8-versions_3
```

## Testing Different Scenarios

### Test with a different repository

1. Add to `~/.a8/repo.properties`:
   ```properties
   bob_url=https://your-repo.example.com/repos
   bob_user=your_user
   bob_password=your_password
   ```

2. Update `a8-codegen.json`:
   ```json
   {
     ...
     "repo": "bob"
   }
   ```

### Test version caching

1. Run herman once to install
2. Check the cache:
   ```bash
   ls -la ~/.a8/herman/builds/io.accur8/a8-versions_3/
   ```
3. You should see:
   - `<version>/` - Version-specific directory
   - `<version>/metadata.json` - Build metadata
   - `<version>/drv` - Symlink to nix store for GC protection
   - `latest-master` - Symlink to the version directory

### Test garbage collection protection

1. Run herman to install an app
2. Run nix garbage collection:
   ```bash
   nix-collect-garbage
   ```
3. Verify the app still works (protected by the .drv symlink)

## Troubleshooting

### "failed to read launcher config"
- Ensure the JSON file exists next to the symlink
- Check JSON syntax is valid

### "failed to read repo config"
- Create `~/.a8/repo.properties`
- Ensure it has the required `<repo>_url` key

### "API returned status 401"
- Check credentials in `~/.a8/repo.properties`
- Verify the username and password are correct

### "nix-build failed"
- Ensure Nix is installed and in PATH
- Check the API returned valid nix files
- Look at the nix-build output for specific errors
