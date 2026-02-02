package main

import (
	"fmt"
	"os"

	"github.com/second-state/WasmEdge-go/wasmedge"
)

func main() {
	// Step 1: Create WasmEdge configuration
	// Configure VM with WASI support for wasm32-wasi modules
	conf := wasmedge.NewConfigure(wasmedge.WASI)
	defer conf.Release()

	// Step 2: Create WasmEdge VM with the configuration
	// This initializes the runtime environment
	vm := wasmedge.NewVMWithConfig(conf)
	defer vm.Release()

	// Step 3: Initialize WASI interface
	// Required for wasm32-wasi modules even if not using WASI syscalls
	wasi := vm.GetImportModule(wasmedge.WASI)
	wasi.InitWasi(
		os.Args[1:],   // Command line arguments
		os.Environ(),  // Environment variables
		[]string{"."}, // Pre-opened directories
	)

	// Step 4: Load WASM file from disk
	// Reads the compiled plugin_abi.wasm into memory
	err := vm.LoadWasmFile("plugin_abi.wasm")
	if err != nil {
		fmt.Printf("Error loading WASM file: %v\n", err)
		os.Exit(1)
	}

	// Step 5: Validate the WASM module
	// Checks if module structure and instructions are valid
	err = vm.Validate()
	if err != nil {
		fmt.Printf("Error validating WASM module: %v\n", err)
		os.Exit(1)
	}

	// Step 6: Instantiate the module
	// Allocates memory and prepares module for execution
	err = vm.Instantiate()
	if err != nil {
		fmt.Printf("Error instantiating WASM module: %v\n", err)
		os.Exit(1)
	}

	// ========================================================================
	// ABI LIFECYCLE: Version Check -> Init -> Process -> Cleanup
	// ========================================================================

	// Step 7: Check ABI version for compatibility
	// Query the plugin's ABI version to ensure compatibility
	result, err := vm.Execute("get_abi_version")
	if err != nil {
		fmt.Printf("Warning: Plugin does not export get_abi_version: %v\n", err)
		fmt.Println("Continuing without version check (backward compatibility)")
	} else {
		version := result[0].(int32)
		major := version / 10000
		minor := (version % 10000) / 100
		patch := version % 100
		fmt.Printf("Plugin ABI version: v%d.%d.%d\n", major, minor, patch)

		// Check major version compatibility
		requiredMajor := int32(1)
		if major != requiredMajor {
			fmt.Printf("Error: Incompatible ABI version (expected major %d, got %d)\n", requiredMajor, major)
			os.Exit(1)
		}
	}

	// Step 8: Initialize the plugin
	// Must be called before any process() calls
	result, err = vm.Execute("init")
	if err != nil {
		fmt.Printf("Error calling init: %v\n", err)
		os.Exit(1)
	}
	initReturn := result[0].(int32)
	if initReturn != 0 {
		fmt.Printf("Error: init() returned error code %d\n", initReturn)
		os.Exit(1)
	}
	fmt.Println("Plugin initialized successfully")

	// Ensure cleanup is called even if process fails
	defer func() {
		result, err := vm.Execute("cleanup")
		if err != nil {
			fmt.Printf("Warning: cleanup failed: %v\n", err)
		} else if result[0].(int32) != 0 {
			fmt.Printf("Warning: cleanup returned error code %d\n", result[0].(int32))
		} else {
			fmt.Println("Plugin cleaned up successfully")
		}
	}()

	// Step 9: Execute the core "process" function
	// Call with integer argument 21
	input := int32(21)
	result, err = vm.Execute("process", input)
	if err != nil {
		fmt.Printf("Error executing process: %v\n", err)
		os.Exit(1)
	}

	// Step 10: Check return value and handle errors
	// Return values: 0 = success, >0 = result, <0 = error code
	if len(result) == 0 {
		fmt.Println("Error: No result returned from process()")
		os.Exit(1)
	}

	returnValue := result[0].(int32)
	if returnValue < 0 {
		fmt.Printf("Error: process() returned error code %d\n", returnValue)
		switch returnValue {
		case -1:
			fmt.Println("  ABI_ERROR_NOT_INITIALIZED")
		case -3:
			fmt.Println("  ABI_ERROR_INVALID_INPUT")
		case -4:
			fmt.Println("  ABI_ERROR_INTERNAL")
		}
		os.Exit(1)
	}

	// Success - print the result
	fmt.Printf("Result: %d\n", returnValue)

	// Step 11 (Optional): Query diagnostic information
	result, err = vm.Execute("get_call_count")
	if err == nil {
		callCount := result[0].(int32)
		fmt.Printf("Total process() calls: %d\n", callCount)
	}
}
