/* flatkey conversion events -> dataLayer (GTM maps these to GA4 / Google Ads) */
(function () {
  /* Preserve paid acquisition across flatkey.ai -> console.flatkey.ai OAuth. */
  function captureAttribution() {
    try {
      var keep = { aff: 1, fbclid: 1, gad_campaignid: 1, gad_source: 1, gbraid: 1, gclid: 1, lng: 1, msclkid: 1, ttclid: 1, wbraid: 1, yclid: 1 };
      var params = new URLSearchParams(location.search || "");
      var values = {};
      params.forEach(function (value, key) {
        if (value && (keep[key] || key.indexOf("utm_") === 0 || key.indexOf("hsa_") === 0)) values[key] = value;
      });
      if (!Object.keys(values).length) return;

      var cookieName = "flatkey_ads_attribution";
      var prefix = cookieName + "=";
      var part = (document.cookie || "").split(";").map(function (value) { return value.trim(); }).find(function (value) { return value.indexOf(prefix) === 0; });
      var existing = {};
      if (part) {
        try { existing = JSON.parse(decodeURIComponent(part.slice(prefix.length))) || {}; } catch (_) { existing = {}; }
      }

      function acquisitionPath(path) {
        return path && path.charAt(0) === "/" && path.indexOf("/oauth/") !== 0 && path !== "/sign-in" && path.indexOf("/sign-in/") !== 0 && path !== "/sign-up" && path.indexOf("/sign-up/") !== 0;
      }
      var path = location.pathname || "/";
      var previous = existing.first_landing_path || existing.landing_path || "";
      var first = acquisitionPath(previous) ? previous : (acquisitionPath(path) ? path : "");
      var now = new Date().toISOString();
      values.landing_path = acquisitionPath(path) ? path : (existing.landing_path || "");
      values.captured_at = now;
      if (first) {
        values.first_landing_path = first;
        values.first_captured_at = existing.first_captured_at || existing.captured_at || now;
      }
      if (document.referrer && document.referrer.indexOf(location.origin + "/") !== 0) values.referrer = document.referrer;

      var attributes = ["path=/", "max-age=7776000", "SameSite=Lax"];
      if (location.hostname === "flatkey.ai" || location.hostname.endsWith(".flatkey.ai")) attributes.push("domain=.flatkey.ai");
      if (location.protocol === "https:") attributes.push("Secure");
      document.cookie = cookieName + "=" + encodeURIComponent(JSON.stringify(values)) + "; " + attributes.join("; ");
    } catch (_) {}
  }
  captureAttribution();

  window.dataLayer = window.dataLayer || [];
  function push(ev, params) { var o = { event: ev, page: location.pathname }; for (var k in params) o[k] = params[k]; window.dataLayer.push(o); }
  document.addEventListener("click", function (e) {
    var a = e.target.closest && e.target.closest("a"); if (!a) return;
    var h = a.getAttribute("href") || "";
    if (h.indexOf("console.flatkey.ai/sign-up") >= 0) push("sign_up_click", { cta_text: (a.textContent || "").trim().slice(0, 40) });
    else if (h.indexOf("login.html") >= 0) push("login_cta_click", { cta_text: (a.textContent || "").trim().slice(0, 40) });
    else if (h.indexOf("discord.gg") >= 0) push("discord_click", {});
    else if (h.indexOf("mailto:") === 0) push("email_click", {});
    else if (h.indexOf("playground.html") >= 0) push("playground_click", {});
  }, true);
  var f = document.querySelector('form[action*="formsubmit"]');
  if (f) f.addEventListener("submit", function () { push("contact_form_submit", {}); });
  if (location.search.indexOf("sent=1") >= 0) push("contact_form_success", {});
})();
