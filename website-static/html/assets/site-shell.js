/* flatkey shared site shell — responsive navigation behavior */
(function () {
  var shell = document.querySelector(".nav") || document.querySelector(".dbar");
  if (!shell || document.querySelector(".nav-toggle")) return;

  var isDocs = shell.classList.contains("dbar");
  var links = isDocs
    ? shell.querySelectorAll(".dtabs a, .dext a")
    : shell.querySelectorAll(":scope > a:not(.logo)");
  if (!links.length) return;

  var panel = document.createElement("div");
  panel.className = "mobile-nav-panel";
  panel.id = "mobile-site-nav";
  panel.hidden = true;

  function syncPanel() {
    panel.innerHTML = "";
    links.forEach(function (link) {
      var copy = link.cloneNode(true);
      copy.removeAttribute("id");
      copy.classList.remove("on");
      copy.addEventListener("click", close);
      panel.appendChild(copy);
    });
  }
  syncPanel();

  var button = document.createElement("button");
  button.className = "nav-toggle";
  button.type = "button";
  button.setAttribute("aria-controls", panel.id);
  button.setAttribute("aria-expanded", "false");
  var menuLabels = {
    en: "Open navigation menu", zh: "打开导航菜单", es: "Abrir menú de navegación",
    fr: "Ouvrir le menu de navigation", pt: "Abrir menu de navegação",
    ru: "Открыть меню навигации", ja: "ナビゲーションメニューを開く",
    vi: "Mở menu điều hướng", de: "Navigationsmenü öffnen", id: "Buka menu navigasi"
  };
  function updateLabel() {
    var selector = document.querySelector(".langsel");
    var language = selector ? selector.value : (document.documentElement.lang || "en").split("-")[0];
    button.setAttribute("aria-label", menuLabels[language] || menuLabels.en);
  }
  updateLabel();
  var languageSelector = document.querySelector(".langsel");
  if (languageSelector) languageSelector.addEventListener("change", updateLabel);
  document.addEventListener("flatkey:languagechange", function () {
    updateLabel();
    syncPanel();
  });
  button.innerHTML = "<span></span><span></span><span></span>";

  function close() {
    panel.hidden = true;
    button.setAttribute("aria-expanded", "false");
    document.body.classList.remove("mobile-nav-open");
  }

  button.addEventListener("click", function () {
    var open = button.getAttribute("aria-expanded") !== "true";
    panel.hidden = !open;
    button.setAttribute("aria-expanded", String(open));
    document.body.classList.toggle("mobile-nav-open", open);
  });
  document.addEventListener("keydown", function (event) {
    if (event.key === "Escape") close();
  });

  shell.appendChild(button);
  shell.insertAdjacentElement("afterend", panel);
})();
