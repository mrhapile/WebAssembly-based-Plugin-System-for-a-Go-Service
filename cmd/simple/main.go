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

	// Step 2: Create WasmEdge VM with the configuration
	// This initializes the runtime environment
	vm := wasmedge.NewVMWithConfig(conf)

	// Step 3: Initialize WASI interface
	// Required for wasm32-wasi modules even if not using WASI syscalls
	wasi := vm.GetImportModule(wasmedge.WASI)
	wasi.InitWasi(
		os.Args[1:],   // Command line arguments
		os.Environ(),  // Environment variables
		[]string{"."}, // Pre-opened directories
	)

	// Step 4: Load WASM file from disk
	// Reads the compiled plugin.wasm into memory
	err := vm.LoadWasmFile("plugin.wasm")
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

	// Step 7: Execute the exported "process" function
	// Call with integer argument 21
	input := int32(21)
	result, err := vm.Execute("process", input)
	if err != nil {
		fmt.Printf("Error executing function: %v\n", err)
		os.Exit(1)
	}

	// Step 8: Extract and print the result
	// WasmEdge returns results as []interface{}, get first element
	if len(result) > 0 {
		// Cast to int32 (WASM i32 type)
		output := result[0].(int32)
		fmt.Printf("Result: %d\n", output)
	} else {
		fmt.Println("Error: No result returned")
		os.Exit(1)
	}

	// Step 9: Release VM resources
	// Clean up memory and close VM instance
	vm.Release()
	conf.Release()
}
