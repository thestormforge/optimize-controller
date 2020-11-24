module github.com/thestormforge/optimize-controller

go 1.14

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.4.2 // indirect
	github.com/Masterminds/sprig v2.20.0+incompatible
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1 // indirect
	github.com/huandu/xstrings v1.2.0 // indirect
	github.com/imdario/mergo v0.3.7 // indirect
	github.com/mdp/qrterminal/v3 v3.0.0
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/common v0.4.1
	github.com/spf13/cobra v1.1.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.6.1
	github.com/thestormforge/optimize-go v0.0.7
	github.com/zorkian/go-datadog-api v2.24.0+incompatible
	golang.org/x/net v0.0.0-20200904194848-62affa334b73
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/client-go v0.17.2
	k8s.io/kubectl v0.17.2
	sigs.k8s.io/controller-runtime v0.5.0
	sigs.k8s.io/kustomize/api v0.4.1
	sigs.k8s.io/kustomize/kyaml v0.1.11
	sigs.k8s.io/yaml v1.2.0
)

replace k8s.io/klog => github.com/istio/klog v0.0.0-20190424230111-fb7481ea8bcf
