# Building the WebAssembly Plugin

## Compilation Command

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

```bash
# Call the process function with argument 21
wasmedge --reactor plugin.wasm process 21

# Expected output: 43 (because 21 * 2 + 1 = 43)
```

Note: The `--reactor` flag tells WasmEdge to treat this as a reactor module (library) rather than a command module with _start.
