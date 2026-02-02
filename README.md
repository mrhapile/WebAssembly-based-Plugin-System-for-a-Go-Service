# WebAssembly-based-Plugin-System-for-a-Go-Service
Build a Go microservice that can load and execute WebAssembly plugins written in C/C++ at runtime.

## Project Structure

This project includes:

### C++ Plugins

**1. Simple Plugin ([plugin.cpp](plugin.cpp))**  
Minimal example with single exported function - ideal for learning

**2. Production ABI Plugin ([plugin_abi.cpp](plugin_abi.cpp))**  
Stable ABI with lifecycle management, versioning, and error handling - production-ready

### Go Runtime Package

**[runtime/](runtime/)** - Clean, testable Go package for loading and executing WASM plugins
- [loader.go](runtime/loader.go) - Plugin loading, validation, and resource management
- [executor.go](runtime/executor.go) - ABI function execution (init/process/cleanup)
- Each plugin runs in an isolated VM instance (sandboxed)
- No global state - fully testable API

### HTTP API Server

**[cmd/server/main.go](cmd/server/main.go)** - Minimal HTTP API for executing plugins
- POST `/run` endpoint
- Loads plugins from `./plugins/<name>/<name>.wasm`
- Full lifecycle management per request
- JSON request/response format

## Simple Plugin Example

### Quick Start

**1. Compile the plugin:**
```bash
clang++ --target=wasm32-wasi -nostdlib -Wl,--no-entry -Wl,--export=process -O3 -o plugin.wasm plugin.cpp
```

**2. Test with WasmEdge:**
```bash
wasmedge --reactor plugin.wasm process 21
# Output: 43
```

**3. Run with Go host:**
```bash
go run cmd/simple/main.go
# Output: Result: 43
```

## Stable ABI Plugin (Recommended)

The ABI plugin demonstrates production best practices:
- ✅ Version checking (`get_abi_version`)
- ✅ Lifecycle management (`init` → `process` → `cleanup`)
- ✅ Error codes (negative = error, 0 = success, positive = result)
- ✅ State validation
- ✅ Diagnostic exports

### Compile ABI Plugin

```bash
clang++ \
  --target=wasm32-wasi \
  -nostdlib \
  -Wl,--no-entry \
  -Wl,--export=get_abi_version \
  -Wl,--export=init \
  -Wl,--export=process \
  -Wl,--export=cleanup \
  -Wl,--export=get_call_count \
  -Wl,--export=is_initialized \
  -O3 \
  -o plugin_abi.wasm \
  plugin_abi.cpp
```

### Run ABI Plugin

```bash
go run cmd/abi/main_abi.go
```

**Expected output:**
```
Plugin ABI version: v1.0.0
Plugin initialized successfully
Result: 43
Total process() calls: 1
Plugin cleaned up successfully
```

### Files

**C++ Plugins:**
- [plugin.cpp](plugin.cpp) - Simple single-function plugin
- [plugin_abi.cpp](plugin_abi.cpp) - Production ABI plugin with lifecycle and versioning
- [plugins/hello/hello.cpp](plugins/hello/hello.cpp) - Example plugin for HTTP API

**Go Runtime Package:**
- [runtime/loader.go](runtime/loader.go) - Plugin loading and VM management
- [runtime/executor.go](runtime/executor.go) - ABI function execution

**Go Host Examples:**
- [cmd/server/main.go](cmd/server/main.go) - **HTTP API server** for plugin execution
- [cmd/example/example.go](cmd/example/example.go) - Uses runtime package (clean API)
- [cmd/simple/main.go](cmd/simple/main.go) - Direct WasmEdge SDK usage (simple plugin)
- [cmd/abi/main_abi.go](cmd/abi/main_abi.go) - Direct WasmEdge SDK usage (full ABI)

**Test Suites:**
- [runtime/loader_test.go](runtime/loader_test.go) - Unit tests for plugin loading
- [runtime/executor_test.go](runtime/executor_test.go) - Unit tests for execution
- [runtime/mock_test.go](runtime/mock_test.go) - Mocked tests with gomonkey
- [cmd/server/handler_test.go](cmd/server/handler_test.go) - HTTP integration tests

**Documentation:**
- [ABI.md](ABI.md) - **Complete ABI specification, versioning strategy, and common pitfalls**
- [BUILD.md](BUILD.md) - Compilation guide with flag explanations
- [go.mod](go.mod) - Go module configuration

### Requirements

- clang++ with WebAssembly support (LLVM 8+)
- WasmEdge runtime (0.13.0+)
- Go 1.21+
- wasmedge-go SDK
- wasm-objdump (optional, for inspection)

## Key Concepts

### Why extern "C"?

```cpp
// ❌ Without extern "C" - C++ name mangling
int process(int x);
// Symbol: _Z7processi (unpredictable)

// ✅ With extern "C" - stable C linkage
extern "C" int process(int x);
// Symbol: process (predictable)
```

Go cannot call mangled C++ symbols - **all exports must use `extern "C"`**

### ABI Return Convention

| Return Value | Meaning |
|--------------|---------|
| `0` | Success (for init/cleanup) |
| `> 0` | Valid result |
| `< 0` | Error code |

### Plugin Lifecycle

```
1. get_abi_version() → Check compatibility
2. init()            → Initialize plugin state
3. process(x)        → Execute logic (can call multiple times)
4. cleanup()         → Release resources
```

## Common Pitfalls

⚠️ **Name Mangling** - Forgetting `extern "C"` makes exports invisible to Go  
⚠️ **Struct Passing** - Don't pass structs across WASM boundary (use primitives)  
⚠️ **Exceptions** - C++ exceptions trap WASM modules (use error codes)  
⚠️ **Missing Init** - Calling process() before init() returns error  
⚠️ **No Version Check** - Breaking changes cause runtime failures  

See [ABI.md](ABI.md) for detailed explanations and solutions.

### Plugin Function

```cpp
extern "C" int process(int x);
```

The current implementation returns `(x * 2) + 1` but can be modified for any deterministic computation.

## Go Host Programs

### Simple Host (cmd/simple/main.go)

Direct WasmEdge SDK usage without abstractions:

```bash
go run cmd/simple/main.go
# Output: Result: 43
```

### ABI Host (cmd/abi/main_abi.go)

Full ABI implementation with version checking and lifecycle:

```bash
go run cmd/abi/main_abi.go
# Output:
# Plugin ABI version: v1.0.0
# Plugin initialized successfully
# Result: 43
# Total process() calls: 1
# Plugin cleaned up successfully
```

### Runtime Package Example (example.go)

**Recommended approach** - Uses the clean `runtime` package:

```bash
go run cmd/example/example.go
# Output:
# Loaded plugin: plugin_abi.wasm
# Plugin initialized successfully
# Result: process(21) = 43
# Result: process(50) = 101
# Plugin cleaned up successfully
```

**Usage:**
```go
import "github.com/mrhapile/wasm-plugin-system/runtime"

// Load plugin (validates and instantiates)
plugin, err := runtime.LoadPlugin("plugin_abi.wasm")
defer plugin.Close()

// Initialize
plugin.Init()
defer plugin.Cleanup()

// Execute
result, err := plugin.Execute(21)
```

**Benefits:**
- Clean API - no WasmEdge details exposed
- Automatic resource cleanup
- Proper error wrapping with context
- Testable - no global state
- Idiomatic Go code

## HTTP API Server

The HTTP server provides a REST API for executing WASM plugins.

### Start the Server

```bash
go run cmd/server/main.go
# Starting WASM plugin server on :8080
```

### Execute a Plugin

**Request:**
```bash
curl -X POST http://localhost:8080/run \
  -H "Content-Type: application/json" \
  -d '{"plugin": "hello", "input": 21}'
```

**Response:**
```json
{"output": 43}
```

### Plugin Directory Structure

Plugins are loaded from `./plugins/<name>/<name>.wasm`:

```
plugins/
└── hello/
    ├── hello.cpp   # Source code
    └── hello.wasm  # Compiled WASM module
```

### Build the Example Plugin

```bash
cd plugins/hello
clang++ --target=wasm32-wasi -nostdlib -Wl,--no-entry \
  -Wl,--export=init -Wl,--export=process -Wl,--export=cleanup \
  -O3 -o hello.wasm hello.cpp
```

### Error Handling

The server returns appropriate HTTP status codes:

| Status | Description |
|--------|-------------|
| 200 | Success - plugin executed |
| 400 | Bad request - invalid JSON or plugin name |
| 405 | Method not allowed - use POST |
| 500 | Internal error - plugin load/execution failed |

**Error response format:**
```json
{"error": "failed to load plugin: plugin file not found"}
```

## Installation

**1. Install WasmEdge:**
```bash
curl -sSf https://raw.githubusercontent.com/WasmEdge/WasmEdge/master/utils/install.sh | bash
```

**2. Install Go dependencies:**
```bash
go mod download
```

## Testing

The project includes comprehensive unit and integration tests using Ginkgo v2, Gomega, testify, and gomonkey.

### Run All Tests

```bash
# Run all tests with verbose output
go test -v ./...

# Run with Ginkgo CLI (recommended)
ginkgo -v ./...
```

### Run Specific Test Suites

```bash
# Runtime package unit tests
go test -v ./runtime/...

# HTTP server integration tests
go test -v ./cmd/server/...
```

### Test Coverage

| Package | Tests |
|---------|-------|
| `runtime` | LoadPlugin, Init, Execute, Cleanup, Close, resource management |
| `cmd/server` | POST /run, JSON validation, error handling, HTTP status codes |

### Testing Stack

- **Ginkgo v2** - BDD-style test framework
- **Gomega** - Expressive matchers
- **testify** - Additional assertions
- **gomonkey** - Function mocking for error path testing

## Learn More

- [ABI.md](ABI.md) - Complete ABI design guide with:
  - Why `extern "C"` is required
  - How Go discovers and calls exports
  - Versioning strategy and forward compatibility
  - 8 common pitfalls with solutions
  - Best practices for production plugins

- [BUILD.md](BUILD.md) - Detailed compilation guide with explanations for every compiler flag


