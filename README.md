# WebAssembly-based-Plugin-System-for-a-Go-Service
Build a Go microservice that can load and execute WebAssembly plugins written in C/C++ at runtime.

## Project Structure

This project includes two plugin examples:

### 1. Simple Plugin ([plugin.cpp](plugin.cpp))
Minimal example with single exported function - ideal for learning

### 2. Production ABI Plugin ([plugin_abi.cpp](plugin_abi.cpp))
Stable ABI with lifecycle management, versioning, and error handling - production-ready

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
go run main.go
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
go run main_abi.go
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

**Plugins:**
- [plugin.cpp](plugin.cpp) - Simple single-function plugin
- [plugin_abi.cpp](plugin_abi.cpp) - Production ABI plugin with lifecycle and versioning

**Go Hosts:**
- [main.go](main.go) - Simple host for plugin.wasm
- [main_abi.go](main_abi.go) - Full ABI host with version checking and lifecycle

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

## Go Host Program

### Simple Host (main.go)

Direct execution without lifecycle management:

```bash
go run main.go
# Output: Result: 43
```

### ABI Host (main_abi.go)

Full production implementation with:
- Version compatibility checking
- Proper init/cleanup lifecycle
- Error code handling
- Diagnostic information

```bash
go run main_abi.go
# Output:
# Plugin ABI version: v1.0.0
# Plugin initialized successfully
# Result: 43
# Total process() calls: 1
# Plugin cleaned up successfully
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

## Learn More

- [ABI.md](ABI.md) - Complete ABI design guide with:
  - Why `extern "C"` is required
  - How Go discovers and calls exports
  - Versioning strategy and forward compatibility
  - 8 common pitfalls with solutions
  - Best practices for production plugins

- [BUILD.md](BUILD.md) - Detailed compilation guide with explanations for every compiler flag
