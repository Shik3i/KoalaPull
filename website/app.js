(function () {
  var reducedMotion = window.matchMedia && window.matchMedia('(prefers-reduced-motion: reduce)').matches;

  function onReady(fn) {
    if (document.readyState === 'loading') {
      document.addEventListener('DOMContentLoaded', fn, { once: true });
      return;
    }
    fn();
  }

  function clamp(value, min, max) {
    return Math.min(max, Math.max(min, value));
  }

  function setupReveal() {
    var items = Array.prototype.slice.call(document.querySelectorAll('[data-reveal]'));
    if (!items.length) return;

    if (reducedMotion || !('IntersectionObserver' in window)) {
      items.forEach(function (item) {
        item.classList.add('is-revealed');
      });
      return;
    }

    var observer = new IntersectionObserver(function (entries) {
      entries.forEach(function (entry) {
        if (!entry.isIntersecting) return;
        entry.target.classList.add('is-revealed');
        observer.unobserve(entry.target);
      });
    }, { threshold: 0.14, rootMargin: '0px 0px -10% 0px' });

    items.forEach(function (item) {
      observer.observe(item);
    });
  }

  function setupStickyNav() {
    var nav = document.querySelector('[data-sticky-nav]');
    if (!nav) return;

    function update() {
      var sticky = window.scrollY > 14;
      nav.classList.toggle('is-sticky', sticky);
      document.body.classList.toggle('is-scrolled', sticky);
    }

    update();
    window.addEventListener('scroll', update, { passive: true });
  }

  function smoothScrollTo(target) {
    if (!target || reducedMotion) return;
    target.scrollIntoView({ behavior: 'smooth', block: 'start' });
  }

  function setupAnchors() {
    Array.prototype.slice.call(document.querySelectorAll('a[href^="#"]')).forEach(function (link) {
      link.addEventListener('click', function (event) {
        var href = link.getAttribute('href') || '';
        if (!href || href === '#') return;
        var target = document.querySelector(href);
        if (!target) return;
        event.preventDefault();
        smoothScrollTo(target);
        history.replaceState(null, '', href);
      });
    });
  }

  function setupMobileNav() {
    var toggles = Array.prototype.slice.call(document.querySelectorAll('[data-nav-toggle]'));
    var menus = Array.prototype.slice.call(document.querySelectorAll('[data-nav-menu]'));
    if (!toggles.length || !menus.length) return;

    function setOpen(isOpen) {
      toggles.forEach(function (button) {
        button.classList.toggle('is-open', isOpen);
        button.setAttribute('aria-expanded', String(isOpen));
      });
      menus.forEach(function (menu) {
        menu.classList.toggle('is-open', isOpen);
      });
      document.documentElement.classList.toggle('nav-open', isOpen);
    }

    toggles.forEach(function (button) {
      button.addEventListener('click', function () {
        setOpen(!menus[0].classList.contains('is-open'));
      });
    });

    menus.forEach(function (menu) {
      menu.addEventListener('click', function (event) {
        var target = event.target;
        if (target instanceof Element && target.matches('a')) {
          setOpen(false);
        }
      });
    });

    window.addEventListener('resize', function () {
      if (window.innerWidth > 900) setOpen(false);
    });
  }

  function setupLanguageSelect() {
    Array.prototype.slice.call(document.querySelectorAll('[data-language-select]')).forEach(function (select) {
      select.addEventListener('change', function () {
        var option = select.selectedOptions && select.selectedOptions[0];
        var next = option && (option.getAttribute('data-url') || option.value);
        if (next) window.location.assign(next);
      });
    });
  }

  function setupEmailReveal() {
    Array.prototype.slice.call(document.querySelectorAll('.email-reveal')).forEach(function (button) {
      button.addEventListener('click', function () {
        var user = button.getAttribute('data-user');
        var domain = button.getAttribute('data-domain');
        if (!user || !domain) return;
        var email = user + '@' + domain;
        button.textContent = email;
        button.setAttribute('aria-label', email);
      });
    });
  }

  function setupExperienceTabs(root, toast) {
    var buttons = Array.prototype.slice.call(root.querySelectorAll('[data-mock-tab]'));
    var panels = Array.prototype.slice.call(root.querySelectorAll('[data-mock-panel]'));

    function activate(tab) {
      buttons.forEach(function (button) {
        var active = button.getAttribute('data-mock-tab') === tab;
        button.classList.toggle('is-active', active);
        button.setAttribute('aria-pressed', String(active));
      });

      panels.forEach(function (panel) {
        var active = panel.getAttribute('data-mock-panel') === tab;
        panel.hidden = !active;
        panel.classList.toggle('is-active', active);
      });
    }

    buttons.forEach(function (button) {
      button.addEventListener('click', function () {
        activate(button.getAttribute('data-mock-tab') || '');
      });
    });

    Array.prototype.slice.call(root.querySelectorAll('[data-demo-jump]')).forEach(function (button) {
      button.addEventListener('click', function () {
        activate(button.getAttribute('data-demo-jump') || 'downloads');
        toast(root.dataset.toastReady || '');
      });
    });

    activate('downloads');
  }

  function setupExperienceDemo() {
    var root = document.querySelector('[data-demo]');
    if (!root) return;

    var toastNode = root.querySelector('[data-demo-toast]');
    var statusNode = root.querySelector('[data-demo-status]');
    var summaryNode = root.querySelector('[data-demo-summary]');
    var presetLabelNode = root.querySelector('[data-demo-preset-label]');
    var analyzeButton = root.querySelector('[data-demo-analyze]');
    var queueButton = root.querySelector('[data-demo-queue]');
    var openButton = root.querySelector('[data-demo-open]');
    var queueStatus = root.querySelector('[data-queue-primary-status]');
    var queueProgress = root.querySelector('[data-queue-primary-progress]');
    var progressFill = root.querySelector('[data-progress-fill]');
    var historyFilters = Array.prototype.slice.call(root.querySelectorAll('[data-history-filter]'));
    var settingToggles = Array.prototype.slice.call(root.querySelectorAll('[data-setting-toggle]'));
    var presetButtons = Array.prototype.slice.call(root.querySelectorAll('[data-demo-preset]'));

    var state = {
      busy: false,
      progress: 0,
      preset: presetButtons[0] ? presetButtons[0].textContent : '',
      initialQueueStatus: queueStatus ? queueStatus.textContent : '',
      initialQueueProgress: queueProgress ? queueProgress.textContent : '',
      timer: 0,
      toastTimer: 0
    };

    function showToast(message) {
      if (!message) return;
      if (!toastNode) return;
      toastNode.textContent = message;
      toastNode.classList.add('is-visible');
      window.clearTimeout(state.toastTimer);
      state.toastTimer = window.setTimeout(function () {
        toastNode.classList.remove('is-visible');
      }, reducedMotion ? 900 : 1800);
    }

    function setStatus(text, tone) {
      if (!statusNode || !summaryNode) return;
      statusNode.textContent = text;
      summaryNode.textContent = text;
      statusNode.classList.remove('is-busy', 'is-done');
      if (tone === 'busy') statusNode.classList.add('is-busy');
      if (tone === 'done') statusNode.classList.add('is-done');
    }

    function updateProgress(value) {
      state.progress = clamp(value, 0, 100);
      if (progressFill) {
        progressFill.style.width = state.progress + '%';
      }
      if (queueProgress) {
        if (state.progress >= 100) {
          queueProgress.textContent = root.dataset.toastDone || '';
        } else {
          queueProgress.textContent = state.progress + '% - 8.4 MB/s - 00:' + String(clamp(100 - state.progress, 8, 59)).padStart(2, '0') + ' left';
        }
      }
    }

    function stopTimer() {
      if (!state.timer) return;
      window.clearInterval(state.timer);
      state.timer = 0;
    }

    function resetDemo() {
      stopTimer();
      state.busy = false;
      analyzeButton.classList.remove('is-busy');
      analyzeButton.disabled = false;
      queueButton.disabled = false;
      updateProgress(0);
      if (queueStatus) {
        queueStatus.textContent = state.initialQueueStatus;
        queueStatus.className = 'queue-item-status queued';
      }
      if (queueProgress) {
        queueProgress.textContent = state.initialQueueProgress;
      }
      setStatus(root.dataset.statusIdle || '', '');
      showToast(root.dataset.toastReset || '');
    }

    function finishDownload() {
      stopTimer();
      state.busy = false;
      if (queueStatus) {
        queueStatus.textContent = root.dataset.statusDone || '';
        queueStatus.className = 'queue-item-status completed';
      }
      setStatus(root.dataset.statusDone || '', 'done');
      showToast(root.dataset.toastDone || '');
    }

    function startDownload() {
      if (state.busy) return;
      state.busy = true;
      updateProgress(4);
      if (queueStatus) {
        queueStatus.textContent = root.dataset.statusBusy || '';
        queueStatus.className = 'queue-item-status downloading';
      }
      setStatus(root.dataset.statusBusy || '', 'busy');
      showToast(root.dataset.toastQueued || '');

      state.timer = window.setInterval(function () {
        updateProgress(state.progress + 7);
        if (state.progress >= 100) {
          updateProgress(100);
          finishDownload();
        }
      }, reducedMotion ? 90 : 260);
    }

    function analyze() {
      if (state.busy || !analyzeButton) return;
      analyzeButton.classList.add('is-busy');
      analyzeButton.disabled = true;
      setStatus(root.dataset.statusBusy || '', 'busy');
      showToast(root.dataset.toastAnalyzing || '');

      window.setTimeout(function () {
        analyzeButton.classList.remove('is-busy');
        analyzeButton.disabled = false;
        setStatus(root.dataset.statusIdle || '', '');
        showToast(root.dataset.toastReady || '');
      }, reducedMotion ? 100 : 1100);
    }

    if (analyzeButton) {
      analyzeButton.addEventListener('click', analyze);
    }

    if (queueButton) {
      queueButton.addEventListener('click', startDownload);
    }

    if (openButton) {
      openButton.addEventListener('click', function () {
        showToast(root.dataset.toastDone || '');
      });
    }

    presetButtons.forEach(function (button) {
      button.addEventListener('click', function () {
        presetButtons.forEach(function (chip) {
          chip.classList.toggle('is-active', chip === button);
        });
        state.preset = button.textContent || '';
        if (presetLabelNode) presetLabelNode.textContent = state.preset;
      });
    });

    historyFilters.forEach(function (button) {
      button.addEventListener('click', function () {
        historyFilters.forEach(function (chip) {
          chip.classList.toggle('is-active', chip === button);
        });
        showToast(button.textContent || '');
      });
    });

    settingToggles.forEach(function (button) {
      button.addEventListener('click', function () {
        button.classList.toggle('is-active');
        showToast(button.textContent || '');
      });
    });

    if (progressFill && !progressFill.style.width) {
      progressFill.style.width = '0%';
    }

    setupExperienceTabs(root, showToast);
    resetDemo();
  }

  onReady(function () {
    setupReveal();
    setupStickyNav();
    setupAnchors();
    setupMobileNav();
    setupLanguageSelect();
    setupEmailReveal();
    setupExperienceDemo();
  });
})();
