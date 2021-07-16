module github.com/thestormforge/optimize-controller/v2

go 1.16

require (
	github.com/Masterminds/goutils v1.1.0 // indirect
	github.com/Masterminds/semver v1.4.2 // indirect
	github.com/Masterminds/sprig v2.20.0+incompatible
	github.com/cenkalti/backoff v2.2.1+incompatible // indirect
	github.com/charmbracelet/bubbles v0.7.6
	github.com/charmbracelet/bubbletea v0.13.1
	github.com/go-logr/logr v0.1.0
	github.com/go-logr/zapr v0.1.1
	github.com/huandu/xstrings v1.2.0 // indirect
	github.com/lestrrat-go/jwx v1.0.6
	github.com/mdp/qrterminal/v3 v3.0.0
	github.com/muesli/termenv v0.7.4
	github.com/newrelic/newrelic-client-go v0.58.5
	github.com/pelletier/go-toml v1.2.0
	github.com/pkg/browser v0.0.0-20180916011732-0a3d74bf9ce4
	github.com/prometheus/client_golang v1.0.0
	github.com/prometheus/common v0.4.1
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.0
	github.com/thestormforge/konjure v0.3.0
	github.com/thestormforge/optimize-go v0.0.13
	github.com/yujunz/go-getter v1.5.1-lite.0.20201201013212-6d9c071adddf
	github.com/zorkian/go-datadog-api v2.24.0+incompatible
	go.uber.org/zap v1.10.0
	golang.org/x/net v0.0.0-20201110031124-69a78807bb2b
	golang.org/x/oauth2 v0.0.0-20200902213428-5d25da1a8d43
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	k8s.io/api v0.17.2
	k8s.io/apimachinery v0.17.2
	k8s.io/cli-runtime v0.17.2
	k8s.io/client-go v0.17.2
	k8s.io/kubectl v0.17.2
	k8s.io/metrics v0.17.2
	sigs.k8s.io/controller-runtime v0.5.0
	sigs.k8s.io/kustomize/api v0.8.6
	sigs.k8s.io/kustomize/kyaml v0.10.17
	sigs.k8s.io/yaml v1.2.0
)

replace k8s.io/klog => github.com/istio/klog v0.0.0-20190424230111-fb7481ea8bcf

// Do not advance yaml.v3 past KYAML, otherwise our formatting will be off
replace gopkg.in/yaml.v3 => gopkg.in/yaml.v3 v3.0.0-20200313102051-9f266ea9e77c
