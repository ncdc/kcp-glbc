
##Description:

Describe a proposed new DNS record structure to support geographic based routing of user requests and the changes required in GLBC to support it.
Will demonstrate how this works when a user deploys a workload to multiple sync targets, each in different geographic locations, and how the routing of traffic changes accordingly


##Setup:

Start local-setup with 3 clusters:

```bash
make local-setup NUM_CLUSTERS=3
```

Start glbc:

```bash
(export $(cat ./config/deploy/local/kcp-glbc/controller-config.env | xargs) && export $(cat ./config/deploy/local/kcp-glbc/aws-credentials.env | xargs) && KUBECONFIG=./tmp/kcp.kubeconfig ./bin/kcp-glbc --zap-log-level=3)
```

Add locations:
```bash
kubectl kcp ws root:kuadrant
kubectl apply -f samples/geo-aware-dns/locations.yaml
```

Add placements:
```bash
kubectl kcp ws '~' 
kubectl apply -f config/apiexports/kubernetes/kubernetes-apibinding.yaml
kubectl apply -f config/deploy/local/kcp-glbc/apiexports/glbc/glbc-apibinding.yaml
kubectl apply -f samples/geo-aware-dns/placements.yaml
kubectl delete placement default
```

##Demo:

Show synctargets and locations
```bash
kubectl kcp ws root:kuadrant
kubectl get synctargets -o wide
kubectl get locations -o wide
```

Show placements
```bash
kubectl kcp ws '~'
kubectl get placements
```

Apply sample service
```bash
kubectl apply -f samples/echo-service/echo.yaml
```

Watch annotations being updated
```bash
kubectl get -w ingress echo -o json | jq .metadata.annotations
```

Show updated DNSRecord based on GeoIP lookup
```bash
kubectl get dnsrecord echo -o yaml
```

Update a clusters geo using an API
```bash
kubectl annotate --overwrite ingress echo experimental.meta.workload.kcp.dev/7KGrUokHNdnuScTbJDussTZuYJnX4FafJUtttZ='{"geo":{"continent_code":"NA"}}'
```

Show updated DNSRecord with API chnage for Asia cluster (7KGrUokHNdnuScTbJDussTZuYJnX4FafJUtttZ/kcp-cluster-3)
```bash
kubectl get dnsrecord echo -o yaml
```

Links:

* Show updates in Route53: https://us-east-1.console.aws.amazon.com/route53/v2/hostedzones?region=us-east-1#ListRecordSets/Z04114632NOABXYWH93QU
* Show IPs being returned in different regions: https://www.whatsmydns.net
