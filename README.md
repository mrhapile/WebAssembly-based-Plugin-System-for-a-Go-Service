# WebAssembly-based-Plugin-System-for-a-Go-Service
Build a Go microservice that can load and execute WebAssembly plugins written in C/C++ at runtime.

## Minimal C++ WASM Plugin

This project includes a minimal C++ WebAssembly plugin that demonstrates:
- Zero standard library dependencies
- No dynamic memory allocation
- Pure deterministic computation
- WASI reactor module (library, not command)
- Exported C ABI function callable from Go or WasmEdge CLI

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

**3. Verify exports:**
```bash
wasm-objdump -x plugin.wasm | grep -A5 "Export"
```

### Files

- [plugin.cpp](plugin.cpp) - Minimal C++ source with exported `process(int)` function
- [BUILD.md](BUILD.md) - Detailed compilation guide with flag explanations

### Requirements

- clang++ with WebAssembly support (LLVM 8+)
- WasmEdge runtime for testing
- wasm-objdump (optional, for inspection)

### Plugin Function

```cpp
extern "C" int process(int x);
```

The current implementation returns `(x * 2) + 1` but can be modified for any deterministic computation.
