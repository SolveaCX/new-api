package controller

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/compute"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Compute admin endpoints power the "flatkey Compute" GPU-rental dashboard.
//
// WHITELABEL: all responses here are strictly flatkey-branded. ComputeNode's
// provider fields are `json:"-"` and compute.Offer's provider identifiers are
// `json:"-"`, so the underlying marketplace (Vast.ai) can never leak through
// these handlers. Do not add provider/contract/host data to any payload below.

// GetComputeNodes lists compute nodes (public fields only) plus a header stat row.
func GetComputeNodes(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)

	nodes, err := model.GetAllComputeNodes(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	total, err := model.CountComputeNodes()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	stats, err := model.GetComputeNodeStats()
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"items":     nodes,
		"total":     total,
		"page":      pageInfo.GetPage(),
		"page_size": pageInfo.GetPageSize(),
		"stats":     stats,
	})
}

// GetComputeNode returns a single compute node by id (public fields only).
func GetComputeNode(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	node, err := model.GetComputeNodeById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "compute node not found")
			return
		}
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, node)
}

// StopComputeNode tears down the upstream instance backing a node and marks it
// stopped. The upstream contract id is used server-side only and is never
// returned to the client.
func StopComputeNode(c *gin.Context) {
	userId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	node, err := model.GetComputeNodeById(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "compute node not found")
			return
		}
		common.ApiError(c, err)
		return
	}

	if err := compute.StopNode(node.ProviderContractID); err != nil {
		if errors.Is(err, compute.ErrProviderNotConfigured) {
			common.ApiErrorMsg(c, "compute provider is not configured")
			return
		}
		common.ApiError(c, err)
		return
	}

	if err := model.UpdateComputeNodeStatus(node.Id, model.ComputeNodeStatusStopped); err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf("stopped compute node %s (id: %d)", node.Label, node.Id))

	common.ApiSuccess(c, gin.H{"id": node.Id, "status": model.ComputeNodeStatusStopped})
}

// GetComputeOffers proxies available upstream GPU offers, stripped of all
// provider branding (only gpu / price / spec fields are surfaced).
func GetComputeOffers(c *gin.Context) {
	gpu := c.Query("gpu")
	offers, err := compute.SearchOffers(gpu)
	if err != nil {
		if errors.Is(err, compute.ErrProviderNotConfigured) {
			common.ApiErrorMsg(c, "compute provider is not configured")
			return
		}
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    gin.H{"items": offers},
	})
}
