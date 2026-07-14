package model

// AdsSpendDaily holds one row per ads-account day: total paid-ads spend and
// clicks across all campaigns. Dates are the ads account's timezone (US
// Pacific), which matches the ops report's Pacific day bucketing, so rows join
// the daily registration funnel by date string directly.
//
// Rows are upserted from outside the app (the ops machine pushes Google Ads
// daily totals after each sync — scripts/gads_push_prod.py in the flatkey ops
// repo); the Go app only reads them. Read-only here, so no multi-node
// coordination is needed.
type AdsSpendDaily struct {
	Date        string  `json:"date" gorm:"column:date;primaryKey;size:16"`
	CostUSD     float64 `json:"cost_usd" gorm:"column:cost_usd"`
	Clicks      int     `json:"clicks" gorm:"column:clicks"`
	Impressions int     `json:"impressions" gorm:"column:impressions"`
	Conversions float64 `json:"conversions" gorm:"column:conversions"`
	UpdatedAt   int64   `json:"updated_at" gorm:"column:updated_at"`
}

func (AdsSpendDaily) TableName() string {
	return "ads_spend_daily"
}

// GetOpsAdsSpendDaily returns ads spend rows for dates >= sinceDate
// (YYYY-MM-DD; lexicographic order equals date order).
func GetOpsAdsSpendDaily(sinceDate string) ([]*AdsSpendDaily, error) {
	var rows []*AdsSpendDaily
	err := DB.Where("date >= ?", sinceDate).Find(&rows).Error
	return rows, err
}
