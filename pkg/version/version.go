package version

var (
	Version       = "v1.0.0-alpha.1"
	BuildMetadata = "unreleased"
	GitCommit     = ""
)

func GetVersion() string {
	if BuildMetadata == "" {
		return Version
	}
	return Version + "+" + BuildMetadata
}
