package controller

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/compute"
	"github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// Public demand intake for the compute market (no account needed).

var (
	computeLeadEmail = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]{2,}$`)
	computeLeadPhone = regexp.MustCompile(`^\+?[\d\s\-()]{7,20}$`)
	computeLeadGpus  = map[string]bool{"B300": true, "B200": true, "H200": true, "H100": true, "A100": true, "L40S": true, "RTX 5090": true, "RTX 4090": true, "M3 Ultra": true}
)

type computeDemandLeadRequest struct {
	Text         string   `json:"text"`
	Contact      string   `json:"contact"`
	Company      string   `json:"company"`
	GpuModel     string   `json:"gpu_model"`
	Gpus         int      `json:"gpus"`
	TermMonths   int      `json:"term_months"`
	Regions      []string `json:"regions"`
	StartDate    string   `json:"start_date"`
	PriceCeiling float64  `json:"price_ceiling"`
	Notes        string   `json:"notes"`
	Ref          string   `json:"ref"`
	Source       string   `json:"source"`
	Website      string   `json:"website"` // honeypot: must stay empty
}

func computeDemandShareURL(code string) string {
	base := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if base == "" || strings.Contains(base, "localhost") || strings.Contains(base, "127.0.0.1") {
		base = "https://flatkey.ai"
	}
	base = strings.Replace(base, "console.flatkey.ai", "flatkey.ai", 1)
	base = strings.Replace(base, "router.flatkey.ai", "flatkey.ai", 1)
	return base + "/compute?ref=" + code
}

func findComputeDemandPool(pools []*model.ComputeDemandPool, gpu string) *model.ComputeDemandPool {
	for _, p := range pools {
		if strings.EqualFold(p.GpuModel, gpu) {
			return p
		}
	}
	return &model.ComputeDemandPool{GpuModel: gpu}
}

// CreateComputeDemandLead accepts a lightweight request from the website.
func CreateComputeDemandLead(c *gin.Context) {
	var req computeDemandLeadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if strings.TrimSpace(req.Website) != "" {
		common.ApiSuccess(c, gin.H{"code": "OK"}) // bot honeypot: pretend success
		return
	}
	req.Contact = strings.TrimSpace(req.Contact)
	if len(req.Contact) < 5 || len(req.Contact) > 191 || !(computeLeadEmail.MatchString(req.Contact) || computeLeadPhone.MatchString(req.Contact) || len(req.Contact) >= 5) {
		common.ApiErrorMsg(c, "please leave an email, phone or WeChat so we can reach you")
		return
	}
	if strings.TrimSpace(req.Text) != "" && (req.GpuModel == "" || req.Gpus == 0) {
		draft := compute.ParseRFQText(c.Request.Context(), req.Text)
		if req.GpuModel == "" {
			req.GpuModel = draft.GpuModel
		}
		if req.Gpus == 0 && draft.Nodes > 0 {
			gpn := draft.GpusPerNode
			if gpn == 0 {
				gpn = 8
			}
			req.Gpus = draft.Nodes * gpn
		}
		if req.TermMonths == 0 {
			req.TermMonths = draft.TermMonths
		}
		if len(req.Regions) == 0 {
			req.Regions = draft.Regions
		}
		if req.StartDate == "" {
			req.StartDate = draft.StartDate
		}
		if req.PriceCeiling == 0 {
			req.PriceCeiling = draft.PriceCeiling
		}
		if req.Notes == "" {
			req.Notes = req.Text
		}
	}
	req.GpuModel = strings.TrimSpace(req.GpuModel)
	if !computeLeadGpus[req.GpuModel] {
		common.ApiErrorMsg(c, "please pick a GPU model")
		return
	}
	if req.Gpus <= 0 || req.Gpus > 100000 {
		common.ApiErrorMsg(c, "how many GPUs do you need?")
		return
	}
	if req.TermMonths <= 0 {
		req.TermMonths = 1
	}
	if req.TermMonths > 60 {
		req.TermMonths = 60
	}
	if req.StartDate != "" && !reISODateOnly.MatchString(req.StartDate) {
		req.StartDate = ""
	}
	ref := strings.ToUpper(strings.TrimSpace(req.Ref))
	if ref != "" {
		if _, err := model.GetComputeDemandLeadByCode(ref); err != nil {
			ref = ""
		}
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "website"
	}
	lead := &model.ComputeDemandLead{
		Contact: req.Contact, Company: strings.TrimSpace(req.Company), GpuModel: req.GpuModel, Gpus: req.Gpus, TermMonths: req.TermMonths,
		Regions: joinList(req.Regions, 8), StartDate: req.StartDate, PriceCeiling: req.PriceCeiling, Notes: scrubContact(req.Notes),
		Source: source, ReferredBy: ref, ClientIP: c.ClientIP(),
	}
	if len(lead.Notes) > 2000 {
		lead.Notes = lead.Notes[:2000]
	}
	if len(lead.Company) > 191 {
		lead.Company = lead.Company[:191]
	}
	if err := lead.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	go notifyComputeDemandLead(lead)
	pools, _ := model.GetComputeDemandPools()
	common.ApiSuccess(c, gin.H{
		"code":      lead.Code,
		"share_url": computeDemandShareURL(lead.Code),
		"pool":      findComputeDemandPool(pools, lead.GpuModel),
		"referrals": 0,
	})
}

func notifyComputeDemandLead(lead *model.ComputeDemandLead) {
	to := strings.TrimSpace(common.GetEnvOrDefaultString("COMPUTE_MARKET_OPS_EMAIL", "support@flatkey.ai"))
	if to == "" {
		return
	}
	subject := fmt.Sprintf("[Compute Market] new demand lead: %s × %d, %d mo", lead.GpuModel, lead.Gpus, lead.TermMonths)
	body := fmt.Sprintf("<p>New compute demand lead <b>%s</b></p><ul><li>Contact: %s</li><li>Company: %s</li><li>GPU: %s × %d</li><li>Term: %d months, start %s</li><li>Regions: %s</li><li>Ceiling: $%.2f</li><li>Referred by: %s</li><li>Source: %s</li></ul><p>%s</p>",
		lead.Code, lead.Contact, lead.Company, lead.GpuModel, lead.Gpus, lead.TermMonths, lead.StartDate, lead.Regions, lead.PriceCeiling, lead.ReferredBy, lead.Source, lead.Notes)
	if err := common.SendEmail(subject, to, body); err != nil {
		common.SysError("compute demand lead notification failed: " + err.Error())
	}
}

// GetComputeDemandPools returns the public per-GPU demand roll-up.
func GetComputeDemandPools(c *gin.Context) {
	pools, err := model.GetComputeDemandPools()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"pools": pools, "tiers": []gin.H{{"gpus": 64, "discount_pct": 5}, {"gpus": 256, "discount_pct": 10}, {"gpus": 1024, "discount_pct": 18}, {"gpus": 4096, "discount_pct": 25}}})
}

// GetComputeDemandLeadStatus returns the public status for a share code.
func GetComputeDemandLeadStatus(c *gin.Context) {
	lead, err := model.GetComputeDemandLeadByCode(c.Param("code"))
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			common.ApiErrorMsg(c, "unknown code")
			return
		}
		common.ApiError(c, err)
		return
	}
	refs, _ := model.CountComputeDemandReferrals(lead.Code)
	pools, _ := model.GetComputeDemandPools()
	common.ApiSuccess(c, gin.H{"code": lead.Code, "gpu_model": lead.GpuModel, "gpus": lead.Gpus, "term_months": lead.TermMonths,
		"share_url": computeDemandShareURL(lead.Code), "referrals": refs, "pool": findComputeDemandPool(pools, lead.GpuModel)})
}

// ListComputeDemandLeads (AdminAuth) lists leads with contact details.
func ListComputeDemandLeads(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	leads, err := model.ListComputeDemandLeads(limit)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"items": leads})
}
