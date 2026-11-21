package handler

import (
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestPPVideoRouteSourceLine(t *testing.T) {
	t.Parallel()

	// The line comes from the group's platform and the model of the prepared request.
	line := ppVideoRouteSourceLine(service.PlatformMiaowuSeedance, service.PPVideoPublicRequest{UpstreamModel: "seedance-2.5-pro"}, nil)
	require.NotNil(t, line)
	require.Equal(t, "Seedance2.5--720p--pro--按次--W", line.Name)

	// Without an upstream model on the public request, the body's model is used.
	line = ppVideoRouteSourceLine(service.PlatformMiaowuSeedance, service.PPVideoPublicRequest{}, []byte(`{"model":"seedance-2.5-pro"}`))
	require.NotNil(t, line)
	require.Equal(t, "Seedance2.5--720p--pro--按次--W", line.Name)

	// A model of another platform is no line of this group, and neither is a missing model.
	require.Nil(t, ppVideoRouteSourceLine(service.PlatformHuahuoSeedance, service.PPVideoPublicRequest{UpstreamModel: "seedance-2.5-pro"}, nil))
	require.Nil(t, ppVideoRouteSourceLine(service.PlatformMiaowuSeedance, service.PPVideoPublicRequest{}, []byte(`{}`)))
}
