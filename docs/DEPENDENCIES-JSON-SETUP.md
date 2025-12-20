# Dependencies.json Setup Guide

## Overview

Projects can publish a `dependencies.json` file that Herman uses to resolve transitive dependencies without runtime resolution. The file is published as: `{artifact}-{version}-dependencies.json`

URL pattern: `{repoURL}/{org-path}/{artifact}/{version}/{artifact}-{version}-dependencies.json`

Example: `https://locus2.accur8.net/repos/all/io/accur8/a8-nefario_2.13/0.0.2-20251220_1053_master/a8-nefario_2.13-0.0.2-20251220_1053_master-dependencies.json`

## Setup Steps

### 1. Copy DependenciesJson.scala

Copy `project/DependenciesJson.scala` from an existing project (e.g., codegen, composite, versions, or odin) to your project's `project/` directory.

### 2. Update project/Common.scala

Add to your `Common` object:

```scala
object Common extends a8.sbt_a8.SharedSettings ... {

  lazy val generateFullDependencies = settingKey[Boolean]("Generate full-dependencies.json with detailed artifact information")

  override def settings: Seq[Def.Setting[_]] =
    super.settings ++
    DependenciesJson.dependencyResourceSettings(generateFullDependencies, repoConfigFile, readRepoUrl()) ++
    Seq(
      generateFullDependencies := false  // Default to false
    )

  // ... rest of Common
}
```

### 3. Enable Per-Project in build.sbt

Add `Common.generateFullDependencies := true` to each project that should publish dependencies.json:

```scala
lazy val myProject =
  Common
    .jvmProject("my-artifact", file("my-project"), "myProject")
    .settings(
      Common.generateFullDependencies := true,
      libraryDependencies ++= Seq(
        // ... dependencies
      )
    )
```

### 4. Build and Publish

```bash
sbt compile  # Generates dependencies.json in target/
sbt publish  # Publishes artifact with dependencies.json
```

## Example Projects

Projects with dependencies.json enabled:
- `composite/remoteapi/nefario` - a8-nefario_2.13
- `composite/remoteapi/zerotier` - a8-zerotier_2.13
- `codegen` - a8-codegen_2.13
- `versions` - a8-versions_3.3
- `odin/zoolander` - a8-zoolander_2.13
- `odin/dist-release` - a8-qubes-dist_2.13

Copy `project/DependenciesJson.scala` from any of these projects to get started.
