package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func sendActivationEvent(c *gin.Context, name string, params map[string]any) {
	if c == nil || c.Request == nil {
		return
	}
	clientID, sessionID := service.ResolveGAIdentifiers(c.Request, "", "")
	if clientID == "" || sessionID == "" {
		return
	}
	if params == nil {
		params = map[string]any{}
	}
	params["activation_surface"] = name
	service.SendGAEvent(c, service.GAEvent{
		Name: name, ClientID: clientID, SessionID: sessionID,
		TimestampMicros: common.GetTimestamp() * 1_000_000, Params: params,
	})
}

func sendActivationEventOnSuccess(c *gin.Context, name string, params map[string]any) {
	if c != nil && c.Writer != nil && c.Writer.Status() >= 200 && c.Writer.Status() < 300 {
		sendActivationEvent(c, name, params)
	}
}
