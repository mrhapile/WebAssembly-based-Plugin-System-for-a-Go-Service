// Minimal C++ WebAssembly Plugin for WasmEdge
// No standard library, no dynamic memory, pure computation

// Export function with C linkage to avoid C++ name mangling
// This ensures the function name in WASM is exactly "process"
extern "C" int process(int x) {
    // Simple deterministic computation
    // Example: multiply by 2 and add 1
    return (x * 2) + 1;
}

// Note: No main() or _start() function
// This creates a WASI reactor module (library) not a command module
