package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestChannelSupportsRequestEndpointRestrictsOpenRouterDecisions(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/api/alpha/decisions", nil)

	require.Equal(t, constant.EndpointTypeOpenRouterDecisions, requestedEndpointType(ctx))
	require.True(t, ChannelSupportsRequestEndpoint(ctx, &model.Channel{
		Type: constant.ChannelTypeOpenRouter,
	}, "typesafe/jev-1.13"))
	require.False(t, ChannelSupportsRequestEndpoint(ctx, &model.Channel{
		Type: constant.ChannelTypeOpenAI,
	}, "typesafe/jev-1.13"))
}
