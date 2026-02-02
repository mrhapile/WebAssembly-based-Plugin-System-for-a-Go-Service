package main

import (
	"fmt"
	"os"

	"github.com/mrhapile/wasm-plugin-system/runtime"
)

func main() {
	// Example: Load and execute a WebAssembly plugin using the runtime package
	
	// Load the plugin from disk
	// This creates an isolated VM instance and validates the module
	plugin, err := runtime.LoadPlugin("plugin_abi.wasm")
	if err != nil {
		fmt.Printf("Error loading plugin: %v\n", err)
		os.Exit(1)
	}
	
	// Ensure VM resources are cleaned up when we're done
	defer plugin.Close()

	fmt.Printf("Loaded plugin: %s\n", plugin.Path())

	// Initialize the plugin
	// Must be called before Execute()
	if err := plugin.Init(); err != nil {
		fmt.Printf("Error initializing plugin: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Plugin initialized successfully")

	// Ensure cleanup is called before Close()
	defer func() {
		if err := plugin.Cleanup(); err != nil {
			fmt.Printf("Warning: cleanup failed: %v\n", err)
		} else {
			fmt.Println("Plugin cleaned up successfully")
		}
	}()

	// Execute the plugin's process function
	input := 21
	result, err := plugin.Execute(input)
	if err != nil {
		fmt.Printf("Error executing plugin: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Result: process(%d) = %d\n", input, result)

	// Execute again to demonstrate multiple calls
	input2 := 50
	result2, err := plugin.Execute(input2)
	if err != nil {
		fmt.Printf("Error executing plugin: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Result: process(%d) = %d\n", input2, result2)
}
