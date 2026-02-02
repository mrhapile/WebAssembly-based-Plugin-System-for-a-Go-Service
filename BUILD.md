# Building the WebAssembly Plugin

## Simple Plugin (plugin.cpp)

Basic single-function plugin for quick testing:

```bash
clang++ \
  --target=wasm32-wasi \
  -nostdlib \
  -Wl,--no-entry \
  -Wl,--export=process \
  -O3 \
  -o plugin.wasm \
  plugin.cpp
```

## Stable ABI Plugin (plugin_abi.cpp)

Production-ready plugin with full lifecycle and versioning:

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

## Flag Explanations

### `--target=wasm32-wasi`
- Sets the compilation target to WebAssembly with WASI (WebAssembly System Interface)
- Produces wasm32 architecture output compatible with WASI runtimes like WasmEdge

### `-nostdlib`
- Disables linking with the C++ standard library
- Prevents inclusion of unnecessary runtime code (malloc, iostream, etc.)
- Results in minimal binary size and no external dependencies
- Required for pure computational plugins without standard library features

### `-Wl,--no-entry`
- Linker flag that creates a WASI reactor module instead of a command module
- Prevents generation of `_start` function (entry point)
- Without this flag, linker would require a main() function
- Essential for creating a library/plugin that exports functions to be called externally

### `-Wl,--export=process`
- Explicitly exports the `process` function from the WASM module
- Makes the function callable from the host (Go service or WasmEdge CLI)
- Without this, the function would be internal and inaccessible
- Can specify multiple exports: `-Wl,--export=func1 -Wl,--export=func2`

### `-O3`
- Enables maximum optimization level
- Produces smaller and faster WASM code
- Safe for deterministic pure functions
- Reduces code size by 50-70% compared to unoptimized builds

### `-o plugin.wasm`
- Specifies output file name
- Extension should be `.wasm` for WebAssembly modules

## Verification

After compilation, you can inspect the WASM module:

```bash
# List exported functions
wasm-objdump -x plugin.wasm | grep -A5 "Export"

# Should show:
# - Export[0]:
#   - name: "process"
```

## Running with WasmEdge

### Simple Plugin (plugin.wasm)

```bash
# Call the process function with argument 21
wasmedge --reactor plugin.wasm process 21

# Expected output: 43 (because 21 * 2 + 1 = 43)
```

### ABI Plugin (plugin_abi.wasm)

```bash
# Check version
wasmedge --reactor plugin_abi.wasm get_abi_version
# Output: 10000 (v1.0.0)

# Execute with proper lifecycle
wasmedge --reactor plugin_abi.wasm init
# Output: 0 (success)

wasmedge --reactor plugin_abi.wasm process 21
# Output: 43

wasmedge --reactor plugin_abi.wasm cleanup
# Output: 0 (success)
```

Note: The `--reactor` flag tells WasmEdge to treat this as a reactor module (library) rather than a command module with _start.

## ABI Verification

After compiling plugin_abi.wasm, verify all required exports:

```bash
wasm-objdump -x plugin_abi.wasm | grep -A20 "Export"
```

Should show:
```
Export[0]:
  - func[0] <get_abi_version>
Export[1]:
  - func[1] <init>
Export[2]:
  - func[2] <process>
Export[3]:
  - func[3] <cleanup>
Export[4]:
  - func[4] <get_call_count>
Export[5]:
  - func[5] <is_initialized>
```
