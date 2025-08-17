// Minimal client-side helpers

// Global helper to read CSRF token from meta tag
// Keeps client-side POSTs CSRF-compliant if any feature uses fetch/XHR
window.getCSRFToken = function () {
  const meta = document.querySelector('meta[name="csrf-token"]');
  return meta ? meta.getAttribute('content') : '';
};

// Navbar burger toggle
window.addEventListener('DOMContentLoaded', function () {
  var burger = document.querySelector('.navbar-burger');
  var menu = document.querySelector('.navbar-menu');
  if (burger && menu) {
    burger.addEventListener('click', function () {
      var active = burger.classList.toggle('is-active');
      menu.classList.toggle('is-active');
      burger.setAttribute('aria-expanded', active ? 'true' : 'false');
    });
  }
});
