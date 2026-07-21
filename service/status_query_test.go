package service

import (
	"fmt"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestListStatusIncidentRecordsUsesConstantQueryCount(t *testing.T) {
	for _, testCase := range []struct {
		name          string
		incidentCount int
		expectedCount int
	}{
		{name: "empty", incidentCount: 0, expectedCount: 1},
		{name: "single", incidentCount: 1, expectedCount: 3},
		{name: "many", incidentCount: 8, expectedCount: 3},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db := setupStatusServiceTestDB(t)
			for index := 0; index < testCase.incidentCount; index++ {
				incident := model.StatusIncident{
					PublicID: fmt.Sprintf("inc_query_%02d", index), Kind: model.StatusIncidentKindIncident,
					Title: "Query count incident", Impact: model.StatusDegraded, Status: "monitoring",
					Visibility: "public", AutomationMode: "manual", IdempotencyKey: fmt.Sprintf("query-count-%02d", index),
					Version: 1, CreatedAt: int64(index + 1), UpdatedAt: int64(index + 1),
				}
				require.NoError(t, db.Create(&incident).Error)
				require.NoError(t, db.Create(&model.StatusIncidentUpdate{
					IncidentID: incident.ID, EventID: fmt.Sprintf("evt_%02d", index), State: "monitoring",
					Body: "Monitoring recovery.", Published: true, PublishedAt: int64(index + 1), CreatedAt: int64(index + 1),
				}).Error)
				for _, componentID := range []int64{int64(index*2 + 2), int64(index*2 + 1)} {
					require.NoError(t, db.Create(&model.StatusIncidentComponent{IncidentID: incident.ID, ComponentID: componentID}).Error)
				}
			}

			queryCount := 0
			callbackName := "test:count_status_incident_record_queries"
			require.NoError(t, db.Callback().Query().Before("gorm:query").Register(callbackName, func(*gorm.DB) {
				queryCount++
			}))

			records, err := ListStatusIncidentRecords(model.StatusIncidentKindIncident, true, 100)
			require.NoError(t, err)
			require.Len(t, records, testCase.incidentCount)
			require.Equal(t, testCase.expectedCount, queryCount)
			for _, record := range records {
				require.Len(t, record.Updates, 1)
				require.Len(t, record.ComponentIDs, 2)
				require.Less(t, record.ComponentIDs[0], record.ComponentIDs[1])
			}
		})
	}
}
