{
  bash,
  fetchurl,
  jdk ? null,
  jdk25,
  lib,
  linkFarm,
  stdenv,
  unzip,
}:

  let

    resolvedJdk = if jdk != null then jdk else jdk25;

    name = "testcase-constructed";
    mainClass = "com.example.ConstructedApp";
    jvmArgs = [];
    args = [];
    webappExplode = null;

    dependencies = [
          { url = "https://repo1.maven.org/maven2/org/apache/commons/commons-lang3/3.12.0/commons-lang3-3.12.0.jar"; hash = "sha256-obLD1OX2p7jJ0OHyo7TF1uf4qbDB0uP0pbbH2OnwobI="; }
          { url = "https://repo1.maven.org/maven2/com/fasterxml/jackson/core/jackson-databind/2.15.0/jackson-databind-2.15.0.jar"; hash = "sha256-ssPU5fanuMnQ4fKjtMXW5/ipsMHS4/SltsfY6fChssM="; }
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
