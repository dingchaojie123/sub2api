package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func modelsListBodyForTest(t *testing.T, platform string, modelIDs []string) string {
	t.Helper()
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/models", nil)

	writeModelsList(c, platform, modelIDs)

	require.Equal(t, http.StatusOK, rec.Code)
	return rec.Body.String()
}

func protocolsForTest(body string, path string) []string {
	protocols := make([]string, 0, 2)
	for _, item := range gjson.Get(body, path).Array() {
		protocols = append(protocols, item.String())
	}
	return protocols
}

func TestWriteModelsListAddsCapabilitiesForVideoGroups(t *testing.T) {
	body := modelsListBodyForTest(t, service.PlatformWan3, []string{"wan3.0-video", "an-unknown-model"})

	require.Equal(t, "list", gjson.Get(body, "object").String())
	require.Equal(t, "wan3.0-video", gjson.Get(body, "data.0.id").String())
	require.Equal(t, "model", gjson.Get(body, "data.0.type").String())
	require.Equal(t, "wan3.0-video", gjson.Get(body, "data.0.display_name").String())
	require.Equal(t, []string{"v1", "v2"}, protocolsForTest(body, "data.0.capabilities.protocols"))
	require.Equal(t, int64(2), gjson.Get(body, "data.0.capabilities.duration.min").Int())
	require.Equal(t, int64(30), gjson.Get(body, "data.0.capabilities.duration.max").Int())
	require.Equal(t, "720p", gjson.Get(body, "data.0.capabilities.resolutions.1").String())
	require.Equal(t, int64(10), gjson.Get(body, "data.0.capabilities.assets.reference_images").Int())
	require.True(t, gjson.Get(body, "data.0.capabilities.generate_audio").Bool())

	// A model without detailed data still reports which protocols its group accepts.
	require.Equal(t, "an-unknown-model", gjson.Get(body, "data.1.id").String())
	require.Equal(t, []string{"v1", "v2"}, protocolsForTest(body, "data.1.capabilities.protocols"))
	require.False(t, gjson.Get(body, "data.1.capabilities.duration").Exists())
	require.False(t, gjson.Get(body, "data.1.capabilities.resolutions").Exists())
}

func TestWriteModelsListReportsV1OnlyForGroupsWithoutUnifiedSupport(t *testing.T) {
	body := modelsListBodyForTest(t, service.PlatformKling, []string{"kling-v3"})

	require.Equal(t, "kling-v3", gjson.Get(body, "data.0.id").String())
	require.Equal(t, []string{"v1"}, protocolsForTest(body, "data.0.capabilities.protocols"))
	require.False(t, gjson.Get(body, "data.0.capabilities.duration").Exists())
}

func TestWriteModelsListKeepsAnEmptyVideoListAsAnArray(t *testing.T) {
	body := modelsListBodyForTest(t, service.PlatformPixverseV6, nil)

	require.True(t, gjson.Get(body, "data").IsArray())
	require.Len(t, gjson.Get(body, "data").Array(), 0)
}

func TestWriteModelsListLeavesNonVideoPlatformsUnchanged(t *testing.T) {
	body := modelsListBodyForTest(t, service.PlatformAnthropic, []string{"claude-sonnet-4-6"})

	require.Equal(t, "claude-sonnet-4-6", gjson.Get(body, "data.0.id").String())
	require.False(t, gjson.Get(body, "data.0.capabilities").Exists())
}
