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
- [main.go](main.go) - Go program that loads and executes the WASM plugin
- [go.mod](go.mod) - Go module with wasmedge-go dependency

### Requirements

- clang++ with WebAssembly support (LLVM 8+)
- WasmEdge runtime (0.13.0+)
- Go 1.21+
- wasmedge-go SDK
- wasm-objdump (optional, for inspection)

### Plugin Function

```cpp
extern "C" int process(int x);
```

The current implementation returns `(x * 2) + 1` but can be modified for any deterministic computation.

## Go Host Program

The Go program demonstrates how to load and execute the WASM plugin from your microservice.

### Setup

**1. Install WasmEdge:**
```bash
curl -sSf https://raw.githubusercontent.com/WasmEdge/WasmEdge/master/utils/install.sh | bash
```

**2. Install Go dependencies:**
```bash
go mod download
```

**3. Compile the WASM plugin:**
```bash
clang++ --target=wasm32-wasi -nostdlib -Wl,--no-entry -Wl,--export=process -O3 -o plugin.wasm plugin.cpp
```

### Running

**Execute the Go program:**
```bash
go run main.go
# Output: Result: 43
```

The program:
1. Initializes WasmEdge VM with WASI support
2. Loads `plugin.wasm` from disk
3. Validates the module structure
4. Instantiates the module
5. Executes `process(21)` function
6. Prints the result: `43` (since 21 × 2 + 1 = 43)
7. Releases all VM resources

### Key Implementation Details

- **No global state**: VM is created and destroyed per execution
- **Explicit error handling**: Each step checks for errors
- **Resource cleanup**: `vm.Release()` and `conf.Release()` free memory
- **WASI initialization**: Required even for modules that don't use WASI syscalls
- **Type conversion**: Results are cast from `interface{}` to `int32`
