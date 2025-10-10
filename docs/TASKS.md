

== current tasks ==

    * put dependency tree in jar file

    * how to use herman for server deployments


== DONE ==

    * build directories are preserved in
      * ~/.a8/herman/builds/
        * run-nix-build.sh to preserve build parms
        * all .nix files are in there
        * result link is in there

    * add --herman--reinstall flag

    * save the json response from nixBuildDescription

    * remove the drv symlink for herman/builds

    * make sure GC is working
        * symlink from nix store to 

    * exec just launches the 
      ~/.a8/herman/builds/io.accur8/a8-codegen_2.13/latest-master/a8-codegen which symlinks to the bin/a8-codegen

    * pin this via a flake and flake.lock

    * what if we had herman only get the dependency tree from locus
        * traverse the repo using maven-metadata.xml
            https://locus.accur8.net/repos/all/io/accur8/a8-codegen_2.13/maven-metadata.xml

            gets the list of versions that are availabe

        * properly parse build numbers

            0.1.0-20220205_2049_master

            0.1.0 is the version number
            20220205_2049 is the build number
            master is the branch

            note that the ordering for the latest on master is get all the versions with master as the branch in maven-metadata.xlm get the most recent version number and then if more than one exists the most recent build number

        * use nixBuildDescription - aka nix-build-descriptin-response.json...  
          resolutionResponse.artifacts 
          * so take that dependency tree and generate the appropriate .nix files from that.  Can spec it on the returned default.nix file

          * Note you can pass the resolved version number in so that the web service doesn't do resolution

          * add support for java 21 22 and 23 (in addition to the support for java 8 11 and 17 that the nix build already has)

          * we could generate a single flake.nix here that uses the root flake

    * support java versions 8 11 17 21 22 23

    * import on java-launcher.template

    * option to create a re-usable build
      * use this for nix-pins/{a8-codegen a8-zoo}

    * resolve version info via
        https://locus.accur8.net/repos/all/a8/a8-codegen_2.13/maven-metadata.xml
        noting build numbers are a timestamp and branch name combo
      * not doing this now simply because the nixBuildDescription web service is fast enough

    * generate the nix-hash from the sha256
      * available from the artifact's url so 


      * convert the sha256 hex encoded to 
          sha256-<base64(sha256-bytes)>
      * use the newer fetchurl form    
        fetchurl {
          url = "...";
          hash = "sha256-<base64string>";
        }

    * have locus / nixBuildDescription populate 

        * sha256 m2RepoPath and filename

          {
            "url": "https://locus.accur8.net/repos/all/org/scala-lang/modules/scala-collection-compat_2.13/2.7.0/scala-collection-compat_2.13-2.7.0.jar",
            "sha256": "",
            "organization": "org.scala-lang.modules",
            "module": "scala-collection-compat_2.13",
            "version": "2.7.0",
            "m2RepoPath": "",
            "filename": ""
          }

    * pass webappExplode through to launcherArgs
        * so add a webappExplore to the a8-codegen.json file
        * also irregardles of 
