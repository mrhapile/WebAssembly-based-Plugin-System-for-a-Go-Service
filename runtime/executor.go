package runtime

import (
	"fmt"
)

// ABI error codes returned by plugin functions
const (
	ABISuccess                 = 0  // Operation completed successfully
	ABIErrorNotInitialized     = -1 // Plugin not initialized (init not called)
	ABIErrorAlreadyInitialized = -2 // Plugin already initialized (init called twice)
	ABIErrorInvalidInput       = -3 // Invalid input parameter
	ABIErrorInternal           = -4 // Internal plugin error
)

// Init initializes the plugin by calling its exported "init" function.
//
// This must be called once before any Execute() calls. Calling Init() multiple
// times may return an error depending on the plugin implementation.
//
// Returns an error if:
// - The plugin does not export an "init" function
// - The init function returns a non-zero error code
// - The VM is in an invalid state
func (p *Plugin) Init() error {
	if p.vm == nil {
		return fmt.Errorf("plugin is closed")
	}

	// Call the exported "init" function
	// Expected signature: int init()
	result, err := p.vm.Execute("init")
	if err != nil {
		return fmt.Errorf("failed to execute init() for %s: %w", p.path, err)
	}

	// Check that we got a return value
	if len(result) == 0 {
		return fmt.Errorf("init() did not return a value for %s", p.path)
	}

	// Extract return code (i32 -> int32)
	returnCode := result[0].(int32)

	// Check for error codes
	if returnCode != ABISuccess {
		return fmt.Errorf("init() returned error code %d for %s: %s",
			returnCode, p.path, abiErrorString(returnCode))
	}

	return nil
}

// Execute calls the plugin's "process" function with the given input.
//
// The plugin must be initialized with Init() before calling Execute().
// Execute() can be called multiple times after a successful Init().
//
// Returns the result value from the plugin, or an error if:
// - The plugin does not export a "process" function
// - The process function returns a negative error code
// - The VM is in an invalid state
func (p *Plugin) Execute(input int) (int, error) {
	if p.vm == nil {
		return 0, fmt.Errorf("plugin is closed")
	}

	// Call the exported "process" function with int32 argument
	// Expected signature: int process(int)
	result, err := p.vm.Execute("process", int32(input))
	if err != nil {
		return 0, fmt.Errorf("failed to execute process(%d) for %s: %w",
			input, p.path, err)
	}

	// Check that we got a return value
	if len(result) == 0 {
		return 0, fmt.Errorf("process() did not return a value for %s", p.path)
	}

	// Extract return value (i32 -> int32)
	returnValue := result[0].(int32)

	// Check for error codes (negative values indicate errors)
	if returnValue < 0 {
		return 0, fmt.Errorf("process() returned error code %d for %s: %s",
			returnValue, p.path, abiErrorString(returnValue))
	}

	// Success - return the computed result
	return int(returnValue), nil
}

// Cleanup calls the plugin's "cleanup" function to release any resources.
//
// This should be called when the plugin is no longer needed, before Close().
// It's safe to call Cleanup() even if Init() was never called or failed.
//
// Returns an error if:
// - The plugin does not export a "cleanup" function
// - The cleanup function returns a non-zero error code
// - The VM is in an invalid state
func (p *Plugin) Cleanup() error {
	if p.vm == nil {
		return fmt.Errorf("plugin is closed")
	}

	// Call the exported "cleanup" function
	// Expected signature: int cleanup()
	result, err := p.vm.Execute("cleanup")
	if err != nil {
		return fmt.Errorf("failed to execute cleanup() for %s: %w", p.path, err)
	}

	// Check that we got a return value
	if len(result) == 0 {
		return fmt.Errorf("cleanup() did not return a value for %s", p.path)
	}

	// Extract return code (i32 -> int32)
	returnCode := result[0].(int32)

	// Check for error codes
	if returnCode != ABISuccess {
		return fmt.Errorf("cleanup() returned error code %d for %s: %s",
			returnCode, p.path, abiErrorString(returnCode))
	}

	return nil
}

// abiErrorString converts ABI error codes to human-readable strings.
func abiErrorString(code int32) string {
	switch code {
	case ABISuccess:
		return "success"
	case ABIErrorNotInitialized:
		return "ABI_ERROR_NOT_INITIALIZED"
	case ABIErrorAlreadyInitialized:
		return "ABI_ERROR_ALREADY_INITIALIZED"
	case ABIErrorInvalidInput:
		return "ABI_ERROR_INVALID_INPUT"
	case ABIErrorInternal:
		return "ABI_ERROR_INTERNAL"
	default:
		return fmt.Sprintf("unknown error code %d", code)
	}
}
