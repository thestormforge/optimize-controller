module github.com/redskyops/k8s-experiment

go 1.12

require (
	cloud.google.com/go v0.39.0 // indirect
	github.com/Azure/go-autorest/autorest v0.5.0 // indirect
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.4.2 // indirect
	github.com/Masterminds/sprig v2.20.0+incompatible
	github.com/docker/spdystream v0.0.0-20181023171402-6480d4af844c // indirect
	github.com/elazarl/goproxy v0.0.0-20190711103511-473e67f1d7d2 // indirect
	github.com/elazarl/goproxy/ext v0.0.0-20190711103511-473e67f1d7d2 // indirect
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20190129154638-5b532d6fd5ef // indirect
	github.com/google/gofuzz v1.0.0 // indirect
	github.com/google/uuid v1.1.1 // indirect
	github.com/gophercloud/gophercloud v0.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.9.4 // indirect
	github.com/huandu/xstrings v1.2.0 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/json-iterator/go v1.1.6 // indirect
	github.com/modern-go/reflect2 v1.0.1 // indirect
	github.com/onsi/ginkgo v1.8.0
	github.com/onsi/gomega v1.5.0
	github.com/pborman/uuid v0.0.0-20180906182336-adf5a7427709 // indirect
	github.com/prometheus/client_golang v0.9.3
	github.com/prometheus/common v0.4.1
	github.com/prometheus/procfs v0.0.2 // indirect
	github.com/spf13/cobra v0.0.4
	github.com/spf13/pflag v1.0.3
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/zap v1.10.0 // indirect
	golang.org/x/crypto v0.0.0-20190605123033-f99c8df09eb5 // indirect
	golang.org/x/net v0.0.0-20190603091049-60506f45cf65
	golang.org/x/oauth2 v0.0.0-20190604053449-0f29369cfe45
	golang.org/x/sys v0.0.0-20190602015325-4c4f7f33c9ed // indirect
	golang.org/x/time v0.0.0-20190308202827-9d24e82272b4 // indirect
	google.golang.org/api v0.7.0 // indirect
	google.golang.org/appengine v1.6.0 // indirect
	google.golang.org/genproto v0.0.0-20190708153700-3bdd9d9f5532 // indirect
	google.golang.org/grpc v1.22.0 // indirect
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	k8s.io/klog v0.3.2 // indirect
	k8s.io/kube-openapi v0.0.0-20190603182131-db7b694dc208 // indirect
	k8s.io/utils v0.0.0-20190529001817-6999998975a7 // indirect
	sigs.k8s.io/controller-runtime v0.2.0
	sigs.k8s.io/yaml v1.1.0
)

replace k8s.io/klog => github.com/istio/klog v0.0.0-20190424230111-fb7481ea8bcf
