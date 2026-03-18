package kubernetes

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gitopshq-io/agent/internal/adapter/render"
	"github.com/gitopshq-io/agent/internal/domain"
	cfgpkg "github.com/gitopshq-io/agent/internal/platform/config"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	managedByLabelKey   = "gitopshq.io/managed-by"
	managedByLabelValue = "gitopshq-agent"
	versionLabelKey     = "gitopshq.io/credential-version"
)

type Client struct {
	typed            kubernetes.Interface
	dynamic          dynamic.Interface
	mapper           *restmapper.DeferredDiscoveryRESTMapper
	fieldManager     string
	forceOwnership   bool
	defaultNamespace string
}

func New(cfg cfgpkg.DirectDeployConfig) (*Client, error) {
	restConfig, err := loadRESTConfig(cfg.KubeconfigPath)
	if err != nil {
		return nil, err
	}
	typedClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create typed kubernetes client: %w", err)
	}
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create dynamic kubernetes client: %w", err)
	}
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("create discovery client: %w", err)
	}
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
	return &Client{
		typed:            typedClient,
		dynamic:          dynamicClient,
		mapper:           mapper,
		fieldManager:     cfg.FieldManager,
		forceOwnership:   cfg.ForceOwnership,
		defaultNamespace: resolveDefaultNamespace(cfg.DefaultNamespace),
	}, nil
}

func NewWithClients(typedClient kubernetes.Interface, dynamicClient dynamic.Interface, mapper *restmapper.DeferredDiscoveryRESTMapper, defaultNamespace, fieldManager string) *Client {
	return &Client{
		typed:            typedClient,
		dynamic:          dynamicClient,
		mapper:           mapper,
		fieldManager:     defaultString(fieldManager, "gitopshq-agent"),
		forceOwnership:   false,
		defaultNamespace: resolveDefaultNamespace(defaultNamespace),
	}
}

func loadRESTConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	}
	if kubeconfigEnv := os.Getenv("KUBECONFIG"); kubeconfigEnv != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfigEnv)
	}
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home dir for kubeconfig: %w", err)
	}
	return clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
}

func resolveDefaultNamespace(configured string) string {
	if configured != "" {
		return configured
	}
	if data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace"); err == nil {
		if namespace := strings.TrimSpace(string(data)); namespace != "" {
			return namespace
		}
	}
	return "default"
}

func (c *Client) DefaultNamespace() string {
	return c.defaultNamespace
}

func (c *Client) CollectInventory(ctx context.Context) (*domain.InventorySnapshot, error) {
	namespaces, err := c.typed.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}
	nodes, err := c.typed.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	pods, err := c.typed.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}
	deployments, err := c.typed.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	services, err := c.typed.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}

	resources := make([]domain.ResourceRef, 0, len(namespaces.Items)+len(nodes.Items)+len(deployments.Items)+len(services.Items))
	for _, namespace := range namespaces.Items {
		resources = append(resources, domain.ResourceRef{
			Kind:   "Namespace",
			Name:   namespace.Name,
			Status: string(namespace.Status.Phase),
			Labels: namespace.Labels,
		})
	}
	readyNodeCount := 0
	for _, node := range nodes.Items {
		status := "Unknown"
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady {
				status = string(condition.Status)
				if condition.Status == corev1.ConditionTrue {
					readyNodeCount++
					status = "Ready"
				}
				break
			}
		}
		resources = append(resources, domain.ResourceRef{
			Kind:   "Node",
			Name:   node.Name,
			Status: status,
			Labels: node.Labels,
		})
	}
	for _, deployment := range deployments.Items {
		resources = append(resources, domain.ResourceRef{
			Kind:      "Deployment",
			Namespace: deployment.Namespace,
			Name:      deployment.Name,
			Status:    deploymentStatus(deployment),
			Labels:    deployment.Labels,
		})
	}
	for _, service := range services.Items {
		resources = append(resources, domain.ResourceRef{
			Kind:      "Service",
			Namespace: service.Namespace,
			Name:      service.Name,
			Status:    string(service.Spec.Type),
			Labels:    service.Labels,
		})
	}
	sort.Slice(resources, func(i, j int) bool {
		if resources[i].Kind == resources[j].Kind {
			if resources[i].Namespace == resources[j].Namespace {
				return resources[i].Name < resources[j].Name
			}
			return resources[i].Namespace < resources[j].Namespace
		}
		return resources[i].Kind < resources[j].Kind
	})

	return &domain.InventorySnapshot{
		Timestamp: time.Now().UTC(),
		Summary: domain.InventorySummary{
			ClusterName:     "kubernetes",
			NamespaceCount:  len(namespaces.Items),
			NodeCount:       len(nodes.Items),
			ReadyNodeCount:  readyNodeCount,
			PodCount:        len(pods.Items),
			DeploymentCount: len(deployments.Items),
		},
		Resources: resources,
	}, nil
}

func (c *Client) CollectDrift(_ context.Context) (*domain.DriftReport, error) {
	return &domain.DriftReport{Timestamp: time.Now().UTC()}, nil
}

func (c *Client) ReadSecretData(ctx context.Context, ref domain.CredentialRef) (map[string][]byte, error) {
	namespace := ref.Namespace
	if namespace == "" {
		namespace = c.defaultNamespace
	}
	secret, err := c.typed.CoreV1().Secrets(namespace).Get(ctx, ref.SecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("read secret %s/%s: %w", namespace, ref.SecretName, err)
	}
	return secret.Data, nil
}

func (c *Client) MirrorCredentials(ctx context.Context, req domain.CredentialSyncRequest, allowedTargets []string) (domain.CredentialSyncResult, error) {
	allowed := make(map[string]struct{}, len(allowedTargets))
	for _, target := range allowedTargets {
		if target == "" {
			continue
		}
		allowed[target] = struct{}{}
	}

	desired := make(map[string]map[string]struct{})
	for _, bundle := range req.Bundles {
		namespace := bundle.Namespace
		if namespace == "" {
			namespace = c.defaultNamespace
		}
		if len(allowed) > 0 {
			if _, ok := allowed[namespace]; !ok {
				return domain.CredentialSyncResult{}, fmt.Errorf("namespace %q is not allowed for credential sync", namespace)
			}
		}
		if _, ok := desired[namespace]; !ok {
			desired[namespace] = make(map[string]struct{})
		}
		desired[namespace][bundle.SecretName] = struct{}{}

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        bundle.SecretName,
				Namespace:   namespace,
				Labels:      mergeStringMaps(bundle.Labels, map[string]string{managedByLabelKey: managedByLabelValue, versionLabelKey: req.Version}),
				Annotations: bundle.Annotations,
			},
			Type:       corev1.SecretType(defaultString(bundle.Type, string(corev1.SecretTypeOpaque))),
			StringData: bundle.StringData,
		}
		if err := c.upsertSecret(ctx, secret); err != nil {
			return domain.CredentialSyncResult{}, err
		}
	}

	namespacesToPrune := make([]string, 0, len(desired))
	if len(allowed) > 0 {
		for namespace := range allowed {
			namespacesToPrune = append(namespacesToPrune, namespace)
		}
	} else {
		for namespace := range desired {
			namespacesToPrune = append(namespacesToPrune, namespace)
		}
	}
	sort.Strings(namespacesToPrune)
	for _, namespace := range namespacesToPrune {
		secretList, err := c.typed.CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: managedByLabelKey + "=" + managedByLabelValue,
		})
		if err != nil {
			return domain.CredentialSyncResult{}, fmt.Errorf("list managed secrets in %s: %w", namespace, err)
		}
		for _, secret := range secretList.Items {
			if _, ok := desired[namespace][secret.Name]; ok {
				continue
			}
			if err := c.typed.CoreV1().Secrets(namespace).Delete(ctx, secret.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return domain.CredentialSyncResult{}, fmt.Errorf("delete managed secret %s/%s: %w", namespace, secret.Name, err)
			}
		}
	}

	result := domain.CredentialSyncResult{
		Version:   req.Version,
		Status:    "synced",
		Message:   fmt.Sprintf("mirrored %d credential bundles", len(req.Bundles)),
		Timestamp: time.Now().UTC(),
	}
	if len(req.Bundles) > 0 {
		result.Namespace = defaultString(req.Bundles[0].Namespace, c.defaultNamespace)
		result.SecretName = req.Bundles[0].SecretName
	}
	if len(req.Bundles) == 0 {
		result.Status = "noop"
		result.Message = "no credential bundles to sync"
	}
	return result, nil
}

func (c *Client) ApplyRendered(ctx context.Context, namespace string, manifests []render.Manifest) ([]domain.ResourceRef, error) {
	if namespace == "" {
		namespace = c.defaultNamespace
	}
	applied := make([]domain.ResourceRef, 0, len(manifests))
	for _, manifest := range manifests {
		object, err := decodeManifest(manifest.Content)
		if err != nil {
			return nil, fmt.Errorf("decode manifest %s: %w", manifest.Path, err)
		}
		if object.GetName() == "" {
			return nil, fmt.Errorf("manifest %s is missing metadata.name", manifest.Path)
		}
		mapping, err := c.mappingFor(object.GroupVersionKind())
		if err != nil {
			return nil, fmt.Errorf("resolve mapping for %s: %w", object.GroupVersionKind().String(), err)
		}

		resourceClient := c.dynamic.Resource(mapping.Resource)
		var resourceInterface dynamic.ResourceInterface = resourceClient
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			targetNamespace := object.GetNamespace()
			if targetNamespace == "" {
				targetNamespace = namespace
				object.SetNamespace(targetNamespace)
			}
			resourceInterface = resourceClient.Namespace(targetNamespace)
		} else {
			object.SetNamespace("")
		}

		payload, err := json.Marshal(object.Object)
		if err != nil {
			return nil, fmt.Errorf("marshal manifest %s: %w", manifest.Path, err)
		}
		options := metav1.PatchOptions{
			FieldManager: c.fieldManager,
		}
		if c.forceOwnership {
			force := true
			options.Force = &force
		}
		appliedObject, err := resourceInterface.Patch(ctx, object.GetName(), types.ApplyPatchType, payload, options)
		if err != nil {
			return nil, fmt.Errorf("apply %s/%s: %w", object.GetKind(), object.GetName(), err)
		}
		applied = append(applied, domain.ResourceRef{
			Kind:      appliedObject.GetKind(),
			Namespace: appliedObject.GetNamespace(),
			Name:      appliedObject.GetName(),
			Labels:    appliedObject.GetLabels(),
		})
	}
	return applied, nil
}

func (c *Client) RestartWorkload(ctx context.Context, command domain.RestartWorkloadCommand) error {
	resourceInterface, name, err := c.workloadResource(command.Namespace, command.Kind, command.Name)
	if err != nil {
		return err
	}
	patch := fmt.Sprintf(`{"spec":{"template":{"metadata":{"annotations":{"kubectl.kubernetes.io/restartedAt":"%s"}}}}}`, time.Now().UTC().Format(time.RFC3339))
	_, err = resourceInterface.Patch(ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{
		FieldManager: c.fieldManager,
	})
	if err != nil {
		return fmt.Errorf("restart %s/%s: %w", command.Kind, command.Name, err)
	}
	return nil
}

func (c *Client) ScaleWorkload(ctx context.Context, command domain.ScaleWorkloadCommand) error {
	resourceInterface, name, err := c.workloadResource(command.Namespace, command.Kind, command.Name)
	if err != nil {
		return err
	}
	patch := fmt.Sprintf(`{"spec":{"replicas":%d}}`, command.Replicas)
	_, err = resourceInterface.Patch(ctx, name, types.MergePatchType, []byte(patch), metav1.PatchOptions{
		FieldManager: c.fieldManager,
	})
	if err != nil {
		return fmt.Errorf("scale %s/%s: %w", command.Kind, command.Name, err)
	}
	return nil
}

func (c *Client) workloadResource(namespace, kind, name string) (dynamic.ResourceInterface, string, error) {
	if namespace == "" {
		namespace = c.defaultNamespace
	}
	gvr, err := workloadResourceForKind(kind)
	if err != nil {
		return nil, "", err
	}
	return c.dynamic.Resource(gvr).Namespace(namespace), name, nil
}

func (c *Client) mappingFor(gvk schema.GroupVersionKind) (*meta.RESTMapping, error) {
	mapping, err := c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err == nil {
		return mapping, nil
	}
	c.mapper.Reset()
	return c.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
}

func decodeManifest(content string) (*unstructured.Unstructured, error) {
	decoder := k8syaml.NewYAMLOrJSONDecoder(strings.NewReader(content), len(content))
	var raw map[string]any
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: raw}, nil
}

func (c *Client) upsertSecret(ctx context.Context, secret *corev1.Secret) error {
	existing, err := c.typed.CoreV1().Secrets(secret.Namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		_, err = c.typed.CoreV1().Secrets(secret.Namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("create secret %s/%s: %w", secret.Namespace, secret.Name, err)
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("get secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}
	secret.ResourceVersion = existing.ResourceVersion
	_, err = c.typed.CoreV1().Secrets(secret.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}
	return nil
}

func mergeStringMaps(base map[string]string, extra map[string]string) map[string]string {
	if len(base) == 0 && len(extra) == 0 {
		return nil
	}
	out := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}

func deploymentStatus(deployment appsv1.Deployment) string {
	desired := int32(1)
	if deployment.Spec.Replicas != nil {
		desired = *deployment.Spec.Replicas
	}
	if deployment.Status.ReadyReplicas == desired {
		return "Ready"
	}
	return fmt.Sprintf("%d/%d ready", deployment.Status.ReadyReplicas, desired)
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func workloadResourceForKind(kind string) (schema.GroupVersionResource, error) {
	switch strings.ToLower(kind) {
	case "deployment", "deployments":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, nil
	case "statefulset", "statefulsets":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "statefulsets"}, nil
	case "daemonset", "daemonsets":
		return schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "daemonsets"}, nil
	default:
		return schema.GroupVersionResource{}, fmt.Errorf("unsupported workload kind %q", kind)
	}
}
