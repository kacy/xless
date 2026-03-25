# agents.md — xless

this file gives llms and ai coding agents everything they need to work with xless.

## what is xless

a go cli that builds and runs ios apps without the xcode ide. it drives `swiftc`, `simctl`, `devicectl`, and `codesign` directly.

## commands

```
xless version                           # print versions
xless info [--target <name>] [--json]   # show project config
xless init [name] [--template simple|spm] [--bundle-id <id>] [--min-ios <ver>]
xless build [--platform simulator|device] [--target <name>] [--build-config debug|release]
xless run [--platform simulator|device] [--logs] [--device <name|UDID>]
xless devices [--simulators] [--physical] [--booted]
xless logs [--filter <term>] [--bundle-id <id>] [--device <name|UDID>]
xless clean
```

all commands accept `--json` for structured ndjson output.

## current support boundary

- supported:
  - native `xless.yml` projects with `build.type: "simple"`
  - swift-only `.xcodeproj` apps
  - swift-only `.xcworkspace` apps that reference xcodeproj members
  - pure-swift package dependencies from xcodeproj/workspace targets when package sources are already available locally
- unsupported:
  - objective-c or mixed swift/objective-c targets
  - package plugins, macros, binary targets, c/c++/objective-c package targets, package resources, or package-specific build settings
  - automatic package fetching/checkouts beyond reading `Package.resolved`
  - native `build.type: "spm"` beyond scaffolding

## project structure

```
myapp/
├── xless.yml                    # project config (native mode)
├── Sources/MyApp/MyAppApp.swift # swift source
└── .build/                      # build artifacts (gitignored)
    └── MyApp/
        ├── MyApp                # compiled executable
        ├── MyApp.app/           # .app bundle
        └── MyApp.ipa            # IPA (device builds only)
```

## xless.yml (native mode)

```yaml
project:
  name: "MyApp"
  bundle_id: "com.example.MyApp"
  version: "1.0.0"
  build_number: "1"

build:
  type: "simple"           # simple (swiftc). "spm" scaffolds only and is not buildable yet.
  sources: ["Sources/MyApp/"]
  min_ios: "16.0"

signing:
  identity: ""                        # empty = ad-hoc (simulator only)
  provisioning_profile: ""            # required for device builds
  entitlements: ""
  team_id: ""

defaults:
  target: ""
  config: "debug"
  simulator: "iPhone 16 Pro"
  device: ""
```

## xless.yml (xcodeproj / workspace overlay mode)

when a `.xcodeproj` or `.xcworkspace` exists, xless reads the project graph as source of truth. `xless.yml` is optional and only overrides specific settings:

```yaml
defaults:
  target: "MyApp"
  config: "debug"

overrides:
  targets:
    MyApp:
      signing:
        identity: "Apple Development: you@example.com"
        team_id: "YOUR_TEAM_ID"
      swift_flags: ["-DXLESS_BUILD"]
      min_ios: "17.0"
```

## config resolution order

cli flags > xless.yml > xcworkspace/xcodeproj > defaults

environment variables: `XLESS_PLATFORM`, `XLESS_TARGET`, `XLESS_JSON`, etc.

## key terminology

| term | meaning | flag |
|---|---|---|
| platform | simulator vs device | `--platform` |
| target | named build product | `--target` |
| config | debug vs release | `--build-config` |

## build pipeline stages

1. **compile** — `swiftc` with sdk, arch, deployment target
2. **bundle** — create `.app/`, copy executable, generate/copy `Info.plist`, copy resources
3. **sign** — `codesign` (ad-hoc for simulator, identity+profile for device)
4. **package** — create `.ipa` zip archive (device builds only)

## error handling

all errors include actionable hints. example:

```
sign: no signing identity configured for device build (hint: set signing.identity in xless.yml or run `security find-identity -v -p codesigning`)
```

## json output format

ndjson (one json object per line):

```json
{"type":"info","message":"build","data":{"target":"MyApp","platform":"simulator","config":"debug"}}
{"type":"success","message":"build complete","data":{"output":".build/MyApp/MyApp.app","time":"1.2s"}}
{"type":"error","message":"compile failed","data":{"hint":"check your source files"}}
{"type":"data","message":"simulator","data":{"name":"iPhone 16 Pro","udid":"...","state":"Booted"}}
{"type":"log","message":"2026-03-24 10:00:00 MyApp[1234] some log line"}
```

types: `info`, `success`, `error`, `warn`, `data`, `log`

## codebase layout

```
main.go                          # entry point
cmd/                             # cobra commands
  root.go                        # global flags
  version.go, info.go, init.go, build.go, run.go, devices.go, logs.go, clean.go
internal/
  config/                        # unified ProjectConfig model, loading, validation
  build/                         # pipeline stages: compiler, bundler, signer, packager
  device/                        # Device interface, simulator (simctl), physical (devicectl), resolver
  output/                        # Formatter interface: human (colored) and json (ndjson)
  project/                       # project detection, scaffolding templates
  toolchain/                     # Toolchain interface, xcrun discovery, command execution
  xcodeproj/                     # pbxproj parser (howett.net/plist)
```

## building from source

```sh
go build -o xless .
./xless version
```

## testing

```sh
go test ./...
```

## common workflows

```sh
# new project
xless init myapp && cd myapp && xless run

# existing xcodeproj
cd MyProject && xless run

# device build
xless build --platform device

# ci/scripting
xless build --json | jq '.data.bundle'
```
