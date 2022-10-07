let PrometheusOperator =
      ( https://raw.githubusercontent.com/coralogix/dhall-prometheus-operator/v8.0.0/package.dhall
          sha256:ebc5f0c5f57d410412c2b7cbb64d0883be648eafc094f0c3e10dba4e6bd46ed4
      ).v1

in  PrometheusOperator.PrometheusRuleSpec::{
    , groups =
      [ PrometheusOperator.RuleGroup::{
        , name = "glbc"
        , rules = Some [ ./GLBCInstanceDown.dhall ]
        }
      ]
    }
