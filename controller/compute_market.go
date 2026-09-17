package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/compute"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Compute Market endpoints (UserAuth unless noted).
//
// Buyers post RFQs and accept bids; suppliers (verified, level >= 1, or any
// admin acting as the flatkey pool) bid. Both sides only ever see each
// other's alias — user ids, company names and contacts are never returned
// across the market boundary. Free-text fields are scrubbed of contact
// details before storage so a buyer cannot leak an email or phone number
// into the public request.

var (
	reContactEmail = regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
	reContactPhone = regexp.MustCompile(`(?:\+?\d[\d\s-]{7,}\d)`)
	reContactIM    = regexp.MustCompile(`(?i)(?:wechat|微信|whatsapp|telegram|tg|line|signal)\s*[:：]?\s*[a-z0-9_\-]{3,}`)
	reContactURL   = regexp.MustCompile(`(?i)https?://\S+`)
)

// scrubContact removes emails, phone numbers, IM handles and links from text
// shown to the other side of the market.
func scrubContact(s string) string {
	s = reContactEmail.ReplaceAllString(s, "[contact removed]")
	s = reContactURL.ReplaceAllString(s, "[link removed]")
	s = reContactIM.ReplaceAllString(s, "[contact removed]")
	s = reContactPhone.ReplaceAllString(s, "[contact removed]")
	return strings.TrimSpace(s)
}

func isComputeMarketAdmin(c *gin.Context) bool {
	return c.GetInt("role") >= common.RoleAdminUser
}

type computeRFQRequest struct {
	GpuModel       string   `json:"gpu_model"`
	GpusPerNode    int      `json:"gpus_per_node"`
	Nodes          int      `json:"nodes"`
	TermMonths     int      `json:"term_months"`
	StartDate      string   `json:"start_date"`
	PriceCeiling   float64  `json:"price_ceiling"`
	TargetPriceMin float64  `json:"target_price_min"`
	TargetPriceMax float64  `json:"target_price_max"`
	Regions        []string `json:"regions"`
	Delivery       string   `json:"delivery"`
	Interconnect   string   `json:"interconnect"`
	StorageTB      float64  `json:"storage_tb"`
	Compliance     []string `json:"compliance"`
	PaymentTerms   string   `json:"payment_terms"`
	Notes          string   `json:"notes"`
}

var reISODateOnly = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

func (r *computeRFQRequest) validate() error {
	r.GpuModel = strings.TrimSpace(r.GpuModel)
	if r.GpuModel == "" || len(r.GpuModel) > 64 {
		return errors.New("please choose a GPU model")
	}
	if r.GpusPerNode <= 0 || r.GpusPerNode > 64 {
		return errors.New("GPUs per node must be between 1 and 64")
	}
	if r.Nodes <= 0 || r.Nodes > 10000 {
		return errors.New("nodes must be between 1 and 10000")
	}
	if r.TermMonths <= 0 || r.TermMonths > 60 {
		return errors.New("term must be between 1 and 60 months")
	}
	if !reISODateOnly.MatchString(strings.TrimSpace(r.StartDate)) {
		return errors.New("start date must be YYYY-MM-DD")
	}
	if r.PriceCeiling <= 0 || r.PriceCeiling > 1000 {
		return errors.New("price ceiling must be a positive USD amount per GPU-hour")
	}
	if r.TargetPriceMax > r.PriceCeiling {
		r.TargetPriceMax = r.PriceCeiling
	}
	if r.TargetPriceMin > r.TargetPriceMax {
		r.TargetPriceMin = r.TargetPriceMax
	}
	if len(r.Regions) == 0 {
		return errors.New("please choose at least one region")
	}
	return nil
}

func joinList(items []string, max int) string {
	out := make([]string, 0, len(items))
	for _, s := range items {
		s = strings.TrimSpace(s)
		if s != "" && len(s) <= 64 {
			out = append(out, s)
		}
		if len(out) == max {
			break
		}
	}
	return strings.Join(out, ",")
}

// ParseComputeRFQ turns a pasted paragraph into a draft RFQ (AI with heuristic fallback).
func ParseComputeRFQ(c *gin.Context) {
	var req struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if len(req.Text) > 8000 {
		req.Text = req.Text[:8000]
	}
	common.ApiSuccess(c, compute.ParseRFQText(c.Request.Context(), req.Text))
}

// CreateComputeRFQ posts a new request into matching.
func CreateComputeRFQ(c *gin.Context) {
	userId := c.GetInt("id")
	var req computeRFQRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := req.validate(); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	rfq := &model.ComputeRFQ{
		UserId: userId, GpuModel: req.GpuModel, GpusPerNode: req.GpusPerNode, Nodes: req.Nodes, TermMonths: req.TermMonths,
		StartDate: strings.TrimSpace(req.StartDate), PriceCeiling: req.PriceCeiling, TargetPriceMin: req.TargetPriceMin, TargetPriceMax: req.TargetPriceMax,
		Regions: joinList(req.Regions, 8), Delivery: strings.TrimSpace(req.Delivery), Interconnect: strings.TrimSpace(req.Interconnect),
		StorageTB: req.StorageTB, Compliance: joinList(req.Compliance, 12), PaymentTerms: strings.TrimSpace(req.PaymentTerms),
		Notes: scrubContact(req.Notes),
	}
	if len(rfq.Notes) > 4000 {
		rfq.Notes = rfq.Notes[:4000]
	}
	if err := rfq.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf("posted compute request %s (%s x %d nodes, %d months)", rfq.Code, rfq.GpuModel, rfq.Nodes, rfq.TermMonths))
	common.ApiSuccess(c, computeRFQView(rfq, userId, c))
}

// computeRFQView returns the RFQ plus fields only the owner may see.
func computeRFQView(rfq *model.ComputeRFQ, userId int, c *gin.Context) gin.H {
	view := gin.H{"rfq": rfq}
	if rfq.UserId == userId || isComputeMarketAdmin(c) {
		view["target_price_min"] = rfq.TargetPriceMin
		view["target_price_max"] = rfq.TargetPriceMax
	}
	return view
}

// ListMyComputeRFQs lists the caller's own requests with bid stats.
func ListMyComputeRFQs(c *gin.Context) {
	userId := c.GetInt("id")
	rfqs, err := model.ListComputeRFQsByUser(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(rfqs))
	for _, r := range rfqs {
		bids, err := model.ListComputeBidsByRFQ(r.Id, userId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		items = append(items, gin.H{
			"rfq":            r,
			"bid_count":      countLiveBids(bids),
			"eligible_count": len(model.TopEligibleComputeBids(bids, 1000)),
			"lowest_price":   model.LowestEligibleComputeBidPrice(bids),
			"accepted_bid":   findBid(bids, r.AcceptedBidId),
		})
	}
	common.ApiSuccess(c, gin.H{"items": items})
}

func countLiveBids(bids []*model.ComputeBid) int {
	n := 0
	for _, b := range bids {
		if b.Status == model.ComputeBidStatusLive || b.Status == model.ComputeBidStatusAccepted {
			n++
		}
	}
	return n
}

func findBid(bids []*model.ComputeBid, id int) *model.ComputeBid {
	if id == 0 {
		return nil
	}
	for _, b := range bids {
		if b.Id == id {
			return b
		}
	}
	return nil
}

// ListOpenComputeRFQs is the supplier board: open requests, buyer aliased.
func ListOpenComputeRFQs(c *gin.Context) {
	userId := c.GetInt("id")
	rfqs, err := model.ListOpenComputeRFQs(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	items := make([]gin.H, 0, len(rfqs))
	for _, r := range rfqs {
		bids, err := model.ListComputeBidsByRFQ(r.Id, userId)
		if err != nil {
			common.ApiError(c, err)
			return
		}
		var mine *model.ComputeBid
		for _, b := range bids {
			if b.Mine && b.Status == model.ComputeBidStatusLive {
				mine = b
			}
		}
		items = append(items, gin.H{
			"rfq":          r,
			"bid_count":    countLiveBids(bids),
			"lowest_price": model.LowestEligibleComputeBidPrice(bids),
			"my_bid":       mine,
		})
	}
	common.ApiSuccess(c, gin.H{"items": items})
}

func loadRFQ(c *gin.Context) (*model.ComputeRFQ, bool) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid request id")
		return nil, false
	}
	rfq, err := model.GetComputeRFQById(id, c.GetInt("id"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "request not found")
			return nil, false
		}
		common.ApiError(c, err)
		return nil, false
	}
	return rfq, true
}

// GetComputeRFQ returns one request with its bid ladder. Owners see every
// bid; suppliers see the public ladder (prices, aliases, eligibility) with
// their own bid flagged. Closed requests are only visible to participants.
func GetComputeRFQ(c *gin.Context) {
	userId := c.GetInt("id")
	rfq, ok := loadRFQ(c)
	if !ok {
		return
	}
	bids, err := model.ListComputeBidsByRFQ(rfq.Id, userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	isOwner := rfq.UserId == userId
	if !isOwner && !isComputeMarketAdmin(c) && rfq.Status != model.ComputeRFQStatusMatching && rfq.Status != model.ComputeRFQStatusChoosing {
		participant := false
		for _, b := range bids {
			if b.Mine {
				participant = true
			}
		}
		if !participant {
			common.ApiErrorMsg(c, "request not found")
			return
		}
	}
	view := computeRFQView(rfq, userId, c)
	view["bids"] = bids
	view["top"] = model.TopEligibleComputeBids(bids, model.ComputeMarketTopN)
	view["lowest_price"] = model.LowestEligibleComputeBidPrice(bids)
	view["is_owner"] = isOwner
	view["accepted_bid"] = findBid(bids, rfq.AcceptedBidId)
	view["now"] = common.GetTimestamp()
	common.ApiSuccess(c, view)
}

type computeBidRequest struct {
	PricePerGpuHour float64         `json:"price_per_gpu_hour"`
	DeliverDate     string          `json:"deliver_date"`
	SlaPct          float64         `json:"sla_pct"`
	HardReqs        map[string]bool `json:"hard_reqs"`
	Deviations      string          `json:"deviations"`
}

// PlaceComputeBid submits or lowers the caller's bid on an open request.
func PlaceComputeBid(c *gin.Context) {
	userId := c.GetInt("id")
	rfq, ok := loadRFQ(c)
	if !ok {
		return
	}
	if rfq.UserId == userId {
		common.ApiErrorMsg(c, "you cannot bid on your own request")
		return
	}
	if !isComputeMarketAdmin(c) {
		supplier, err := model.GetComputeSupplierByUserId(userId)
		if err != nil || supplier.Level < model.ComputeSupplierLevelVerified {
			common.ApiErrorMsg(c, "complete supplier verification before bidding")
			return
		}
	}
	var req computeBidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.PricePerGpuHour <= 0 {
		common.ApiErrorMsg(c, "please enter a price per GPU-hour")
		return
	}
	if req.DeliverDate != "" && !reISODateOnly.MatchString(req.DeliverDate) {
		common.ApiErrorMsg(c, "deliver date must be YYYY-MM-DD")
		return
	}
	if req.SlaPct < 0 || req.SlaPct > 100 {
		common.ApiErrorMsg(c, "SLA must be a percentage")
		return
	}
	eligible := true
	for _, v := range req.HardReqs {
		if !v {
			eligible = false
		}
	}
	hardReqsJSON := "{}"
	if req.HardReqs != nil {
		if raw, err := json.Marshal(req.HardReqs); err == nil && len(raw) <= 4000 {
			hardReqsJSON = string(raw)
		}
	}
	deviations := scrubContact(req.Deviations)
	if len(deviations) > 500 {
		deviations = deviations[:500]
	}
	bid, err := model.PlaceComputeBid(rfq, userId, req.PricePerGpuHour, req.DeliverDate, req.SlaPct, hardReqsJSON, deviations, eligible)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf("bid $%.2f/GPU-hr on compute request %s", bid.PricePerGpuHour, rfq.Code))
	common.ApiSuccess(c, gin.H{"bid": bid, "matching_deadline": rfq.MatchingDeadline})
}

// WithdrawComputeBid withdraws the caller's live bid.
func WithdrawComputeBid(c *gin.Context) {
	userId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid bid id")
		return
	}
	if err := model.WithdrawComputeBid(id, userId); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, gin.H{"id": id, "status": model.ComputeBidStatusWithdrawn})
}

// AcceptComputeBid lets the buyer match with a bid.
func AcceptComputeBid(c *gin.Context) {
	userId := c.GetInt("id")
	rfq, ok := loadRFQ(c)
	if !ok {
		return
	}
	var req struct {
		BidId int `json:"bid_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	bid, err := model.AcceptComputeBid(rfq, req.BidId, userId)
	if err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	model.RecordLog(userId, model.LogTypeSystem, fmt.Sprintf("matched compute request %s with %s at $%.2f/GPU-hr", rfq.Code, bid.SupplierAlias, bid.PricePerGpuHour))
	model.RecordLog(bid.SupplierUserId, model.LogTypeSystem, fmt.Sprintf("your bid on compute request %s was accepted at $%.2f/GPU-hr", rfq.Code, bid.PricePerGpuHour))
	common.ApiSuccess(c, gin.H{"rfq": rfq, "accepted_bid": bid})
}

// ExtendComputeRFQ adds 24h of matching.
func ExtendComputeRFQ(c *gin.Context) {
	rfq, ok := loadRFQ(c)
	if !ok {
		return
	}
	if err := model.ExtendComputeRFQ(rfq, c.GetInt("id")); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, gin.H{"rfq": rfq})
}

// RaiseComputeRFQCeiling raises the public ceiling to attract more bids.
func RaiseComputeRFQCeiling(c *gin.Context) {
	rfq, ok := loadRFQ(c)
	if !ok {
		return
	}
	var req struct {
		PriceCeiling float64 `json:"price_ceiling"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.PriceCeiling > 1000 {
		common.ApiErrorMsg(c, "price ceiling is out of range")
		return
	}
	if err := model.RaiseComputeRFQCeiling(rfq, c.GetInt("id"), req.PriceCeiling); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, gin.H{"rfq": rfq})
}

// CancelComputeRFQ withdraws an unmatched request.
func CancelComputeRFQ(c *gin.Context) {
	rfq, ok := loadRFQ(c)
	if !ok {
		return
	}
	if err := model.CancelComputeRFQ(rfq, c.GetInt("id")); err != nil {
		common.ApiErrorMsg(c, err.Error())
		return
	}
	common.ApiSuccess(c, gin.H{"rfq": rfq})
}

// ---- Supplier profile ----

type computeSupplierRequest struct {
	Company   string   `json:"company"`
	Contact   string   `json:"contact"`
	Regions   []string `json:"regions"`
	Inventory string   `json:"inventory"`
}

// GetMyComputeSupplier returns the caller's supplier profile (or null).
func GetMyComputeSupplier(c *gin.Context) {
	userId := c.GetInt("id")
	s, err := model.GetComputeSupplierByUserId(userId)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiSuccess(c, gin.H{"supplier": nil, "alias": model.ComputeMarketAlias("S", userId), "can_bid": isComputeMarketAdmin(c)})
			return
		}
		common.ApiError(c, err)
		return
	}
	bids, err := model.ListComputeBidsBySupplier(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"supplier": s,
		"alias":    model.ComputeMarketAlias("S", userId),
		"can_bid":  s.Level >= model.ComputeSupplierLevelVerified || isComputeMarketAdmin(c),
		"bids":     bids,
	})
}

// UpsertMyComputeSupplier registers or updates the caller's supplier profile.
func UpsertMyComputeSupplier(c *gin.Context) {
	userId := c.GetInt("id")
	var req computeSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	req.Company = strings.TrimSpace(req.Company)
	if req.Company == "" || len(req.Company) > 191 {
		common.ApiErrorMsg(c, "company name is required")
		return
	}
	if len(req.Contact) > 191 {
		common.ApiErrorMsg(c, "contact is too long")
		return
	}
	if len(req.Regions) == 0 {
		common.ApiErrorMsg(c, "please list the regions where you have capacity")
		return
	}
	if len(req.Inventory) > 4000 {
		req.Inventory = req.Inventory[:4000]
	}
	s, err := model.UpsertComputeSupplier(userId, req.Company, strings.TrimSpace(req.Contact), joinList(req.Regions, 12), strings.TrimSpace(req.Inventory))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"supplier": s, "alias": model.ComputeMarketAlias("S", userId), "can_bid": s.Level >= model.ComputeSupplierLevelVerified || isComputeMarketAdmin(c)})
}

// ---- Admin ----

// ListComputeSuppliers (AdminAuth) lists supplier profiles for verification.
func ListComputeSuppliers(c *gin.Context) {
	items, err := model.ListComputeSuppliers()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": items})
}

// SetComputeSupplierLevel (AdminAuth) sets a supplier's verification level.
func SetComputeSupplierLevel(c *gin.Context) {
	userId, err := strconv.Atoi(c.Param("user_id"))
	if err != nil {
		common.ApiErrorMsg(c, "invalid user id")
		return
	}
	var req struct {
		Level int `json:"level"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.SetComputeSupplierLevel(userId, req.Level); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "supplier not found")
			return
		}
		common.ApiErrorMsg(c, err.Error())
		return
	}
	model.RecordLog(c.GetInt("id"), model.LogTypeManage, fmt.Sprintf("set compute supplier level of user %d to %d", userId, req.Level))
	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": gin.H{"user_id": userId, "level": req.Level}})
}
