/* Set theme and language before the first paint to avoid a light flash in
   dark mode or an English flash on Chinese systems. Kept tiny and dependency
   free because it runs before the app bundle. */
(function () {
  try {
    var theme = localStorage.getItem("focusd.theme");
    if (theme !== "dark" && theme !== "light") {
      theme = window.matchMedia && window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    }
    document.documentElement.dataset.theme = theme;

    var lang = localStorage.getItem("focusd.lang");
    if (lang !== "zh" && lang !== "en") {
      lang = ((navigator.language || "").toLowerCase().indexOf("zh") === 0) ? "zh" : "en";
    }
    document.documentElement.lang = lang === "zh" ? "zh-CN" : "en";
  } catch (err) {
    /* Private mode or a locked-down webview: the app applies defaults itself. */
  }
})();
