package model

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/model_setting"
	"github.com/alicebob/miniredis/v2"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupAutoModelConcurrentSQLiteTestDB(t *testing.T) {
	t.Helper()
	originalDB := DB
	originalRedisEnabled := common.RedisEnabled
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	common.OptionMapRWMutex.Lock()
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}
	common.OptionMapRWMutex.Unlock()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared&_pragma=busy_timeout(5000)", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Option{}, &Channel{}, &Ability{}))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(4)
	DB = db
	common.RedisEnabled = false
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false

	t.Cleanup(func() {
		DB = originalDB
		common.RedisEnabled = originalRedisEnabled
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.OptionMapRWMutex.Lock()
		common.OptionMap = originalOptionMap
		common.OptionMapRWMutex.Unlock()
		require.NoError(t, sqlDB.Close())
	})
}

func autoModelConfigRawForTest(t *testing.T, classifierModel string) string {
	t.Helper()
	cfg := model_setting.AutoModelConfig{
		Version:                 model_setting.AutoModelConfigVersion,
		Enabled:                 true,
		ClassifierBaseURL:       "https://classifier.example.com/v1",
		ClassifierModel:         classifierModel,
		ClassifierTimeoutMS:     800,
		ClassifierInputMaxChars: 8000,
		DefaultModel:            "model-a",
		Routes: map[string][]string{
			"general":     {"model-a", "model-b"},
			"coding":      {"model-c"},
			"reasoning":   {"model-d"},
			"translation": {"model-e"},
		},
	}
	raw, err := common.Marshal(cfg)
	require.NoError(t, err)
	return string(raw)
}

func autoModelDisabledConfigRawForTest(t *testing.T) string {
	t.Helper()
	cfg := model_setting.DefaultAutoModelConfig()
	cfg.Enabled = false
	raw, err := common.Marshal(cfg)
	require.NoError(t, err)
	return string(raw)
}

func readStoredAutoModelPair(t *testing.T) (model_setting.AutoModelConfig, model_setting.AutoModelCredential) {
	t.Helper()
	var options []Option
	require.NoError(t, DB.Where("key IN ?", []string{
		model_setting.AutoModelConfigOptionKey,
		model_setting.AutoModelClassifierAPIKeyOptionKey,
	}).Find(&options).Error)
	require.Len(t, options, 2)
	values := make(map[string]string, 2)
	for _, option := range options {
		values[option.Key] = option.Value
	}
	cfg, _, err := model_setting.NormalizeAutoModelConfig(values[model_setting.AutoModelConfigOptionKey])
	require.NoError(t, err)
	credential, err := model_setting.ParseAutoModelCredential(values[model_setting.AutoModelClassifierAPIKeyOptionKey])
	require.NoError(t, err)
	return cfg, credential
}

func TestAutoModelOptionsRequireDedicatedBulkUpdate(t *testing.T) {
	setupOptionGroupRenameTestDB(t)
	t.Cleanup(func() { require.NoError(t, model_setting.ReloadAutoModelSnapshot("", "")) })

	require.ErrorIs(t, UpdateOption(model_setting.AutoModelConfigOptionKey, autoModelConfigRawForTest(t, "router")), ErrAutoModelOptionsRequireBulkUpdate)
	require.ErrorIs(t, UpdateOption(model_setting.AutoModelClassifierAPIKeyOptionKey, "sk-secret"), ErrAutoModelOptionsRequireBulkUpdate)
	require.ErrorIs(t, UpdateOptionsBulk(map[string]string{
		model_setting.AutoModelConfigOptionKey: autoModelConfigRawForTest(t, "router"),
	}), ErrAutoModelOptionsRequireBulkUpdate)

	var count int64
	require.NoError(t, DB.Model(&Option{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestUpdateAutoModelOptionsRollsBackSeedRowsOnValidationFailure(t *testing.T) {
	setupOptionGroupRenameTestDB(t)
	t.Cleanup(func() { require.NoError(t, model_setting.ReloadAutoModelSnapshot("", "")) })

	err := UpdateAutoModelOptions(map[string]string{
		model_setting.AutoModelConfigOptionKey: autoModelConfigRawForTest(t, "router"),
	})
	require.ErrorContains(t, err, "API key is required")
	var count int64
	require.NoError(t, DB.Model(&Option{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestUpdateAutoModelOptionsSavesDisabledDraftWithoutCredential(t *testing.T) {
	setupOptionGroupRenameTestDB(t)
	t.Cleanup(func() { require.NoError(t, model_setting.ReloadAutoModelSnapshot("", "")) })

	require.NoError(t, UpdateAutoModelOptions(map[string]string{
		model_setting.AutoModelConfigOptionKey: autoModelDisabledConfigRawForTest(t),
	}))
	cfg, credential := readStoredAutoModelPair(t)
	require.False(t, cfg.Enabled)
	require.Empty(t, cfg.CredentialVersion)
	require.Empty(t, credential.Version)
	require.Empty(t, credential.APIKey)
	snapshot := model_setting.GetAutoModelSnapshot()
	require.False(t, snapshot.Config.Enabled)
	require.Empty(t, snapshot.ClassifierAPIKey)
}

func TestUpdateAutoModelOptionsDisablesWithoutLosingStoredCredential(t *testing.T) {
	setupOptionGroupRenameTestDB(t)
	t.Cleanup(func() { require.NoError(t, model_setting.ReloadAutoModelSnapshot("", "")) })

	require.NoError(t, UpdateAutoModelOptions(map[string]string{
		model_setting.AutoModelConfigOptionKey:           autoModelConfigRawForTest(t, "router"),
		model_setting.AutoModelClassifierAPIKeyOptionKey: "sk-preserved",
	}))
	require.NoError(t, UpdateAutoModelOptions(map[string]string{
		model_setting.AutoModelConfigOptionKey: autoModelDisabledConfigRawForTest(t),
	}))
	cfg, credential := readStoredAutoModelPair(t)
	require.False(t, cfg.Enabled)
	require.Equal(t, credential.Version, cfg.CredentialVersion)
	require.NotEmpty(t, cfg.CredentialVersion)
	require.Equal(t, "sk-preserved", credential.APIKey)
	require.Empty(t, model_setting.GetAutoModelSnapshot().ClassifierAPIKey)

	require.NoError(t, UpdateAutoModelOptions(map[string]string{
		model_setting.AutoModelConfigOptionKey: autoModelConfigRawForTest(t, "router-reenabled"),
	}))
	_, credential = readStoredAutoModelPair(t)
	require.Equal(t, "sk-preserved", credential.APIKey)
	require.Equal(t, "sk-preserved", model_setting.GetAutoModelSnapshot().ClassifierAPIKey)
}

func TestAutoModelRealNameConflictChecksStrictAndCachedViews(t *testing.T) {
	setupOptionGroupRenameTestDB(t)
	t.Cleanup(func() { require.NoError(t, model_setting.ReloadAutoModelSnapshot("", "")) })

	conflict, err := HasRealAutoModelConflict()
	require.NoError(t, err)
	require.False(t, conflict)

	require.NoError(t, DB.Create(&Channel{Id: 501, Key: "key", Name: "conflict", Models: "model-a, auto ,model-b", Status: common.ChannelStatusManuallyDisabled}).Error)
	conflict, err = HasRealAutoModelConflict()
	require.NoError(t, err)
	require.True(t, conflict, "strict save check includes channel model declarations even when inactive")
	err = UpdateAutoModelOptions(map[string]string{
		model_setting.AutoModelConfigOptionKey:           autoModelConfigRawForTest(t, "router"),
		model_setting.AutoModelClassifierAPIKeyOptionKey: "sk-secret",
	})
	require.ErrorContains(t, err, "existing real model named auto")

	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	channelSyncLock.Lock()
	originalConflict := hasRealAutoModelConflict
	hasRealAutoModelConflict = true
	channelSyncLock.Unlock()
	t.Cleanup(func() {
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		channelSyncLock.Lock()
		hasRealAutoModelConflict = originalConflict
		channelSyncLock.Unlock()
	})
	activeConflict, err := HasCachedRealAutoModelConflict()
	require.NoError(t, err)
	require.True(t, activeConflict)
	channelSyncLock.Lock()
	hasRealAutoModelConflict = false
	channelSyncLock.Unlock()
	activeConflict, err = HasCachedRealAutoModelConflict()
	require.NoError(t, err)
	require.False(t, activeConflict)
}

func TestInitChannelCacheIncludesDisabledAutoDeclarationsInConflictState(t *testing.T) {
	setupOptionGroupRenameTestDB(t)
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = originalMemoryCacheEnabled })

	require.NoError(t, DB.Create(&Channel{
		Id:     502,
		Key:    "key",
		Name:   "disabled-conflict",
		Models: "model-a,auto",
		Status: common.ChannelStatusManuallyDisabled,
	}).Error)
	InitChannelCache()
	conflict, err := HasCachedRealAutoModelConflict()
	require.NoError(t, err)
	require.True(t, conflict)
}

func TestUpdateAutoModelOptionsReusesOmittedEmptyAndMaskedKey(t *testing.T) {
	setupOptionGroupRenameTestDB(t)
	t.Cleanup(func() { require.NoError(t, model_setting.ReloadAutoModelSnapshot("", "")) })

	require.NoError(t, UpdateAutoModelOptions(map[string]string{
		model_setting.AutoModelConfigOptionKey:           autoModelConfigRawForTest(t, "router-one"),
		model_setting.AutoModelClassifierAPIKeyOptionKey: "sk-original",
	}))
	firstConfig, firstCredential := readStoredAutoModelPair(t)
	require.Equal(t, firstConfig.CredentialVersion, firstCredential.Version)
	require.Equal(t, "sk-original", firstCredential.APIKey)

	for _, keyValue := range []struct {
		name    string
		include bool
		value   string
	}{
		{name: "omitted"},
		{name: "empty", include: true, value: ""},
		{name: "masked", include: true, value: model_setting.AutoModelAPIKeyMask},
	} {
		t.Run(keyValue.name, func(t *testing.T) {
			values := map[string]string{
				model_setting.AutoModelConfigOptionKey: autoModelConfigRawForTest(t, "router-two"),
			}
			if keyValue.include {
				values[model_setting.AutoModelClassifierAPIKeyOptionKey] = keyValue.value
			}
			require.NoError(t, UpdateAutoModelOptions(values))
			cfg, credential := readStoredAutoModelPair(t)
			require.Equal(t, cfg.CredentialVersion, credential.Version)
			require.NotEqual(t, firstCredential.Version, credential.Version)
			require.Equal(t, "sk-original", credential.APIKey)
			require.NotEqual(t, model_setting.AutoModelAPIKeyMask, credential.APIKey)
			firstCredential = credential
		})
	}

	snapshot := model_setting.GetAutoModelSnapshot()
	require.Equal(t, "router-two", snapshot.Config.ClassifierModel)
	require.Equal(t, "sk-original", snapshot.ClassifierAPIKey)
	common.OptionMapRWMutex.RLock()
	require.Equal(t, snapshot.Config.CredentialVersion, firstCredential.Version)
	require.Contains(t, common.OptionMap[model_setting.AutoModelClassifierAPIKeyOptionKey], "sk-original")
	common.OptionMapRWMutex.RUnlock()
}

func TestUpdateAutoModelOptionsConcurrentWritersPersistCompletePair(t *testing.T) {
	setupAutoModelConcurrentSQLiteTestDB(t)
	t.Cleanup(func() { require.NoError(t, model_setting.ReloadAutoModelSnapshot("", "")) })

	updates := []map[string]string{
		{
			model_setting.AutoModelConfigOptionKey:           autoModelConfigRawForTest(t, "router-one"),
			model_setting.AutoModelClassifierAPIKeyOptionKey: "sk-one",
		},
		{
			model_setting.AutoModelConfigOptionKey:           autoModelConfigRawForTest(t, "router-two"),
			model_setting.AutoModelClassifierAPIKeyOptionKey: "sk-two",
		},
	}
	var wg sync.WaitGroup
	errs := make(chan error, len(updates))
	for _, update := range updates {
		update := update
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs <- UpdateAutoModelOptions(update)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	cfg, credential := readStoredAutoModelPair(t)
	require.Equal(t, cfg.CredentialVersion, credential.Version)
	switch cfg.ClassifierModel {
	case "router-one":
		require.Equal(t, "sk-one", credential.APIKey)
	case "router-two":
		require.Equal(t, "sk-two", credential.APIKey)
	default:
		t.Fatalf("unexpected classifier model %q", cfg.ClassifierModel)
	}
	snapshot := model_setting.GetAutoModelSnapshot()
	require.Equal(t, cfg.CredentialVersion, snapshot.Config.CredentialVersion)
	require.Equal(t, credential.APIKey, snapshot.ClassifierAPIKey)
}

func TestLoadOptionsFromDatabasePublishesCompletePeerSnapshot(t *testing.T) {
	setupOptionGroupRenameTestDB(t)
	t.Cleanup(func() { require.NoError(t, model_setting.ReloadAutoModelSnapshot("", "")) })

	require.NoError(t, UpdateAutoModelOptions(map[string]string{
		model_setting.AutoModelConfigOptionKey:           autoModelConfigRawForTest(t, "router-local"),
		model_setting.AutoModelClassifierAPIKeyOptionKey: "sk-local",
	}))

	cfg, _, err := model_setting.NormalizeAutoModelConfig(autoModelConfigRawForTest(t, "router-peer"))
	require.NoError(t, err)
	cfg.CredentialVersion = "peer-version"
	configRaw, err := common.Marshal(cfg)
	require.NoError(t, err)
	credentialRaw, err := model_setting.MarshalAutoModelCredential(model_setting.AutoModelCredential{
		Version: "peer-version",
		APIKey:  "sk-peer",
	})
	require.NoError(t, err)
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&Option{}).Where("key = ?", model_setting.AutoModelConfigOptionKey).Update("value", string(configRaw)).Error; err != nil {
			return err
		}
		return tx.Model(&Option{}).Where("key = ?", model_setting.AutoModelClassifierAPIKeyOptionKey).Update("value", credentialRaw).Error
	}))

	LoadOptionsFromDatabase()
	snapshot := model_setting.GetAutoModelSnapshot()
	require.Equal(t, "router-peer", snapshot.Config.ClassifierModel)
	require.Equal(t, "peer-version", snapshot.Config.CredentialVersion)
	require.Equal(t, "sk-peer", snapshot.ClassifierAPIKey)
}

func TestUpdateAutoModelOptionsPublishesPeerReloadNotification(t *testing.T) {
	setupOptionGroupRenameTestDB(t)
	t.Cleanup(func() { require.NoError(t, model_setting.ReloadAutoModelSnapshot("", "")) })
	mr := miniredis.RunT(t)
	previousRDB := common.RDB
	previousRedisEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	common.RedisEnabled = true
	t.Cleanup(func() {
		require.NoError(t, common.RDB.Close())
		common.RDB = previousRDB
		common.RedisEnabled = previousRedisEnabled
	})

	ctx := context.Background()
	sub := common.RDB.Subscribe(ctx, common.ConfigChangedChannel)
	defer sub.Close()
	_, err := sub.Receive(ctx)
	require.NoError(t, err)

	require.NoError(t, UpdateAutoModelOptions(map[string]string{
		model_setting.AutoModelConfigOptionKey:           autoModelConfigRawForTest(t, "router"),
		model_setting.AutoModelClassifierAPIKeyOptionKey: "sk-secret",
	}))
	select {
	case message := <-sub.Channel():
		require.True(t, strings.Contains(message.Payload, `"scope":"options"`), message.Payload)
	case <-time.After(2 * time.Second):
		t.Fatal("expected Auto Model option change notification")
	}
}
