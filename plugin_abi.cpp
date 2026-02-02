// Stable ABI WebAssembly Plugin for Go Host + WasmEdge
// 
// ABI Design Principles:
// 1. C linkage prevents C++ name mangling
// 2. Simple integer types only (no structs, no pointers)
// 3. Explicit lifecycle management (init -> process -> cleanup)
// 4. Zero-based error codes (0 = success, negative = error)
// 5. Version export for forward compatibility

// ============================================================================
// ABI VERSION
// ============================================================================

// Export ABI version as a constant
// The Go host can query this to verify compatibility
// Version format: MAJOR * 10000 + MINOR * 100 + PATCH
// Example: 10000 = v1.0.0, 10001 = v1.0.1, 10100 = v1.1.0
//
// extern "C" is REQUIRED because:
// - C++ compilers perform "name mangling" to support function overloading
// - Example: get_abi_version() might become _Z15get_abi_versionv
// - Go host cannot predict mangled names
// - extern "C" forces the exact symbol name "get_abi_version"
extern "C" int get_abi_version() {
    return 10000;  // v1.0.0
}

// ============================================================================
// ERROR CODES
// ============================================================================

// Define error codes as negative integers
// This allows the host to distinguish success (0) from various failure modes
#define ABI_SUCCESS 0
#define ABI_ERROR_NOT_INITIALIZED -1
#define ABI_ERROR_ALREADY_INITIALIZED -2
#define ABI_ERROR_INVALID_INPUT -3
#define ABI_ERROR_INTERNAL -4

// ============================================================================
// PLUGIN STATE
// ============================================================================

// Minimal state tracking without dynamic memory
// Static variables have lifetime of entire WASM module
// They persist across function calls within same instance
static int plugin_initialized = 0;
static int call_count = 0;

// ============================================================================
// ABI FUNCTION: init
// ============================================================================

// Initialize the plugin
// MUST be called before any process() calls
// 
// Returns:
//   0 on success
//   ABI_ERROR_ALREADY_INITIALIZED if called multiple times
//
// Why extern "C":
// - Without it: symbol becomes _Z4initv (mangled)
// - With it: symbol is exactly "init"
// - Go host can call vm.Execute("init") reliably
extern "C" int init() {
    if (plugin_initialized) {
        return ABI_ERROR_ALREADY_INITIALIZED;
    }
    
    plugin_initialized = 1;
    call_count = 0;
    
    return ABI_SUCCESS;
}

// ============================================================================
// ABI FUNCTION: process
// ============================================================================

// Process input and return result
// Core plugin logic - deterministic and side-effect free
//
// Parameters:
//   input: integer input value
//
// Returns:
//   >= 0: computed result (success)
//   < 0: error code
//
// Implementation notes:
// - Validates initialization state
// - Validates input range
// - Performs deterministic computation
// - No exceptions (disabled by -fno-exceptions)
extern "C" int process(int input) {
    // Guard: ensure init() was called
    if (!plugin_initialized) {
        return ABI_ERROR_NOT_INITIALIZED;
    }
    
    // Validate input range (example: must be positive)
    if (input < 0) {
        return ABI_ERROR_INVALID_INPUT;
    }
    
    // Track usage
    call_count++;
    
    // Deterministic computation: (input * 2) + 1
    // This is side-effect free and always produces same output for same input
    int result = (input * 2) + 1;
    
    // Ensure result is non-negative (to distinguish from error codes)
    if (result < 0) {
        return ABI_ERROR_INTERNAL;
    }
    
    return result;
}

// ============================================================================
// ABI FUNCTION: cleanup
// ============================================================================

// Clean up plugin resources
// SHOULD be called when plugin is no longer needed
//
// Returns:
//   0 on success
//   ABI_ERROR_NOT_INITIALIZED if init() was never called
//
// Notes:
// - In this minimal example, just resets state
// - In complex plugins, would free resources (if using dynamic memory)
// - Safe to call multiple times
extern "C" int cleanup() {
    if (!plugin_initialized) {
        return ABI_ERROR_NOT_INITIALIZED;
    }
    
    plugin_initialized = 0;
    call_count = 0;
    
    return ABI_SUCCESS;
}

// ============================================================================
// OPTIONAL: Diagnostic exports
// ============================================================================

// Get number of times process() was called
// Useful for debugging and monitoring
extern "C" int get_call_count() {
    return call_count;
}

// Check if plugin is initialized
// Returns 1 if initialized, 0 otherwise
extern "C" int is_initialized() {
    return plugin_initialized;
}

// ============================================================================
// ABI STABILITY NOTES
// ============================================================================
//
// WHY THIS ABI IS STABLE:
//
// 1. Fixed Function Signatures
//    - All functions use int parameters and return values
//    - int is 32-bit (i32) in wasm32 target
//    - No structs, pointers, or complex types
//
// 2. C Linkage
//    - extern "C" prevents name mangling
//    - Symbol names are predictable: "init", "process", "cleanup"
//    - Go host can discover exports programmatically
//
// 3. Error Code Convention
//    - 0 = success (universal convention)
//    - Negative = error (allows 2 billion error codes)
//    - Positive = valid result
//
// 4. Versioning Support
//    - get_abi_version() allows compatibility checks
//    - Host can reject incompatible versions
//    - Enables gradual migration
//
// 5. No Side Effects
//    - Functions are deterministic
//    - No file I/O, no syscalls
//    - Results depend only on inputs and internal state
//
// 6. No C++ Runtime Dependencies
//    - Compiled with -nostdlib
//    - No exceptions (-fno-exceptions implicit with -nostdlib)
//    - No RTTI, no virtual functions
//    - Minimal binary size
//
// FORWARD COMPATIBILITY STRATEGY:
//
// - Major version change: Breaking changes (new required exports)
// - Minor version change: New optional exports added
// - Patch version change: Bug fixes, no ABI changes
//
// Example evolution:
//   v1.0.0: init, process, cleanup
//   v1.1.0: + get_statistics (optional, host can check if exists)
//   v2.0.0: process now requires two parameters (breaking change)
//
// ============================================================================
