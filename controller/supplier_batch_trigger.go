package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type supplierDailyBatchCatchUpFunc func(context.Context, *gorm.DB, *gorm.DB, string, time.Time) (service.SupplierDailyBatchCatchUpResult, error)

func TriggerSupplierDailyBatchCatchUp(c *gin.Context) {
	triggerSupplierDailyBatchCatchUp(
		c,
		service.CatchUpSupplierDailyBatches,
		model.DB,
		model.LOG_DB,
		common.GetReplicaID()+":supplier-daily-batch:"+common.GetUUID(),
		time.Now(),
	)
}

func triggerSupplierDailyBatchCatchUp(c *gin.Context, catchUp supplierDailyBatchCatchUpFunc, mainDB, logDB *gorm.DB, owner string, now time.Time) {
	if mainDB == nil || logDB == nil {
		supplierDailyBatchCatchUpError(c, model.ErrDatabase, owner, now)
		return
	}
	result, err := catchUp(c.Request.Context(), mainDB, logDB, owner, now)
	if err == nil {
		common.ApiSuccess(c, result)
		return
	}
	supplierDailyBatchCatchUpError(c, err, owner, now)
}

func supplierDailyBatchCatchUpError(c *gin.Context, err error, owner string, now time.Time) {
	if errors.Is(err, model.ErrSupplierDailyBatchBusy) {
		supplierDailyBatchTriggerError(c, http.StatusConflict, "busy", i18n.MsgSupplyChainConflict)
		return
	}
	logger.LogError(c.Request.Context(), fmt.Sprintf(
		"supplier daily batch catch-up trigger failed target_batch_date=%s owner=%q triggered_at=%q error_category=%s error_type=%T",
		supplierDailyBatchTargetDate(now), owner, now.Format(time.RFC3339Nano), supplierDailyBatchErrorCategory(err), err,
	))
	supplierDailyBatchTriggerError(c, http.StatusInternalServerError, "error", i18n.MsgSupplyChainInternalError)
}

func supplierDailyBatchErrorCategory(err error) string {
	switch {
	case errors.Is(err, model.ErrDatabase):
		return "database"
	case errors.Is(err, model.ErrSupplierDailyBatchFenceLost):
		return "fence_lost"
	default:
		return "internal"
	}
}

func supplierDailyBatchTargetDate(now time.Time) string {
	location, err := time.LoadLocation(service.SupplierDailyBatchTimezone)
	if err != nil {
		return "unknown"
	}
	return now.In(location).AddDate(0, 0, -1).Format("2006-01-02")
}

func supplierDailyBatchTriggerError(c *gin.Context, status int, machineStatus, messageKey string) {
	c.JSON(status, gin.H{
		"success": false,
		"message": common.TranslateMessage(c, messageKey),
		"data":    gin.H{"status": machineStatus},
	})
}
