package i18n

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPhoneAlreadyRegisteredMessageIsLocalizedAcrossSupportedLanguages(t *testing.T) {
	require.NoError(t, Init())

	expected := map[string]string{
		LangEn:   "This phone number is already registered",
		LangZhCN: "该手机号已注册",
		LangZhTW: "此手機號碼已註冊",
		LangPt:   "Este número de telefone já está registrado",
		LangEs:   "Este número de teléfono ya está registrado",
		LangFr:   "Ce numéro de téléphone est déjà enregistré",
		LangRu:   "Этот номер телефона уже зарегистрирован",
		LangJa:   "この電話番号はすでに登録されています",
		LangVi:   "Số điện thoại này đã được đăng ký",
	}

	for lang, want := range expected {
		t.Run(lang, func(t *testing.T) {
			require.Equal(t, want, Translate(lang, MsgUserPhoneAlreadyRegistered))
		})
	}
}

func TestPhoneVerificationReminderForAPIIsLocalizedAcrossSupportedLanguages(t *testing.T) {
	require.NoError(t, Init())

	data := map[string]any{"SystemName": "Flatkey", "Link": "https://console.flatkey.ai/profile"}
	langs := []string{LangEn, LangZhCN, LangZhTW, LangPt, LangEs, LangFr, LangRu, LangJa, LangVi}
	seen := map[string]string{}
	for _, lang := range langs {
		t.Run(lang, func(t *testing.T) {
			out := Translate(lang, MsgNotifyPhoneVerificationReminderForAPI, data)
			require.NotEqual(t, MsgNotifyPhoneVerificationReminderForAPI, out)
			require.NotContains(t, out, "{{")
			require.NotContains(t, out, "<no value>")
			require.Contains(t, out, "Flatkey")
			require.Contains(t, out, "https://console.flatkey.ai/profile")
			for other, text := range seen {
				require.NotEqual(t, text, out, "%s and %s share the same text", lang, other)
			}
			seen[lang] = out
		})
	}
}
