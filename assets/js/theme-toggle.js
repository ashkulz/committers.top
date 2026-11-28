(function() {
  'use strict';

  var STORAGE_KEY = 'theme';
  var root = document.documentElement;
  var button = document.getElementById('theme-toggle');
  var media = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;

  function storedTheme() {
    try {
      var theme = localStorage.getItem(STORAGE_KEY);
      return (theme === 'dark' || theme === 'light') ? theme : null;
    } catch (e) {
      // localStorage can be unavailable (e.g. private browsing modes)
      return null;
    }
  }

  function currentTheme() {
    var theme = storedTheme();
    if (theme) return theme;
    return (media && media.matches) ? 'dark' : 'light';
  }

  function apply(theme) {
    root.setAttribute('data-theme', theme);
    if (button) {
      button.setAttribute('aria-pressed', String(theme === 'dark'));
      button.setAttribute('aria-label', 'Switch to ' + (theme === 'dark' ? 'light' : 'dark') + ' mode');
    }
  }

  apply(currentTheme());

  if (button) {
    button.addEventListener('click', function() {
      var theme = currentTheme() === 'dark' ? 'light' : 'dark';
      try { localStorage.setItem(STORAGE_KEY, theme); } catch (e) {}
      apply(theme);
    });
  }

  // follow OS preference changes while the user hasn't chosen explicitly
  if (media && media.addEventListener) {
    media.addEventListener('change', function(e) {
      if (!storedTheme()) apply(e.matches ? 'dark' : 'light');
    });
  }
})();
