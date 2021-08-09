package generation

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	optimizeappsv1alpha1 "github.com/thestormforge/optimize-controller/v2/api/apps/v1alpha1"
)

func TestFixRef(t *testing.T) {
	testCases := []struct {
		desc     string
		data     string
		expected string
	}{
		{
			desc:     "strip newline",
			data:     "im a little teajwt\n",
			expected: "im a little teajwt",
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%q", tc.desc), func(t *testing.T) {
			os.Setenv("STORMFORGER_TESTTESTTEST_JWT", tc.data)
			defer os.Unsetenv("STORMFORGER_TESTTESTTEST_JWT")

			s := &StormForgerSource{Application: &optimizeappsv1alpha1.Application{}}
			token := s.stormForgerAccessToken("TESTTESTTEST")

			assert.Equal(t, tc.expected, token.Literal)
		})
	}
}
