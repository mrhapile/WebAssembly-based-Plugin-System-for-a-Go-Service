// Package fluid provides an abstraction layer for plugin storage.
//
// This package enables switching between local filesystem storage (development)
// and Fluid dataset mounts (production) without changing plugin execution logic.
//
// # Fluid Dataset Integration
//
// Fluid (https://github.com/fluid-cloudnative/fluid) is a Kubernetes-native
// distributed dataset orchestrator. It provides:
//   - Dataset abstraction over various storage backends (S3, HDFS, etc.)
//   - Caching and data locality optimization
//   - Seamless filesystem mount via FUSE
//
// In production, Fluid mounts the dataset as a regular POSIX filesystem path
// (e.g., /mnt/fluid/plugins). This package treats that mount as an ordinary
// directory, requiring NO Kubernetes client or Fluid-specific APIs.
//
// # Architecture Decision
//
// We intentionally avoid:
//   - Direct Kubernetes API calls (the runtime shouldn't know about K8s)
//   - Fluid-specific client libraries (mount is just a filesystem)
//   - Caching logic (Fluid handles caching at the storage layer)
//   - Environment detection (caller decides which store to use)
//
// This keeps the plugin system portable and testable without a cluster.
package fluid

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// ErrPluginNotFound is returned when a plugin cannot be resolved.
var ErrPluginNotFound = errors.New("plugin not found")

// PluginStore resolves plugin names to filesystem paths.
//
// Implementations must:
//   - Return the absolute path to the .wasm file
//   - Return ErrPluginNotFound if the plugin doesn't exist
//   - NOT modify or cache plugin files
type PluginStore interface {
	// Resolve converts a plugin name to its filesystem path.
	//
	// The returned path points to the compiled .wasm file, ready for loading
	// by the runtime. The path format is implementation-specific:
	//   - LocalPluginStore: ./plugins/<name>/<name>.wasm
	//   - FluidPluginStore: /mnt/fluid/plugins/<name>/<name>.wasm
	//
	// Returns ErrPluginNotFound if the plugin does not exist.
	Resolve(pluginName string) (string, error)
}

// LocalPluginStore resolves plugins from the local filesystem.
//
// Use this for development and testing where plugins are compiled
// and stored locally in the project directory.
//
// Directory structure expected:
//
//	<basePath>/
//	├── hello/
//	│   └── hello.wasm
//	├── transform/
//	│   └── transform.wasm
//	└── validate/
//	    └── validate.wasm
type LocalPluginStore struct {
	// basePath is the root directory containing plugin subdirectories.
	// Example: "./plugins" or "/app/plugins"
	basePath string
}

// NewLocalPluginStore creates a LocalPluginStore with the given base path.
//
// The basePath should point to the directory containing plugin subdirectories.
// Each plugin is expected at: <basePath>/<name>/<name>.wasm
//
// Example:
//
//	store := NewLocalPluginStore("./plugins")
//	path, err := store.Resolve("hello") // returns "./plugins/hello/hello.wasm"
func NewLocalPluginStore(basePath string) *LocalPluginStore {
	return &LocalPluginStore{basePath: basePath}
}

// Resolve returns the path to a plugin's .wasm file.
//
// Path format: <basePath>/<pluginName>/<pluginName>.wasm
func (s *LocalPluginStore) Resolve(pluginName string) (string, error) {
	wasmPath := filepath.Join(s.basePath, pluginName, pluginName+".wasm")

	// Check if the file exists
	if _, err := os.Stat(wasmPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrPluginNotFound, pluginName)
		}
		return "", fmt.Errorf("failed to access plugin: %w", err)
	}

	return wasmPath, nil
}

// FluidPluginStore resolves plugins from a Fluid dataset mount.
//
// In production, Fluid mounts a Dataset (backed by S3, HDFS, etc.) as a
// local filesystem path using AlluxioFUSE or JuiceFSFuse. This store
// treats that mount point as a regular directory.
//
// # How Fluid Works
//
// 1. A Dataset CR defines the remote storage (S3 bucket, HDFS path, etc.)
// 2. An AlluxioRuntime or JuiceFSRuntime handles caching and mounting
// 3. A PVC exposes the dataset as a mountable volume
// 4. The pod mounts the PVC at a path (e.g., /mnt/fluid/plugins)
//
// From the application's perspective, it's just a filesystem path.
// This store requires no Kubernetes client or Fluid SDK.
//
// # Example Kubernetes Setup
//
//	apiVersion: data.fluid.io/v1alpha1
//	kind: Dataset
//	metadata:
//	  name: wasm-plugins
//	spec:
//	  mounts:
//	    - mountPoint: s3://my-bucket/plugins
//	      name: plugins
//	---
//	apiVersion: data.fluid.io/v1alpha1
//	kind: AlluxioRuntime
//	metadata:
//	  name: wasm-plugins
//	spec:
//	  replicas: 2
//	  tieredstore:
//	    levels:
//	      - mediumtype: MEM
//	        path: /dev/shm
//	        quota: 2Gi
//
// The application pod then mounts the PVC:
//
//	volumes:
//	  - name: plugins
//	    persistentVolumeClaim:
//	      claimName: wasm-plugins
//	volumeMounts:
//	  - name: plugins
//	    mountPath: /mnt/fluid/plugins
//
// Directory structure on the mount:
//
//	/mnt/fluid/plugins/
//	├── hello/
//	│   └── hello.wasm
//	├── transform/
//	│   └── transform.wasm
//	└── validate/
//	    └── validate.wasm
type FluidPluginStore struct {
	// mountPath is the Fluid dataset mount point.
	// Example: "/mnt/fluid/plugins"
	mountPath string
}

// NewFluidPluginStore creates a FluidPluginStore with the given mount path.
//
// The mountPath should be where the Fluid dataset is mounted in the pod.
// Common patterns:
//   - /mnt/fluid/<dataset-name>
//   - /data/plugins
//   - /var/lib/plugins
//
// Example:
//
//	store := NewFluidPluginStore("/mnt/fluid/plugins")
//	path, err := store.Resolve("hello") // returns "/mnt/fluid/plugins/hello/hello.wasm"
func NewFluidPluginStore(mountPath string) *FluidPluginStore {
	return &FluidPluginStore{mountPath: mountPath}
}

// Resolve returns the path to a plugin's .wasm file from the Fluid mount.
//
// Path format: <mountPath>/<pluginName>/<pluginName>.wasm
//
// The underlying storage (S3, HDFS, etc.) is abstracted by Fluid.
// This method simply constructs the path and verifies the file exists.
// Caching and data locality are handled transparently by the Fluid runtime.
func (s *FluidPluginStore) Resolve(pluginName string) (string, error) {
	wasmPath := filepath.Join(s.mountPath, pluginName, pluginName+".wasm")

	// Check if the file exists on the mount
	// Fluid's FUSE layer handles fetching from remote storage if needed
	if _, err := os.Stat(wasmPath); err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrPluginNotFound, pluginName)
		}
		// Could be permission issues, mount problems, or network errors
		// (abstracted as filesystem errors by FUSE)
		return "", fmt.Errorf("failed to access plugin on Fluid mount: %w", err)
	}

	return wasmPath, nil
}
