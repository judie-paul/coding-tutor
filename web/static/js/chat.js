// Asynchronous follow-up questions for the tutor conversation page.
// Without JavaScript the form still works as a normal POST.
(function () {
  "use strict";

  var form = document.getElementById("chat-form");
  if (!form || !window.fetch) return;
  var list = document.getElementById("chat-messages");
  var anchor = document.getElementById("latest");
  var errorBox = document.getElementById("chat-error");
  var textarea = form.querySelector("textarea");
  var button = form.querySelector("button[type=submit]");
  var userInitial = form.getAttribute("data-user-initial") || "?";
  var T = window.Tutor;

  function scrollToLatest() {
    anchor.scrollIntoView({ behavior: "smooth", block: "end" });
  }

  function addMessage(role, bodyNode, time) {
    var wrap = document.createElement("div");
    wrap.className = "message message-" + role;
    var avatar = document.createElement("span");
    avatar.className = "message-avatar";
    avatar.setAttribute("aria-hidden", "true");
    avatar.textContent = role === "user" ? userInitial : "AI";
    var body = document.createElement("div");
    body.className = "message-body" + (role === "assistant" ? " prose" : "");
    body.appendChild(bodyNode);
    if (time) {
      var t = document.createElement("span");
      t.className = "message-time";
      t.textContent = time;
      body.appendChild(t);
    }
    wrap.appendChild(avatar);
    wrap.appendChild(body);
    list.insertBefore(wrap, anchor);
    return wrap;
  }

  function showError(msg) {
    errorBox.innerHTML = "";
    if (!msg) return;
    var div = document.createElement("div");
    div.className = "alert alert-error";
    div.textContent = msg;
    errorBox.appendChild(div);
  }

  function typingNode() {
    var span = document.createElement("span");
    span.className = "typing";
    span.setAttribute("aria-label", "The tutor is typing");
    span.innerHTML = "<span></span><span></span><span></span>";
    return span;
  }

  var busy = false;
  form.addEventListener("submit", function (e) {
    e.preventDefault();
    var question = textarea.value.trim();
    if (!question || busy) return;
    busy = true;
    button.disabled = true;
    showError("");

    var data = new FormData(form);
    var pendingUser = addMessage("user", document.createTextNode(question), "sending…");
    var typing = addMessage("assistant", typingNode());
    textarea.value = "";
    scrollToLatest();

    fetch(form.action, {
      method: "POST",
      body: new URLSearchParams(data),
      headers: { "Accept": "application/json" },
      credentials: "same-origin"
    }).then(function (res) {
      return res.json().catch(function () { return {}; }).then(function (body) {
        return { ok: res.ok, status: res.status, body: body };
      });
    }).then(function (r) {
      typing.remove();
      if (!r.ok) {
        pendingUser.remove();
        textarea.value = question;
        if (r.status === 401) {
          window.location.href = "/login?next=" + encodeURIComponent(window.location.pathname);
          return;
        }
        showError(r.body.error || "Something went wrong. Please try again.");
        return;
      }
      pendingUser.querySelector(".message-time").textContent = r.body.user.time;
      var tpl = document.createElement("template");
      tpl.innerHTML = r.body.assistant.html; // rendered and escaped on the server
      var msg = addMessage("assistant", tpl.content, r.body.assistant.time);
      T.addCopyButtons(msg);
      scrollToLatest();
    }).catch(function () {
      typing.remove();
      pendingUser.remove();
      textarea.value = question;
      showError("Could not reach the server. Check your connection and try again.");
    }).then(function () {
      busy = false;
      button.disabled = false;
      textarea.focus();
    });
  });

  // Enter sends, Shift+Enter adds a new line.
  textarea.addEventListener("keydown", function (e) {
    if (e.key === "Enter" && !e.shiftKey && !e.isComposing) {
      e.preventDefault();
      form.requestSubmit ? form.requestSubmit() : form.dispatchEvent(new Event("submit", { cancelable: true }));
    }
  });

  document.querySelectorAll("[data-followup]").forEach(function (btn) {
    btn.addEventListener("click", function () {
      textarea.value = btn.getAttribute("data-followup");
      textarea.focus();
    });
  });

  if (list.children.length > 2) scrollToLatest();
})();
