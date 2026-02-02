# WebAssembly Plugin ABI Design

## Overview

This document describes a stable Application Binary Interface (ABI) for WebAssembly plugins compiled from C++ and executed by Go hosts using WasmEdge.

## Core ABI Principles

### 1. C Linkage Requirement

**All exported functions MUST use `extern "C"`**

```cpp
// ❌ WRONG - C++ name mangling
int process(int x);
// Symbol name: _Z7processi (unpredictable)

// ✅ CORRECT - C linkage
extern "C" int process(int x);
// Symbol name: process (predictable)
```

**Why this matters:**
- C++ compilers mangle function names to support overloading
- Mangled names are compiler-specific and unpredictable
- Go host cannot discover or call mangled exports
- `extern "C"` guarantees stable symbol names across compilers

### 2. Required Exports

Every conforming plugin MUST export:

```cpp
extern "C" int get_abi_version();  // Returns version number
extern "C" int init();              // Initialize plugin
extern "C" int process(int input);  // Core logic
extern "C" int cleanup();           // Release resources
```

### 3. Return Value Convention

| Return Value | Meaning |
|--------------|---------|
| `0` | Success |
| `> 0` | Valid result (for data-returning functions) |
| `< 0` | Error code |

**Error Codes:**
```cpp
#define ABI_SUCCESS                    0
#define ABI_ERROR_NOT_INITIALIZED     -1
#define ABI_ERROR_ALREADY_INITIALIZED -2
#define ABI_ERROR_INVALID_INPUT       -3
#define ABI_ERROR_INTERNAL            -4
```

### 4. Type Restrictions

**Allowed:**
- `int` (i32 in WASM)
- `long long` (i64 in WASM)
- `float` (f32 in WASM)
- `double` (f64 in WASM)

**Prohibited:**
- Pointers (no shared memory model yet)
- Structs (layout may differ between languages)
- C++ objects (no C++ ABI across languages)
- Arrays (use multiple calls instead)
- Exceptions (must use error codes)

## ABI Versioning Strategy

### Version Number Format

```
version = MAJOR * 10000 + MINOR * 100 + PATCH
```

Examples:
- `10000` = v1.0.0
- `10001` = v1.0.1
- `10100` = v1.1.0
- `20000` = v2.0.0

### Version Change Guidelines

**MAJOR** (breaking changes):
- Changing function signatures
- Removing exports
- Changing error code meanings
- Requires host update

**MINOR** (backward compatible):
- Adding new optional exports
- Adding new error codes
- Old hosts can still use plugin

**PATCH** (bug fixes):
- Internal implementation fixes
- No ABI changes
- Fully compatible

### Go Host Version Checking

```go
// Query plugin ABI version
result, err := vm.Execute("get_abi_version")
if err != nil {
    return fmt.Errorf("plugin does not support ABI versioning")
}

pluginVersion := result[0].(int32)
requiredMajor := int32(1)
pluginMajor := pluginVersion / 10000

if pluginMajor != requiredMajor {
    return fmt.Errorf("incompatible ABI version: got %d, need major %d", 
        pluginVersion, requiredMajor)
}
```

## Go Host Discovery Pattern

### 1. Load and Validate Module

```go
vm := wasmedge.NewVMWithConfig(conf)
vm.LoadWasmFile("plugin.wasm")
vm.Validate()
vm.Instantiate()
```

### 2. Check ABI Version

```go
result, err := vm.Execute("get_abi_version")
if err != nil {
    // Old plugin without versioning
    log.Warn("Plugin does not export get_abi_version")
}
// Verify compatibility
```

### 3. Initialize Plugin

```go
result, err := vm.Execute("init")
if err != nil {
    return fmt.Errorf("init failed: %v", err)
}
if result[0].(int32) != 0 {
    return fmt.Errorf("init returned error: %d", result[0])
}
```

### 4. Execute Core Logic

```go
result, err := vm.Execute("process", int32(21))
if err != nil {
    return fmt.Errorf("process failed: %v", err)
}

returnValue := result[0].(int32)
if returnValue < 0 {
    return fmt.Errorf("process error: %d", returnValue)
}
// Use returnValue
```

### 5. Cleanup

```go
result, err := vm.Execute("cleanup")
if err != nil {
    log.Warn("cleanup failed: %v", err)
}
vm.Release()
```

### 6. Optional Export Discovery

```go
// Check if optional export exists
result, err := vm.Execute("get_statistics")
if err != nil {
    // Export doesn't exist, skip gracefully
    log.Debug("Plugin does not support statistics")
} else {
    stats := result[0].(int32)
    // Use stats
}
```

## Common ABI Pitfalls

### 1. **Name Mangling**

**Problem:** Forgetting `extern "C"` leads to mangled symbols

```cpp
// ❌ Plugin exports _Z7processi
int process(int x) { return x * 2; }
```

```go
// ❌ Go host cannot find "process"
vm.Execute("process", 21)  // Error: function not found
```

**Solution:** Always use `extern "C"`

```cpp
// ✅ Plugin exports process
extern "C" int process(int x) { return x * 2; }
```

### 2. **Struct Passing**

**Problem:** Struct layout differs between C++ and Go

```cpp
// ❌ WRONG - struct layout is undefined across languages
struct Config {
    int width;
    int height;
};
extern "C" void configure(Config* cfg);  // DANGEROUS
```

**Solution:** Pass primitive types separately or serialize

```cpp
// ✅ CORRECT
extern "C" int configure(int width, int height);
```

### 3. **Pointer Abuse**

**Problem:** WASM has separate memory space from host

```cpp
// ❌ WRONG - pointer is meaningless to Go host
extern "C" const char* get_message();
```

**Solution:** Use integer indices or avoid pointers entirely

```cpp
// ✅ CORRECT - return string via memory offset (advanced)
// Or better: use multiple integer returns
extern "C" int get_message_length();
extern "C" int get_message_char(int index);
```

### 4. **Exception Throwing**

**Problem:** C++ exceptions don't cross WASM boundary

```cpp
// ❌ WRONG - exception will trap WASM module
extern "C" int process(int x) {
    if (x < 0) throw std::invalid_argument("negative");
    return x * 2;
}
```

**Solution:** Use error codes

```cpp
// ✅ CORRECT
extern "C" int process(int x) {
    if (x < 0) return ABI_ERROR_INVALID_INPUT;
    return x * 2;
}
```

### 5. **Missing Initialization**

**Problem:** Plugin state not initialized before use

```go
// ❌ WRONG - calling process before init
vm.Execute("process", 21)  // May return ABI_ERROR_NOT_INITIALIZED
```

**Solution:** Always follow lifecycle order

```go
// ✅ CORRECT
vm.Execute("init")
vm.Execute("process", 21)
vm.Execute("cleanup")
```

### 6. **No Version Checking**

**Problem:** Host and plugin may be incompatible

```go
// ❌ WRONG - assuming plugin is compatible
vm.Execute("process", 21)  // May fail if ABI changed
```

**Solution:** Check version first

```go
// ✅ CORRECT
result, _ := vm.Execute("get_abi_version")
if result[0].(int32) / 10000 != 1 {
    return errors.New("incompatible plugin version")
}
```

### 7. **Memory Leaks**

**Problem:** Not calling cleanup()

```go
// ❌ WRONG
vm.Execute("init")
vm.Execute("process", 21)
vm.Release()  // Plugin cleanup() never called
```

**Solution:** Always cleanup

```go
// ✅ CORRECT
defer vm.Release()
vm.Execute("init")
defer vm.Execute("cleanup")
vm.Execute("process", 21)
```

### 8. **Non-Deterministic Behavior**

**Problem:** Using time, random, or syscalls

```cpp
// ❌ WRONG - not deterministic
extern "C" int process(int x) {
    return x * time(NULL);  // Different result each call
}
```

**Solution:** Pure functions only

```cpp
// ✅ CORRECT
extern "C" int process(int x) {
    return x * 2;  // Same input = same output
}
```

## Compilation Flags for ABI

```bash
clang++ \
  --target=wasm32-wasi \
  -nostdlib \              # No C++ stdlib (prevents ABI dependencies)
  -Wl,--no-entry \         # No _start (reactor module)
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

## Future ABI Extensions

### Potential v1.1.0 Features
- `get_metadata()` - return plugin info
- `validate_input(int)` - check input without processing
- `get_error_message(int)` - human-readable error strings

### Potential v2.0.0 Changes
- Linear memory sharing for bulk data
- `process_batch(int* inputs, int count)` - bulk operations
- String passing via memory buffer
- Callback functions for progress reporting

## Best Practices

1. **Always version your ABI** - Export `get_abi_version()`
2. **Document error codes** - Make them discoverable
3. **Keep it simple** - Fewer exports = more stable
4. **Test across compilers** - Verify with clang++ and g++
5. **Use CI/CD** - Automate compatibility testing
6. **Log everything** - Version checks, init/cleanup status
7. **Fail fast** - Return errors immediately, don't continue
8. **Make cleanup idempotent** - Safe to call multiple times

## Reference Implementation

See [plugin_abi.cpp](plugin_abi.cpp) for a complete working example implementing this ABI specification.
