package i18n

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type supplierAccountingLocalizedMessage struct {
	key  string
	en   string
	zhCN string
	zhTW string
	pt   string
}

func TestSupplierAccountingMessagesCoverCompleteBackendLocales(t *testing.T) {
	require.NoError(t, Init())
	tests := []supplierAccountingLocalizedMessage{
		{key: MsgSettingReservedOption, en: "This setting cannot be modified through the general settings API", zhCN: "该配置项不允许通过通用设置接口修改", zhTW: "此設定項不允許透過通用設定介面修改", pt: "Esta configuração não pode ser alterada pela API de configurações gerais"},
		{key: MsgSupplierAccountingNotReady, en: "supplier accounting is not ready", zhCN: "供应商核算尚未就绪", zhTW: "供應商核算尚未就緒", pt: "A contabilização de fornecedores ainda não está pronta"},
		{key: MsgSupplierAccountingInvalidRequest, en: "invalid supplier accounting request", zhCN: "无效的供应商核算请求", zhTW: "無效的供應商核算請求", pt: "Solicitação de contabilização de fornecedores inválida"},
		{key: MsgSupplierAccountingCommandFieldsRequired, en: "expected_state_version and reason are required", zhCN: "expected_state_version 和 reason 为必填项", zhTW: "expected_state_version 和 reason 為必填項", pt: "expected_state_version e reason são obrigatórios"},
		{key: MsgSupplierAccountingIdempotencyKeyRequired, en: "a valid Idempotency-Key is required", zhCN: "必须提供有效的 Idempotency-Key", zhTW: "必須提供有效的 Idempotency-Key", pt: "É necessário fornecer um Idempotency-Key válido"},
		{key: MsgSupplierAccountingIdempotencyConflict, en: "idempotency key payload conflict", zhCN: "幂等键对应的请求负载冲突", zhTW: "冪等鍵對應的請求內容衝突", pt: "Conflito entre a chave de idempotência e o conteúdo da solicitação"},
		{key: MsgSupplierAccountingVersionConflict, en: "supplier accounting version conflict", zhCN: "供应商核算版本冲突", zhTW: "供應商核算版本衝突", pt: "Conflito de versão na contabilização de fornecedores"},
		{key: MsgSupplierAccountingInvalidTransition, en: "supplier accounting transition is not allowed", zhCN: "不允许执行该供应商核算状态转换", zhTW: "不允許執行此供應商核算狀態轉換", pt: "A transição da contabilização de fornecedores não é permitida"},
		{key: MsgSupplierAccountingCoverageUnresolved, en: "supplier accounting coverage gaps remain unresolved", zhCN: "供应商核算覆盖缺口仍未解决", zhTW: "供應商核算覆蓋缺口仍未解決", pt: "Ainda existem lacunas não resolvidas na cobertura da contabilização de fornecedores"},
		{key: MsgSupplierAccountingCoverageGapNotFound, en: "supplier accounting coverage gap was not found", zhCN: "未找到供应商核算覆盖缺口", zhTW: "找不到供應商核算覆蓋缺口", pt: "A lacuna de cobertura da contabilização de fornecedores não foi encontrada"},
		{key: MsgSupplierAccountingStateMalformed, en: "supplier accounting state is malformed", zhCN: "供应商核算状态格式无效", zhTW: "供應商核算狀態格式無效", pt: "O estado da contabilização de fornecedores é inválido"},
		{key: MsgSupplierAccountingControlPlaneUnavailable, en: "supplier accounting control plane is unavailable", zhCN: "供应商核算控制面不可用", zhTW: "供應商核算控制面無法使用", pt: "O plano de controle da contabilização de fornecedores está indisponível"},
		{key: MsgSupplierAccountingMutationGateUnavailable, en: "supplier mutation gate is unavailable", zhCN: "供应商变更控制不可用", zhTW: "供應商變更控制無法使用", pt: "O controle de alterações de fornecedores está indisponível"},
		{key: MsgSupplierAccountingMutationsDisabled, en: "supplier mutations are disabled", zhCN: "供应商变更已被禁用", zhTW: "供應商變更已停用", pt: "As alterações de fornecedores estão desabilitadas"},
	}
	locales := []struct {
		language string
		value    func(supplierAccountingLocalizedMessage) string
	}{
		{language: LangEn, value: func(test supplierAccountingLocalizedMessage) string { return test.en }},
		{language: LangZhCN, value: func(test supplierAccountingLocalizedMessage) string { return test.zhCN }},
		{language: LangZhTW, value: func(test supplierAccountingLocalizedMessage) string { return test.zhTW }},
		{language: LangPt, value: func(test supplierAccountingLocalizedMessage) string { return test.pt }},
	}

	for _, testCase := range tests {
		for _, locale := range locales {
			t.Run(testCase.key+"/"+locale.language, func(t *testing.T) {
				require.Equal(t, locale.value(testCase), Translate(locale.language, testCase.key))
			})
		}
		require.Equal(t, testCase.en, Translate(LangEs, testCase.key), "partial backend locales must fall back to English")
	}
}
