# Herman

Herman is a launcher for Java applications that uses Nix for dependency management and installation. When invoked via a symlink, it reads a corresponding JSON configuration file, installs the application if needed, and executes it.

## How It Works

1. Herman is invoked via a symlink (e.g., `a8-codegen`)
2. It reads the configuration from `a8-codegen.json` in the same directory
3. It checks if the application is already installed in `~/.a8/herman/builds/`
4. If not installed:
   - Discovers the latest version from the Maven repository's `maven-metadata.xml`
   - Fetches dependency information from the configured repository API
   - Fetches SHA256 hashes using `nix store prefetch-file` (adds files to Nix store)
   - Generates Nix files locally (default.nix with inline launcher script)
   - Writes the per-package flake.nix to the build directory
   - Runs `nix build` using the shared nixpkgs from the root flake
   - Caches the result
5. Executes the installed application, passing through all arguments

## Configuration

### Launcher Config (e.g., `a8-codegen.json`)

```json
{
    "mainClass": "a8.codegen.Codegen",
    "organization": "io.accur8",
    "artifact": "a8-versions_3",
    "branch": "master",
    "jvmArgs": [],
    "args": [],
    "name": "a8-codegen",
    "repo": "repo",
    "webappExplode": false
}
```

**Configuration Fields:**
- `mainClass` (required): The Java main class to execute
- `organization` (required): Maven organization/group ID
- `artifact` (required): Maven artifact ID
- `branch` (required): Branch name for version resolution
- `jvmArgs` (optional): Array of JVM arguments (e.g., `["-Xmx2g"]`)
- `args` (optional): Array of default application arguments
- `name` (required): Application name (used for executable naming)
- `repo` (required): Repository prefix from `~/.a8/repo.properties`
- `webappExplode` (optional): If true, extracts `webapp/*` from all JARs to `$HERMAN_NIX_STORE/webapp-composite/`

### Repository Config (`~/.a8/repo.properties`)

```properties
repo_url=https://locus2.accur8.net/repos/all
repo_user=reader
repo_password=a_password
```

You can configure multiple repositories by using different prefixes:

```properties
repo_url=https://locus2.accur8.net/repos/all
repo_user=reader
repo_password=a_password

bob_url=https://another.example.com/repos
bob_user=user
bob_password=pass
```

Then in your launcher config, set `"repo": "bob"` to use the `bob_*` properties.

**Note:** `user` and `password` are optional.

## Nix Flakes and Shared nixpkgs

Herman uses Nix flakes to ensure all managed packages share the same nixpkgs version, providing reproducibility and better caching.

### Architecture

Herman maintains two flakes:

1. **Root flake** (`~/.a8/herman/flake.nix`): Defines the shared nixpkgs input with a pinned version
2. **Per-package flakes**: Each package's `nix-build/flake.nix` follows the root nixpkgs

This ensures all Herman-managed packages use the exact same nixpkgs version, defined by `~/.a8/herman/flake.lock`.

### Updating nixpkgs for All Packages

To update the shared nixpkgs version used by all Herman-managed packages:

```bash
cd ~/.a8/herman
nix flake update
```

This updates `flake.lock` with the latest nixpkgs. All subsequent builds will use this new version.

### Checking Current nixpkgs Version

```bash
cd ~/.a8/herman
nix flake metadata
```

Or examine the flake.lock file:

```bash
cat ~/.a8/herman/flake.lock | grep -A 5 '"nixpkgs"'
```

### Benefits

- **Reproducibility**: All packages share the same nixpkgs version
- **Efficient caching**: Nix can share builds across packages
- **Easy updates**: Update all packages' nixpkgs with one command
- **Explicit dependencies**: The exact nixpkgs version is recorded in flake.lock

## Environment Variables

Herman sets the following environment variables for launched applications:

- `HERMAN_NIX_STORE`: Path to the Nix store directory for the application (e.g., `/nix/store/xxx-a8-codegen`). This is set at build time and available to the application at runtime, useful for locating resources relative to the installation directory.

## Herman Flags

Herman supports special `--herman-*` flags that control the launcher itself, separate from the application's arguments:

### Available Flags

- `--herman-help` - Show Herman help message and exit
- `--herman-trace` - Enable verbose trace output for debugging
- `--herman-update` - Check for and install updates, then run the app
- `--herman-update-only` - Check for and install updates, then exit without running
- `--herman-version` - Show version information, then run the app
- `--herman-info` - Show installation information, then run the app

### Examples

```bash
# Show Herman help
a8-codegen --herman-help

# Run with trace mode to see what Herman is doing
a8-codegen --herman-trace --help

# Update to latest version, then run
a8-codegen --herman-update --some-app-flag

# Just update, don't run
a8-codegen --herman-update-only

# Show version and installation info
a8-codegen --herman-version
a8-codegen --herman-info

# Combine multiple Herman flags
a8-codegen --herman-trace --herman-update --help
```

### Command Mode

When invoked directly as `herman` (not via symlink), Herman enters command mode for managing installations:

```bash
# Show command help
herman help

# Generate Nix files from a config file (for embedding in Nix builds)
herman generate app-config.json ./output-dir

# Update a specific installation
herman update ~/bin/a8-codegen

# List all installations
herman list

# Clean old versions
herman clean io.accur8/a8-versions_3

# Run Nix garbage collection
herman gc

# Show installation info
herman info ~/bin/a8-codegen
```

#### Generate Command

The `herman generate` command creates standalone Nix files from a launcher config without requiring Herman at runtime:

```bash
herman generate my-app.json ./nix-output
```

This is useful for:
- Embedding applications in other Nix builds
- Creating reproducible builds without Herman dependency
- Pre-generating Nix expressions for version control

The command:
1. Discovers the latest version from Maven metadata
2. Fetches dependency information from the API
3. Downloads SHA256 hashes from Maven `.sha256` files (faster, no JAR downloads)
4. Generates `default.nix` with all dependencies in SRI hash format
5. Creates a `flake.nix` that references the root Herman flake

The generated files can be built with `nix build` without Herman installed.

## Project Structure

```
herman/
├── src/              # Go source files
│   ├── main.go       # Entry point and main logic
│   ├── repo.go       # Repository configuration reader
│   ├── api.go        # API client
│   ├── install.go    # Installation and nix-build logic
│   └── *_test.go     # Unit tests
├── test/
│   └── integration/  # Integration test setup and examples
├── flake.nix         # Nix flake for development and building
├── go.mod            # Go module definition
└── README.md
```

## Building

### Using Nix (recommended)

```bash
# Enter development environment
nix develop

# Build the project
go build -o herman ./src

# Or build via Nix
nix build
```

### Without Nix

```bash
# Requires Go 1.25+
go build -o herman ./src
```

## Usage

1. Build herman:
   ```bash
   nix build
   ```

2. Create a symlink for your application:
   ```bash
   ln -s /path/to/herman /usr/local/bin/a8-codegen
   ```

3. Create the config file next to the symlink:
   ```bash
   cat > /usr/local/bin/a8-codegen.json <<EOF
   {
       "mainClass": "a8.codegen.Codegen",
       "organization": "io.accur8",
       "artifact": "a8-versions_3",
       "branch": "master",
       "jvmArgs": [],
       "args": [],
       "name": "a8-codegen",
       "repo": "repo"
   }
   EOF
   ```

4. Create the repo config:
   ```bash
   mkdir -p ~/.a8
   cat > ~/.a8/repo.properties <<EOF
   repo_url=https://locus2.accur8.net/repos/all
   repo_user=reader
   repo_password=a_password
   EOF
   ```

5. Run your application:
   ```bash
   a8-codegen --help
   ```

On first run, Herman will install the application via Nix and cache it. Subsequent runs will use the cached version.

## Testing

### Unit Tests

Run the automated unit tests:

```bash
nix develop --command go test -v ./src
```

The tests verify:
- Config file parsing (launcher config and version files)
- Repository properties reading
- Multi-repository support

### Integration Testing

For manual/integration testing, see the [test/integration/](test/integration/) directory which includes:
- Sample configuration files
- Test script (`test-herman.sh`)
- Detailed testing instructions

Quick test:
```bash
cd test/integration
./test-herman.sh
```

See [test/integration/README.md](test/integration/README.md) for complete testing documentation.

## Directory Structure

```
~/.a8/
├── repo.properties              # Repository configuration
└── herman/
    ├── flake.nix                # Root flake with shared nixpkgs
    ├── flake.lock               # Pinned nixpkgs version (source of truth)
    └── builds/
        └── <org>/
            └── <artifact>/
                ├── <version>/
                │   ├── metadata.json           # Build metadata (exec path, version info)
                │   ├── <name> -> /nix/store/.../bin/<name>  # Executable symlink
                │   └── nix-build/              # Nix build files and script
                │       ├── flake.nix           # Per-package flake (follows root)
                │       ├── default.nix         # Generated Nix build with inline launcher
                │       ├── nixBuildDescription-response.json  # Raw API response
                │       ├── build.sh            # Reproducible build script
                │       └── result -> /nix/store/...  # Build result (prevents GC)
                └── latest-<branch> -> <version>/    # Symlink to latest version
```

Example:
```
~/.a8/herman/
├── flake.nix
├── flake.lock
└── builds/io.accur8/a8-codegen_2.13/
    ├── 0.1.0-20250503_1316_master/
    │   ├── metadata.json
    │   ├── a8-codegen -> /nix/store/6kxpzdzxzhphnm0kpb0v2ii4qxb4ddqh-a8-codegen/bin/a8-codegen
    │   └── nix-build/
    │       ├── flake.nix
    │       ├── default.nix
    │       ├── nixBuildDescription-response.json
    │       ├── build.sh
    │       └── result -> /nix/store/6kxpzdzxzhphnm0kpb0v2ii4qxb4ddqh-a8-codegen
    └── latest-master -> 0.1.0-20250503_1316_master/
```

The `nix-build/` directory contains:
- `flake.nix`: Per-package flake that follows the shared nixpkgs from `~/.a8/herman/flake.lock`
- `default.nix`: Generated Nix build file with:
  - Dependency list with SRI format hashes (`sha256-<base64>`)
  - Inline launcher script with HERMAN_NIX_STORE environment variable
  - Java version selection (supports JDK 8, 11, 17, 21, 22, 23)
  - Optional webapp explosion for web applications
- `nixBuildDescription-response.json`: Raw API response for debugging
- `build.sh`: Reproducible build script using `nix build`
- `result`: Symlink to Nix store, preventing garbage collection

The executable symlink in the version directory (named from the launcher config's `name` field) provides a stable path for execution (e.g., `latest-master/a8-codegen`).

## Garbage Collection

Herman creates symlinks to the Nix store paths (`result` symlinks in the `nix-build/` directory) to prevent them from being garbage collected. If you want to clean up old versions:

```bash
# Remove old version directory
rm -rf ~/.a8/herman/builds/<org>/<artifact>/<old-version>

# Run Nix garbage collection
nix-collect-garbage
```

## License

MIT
