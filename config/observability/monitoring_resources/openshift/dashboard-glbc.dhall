let K8s =
      https://raw.githubusercontent.com/dhall-lang/dhall-kubernetes/v6.0.0/package.dhall
        sha256:532e110f424ea8a9f960a13b2ca54779ddcac5d5aa531f86d82f41f8f18d7ef1

let GrafanaOperator =
      https://raw.githubusercontent.com/david-martin/dhall-grafana-operator/main/package.dhall

let dashboardjson = ../common/dashboard_glbc.json as Text

let dashboard =
      GrafanaOperator.GrafanaDashboard::{
      , metadata = K8s.ObjectMeta::{
        , name = Some "glbc-dashboard"
        , namespace = Some "monitoring"
        , labels = Some [ { mapKey = "app", mapValue = "glbc" } ]
        }
      , spec = Some
        { configMapRef =
            None { key : Text, name : Optional Text, optional : Optional Bool }
        , customFolderName = None Text
        , datasources = None (List { datasourceName : Text, inputName : Text })
        , grafanaCom = None { id : Integer, revision : Optional Integer }
        , json = Some dashboardjson
        , jsonnet = None Text
        , plugins = None (List { name : Text, version : Text })
        , url = None Text
        }
      }

in  dashboard
