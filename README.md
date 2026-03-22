# xless

build and run ios apps from the terminal. no xcode ide required.

xless drives the apple toolchain directly — `swiftc`, `simctl`, `devicectl`, `codesign` — so you never need to open xcode. it can read your existing `.xcodeproj` or work with its own simple config file.

> xcode must be *installed* (for sdks and simulator runtimes), but you never open it.

## install

```sh
# homebrew (coming soon)
brew install kacy/tap/xless

# or grab the latest binary
curl -fsSL https://github.com/kacy/xless/releases/latest/download/install.sh | sh
```

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
| `xless info` | display resolved project configuration |
| `xless version` | print cli and toolchain versions |
| `xless init` | scaffold a new project *(coming soon)* |
| `xless build` | compile and bundle an ios app |
| `xless run` | build, install, and launch *(coming soon)* |
| `xless devices` | list simulators and physical devices *(coming soon)* |
| `xless logs` | stream app logs *(coming soon)* |
| `xless clean` | remove build artifacts *(coming soon)* |

every command supports `--json` for structured output, making it easy for scripts and llms to work with.

## how it works

xless auto-detects your project type:

| what's in the directory | what xless does |
|---|---|
| `.xcodeproj` + `xless.yml` | reads xcodeproj as source of truth, applies xless.yml as overlay |
| `.xcodeproj` only | reads xcodeproj directly — zero config |
| `xless.yml` only | uses xless.yml as the full config (native mode) |

### xcodeproj support

xless reads your `.xcodeproj/project.pbxproj` live at build time. it extracts targets, build configurations, source files, signing settings, and deployment targets. no import step, no config drift.

```sh
$ xless info
  info  project detected mode=xcodeproj
  info  xcodeproj path=./MyApp.xcodeproj
  project:
    name     MyApp
    mode     xcodeproj
    targets  3
  target:MyApp:
    name          MyApp
    bundle_id     com.example.MyApp
    product_type  app
    min_ios       17.0
    ...
  target:MyWidget:
    name          MyWidget
    product_type  app-extension
    ...
```

multi-target projects work out of the box. use `--target` to select which one to build:

```sh
xless build --target MyWidget
```

### building

`xless build` compiles swift sources, creates a `.app` bundle with `Info.plist`, and ad-hoc signs for the simulator:

```sh
$ xless build
  info  build target=MyApp platform=simulator config=debug
  info  compile files=3
  info  bundle path=.build/MyApp/MyApp.app
  info  sign identity=-
  ok    build complete bundle=.build/MyApp/MyApp.app time=1.2s
```

use `--build-config release` for an optimized build, or `--platform device` for a device build (requires signing identity):

```sh
xless build --build-config release
xless build --platform device
```

the build pipeline runs three stages in order: **compile** (swiftc), **bundle** (.app creation + Info.plist), and **sign** (codesign). if any stage fails, you get an error with a hint on what to fix.

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
--build-config  debug or release
--verbose       enable verbose output
--no-color      disable colored output
```

## json output

every command supports `--json` for machine-readable output:

```sh
$ xless info --json
{"type":"data","message":"project","data":{"name":"MyApp","mode":"xcodeproj","targets":"3"}}
{"type":"data","message":"target:MyApp","data":{"name":"MyApp","bundle_id":"com.example.MyApp",...}}
{"type":"data","message":"defaults","data":{"config":"debug","simulator":"iPhone 16 Pro"}}

$ xless build --json
{"type":"info","message":"build","data":{"target":"MyApp","platform":"simulator","config":"debug"}}
{"type":"info","message":"compile","data":{"files":"3"}}
{"type":"info","message":"bundle","data":{"path":".build/MyApp/MyApp.app"}}
{"type":"info","message":"sign","data":{"identity":"-"}}
{"type":"success","message":"build complete","data":{"bundle":".build/MyApp/MyApp.app","time":"1.2s"}}
{"type":"data","message":"build","data":{"target":"MyApp","bundle_id":"com.example.MyApp","platform":"simulator","config":"debug","bundle":".build/MyApp/MyApp.app","time":"1.2s"}}
```

## config resolution order

cli flags > xless.yml > xcodeproj > defaults

environment variables with `XLESS_` prefix are also supported (e.g., `XLESS_PLATFORM=device`).

## requirements

- macos with xcode installed (for sdks and simulator runtimes)
- go 1.21+ (for building from source)

## license

mit — see [LICENSE](LICENSE) for details.
