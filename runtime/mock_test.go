package runtime_test

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/agiledragon/gomonkey/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/mrhapile/wasm-plugin-system/runtime"
)

// =========================================================================
// Mock Tests using gomonkey
// Why: Test error handling paths that are difficult to trigger with real
//
//	WASM plugins. gomonkey allows us to mock function behavior.
//
// =========================================================================
var _ = Describe("Mocked Tests", func() {
	var (
		patches *gomonkey.Patches
	)

	AfterEach(func() {
		// Always reset patches to avoid affecting other tests
		if patches != nil {
			patches.Reset()
			patches = nil
		}
	})

	// =========================================================================
	// TEST: os.Stat failure (mocked)
	// Why: Verify LoadPlugin handles file system errors correctly, not just
	//      "file not found" but permission errors, I/O errors, etc.
	// =========================================================================
	Describe("LoadPlugin with mocked os.Stat", func() {
		Context("when os.Stat returns an error", func() {
			It("should return plugin file not found error", func() {
				// Mock os.Stat to always return an error
				patches = gomonkey.ApplyFunc(os.Stat, func(name string) (os.FileInfo, error) {
					return nil, errors.New("mock: permission denied")
				})

				plugin, err := runtime.LoadPlugin("any/path.wasm")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("plugin file not found"))
				Expect(plugin).To(BeNil())
			})
		})
	})

	// =========================================================================
	// TEST: File exists but is corrupted (simulated)
	// Why: Verify that LoadPlugin handles corrupted WASM files gracefully.
	// =========================================================================
	Describe("LoadPlugin with corrupted file", func() {
		var (
			tmpDir      string
			corruptPath string
		)

		BeforeEach(func() {
			var err error
			tmpDir, err = os.MkdirTemp("", "wasm-mock-test-*")
			Expect(err).NotTo(HaveOccurred())

			// Create a file with invalid WASM content
			corruptPath = filepath.Join(tmpDir, "corrupt.wasm")
			err = os.WriteFile(corruptPath, []byte{0x00, 0x61, 0x73, 0x6d}, 0644)
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			os.RemoveAll(tmpDir)
		})

		It("should return an error for corrupted WASM", func() {
			plugin, err := runtime.LoadPlugin(corruptPath)

			Expect(err).To(HaveOccurred())
			Expect(plugin).To(BeNil())
		})
	})

	// =========================================================================
	// TEST: Plugin with missing export (simulated via temp file)
	// Why: Verify that calling Init/Execute/Cleanup on a plugin without the
	//      required exports fails gracefully.
	// Note: This would require a specially compiled WASM without exports.
	//       For now, we document the test case for future implementation.
	// =========================================================================
	Describe("Plugin with missing exports", func() {
		// This test requires a WASM module compiled without init/process/cleanup exports
		// Skip for now - would need a test fixture
		PIt("should return error when init export is missing", func() {
			// Would load a WASM without init() export and verify error
		})

		PIt("should return error when process export is missing", func() {
			// Would load a WASM without process() export and verify error
		})

		PIt("should return error when cleanup export is missing", func() {
			// Would load a WASM without cleanup() export and verify error
		})
	})

	// =========================================================================
	// TEST: ABI error code handling
	// Why: Verify that different ABI error codes are handled and reported
	//      correctly.
	// =========================================================================
	Describe("ABI Error Codes", func() {
		It("should define correct error constants", func() {
			Expect(runtime.ABISuccess).To(Equal(0))
			Expect(runtime.ABIErrorNotInitialized).To(Equal(-1))
			Expect(runtime.ABIErrorAlreadyInitialized).To(Equal(-2))
			Expect(runtime.ABIErrorInvalidInput).To(Equal(-3))
			Expect(runtime.ABIErrorInternal).To(Equal(-4))
		})
	})
})

// =========================================================================
// TEST: Resource cleanup verification
// Why: Ensure that Close() actually releases resources and subsequent calls
//
//	to Init/Execute/Cleanup fail appropriately.
//
// =========================================================================
var _ = Describe("Resource Cleanup", func() {
	var (
		plugin          *runtime.Plugin
		validPluginPath string
	)

	BeforeEach(func() {
		validPluginPath = filepath.Join("..", "plugins", "hello", "hello.wasm")

		if _, err := os.Stat(validPluginPath); os.IsNotExist(err) {
			Skip("Test plugin not found")
		}

		var err error
		plugin, err = runtime.LoadPlugin(validPluginPath)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		if plugin != nil {
			plugin.Close()
		}
	})

	It("should prevent Init after Close", func() {
		plugin.Close()
		err := plugin.Init()

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("plugin is closed"))
	})

	It("should prevent Execute after Close", func() {
		plugin.Close()
		_, err := plugin.Execute(21)

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("plugin is closed"))
	})

	It("should prevent Cleanup after Close", func() {
		plugin.Close()
		err := plugin.Cleanup()

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("plugin is closed"))
	})

	It("should allow multiple Close calls without panic", func() {
		Expect(func() {
			plugin.Close()
			plugin.Close()
			plugin.Close()
		}).NotTo(Panic())
	})
})
