let K8s =
      https://raw.githubusercontent.com/dhall-lang/dhall-kubernetes/v6.0.0/package.dhall
        sha256:532e110f424ea8a9f960a13b2ca54779ddcac5d5aa531f86d82f41f8f18d7ef1

let PrometheusOperator =
      ( https://raw.githubusercontent.com/coralogix/dhall-prometheus-operator/v8.0.0/package.dhall
          sha256:ebc5f0c5f57d410412c2b7cbb64d0883be648eafc094f0c3e10dba4e6bd46ed4
      ).v1

in  PrometheusOperator.PodMonitor::{
    , metadata = K8s.ObjectMeta::{
      , name = Some "kcp-glbc-controller-manager"
      , labels = Some
        [ { mapKey = "app", mapValue = "glbc" }
        , { mapKey = "app.kubernetes.io/name", mapValue = "kcp-glbc" }
        , { mapKey = "app.kubernetes.io/component"
          , mapValue = "controller-manager"
          }
        ]
      }
    , spec = PrometheusOperator.PodMonitorSpec::{
      , namespaceSelector = Some
          (PrometheusOperator.NamespaceSelector.Any { any = True })
      , selector = K8s.LabelSelector::{
        , matchLabels = Some
          [ { mapKey = "app.kubernetes.io/name", mapValue = "kcp-glbc" }
          , { mapKey = "app.kubernetes.io/component"
            , mapValue = "controller-manager"
            }
          ]
        }
      , podMetricsEndpoints = Some
        [ PrometheusOperator.PodMetricsEndpoint::{ port = Some "metrics" } ]
      }
    }
