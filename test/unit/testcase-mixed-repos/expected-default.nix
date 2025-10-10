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

        name = "mixed-app_2.13";
        mainClass = "a8.example.MixedApp";
        jvmArgs = [];
        args =  [];
        repo = "repo";
        organization = "a8";
        artifact = "mixed-app_2.13";
        version = "2.7.1-20251010_0836_master";
        branch = "master";
        webappExplode = null;
        javaVersion = null;

        dependencies = [
          { url = "https://locus2.accur8.net/repos/all/a8/m3-common_2.13/2.7.1-20251010_0836_master/m3-common_2.13-2.7.1-20251010_0836_master.jar";  hash = "sha256-obLD1OX2eJASNFZ4kBI0VniQq83vEjRWeJCrze8SNFY=";  organization = "a8";  module = "m3-common_2.13";  version = "2.7.1-20251010_0836_master";  m2RepoPath = "a8/m3-common_2.13/2.7.1-20251010_0836_master";  filename = "m3-common_2.13-2.7.1-20251010_0836_master.jar";  }
          { url = "https://repo1.maven.org/maven2/com/google/guava/guava/32.1.2-jre/guava-32.1.2-jre.jar";  hash = "sha256-8aKzxNXm94kBI0VniavN7wEjRWeJq83vASNFZ4mrze8=";  organization = "com.google.guava";  module = "guava";  version = "32.1.2-jre";  m2RepoPath = "com/google/guava/guava/32.1.2-jre";  filename = "guava-32.1.2-jre.jar";  }
          { url = "https://locus2.accur8.net/repos/all/io/accur8/some-lib_2.13/1.2.3/some-lib_2.13-1.2.3.jar";  hash = "sha256-3q2+78r+ur7erb7vyv66vt6tvu/K/rq+3q2+78r+ur4=";  organization = "io.accur8";  module = "some-lib_2.13";  version = "1.2.3";  m2RepoPath = "io/accur8/some-lib_2.13/1.2.3";  filename = "some-lib_2.13-1.2.3.jar";  }
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
