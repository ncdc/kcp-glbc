package route

import (
	"context"

	tenancyv1alpha1 "github.com/kcp-dev/kcp/pkg/apis/tenancy/v1alpha1"
	"github.com/kcp-dev/logicalcluster/v2"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	runtimeUtils "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	routeapiv1 "github.com/openshift/api/route/v1"

	certmaninformer "github.com/jetstack/cert-manager/pkg/client/informers/externalversions"
	certmanlister "github.com/jetstack/cert-manager/pkg/client/listers/certmanager/v1"
	kuadrantclientv1 "github.com/kuadrant/kcp-glbc/pkg/client/kuadrant/clientset/versioned"
	dnsrecordinformer "github.com/kuadrant/kcp-glbc/pkg/client/kuadrant/informers/externalversions"
	"github.com/kuadrant/kcp-glbc/pkg/net"
	basereconciler "github.com/kuadrant/kcp-glbc/pkg/reconciler"
	"github.com/kuadrant/kcp-glbc/pkg/tls"
)

const (
	controllerName                      = "kcp-glbc-route"
	annotationIngressKey                = "kuadrant.dev/ingress-key"
	annotationCertificateState          = "kuadrant.dev/certificate-status"
	ANNOTATION_HCG_HOST                 = "kuadrant.dev/host.generated"
	ANNOTATION_HEALTH_CHECK_PREFIX      = "kuadrant.experimental/health-"
	ANNOTATION_HCG_CUSTOM_HOST_REPLACED = "kuadrant.dev/custom-hosts.replaced"
	LABEL_HCG_MANAGED                   = "kuadrant.dev/hcg.managed"
)

// NewController returns a new Controller which reconciles Routes.
func NewController(config *ControllerConfig) *Controller {
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)

	hostResolver := config.HostResolver
	switch impl := hostResolver.(type) {
	case *net.ConfigMapHostResolver:
		impl.Client = config.KubeClient.Cluster(tenancyv1alpha1.RootCluster)
	}
	hostResolver = net.NewSafeHostResolver(hostResolver)

	base := basereconciler.NewController(controllerName, queue)
	c := &Controller{
		Controller:                   base,
		kubeClient:                   config.KubeClient,
		kubeDynamicClient:            config.KubeDynamicClient,
		certProvider:                 config.CertProvider,
		sharedInformerFactory:        config.KCPSharedInformerFactory,
		dynamicSharedInformerFactory: config.KCPDynamicSharedInformerFactory,
		glbcInformerFactory:          config.GlbcInformerFactory,
		dnsRecordClient:              config.DnsRecordClient,
		domain:                       config.Domain,
		hostResolver:                 hostResolver,
		hostsWatcher:                 net.NewHostsWatcher(&base.Logger, hostResolver, net.DefaultInterval),
		customHostsEnabled:           config.CustomHostsEnabled,
		certInformerFactory:          config.CertificateInformer,
		dnsRecordInformerFactory:     config.DNSRecordInformer,
	}
	c.Process = c.process

	routeResource := schema.GroupVersionResource{Group: "route.openshift.io", Version: "v1", Resource: "routes"}
	c.indexer = c.dynamicSharedInformerFactory.ForResource(routeResource).Informer().GetIndexer()
	c.routeLister = c.dynamicSharedInformerFactory.ForResource(routeResource).Lister()
	c.dynamicSharedInformerFactory.ForResource(routeResource).Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.V(3).Info("route add", "name", u.GetName())
			route := &routeapiv1.Route{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, route)
			c.Logger.V(3).Info("route add", "err", err)
			c.Logger.V(3).Info("route add", "route.Spec.Host", route.Spec.Host)
			c.Enqueue(route)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			u := newObj.(*unstructured.Unstructured)
			c.Logger.V(3).Info("route update", "name", u.GetName())
			route := &routeapiv1.Route{}
			_ = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, route)
			c.Enqueue(route)
		},
		DeleteFunc: func(obj interface{}) {
			u := obj.(*unstructured.Unstructured)
			c.Logger.V(3).Info("route delete", "name", u.GetName())
			route := &routeapiv1.Route{}
			_ = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, route)
			c.Enqueue(route)
		},
	})
	return c
}

type ControllerConfig struct {
	KubeClient        kubernetes.ClusterInterface
	KubeDynamicClient dynamic.ClusterInterface
	DnsRecordClient   kuadrantclientv1.ClusterInterface
	// informer for
	KCPSharedInformerFactory        informers.SharedInformerFactory
	KCPDynamicSharedInformerFactory dynamicinformer.DynamicSharedInformerFactory
	CertificateInformer             certmaninformer.SharedInformerFactory
	GlbcInformerFactory             informers.SharedInformerFactory
	DNSRecordInformer               dnsrecordinformer.SharedInformerFactory
	Domain                          string
	CertProvider                    tls.Provider
	HostResolver                    net.HostResolver
	CustomHostsEnabled              bool
}

type Controller struct {
	*basereconciler.Controller
	kubeClient                   kubernetes.ClusterInterface
	kubeDynamicClient            dynamic.ClusterInterface
	sharedInformerFactory        informers.SharedInformerFactory
	dynamicSharedInformerFactory dynamicinformer.DynamicSharedInformerFactory
	dnsRecordClient              kuadrantclientv1.ClusterInterface
	indexer                      cache.Indexer
	routeLister                  cache.GenericLister
	certificateLister            certmanlister.CertificateLister
	certProvider                 tls.Provider
	domain                       string
	hostResolver                 net.HostResolver
	hostsWatcher                 *net.HostsWatcher
	customHostsEnabled           bool
	certInformerFactory          certmaninformer.SharedInformerFactory
	glbcInformerFactory          informers.SharedInformerFactory
	dnsRecordInformerFactory     dnsrecordinformer.SharedInformerFactory
}

func (c *Controller) enqueueIngressByKey(key string) {
	route, err := c.getRouteByKey(key)
	//no need to handle not found as the route is gone
	if err != nil {
		if errors.IsNotFound(err) {
			return
		}
		runtimeUtils.HandleError(err)
		return
	}
	c.Enqueue(route)
}

func (c *Controller) process(ctx context.Context, key string) error {
	object, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	if !exists {
		return nil
	}

	u := object.(*unstructured.Unstructured)
	current := &routeapiv1.Route{}
	_ = runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, current)
	target := current.DeepCopy()

	err = c.reconcile(ctx, target)
	if err != nil {
		return err
	}
	if !equality.Semantic.DeepEqual(current, target) {
		c.Logger.V(3).Info("attempting update of changed route ", "route key ", key)
		raw, _ := runtime.DefaultUnstructuredConverter.ToUnstructured(target)
		u = &unstructured.Unstructured{}
		u.Object = raw
		routeResource := schema.GroupVersionResource{Group: "route.openshift.io", Version: "v1", Resource: "routes"}
		_, err = c.kubeDynamicClient.Cluster(logicalcluster.From(target)).Resource(routeResource).Namespace(target.Namespace).Update(ctx, u, metav1.UpdateOptions{})
		return err
	}

	return nil
}

func (c *Controller) getRouteByKey(key string) (*routeapiv1.Route, error) {
	i, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(routeapiv1.Resource("route"), key)
	}
	return i.(*routeapiv1.Route), nil
}
