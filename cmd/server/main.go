package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/mrhapile/wasm-plugin-system/fluid"
	"github.com/mrhapile/wasm-plugin-system/runtime"
)

// Server encapsulates the HTTP server dependencies.
//
// Using a struct instead of global variables allows:
//   - Easy testing with mock PluginStore
//   - Multiple server instances with different configurations
//   - Clear dependency injection
type Server struct {
	store fluid.PluginStore
}

// NewServer creates a Server with the given plugin store.
func NewServer(store fluid.PluginStore) *Server {
	return &Server{store: store}
}

// Request represents the JSON request body for POST /run
type Request struct {
	Plugin string `json:"plugin"` // Plugin name (e.g., "hello")
	Input  int    `json:"input"`  // Integer input to pass to process()
}

// Response represents the JSON response body
type Response struct {
	Output int `json:"output"` // Result from plugin's process() function
}

// ErrorResponse represents an error in JSON format
type ErrorResponse struct {
	Error string `json:"error"` // Human-readable error message
}

// handleRun handles POST /run requests
//
// Request lifecycle per call:
// 1. Parse and validate JSON request
// 2. Resolve plugin path via PluginStore
// 3. Load plugin (creates isolated VM)
// 4. Initialize plugin (calls init())
// 5. Execute plugin (calls process(input))
// 6. Cleanup plugin (calls cleanup())
// 7. Close VM (release all resources)
// 8. Return JSON response
//
// On any error, cleanup is guaranteed via defer.
func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	// Only accept POST requests
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse JSON request body
	var req Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("invalid JSON: %v", err))
		return
	}

	// Validate plugin name (basic sanitization)
	if req.Plugin == "" {
		writeError(w, http.StatusBadRequest, "plugin name is required")
		return
	}
	if !isValidPluginName(req.Plugin) {
		writeError(w, http.StatusBadRequest, "invalid plugin name")
		return
	}

	// Resolve plugin path via PluginStore
	// This abstracts the difference between local and Fluid storage
	pluginPath, err := s.store.Resolve(req.Plugin)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("plugin not found: %s", req.Plugin))
		return
	}

	// Execute plugin with full lifecycle management
	output, err := executePlugin(pluginPath, req.Input)
	if err != nil {
		// Determine appropriate HTTP status code based on error
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Return successful response
	writeJSON(w, http.StatusOK, Response{Output: output})
}

// executePlugin loads, initializes, executes, and cleans up a plugin
//
// This function guarantees:
// - Plugin is always closed (VM resources released)
// - Cleanup is called if init succeeded
// - Errors are wrapped with context
func executePlugin(pluginPath string, input int) (int, error) {
	// Step 1: Load the plugin
	// This creates an isolated WasmEdge VM instance
	plugin, err := runtime.LoadPlugin(pluginPath)
	if err != nil {
		return 0, fmt.Errorf("failed to load plugin: %w", err)
	}

	// Guarantee VM resources are released when we're done
	defer plugin.Close()

	// Step 2: Initialize the plugin
	// Calls the exported init() function
	if err := plugin.Init(); err != nil {
		return 0, fmt.Errorf("failed to initialize plugin: %w", err)
	}

	// Guarantee cleanup is called after successful init
	defer func() {
		// Best effort cleanup - don't fail the request if cleanup fails
		_ = plugin.Cleanup()
	}()

	// Step 3: Execute the plugin's process function
	// Calls the exported process(int) function
	output, err := plugin.Execute(input)
	if err != nil {
		return 0, fmt.Errorf("failed to execute plugin: %w", err)
	}

	return output, nil
}

// isValidPluginName checks if the plugin name is safe to use in file paths
// Prevents path traversal attacks (e.g., "../etc/passwd")
func isValidPluginName(name string) bool {
	// Must be non-empty
	if len(name) == 0 {
		return false
	}

	// Only allow alphanumeric, underscore, and hyphen
	for _, c := range name {
		if !((c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_' || c == '-') {
			return false
		}
	}

	return true
}

// writeJSON writes a JSON response with the given status code
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response with the given status code
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{Error: message})
}

func main() {
	// Determine which plugin store to use based on environment.
	//
	// In production with Fluid:
	//   PLUGIN_STORE=fluid
	//   FLUID_MOUNT_PATH=/mnt/fluid/plugins
	//
	// In development (default):
	//   Plugins are loaded from ./plugins/
	var store fluid.PluginStore

	storeType := os.Getenv("PLUGIN_STORE")
	switch storeType {
	case "fluid":
		// Production: use Fluid dataset mount
		mountPath := os.Getenv("FLUID_MOUNT_PATH")
		if mountPath == "" {
			mountPath = "/mnt/fluid/plugins" // Default Fluid mount path
		}
		store = fluid.NewFluidPluginStore(mountPath)
		fmt.Printf("Using Fluid plugin store: %s\n", mountPath)
	default:
		// Development: use local filesystem
		store = fluid.NewLocalPluginStore("./plugins")
		fmt.Println("Using local plugin store: ./plugins")
	}

	// Create server with the plugin store
	server := NewServer(store)

	// Register the /run endpoint
	http.HandleFunc("/run", server.handleRun)

	// Start the server
	addr := ":8080"
	fmt.Printf("Starting WASM plugin server on %s\n", addr)
	fmt.Println("POST /run - Execute a plugin")
	fmt.Println("  Request:  { \"plugin\": \"hello\", \"input\": 21 }")
	fmt.Println("  Response: { \"output\": 43 }")

	if err := http.ListenAndServe(addr, nil); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}
