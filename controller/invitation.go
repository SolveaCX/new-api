package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
)

const (
	defaultInvitationPage     = 1
	defaultInvitationPageSize = 10
	maxInvitationPageSize     = 100
)

type invitationSummary struct {
	InviterRewardQuota    int   `json:"inviter_reward_quota"`
	InviteeRewardQuota    int   `json:"invitee_reward_quota"`
	InviterRewardMaxCount int   `json:"inviter_reward_max_count"`
	HistoryQuota          int   `json:"history_quota"`
	TransferableQuota     int   `json:"transferable_quota"`
	GrantedCount          int   `json:"granted_count"`
	PendingCount          int64 `json:"pending_count"`
	TransferEnabled       bool  `json:"transfer_enabled"`
}

type invitationResponse struct {
	Summary  invitationSummary        `json:"summary"`
	Items    []model.InvitationRecord `json:"items"`
	Page     int                      `json:"page"`
	PageSize int                      `json:"page_size"`
	Total    int64                    `json:"total"`
}

func getInvitationPagination(c *gin.Context) (int, int) {
	page := defaultInvitationPage
	if rawPage, exists := c.GetQuery("page"); exists {
		if parsedPage, err := strconv.Atoi(rawPage); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	} else if rawPage, exists := c.GetQuery("p"); exists {
		if parsedPage, err := strconv.Atoi(rawPage); err == nil && parsedPage > 0 {
			page = parsedPage
		}
	}

	pageSize := defaultInvitationPageSize
	if rawPageSize, exists := c.GetQuery("page_size"); exists {
		if parsedPageSize, err := strconv.Atoi(rawPageSize); err == nil && parsedPageSize > 0 {
			pageSize = parsedPageSize
		}
	}
	if pageSize > maxInvitationPageSize {
		pageSize = maxInvitationPageSize
	}

	return page, pageSize
}

func GetSelfInvitations(c *gin.Context) {
	page, pageSize := getInvitationPagination(c)
	user, err := model.GetUserById(c.GetInt("id"), true)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	invitationPage, err := model.GetInvitationPage(user.Id, (page-1)*pageSize, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, invitationResponse{
		Summary: invitationSummary{
			InviterRewardQuota:    common.QuotaForInviter,
			InviteeRewardQuota:    common.QuotaForInvitee,
			InviterRewardMaxCount: common.QuotaForInviterMaxCount,
			HistoryQuota:          user.AffHistoryQuota,
			TransferableQuota:     user.AffQuota,
			GrantedCount:          user.AffCount,
			PendingCount:          invitationPage.PendingCount,
			TransferEnabled:       operation_setting.IsPaymentComplianceConfirmed(),
		},
		Items:    invitationPage.Items,
		Page:     page,
		PageSize: pageSize,
		Total:    invitationPage.Total,
	})
}
