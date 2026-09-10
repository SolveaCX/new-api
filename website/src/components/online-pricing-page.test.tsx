import { describe, expect, mock, test } from "bun:test";
import { readFileSync } from "node:fs";
import path from "node:path";
import { renderToStaticMarkup } from "react-dom/server";

mock.module("server-only", () => ({}));

describe("OnlinePricingPage", () => {
  test("renders subscribe and contact sales actions below the plan prices", async () => {
    const { OnlinePricingPage } = await import("./online-pricing-page");
    const html = renderToStaticMarkup(<OnlinePricingPage locale="en" />);

    const starterPrice = html.indexOf("<b>$10</b>");
    const proBadge = html.indexOf('<div class="tier hot"><div class="badge">MOST POPULAR</div>');
    const enterpriseCustom = html.indexOf(">Custom<");
    const starterCta = html.indexOf("Subscribe", starterPrice);
    const enterpriseCta = html.indexOf("Contact sales", enterpriseCustom);

    expect(starterCta).toBeGreaterThanOrEqual(0);
    expect(starterPrice).toBeGreaterThanOrEqual(0);
    expect(starterCta).toBeGreaterThan(starterPrice);
    expect(html).not.toMatch(/Short-term caps|短期上限|Límites a corto plazo|Limites court terme|Limites de curto prazo|Краткосрочные лимиты|Giới hạn ngắn hạn|Kurzfristige Limits|Batas jangka pendek/);
    expect(proBadge).toBeGreaterThanOrEqual(0);
    expect(html).toContain('<div class="tier limited-offer"><div class="badge limited">LIMITED</div>');
    expect(html).toContain("width:180px");
    expect(html).toContain("white-space:normal");
    expect(html).toContain("overflow-wrap:anywhere");
    expect(html).toContain('<div class="badge limited">LIMITED</div>');
    expect(html).toContain('<div class="tname">Starter</div>');
    expect(html).not.toContain('<div class="tier hot"><div class="badge">MOST POPULAR</div><div class="tname">Go</div>');
    expect(enterpriseCta).toBeGreaterThanOrEqual(0);
    expect(enterpriseCustom).toBeGreaterThanOrEqual(0);
    expect(enterpriseCta).toBeGreaterThan(enterpriseCustom);
    expect(html).toContain('<div class="tier enterprise">');
    expect(html).toContain('<div class="tprice"><b class="tcustom">Custom</b></div><div class="bonus" aria-hidden="true"></div><a class="btn black tcta"');
    expect(html).toContain('data-payment-method="pix"');
    expect(html).toContain('<span class="pm" data-payment-method="card"');
    expect(html).not.toContain('aria-pressed="true"');
    expect(html).not.toContain('class="pm on"');
    expect(html).not.toContain('<button class="pm"');
    expect(html).toContain('payment_method%3Dstripe_recurring');
    expect(html).toContain("/assets/logos/payment/pix.jpg");
    expect(html).toContain("/assets/logos/payment/upi.jpg");
    expect(html).toContain('src="/assets/logos/payment/alipay.svg"');
    expect(html).toContain("All models");
    expect(html).not.toContain('<del class="toldprice">$90</del>');
    expect(html).not.toContain('<del class="toldprice">$300</del>');
    expect(html).not.toContain("80% off");
    expect(html).not.toContain("70% off");
    expect(html).toContain('<div class="bonus">Top up $10, get $3 bonus</div>');
    expect(html).toContain('<div class="bonus">Top up $30, get $15 bonus</div>');
    expect(html).toContain('<div class="bonus">Top up $100, get $70 bonus</div>');
    expect(html).not.toContain("Text models");
    expect(html).not.toContain("B2B");
  });

  test("localizes payment method labels outside English", async () => {
    const { OnlinePricingPage } = await import("./online-pricing-page");
    const zhHtml = renderToStaticMarkup(<OnlinePricingPage locale="zh" />);
    const esHtml = renderToStaticMarkup(<OnlinePricingPage locale="es" />);

    expect(zhHtml).toContain("支付方式");
    expect(zhHtml).toContain("银行卡 · 3DS");
    expect(zhHtml).toContain("支付宝");
    expect(esHtml).toContain("Paga con");
    expect(esHtml).toContain("Tarjeta · 3DS");
    expect(esHtml).not.toContain("Pay with");
  });

  test("localizes pricing plan details outside English", async () => {
    const { OnlinePricingPage } = await import("./online-pricing-page");
    const localizedCases = [
      { locale: "zh", limited: "限时特惠", snippets: ["灵活定价", "适合个人与轻量日常使用", "全部模型", "定制", "/月"], legacyQuota: "每月最多 $25 模型用量" },
      { locale: "es", limited: "LIMITADO", snippets: ["Precios flexibles", "Para uso individual y diario ligero", "Todos los modelos", "Personalizado", "/mes"], legacyQuota: "Hasta $25 de uso de modelos / mes" },
      { locale: "fr", limited: "LIMITÉ", snippets: ["Tarifs flexibles", "Pour les particuliers", "Tous les modèles", "Sur mesure", "/mois"], legacyQuota: "Jusqu'à $25 d'utilisation de modèles / mois" },
      { locale: "pt", limited: "LIMITADO", snippets: ["Preços flexíveis", "Para uso individual", "Todos os modelos", "Personalizado", "/mês"], legacyQuota: "Até $25 de uso de modelos / mês" },
      { locale: "ru", limited: "ОГРАНИЧЕНО", snippets: ["Гибкие тарифы", "Для индивидуального", "Все модели", "Индивидуально", "/мес."], legacyQuota: "До $25 использования моделей / мес." },
      { locale: "ja", limited: "限定", snippets: ["柔軟な料金", "個人利用と軽い日常利用向け", "すべてのモデル", "カスタム", "/月"], legacyQuota: "月あたり最大 $25 のモデル利用" },
      { locale: "vi", limited: "GIỚI HẠN", snippets: ["Giá linh hoạt", "Cho cá nhân", "Tất cả model", "Tùy chỉnh", "/tháng"], legacyQuota: "Tối đa $25 mức sử dụng model / tháng" },
      { locale: "de", limited: "LIMITIERT", snippets: ["Flexible Preise", "Für Einzelpersonen", "Alle Modelle", "Individuell", "/Monat"], legacyQuota: "Bis zu $25 Modellnutzung / Monat" },
      { locale: "id", limited: "TERBATAS", snippets: ["Harga fleksibel", "Untuk individu", "Semua model", "Kustom", "/bulan"], legacyQuota: "Hingga $25 penggunaan model / bulan" },
    ] as const;

    for (const item of localizedCases) {
      const html = renderToStaticMarkup(<OnlinePricingPage locale={item.locale} />);
      for (const snippet of item.snippets) {
        expect(html).toContain(snippet);
      }
      expect(html).toContain(`<div class="badge limited">${item.limited}</div>`);
      expect(html).not.toContain("80% off");
      expect(html).not.toContain("70% off");
      for (const referencePrice of ["$45", "$90", "$300"]) {
        expect(html).not.toContain(`<del class="toldprice">${referencePrice}</del>`);
      }
      expect(html).toContain("$10");
      expect(html).toContain("$30");
      expect(html).toContain("$100");
      expect(html).not.toContain(item.legacyQuota);
      expect(html).not.toContain("For individuals & light daily use");
      expect(html).not.toMatch(/Short-term caps|短期上限|Límites a corto plazo|Limites court terme|Limites de curto prazo|Краткосрочные лимиты|Giới hạn ngắn hạn|Kurzfristige Limits|Batas jangka pendek/);
      expect(html).not.toContain("$450");
      expect(html).not.toContain(">Go<");
      expect(html).not.toContain("Text models");
      expect(html).not.toMatch(/media credits|media quota|media credit|crédit(?:s)? média|créditos multimedia|медиакредит|メディアクレジット|媒体额度|მედиа/i);
    }
  });

  test("does not render a separate media-credit balance in any locale", async () => {
    const { OnlinePricingPage } = await import("./online-pricing-page");
    for (const locale of ["en", "zh", "es", "fr", "pt", "ru", "ja", "vi", "de", "id"] as const) {
      const html = renderToStaticMarkup(<OnlinePricingPage locale={locale} />);
      expect(html).not.toMatch(/media credits|media quota|media credit|crédit(?:s)? média|créditos multimedia|медиакредит|メディアクレジット|媒体额度/i);
    }
  });

  test("localizes the top-up bonus shown on every plan card", async () => {
    const { OnlinePricingPage } = await import("./online-pricing-page");
    const cases = {
      en: ["Top up $10, get $3 bonus", "Top up $30, get $15 bonus", "Top up $100, get $70 bonus"],
      zh: ["充 $10 送 $3", "充 $30 送 $15", "充 $100 送 $70"],
      es: ["Recarga $10 y recibe $3 de bono", "Recarga $30 y recibe $15 de bono", "Recarga $100 y recibe $70 de bono"],
      fr: ["Rechargez 10 $ et recevez 3 $ de bonus", "Rechargez 30 $ et recevez 15 $ de bonus", "Rechargez 100 $ et recevez 70 $ de bonus"],
      pt: ["Recarregue $10 e ganhe $3 de bônus", "Recarregue $30 e ganhe $15 de bônus", "Recarregue $100 e ganhe $70 de bônus"],
      ru: ["Пополните на $10 и получите бонус $3", "Пополните на $30 и получите бонус $15", "Пополните на $100 и получите бонус $70"],
      ja: ["$10 をチャージすると $3 ボーナス", "$30 をチャージすると $15 ボーナス", "$100 をチャージすると $70 ボーナス"],
      vi: ["Nạp $10, nhận thêm $3", "Nạp $30, nhận thêm $15", "Nạp $100, nhận thêm $70"],
      de: ["$10 aufladen und $3 Bonus erhalten", "$30 aufladen und $15 Bonus erhalten", "$100 aufladen und $70 Bonus erhalten"],
      id: ["Top up $10, dapat bonus $3", "Top up $30, dapat bonus $15", "Top up $100, dapat bonus $70"],
    } as const;

    for (const [locale, bonuses] of Object.entries(cases)) {
      const html = renderToStaticMarkup(
        <OnlinePricingPage locale={locale as keyof typeof cases} />,
      );
      for (const bonus of bonuses) {
        expect(html).toContain(`<div class="bonus">${bonus}</div>`);
      }
    }
  });

  test("does not publish legacy media-credit pricing copy", () => {
    const publicCopy = readFileSync(path.join(process.cwd(), "public", "assets", "i18n.js"), "utf8");
    const offerCopy = readFileSync(path.join(process.cwd(), "src", "components", "lp-limited-offer-modal.tsx"), "utf8");

    const legacyMediaCreditCopy = /media credits?|image (?:and video )?credits?|video credits?|媒体额度|媒体点数|图像与视频额度|créditos? (?:de )?imagen(?: y v[ií]deo)?|crédits? image(?: et vidéo)?|кредиты? на изображения(?: и видео)?|画像・動画クレジット|hạn mức ảnh và video|Bild- und Video-Credits/i;

    expect(publicCopy).not.toMatch(legacyMediaCreditCopy);
    expect(offerCopy).not.toMatch(legacyMediaCreditCopy);
    expect(publicCopy).not.toMatch(/"tu\.(?:mc|mv)[123]"/);
  });

  test("localizes only payable prices for Portuguese and Japanese", async () => {
    const { OnlinePricingPage } = await import("./online-pricing-page");
    const cases = [
      {
        locale: "pt",
        prices: ["R$ 49,90", "R$ 149,90", "R$ 499"],
        cta: "Assine Pro por R$ 149,90/mês e entre",
      },
      {
        locale: "ja",
        prices: ["¥1,500", "¥4,500", "¥15,000"],
        cta: "Pro を ¥4,500/月で登録してログイン",
      },
    ] as const;

    for (const item of cases) {
      const html = renderToStaticMarkup(<OnlinePricingPage locale={item.locale} />);
      for (const price of item.prices) {
        expect(html).toContain(`<b>${price}</b>`);
      }
      expect(html).toContain(item.cta);
      expect(html).not.toContain('<del class="toldprice">$45</del>');
      expect(html).not.toContain('<del class="toldprice">$90</del>');
      expect(html).not.toContain('<del class="toldprice">$300</del>');
      if (item.locale === "pt") expect(html).not.toContain("<b>R$ 499,90</b>");
      expect(html).not.toContain("<b>$10</b>");
      expect(html).not.toContain("<b>$30</b>");
      expect(html).not.toContain("<b>$100</b>");
    }
  });
});
