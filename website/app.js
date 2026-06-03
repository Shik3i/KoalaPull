(function () {
  var reducedMotion = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches

  function onReady(fn) {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', fn, { once: true })
      return
    }
    fn()
  }

  function clamp(value, min, max) {
    return Math.min(max, Math.max(min, value))
  }

  function setupReveal() {
    var items = Array.prototype.slice.call(document.querySelectorAll('[data-reveal]'))
    if (!items.length) return

    if (reducedMotion || !('IntersectionObserver' in window)) {
      items.forEach(function (item) {
        item.classList.add('is-revealed')
      })
      return
    }

    var observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return
        entry.target.classList.add('is-revealed')
        observer.unobserve(entry.target)
      })
    }, { threshold: 0.18, rootMargin: '0px 0px -8% 0px' })

    items.forEach(function (item) {
      observer.observe(item)
    })
  }

  function setupStickyNav() {
    var nav = document.querySelector('[data-sticky-nav]') || document.querySelector('nav')
    if (!nav) return

    function update() {
      nav.classList.toggle('is-sticky', window.scrollY > 12)
      document.body.classList.toggle('is-scrolled', window.scrollY > 12)
    }

    update()
    window.addEventListener('scroll', update, { passive: true })
  }

  function smoothScrollTo(target) {
    if (!target) return
    if (reducedMotion) return
    target.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  function setupAnchors() {
    var links = Array.prototype.slice.call(document.querySelectorAll('a[href^="#"]'))
    links.forEach(function (link) {
      link.addEventListener('click', function (event) {
        var href = link.getAttribute('href') || ''
        if (!href || href === '#') return
        var target = document.querySelector(href)
        if (!target) return
        event.preventDefault()
        smoothScrollTo(target)
        history.replaceState(null, '', href)
      })
    })
  }

  function setupMobileNav() {
    var toggles = Array.prototype.slice.call(document.querySelectorAll('[data-nav-toggle]'))
    var menus = Array.prototype.slice.call(document.querySelectorAll('[data-nav-menu]'))
    if (!toggles.length || !menus.length) return

    function setOpen(isOpen) {
      toggles.forEach(function (button) {
        button.classList.toggle('is-open', isOpen)
        button.setAttribute('aria-expanded', String(isOpen))
      })
      menus.forEach(function (menu) {
        menu.classList.toggle('is-open', isOpen)
      })
      document.documentElement.classList.toggle('nav-open', isOpen)
    }

    toggles.forEach(function (button) {
      button.addEventListener('click', function () {
        var isOpen = !menus[0].classList.contains('is-open')
        setOpen(isOpen)
      })
    })

    menus.forEach(function (menu) {
      menu.addEventListener('click', function (event) {
        var target = event.target
        if (!(target instanceof Element)) return
        if (target.matches('a')) {
          setOpen(false)
        }
      })
    })

    window.addEventListener('resize', function () {
      if (window.innerWidth > 900) setOpen(false)
    })
  }

  function setupLanguageSelect() {
    var selects = Array.prototype.slice.call(document.querySelectorAll('[data-language-select]'))
    if (!selects.length) return

    function resolveLanguageUrl(value) {
      if (!value) return ''
      if (/^https?:\/\//i.test(value) || value.startsWith('/')) return value
      if (value === 'en') return '/'
      if (value === 'de' || value === 'fr') return '/' + value + '/'
      return value
    }

    selects.forEach(function (select) {
      select.addEventListener('change', function () {
        var option = select.selectedOptions && select.selectedOptions[0]
        var next = option && resolveLanguageUrl(option.getAttribute('data-url') || option.getAttribute('data-path') || option.value)
        if (!next) return
        window.location.assign(next)
      })
    })
  }

  function setupMockTabs() {
    var buttons = Array.prototype.slice.call(document.querySelectorAll('[data-mock-tab]'))
    var panels = Array.prototype.slice.call(document.querySelectorAll('[data-mock-panel]'))
    if (!buttons.length || !panels.length) return

    function activate(tab) {
      buttons.forEach(function (button) {
        var isActive = button.getAttribute('data-mock-tab') === tab
        button.classList.toggle('is-active', isActive)
        button.setAttribute('aria-pressed', String(isActive))
      })

      panels.forEach(function (panel) {
        var isMatch = panel.getAttribute('data-mock-panel') === tab
        panel.hidden = !isMatch
        panel.classList.toggle('is-active', isMatch)
      })
    }

    var initial = null
    buttons.forEach(function (button) {
      if (!initial && button.classList.contains('is-active')) {
        initial = button.getAttribute('data-mock-tab')
      }
      button.addEventListener('click', function () {
        activate(button.getAttribute('data-mock-tab') || '')
      })
    })

    activate(initial || buttons[0].getAttribute('data-mock-tab') || '')
  }

  function setupFakeProgress() {
    var bars = Array.prototype.slice.call(document.querySelectorAll('[data-progress-fill]'))
    if (!bars.length) return

    var value = 42
    var step = 1

    function paint() {
      bars.forEach(function (bar, index) {
        var min = Number(bar.getAttribute('data-progress-min') || 38)
        var max = Number(bar.getAttribute('data-progress-max') || 91)
        var offset = index * 7
        var next = clamp(value + offset, min, max)
        bar.style.width = next + '%'
        bar.setAttribute('aria-valuenow', String(Math.round(next)))
      })
      value += step
      if (value >= 92) {
        value = 42
      }
    }

    paint()
    window.setInterval(paint, 1200)
  }

  onReady(function () {
    setupReveal()
    setupStickyNav()
    setupAnchors()
    setupMobileNav()
    setupLanguageSelect()
    setupMockTabs()
    setupFakeProgress()
  })
})();
