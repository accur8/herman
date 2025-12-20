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
	Name          string
	MainClass     string
	JvmArgs       []string
	Args          []string
	Repo          string
	Organization  string
	Artifact      string
	Version       string
	Branch        string
	JavaVersion   string
	WebappExplode *bool
	Dependencies  []Dependency
}

// GenerateDefaultNix generates the default.nix file content
func GenerateDefaultNix(config LauncherNixConfig) string {
	// Determine which JDK to use as default (jdk25 if not specified in JSON)
	javaVersion := config.JavaVersion
	if javaVersion == "" {
		javaVersion = "25"
	}
	jdkPackage := fmt.Sprintf("jdk%s", javaVersion)

	// Build dependencies list - only include url and hash (what fetchurl actually uses)
	var depsBuilder strings.Builder
	for i, dep := range config.Dependencies {
		depsBuilder.WriteString(fmt.Sprintf(
			"          { url = %q; hash = %q; }",
			dep.URL, dep.SHA256,
		))
		if i < len(config.Dependencies)-1 {
			depsBuilder.WriteString("\n")
		}
	}

	// Format JVM args
	jvmArgsStr := formatNixList(config.JvmArgs)
	argsStr := formatNixList(config.Args)

	// Determine webappExplode value (or null)
	webappExplodeStr := "null"
	if config.WebappExplode != nil {
		if *config.WebappExplode {
			webappExplodeStr = "true"
		} else {
			webappExplodeStr = "false"
		}
	}

	return fmt.Sprintf(`{
  bash,
  fetchurl,
  jdk ? null,
  %s,
  lib,
  linkFarm,
  stdenv,
  unzip,
}:

  let

    resolvedJdk = if jdk != null then jdk else %s;

    name = %q;
    mainClass = %q;
    jvmArgs = %s;
    args = %s;
    webappExplode = %s;

    dependencies = [
%s
    ];

    artifacts = map (dep: fetchurl { url = dep.url; hash = dep.hash; }) dependencies;

    classpathBuilder = linkFarm name (map (drv: { name = drv.name; path = drv; }) artifacts);

    # Properly escape args for safe shell evaluation
    argsEscaped = lib.escapeShellArgs (jvmArgs ++ [mainClass] ++ args);

    webappExploder =
      if webappExplode == true then
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
      name = name;
      dontUnpack = true;
      installPhase = ''

        mkdir -p $out/bin

        # create link to jdk bin so that top and other tools show the process name as something meaningful
        ln -s ${resolvedJdk}/bin/java $out/bin/${name}j

        # create link to lib folder derivation
        ln -s ${classpathBuilder} $out/lib

        LAUNCHER=$out/bin/${name}

        # Generate launcher script inline (no template file needed)
        cat > $LAUNCHER <<EOF
#!${bash}/bin/bash
# Generated at build time. Invokes the per-JDK wrapper (${name}j).
# -cp includes all jars in $out/lib plus the working dir.
export HERMAN_NIX_STORE=$out
exec $out/bin/${name}j -cp $out/lib/*:. ${argsEscaped} "\$@"
EOF

        chmod +x $LAUNCHER
        patchShebangs $LAUNCHER

      '' + webappExploder;
    }
`,
		jdkPackage,
		jdkPackage,
		config.Name,
		config.MainClass,
		jvmArgsStr,
		argsStr,
		webappExplodeStr,
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
