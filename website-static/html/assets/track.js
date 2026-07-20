/* flatkey conversion events -> dataLayer (GTM maps these to GA4 / Google Ads) */
(function () {
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
