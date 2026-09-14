import type { Locale } from "./locales";

type PhoneBindingCopy = {
  title: string;
  description: string;
  action: string;
  later: string;
  close: string;
};

export const phoneBindingCopy: Record<Locale, PhoneBindingCopy> = {
  en: {
    title: "Link your phone number",
    description:
      "Verify a phone number to keep your account secure and continue using Flatkey.",
    action: "Link phone number",
    later: "Maybe later",
    close: "Close",
  },
  zh: {
    title: "绑定手机号",
    description: "验证手机号以保障账号安全，并继续使用 Flatkey。",
    action: "去绑定手机号",
    later: "暂时跳过",
    close: "关闭",
  },
  es: {
    title: "Vincula tu número de teléfono",
    description:
      "Verifica un número de teléfono para proteger tu cuenta y seguir usando Flatkey.",
    action: "Vincular teléfono",
    later: "Más tarde",
    close: "Cerrar",
  },
  fr: {
    title: "Associez votre numéro de téléphone",
    description:
      "Vérifiez un numéro de téléphone pour sécuriser votre compte et continuer à utiliser Flatkey.",
    action: "Associer le téléphone",
    later: "Plus tard",
    close: "Fermer",
  },
  pt: {
    title: "Vincule seu número de telefone",
    description:
      "Verifique um número de telefone para proteger sua conta e continuar usando o Flatkey.",
    action: "Vincular telefone",
    later: "Mais tarde",
    close: "Fechar",
  },
  ru: {
    title: "Привяжите номер телефона",
    description:
      "Подтвердите номер телефона, чтобы защитить аккаунт и продолжить пользоваться Flatkey.",
    action: "Привязать телефон",
    later: "Позже",
    close: "Закрыть",
  },
  ja: {
    title: "電話番号を登録",
    description:
      "電話番号を認証してアカウントを保護し、Flatkey の利用を続けてください。",
    action: "電話番号を登録",
    later: "後で",
    close: "閉じる",
  },
  vi: {
    title: "Liên kết số điện thoại",
    description:
      "Xác minh số điện thoại để bảo vệ tài khoản và tiếp tục sử dụng Flatkey.",
    action: "Liên kết số điện thoại",
    later: "Để sau",
    close: "Đóng",
  },
  de: {
    title: "Telefonnummer verknüpfen",
    description:
      "Bestätige eine Telefonnummer, um dein Konto zu schützen und Flatkey weiter zu nutzen.",
    action: "Telefonnummer verknüpfen",
    later: "Später",
    close: "Schließen",
  },
  id: {
    title: "Tautkan nomor telepon",
    description:
      "Verifikasi nomor telepon untuk melindungi akun dan terus menggunakan Flatkey.",
    action: "Tautkan nomor telepon",
    later: "Nanti",
    close: "Tutup",
  },
};
