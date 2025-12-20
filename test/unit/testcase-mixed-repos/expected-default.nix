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

    name = "testcase-mixed-repos";
    mainClass = "a8.example.MixedApp";
    jvmArgs = [];
    args = [];
    webappExplode = null;

    dependencies = [
          { url = "https://locus2.accur8.net/repos/all/a8/m3-common_2.13/2.7.1-20251010_0836_master/m3-common_2.13-2.7.1-20251010_0836_master.jar"; hash = "sha256-obLD1OX2eJASNFZ4kBI0VniQq83vEjRWeJCrze8SNFY="; }
          { url = "https://repo1.maven.org/maven2/com/google/guava/guava/32.1.2-jre/guava-32.1.2-jre.jar"; hash = "sha256-8aKzxNXm94kBI0VniavN7wEjRWeJq83vASNFZ4mrze8="; }
          { url = "https://locus2.accur8.net/repos/all/io/accur8/some-lib_2.13/1.2.3/some-lib_2.13-1.2.3.jar"; hash = "sha256-3q2+78r+ur7erb7vyv66vt6tvu/K/rq+3q2+78r+ur4="; }
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
