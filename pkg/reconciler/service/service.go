package service

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/kcp-dev/logicalcluster/v2"

	"github.com/kuadrant/kcp-glbc/pkg/util/metadata"
	"github.com/kuadrant/kcp-glbc/pkg/util/workloadMigration"
)

func (c *Controller) reconcile(ctx context.Context, service *corev1.Service) error {
	c.Logger.V(3).Info("starting reconcile of service ", "name", service.Name, "namespace", service.Namespace, "cluster", logicalcluster.From(service))
	workloadMigration.Process(service, c.Queue, c.Logger)
	if service.DeletionTimestamp != nil && !service.DeletionTimestamp.IsZero() {
		//in 0.5.0 these are never cleaned up properly
		for _, f := range service.Finalizers {
			if strings.Contains(f, workloadMigration.SyncerFinalizer) {
				metadata.RemoveFinalizer(service, f)
			}
		}
	}
	return nil
}
