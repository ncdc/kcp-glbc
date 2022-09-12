package dns

import (
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/kcp-dev/logicalcluster/v2"

	v1 "github.com/kuadrant/kcp-glbc/pkg/apis/kuadrant/v1"
	kuadrantv1 "github.com/kuadrant/kcp-glbc/pkg/client/kuadrant/clientset/versioned"
	"github.com/kuadrant/kcp-glbc/pkg/client/kuadrant/informers/externalversions"
	kuadrantv1lister "github.com/kuadrant/kcp-glbc/pkg/client/kuadrant/listers/kuadrant/v1"
	"github.com/kuadrant/kcp-glbc/pkg/dns"
	awsdns "github.com/kuadrant/kcp-glbc/pkg/dns/aws"
	"github.com/kuadrant/kcp-glbc/pkg/reconciler"
)

const defaultControllerName = "kcp-glbc-dns"

// NewController returns a new Controller which reconciles DNSRecord.
func NewController(config *ControllerConfig) (*Controller, error) {
	controllerName := config.GetName(defaultControllerName)
	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), controllerName)
	c := &Controller{
		Controller:            reconciler.NewController(controllerName, queue),
		dnsRecordClient:       config.DnsRecordClient,
		sharedInformerFactory: config.SharedInformerFactory,
	}
	c.Process = c.process

	dnsProvider, err := c.createDNSProvider(config.DNSProvider)
	if err != nil {
		return nil, err
	}
	c.dnsProvider = dnsProvider

	var dnsZones []v1.DNSZone
	zoneID, zoneIDSet := os.LookupEnv("AWS_DNS_PUBLIC_ZONE_ID")
	if zoneIDSet {
		dnsZone := &v1.DNSZone{
			ID: zoneID,
		}
		dnsZones = append(dnsZones, *dnsZone)
		c.Logger.Info("Using AWS DNS zone", "id", zoneID)
	} else {
		c.Logger.Info("No AWS DNS zone id set (AWS_DNS_PUBLIC_ZONE_ID), no DNS records will be created!")
	}
	c.dnsZones = dnsZones

	//Logging state of AWS credentials
	awsIdKey := os.Getenv("AWS_ACCESS_KEY_ID")
	if awsIdKey != "" {
		c.Logger.Info("AWS Access Key set")
	} else {
		c.Logger.Info("AWS Access Key is NOT set")
	}

	awsSecretKey := os.Getenv("AWS_SECRET_ACCESS_KEY")
	if awsSecretKey != "" {
		c.Logger.Info("AWS Secret Key set")
	} else {
		c.Logger.Info("AWS Secret Key is NOT set")
	}

	c.sharedInformerFactory.Kuadrant().V1().DNSRecords().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) { c.Enqueue(obj) },
		UpdateFunc: func(old, obj interface{}) {
			if old.(*v1.DNSRecord).ResourceVersion != obj.(*v1.DNSRecord).ResourceVersion {
				c.Enqueue(obj)
			}
		},
		DeleteFunc: func(obj interface{}) { c.Enqueue(obj) },
	})

	c.indexer = c.sharedInformerFactory.Kuadrant().V1().DNSRecords().Informer().GetIndexer()
	c.lister = c.sharedInformerFactory.Kuadrant().V1().DNSRecords().Lister()

	return c, nil
}

type ControllerConfig struct {
	*reconciler.ControllerConfig
	DnsRecordClient       kuadrantv1.ClusterInterface
	SharedInformerFactory externalversions.SharedInformerFactory
	DNSProvider           string
}

type Controller struct {
	*reconciler.Controller
	sharedInformerFactory externalversions.SharedInformerFactory
	dnsRecordClient       kuadrantv1.ClusterInterface
	indexer               cache.Indexer
	lister                kuadrantv1lister.DNSRecordLister
	dnsProvider           dns.Provider
	dnsZones              []v1.DNSZone
}

func (c *Controller) process(ctx context.Context, key string) error {
	object, exists, err := c.indexer.GetByKey(key)
	if err != nil {
		return err
	}

	if !exists {
		return nil
	}

	previous := object.(*v1.DNSRecord)
	current := previous.DeepCopy()

	if err = c.reconcile(ctx, current); err != nil {
		return err
	}

	if !equality.Semantic.DeepEqual(previous.Status, current.Status) {
		refresh, err := c.dnsRecordClient.Cluster(logicalcluster.From(current)).KuadrantV1().DNSRecords(current.Namespace).UpdateStatus(ctx, current, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		current.ObjectMeta.ResourceVersion = refresh.ObjectMeta.ResourceVersion
	}

	if !equality.Semantic.DeepEqual(previous, current) {
		_, err := c.dnsRecordClient.Cluster(logicalcluster.From(current)).KuadrantV1().DNSRecords(current.Namespace).Update(ctx, current, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *Controller) createDNSProvider(dnsProviderName string) (dns.Provider, error) {
	var dnsProvider dns.Provider
	var dnsError error
	switch dnsProviderName {
	case "aws":
		dnsProvider, dnsError = newAWSDNSProvider()
	default:
		dnsProvider = &dns.FakeProvider{}
	}
	return dnsProvider, dnsError
}

func newAWSDNSProvider() (dns.Provider, error) {
	var dnsProvider dns.Provider
	provider, err := awsdns.NewProvider(awsdns.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS DNS manager: %v", err)
	}
	dnsProvider = provider

	return dnsProvider, nil
}
