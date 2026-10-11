/* /lite 全站增强脚本，每页在 </body> 前加载。
 *
 * 必须是 ES5：老 Kindle（WebKit 534）遇到任何一个 ES6 写法，整段脚本在解析阶段就失败
 * （write_test.go 的 assertES5 有护栏）。也不用 fetch / Promise 构造 / NodeList.forEach /
 * Function.bind / Element.closest。
 *
 * 内容都是服务端直出的，这里只做增强，每一块各自兜底，任何一块失败都不影响阅读：
 *   - 夜间模式开关（<head> 里的内联脚本负责在绘制前加 class，这里只管切换）
 *   - 页脚避风时长每秒走字（同主站 Footer.vue）
 *   - 复制：RSS 地址、正文代码块（同主站 rss 页与 utils/prose.js）
 */
(function () {
  'use strict';

  function $(id) {
    return document.getElementById(id);
  }

  function setText(el, text) {
    el.innerHTML = '';
    el.appendChild(document.createTextNode(text));
  }

  function hasClass(el, name) {
    return (' ' + el.className + ' ').indexOf(' ' + name + ' ') >= 0;
  }

  // 每一块各自 try：一块出错不连累其它
  function safely(fn) {
    try {
      fn();
    } catch (e) {}
  }

  // ── 夜间模式开关 ──────────────────────────────────────────────────
  // 切 class 即时生效，并写回 coblog-theme（主站的主题也会跟着变）。
  // 只在用户点了之后才写，不像主站那样一进来就把系统主题固化下来。

  function setupThemeToggle() {
    var b = $('theme-toggle');
    if (!b) {
      return;
    }
    var d = document.documentElement;
    function label() {
      setText(b, hasClass(d, 'lite-dark') ? '切换到亮色' : '切换到暗色');
    }
    b.onclick = function () {
      var dark = !hasClass(d, 'lite-dark');
      if (dark) {
        d.className += ' lite-dark';
      } else {
        d.className = d.className.replace(/(^|\s)lite-dark(?=\s|$)/g, '');
      }
      try {
        window.localStorage.setItem('coblog-theme', dark ? 'dark' : 'light');
      } catch (e) {}
      label();
    };
    label();
  }

  // ── 避风时长 ──────────────────────────────────────────────────────
  // 服务端先渲染一份快照（没有脚本时就停在那一刻），这里接着每秒走字。
  // 以服务端渲染的时刻为准校准：设备时钟不准时也不会跑偏。
  // 每次都由时间差重新算，不靠累加：老 Kindle 的定时器会漂，累加会越走越慢。

  function formatUptime(secs) {
    var neg = secs < 0;
    if (neg) {
      secs = -secs;
    }
    var d = Math.floor(secs / 86400);
    var h = Math.floor((secs % 86400) / 3600);
    var m = Math.floor((secs % 3600) / 60);
    var s = secs % 60;
    var span = d + '天' + h + '时' + m + '分' + s + '秒';
    // 与服务端 liteview.FormatUptime 一致
    return neg ? '本港湾还有 ' + span + ' 开放' : '已避风 ' + span;
  }

  function setupUptime() {
    var el = $('lite-uptime');
    if (!el) {
      return;
    }
    var start = parseInt(el.getAttribute('data-start'), 10);
    var serverNow = parseInt(el.getAttribute('data-now'), 10);
    if (!start || !serverNow) {
      return;
    }
    var offset = serverNow * 1000 - new Date().getTime();
    function tick() {
      var now = new Date().getTime() + offset;
      setText(el, formatUptime(Math.floor(now / 1000) - start));
    }
    tick();
    window.setInterval(tick, 1000);
  }

  // ── 复制 ──────────────────────────────────────────────────────────
  // 优先用异步剪贴板 API（只在 https 下有），否则退回 execCommand('copy')。
  // 两条路都不通时，把要复制的内容选中，让用户自己长按复制。

  var canCopy = false;
  try {
    canCopy = !!(navigator.clipboard && navigator.clipboard.writeText) ||
      !!(document.queryCommandSupported && document.queryCommandSupported('copy'));
  } catch (e) {
    canCopy = false;
  }

  var COPIED_MS = 1500;

  function execCopy(text) {
    var ta = document.createElement('textarea');
    ta.value = text;
    ta.setAttribute('readonly', '');
    // 必须留在文档流里且可选中，display:none 的元素选不中
    ta.style.position = 'fixed';
    ta.style.top = '0';
    ta.style.left = '-9999px';
    document.body.appendChild(ta);
    ta.select();
    var ok = false;
    try {
      ok = document.execCommand('copy');
    } catch (e) {
      ok = false;
    }
    document.body.removeChild(ta);
    return ok;
  }

  // done(ok)
  function copyText(text, done) {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text).then(function () {
        done(true);
      }, function () {
        done(execCopy(text));
      });
      return;
    }
    done(execCopy(text));
  }

  function selectContents(el) {
    try {
      var range = document.createRange();
      range.selectNodeContents(el);
      var sel = window.getSelection();
      sel.removeAllRanges();
      sel.addRange(range);
    } catch (e) {}
  }

  // 按钮点一下：复制 source 的文本，按钮文字短暂换成结果
  function bindCopy(button, source) {
    var idle = button.textContent || '复制';
    var timer = null;
    button.onclick = function () {
      copyText(source.textContent || '', function (ok) {
        if (!ok) {
          selectContents(source);
        }
        setText(button, ok ? '已复制' : '已选中，请手动复制');
        window.clearTimeout(timer);
        timer = window.setTimeout(function () {
          setText(button, idle);
        }, COPIED_MS);
      });
    };
  }

  // 页面上写好的复制按钮：data-copy-target 指向要复制的元素。
  // 按钮默认 style="display:none"，确认能复制才露出来。
  function setupCopyButtons() {
    if (!canCopy) {
      return;
    }
    var buttons = document.querySelectorAll('[data-copy-target]');
    for (var i = 0; i < buttons.length; i++) {
      var target = $(buttons[i].getAttribute('data-copy-target'));
      if (target) {
        bindCopy(buttons[i], target);
        buttons[i].style.display = '';
      }
    }
  }

  // ── 正文代码块：语言标签 + 复制按钮（同主站 utils/prose.js） ──────────
  // 工具条放在 <pre> 前面，不放进 <pre> / <code> 里：否则会被一起复制。
  // 幂等：处理过的 <pre> 打上 data-enhanced，写作页预览每次重渲染都会再调一遍。

  function enhanceProse(root) {
    if (!root || !root.getElementsByTagName) {
      return;
    }
    // getElementsByTagName 是活的集合，插入节点前先拷出来
    var live = root.getElementsByTagName('pre');
    var pres = [];
    var i;
    for (i = 0; i < live.length; i++) {
      pres.push(live[i]);
    }
    for (i = 0; i < pres.length; i++) {
      var pre = pres[i];
      var code = pre.getElementsByTagName('code')[0];
      if (!code || pre.getAttribute('data-enhanced')) {
        continue;
      }
      var match = /(?:^|\s)language-([\w+#.-]+)/.exec(code.className || '');
      if (!match && !canCopy) {
        continue;
      }
      var bar = document.createElement('div');
      bar.className = 'lite-code-toolbar';
      var lang = document.createElement('span');
      lang.className = 'lite-code-lang';
      setText(lang, match ? match[1].toLowerCase() : '');
      bar.appendChild(lang);
      if (canCopy) {
        var button = document.createElement('button');
        button.type = 'button';
        button.className = 'lite-code-copy';
        setText(button, '复制');
        bindCopy(button, code);
        bar.appendChild(button);
      }
      pre.parentNode.insertBefore(bar, pre);
      pre.setAttribute('data-enhanced', '1');
    }
  }

  // 写作页的预览渲染完也要调它（见 write.js）
  window.liteEnhanceProse = enhanceProse;

  safely(setupThemeToggle);
  safely(setupUptime);
  safely(setupCopyButtons);
  safely(function () {
    var list = document.querySelectorAll('.lite-prose');
    for (var i = 0; i < list.length; i++) {
      enhanceProse(list[i]);
    }
  });
})();
