// Small progressive enhancements. Every page works without JavaScript.
(function () {
  "use strict";

  // Mobile navigation drawer.
  function setupNav() {
    var toggle = document.querySelector("[data-nav-toggle]");
    if (!toggle) return;
    function setOpen(open) {
      document.body.classList.toggle("nav-open", open);
      toggle.setAttribute("aria-expanded", open ? "true" : "false");
    }
    toggle.addEventListener("click", function () {
      setOpen(!document.body.classList.contains("nav-open"));
    });
    document.querySelectorAll("[data-nav-close]").forEach(function (el) {
      el.addEventListener("click", function () { setOpen(false); });
    });
    document.addEventListener("keydown", function (e) {
      if (e.key === "Escape") setOpen(false);
    });
  }

  // Confirmation prompts for destructive or irreversible actions.
  function setupConfirm() {
    document.querySelectorAll("form[data-confirm]").forEach(function (form) {
      form.addEventListener("submit", function (e) {
        if (!window.confirm(form.getAttribute("data-confirm"))) {
          e.preventDefault();
          e.stopImmediatePropagation();
        }
      });
    });
  }

  // Forms marked data-loading disable their submit button and show a spinner
  // while the (possibly slow) AI request runs.
  function setupLoadingForms() {
    document.querySelectorAll("form[data-loading]").forEach(function (form) {
      form.addEventListener("submit", function (e) {
        if (e.defaultPrevented) return;
        if (form.classList.contains("is-loading")) {
          e.preventDefault();
          return;
        }
        setLoading(form, true);
      });
    });
    // Restore buttons when the page is shown from the back/forward cache.
    window.addEventListener("pageshow", function () {
      document.querySelectorAll("form.is-loading").forEach(function (f) { setLoading(f, false); });
    });
  }

  function setLoading(form, on) {
    form.classList.toggle("is-loading", on);
    form.querySelectorAll("button[type=submit]").forEach(function (btn) {
      if (on) {
        btn.dataset.label = btn.innerHTML;
        btn.disabled = true;
        btn.innerHTML = '<span class="spinner" aria-hidden="true"></span> ' +
          escapeHTML(form.getAttribute("data-loading") || "Working…");
      } else if (btn.dataset.label) {
        btn.disabled = false;
        btn.innerHTML = btn.dataset.label;
      }
    });
  }

  // Copy buttons on code blocks.
  function addCopyButtons(root) {
    (root || document).querySelectorAll(".code-block").forEach(function (block) {
      if (block.querySelector(".copy-btn") || !navigator.clipboard) return;
      var btn = document.createElement("button");
      btn.type = "button";
      btn.className = "copy-btn";
      btn.textContent = "Copy";
      btn.addEventListener("click", function () {
        var code = block.querySelector("code, pre");
        navigator.clipboard.writeText(code ? code.innerText : "").then(function () {
          btn.textContent = "Copied!";
          setTimeout(function () { btn.textContent = "Copy"; }, 1500);
        }, function () {
          btn.textContent = "Press Ctrl+C";
        });
      });
      block.appendChild(btn);
    });
  }

  // Let Tab insert spaces in code editors instead of moving focus.
  function setupCodeInputs() {
    document.querySelectorAll("textarea.code-input").forEach(function (ta) {
      ta.addEventListener("keydown", function (e) {
        if (e.key !== "Tab" || e.shiftKey || e.ctrlKey || e.altKey || e.metaKey) return;
        e.preventDefault();
        var start = ta.selectionStart, end = ta.selectionEnd;
        ta.value = ta.value.slice(0, start) + "\t" + ta.value.slice(end);
        ta.selectionStart = ta.selectionEnd = start + 1;
      });
    });
  }

  // Suggestion chips fill the nearest question box.
  function setupSuggestions() {
    var target = document.querySelector("textarea[data-question]");
    if (!target) return;
    document.querySelectorAll("[data-suggest]").forEach(function (btn) {
      btn.addEventListener("click", function () {
        target.value = btn.getAttribute("data-suggest");
        target.focus();
      });
    });
  }

  function escapeHTML(s) {
    var d = document.createElement("div");
    d.textContent = s;
    return d.innerHTML;
  }

  // Exposed for page-specific features (chat).
  window.Tutor = { addCopyButtons: addCopyButtons, escapeHTML: escapeHTML, setLoading: setLoading };

  document.addEventListener("DOMContentLoaded", function () {
    setupNav();
    setupConfirm();
    setupLoadingForms();
    setupCodeInputs();
    setupSuggestions();
    addCopyButtons(document);
  });
})();
