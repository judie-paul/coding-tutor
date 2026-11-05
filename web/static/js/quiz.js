// Shows how many quiz questions are answered and warns before submitting
// with unanswered questions. The quiz works without JavaScript too.
(function () {
  "use strict";
  var form = document.getElementById("quiz-form");
  if (!form) return;
  var counter = document.getElementById("quiz-count");
  var total = parseInt(form.getAttribute("data-total"), 10) || 0;

  function answered() {
    var n = 0;
    form.querySelectorAll("[data-question]").forEach(function (q) {
      var checked = q.querySelector("input[type=radio]:checked");
      var text = q.querySelector("input[type=text]");
      if (checked || (text && text.value.trim() !== "")) n++;
    });
    return n;
  }

  function update() {
    counter.textContent = answered() + " of " + total + " answered";
  }

  form.addEventListener("input", update);
  form.addEventListener("change", update);
  form.addEventListener("submit", function (e) {
    var missing = total - answered();
    if (missing > 0 && !window.confirm(missing + " question(s) are unanswered and will be marked wrong. Submit anyway?")) {
      e.preventDefault();
    }
  });
  update();
})();
