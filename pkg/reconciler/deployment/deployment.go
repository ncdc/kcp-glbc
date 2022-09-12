package deployment

import (
	"context"
	"strings"

	appsv1 "k8s.io/api/apps/v1"

	"github.com/kcp-dev/logicalcluster/v2"

	"github.com/kuadrant/kcp-glbc/pkg/util/metadata"
	"github.com/kuadrant/kcp-glbc/pkg/util/workloadMigration"
)

func (c *Controller) reconcile(ctx context.Context, deployment *appsv1.Deployment) error {
	c.Logger.V(3).Info("starting reconcile of deployment ", "name", deployment.Name, "namespace", deployment.Namespace, "cluster", logicalcluster.From(deployment))
	workloadMigration.Process(deployment, c.Queue, c.Logger)
	if deployment.DeletionTimestamp != nil && !deployment.DeletionTimestamp.IsZero() {
		//in 0.5.0 these are never cleaned up properly
		for _, f := range deployment.Finalizers {
			if strings.Contains(f, workloadMigration.SyncerFinalizer) {
				metadata.RemoveFinalizer(deployment, f)
			}
		}
	}
	return nil
}
