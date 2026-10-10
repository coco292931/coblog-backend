/* /lite 写作页的增强脚本。
 *
 * 必须是 ES5：老 Kindle（WebKit 534）遇到 let / const / 箭头函数 / 模板字符串
 * 等任何一个 ES6 写法，整段脚本在解析阶段就失败（write_test.go 有护栏）。
 * 也不用 fetch / Promise / NodeList.forEach / Function.bind / Element.closest。
 *
 * 脚本只做增强：不加载或执行失败时，表单本身照样能提交，
 * 选中的图片会随表单一起上传（见 write.go 的 handleUploads）。
 */
(function () {
  'use strict';

  if (!window.XMLHttpRequest || !window.FormData || !document.getElementById) {
    return;
  }

  var UPLOAD_URL = '/lite/upload';

  function $(id) {
    return document.getElementById(id);
  }

  function setStatus(text, isError) {
    var el = $('upload-status');
    if (!el) {
      return;
    }
    el.innerHTML = '';
    el.appendChild(document.createTextNode(text));
    el.className = 'lite-upload-status' + (isError ? ' is-error' : '');
  }

  // 上传一张图片。done(err, data)，data 为 { url, thumb_url }
  function upload(file, done) {
    var xhr = new XMLHttpRequest();
    var fd = new FormData();
    fd.append('file', file);
    xhr.open('POST', UPLOAD_URL, true);
    xhr.setRequestHeader('X-Requested-With', 'XMLHttpRequest');
    // CSRF token 取自写作表单里的隐藏字段（见 layout.html 的 "csrf"）
    var form = $('write-form');
    var csrf = form && form.elements ? form.elements['_csrf'] : null;
    if (csrf && csrf.value) {
      xhr.setRequestHeader('X-CSRF-Token', csrf.value);
    }
    xhr.onreadystatechange = function () {
      if (xhr.readyState !== 4) {
        return;
      }
      var res = null;
      try {
        res = JSON.parse(xhr.responseText);
      } catch (e) {
        res = null;
      }
      if (res && res.ok && res.data && res.data.url) {
        done(null, res.data);
      } else if (res && res.msg) {
        done(res.msg);
      } else {
        done('上传失败（' + xhr.status + '），请稍后重试');
      }
    };
    xhr.send(fd);
  }

  // 清空文件框，避免提交表单时同一张图再传一遍
  function clearFileInput(input) {
    try {
      input.value = '';
    } catch (e) {
      // 个别老引擎不允许改 file 的 value，最坏情况是提交时再传一次
    }
  }

  // ── 正文：插入到光标处，取不到光标就追加到末尾 ─────────────────────

  function insertIntoEditor(text) {
    var ta = $('md_content');
    if (!ta) {
      return;
    }
    var value = ta.value;
    if (typeof ta.selectionStart === 'number' && typeof ta.selectionEnd === 'number') {
      var start = ta.selectionStart;
      var end = ta.selectionEnd;
      var before = value.substring(0, start);
      var after = value.substring(end);
      // 图片独占一段：前后不是空行就补换行
      var prefix = before === '' || /\n\n$/.test(before) ? '' : (/\n$/.test(before) ? '\n' : '\n\n');
      var suffix = after === '' || /^\n/.test(after) ? '' : '\n';
      ta.value = before + prefix + text + suffix + after;
      var pos = (before + prefix + text).length;
      try {
        ta.selectionStart = ta.selectionEnd = pos;
      } catch (e) {}
    } else {
      var trimmed = value.replace(/\s+$/, '');
      ta.value = (trimmed === '' ? '' : trimmed + '\n\n') + text + '\n';
    }
    updateCount();
  }

  // 依次上传多张，全部完成后一次插入（顺序与选择顺序一致）
  function uploadImages(files, input) {
    var list = [];
    var i;
    for (i = 0; i < files.length; i++) {
      list.push(files[i]);
    }
    if (list.length === 0) {
      return;
    }
    var snippets = [];
    var failed = [];
    var idx = 0;

    function next() {
      if (idx >= list.length) {
        if (snippets.length > 0) {
          insertIntoEditor(snippets.join('\n\n'));
        }
        clearFileInput(input);
        if (failed.length > 0) {
          setStatus(failed.join('；'), true);
        } else {
          setStatus('已插入 ' + snippets.length + ' 张图片', false);
        }
        return;
      }
      var file = list[idx];
      setStatus('正在上传第 ' + (idx + 1) + ' / ' + list.length + ' 张…', false);
      upload(file, function (err, data) {
        if (err) {
          failed.push((file.name || '图片') + '：' + err);
        } else {
          snippets.push('![' + (file.name || '') + '](' + data.url + ')');
        }
        idx++;
        next();
      });
    }
    next();
  }

  // ── 封面：上传后填进 URL 框并刷新预览 ──────────────────────────────

  function setCoverPreview(thumbURL) {
    var box = $('cover-preview');
    if (!box) {
      return;
    }
    box.style.backgroundImage = thumbURL ? "url('" + thumbURL.replace(/'/g, '%27') + "')" : '';
    box.style.display = thumbURL ? '' : 'none';
  }

  function uploadCover(input) {
    var file = input.files && input.files[0];
    if (!file) {
      return;
    }
    setStatus('正在上传封面…', false);
    upload(file, function (err, data) {
      clearFileInput(input);
      if (err) {
        setStatus('封面：' + err, true);
        return;
      }
      $('cover_image').value = data.url;
      setCoverPreview(data.thumb_url);
      setStatus('封面已更新', false);
    });
  }

  // ── 字数（与主站一致：不含空白） ─────────────────────────────────

  function updateCount() {
    var ta = $('md_content');
    var el = $('char-count');
    if (!ta || !el) {
      return;
    }
    el.innerHTML = '';
    el.appendChild(document.createTextNode(ta.value.replace(/\s/g, '').length + ' 字'));
  }

  // ── 挂事件 ──────────────────────────────────────────────────────

  try {
    var cover = $('cover_file');
    if (cover) {
      cover.onchange = function () {
        uploadCover(cover);
      };
    }
    var images = $('images');
    if (images) {
      images.onchange = function () {
        uploadImages(images.files || [], images);
      };
    }
    var ta = $('md_content');
    if (ta) {
      ta.onkeyup = updateCount;
      ta.oninput = updateCount;
      updateCount();
    }
    // 手改封面 URL 时预览跟着变（站内图换成压缩图，与服务端 ThumbURL 一致）
    var coverURL = $('cover_image');
    if (coverURL) {
      coverURL.onchange = function () {
        var v = coverURL.value.replace(/^\s+|\s+$/g, '');
        if (v && v.indexOf('/static/uploads/') >= 0 && v.indexOf('thumb=1') < 0) {
          v += (v.indexOf('?') >= 0 ? '&' : '?') + 'thumb=1';
        }
        setCoverPreview(v);
      };
    }
    // 标记脚本已生效：页面上「随表单一起上传」的说明换成即时上传的说明
    var form = $('write-form');
    if (form) {
      form.className += ' is-enhanced';
    }
  } catch (e) {
    // 增强失败不影响表单本身
  }
})();
