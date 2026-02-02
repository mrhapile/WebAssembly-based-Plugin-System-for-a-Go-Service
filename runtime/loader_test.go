package runtime_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mrhapile/wasm-plugin-system/runtime"
)

var _ = Describe("Loader", func() {
	var (
		testPluginDir  string
		validPluginPath string
	)

	// Setup: Create test fixtures before each test
	BeforeEach(func() {
		// Use the existing hello plugin for testing
		// This plugin follows the ABI: init(), process(int), cleanup()
		testPluginDir = filepath.Join("..", "plugins", "hello")
		validPluginPath = filepath.Join(testPluginDir, "hello.wasm")
	})

	// =========================================================================
	// TEST: Successful plugin load
	// Why: Verify that LoadPlugin correctly initializes a VM and loads a valid
	//      WASM module. This is the happy path that must work for all other
	//      functionality to work.
	// =========================================================================
	Describe("LoadPlugin", func() {
		Context("with a valid WASM file", func() {
			It("should load the plugin successfully", func() {
				// Skip if test plugin doesn't exist
				if _, err := os.Stat(validPluginPath); os.IsNotExist(err) {
					Skip("Test plugin not found: " + validPluginPath + " - run 'make build-plugins' first")
				}

				plugin, err := runtime.LoadPlugin(validPluginPath)

				Expect(err).NotTo(HaveOccurred())
				Expect(plugin).NotTo(BeNil())
				Expect(plugin.Path()).To(Equal(validPluginPath))

				// Cleanup: Always release resources
				plugin.Close()
			})

			It("should return the correct path", func() {
				if _, err := os.Stat(validPluginPath); os.IsNotExist(err) {
					Skip("Test plugin not found")
				}

				plugin, err := runtime.LoadPlugin(validPluginPath)
				Expect(err).NotTo(HaveOccurred())
				defer plugin.Close()

				Expect(plugin.Path()).To(Equal(validPluginPath))
			})
		})

		// =====================================================================
		// TEST: Missing WASM file
		// Why: LoadPlugin must fail gracefully with a clear error when the file
		//      doesn't exist. This prevents confusing errors later in the chain.
		// =====================================================================
		Context("with a missing WASM file", func() {
			It("should return an error", func() {
				nonExistentPath := "/nonexistent/path/plugin.wasm"

				plugin, err := runtime.LoadPlugin(nonExistentPath)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("plugin file not found"))
				Expect(plugin).To(BeNil())
			})
		})

		// =====================================================================
		// TEST: Invalid WASM file
		// Why: LoadPlugin must detect and reject corrupted or invalid WASM
		//      binaries during validation, not during execution.
		// =====================================================================
		Context("with an invalid WASM file", func() {
			var invalidWasmPath string

			BeforeEach(func() {
				// Create a temporary invalid WASM file
				tmpDir, err := os.MkdirTemp("", "wasm-test-*")
				Expect(err).NotTo(HaveOccurred())
				
				invalidWasmPath = filepath.Join(tmpDir, "invalid.wasm")
				err = os.WriteFile(invalidWasmPath, []byte("not a valid wasm file"), 0644)
				Expect(err).NotTo(HaveOccurred())
			})

			AfterEach(func() {
				// Cleanup temp files
				os.RemoveAll(filepath.Dir(invalidWasmPath))
			})

			It("should return a validation error", func() {
				plugin, err := runtime.LoadPlugin(invalidWasmPath)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to load WASM file"))
				Expect(plugin).To(BeNil())
			})
		})
	})

	// =========================================================================
	// TEST: Close() idempotency
	// Why: Close() must be safe to call multiple times without panicking.
	//      This is critical for defer chains and error recovery paths.
	// =========================================================================
	Describe("Close", func() {
		Context("when called multiple times", func() {
			It("should not panic", func() {
				if _, err := os.Stat(validPluginPath); os.IsNotExist(err) {
					Skip("Test plugin not found")
				}

				plugin, err := runtime.LoadPlugin(validPluginPath)
				Expect(err).NotTo(HaveOccurred())

				// Should not panic on multiple closes
				Expect(func() {
					plugin.Close()
					plugin.Close()
					plugin.Close()
				}).NotTo(Panic())
			})
		})
	})
})
