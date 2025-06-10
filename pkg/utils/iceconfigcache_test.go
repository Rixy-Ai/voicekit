package utils

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/voicekit/protocol/voicekit"
)

func TestIceConfigCache(t *testing.T) {
	cache := NewIceConfigCache[string](10 * time.Second)
	t.Cleanup(cache.Stop)

	cache.Put("test", &voicekit.ICEConfig{})
	require.NotNil(t, cache)
}
