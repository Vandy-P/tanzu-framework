// Copyright 2021 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"testing"

	"github.com/stretchr/testify/assert"

	configapi "github.com/vmware-tanzu/tanzu-framework/cli/runtime/apis/config/v1alpha1"
)

func Test_createDiscoverySource(t *testing.T) {
	assert := assert.New(t)

	// When discovery source name is empty
	_, err := createDiscoverySource("LOCAL", "", "fake/path")
	assert.NotNil(err)
	assert.Equal(err.Error(), "discovery source name cannot be empty")

	// When discovery source type is empty
	_, err = createDiscoverySource("", "fake-discovery-name", "fake/path")
	assert.NotNil(err)
	assert.Contains(err.Error(), "discovery source type cannot be empty")

	// When discovery source is `local` and data is provided correctly
	pd, err := createDiscoverySource("local", "fake-discovery-name", "fake/path")
	assert.Nil(err)
	assert.NotNil(pd.Local)
	assert.Equal(pd.Local.Name, "fake-discovery-name")
	assert.Equal(pd.Local.Path, "fake/path")

	// When discovery source is `LOCAL`
	pd, err = createDiscoverySource("LOCAL", "fake-discovery-name", "fake/path")
	assert.Nil(err)
	assert.NotNil(pd.Local)
	assert.Equal(pd.Local.Name, "fake-discovery-name")
	assert.Equal(pd.Local.Path, "fake/path")

	// When discovery source is `oci`
	pd, err = createDiscoverySource("oci", "fake-oci-discovery-name", "test.registry.com/test-image:v1.0.0")
	assert.Nil(err)
	assert.NotNil(pd.OCI)
	assert.Equal(pd.OCI.Name, "fake-oci-discovery-name")
	assert.Equal(pd.OCI.Image, "test.registry.com/test-image:v1.0.0")

	// When discovery source is gcp
	_, err = createDiscoverySource("gcp", "fake-discovery-name", "fake/path")
	assert.NotNil(err)
	assert.Contains(err.Error(), "not yet supported")

	// When discovery source is kubernetes
	_, err = createDiscoverySource("kubernetes", "fake-discovery-name", "fake/path")
	assert.NotNil(err)
	assert.Contains(err.Error(), "not yet supported")

	// When discovery source is rest
	pd, err = createDiscoverySource("rest", "fake-discovery-name", "fake/path")
	assert.Nil(err)
	assert.NotNil(pd.REST)
	assert.Equal(pd.REST.Name, "fake-discovery-name")
	assert.Equal(pd.REST.Endpoint, "fake/path")

	// When discovery source is an unknown value
	_, err = createDiscoverySource("unexpectedValue", "fake-discovery-name", "fake/path")
	assert.NotNil(err)
	assert.Contains(err.Error(), "unknown discovery source type 'unexpectedValue'")
}

func Test_addDiscoverySource(t *testing.T) {
	assert := assert.New(t)

	discoverySources := []configapi.PluginDiscovery{
		configapi.PluginDiscovery{
			Local: &configapi.LocalDiscovery{
				Name: "source1",
				Path: "fakepath1",
			},
		},
	}

	// When discovery source already exists
	_, err := addDiscoverySource(discoverySources, "source1", "local", "fakepath2")
	assert.NotNil(err)
	assert.Contains(err.Error(), "discovery name \"source1\" already exists")

	// When new discovery source gets added
	updatedDiscoverySources, err := addDiscoverySource(discoverySources, "source2", "local", "fakepath2")
	assert.Nil(err)
	assert.Equal(2, len(updatedDiscoverySources))
	assert.NotNil(updatedDiscoverySources[1].Local)
	assert.Equal("source2", updatedDiscoverySources[1].Local.Name)
	assert.Equal("fakepath2", updatedDiscoverySources[1].Local.Path)
}

func Test_updateDiscoverySources(t *testing.T) {
	assert := assert.New(t)

	discoverySources := []configapi.PluginDiscovery{
		configapi.PluginDiscovery{
			Local: &configapi.LocalDiscovery{
				Name: "source1",
				Path: "fakepath1",
			},
		},
	}

	// When discovery source already exists
	updatedDiscoverySources, err := updateDiscoverySources(discoverySources, "source1", "local", "fakepath2")
	assert.Nil(err)
	assert.Equal(1, len(updatedDiscoverySources))
	assert.NotNil(updatedDiscoverySources[0].Local)
	assert.Equal("source1", updatedDiscoverySources[0].Local.Name)
	assert.Equal("fakepath2", updatedDiscoverySources[0].Local.Path)

	// When discovery source does not exist
	_, err = updateDiscoverySources(discoverySources, "source2", "local", "fakepath2")
	assert.NotNil(err)
	assert.Contains(err.Error(), "discovery source \"source2\" does not exist")
}

func Test_deleteDiscoverySource(t *testing.T) {
	assert := assert.New(t)

	discoverySources := []configapi.PluginDiscovery{
		configapi.PluginDiscovery{
			Local: &configapi.LocalDiscovery{
				Name: "source1",
				Path: "fakepath1",
			},
		},
		configapi.PluginDiscovery{
			OCI: &configapi.OCIDiscovery{
				Name:  "source2",
				Image: "test.registry.com/test-image:v1.0.0",
			},
		},
	}

	// When discovery source does not exist
	_, err := deleteDiscoverySource(discoverySources, "source-does-not-exists")
	assert.NotNil(err)
	assert.Contains(err.Error(), "discovery source \"source-does-not-exists\" does not exist")

	// When deleting existing discovery source
	updatedDiscoverySources, err := deleteDiscoverySource(discoverySources, "source1")
	assert.Nil(err)
	assert.Equal(1, len(updatedDiscoverySources))
	assert.Nil(updatedDiscoverySources[0].Local)
	assert.NotNil(updatedDiscoverySources[0].OCI)
	assert.Equal("source2", updatedDiscoverySources[0].OCI.Name)
	assert.Equal("test.registry.com/test-image:v1.0.0", updatedDiscoverySources[0].OCI.Image)

	// When deleting last discovery source
	updatedDiscoverySources, err = deleteDiscoverySource(updatedDiscoverySources, "source2")
	assert.Nil(err)
	assert.Equal(0, len(updatedDiscoverySources))
}
