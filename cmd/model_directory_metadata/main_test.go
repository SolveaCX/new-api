package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestParseCommandOptionsRequiresExplicitSQLDSN(t *testing.T) {
	_, err := parseCommandOptions([]string{"--file", "metadata.json", "--dry-run"}, func(string) string { return "" })
	require.ErrorContains(t, err, "SQL_DSN")
}

func TestParseCommandOptionsRequiresExactlyOneMode(t *testing.T) {
	lookupEnv := func(key string) string {
		if key == "SQL_DSN" {
			return "local"
		}
		return ""
	}

	_, err := parseCommandOptions([]string{"--file", "metadata.json"}, lookupEnv)
	require.ErrorContains(t, err, "exactly one")
	_, err = parseCommandOptions([]string{"--file", "metadata.json", "--dry-run", "--apply"}, lookupEnv)
	require.ErrorContains(t, err, "exactly one")
}

func TestParseCommandOptionsRequiresFile(t *testing.T) {
	_, err := parseCommandOptions([]string{"--apply"}, func(string) string { return "local" })
	require.ErrorContains(t, err, "file")
}

func TestDryRunUsesDatabaseConnectionWithoutMigration(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "dry-run.db")
	metadataPath := filepath.Join(t.TempDir(), "metadata.json")
	require.NoError(t, os.WriteFile(metadataPath, []byte(`[{"model_name":"gpt-5","author":"OpenAI","providers":["OpenAI"],"modalities":["text","image"],"output_modalities":["text"],"context_tokens":128000,"series":"GPT","categories":["coding"],"released_at":"2026-08-01","distillable":true,"reasoning":true}]`), 0600))
	t.Setenv("SQL_DSN", "local")
	t.Setenv("SQLITE_PATH", databasePath)
	t.Setenv("NODE_TYPE", "")
	setupDB, err := gorm.Open(sqlite.Open(databasePath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, setupDB.AutoMigrate(&model.ModelDirectoryMetadata{}))
	setupSQLDB, err := setupDB.DB()
	require.NoError(t, err)
	require.NoError(t, setupSQLDB.Close())

	originalDB := model.DB
	originalSQLitePath := common.SQLitePath
	t.Cleanup(func() {
		if model.DB != nil {
			if sqlDB, err := model.DB.DB(); err == nil {
				_ = sqlDB.Close()
			}
		}
		model.DB = originalDB
		common.SQLitePath = originalSQLitePath
	})

	var output strings.Builder
	err = run([]string{"--file", metadataPath, "--dry-run"}, os.Getenv, &output)
	require.NoError(t, err)
	require.NotNil(t, model.DB)
	require.True(t, model.DB.Migrator().HasTable(&model.ModelDirectoryMetadata{}))
	require.False(t, model.DB.Migrator().HasTable(&model.User{}))
}

func TestDecodeImportFileMapsOutputModalitiesAndReasoning(t *testing.T) {
	metadataPath := filepath.Join(t.TempDir(), "metadata.json")
	require.NoError(t, os.WriteFile(metadataPath, []byte(`[{"model_name":"gpt-5","author":"OpenAI","providers":["OpenAI"],"modalities":["text","image"],"output_modalities":["text"],"context_tokens":128000,"series":"GPT","categories":["coding"],"released_at":"2026-08-01","distillable":true,"reasoning":true}]`), 0600))

	rows, err := decodeImportFile(metadataPath)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.JSONEq(t, `["text","image"]`, rows[0].ModalitiesJSON)
	require.JSONEq(t, `["text"]`, rows[0].OutputModalitiesJSON)
	require.True(t, rows[0].Reasoning)
}

func TestDecodeImportFileRequiresExplicitReasoning(t *testing.T) {
	metadataPath := filepath.Join(t.TempDir(), "metadata.json")
	require.NoError(t, os.WriteFile(metadataPath, []byte(`[{"model_name":"gpt-5","author":"OpenAI","providers":["OpenAI"],"modalities":["text"],"output_modalities":["text"],"context_tokens":128000,"series":"GPT","categories":["coding"],"released_at":"2026-08-01","distillable":true}]`), 0600))

	_, err := decodeImportFile(metadataPath)
	require.ErrorContains(t, err, "reasoning is required")
}

func TestReviewedImportFilesCarryOutputModalitiesAndReasoning(t *testing.T) {
	for _, path := range []string{
		filepath.Join("..", "..", "data", "model-directory", "metadata.json"),
		filepath.Join("..", "..", "data", "model-directory", "production-candidate.json"),
	} {
		t.Run(filepath.Base(path), func(t *testing.T) {
			rows, err := decodeImportFile(path)
			require.NoError(t, err)
			require.NotEmpty(t, rows)
			for index := range rows {
				require.NoError(t, rows[index].NormalizeAndValidate(), "row %d %s", index+1, rows[index].ModelName)
			}
		})
	}
}
