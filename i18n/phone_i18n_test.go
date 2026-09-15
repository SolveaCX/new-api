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
