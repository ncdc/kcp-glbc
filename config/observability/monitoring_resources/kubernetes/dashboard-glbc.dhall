let K8s =
      https://raw.githubusercontent.com/dhall-lang/dhall-kubernetes/v6.0.0/package.dhall
        sha256:532e110f424ea8a9f960a13b2ca54779ddcac5d5aa531f86d82f41f8f18d7ef1

let configmap =
      K8s.ConfigMap::{
      , metadata = K8s.ObjectMeta::{
        , name = Some "glbc-dashboard"
        , namespace = Some "monitoring"
        }
      , data = Some
          ( toMap
              { `glbc_overview.json` = ../common/dashboard_glbc.json as Text }
          )
      }

in  configmap
