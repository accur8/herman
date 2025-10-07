package main

import (
	"fmt"
	"strings"
)

// Dependency represents a resolved Maven dependency
type Dependency struct {
	URL          string `json:"url"`
	SHA256       string `json:"sha256"`
	Organization string `json:"organization"`
	Module       string `json:"module"`
	Version      string `json:"version"`
	M2RepoPath   string `json:"m2RepoPath"`
	Filename     string `json:"filename"`
}

// LauncherNixConfig holds the configuration for generating launcher Nix files
type LauncherNixConfig struct {
	Name         string
	MainClass    string
	JvmArgs      []string
	Args         []string
	Repo         string
	Organization string
	Artifact     string
	Version      string
	Branch       string
	JavaVersion  string
	Dependencies []Dependency
}

// GenerateDefaultNix generates the default.nix file content
func GenerateDefaultNix(config LauncherNixConfig) string {
	// Build JDK imports based on Java versions we support (8, 11, 17, 21, 22, 23)
	jdkImports := "jdk8,\n  jdk11,\n  jdk17,\n  jdk21,\n  jdk22,\n  jdk23,"

	// Build dependencies list
	var depsBuilder strings.Builder
	for i, dep := range config.Dependencies {
		depsBuilder.WriteString(fmt.Sprintf(
			"          { url = %q;  sha256 = %q;  organization = %q;  module = %q;  version = %q;  m2RepoPath = %q;  filename = %q;  }",
			dep.URL, dep.SHA256, dep.Organization, dep.Module, dep.Version, dep.M2RepoPath, dep.Filename,
		))
		if i < len(config.Dependencies)-1 {
			depsBuilder.WriteString("\n")
		}
	}

	// Format JVM args
	jvmArgsStr := formatNixList(config.JvmArgs)
	argsStr := formatNixList(config.Args)

	// Determine Java version string (or null)
	javaVersionStr := "null"
	if config.JavaVersion != "" {
		javaVersionStr = fmt.Sprintf("%q", config.JavaVersion)
	}

	return fmt.Sprintf(`{
  bash,
  fetchurl,
  lib,
  linkFarm,
  %s
  stdenv,
  unzip,
}:

  let

    launcherConfig =
      {

        name = %q;
        mainClass = %q;
        jvmArgs = %s;
        args =  %s;
        repo = %q;
        organization = %q;
        artifact = %q;
        version = %q;
        branch = %q;
        webappExplode = null;
        javaVersion = %s;

        dependencies = [
%s
        ];
      };

    webappExplode = if launcherConfig.webappExplode == null then false else launcherConfig.webappExplode;

    fetcherFn =
      dep: (
        fetchurl {
          url = dep.url;
          sha256 = dep.sha256;
        }
      );

    javaVersion = launcherConfig.javaVersion;

    jdk =
      if javaVersion == null then jdk11
      else if javaVersion == "8" then jdk8
      else if javaVersion == "11" then jdk11
      else if javaVersion == "17" then jdk17
      else if javaVersion == "21" then jdk21
      else if javaVersion == "22" then jdk22
      else if javaVersion == "23" then jdk23
      else abort("expected javaVersion = [ 8 | 11 | 17 | 21 | 22 | 23 ] got ${javaVersion}")
    ;

    artifacts = map fetcherFn launcherConfig.dependencies;

    linkFarmEntryFn = drv: { name = drv.name; path = drv; };

    classpathBuilder = linkFarm launcherConfig.name (map linkFarmEntryFn artifacts);

    # Properly escape args for safe shell evaluation
    argsEscaped = lib.escapeShellArgs (launcherConfig.jvmArgs ++ [launcherConfig.mainClass] ++ launcherConfig.args);

    webappExploder =
      if webappExplode then
        ''
          echo exploding webapp-composite folder
          for jar in ${classpathBuilder}/*.jar
          do
            ${unzip}/bin/unzip $jar "webapp/*" -d $out/webapp-composite 2> /dev/null 1> /dev/null || true
          done
        ''
      else
        ""
    ;

  in

    stdenv.mkDerivation {
      name = launcherConfig.name;
      src = ./.;
      installPhase = ''

        mkdir -p $out/bin

        # create link to jdk bin so that top and other tools show the process name as something meaningful
        ln -s ${jdk}/bin/java $out/bin/${launcherConfig.name}j

        # create link to lib folder derivation
        ln -s ${classpathBuilder} $out/lib

        LAUNCHER=$out/bin/${launcherConfig.name}

        # Generate launcher script inline (no template file needed)
        cat > $LAUNCHER <<EOF
#!${bash}/bin/bash
# Generated at build time. Invokes the per-JDK wrapper (${launcherConfig.name}j).
# -cp includes all jars in $out/lib plus the working dir.
exec $out/bin/${launcherConfig.name}j -cp $out/lib/*:. ${argsEscaped} "\$@"
EOF

        chmod +x $LAUNCHER
        patchShebangs $LAUNCHER

      '' + webappExploder;
    }
`,
		jdkImports,
		config.Name,
		config.MainClass,
		jvmArgsStr,
		argsStr,
		config.Repo,
		config.Organization,
		config.Artifact,
		config.Version,
		config.Branch,
		javaVersionStr,
		depsBuilder.String(),
	)
}

// formatNixList formats a Go string slice as a Nix list
func formatNixList(items []string) string {
	if len(items) == 0 {
		return "[]"
	}

	var builder strings.Builder
	builder.WriteString("[")
	for i, item := range items {
		builder.WriteString(fmt.Sprintf("%q", item))
		if i < len(items)-1 {
			builder.WriteString("; ")
		}
	}
	builder.WriteString("]")
	return builder.String()
}
