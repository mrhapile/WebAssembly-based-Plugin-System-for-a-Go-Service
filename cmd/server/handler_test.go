package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

	"github.com/mrhapile/wasm-plugin-system/fluid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
)

var _ = Describe("HTTP Server", func() {
	var (
		server *httptest.Server
		store  fluid.PluginStore
	)

	// Setup: Create test server before each test
	BeforeEach(func() {
		// Use LocalPluginStore for testing (relative to project root)
		store = fluid.NewLocalPluginStore("plugins")

		// Create server with the test store
		srv := NewServer(store)

		// Use httptest.Server to create a local HTTP server for testing
		// This avoids binding to actual ports and is safe for parallel tests
		mux := http.NewServeMux()
		mux.HandleFunc("/run", srv.handleRun)
		server = httptest.NewServer(mux)
	})

	// Cleanup: Shut down server after each test
	AfterEach(func() {
		if server != nil {
			server.Close()
			server = nil
		}
	})

	// =========================================================================
	// TEST: Successful plugin execution via HTTP
	// Why: End-to-end test that the HTTP API correctly loads, executes, and
	//      returns results from a WASM plugin.
	// =========================================================================
	Describe("POST /run", func() {
		Context("with a valid request", func() {
			BeforeEach(func() {
				// Check if hello plugin exists
				pluginPath := filepath.Join("plugins", "hello", "hello.wasm")
				if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
					Skip("Test plugin not found: " + pluginPath)
				}
			})

			It("should return correct output", func() {
				// Change to project root for plugin path resolution
				originalDir, _ := os.Getwd()
				os.Chdir(filepath.Join("..", ".."))
				defer os.Chdir(originalDir)

				reqBody := Request{Plugin: "hello", Input: 21}
				jsonBody, _ := json.Marshal(reqBody)

				resp, err := http.Post(server.URL+"/run", "application/json", bytes.NewBuffer(jsonBody))

				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				var response Response
				err = json.NewDecoder(resp.Body).Decode(&response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Output).To(Equal(43)) // 21 * 2 + 1 = 43
			})
		})

		// =====================================================================
		// TEST: Invalid JSON input
		// Why: Server must return 400 Bad Request for malformed JSON, not crash
		//      or return 500.
		// =====================================================================
		Context("with invalid JSON", func() {
			It("should return 400 Bad Request", func() {
				invalidJSON := []byte(`{"plugin": "hello", "input": }`) // Invalid JSON

				resp, err := http.Post(server.URL+"/run", "application/json", bytes.NewBuffer(invalidJSON))

				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

				var errorResp ErrorResponse
				json.NewDecoder(resp.Body).Decode(&errorResp)
				Expect(errorResp.Error).To(ContainSubstring("invalid JSON"))
			})
		})

		// =====================================================================
		// TEST: Empty plugin name
		// Why: Plugin name is required. Empty string must be rejected with 400.
		// =====================================================================
		Context("with empty plugin name", func() {
			It("should return 400 Bad Request", func() {
				reqBody := Request{Plugin: "", Input: 21}
				jsonBody, _ := json.Marshal(reqBody)

				resp, err := http.Post(server.URL+"/run", "application/json", bytes.NewBuffer(jsonBody))

				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

				var errorResp ErrorResponse
				json.NewDecoder(resp.Body).Decode(&errorResp)
				Expect(errorResp.Error).To(ContainSubstring("plugin name is required"))
			})
		})

		// =====================================================================
		// TEST: Invalid plugin name (path traversal attempt)
		// Why: Security test - must reject plugin names that could escape the
		//      plugins directory (e.g., "../etc/passwd").
		// =====================================================================
		Context("with invalid plugin name (path traversal)", func() {
			It("should return 400 Bad Request for ../", func() {
				reqBody := Request{Plugin: "../etc/passwd", Input: 21}
				jsonBody, _ := json.Marshal(reqBody)

				resp, err := http.Post(server.URL+"/run", "application/json", bytes.NewBuffer(jsonBody))

				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))

				var errorResp ErrorResponse
				json.NewDecoder(resp.Body).Decode(&errorResp)
				Expect(errorResp.Error).To(ContainSubstring("invalid plugin name"))
			})

			It("should return 400 Bad Request for special characters", func() {
				reqBody := Request{Plugin: "hello;rm -rf /", Input: 21}
				jsonBody, _ := json.Marshal(reqBody)

				resp, err := http.Post(server.URL+"/run", "application/json", bytes.NewBuffer(jsonBody))

				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusBadRequest))
			})
		})

		// =====================================================================
		// TEST: Unknown plugin name
		// Why: Must return 404 when plugin file doesn't exist, with a clear
		//      error message.
		// =====================================================================
		Context("with unknown plugin name", func() {
			It("should return 404 Not Found", func() {
				reqBody := Request{Plugin: "nonexistent", Input: 21}
				jsonBody, _ := json.Marshal(reqBody)

				resp, err := http.Post(server.URL+"/run", "application/json", bytes.NewBuffer(jsonBody))

				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusNotFound))

				var errorResp ErrorResponse
				json.NewDecoder(resp.Body).Decode(&errorResp)
				Expect(errorResp.Error).To(ContainSubstring("plugin not found"))
			})
		})

		// =====================================================================
		// TEST: Wrong HTTP method
		// Why: Only POST is allowed. GET/PUT/DELETE must return 405.
		// =====================================================================
		Context("with wrong HTTP method", func() {
			It("should return 405 for GET", func() {
				resp, err := http.Get(server.URL + "/run")

				Expect(err).NotTo(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusMethodNotAllowed))

				var errorResp ErrorResponse
				json.NewDecoder(resp.Body).Decode(&errorResp)
				Expect(errorResp.Error).To(ContainSubstring("method not allowed"))
			})
		})
	})

	// =========================================================================
	// TEST: Response format validation
	// Why: API contract - responses must have correct Content-Type and JSON
	//      structure.
	// =========================================================================
	Describe("Response Format", func() {
		It("should return application/json Content-Type", func() {
			reqBody := Request{Plugin: "hello", Input: 21}
			jsonBody, _ := json.Marshal(reqBody)

			resp, err := http.Post(server.URL+"/run", "application/json", bytes.NewBuffer(jsonBody))

			Expect(err).NotTo(HaveOccurred())
			Expect(resp.Header.Get("Content-Type")).To(Equal("application/json"))
		})
	})
})

// =========================================================================
// TEST: isValidPluginName unit tests
// Why: This function is critical for security. Test edge cases thoroughly.
// =========================================================================
var _ = Describe("isValidPluginName", func() {
	DescribeTable("validation cases",
		func(name string, expected bool) {
			result := isValidPluginName(name)
			Expect(result).To(Equal(expected))
		},
		// Valid names
		Entry("alphanumeric lowercase", "hello", true),
		Entry("alphanumeric uppercase", "HELLO", true),
		Entry("mixed case", "HelloWorld", true),
		Entry("with numbers", "plugin123", true),
		Entry("with underscore", "my_plugin", true),
		Entry("with hyphen", "my-plugin", true),
		Entry("complex valid", "My_Plugin-123", true),

		// Invalid names
		Entry("empty string", "", false),
		Entry("with slash", "path/to/plugin", false),
		Entry("with backslash", "path\\plugin", false),
		Entry("with dot", "plugin.wasm", false),
		Entry("with space", "my plugin", false),
		Entry("with semicolon", "plugin;rm", false),
		Entry("path traversal", "../etc", false),
		Entry("with null byte", "plugin\x00bad", false),
	)
})

// =========================================================================
// TEST: Using testify for additional assertions
// Why: Demonstrate testify integration where it provides clearer assertions.
// =========================================================================
var _ = Describe("Testify Integration", func() {
	It("should work with testify assertions", func() {
		t := GinkgoT()

		// Test isValidPluginName with testify
		assert.True(t, isValidPluginName("hello"), "hello should be valid")
		assert.False(t, isValidPluginName(""), "empty should be invalid")
		assert.False(t, isValidPluginName("../bad"), "path traversal should be invalid")
	})
})
