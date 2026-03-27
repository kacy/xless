# xless

build and run ios apps from the terminal. no xcode ide required.

xless gives you a cleaner cli over the apple toolchain. in native mode it drives `swiftc`, `simctl`, `devicectl`, and `codesign` directly. for existing `.xcworkspace` / `.xcodeproj` apps it uses xcode's own build engine, then keeps xless in charge of target selection, output, install, launch, and logs.

> xcode must be *installed* (for sdks and simulator runtimes), but you never open it.

## install

```sh
# homebrew
brew install kacy/tap/xless

# or install the latest release from xless.dev
curl -fsSL https://xless.dev/install.sh | sh
```

the install script currently supports macOS on Apple Silicon (`arm64`) only.

## quick start

```sh
# start a new project
xless init myapp
cd myapp
xless run

# or use an existing xcode project — no setup needed
cd ~/Projects/MyExistingApp
xless info          # see what xless detected
xless run           # build and launch in simulator
```

## commands

| command | description |
|---|---|
| `xless version` | print cli and toolchain versions |
| `xless info` | display resolved project configuration |
| `xless init [name]` | scaffold a new project |
| `xless build` | compile and bundle an ios app |
| `xless run` | build, install, and launch on simulator or device |
| `xless devices` | list simulators and physical devices |
| `xless logs` | stream app logs from a simulator |
| `xless clean` | remove build artifacts |

every command supports `--json` for structured output, making it easy for scripts and llms to work with.

## how it works

xless auto-detects your project type:

| what's in the directory | what xless does |
|---|---|
| `.xcworkspace` + `xless.yml` | reads the workspace's member xcodeproj files as source of truth, applies xless.yml as overlay |
| `.xcworkspace` only | reads workspace member xcodeproj files directly |
| `.xcodeproj` + `xless.yml` | reads xcodeproj as source of truth, applies xless.yml as overlay |
| `.xcodeproj` only | reads xcodeproj directly — zero config |
| `xless.yml` only | uses xless.yml as the full config (native mode) |

### project scaffolding

```sh
# simple swift project (default)
xless init myapp

# swift package manager project
xless init myapp --template spm

# custom bundle id and deployment target
xless init myapp --bundle-id com.mycompany.myapp --min-ios 17.0
```

the simple template creates a minimal SwiftUI app with `xless.yml`. the spm template adds a `Package.swift` manifest for future use.

> note: the build pipeline currently supports `type: "simple"` only. native `type: "spm"` projects fail explicitly as unsupported.

### xcodeproj and workspace support

xless reads your `.xcodeproj/project.pbxproj` live for detection, target selection, overlays, and `info`. when a workspace is present, it reads the member xcodeproj files referenced by `contents.xcworkspacedata`. builds in project/workspace mode are delegated to `xcodebuild`, so compatibility tracks Xcode much more closely than the native `xless.yml` pipeline.

```sh
$ xless info
  info  project detected mode=xcworkspace
  info  xcworkspace path=./MyApp.xcworkspace
  project:
    name     MyApp
    mode     xcworkspace
    targets  3
  selection:
    target         MyApp
    platform       simulator
    config         debug
    backend        xcodebuild
    xcode_scheme   MyApp
    xcode_selector -scheme=MyApp
  target:MyApp:
    name          MyApp
    bundle_id     com.example.MyApp
    product_type  app
    min_ios       17.0
    ...
```

multi-target projects work out of the box. use `--target` to select which one to build:

```sh
xless build --target MyWidget
```

### building

`xless build` produces a signed `.app` bundle and normalizes it into `.build/<target>/`. in native mode xless runs its own compile/bundle/sign pipeline. in xcodeproj/workspace mode it delegates to `xcodebuild`, then keeps the artifact layout and output format consistent:

```sh
$ xless build
  info  build target=MyApp platform=simulator config=debug
  info  compile files=3
  info  bundle path=.build/MyApp/MyApp.app
  info  sign identity=-
  ok    build complete output=.build/MyApp/MyApp.app time=1.2s
```

use `--build-config release` for an optimized build, or `--platform device` for a device build:

```sh
xless build --build-config release
xless build --platform device
```

native mode runs stages in order: **compile** (swiftc), **bundle** (.app creation + Info.plist), **sign** (codesign), and **package** (IPA, device builds only). xcodeproj/workspace mode uses `xcodebuild build` for simulator builds and `xcodebuild archive` + `-exportArchive` for device builds, then xless handles normalization, install, launch, logging, and JSON output. if any stage fails, you get an error with a hint on what to fix.

### running

```sh
# simulator (default)
xless run

# physical device
xless run --platform device

# with log streaming
xless run --logs
```

### device management

```sh
# list everything
xless devices

# simulators only
xless devices --simulators

# physical devices only
xless devices --physical

# only booted simulators
xless devices --booted
```

### log streaming

```sh
# stream logs from default simulator
xless logs

# filter by keyword
xless logs --filter "error"

# explicit bundle id
xless logs --bundle-id com.example.MyApp
```

### cleaning

```sh
xless clean
```

### device builds

device builds require a signing identity, provisioning profile, and optionally entitlements. configure these in `xless.yml`:

```yaml
signing:
  identity: "Apple Development: you@example.com"
  provisioning_profile: "path/to/profile.mobileprovision"
  entitlements: "path/to/entitlements.plist"
  team_id: "YOUR_TEAM_ID"
```

the device build pipeline produces an IPA (`.ipa`) file that gets installed via `devicectl`. in xcodeproj/workspace mode that IPA now comes from `xcodebuild -exportArchive`, not a custom zip step.

## support limits

xless now has two build backends, and the limits depend on which one you are using.

- native `xless.yml` mode:
  - supports `build.type: "simple"` only
  - is still focused on simple Swift apps
  - does not support native `build.type: "spm"`
- `.xcodeproj` / `.xcworkspace` mode:
  - delegates builds to `xcodebuild`
  - is the recommended path for real-world Apple projects, including mixed-language apps, package dependencies, resource compilation, and broader Xcode build behavior
  - requires a buildable shared Xcode scheme; use `--scheme` when target-name matching is not enough

project/workspace mode still has a few xless-specific limits:
- `xless info` is still based on xless's own parser, so it may show `parsed_notes` for features that are still buildable through `xcodebuild`
- `xless info` includes a top-level `selection` block for what xless would build right now, and target entries may also report `xcode_scheme`, `xcode_selector`, and `xcode_selector_hint`
- delegated `build` and `run` no longer fall back to raw Xcode targets; if no shared scheme matches, xless errors clearly and asks for `--scheme` or a shared scheme
- native mode remains much narrower than delegated project/workspace mode

for delegated project/workspace builds, xless resolves the Xcode build entry in this order:
- explicit `--scheme`
- scheme matching the selected xless target name
- the only available shared scheme, if there is exactly one

### xless.yml overlay

when using an xcodeproj, `xless.yml` is optional. it lets you override xless-specific settings without touching the xcodeproj:

```yaml
defaults:
  target: "MyApp"
  config: "debug"
  simulator: "iPhone 16 Pro"

overrides:
  targets:
    MyApp:
      signing:
        identity: "Apple Development: you@example.com"
        team_id: "YOUR_TEAM_ID"
      swift_flags: ["-DXLESS_BUILD"]
      min_ios: "17.0"
```

merge rules: signing is fully replaced, swift flags are appended, min_ios replaces.

### native mode

for projects without an xcodeproj, `xless.yml` is the full config:

```yaml
project:
  name: "MyApp"
  bundle_id: "com.example.MyApp"
  version: "1.0.0"

build:
  sources: ["Sources/"]
  min_ios: "16.0"

defaults:
  simulator: "iPhone 16 Pro"
```

## global flags

```
--json          output as newline-delimited json
--platform      simulator or device
--target        build target name
--build-config  build configuration name (for example: Debug or Release)
--device        device name or UDID
--verbose       enable verbose output
--no-color      disable colored output
```

## json output

every command supports `--json` for machine-readable output:

```sh
$ xless info --json
{"type":"data","message":"project","data":{"name":"MyApp","mode":"xcodeproj","targets":"3"}}
{"type":"data","message":"target:MyApp","data":{"name":"MyApp","bundle_id":"com.example.MyApp",...}}

$ xless build --json
{"type":"info","message":"build","data":{"target":"MyApp","platform":"simulator","config":"debug"}}
{"type":"info","message":"compile","data":{"files":"3"}}
{"type":"success","message":"build complete","data":{"output":".build/MyApp/MyApp.app","time":"1.2s"}}
{"type":"data","message":"build","data":{"target":"MyApp","bundle_id":"com.example.MyApp","platform":"simulator","config":"debug","bundle":".build/MyApp/MyApp.app","time":"1.2s"}}

$ xless devices --json
{"type":"data","message":"simulator","data":{"name":"iPhone 16 Pro","udid":"...","state":"Booted","runtime":"iOS 18.2"}}
{"type":"data","message":"device","data":{"name":"Kacy's iPhone","udid":"...","type":"iPhone","transport":"wired","state":"connected"}}
```

## config resolution order

cli flags > xless.yml > xcodeproj > defaults

environment variables with `XLESS_` prefix are also supported (e.g., `XLESS_PLATFORM=device`).

## requirements

- macos with xcode installed (for sdks and simulator runtimes)
- go 1.21+ (for building from source)

## license

mit — see [LICENSE](LICENSE) for details.
