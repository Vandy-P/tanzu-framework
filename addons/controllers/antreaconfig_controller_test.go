// Copyright 2022 VMware, Inc. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	capvvmwarev1beta1 "sigs.k8s.io/cluster-api-provider-vsphere/apis/vmware/v1beta1"
	clusterapiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	antreatype "github.com/vmware-tanzu/tanzu-framework/addons/controllers/antrea"
	cutil "github.com/vmware-tanzu/tanzu-framework/addons/controllers/utils"
	"github.com/vmware-tanzu/tanzu-framework/addons/pkg/constants"
	"github.com/vmware-tanzu/tanzu-framework/addons/pkg/util"
	"github.com/vmware-tanzu/tanzu-framework/addons/test/testutil"
	cniv1alpha2 "github.com/vmware-tanzu/tanzu-framework/apis/addonconfigs/cni/v1alpha2"
	runtanzuv1alpha3 "github.com/vmware-tanzu/tanzu-framework/apis/run/v1alpha3"
)

var _ = Describe("AntreaConfig Reconciler and Webhooks", func() {
	var (
		clusterName             string
		clusterNamespace        string
		configCRName            string
		configName              string
		clusterResourceFilePath string
		err                     error
		f                       *os.File
		tkrString               string
		newDefaultMTU           string
		clusterBootstrap        *runtanzuv1alpha3.ClusterBootstrap
		clusterBoostrapFile     string
		clusterInfraName        string
	)

	const (
		waitTimeout                            = waitTimeout //use this to change test speed when debugging
		antreaManifestsTestFile1               = "testdata/antrea-test-1.yaml"
		antreamanifestsTestFile2               = "testdata/antrea-test-2.yaml"
		antreaTemplateConfigManifestsTestFile1 = "testdata/antrea-test-template-config-1.yaml"
		antreaTestCluster1                     = "test-cluster-4"
		antreaTestCluster2                     = "test-cluster-5"
		vsphereCluster1                        = "test-cluster-5-6gvvc"
	)

	JustBeforeEach(func() {
		// Create the admission webhooks
		f, err = os.Open(cniWebhookManifestFile)
		Expect(err).ToNot(HaveOccurred())
		err = testutil.CreateResources(f, cfg, dynamicClient)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		// set up the certificates and webhook before creating any objects
		By("Creating and installing new certificates for Antrea Admission Webhooks")
		err = testutil.SetupWebhookCertificates(ctx, k8sClient, k8sConfig, &webhookCertDetails)
		Expect(err).ToNot(HaveOccurred())

		// create cluster resources
		By("Creating a cluster and a AntreaConfig")
		f, err = os.Open(clusterResourceFilePath)
		Expect(err).ToNot(HaveOccurred())
		err = testutil.CreateResources(f, cfg, dynamicClient)
		Expect(err).ToNot(HaveOccurred())
		f.Close()
	})

	AfterEach(func() {
		By("Deleting cluster and AntreaConfig")
		f, err = os.Open(clusterResourceFilePath)
		Expect(err).ToNot(HaveOccurred())
		err = testutil.DeleteResources(f, cfg, dynamicClient, true)
		Expect(err).ToNot(HaveOccurred())
		f.Close()

		By("Deleting the Admission Webhook configuration for Antrea")
		f, err = os.Open(cniWebhookManifestFile)
		Expect(err).ToNot(HaveOccurred())
		err = testutil.DeleteResources(f, cfg, dynamicClient, true)
		Expect(err).ToNot(HaveOccurred())
		f.Close()
	})

	Context("Reconcile AntreaConfig for management cluster", func() {

		BeforeEach(func() {
			clusterName = antreaTestCluster1
			clusterNamespace = defaultString
			configCRName = util.GeneratePackageSecretName(clusterName, constants.AntreaDefaultRefName)
			clusterResourceFilePath = antreaManifestsTestFile1
		})

		It("Should reconcile AntreaConfig and create data value secret on management cluster", func() {

			clusterKey := client.ObjectKey{
				Namespace: clusterNamespace,
				Name:      clusterName,
			}
			configKey := client.ObjectKey{
				Namespace: clusterNamespace,
				Name:      configCRName,
			}

			cluster := &clusterapiv1beta1.Cluster{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, clusterKey, cluster)
				return err == nil
			}, waitTimeout, pollingInterval).Should(BeTrue())

			config := &cniv1alpha2.AntreaConfig{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, configKey, config)
				if err != nil {
					return false
				}

				// Check owner reference
				if len(config.OwnerReferences) == 0 {
					return false
				}

				Expect(len(config.OwnerReferences)).Should(Equal(1))
				Expect(config.OwnerReferences[0].Name).Should(Equal(clusterName))

				Expect(config.Spec.Antrea.AntreaConfigDataValue.TrafficEncapMode).Should(Equal("encap"))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.AntreaTraceflow).Should(Equal(false))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.AntreaPolicy).Should(Equal(true))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.FlowExporter).Should(Equal(false))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.AntreaIPAM).Should(Equal(false))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.ServiceExternalIP).Should(Equal(false))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.Multicast).Should(Equal(false))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.MultiCluster).Should(Equal(false))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.SecondaryNetwork).Should(Equal(false))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.TrafficControl).Should(Equal(false))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.NodePortLocal).Should(Equal(true))

				return true
			}, waitTimeout, pollingInterval).Should(BeTrue())

			Eventually(func() bool {
				cluster := &clusterapiv1beta1.Cluster{}
				err := k8sClient.Get(ctx, clusterKey, cluster)
				if err != nil {
					return false
				}

				serviceCIDR, serviceCIDRv6, err := util.GetServiceCIDRs(cluster)
				if err != nil {
					return false
				}

				infraProvider, err := util.GetInfraProvider(cluster)
				if err != nil {
					return false
				}

				// Check infraProvider values
				Expect(infraProvider).Should(Equal("docker"))

				// Check ServiceCIDR and ServiceCIDRv6 values
				Expect(serviceCIDR).Should(Equal("192.168.0.0/16"))
				Expect(serviceCIDRv6).Should(Equal("fd00:100:96::/48"))
				return true
			}, waitTimeout, pollingInterval).Should(BeTrue())

			Eventually(func() bool {
				secretKey := client.ObjectKey{
					Namespace: clusterNamespace,
					Name:      util.GenerateDataValueSecretName(clusterName, constants.AntreaAddonName),
				}
				secret := &v1.Secret{}
				err := k8sClient.Get(ctx, secretKey, secret)
				if err != nil {
					return false
				}
				Expect(secret.Type).Should(Equal(v1.SecretTypeOpaque))

				// check data value secret contents
				content := secret.Data["values.yaml"]
				spec := antreatype.AntreaConfigSpec{}
				err = yaml.Unmarshal(content, &spec)
				if err != nil {
					return false
				}
				Expect(spec.Antrea.AntreaConfigDataValue.ServiceCIDR).Should(Equal("192.168.0.0/16"))
				Expect(spec.Antrea.AntreaConfigDataValue.ServiceCIDRv6).Should(Equal("fd00:100:96::/48"))
				Expect(spec.InfraProvider).Should(Equal("docker"))
				Expect(spec.Antrea.AntreaConfigDataValue.TrafficEncapMode).Should(Equal("encap"))
				Expect(spec.Antrea.AntreaConfigDataValue.TLSCipherSuites).Should(Equal("TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384"))
				Expect(spec.Antrea.AntreaConfigDataValue.FeatureGates.AntreaProxy).Should(Equal(true))
				Expect(spec.Antrea.AntreaConfigDataValue.FeatureGates.AntreaPolicy).Should(Equal(true))

				return true
			}, waitTimeout, pollingInterval).Should(BeTrue())

			Eventually(func() bool {
				// Check status.secretRef after reconciliation
				config := &cniv1alpha2.AntreaConfig{}
				err := k8sClient.Get(ctx, configKey, config)
				if err != nil {
					return false
				}
				return config.Status.SecretRef == util.GenerateDataValueSecretName(clusterName, constants.AntreaAddonName)
			}, waitTimeout, pollingInterval).Should(BeTrue())

		})

	})

	Context("Reconcile AntreaConfig with antreaNsx enabled for management cluster", func() {

		BeforeEach(func() {
			clusterName = antreaTestCluster2
			clusterNamespace = defaultString
			configCRName = util.GeneratePackageSecretName(clusterName, constants.AntreaDefaultRefName)
			clusterResourceFilePath = antreamanifestsTestFile2
		})

		It("Should reconcile AntreaConfig and create data value secret on management cluster", func() {

			clusterKey := client.ObjectKey{
				Namespace: clusterNamespace,
				Name:      clusterName,
			}
			configKey := client.ObjectKey{
				Namespace: clusterNamespace,
				Name:      configCRName,
			}

			cluster := &clusterapiv1beta1.Cluster{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, clusterKey, cluster)
				return err == nil
			}, waitTimeout, pollingInterval).Should(BeTrue())

			config := &cniv1alpha2.AntreaConfig{}
			Eventually(func() bool {
				err := k8sClient.Get(ctx, configKey, config)
				if err != nil {
					return false
				}

				// Check owner reference
				if len(config.OwnerReferences) == 0 {
					return false
				}

				Expect(len(config.OwnerReferences)).Should(Equal(1))
				Expect(config.OwnerReferences[0].Name).Should(Equal(clusterName))

				Expect(config.Spec.Antrea.AntreaConfigDataValue.TrafficEncapMode).Should(Equal("encap"))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.AntreaTraceflow).Should(Equal(false))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.AntreaPolicy).Should(Equal(true))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.FlowExporter).Should(Equal(false))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.AntreaIPAM).Should(Equal(false))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.ServiceExternalIP).Should(Equal(false))
				Expect(config.Spec.Antrea.AntreaConfigDataValue.FeatureGates.Multicast).Should(Equal(false))
				Expect(config.Spec.AntreaNsx.Enable).Should(Equal(true))
				return true
			}, waitTimeout, pollingInterval).Should(BeTrue())

			Eventually(func() bool {
				cluster := &clusterapiv1beta1.Cluster{}
				err := k8sClient.Get(ctx, clusterKey, cluster)
				if err != nil {
					return false
				}

				serviceCIDR, serviceCIDRv6, err := util.GetServiceCIDRs(cluster)
				if err != nil {
					return false
				}

				infraProvider, err := util.GetInfraProvider(cluster)
				if err != nil {
					return false
				}

				// Check infraProvider values
				Expect(infraProvider).Should(Equal("vsphere"))

				// Check ServiceCIDR and ServiceCIDRv6 values
				Expect(serviceCIDR).Should(Equal("192.168.0.0/16"))
				Expect(serviceCIDRv6).Should(Equal("fd00:100:96::/48"))
				return true
			}, waitTimeout, pollingInterval).Should(BeTrue())

			Eventually(func() bool {
				secretKey := client.ObjectKey{
					Namespace: clusterNamespace,
					Name:      util.GenerateDataValueSecretName(clusterName, constants.AntreaAddonName),
				}
				secret := &v1.Secret{}
				err := k8sClient.Get(ctx, secretKey, secret)
				if err != nil {
					return false
				}
				Expect(secret.Type).Should(Equal(v1.SecretTypeOpaque))

				// check data value secret contents
				content := secret.Data["values.yaml"]
				spec := antreatype.AntreaConfigSpec{}
				err = yaml.Unmarshal(content, &spec)
				if err != nil {
					return false
				}
				Expect(spec.Antrea.AntreaConfigDataValue.ServiceCIDR).Should(Equal("192.168.0.0/16"))
				Expect(spec.Antrea.AntreaConfigDataValue.ServiceCIDRv6).Should(Equal("fd00:100:96::/48"))
				Expect(spec.InfraProvider).Should(Equal("vsphere"))
				Expect(spec.Antrea.AntreaConfigDataValue.TrafficEncapMode).Should(Equal("encap"))
				Expect(spec.Antrea.AntreaConfigDataValue.TLSCipherSuites).Should(Equal("TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,TLS_RSA_WITH_AES_256_GCM_SHA384"))
				Expect(spec.Antrea.AntreaConfigDataValue.FeatureGates.AntreaProxy).Should(Equal(true))
				Expect(spec.Antrea.AntreaConfigDataValue.FeatureGates.AntreaPolicy).Should(Equal(true))

				return true
			}, waitTimeout, pollingInterval).Should(BeTrue())

			Eventually(func() bool {
				// Check status.secretRef after reconciliation
				config := &cniv1alpha2.AntreaConfig{}
				err := k8sClient.Get(ctx, configKey, config)
				if err != nil {
					return false
				}
				return config.Status.SecretRef == util.GenerateDataValueSecretName(clusterName, constants.AntreaAddonName)
			}, waitTimeout, pollingInterval).Should(BeTrue())

			// check if ProviderServiceAccount is created
			By("check ProviderServiceAccount is created")
			serviceAccount := &capvvmwarev1beta1.ProviderServiceAccount{}
			Eventually(func() bool {
				serviceAccountKey := client.ObjectKey{
					Namespace: defaultString,
					Name:      fmt.Sprintf("%s-antrea", vsphereCluster1),
				}
				if err := k8sClient.Get(ctx, serviceAccountKey, serviceAccount); err != nil {
					return false
				}
				Expect(serviceAccount.Spec.Ref.Name).To(Equal(vsphereCluster1))
				Expect(serviceAccount.Spec.Ref.Namespace).To(Equal(serviceAccountKey.Namespace))
				Expect(serviceAccount.Spec.Rules).To(HaveLen(2))
				Expect(serviceAccount.Spec.TargetNamespace).To(Equal("vmware-system-antrea"))
				Expect(serviceAccount.Spec.TargetSecretName).To(Equal("supervisor-cred"))
				return true
			})

			// TODO: we shall check if nsxServiceAccount is created, but since it not registered, we do not check now
		})

	})

	Context("Reconcile AntreaConfig used as template", func() {

		BeforeEach(func() {
			clusterName = antreaTestCluster1
			configCRName = util.GeneratePackageSecretName(clusterName, constants.AntreaDefaultRefName)
			clusterResourceFilePath = antreaTemplateConfigManifestsTestFile1
		})

		It("Should skip the reconciliation", func() {

			key := client.ObjectKey{
				Namespace: addonNamespace,
				Name:      configCRName,
			}
			config := &cniv1alpha2.AntreaConfig{}
			Expect(k8sClient.Get(ctx, key, config)).To(Succeed())

			By("OwnerReferences is not set")
			Expect(len(config.OwnerReferences)).Should(Equal(0))
		})
	})

	Context("Mutating webhooks for AntreaConfig", func() {

		BeforeEach(func() {
			clusterName = antreaTestCluster1
			clusterNamespace = defaultString
			configCRName = util.GeneratePackageSecretName(clusterName, constants.AntreaDefaultRefName)
			clusterResourceFilePath = antreaManifestsTestFile1
		})

		It("Should fail mutating webhooks for immutable field for AntreaConfig", func() {

			key := client.ObjectKey{
				Namespace: clusterNamespace,
				Name:      configCRName,
			}
			config := &cniv1alpha2.AntreaConfig{}
			Expect(k8sClient.Get(ctx, key, config)).To(Succeed())

			By("Trying to update the immutable TrafficEncapMode field in Antrea Spec")
			config.Spec.Antrea.AntreaConfigDataValue.TrafficEncapMode = "noEncap"
			Expect(k8sClient.Update(ctx, config)).ToNot(Succeed())
		})
	})

	When("antrea config is precreated with name {CLUSTER_NAME}-{package-short-name}-package", func() {

		BeforeEach(func() {
			clusterName = "antrea-custom-cb-cluster"
			clusterNamespace = "antrea-custom-cb-ns"
			configName = util.GeneratePackageSecretName(clusterName, constants.AntreaDefaultRefName)
			tkrString = "v1.23.1"
			clusterResourceFilePath = "testdata/antrea-custom-tkg-system.yaml"
			clusterInfraName = "custom-cb-docker-cluster"
			newDefaultMTU = "8901"
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
			}

		})

		It("should add cluster as owner reference to antrea config", func() {
			// define custom resources in cluster namespace
			f, err := os.Open("testdata/antrea-custom-cb-ns.yaml")
			Expect(err).ToNot(HaveOccurred())
			err = testutil.CreateResources(f, cfg, dynamicClient)
			Expect(err).ToNot(HaveOccurred())
			f.Close()

			By("Create antrea config in the cluster's namespace with expected name pattern", func() {
				datavalues := &cniv1alpha2.AntreaConfigDataValue{DefaultMTU: newDefaultMTU}
				antreaConfig := generateAntreaConfig(configName, clusterNamespace, datavalues)
				err := k8sClient.Create(ctx, antreaConfig)
				Expect(err).ToNot(HaveOccurred())
				err = k8sClient.Get(ctx, client.ObjectKeyFromObject(antreaConfig), antreaConfig)
				Expect(err).ToNot(HaveOccurred())
			})

			By("Create the cluster", func() {
				cluster := generateDockerCluster(clusterName, clusterNamespace, tkrString, clusterInfraName)
				err := k8sClient.Create(ctx, cluster)
				Expect(err).ToNot(HaveOccurred())
				err = k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
				Expect(err).ToNot(HaveOccurred())

			})

			By("Verify after reconciliation clusterBootstrap ProviderRef.Name points to pre-created antrea config", func() {
				clusterBootstrap = &runtanzuv1alpha3.ClusterBootstrap{}
				Eventually(func() bool {
					err := k8sClient.Get(ctx, client.ObjectKey{Namespace: clusterNamespace, Name: clusterName}, clusterBootstrap)
					return err == nil
				}, waitTimeout, pollingInterval).Should(BeTrue())

				Eventually(func() bool {
					return clusterBootstrap.Spec.CNI.ValuesFrom.ProviderRef.Name == configName

				}, waitTimeout, pollingInterval).Should(BeTrue())
			})

			By("Verify contents of resulting antrea config", func() {
				antreaConfig := &cniv1alpha2.AntreaConfig{}
				key := client.ObjectKey{Name: clusterBootstrap.Spec.CNI.ValuesFrom.ProviderRef.Name, Namespace: clusterNamespace}
				err := k8sClient.Get(ctx, key, antreaConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(cutil.VerifyOwnerRef(antreaConfig, clusterName, constants.ClusterKind)).To(BeTrue())
				Expect(antreaConfig.Spec.Antrea.AntreaConfigDataValue.DefaultMTU).Should(Equal(newDefaultMTU))
			})

		})

	})

	When("custom clusterbootstrap points to precreated antrea config", func() {
		BeforeEach(func() {
			clusterName = "custom-cb-2-cluster"
			clusterNamespace = "custom-cb-2-namespace"
			configName = "antrea-config-custom-2"
			clusterBoostrapFile = "testdata/antrea-custom-cb.yaml"
			clusterInfraName = "custom-cb-docker-cluster-2"
			tkrString = "v1.23.2"
			clusterResourceFilePath = "testdata/antrea-custom-cb-2-resources.yaml"
			newDefaultMTU = "8902"
			ns := &v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterNamespace,
				},
			}
			err := k8sClient.Create(ctx, ns)
			if err != nil {
				Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
			}

		})
		It("should add cluster as owner reference to precreated antrea config", func() {
			By("check resources like the kapp-controller config have been created", func() {
				kappConfig := &runtanzuv1alpha3.KappControllerConfig{}
				kappConfigKey := client.ObjectKey{Name: "test-cluster-custom-cb-2-kapp-controller-config",
					Namespace: addonNamespace}
				err := k8sClient.Get(ctx, kappConfigKey, kappConfig)
				Expect(err).ToNot(HaveOccurred())

			})

			By("Create antrea config in the cluster's namespace with random name", func() {
				datavalues := &cniv1alpha2.AntreaConfigDataValue{DefaultMTU: newDefaultMTU}
				antreaConfig := generateAntreaConfig(configName, clusterNamespace, datavalues)
				err := k8sClient.Create(ctx, antreaConfig)
				Expect(err).ToNot(HaveOccurred())
				err = k8sClient.Get(ctx, client.ObjectKeyFromObject(antreaConfig), antreaConfig)
				Expect(err).ToNot(HaveOccurred())
			})

			By("Create custom clusterbootstrap in clusters namespace", func() {
				f, err := os.Open(clusterBoostrapFile)
				Expect(err).ToNot(HaveOccurred())
				defer f.Close()
				Expect(testutil.CreateResources(f, cfg, dynamicClient)).To(Succeed())

			})

			By("Create the cluster", func() {
				cluster := generateDockerCluster(clusterName, clusterNamespace, tkrString, clusterInfraName)
				err := k8sClient.Create(ctx, cluster)
				Expect(err).ToNot(HaveOccurred())
				err = k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), cluster)
				Expect(err).ToNot(HaveOccurred())

			})

			By("Verify contents of resulting antrea config", func() {
				// eventually the secret ref to the data values should be updated
				antreaConfig := &cniv1alpha2.AntreaConfig{}
				Eventually(func() error {
					configKey := client.ObjectKey{
						Namespace: clusterNamespace,
						Name:      configName,
					}
					if err := k8sClient.Get(ctx, configKey, antreaConfig); err != nil {
						return fmt.Errorf("failed to get antrea config '%v': '%v'", configKey, err)
					}
					if antreaConfig.Status.SecretRef == "" {
						return fmt.Errorf("antrea config status not yet updated: %v", configKey)
					}
					Expect(antreaConfig.Status.SecretRef).To(Equal(fmt.Sprintf("%s-%s-data-values", clusterName, constants.AntreaAddonName)))
					return nil
				}, waitTimeout, pollingInterval).Should(Succeed())

				antreaConfig = &cniv1alpha2.AntreaConfig{}
				key := client.ObjectKey{Name: configName, Namespace: clusterNamespace}
				err := k8sClient.Get(ctx, key, antreaConfig)
				Expect(err).ToNot(HaveOccurred())
				Expect(cutil.VerifyOwnerRef(antreaConfig, clusterName, constants.ClusterKind)).To(BeTrue())
				Expect(antreaConfig.Spec.Antrea.AntreaConfigDataValue.DefaultMTU).Should(Equal(newDefaultMTU))
			})

		})
	})
})

func generateAntreaConfig(name, namespace string, datavalues *cniv1alpha2.AntreaConfigDataValue) *cniv1alpha2.AntreaConfig {
	labels := map[string]string{}
	labels["tkg.tanzu.vmware.com/package-name"] = "antrea.tanzu.vmware.com.1.7.2---tkg.1-advanced"
	config := &cniv1alpha2.AntreaConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: cniv1alpha2.AntreaConfigSpec{
			Antrea: cniv1alpha2.Antrea{
				AntreaConfigDataValue: *datavalues,
			},
		},
	}
	return config
}

func generateDockerCluster(name, namespace, tkr, infraName string) *clusterapiv1beta1.Cluster {
	labels := map[string]string{}
	labels["tkg.tanzu.vmware.com/cluster-name"] = name
	labels["run.tanzu.vmware.com/tkr"] = tkr

	pods := &clusterapiv1beta1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16"}}
	services := &clusterapiv1beta1.NetworkRanges{CIDRBlocks: []string{"192.168.0.0/16", "fd00:100:96::/48"}}
	clusterNetworks := &clusterapiv1beta1.ClusterNetwork{
		Services: services,
		Pods:     pods,
	}
	clusterSpecs := clusterapiv1beta1.ClusterSpec{
		ClusterNetwork: clusterNetworks,
		InfrastructureRef: &v1.ObjectReference{
			Kind:       "DockerCluster",
			Name:       infraName,
			APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
		}}

	cluster := &clusterapiv1beta1.Cluster{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec:   clusterSpecs,
		Status: clusterapiv1beta1.ClusterStatus{},
	}
	return cluster
}
