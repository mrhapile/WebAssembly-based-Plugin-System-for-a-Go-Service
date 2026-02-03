package fluid_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mrhapile/wasm-plugin-system/fluid"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFluid(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Fluid Suite")
}

var _ = Describe("PluginStore", func() {
	var (
		tempDir string
	)

	// Setup: Create temporary directory structure for testing
	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "fluid-test-*")
		Expect(err).NotTo(HaveOccurred())

		// Create test plugin directory structure
		// <tempDir>/hello/hello.wasm
		pluginDir := filepath.Join(tempDir, "hello")
		Expect(os.MkdirAll(pluginDir, 0755)).To(Succeed())

		// Create a dummy .wasm file
		wasmFile := filepath.Join(pluginDir, "hello.wasm")
		Expect(os.WriteFile(wasmFile, []byte("dummy wasm content"), 0644)).To(Succeed())
	})

	// Cleanup: Remove temporary directory
	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	// =========================================================================
	// LocalPluginStore Tests
	// =========================================================================
	Describe("LocalPluginStore", func() {
		var store *fluid.LocalPluginStore

		BeforeEach(func() {
			store = fluid.NewLocalPluginStore(tempDir)
		})

		// =====================================================================
		// TEST: Successful plugin resolution
		// Why: Core functionality - must correctly construct and verify paths.
		// =====================================================================
		Context("when plugin exists", func() {
			It("should return the correct path", func() {
				path, err := store.Resolve("hello")

				Expect(err).NotTo(HaveOccurred())
				Expect(path).To(Equal(filepath.Join(tempDir, "hello", "hello.wasm")))
			})
		})

		// =====================================================================
		// TEST: Plugin not found
		// Why: Must return ErrPluginNotFound for missing plugins, not generic
		//      error or panic.
		// =====================================================================
		Context("when plugin does not exist", func() {
			It("should return ErrPluginNotFound", func() {
				_, err := store.Resolve("nonexistent")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("plugin not found"))
				Expect(err.Error()).To(ContainSubstring("nonexistent"))
			})
		})

		// =====================================================================
		// TEST: Empty plugin name
		// Why: Edge case - empty name should result in "plugin not found".
		// =====================================================================
		Context("when plugin name is empty", func() {
			It("should return ErrPluginNotFound", func() {
				_, err := store.Resolve("")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("plugin not found"))
			})
		})

		// =====================================================================
		// TEST: Invalid base path
		// Why: Store with non-existent base path should fail gracefully.
		// =====================================================================
		Context("with non-existent base path", func() {
			BeforeEach(func() {
				store = fluid.NewLocalPluginStore("/nonexistent/path")
			})

			It("should return error for any plugin", func() {
				_, err := store.Resolve("hello")

				Expect(err).To(HaveOccurred())
			})
		})
	})

	// =========================================================================
	// FluidPluginStore Tests
	// =========================================================================
	Describe("FluidPluginStore", func() {
		var store *fluid.FluidPluginStore

		BeforeEach(func() {
			// In tests, FluidPluginStore works the same as LocalPluginStore
			// because Fluid mounts appear as regular filesystem paths
			store = fluid.NewFluidPluginStore(tempDir)
		})

		// =====================================================================
		// TEST: Successful plugin resolution from "mount"
		// Why: Verify FluidPluginStore correctly resolves paths from the mount.
		// =====================================================================
		Context("when plugin exists on mount", func() {
			It("should return the correct path", func() {
				path, err := store.Resolve("hello")

				Expect(err).NotTo(HaveOccurred())
				Expect(path).To(Equal(filepath.Join(tempDir, "hello", "hello.wasm")))
			})
		})

		// =====================================================================
		// TEST: Plugin not found on mount
		// Why: Must handle missing files on Fluid mount correctly.
		// =====================================================================
		Context("when plugin does not exist on mount", func() {
			It("should return ErrPluginNotFound", func() {
				_, err := store.Resolve("nonexistent")

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("plugin not found"))
			})
		})
	})

	// =========================================================================
	// PluginStore Interface Compliance
	// =========================================================================
	Describe("Interface Compliance", func() {
		It("LocalPluginStore should implement PluginStore", func() {
			var _ fluid.PluginStore = fluid.NewLocalPluginStore("./plugins")
		})

		It("FluidPluginStore should implement PluginStore", func() {
			var _ fluid.PluginStore = fluid.NewFluidPluginStore("/mnt/fluid/plugins")
		})
	})
})
