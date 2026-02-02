// Hello Plugin - Example WASM plugin following the stable ABI
// 
// Build command:
// clang++ --target=wasm32-wasi -nostdlib -Wl,--no-entry \
//   -Wl,--export=init -Wl,--export=process -Wl,--export=cleanup \
//   -O3 -o hello.wasm hello.cpp

#define ABI_SUCCESS 0
#define ABI_ERROR_NOT_INITIALIZED -1

static int initialized = 0;

extern "C" int init() {
    initialized = 1;
    return ABI_SUCCESS;
}

extern "C" int process(int input) {
    if (!initialized) {
        return ABI_ERROR_NOT_INITIALIZED;
    }
    // Compute: (input * 2) + 1
    return (input * 2) + 1;
}

extern "C" int cleanup() {
    initialized = 0;
    return ABI_SUCCESS;
}
