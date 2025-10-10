{
  bash,
  fetchurl,
  lib,
  linkFarm,
  jdk8,
  jdk11,
  jdk17,
  jdk21,
  jdk22,
  jdk23,
  stdenv,
  unzip,
}:

  let

    launcherConfig =
      {

        name = "single-app_2.13";
        mainClass = "com.example.SingleApp";
        jvmArgs = [];
        args =  [];
        repo = "repo";
        organization = "com.example";
        artifact = "single-app_2.13";
        version = "1.0.0-20251010_1200_master";
        branch = "master";
        webappExplode = null;
        javaVersion = null;

        dependencies = [
          { url = "https://repo1.maven.org/maven2/com/google/guava/guava/32.1.2-jre/guava-32.1.2-jre.jar";  hash = "sha256-8qHz5qHl+NWy48TVprfI2eDxorPE1eb3qLnA0eLzpLU=";  organization = "com.google.guava";  module = "";  version = "32.1.2-jre";  m2RepoPath = "com/google/guava//32.1.2-jre";  filename = "guava-32.1.2-jre.jar";  }
        ];
      };

    webappExplode = if launcherConfig.webappExplode == null then false else launcherConfig.webappExplode;

    fetcherFn =
      dep: (
        fetchurl {
          url = dep.url;
          hash = dep.hash;
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
      dontUnpack = true;
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
export HERMAN_NIX_STORE=$out
exec $out/bin/${launcherConfig.name}j -cp $out/lib/*:. ${argsEscaped} "\$@"
EOF

        chmod +x $LAUNCHER
        patchShebangs $LAUNCHER

      '' + webappExploder;
    }
