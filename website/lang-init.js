(function () {
  var path = window.location.pathname || '/'
  var head = path.split('/').filter(Boolean)[0]
  var lang = head === 'de' || head === 'fr' ? head : 'en'

  document.documentElement.lang = lang
  document.documentElement.dataset.lang = lang
})();
