package channel

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	rootconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestDoApiRequestFinalizesOnlyCopilot(t *testing.T) {
	service.InitHttpClient()
	for _, tc := range []struct {
		name    string
		apiType int
		fail    bool
	}{
		{"copilot", rootconstant.APITypeCopilot, false},
		{"copilot failure before send", rootconstant.APITypeCopilot, true},
		{"openai unchanged", rootconstant.APITypeOpenAI, true},
		{"claude unchanged", rootconstant.APITypeAnthropic, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var requests atomic.Int32
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests.Add(1)
				w.WriteHeader(http.StatusOK)
			}))
			defer upstream.Close()
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
			info := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{ApiType: tc.apiType}}
			a := &copilotFinalizerProbe{requestContextAdaptor: requestContextAdaptor{url: upstream.URL}, fail: tc.fail}
			resp, err := DoApiRequest(a, c, info, strings.NewReader(`{}`))
			isCopilot := tc.apiType == rootconstant.APITypeCopilot
			require.Equal(t, isCopilot, a.called)
			if isCopilot && tc.fail {
				require.Error(t, err)
				require.True(t, IsDefinitelyNotSent(err))
				require.Nil(t, resp)
				require.Zero(t, requests.Load())
				return
			}
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)
			require.EqualValues(t, 1, requests.Load())
		})
	}
}

type copilotFinalizerProbe struct {
	requestContextAdaptor
	called bool
	fail   bool
}

func (a *copilotFinalizerProbe) FinalizeRequest(*gin.Context, *http.Request, *relaycommon.RelayInfo) error {
	a.called = true
	if a.fail {
		return MarkDefinitelyNotSent(errors.New("test finalization failure"))
	}
	return nil
}
