// Copyright 2021-2022 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/aunum/log"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	capi "sigs.k8s.io/cluster-api/api/v1beta1"

	"github.com/vmware-tanzu/tanzu-framework/pinniped-components/common/pkg/pinnipedinfo"
	"github.com/vmware-tanzu/tanzu-framework/tkg/clusterclient"
	"github.com/vmware-tanzu/tanzu-framework/tkg/region"
	"github.com/vmware-tanzu/tanzu-framework/tkg/utils"
)

// GetClusterPinnipedInfoOptions contains options supported by GetClusterPinnipedInfo
type GetClusterPinnipedInfoOptions struct {
	ClusterName         string
	Namespace           string
	IsManagementCluster bool
}

// ClusterPinnipedInfo defines the fields of cluster pinniped info
type ClusterPinnipedInfo struct {
	ClusterName     string
	ClusterInfo     *clientcmdapi.Cluster
	ClusterAudience *string
	PinnipedInfo    *pinnipedinfo.PinnipedInfo
}

// GetClusterPinnipedInfo gets pinniped information from cluster
func (c *TkgClient) GetClusterPinnipedInfo(options GetClusterPinnipedInfoOptions) (*ClusterPinnipedInfo, error) {
	clusterclientOptions := clusterclient.Options{
		GetClientInterval: 1 * time.Second,
		GetClientTimeout:  3 * time.Second,
	}

	curRegion, err := c.GetCurrentRegionContext()
	if err != nil {
		return nil, errors.Wrap(err, "unable to get current management cluster configuration")
	}

	regionalClusterClient, err := clusterclient.NewClient(curRegion.SourceFilePath, curRegion.ContextName, clusterclientOptions)
	if err != nil {
		return nil, errors.Wrap(err, "unable to get cluster client while getting cluster pinniped info of tkg clusters")
	}

	isPacific, err := regionalClusterClient.IsPacificRegionalCluster()
	if err != nil {
		return nil, errors.Wrap(err, "error determining 'Tanzu Kubernetes Cluster service for vSphere' management cluster")
	}
	if isPacific && options.IsManagementCluster {
		return nil, errors.New("getting pinniped information not supported for 'Tanzu Kubernetes Cluster service for vSphere' management cluster")
	}

	if options.IsManagementCluster {
		return c.GetMCClusterPinnipedInfo(regionalClusterClient, curRegion, options)
	}

	// Check if the cluster is a ClusterClass cluster, which would include TKG 1.7+ "classy" clusters.
	isClusterClassBased, err := regionalClusterClient.IsClusterClassBased(options.ClusterName, options.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to determine if workload cluster is ClusterClass based")
	}

	return c.GetWCClusterPinnipedInfo(regionalClusterClient, curRegion, options, isPacific, isClusterClassBased)
}

// GetWCClusterPinnipedInfo gets pinniped information for workload cluster
func (c *TkgClient) GetWCClusterPinnipedInfo(
	regionalClusterClient clusterclient.Client,
	_ region.RegionContext,
	options GetClusterPinnipedInfoOptions,
	isPacific bool,
	isClusterClassBased bool) (*ClusterPinnipedInfo, error) {

	wcClusterInfo, err := getClusterInfo(regionalClusterClient, options.ClusterName, options.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get workload cluster information")
	}

	// The existing PinnipedInfoConfigMap struct is a marshaled form of a
	// ConfigMap. Marshal and unmarshal the raw CM into that struct so we can
	// use it.
	// TODO(mayankbh) This is a shorter term approach. The _right_ thing might
	// be to significantly refactor the PinnipedConfigMapInfo struct so it can
	// be constructed from an existing ConfigMap.
	configMap := corev1.ConfigMap{}

	if err := regionalClusterClient.GetResource(&configMap, utils.PinnipedInfoConfigMapName, utils.KubePublicNamespace, nil, nil); err != nil {
		return nil, errors.New("failed to get pinniped-info from management cluster")
	}

	log.Debugf("Management cluster pinniped ConfigMap: %+v", configMap)

	marshalledCM, err := json.Marshal(configMap.Data)
	if err != nil {
		return nil, errors.New("failed to marshal pinniped-info from management cluster")
	}

	managementClusterPinnipedInfo := &pinnipedinfo.PinnipedInfo{}

	// Really, this should never fail unless we're doing something silly like
	// marshaling a channel/function. Which we aren't.
	if err := json.Unmarshal(marshalledCM, &managementClusterPinnipedInfo); err != nil {
		return nil, errors.New("failed to unmarshal pinniped-info from management cluster")
	}

	log.Debugf("Management cluster pinniped info: %+v", managementClusterPinnipedInfo)

	workloadClusterPinnipedInfo, err := utils.GetPinnipedInfoFromCluster(wcClusterInfo, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pinniped-info from workload cluster")
	}

	pinnipedInfo := managementClusterPinnipedInfo
	if workloadClusterPinnipedInfo != nil {
		// Get ConciergeIsClusterScoped from workload cluster in case it is different from the management cluster
		pinnipedInfo.ConciergeIsClusterScoped = workloadClusterPinnipedInfo.ConciergeIsClusterScoped
	} else {
		// If workloadClusterPinnipedInfo is nil, assume it is an older TKG cluster and set ConciergeIsClusterScoped to defaults
		pinnipedInfo.ConciergeIsClusterScoped = false
	}

	// For clusters that use a TKr API version newer than v1alpha1, we use the cluster name + UID as the audience.
	// Do this on pacific clusters and TKG "classy" clusters, but not on TKG legacy (non-classy) clusters.
	var audience *string
	if isPacific || isClusterClassBased {
		var cluster capi.Cluster
		if err := regionalClusterClient.GetResource(
			&cluster,
			options.ClusterName,
			options.Namespace,
			nil,
			nil,
		); err != nil {
			return nil, errors.Wrap(err, "get cluster")
		}
		if _, ok := cluster.Labels[LegacyClusterTKRLabel]; !ok {
			audience = stringPtr(fmt.Sprintf("%s-%s", cluster.Name, cluster.UID))
		}
	}

	if isPacific {
		// Pacific uses a different Concierge endpoint. Ignore it when fetching
		// a kubeconfig for a workload cluster since we use the workload
		// cluster APIserver as the concierge endpoint.
		pinnipedInfo.ConciergeEndpoint = ""
	}

	return &ClusterPinnipedInfo{
		ClusterName:     options.ClusterName,
		ClusterAudience: audience,
		ClusterInfo:     wcClusterInfo,
		PinnipedInfo:    pinnipedInfo,
	}, nil
}

// GetMCClusterPinnipedInfo get pinniped information for management cluster
func (c *TkgClient) GetMCClusterPinnipedInfo(regionalClusterClient clusterclient.Client,
	curRegion region.RegionContext, options GetClusterPinnipedInfoOptions) (*ClusterPinnipedInfo, error) {
	// it is expected that user would call get cluster pinnedInfo of the same management cluster
	clusterInfo, err := getClusterInfo(regionalClusterClient, options.ClusterName, options.Namespace)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get cluster information")
	}
	pinnipedInfo, err := utils.GetPinnipedInfoFromCluster(clusterInfo, nil)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pinniped-info from cluster")
	}

	if pinnipedInfo == nil {
		return nil, errors.New("failed to get pinniped-info from cluster")
	}

	return &ClusterPinnipedInfo{
		ClusterName:  options.ClusterName,
		ClusterInfo:  clusterInfo,
		PinnipedInfo: pinnipedInfo,
	}, nil
}

func getClusterInfo(
	regionalClusterClient clusterclient.Client,
	clusterName, clusterNamespace string,
) (*clientcmdapi.Cluster, error) {

	kubeconfigData, err := regionalClusterClient.GetKubeConfigForCluster(clusterName, clusterNamespace, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get kubeconfig for cluster %s/%s: %w", clusterNamespace, clusterName, err)
	}

	config, err := clientcmd.Load(kubeconfigData)
	if err != nil {
		return nil, errors.Wrap(err, "failed to load the kubeconfig")
	}

	if len(config.Clusters) == 0 {
		return nil, errors.New("failed to get cluster information")
	}

	// since it is a map with one cluster object, get the first entry
	var cluster *clientcmdapi.Cluster
	for _, cluster = range config.Clusters {
		break
	}

	return cluster, nil
}

func stringPtr(s string) *string { return &s }
