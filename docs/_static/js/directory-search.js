// Filter the cards of each searchable directory as the user types in its search box.
document.addEventListener("DOMContentLoaded", function () {
  document.querySelectorAll(".directory").forEach(function (directory) {
    var input = directory.querySelector(".directory-search-input");
    var cards = directory.querySelectorAll("[data-search]");
    var visibleCount = directory.querySelector(".directory-visible-count");
    var empty = directory.querySelector(".directory-empty");

    input.addEventListener("input", function () {
      var query = input.value.trim().toLowerCase();
      var visible = 0;
      cards.forEach(function (card) {
        var match = card.dataset.search.includes(query);
        card.hidden = !match;
        if (match) {
          visible++;
        }
      });
      visibleCount.textContent = visible;
      empty.hidden = visible !== 0;
    });
  });
});
