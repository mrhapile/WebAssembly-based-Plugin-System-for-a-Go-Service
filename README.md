# WASM Plugin System for Go Services

A sandboxed, hot-swappable plugin system for Go services using WebAssembly. Plugins are written in C/C++, compiled to `wasm32-wasi`, and executed in isolated WasmEdge runtimes. The system provides a strict ABI contract, lifecycle management, and supports both local filesystem and Fluid dataset storage backends.

## Problem Statement

Traditional plugin systems in Go face fundamental challenges:

- **Native plugins** (`plugin` package) share the host process memory, making crashes and memory corruption possible. They require exact Go version matching and cannot be unloaded.
- **RPC-based plugins** (e.g., HashiCorp go-plugin) introduce network overhead, serialization complexity, and process management burden.
- **Scripting engines** (Lua, JavaScript) add large runtime dependencies and lack compile-time type safety.

These approaches trade off between safety, performance, and operational complexity. None provide true isolation with near-native performance.

## Why WebAssembly

WebAssembly addresses these constraints:

| Property | Benefit |
|----------|---------|
| **Memory isolation** | Each plugin runs in a sandboxed linear memory. Host memory is inaccessible. |
| **Capability-based security** | Plugins have no filesystem, network, or syscall access unless explicitly granted. |
| **Deterministic execution** | Same inputs produce same outputs. No undefined behavior from uninitialized memory. |
| **Language agnostic** | Plugins can be written in C, C++, Rust, or any language targeting `wasm32-wasi`. |
| **Hot-swappable** | Plugins can be loaded and unloaded at runtime without service restart. |
| **Portable** | Same `.wasm` binary runs on Linux, macOS, Windows, and edge devices. |

WasmEdge was chosen for its CNCF sandbox status, AOT compilation support, and mature Go SDK.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                              Go Service                                 │
│                                                                         │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────────┐   │
│  │  HTTP API    │───▶│ PluginStore  │───▶│  Plugin Path Resolution  │   │
│  │  POST /run   │    │  (Interface) │    │  - LocalPluginStore      │   │
│  └──────────────┘    └──────────────┘    │  - FluidPluginStore      │   │
│         │                                 └──────────────────────────┘  │
│         ▼                                              │                │
│  ┌──────────────────────────────────────────────────────────────────┐   │
│  │                     Runtime Package                               │  │
│  │  ┌────────────┐    ┌────────────┐    ┌────────────────────────┐  │   │
│  │  │  Loader    │───▶│  Executor  │───▶│  WasmEdge VM Instance  │  │   │
│  │  │            │    │            │    │  (Isolated Sandbox)    │  │   │
│  │  └────────────┘    └────────────┘    └────────────────────────┘  │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                    │                                    │
└────────────────────────────────────│────────────────────────────────────┘
                                     ▼
                    ┌─────────────────────────────────┐
                    │        WASM Plugin (.wasm)      │
                    │  ┌─────────────────────────────┐│
                    │  │  init() → process() → cleanup│
                    │  │  (Stable ABI Contract)       │
                    │  └─────────────────────────────┘│
                    └─────────────────────────────────┘
```

**Data flow:**
1. HTTP request specifies plugin name and input
2. PluginStore resolves plugin name to filesystem path
3. Loader creates isolated WasmEdge VM and loads WASM binary
4. Executor calls ABI functions in sequence
5. VM is destroyed after request completes

## Plugin ABI Contract

Plugins must export exactly three functions:

```c
// Initialize plugin state. Called once after loading.
// Returns: 0 on success, negative on error.
int init();

// Process input and return result. May be called multiple times.
// Returns: computed result (positive), or negative error code.
int process(int input);

// Release resources. Called before unloading.
// Returns: 0 on success, negative on error.
int cleanup();
```

**Error codes:**
| Code | Meaning |
|------|---------|
| `0` | Success |
| `-1` | Not initialized |
| `-2` | Already initialized |
| `-3` | Invalid argument |
| `-4` | Internal error |

**Constraints:**
- All functions use C linkage (`extern "C"`)
- No dynamic memory allocation (no `malloc`, `new`)
- No standard library dependencies (`-nostdlib`)
- No floating-point arguments (integer-only ABI)

See [ABI.md](ABI.md) for versioning strategy and compatibility guidelines.

## Plugin Lifecycle

```
     Load                Init               Execute              Cleanup            Release
       │                  │                    │                    │                  │
       ▼                  ▼                    ▼                    ▼                  ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────────┐    ┌─────────────┐    ┌─────────────┐
│ LoadPlugin()│───▶│   Init()    │───▶│   Execute(n)    │───▶│  Cleanup()  │───▶│   Close()   │
│ Create VM   │    │ Call init() │    │ Call process(n) │    │Call cleanup │    │ Destroy VM  │
│ Load WASM   │    │ Set state   │    │ Return result   │    │ Reset state │    │ Free memory │
└─────────────┘    └─────────────┘    └─────────────────┘    └─────────────┘    └─────────────┘
```

Each HTTP request creates a fresh VM instance. No state persists between requests.

## Fluid Integration

In production, plugins may be stored in distributed storage (S3, HDFS, etc.) and cached locally using [Fluid](https://github.com/fluid-cloudnative/fluid).

**How it works:**
1. Fluid Dataset CRD defines remote storage location
2. AlluxioRuntime or JuiceFSRuntime handles caching
3. Dataset is mounted as a POSIX filesystem path (e.g., `/mnt/fluid/plugins`)
4. This system treats the mount as a regular directory

**No Kubernetes client required.** The `FluidPluginStore` implementation simply reads from the mounted path. Caching, replication, and data locality are handled transparently by Fluid.

```go
// Development: local filesystem
store := fluid.NewLocalPluginStore("./plugins")

// Production: Fluid dataset mount
store := fluid.NewFluidPluginStore("/mnt/fluid/plugins")
```

Environment-based selection:
```bash
# Development (default)
go run ./cmd/server

# Production with Fluid
PLUGIN_STORE=fluid FLUID_MOUNT_PATH=/mnt/fluid/plugins go run ./cmd/server
```

## HTTP API

### POST /run

Execute a plugin with the given input.

**Request:**
```json
{
  "plugin": "hello",
  "input": 21
}
```

**Response:**
```json
{
  "output": 43
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"plugin": "hello", "input": 21}'
```

**Error responses:**
| Status | Condition |
|--------|-----------|
| 400 | Invalid JSON, missing plugin name, or invalid characters |
| 404 | Plugin not found |
| 405 | Method not POST |
| 500 | Plugin execution failed |

## Testing Strategy

Tests are written using Ginkgo v2 with Gomega matchers. Testify is used for specific assertions. Gomonkey enables mocking of filesystem operations.

### Unit Tests

Located alongside source files (`*_test.go`):

| Package | Coverage |
|---------|----------|
| `runtime` | Loader initialization, error paths, resource cleanup |
| `runtime` | Executor lifecycle, ABI error code handling |
| `fluid` | Path resolution, missing plugin handling |

### Integration Tests

| Package | Coverage |
|---------|----------|
| `cmd/server` | HTTP status codes, JSON parsing, path traversal prevention |

### Running Tests

```bash
# All tests
go test -v ./...

# With race detection
go test -race ./...

# With coverage
go test -cover -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

### Failure Cases Tested

- Missing plugin file
- Invalid WASM binary
- Plugin not initialized before `process()`
- Double initialization
- Missing ABI exports
- Path traversal attempts (`../`)
- Malformed JSON input

## CI Pipeline

GitHub Actions workflow (`.github/workflows/ci.yml`):

| Step | Description |
|------|-------------|
| Checkout | Clone repository |
| Setup Go | Install Go 1.24 with module caching |
| Install clang | C++ to WASM compilation |
| Install WasmEdge | Runtime for tests |
| Build WASM | Compile `plugins/hello/hello.wasm` |
| Build Go | Verify compilation |
| Vet | Static analysis |
| Test | Unit and integration tests with race detection |
| Coverage | Print summary, upload artifact |
| Artifacts | Upload WASM binaries |

Triggers: push to `main`, pull requests to `main`.

## Project Structure

```
.
├── .github/workflows/     # CI pipeline
│   └── ci.yml
├── cmd/                   # Executable entry points
│   ├── server/            # HTTP API server
│   ├── abi/               # ABI plugin demo
│   ├── simple/            # Simple plugin demo
│   └── example/           # Additional examples
├── runtime/               # Core Go package
│   ├── loader.go          # Plugin loading, VM management
│   ├── executor.go        # ABI function execution
│   └── *_test.go          # Unit tests
├── fluid/                 # Storage abstraction
│   ├── plugin_store.go    # PluginStore interface + implementations
│   └── *_test.go          # Unit tests
├── plugins/               # Plugin source and binaries
│   └── hello/
│       ├── hello.cpp      # Example plugin source
│       └── hello.wasm     # Compiled binary (git-ignored)
├── plugin.cpp             # Simple plugin example
├── plugin_abi.cpp         # Full ABI plugin example
├── ABI.md                 # ABI design document
├── BUILD.md               # Compilation instructions
├── go.mod
└── go.sum
```

## Limitations

- **Integer-only ABI**: No floating-point, string, or complex type passing. Requires serialization for structured data.
- **No WASI filesystem**: Plugins cannot read files. All data must be passed through function arguments.
- **Single-threaded execution**: Each VM instance is single-threaded. Parallelism requires multiple VM instances.
- **No plugin-to-plugin communication**: Plugins are isolated. The host must mediate all data exchange.
- **WasmEdge dependency**: Requires WasmEdge runtime and development libraries installed on the host.

## Future Work

- **Streaming API**: Support for processing large datasets without loading into memory.
- **Plugin registry**: Versioned plugin discovery with semver constraints.
- **Metrics export**: Prometheus metrics for plugin execution latency and error rates.
- **String passing**: Memory-based ABI for passing byte arrays between host and plugin.
- **Multi-function plugins**: Support for plugins exporting multiple processing functions.
- **Wasm Component Model**: Migration to the emerging component model standard.

## Ecosystem Alignment

This project integrates with CNCF ecosystem components:

| Project | Integration |
|---------|-------------|
| [WasmEdge](https://wasmedge.org/) | CNCF Sandbox runtime. Provides the execution environment for WASM plugins. |
| [Fluid](https://github.com/fluid-cloudnative/fluid) | CNCF Sandbox data orchestration. Enables distributed plugin storage with local caching. |
| [Kubernetes](https://kubernetes.io/) | Deployment target. Fluid integration assumes K8s-managed storage mounts. |

**Design principles aligned with cloud-native:**
- Stateless request handling
- Horizontal scalability (each request is independent)
- Observable (structured error codes, testable interfaces)
- Declarative configuration (environment variables for store selection)

## License

[Specify your license here]

## References

- [WasmEdge Documentation](https://wasmedge.org/docs/)
- [WasmEdge Go SDK](https://github.com/second-state/WasmEdge-go)
- [Fluid GitHub](https://github.com/fluid-cloudnative/fluid)
- [WebAssembly Specification](https://webassembly.github.io/spec/)
- [WASI Specification](https://wasi.dev/)


