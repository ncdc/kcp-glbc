package secret

import (
	"context"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/kcp-dev/logicalcluster/v2"

	"github.com/kuadrant/kcp-glbc/pkg/util/metadata"
	"github.com/kuadrant/kcp-glbc/pkg/util/workloadMigration"
)

func (c *Controller) reconcile(ctx context.Context, secret *corev1.Secret) error {
	c.Logger.V(3).Info("starting reconcile of secret ", "name", secret.Name, "namespace", secret.Namespace, "cluster", logicalcluster.From(secret))
	workloadMigration.Process(secret, c.Queue, c.Logger)
	if secret.DeletionTimestamp != nil && !secret.DeletionTimestamp.IsZero() {
		//in 0.5.0 these are never cleaned up properly
		for _, f := range secret.Finalizers {
			if strings.Contains(f, workloadMigration.SyncerFinalizer) {
				metadata.RemoveFinalizer(secret, f)
			}
		}
	}
	return nil
}
