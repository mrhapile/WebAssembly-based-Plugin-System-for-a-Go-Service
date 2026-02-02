package runtime

import (
	"fmt"
	"os"

	"github.com/second-state/WasmEdge-go/wasmedge"
)

// Plugin represents a loaded WebAssembly plugin with its own isolated VM instance.
// Each Plugin owns its WasmEdge VM, configuration, and lifecycle state.
// Plugins are not safe for concurrent use - caller must synchronize access.
type Plugin struct {
	path   string                // Original file path for error reporting
	vm     *wasmedge.VM          // WasmEdge VM instance (owns module execution)
	config *wasmedge.Configure   // VM configuration (WASI support)
}

// LoadPlugin loads a WebAssembly module from disk and creates an isolated VM instance.
//
// The function performs the complete loading sequence:
// 1. Creates WasmEdge configuration with WASI support
// 2. Initializes a new VM with the configuration
// 3. Initializes WASI interface (required for wasm32-wasi modules)
// 4. Loads the WASM file from disk
// 5. Validates module structure and bytecode
// 6. Instantiates the module (allocates memory, prepares exports)
//
// If any step fails, all resources are cleaned up before returning the error.
// The returned Plugin must be closed with Close() when no longer needed.
//
// Example:
//   plugin, err := runtime.LoadPlugin("plugin.wasm")
//   if err != nil {
//       return err
//   }
//   defer plugin.Close()
func LoadPlugin(path string) (*Plugin, error) {
	// Verify file exists before attempting to load
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("plugin file not found: %w", err)
	}

	// Step 1: Create configuration with WASI support
	// This enables wasm32-wasi modules to work even if they don't use WASI syscalls
	config := wasmedge.NewConfigure(wasmedge.WASI)
	if config == nil {
		return nil, fmt.Errorf("failed to create WasmEdge configuration")
	}

	// Step 2: Create VM instance with the configuration
	// Each plugin gets its own isolated VM for sandboxing
	vm := wasmedge.NewVMWithConfig(config)
	if vm == nil {
		config.Release()
		return nil, fmt.Errorf("failed to create WasmEdge VM")
	}

	// Step 3: Initialize WASI interface
	// Required for wasm32-wasi target even if plugin doesn't use WASI features
	wasi := vm.GetImportModule(wasmedge.WASI)
	if wasi == nil {
		vm.Release()
		config.Release()
		return nil, fmt.Errorf("failed to get WASI module")
	}
	
	// Initialize WASI with minimal environment
	// No command-line args, inherit host environment, no pre-opened directories
	wasi.InitWasi(
		[]string{},      // No command-line arguments
		os.Environ(),    // Inherit host environment variables
		[]string{},      // No pre-opened directories (sandbox)
	)

	// Step 4: Load WASM file from disk
	// Reads and parses the WebAssembly binary
	if err := vm.LoadWasmFile(path); err != nil {
		vm.Release()
		config.Release()
		return nil, fmt.Errorf("failed to load WASM file %s: %w", path, err)
	}

	// Step 5: Validate the module
	// Verifies bytecode structure, type checking, and instruction validity
	if err := vm.Validate(); err != nil {
		vm.Release()
		config.Release()
		return nil, fmt.Errorf("WASM module validation failed for %s: %w", path, err)
	}

	// Step 6: Instantiate the module
	// Allocates linear memory, initializes globals, runs start functions (if any)
	// After this point, exports are callable
	if err := vm.Instantiate(); err != nil {
		vm.Release()
		config.Release()
		return nil, fmt.Errorf("WASM module instantiation failed for %s: %w", path, err)
	}

	// Success - return initialized plugin
	return &Plugin{
		path:   path,
		vm:     vm,
		config: config,
	}, nil
}

// Close releases all VM resources owned by this plugin.
//
// This method must be called when the plugin is no longer needed to prevent
// resource leaks. It's safe to call Close() multiple times - subsequent calls
// are no-ops.
//
// After Close() is called, Init(), Execute(), and Cleanup() must not be called.
//
// Example:
//   plugin, _ := runtime.LoadPlugin("plugin.wasm")
//   defer plugin.Close()
func (p *Plugin) Close() {
	if p.vm != nil {
		p.vm.Release()
		p.vm = nil
	}
	if p.config != nil {
		p.config.Release()
		p.config = nil
	}
}

// Path returns the original file path of the loaded plugin.
// Useful for logging and error reporting.
func (p *Plugin) Path() string {
	return p.path
}
