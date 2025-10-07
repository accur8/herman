
bootstrap a go project backed by a flake.nix file


Create a go project that will be a launcher to launch java apps.  The launcher will be symlinked.  So the launcher herman

We are calling this herman, because why not

- so if the symlink is `a8-codegen` read `a8-codegen.json` (which will be right next to the symlink)

{
    "mainClass": "a8.codegen.Codegen",
    "organization":"io.accur8",
    "artifact":"a8-versions_3",
    "branch":"master",
    "jvmArgs":[],
    "args":[],
    "name":"a8-codegen", 
    "repo": "repo"
}


- then look to see if this is already installed 

ls ~/.a8/hermans/cache/a8/a8-launch-me_3/latest_master.json

Noting that in the install step we will talk about how that file gets created


- if the file doesn't exist run an install.  

Call the repo endpoint to get the install instructions.  the repo defined by repo 

cat ~/.a8/repo.properties

repo_url=https://locus2.accur8.net/repos/all
repo_user=reader
repo_password=a_password


So if they said "repo": "bob" we would look for this

bob_url=https://locus2.accur8.net/repos/all
bob_user=reader
bob_password=a_password


Noting that user and password are optional


So call this endpoint on the repo.  Here is the curl


curl 'https://locus2.accur8.net/api/nixBuildDescription' --data '{"kind":"jvm_cli","mainClass":"a8.codegen.Codegen","organization":"io.accur8","artifact":"a8-versions_3","branch":"master","commandLineParms":{"programName":"demo/a8-codegen","rawCommandLineArgs":["demo/a8-codegen","--l-resolveOnly","--l-verbose"],"resolvedCommandLineArgs":[],"resolveOnly":true,"quiet":false,"explicitVersion":null,"launcherJson":null,"showHelp":false},"quiet":false,"logRollers":[],"logFiles":false,"installDir":null,"jvmArgs":[],"args":[],"name":"a8-codegen", "repo": "repo"}'



Noting this will return what is in the file nix-build-description-response.json

That will content a list of files to write to local disk.  As well as the resolved version and other resolution details.

Write the files to a temp direction and then run the following with the cwd as that temp directory

`nix-build --out-link build -E with import <nixpkgs> {}; (callPackage ./default.nix {})`

If successful that will output a symlink `build` in there should be a bin folder resolve that to the canonical path to use to put into the ~/.a8/hermans/cache/a8/a8-launch-me_3/latest_master.json file



- write the version file 

so write ~/.a8/versions/cache/a8/a8-codegen_2.12/

and if it is the latest then also a symlink

latest_master.json -> 0.1.0-20210804_1624_master.json
latest_master.json.drv -> /nix/store/0dp865fkiq2n7r90n7k3cwf3qyxc76df-a8-codegen

- if installing we need to also properly setup garbage collection between the link to the drv and nix



- now that the file exists (either already existed or via installation) load the file

{
    "exec": "/nix/store/i94pmah2nhsa5mkqpfigj20409wjdnbi-circe-generic_2.12-0.14.1/bin/launch",
    "appInstallerConfig": {
        "organization": "io.accur8",
        "artifact": "a8-launch-me_3",
        "version": "0.1.0-20210804_1624_master"
    }
}

then exec (“A process image replacement occurs.”) the exec



=========================




