let K8s =
      https://raw.githubusercontent.com/dhall-lang/dhall-kubernetes/v6.0.0/package.dhall
        sha256:532e110f424ea8a9f960a13b2ca54779ddcac5d5aa531f86d82f41f8f18d7ef1

let TimeUnit = ../dhall/TimeUnit/package.dhall

let Duration = ../dhall/Duration/package.dhall

let AlertSeverity = ../dhall/AlertSeverity/package.dhall

let PrometheusOperator =
      ( https://raw.githubusercontent.com/coralogix/dhall-prometheus-operator/v8.0.0/package.dhall
          sha256:ebc5f0c5f57d410412c2b7cbb64d0883be648eafc094f0c3e10dba4e6bd46ed4
      ).v1

in  PrometheusOperator.Rule::{
    , alert = Some "GLBCInstanceDown"
    , expr =
        K8s.IntOrString.String
          "(absent(kube_pod_labels{label_glbc_name=\"kcp-stable-redhat-hcg\", pod=~\"kcp-glbc-controller-manager.*\"}) or absent(kube_pod_labels{label_glbc_name=\"kcp-stable-redhat-hcg-unstable\", pod=~\"kcp-glbc-controller-manager.*\"}) or absent(kube_pod_labels{label_glbc_name=\"kcp-unstable-redhat-hcg\", pod=~\"kcp-glbc-controller-manager.*\"})) > 0"
    , for = Some (Duration.show { amount = 5, unit = TimeUnit.Type.Minutes })
    , labels = Some
        (toMap { severity = AlertSeverity.show AlertSeverity.Type.Critical })
    , annotations = Some
        ( toMap
            { summary = "One or more GLBC instances are down"
            , description =
                "One or more GLBC instances are down: {{ \$labels.label_glbc_name }} - Either the GLBC component is not running, is misconfigured, or the metrics endpoint is not responding."
            , runbook_url =
                "https://github.com/Kuadrant/kcp-glbc/blob/main/docs/observability/runbooks/glbctargetdown.adoc"
            }
        )
    }
