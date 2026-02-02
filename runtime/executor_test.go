package runtime_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"

	"github.com/mrhapile/wasm-plugin-system/runtime"
)

var _ = Describe("Executor", func() {
	var (
		plugin          *runtime.Plugin
		validPluginPath string
	)

	// Setup: Load plugin before each test
	BeforeEach(func() {
		validPluginPath = filepath.Join("..", "plugins", "hello", "hello.wasm")

		// Skip all tests if plugin doesn't exist
		if _, err := os.Stat(validPluginPath); os.IsNotExist(err) {
			Skip("Test plugin not found: " + validPluginPath)
		}

		var err error
		plugin, err = runtime.LoadPlugin(validPluginPath)
		Expect(err).NotTo(HaveOccurred())
	})

	// Cleanup: Always release resources
	AfterEach(func() {
		if plugin != nil {
			plugin.Close()
			plugin = nil
		}
	})

	// =========================================================================
	// TEST: Init() success
	// Why: Init() is the first step in the ABI lifecycle. It must work
	//      correctly before any Execute() calls can succeed.
	// =========================================================================
	Describe("Init", func() {
		Context("with a valid plugin", func() {
			It("should initialize successfully", func() {
				err := plugin.Init()

				Expect(err).NotTo(HaveOccurred())
			})
		})

		// =====================================================================
		// TEST: Init() on closed plugin
		// Why: Calling Init() after Close() must return a clear error, not
		//      cause a nil pointer dereference or undefined behavior.
		// =====================================================================
		Context("on a closed plugin", func() {
			It("should return an error", func() {
				plugin.Close()

				err := plugin.Init()

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("plugin is closed"))
			})
		})
	})

	// =========================================================================
	// TEST: Execute() success
	// Why: This is the core functionality - executing plugin logic. Must verify
	//      correct input/output handling.
	// =========================================================================
	Describe("Execute", func() {
		Context("after successful Init", func() {
			BeforeEach(func() {
				err := plugin.Init()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should execute and return correct result", func() {
				// The hello plugin computes: (input * 2) + 1
				result, err := plugin.Execute(21)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(43)) // 21 * 2 + 1 = 43
			})

			It("should work with multiple calls", func() {
				// Execute multiple times to verify state is maintained
				result1, err := plugin.Execute(10)
				Expect(err).NotTo(HaveOccurred())
				Expect(result1).To(Equal(21)) // 10 * 2 + 1 = 21

				result2, err := plugin.Execute(50)
				Expect(err).NotTo(HaveOccurred())
				Expect(result2).To(Equal(101)) // 50 * 2 + 1 = 101
			})

			It("should handle zero input", func() {
				result, err := plugin.Execute(0)

				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(1)) // 0 * 2 + 1 = 1
			})
		})

		// =====================================================================
		// TEST: Execute() without Init
		// Why: The ABI requires init() before process(). Execute() must fail
		//      with a meaningful error if init() wasn't called.
		// =====================================================================
		Context("without calling Init first", func() {
			It("should return an error", func() {
				// Skip Init() and try to Execute directly
				result, err := plugin.Execute(21)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error"))
				Expect(result).To(Equal(0))
			})
		})

		// =====================================================================
		// TEST: Execute() on closed plugin
		// Why: Safety check - must not crash when called on released resources.
		// =====================================================================
		Context("on a closed plugin", func() {
			It("should return an error", func() {
				plugin.Close()

				result, err := plugin.Execute(21)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("plugin is closed"))
				Expect(result).To(Equal(0))
			})
		})

		// =====================================================================
		// TEST: Execute() with negative input (invalid per ABI)
		// Why: The hello plugin treats negative input as invalid and returns
		//      an error code. Verify error handling works correctly.
		// =====================================================================
		Context("with negative input", func() {
			BeforeEach(func() {
				err := plugin.Init()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return an error for negative input", func() {
				result, err := plugin.Execute(-5)

				// The plugin returns ABI_ERROR_INVALID_INPUT for negative input
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("error code"))
				Expect(result).To(Equal(0))
			})
		})
	})

	// =========================================================================
	// TEST: Cleanup() success
	// Why: Cleanup() must work after Init() to properly release plugin state.
	// =========================================================================
	Describe("Cleanup", func() {
		Context("after successful Init", func() {
			BeforeEach(func() {
				err := plugin.Init()
				Expect(err).NotTo(HaveOccurred())
			})

			It("should cleanup successfully", func() {
				err := plugin.Cleanup()

				Expect(err).NotTo(HaveOccurred())
			})
		})

		// =====================================================================
		// TEST: Cleanup() without Init
		// Why: Cleanup() on uninitialized plugin should fail gracefully, not
		//      crash or corrupt state.
		// =====================================================================
		Context("without calling Init first", func() {
			It("should return an error", func() {
				err := plugin.Cleanup()

				// Plugin was never initialized
				Expect(err).To(HaveOccurred())
			})
		})

		// =====================================================================
		// TEST: Cleanup() on closed plugin
		// Why: Must not crash when called on released resources.
		// =====================================================================
		Context("on a closed plugin", func() {
			It("should return an error", func() {
				plugin.Close()

				err := plugin.Cleanup()

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("plugin is closed"))
			})
		})
	})

	// =========================================================================
	// TEST: Full lifecycle
	// Why: Integration test for complete ABI lifecycle: Init -> Execute -> Cleanup
	// =========================================================================
	Describe("Full Lifecycle", func() {
		It("should complete Init -> Execute -> Cleanup successfully", func(ctx SpecContext) {
			// Use testify for cleaner assertions
			t := GinkgoT()

			// Step 1: Init
			err := plugin.Init()
			assert.NoError(t, err, "Init should succeed")

			// Step 2: Execute multiple times
			result1, err := plugin.Execute(21)
			assert.NoError(t, err, "Execute should succeed")
			assert.Equal(t, 43, result1, "Result should be 43")

			result2, err := plugin.Execute(100)
			assert.NoError(t, err, "Second execute should succeed")
			assert.Equal(t, 201, result2, "Result should be 201")

			// Step 3: Cleanup
			err = plugin.Cleanup()
			assert.NoError(t, err, "Cleanup should succeed")
		})
	})
})
