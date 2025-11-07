package server

import (
	"bufio"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/elpatron68/dstask-ui/internal/auth"
	"github.com/elpatron68/dstask-ui/internal/config"
	"github.com/elpatron68/dstask-ui/internal/dstask"
	applog "github.com/elpatron68/dstask-ui/internal/log"
	"github.com/elpatron68/dstask-ui/internal/music"
	"github.com/elpatron68/dstask-ui/internal/ui"
)

type Server struct {
	userStore auth.UserStore
	mux       *http.ServeMux
	layoutTpl *template.Template
	cfg       *config.Config
	runner    *dstask.Runner
	cmdStore  *ui.CommandLogStore
	uiCfg     config.UIConfig
}

const faviconSVG = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64">
  <rect rx="12" width="64" height="64" fill="#0366d6"/>
  <path d="M26 44L14 32l4-4 8 8 20-20 4 4-24 24z" fill="#fff"/>
  <!-- simple checkmark in a rounded square -->
 </svg>`

func NewServer(userStore auth.UserStore) *Server {
	return NewServerWithConfig(userStore, config.Default())
}

func NewServerWithConfig(userStore auth.UserStore, cfg *config.Config) *Server {
	s := &Server{userStore: userStore, cfg: cfg, uiCfg: cfg.UI}
	s.runner = dstask.NewRunner(cfg)
	s.mux = http.NewServeMux()
	s.cmdStore = ui.NewCommandLogStore(cfg.UI.CommandLogMax)

	// Templates: register helpers (e.g., split, linkifyURLs, renderMarkdown)
	baseTpl := template.New("layout").Funcs(template.FuncMap{
		"split":          func(s, sep string) []string { return strings.Split(s, sep) },
		"linkifyURLs":    linkifyURLs,
		"renderMarkdown": renderMarkdown,
	})
	s.layoutTpl = template.Must(baseTpl.Parse(`<!doctype html><html><head><meta charset="utf-8"><title>dstask</title><link rel="icon" href="/favicon.svg" type="image/svg+xml">
<style>
body{font-family:system-ui,-apple-system,Segoe UI,Roboto,Ubuntu,Helvetica,Arial,sans-serif;margin:16px}
nav a{padding:6px 10px; text-decoration:none; color:#0366d6; border-radius:4px}
nav a.active{background:#0366d6;color:#fff}
nav{margin-bottom:12px}
.cmdlog{margin-top:16px;border-top:1px solid #eee;padding-top:8px}
.cmdlog .hdr{display:flex;justify-content:space-between;align-items:center}
.cmdlog pre{background:#fff;border:1px solid #d0d7de;padding:8px;max-height:160px;overflow:auto}
.cmdlog .ts{color:#6a737d}
.cmdlog .cmd{color:#24292e;font-weight:600}
.cmdlog .ctx{color:#111827;font-weight:600}
table{border-collapse:collapse;width:100%}
thead th{position:sticky;top:0;background:#f6f8fa;border-bottom:1px solid #d0d7de}
tbody tr:nth-child(even){background:#f9fbfd}
.table-mono, .table-mono th, .table-mono td, table, th, td {font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace}
table, th, td, table pre {font-size:13px}
.badge{display:inline-block;padding:2px 6px;border-radius:12px;font-size:inherit;line-height:1}
.badge.status.active{background:#dcfce7;color:#166534}
.badge.status.pending{background:#e0e7ff;color:#3730a3}
.badge.status.paused{background:#fef3c7;color:#92400e}
.badge.status.resolved{background:#e5e7eb;color:#374151}
.badge.prio{background:#eef2ff;color:#1f2937}
.badge.prio.P0{background:#fee2e2;color:#991b1b}
.badge.prio.P1{background:#ffedd5;color:#9a3412}
.badge.prio.P2{background:#dbeafe;color:#1e3a8a}
.badge.prio.P3{background:#e5e7eb;color:#374151}
.pill{display:inline-block;padding:2px 6px;border-radius:999px;background:#e5e7eb;color:#374151;margin-right:6px;font-size:inherit}
.due.overdue{color:#991b1b;font-weight:600}
.notes-content{padding:12px;background:#f9fafb;border-left:3px solid #0366d6;font-size:14px;line-height:1.6}
.notes-content h1,.notes-content h2,.notes-content h3,.notes-content h4,.notes-content h5,.notes-content h6{margin-top:12px;margin-bottom:8px;font-weight:600}
.notes-content h1{font-size:1.5em}
.notes-content h2{font-size:1.3em}
.notes-content h3{font-size:1.1em}
.notes-content p{margin:8px 0}
.notes-content pre{background:#fff;border:1px solid #d0d7de;padding:8px;overflow-x:auto;border-radius:3px}
.notes-content code{background:#f6f8fa;padding:2px 4px;border-radius:3px;font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,"Liberation Mono","Courier New",monospace;font-size:0.9em}
.notes-content pre code{background:transparent;padding:0}
.notes-content ul,.notes-content ol{margin-left:20px;margin-top:8px;margin-bottom:8px}
.notes-content li{margin:4px 0}
.notes-content blockquote{border-left:3px solid #d0d7de;padding-left:12px;margin:8px 0;color:#6a737d}
.notes-content a{color:#0366d6;text-decoration:none}
.notes-content a:hover{text-decoration:underline}
.notes-content table{border-collapse:collapse;width:100%;margin:8px 0}
.notes-content table th,.notes-content table td{border:1px solid #d0d7de;padding:6px}
.notes-content table th{background:#f6f8fa;font-weight:600}
.hovercard{display:inline-block;position:relative}
.hovercard .label{cursor:help;padding:0 2px}
.hovercard .card{display:none;position:absolute;top:1.2em;left:0;z-index:9999;background:#fff;border:1px solid #d0d7de;box-shadow:0 8px 24px rgba(140,149,159,0.2);border-radius:6px;max-width:min(90vw,520px);max-height:50vh;overflow:auto;padding:8px}
.hovercard:hover .card{display:block}
</style>
</head><body>
<nav>
  <a href="/" class="{{if eq .Active "home"}}active{{end}}">Home</a>
  <a href="/next?html=1" class="{{if eq .Active "next"}}active{{end}}">Next</a>
  <a href="/open?html=1" class="{{if eq .Active "open"}}active{{end}}">Open</a>
  <a href="/active?html=1" class="{{if eq .Active "active"}}active{{end}}">Active</a>
  <a href="/paused?html=1" class="{{if eq .Active "paused"}}active{{end}}">Paused</a>
  <a href="/resolved?html=1" class="{{if eq .Active "resolved"}}active{{end}}">Resolved</a>
  <a href="/tags" class="{{if eq .Active "tags"}}active{{end}}">Tags</a>
  <a href="/projects" class="{{if eq .Active "projects"}}active{{end}}">Projects</a>
  <a href="/templates" class="{{if eq .Active "templates"}}active{{end}}">Templates</a>
  <a href="/context" class="{{if eq .Active "context"}}active{{end}}">Context</a>
  <a href="/tasks/new" class="{{if eq .Active "new"}}active{{end}}">New task</a>
  <a href="/tasks/action" class="{{if eq .Active "action"}}active{{end}}">Actions</a>
  <a href="/version" class="{{if eq .Active "version"}}active{{end}}">Version</a>
  <form method="post" action="/undo" style="display:inline;margin-left:8px;">
    <button type="submit" style="background:#f59e0b;color:#fff;border:none;padding:6px 10px;border-radius:4px;cursor:pointer;">Undo</button>
  </form>
</nav>
<div id="music-player" style="position:fixed;right:16px;bottom:16px;background:#fff;border:1px solid #d0d7de;border-radius:6px;padding:8px;box-shadow:0 8px 24px rgba(140,149,159,0.2);">
  <strong>Music</strong>
  <div style="margin-top:4px;display:flex;gap:6px;align-items:center;">
    <button id="mp-play" title="Play/Pause">▶️/⏸</button>
    <button id="mp-mute" title="Mute">🔇</button>
    <input id="mp-vol" type="range" min="0" max="1" step="0.01" value="0.8"/>
    <span id="mp-label" style="font-size:12px;color:#6a737d;max-width:220px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;">—</span>
  </div>
</div>
<script>
(function(){
  const audio = new Audio();
  audio.preload = 'auto';
  let current = null;
  const log = (...a)=>{ try{ console.log('[music]', ...a); } catch(e){} };
  const elPlay  = document.getElementById('mp-play');
  const elMute  = document.getElementById('mp-mute');
  const elVol   = document.getElementById('mp-vol');
  const elLabel = document.getElementById('mp-label');
  let persistCurrent = function(){ /* noop until stream active */ };

  function clampVolume(v){
    if (typeof v !== 'number' || isNaN(v)) { return null; }
    if (v < 0) v = 0;
    if (v > 1) v = 1;
    return v;
  }

  function volumeStorageKey(id){
    if (id) return 'dstask-music-vol:'+id;
    if (current && current.url) return 'dstask-music-url:'+current.url;
    return 'dstask-music-default';
  }

  function mutedStorageKey(id){
    return volumeStorageKey(id)+':muted';
  }

  function applyVolume(v, reason){
    const cv = clampVolume(typeof v === 'string' ? parseFloat(v) : v);
    if (cv === null) return;
    audio.volume = cv;
    if (elVol) { elVol.value = cv.toFixed(2); }
    if (current) {
      current.volume = cv;
      try { localStorage.setItem(volumeStorageKey(current.id), String(cv)); } catch(_){ }
    }
    log('volume applied', cv, reason||'');
  }

  function applyMuted(val){
    if (typeof val === 'boolean') {
      audio.muted = val;
      if (current) {
        current.muted = val;
        try { localStorage.setItem(mutedStorageKey(current.id), val ? '1' : '0'); } catch(_){ }
      }
      log('muted applied', val);
    }
  }

  function applyStoredMuted(id){
    try {
      const storedMuted = localStorage.getItem(mutedStorageKey(id));
      if (storedMuted != null) {
        applyMuted(storedMuted === '1');
        return true;
      }
    } catch(_){ }
    return false;
  }

  try {
    const baseVol = localStorage.getItem(volumeStorageKey(''));
    if (baseVol != null) {
      applyVolume(baseVol, 'init');
    } else if (elVol) {
      elVol.value = parseFloat(elVol.value || '0.8').toFixed(2);
    }
  } catch(_){ }

  function setSrcAndPlay(name, url, opts){
    opts = opts || {};
    current = {type:'radio', name, url, id: opts.id||'', volume: opts.vol, muted: opts.muted};
    if (elLabel) elLabel.textContent = name || url;
    // Immer über Proxy (mit Referer & Cache-Buster), da Quelle sonst 401/CORS liefern kann
    try {
      const orig = url;
      // best-guess Referer je nach Stream-Host
      let upstream;
      try { upstream = new URL(orig); } catch(_) {}
      let refererStr = window.location.href;
      if (upstream && /(^|\.)rndfnk\.com$/i.test(upstream.hostname)) {
        refererStr = 'https://www.deutschlandfunk.de/';
      }
      url = '/music/proxy?url=' + encodeURIComponent(orig) + '&referer=' + encodeURIComponent(refererStr) + '&_ts=' + Date.now();
    } catch(e) { /* ignore */ }
    log('set src', url);
    try { audio.pause(); } catch(e){}
    audio.src = url;
    // helper to persist current settings for this task
    persistCurrent = (reason)=>{
      if (!(current && current.id)) {
        log('persist skip', reason, 'no current id');
        return;
      }
      const payload = { type: 'radio', name: current.name||'', url: current.url||'', volume: (typeof audio.volume==='number'?audio.volume:undefined), muted: !!audio.muted };
      log('persist', reason, 'id=', current.id, payload);
      fetch('/music/tasks/' + encodeURIComponent(current.id), { method: 'PUT', headers: { 'Content-Type': 'application/json' }, credentials: 'same-origin', body: JSON.stringify(payload) })
        .then(()=>{
          log('persist ok');
          if (current) {
            if (typeof payload.volume === 'number') {
              current.volume = payload.volume;
              try { localStorage.setItem(volumeStorageKey(current.id), String(payload.volume)); } catch(_){ }
            }
            current.muted = !!payload.muted;
            try { localStorage.setItem(mutedStorageKey(current.id), payload.muted ? '1' : '0'); } catch(_){ }
          }
        })
        .catch(e=>log('persist failed', e));
    };

    // apply persisted volume/muted if provided
    let appliedVolume = false;
    if (typeof current.volume === 'number' && !isNaN(current.volume)) {
      applyVolume(current.volume, 'opts');
      appliedVolume = true;
    }
    if (!appliedVolume) {
      try {
        const stored = localStorage.getItem(volumeStorageKey(current.id));
        if (stored != null) {
          applyVolume(stored, 'localStorage');
          appliedVolume = true;
        }
      } catch(_){ }
    }
    if (!appliedVolume && current && current.id) {
      fetch('/music/tasks/' + encodeURIComponent(current.id), { credentials: 'same-origin' })
        .then(res=>{ if (!res.ok) throw new Error('status '+res.status); return res.json(); })
        .then(data=>{
          if (!(current && data && current.id === data.id)) return;
          if (typeof data.volume === 'number') {
            applyVolume(data.volume, 'fetch');
          }
          if (typeof data.muted === 'boolean') {
            applyMuted(data.muted);
          } else {
            applyStoredMuted(current.id);
          }
        })
        .catch(err=>log('volume fetch failed', err));
    } else {
      if (current) { applyStoredMuted(current.id); }
    }
    if (typeof current.muted !== 'boolean' && current) {
      applyStoredMuted(current.id);
    }
    if (typeof current.muted === 'boolean') {
      applyMuted(current.muted);
    }
    audio.load();
    // persist initial application so it's stored even ohne Slider-Interaktion
    persistCurrent('onStart');
    // try to play with muted first to satisfy autoplay
    audio.muted = true;
    const p = audio.play();
    if (p && typeof p.then === 'function') {
      p.then(()=>{
        log('play() resolved; unmuting soon');
        setTimeout(()=>{ audio.muted = false; log('unmuted'); }, 600);
      }).catch(err=>{
        log('play() blocked (autoplay?):', err);
        if (elLabel) elLabel.textContent = (name||url) + ' (click ▶️ to play)';
      });
    }
    // watchdog: if stuck in waiting, retry once
    let checks = 0;
    const iv = setInterval(()=>{
      checks++;
      log('state', 'network=', audio.networkState, 'ready=', audio.readyState);
      if (!audio.paused && audio.readyState >= 3) { // HAVE_FUTURE_DATA
        clearInterval(iv)
        return;
      }
      if (checks === 6 && audio.readyState < 2) { // after ~3s
        log('retry load() after waiting…');
        try { audio.load(); audio.play().catch(()=>{}); } catch(e){}
        clearInterval(iv);
      }
    }, 500);
  }

  // Controls
  elPlay && elPlay.addEventListener('click', ()=>{
    if (audio.paused) { log('play click'); audio.play().catch(e=>log('play rejected', e)); }
    else { log('pause click'); audio.pause(); }
  });
  elMute && elMute.addEventListener('click', ()=>{ audio.muted = !audio.muted; log('mute', audio.muted); persistCurrent('muteClick'); });
  let volSaveTimer = null;
  function onVolumeInput(){ const v = parseFloat(elVol.value||'0.8');
    applyVolume(v, 'slider');
    log('volume', v);
    if (volSaveTimer) clearTimeout(volSaveTimer);
    volSaveTimer = setTimeout(()=>{ persistCurrent('volumeChange'); }, 300);
  }
  elVol && elVol.addEventListener('input', onVolumeInput);
  elVol && elVol.addEventListener('change', onVolumeInput);

  // Media events
  audio.addEventListener('play',    ()=>log('event: play'));
  audio.addEventListener('pause',   ()=>log('event: pause'));
  audio.addEventListener('canplay', ()=>{ log('event: canplay'); if (audio.paused) { audio.play().catch(()=>{}); } });
  audio.addEventListener('playing', ()=>log('event: playing'));
  audio.addEventListener('waiting', ()=>{ log('event: waiting'); /* try nudging play */ if (audio.src) { audio.play().catch(()=>{}); } });
  audio.addEventListener('stalled', ()=>log('event: stalled'));
  audio.addEventListener('error',   ()=>{ const e=audio.error; log('event: error', e && (e.code+':'+e.message)); });
  audio.addEventListener('volumechange', ()=>{
    if (!current) return;
    try { localStorage.setItem(volumeStorageKey(current.id), String(audio.volume)); } catch(_){ }
    try { localStorage.setItem(mutedStorageKey(current.id), audio.muted ? '1' : '0'); } catch(_){ }
  });

  // API for server-driven start/stop
  window.dstaskMusic = {
    playRadio: (name, url, opts)=> setSrcAndPlay(name, url, opts),
    stop: ()=>{ log('stop'); try{ audio.pause(); }catch(e){}; audio.currentTime = 0; current = null; persistCurrent = function(){}; },
    setVolume: (v)=>{ applyVolume(parseFloat(v), 'api'); log('set volume', v); }
  };
})();
</script>
{{ template "content" . }}
{{ if .ShowCmdLog }}
<div class="cmdlog">
  <div class="hdr">
    <strong>Recent dstask commands</strong>
    <div>
      <a href="/__cmdlog?show=0&return={{.ReturnURL}}">Hide</a>
      {{if .CanShowMore}} | <a href="{{.MoreURL}}">Show more</a>{{end}}
    </div>
  </div>
  <pre>{{range .CmdEntries}}<span class="ts">{{.When}}</span> — <span class="ctx">{{.Context}}:</span> <span class="cmd">dstask {{.Args}}</span>
{{end}}</pre>
</div>
{{ else }}
<div class="cmdlog">
  <a href="/__cmdlog?show=1&return={{.ReturnURL}}">Show recent dstask commands</a>
</div>
{{ end }}
{{ if .Flash }}
<div class="flash {{.Flash.Type}}" style="margin:10px 0;padding:8px;border:1px solid #d0d7de; border-left-width:4px; background:#fff;">
  {{.Flash.Text}}
</div>
{{ end }}
<script>
// Read flash content to control music if present
(function(){
  const log = (...a)=>{ try{ console.log('[flash]', ...a); } catch(e){} };
  function tryProcessFlash(){
    var el = document.querySelector('.flash');
    if(!el) { log('no .flash element found'); return false; }
    var t = el.textContent || el.innerText || '';
    log('flash text:', t.substring(0, 200));
    var idx = t.indexOf('__MUSIC_START__');
    if(idx >= 0){
      var p = t.slice(idx).replace('__MUSIC_START__','');
      var i = p.indexOf('|');
      var name = '';
      var url = '';
      var id = '';
      var vol = undefined;
      var muted = undefined;
      if(i >= 0){
        name = p.slice(0,i).trim();
        var rest = p.slice(i+1).trim();
        // split rest on '|' for k=v pairs
        var parts = rest.split('|');
        url = (parts.shift()||'').trim();
        parts.forEach(function(seg){
          var eq = seg.indexOf('=');
          var k = eq>=0 ? seg.slice(0,eq) : seg;
          var v = eq>=0 ? seg.slice(eq+1) : '';
          k = (k||'').trim().toLowerCase(); v = (v||'').trim();
          if (k === 'id') id = v;
          else if (k === 'vol') { var f = parseFloat(v); if (!isNaN(f)) vol = f; }
          else if (k === 'muted') { muted = (v === '1' || v === 'true'); }
        });
      } else {
        // Fallback: finde URL-Beginn über http(s)://
        var h = p.indexOf('http://');
        if (h < 0) h = p.indexOf('https://');
        if (h >= 0){
          name = p.slice(0, h).replace(/[\s/\-]+$/,'').trim();
          url = p.slice(h).trim();
        }
      }
      // Remove trailing text after URL (e.g., "Task action applied")
      if(url) {
        var spaceIdx = url.indexOf(' ');
        if(spaceIdx > 0) url = url.slice(0, spaceIdx);
        var newlineIdx = url.indexOf('\n');
        if(newlineIdx > 0) url = url.slice(0, newlineIdx);
      }
      log('parsed name:', name, 'url:', url.substring(0, 100), 'id:', id, 'vol:', vol, 'muted:', muted);
      if(window.dstaskMusic && url){
        log('calling playRadio');
        window.dstaskMusic.playRadio(name, url, { id: id, vol: vol, muted: muted });
        return true;
      } else {
        log('dstaskMusic not available yet');
        return false;
      }
    }
    var idx2 = t.indexOf('__MUSIC_STOP__');
    if(idx2 >= 0){
      if(window.dstaskMusic){
        log('calling stop');
        window.dstaskMusic.stop();
        return true;
      } else {
        log('dstaskMusic not available for stop');
        return false;
      }
    }
    return false;
  }
  // Try immediately, then retry if dstaskMusic not ready
  if(!tryProcessFlash()){
    var retries = 0;
    var iv = setInterval(function(){
      retries++;
      if(tryProcessFlash() || retries >= 50){ // bis zu 5s warten
        clearInterval(iv);
      }
    }, 100);
  }
})();
</script>
</body></html>`))

	s.routes()
	return s
}

func (s *Server) routes() {
	// Favicon
	s.mux.HandleFunc("/favicon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
		_, _ = w.Write([]byte(faviconSVG))
	})
	s.mux.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/favicon.svg", http.StatusMovedPermanently)
	})
	// Music: search proxy to Radio Browser
	s.mux.HandleFunc("/music/search", func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if q == "" {
			http.Error(w, "missing q", http.StatusBadRequest)
			return
		}
		u := &url.URL{Scheme: "https", Host: "de2.api.radio-browser.info", Path: "/json/stations/search"}
		params := url.Values{}
		params.Set("name", q)
		params.Set("limit", "20")
		u.RawQuery = params.Encode()
		req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
		req.Header.Set("User-Agent", "dstask-web/0.1.5")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			http.Error(w, "radio browser error", http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.Copy(w, resp.Body)
	})
	// TODO(music): Lokale MP3-Wiedergabe später reaktivieren. Routen vorübergehend deaktiviert.
	/*
		// Music: playlist (M3U) for folder under user's HOME/.dstask scope
		s.mux.HandleFunc("/music/playlist", func(w http.ResponseWriter, r *http.Request) {
			...
		})
		// Minimal file serving for audio under HOME/.dstask
		s.mux.HandleFunc("/music/file", func(w http.ResponseWriter, r *http.Request) {
			...
		})
	*/

	// Simple streaming proxy to improve compatibility (e.g., AAC/MP3/ICY/CORS)
	s.mux.HandleFunc("/music/proxy", func(w http.ResponseWriter, r *http.Request) {
		// Robust extraction of the upstream URL: prefer full RawQuery tail after 'url='
		// This handles unencoded '&' inside the upstream URL parameters (token/sid/etc.).
		rq := r.URL.RawQuery
		raw := strings.TrimSpace(r.URL.Query().Get("url"))
		// If RawQuery tail was used previously, it may have included '&referer=' or other proxy params.
		// Prefer the explicit query param value; only fall back if empty.
		if raw == "" {
			if i := strings.Index(rq, "url="); i >= 0 {
				cand := rq[i+4:]
				// Trim at next '&' to avoid appending proxy params like '&referer=' or '&_ts='
				if j := strings.IndexByte(cand, '&'); j >= 0 {
					cand = cand[:j]
				}
				if d, err := url.QueryUnescape(cand); err == nil && d != "" {
					cand = d
				}
				raw = strings.TrimSpace(cand)
			}
		}
		if raw == "" {
			http.Error(w, "missing url", http.StatusBadRequest)
			return
		}
		// sanitize accidental token tailings (e.g., "|id=1" appended) from client flash parsing
		if k := strings.Index(raw, "|"); k >= 0 {
			raw = raw[:k]
		}
		if k := strings.Index(raw, "/id="); k >= 0 {
			raw = raw[:k]
		}
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			http.Error(w, "invalid url", http.StatusBadRequest)
			return
		}
		applog.Infof("/music/proxy fetch %s", raw)
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, raw, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// Generic UA
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; dstask-web)")
		req.Header.Set("Accept", "audio/*, */*;q=0.5")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Pragma", "no-cache")
		req.Header.Set("Accept-Encoding", "identity")
		if al := r.Header.Get("Accept-Language"); al != "" {
			req.Header.Set("Accept-Language", al)
		}
		if ua := r.Header.Get("User-Agent"); ua != "" {
			req.Header.Set("User-Agent", ua)
		}
		// try to preserve client IP for geo/token backends
		if xffVal := r.Header.Get("X-Forwarded-For"); xffVal != "" {
			req.Header.Set("X-Forwarded-For", xffVal)
		} else {
			host := r.RemoteAddr
			if i := strings.LastIndex(host, ":"); i > 0 {
				host = host[:i]
			}
			req.Header.Set("X-Forwarded-For", host)
		}
		// Optional Referer passthrough for token-gated streams
		referer := strings.TrimSpace(r.URL.Query().Get("referer"))
		if referer == "" {
			// Also try to extract from RawQuery if provided without encoding
			rq := r.URL.RawQuery
			if j := strings.Index(rq, "referer="); j >= 0 {
				ref := rq[j+8:]
				if d, err := url.QueryUnescape(ref); err == nil && d != "" {
					referer = d
				} else {
					referer = ref
				}
			}
		}
		if referer != "" {
			req.Header.Set("Referer", referer)
		}

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// Determine content type; fallback anhand Dateiendung
		ct := resp.Header.Get("Content-Type")
		if ct == "" {
			lp := strings.ToLower(u.Path)
			switch {
			case strings.Contains(lp, ".aac"):
				ct = "audio/aac"
			case strings.Contains(lp, ".m4a") || strings.Contains(lp, ".mp4"):
				ct = "audio/mp4"
			default:
				ct = "audio/mpeg"
			}
		}

		// Response-Header setzen; Content-Length entfernen, damit Chunked-Streaming genutzt wird
		w.Header().Set("Content-Type", ct)
		if v := resp.Header.Get("Ice-Audio-Info"); v != "" {
			w.Header().Set("Ice-Audio-Info", v)
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Del("Content-Length")
		w.WriteHeader(resp.StatusCode)

		// Chunked stream mit periodischem Flush
		var total int64
		if f, ok := w.(http.Flusher); ok {
			buf := make([]byte, 32*1024)
			bw := bufio.NewWriterSize(w, 64*1024)
			lastFlush := time.Now()
			bytesSinceFlush := 0
			for {
				n, er := resp.Body.Read(buf)
				if n > 0 {
					total += int64(n)
					if _, ew := bw.Write(buf[:n]); ew != nil {
						applog.Warnf("/music/proxy write error: %v (bytes_sent=%d)", ew, total)
						return
					}
					bytesSinceFlush += n
					if bytesSinceFlush >= 128*1024 || time.Since(lastFlush) >= 300*time.Millisecond {
						if err := bw.Flush(); err != nil {
							applog.Warnf("/music/proxy buffer flush error: %v (bytes_sent=%d)", err, total)
							return
						}
						f.Flush()
						bytesSinceFlush = 0
						lastFlush = time.Now()
					}
				}
				if er != nil {
					if er != io.EOF {
						applog.Warnf("/music/proxy stream error: %v (bytes_sent=%d)", er, total)
					}
					break
				}
			}
			if err := bw.Flush(); err != nil {
				applog.Warnf("/music/proxy final flush error: %v (bytes_sent=%d)", err, total)
			} else {
				f.Flush()
			}
		} else {
			n, err := io.Copy(w, resp.Body)
			total = n
			if err != nil {
				applog.Warnf("/music/proxy stream error: %v (bytes_sent=%d)", err, total)
			}
		}

		applog.Infof("/music/proxy done %s status=%d bytes_sent=%d", raw, resp.StatusCode, total)
	})
	// Batch actions
	s.mux.HandleFunc("/tasks/batch", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		// CSRF validation
		csrfToken := r.FormValue("csrf_token")
		if !validateCSRFToken(r, csrfToken) {
			s.setFlash(w, "error", "Invalid security token. Please refresh the page and try again.")
			http.Redirect(w, r, "/open?html=1", http.StatusSeeOther)
			return
		}
		ids := r.Form["ids"]
		action := strings.TrimSpace(r.FormValue("action"))
		note := strings.TrimSpace(r.FormValue("note"))
		if len(ids) == 0 || action == "" {
			http.Error(w, "ids/action required", http.StatusBadRequest)
			return
		}
		username, _ := auth.UsernameFromRequest(r)
		var ok, skipped, failed int
		for _, id := range ids {
			id = strings.TrimSpace(id)
			if id == "" {
				continue
			}
			var res dstask.Result
			switch action {
			case "start", "stop", "done", "remove", "log":
				res = s.runner.Run(username, 10*time.Second, action, id)
			case "note":
				if note == "" {
					skipped++
					continue
				}
				res = s.runner.Run(username, 10*time.Second, "note", id, note)
			default:
				skipped++
				continue
			}
			if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
				failed++
			} else {
				ok++
			}
		}
		msg := fmt.Sprintf("Batch %s: %d ok, %d skipped, %d failed", action, ok, skipped, failed)
		s.setFlash(w, "info", msg)
		http.Redirect(w, r, "/open?html=1", http.StatusSeeOther)
	})
	// Toggle command log visibility via cookie
	s.mux.HandleFunc("/__cmdlog", func(w http.ResponseWriter, r *http.Request) {
		show := r.URL.Query().Get("show")
		ret := r.URL.Query().Get("return")
		if show == "0" {
			http.SetCookie(w, &http.Cookie{Name: "cmdlog", Value: "off", Path: "/", MaxAge: 86400 * 365})
		} else if show == "1" {
			http.SetCookie(w, &http.Cookie{Name: "cmdlog", Value: "on", Path: "/", MaxAge: 86400 * 365})
		}
		if ret == "" {
			ret = "/"
		}
		http.Redirect(w, r, ret, http.StatusSeeOther)
	})
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})
	// Music map CRUD
	s.mux.HandleFunc("/music/map", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		switch r.Method {
		case http.MethodGet:
			m, _, err := music.LoadForUser(s.cfg, username)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, m)
		case http.MethodPut:
			var m music.Map
			if err := jsonNewDecoder(r).Decode(&m); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if m.Version == 0 {
				m.Version = 1
			}
			if m.Tasks == nil {
				m.Tasks = map[string]music.TaskMusic{}
			}
			if _, err := music.SaveForUser(s.cfg, username, &m); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]any{"ok": true})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	s.mux.HandleFunc("/music/tasks/", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		id := strings.TrimPrefix(r.URL.Path, "/music/tasks/")
		if id == "" {
			http.Error(w, "missing id", http.StatusBadRequest)
			return
		}
		m, path, err := music.LoadForUser(s.cfg, username)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		switch r.Method {
		case http.MethodGet:
			if m.Tasks == nil {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			tm, ok := m.Tasks[id]
			if !ok {
				http.Error(w, "not found", http.StatusNotFound)
				return
			}
			writeJSON(w, map[string]any{
				"id":     id,
				"type":   tm.Type,
				"name":   tm.Name,
				"url":    tm.URL,
				"path":   tm.Path,
				"volume": tm.Volume,
				"muted":  tm.Muted,
			})
		case http.MethodPut:
			var tm music.TaskMusic
			if err := jsonNewDecoder(r).Decode(&tm); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			if m.Tasks == nil {
				m.Tasks = map[string]music.TaskMusic{}
			}
			m.Tasks[id] = tm
			if _, err := music.SaveForUser(s.cfg, username, m); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]any{"ok": true, "path": path})
		case http.MethodDelete:
			if m.Tasks != nil {
				delete(m.Tasks, id)
			}
			if _, err := music.SaveForUser(s.cfg, username, m); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]any{"ok": true, "path": path})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t := template.Must(s.layoutTpl.Clone())
		_, _ = t.New("content").Parse(`<h1>dstask Web UI</h1><p>Signed in as: {{.User}}</p>
{{if .IsGitRepo}}
  {{if .RemoteURL}}
    <div style="margin:8px 0;">Remote: <code>{{.RemoteURL}}</code></div>
    <form method="post" action="/sync" style="margin-top:8px"><button type="submit">Sync</button></form>
  {{else}}
    <div style="margin:8px 0;background:#fff3cd;border:1px solid #ffeeba;padding:8px;">Kein Git-Remote konfiguriert. Sync erfordert ein Remote-Repository.</div>
    <form method="post" action="/sync" style="margin-top:8px"><button type="submit">Sync…</button></form>
    <form method="post" action="/sync/set-remote" style="display:inline;margin-left:8px;">
      <input name="url" placeholder="https://... oder git@..." style="width:50%" required />
      <button type="submit">Remote speichern</button>
      <a href="/" style="margin-left:8px;">abbrechen</a>
    </form>
  {{end}}
{{else}}
  <div style="margin:8px 0;background:#fff3cd;border:1px solid #ffeeba;padding:8px;">Kein Git-Repository im .dstask-Verzeichnis. Du kannst ein Remote hier klonen.<br/><small>Verwendetes lokales Verzeichnis: <code>{{.RepoDir}}</code></small></div>
  <form method="post" action="/sync/clone-remote">
    <input name="url" placeholder="https://... oder git@..." style="width:50%" required />
    <button type="submit">Remote klonen</button>
    <a href="/" style="margin-left:8px;">abbrechen</a>
  </form>
{{end}}`) // placeholder
		username, _ := auth.UsernameFromRequest(r)
		remoteURL, _ := s.runner.GitRemoteURL(username)
		// Prüfe Git-Repo vorhanden
		isRepo := false
		repoDir := ""
		// Primär aus Konfiguration (repos) ableiten
		if home, ok := config.ResolveHomeForUsername(s.cfg, username); ok && home != "" {
			dir := home
			if !strings.HasSuffix(strings.ToLower(dir), ".dstask") {
				dir = dir + string('/') + ".dstask"
			}
			repoDir = dir
			if fi, err := os.Stat(dir + string('/') + ".git"); err == nil && fi.IsDir() {
				isRepo = true
			}
		} else {
			// Fallback: Prozess-HOME verwenden, um dem Nutzer den erwarteten Pfad anzuzeigen
			if h, err := os.UserHomeDir(); err == nil && h != "" {
				d := h
				if !strings.HasSuffix(strings.ToLower(d), ".dstask") {
					d = d + string('/') + ".dstask"
				}
				repoDir = d
				if fi, err := os.Stat(d + string('/') + ".git"); err == nil && fi.IsDir() {
					isRepo = true
				}
			}
		}
		show, entries, moreURL, canMore, ret := s.footerData(r, username)
		_ = t.Execute(w, map[string]any{
			"User":        username,
			"RemoteURL":   remoteURL,
			"IsGitRepo":   isRepo,
			"RepoDir":     repoDir,
			"Active":      activeFromPath(r.URL.Path),
			"Flash":       s.getFlash(r),
			"ShowCmdLog":  show,
			"CmdEntries":  entries,
			"MoreURL":     moreURL,
			"CanShowMore": canMore,
			"ReturnURL":   ret,
		})
	})

	s.mux.HandleFunc("/next", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		s.cmdStore.Append(username, "List next tasks", []string{"next"})
		if r.URL.Query().Get("html") == "1" {
			exp := s.runner.Run(username, 5_000_000_000, "export")
			if exp.Err == nil && exp.ExitCode == 0 && !exp.TimedOut {
				if tasks, ok := decodeTasksJSONFlexible(exp.Stdout); ok && len(tasks) > 0 {
					rows := buildRowsFromTasks(tasks, "")
					rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
					dueFilter := buildDueFilterToken(r.URL.Query())
					rows = applyDueFilter(rows, dueFilter)
					if len(rows) > 0 {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						s.renderExportTable(w, r, "Next", rows)
						return
					}
				}
			}
			res := s.runner.Run(username, 5_000_000_000, "next")
			if res.Err != nil && !res.TimedOut {
				http.Error(w, res.Stderr, http.StatusBadGateway)
				return
			}
			// Versuch: JSON direkt aus next-Stdout extrahieren
			if tasks2, ok := decodeTasksJSONFlexible(res.Stdout); ok && len(tasks2) > 0 {
				rows := buildRowsFromTasks(tasks2, "")
				rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
				dueFilter := buildDueFilterToken(r.URL.Query())
				rows = applyDueFilter(rows, dueFilter)
				if len(rows) > 0 {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					s.renderExportTable(w, r, "Next", rows)
					return
				}
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			s.renderListHTML(w, r, "Next", res.Stdout)
			return
		}
		res := s.runner.Run(username, 5_000_000_000, "next")
		if res.Err != nil && !res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(res.Stdout))
	})

	s.mux.HandleFunc("/open", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		s.cmdStore.Append(username, "List open tasks", []string{"show-open"})
		if r.URL.Query().Get("html") == "1" {
			// Primär: export rohen JSON-Text holen und parsen (robuster, da wir Json sehen)
			exp := s.runner.Run(username, 5_000_000_000, "export")
			if exp.Err == nil && exp.ExitCode == 0 && !exp.TimedOut {
				if tasks, ok := decodeTasksJSON(exp.Stdout); ok && len(tasks) > 0 {
					rows := make([]map[string]string, 0, len(tasks))
					for _, t := range tasks {
						// Zeige alle offenen und aktiven; resolved werden unten ggf. herausgefiltert
						id := str(firstOf(t, "id", "ID", "Id", "uuid", "UUID"))
						if id == "" {
							continue
						}
						rows = append(rows, map[string]string{
							"id":       id,
							"status":   str(firstOf(t, "status", "state")),
							"summary":  trimQuotes(str(firstOf(t, "summary", "Summary", "description", "Description"))),
							"project":  trimQuotes(str(firstOf(t, "project", "Project"))),
							"priority": str(firstOf(t, "priority", "Priority")),
							"due":      trimQuotes(str(firstOf(t, "due", "Due", "dueDate", "DueDate"))),
							"created":  trimQuotes(str(firstOf(t, "created", "Created"))),
							"resolved": trimQuotes(str(firstOf(t, "resolved", "Resolved"))),
							"age":      ageInDays(trimQuotes(str(firstOf(t, "created", "Created")))),
							"tags":     joinTags(firstOf(t, "tags", "Tags")),
							"notes":    trimQuotes(str(firstOf(t, "notes", "annotations", "note"))),
						})
					}
					dueFilter := buildDueFilterToken(r.URL.Query())
					rows = applyDueFilter(rows, dueFilter)
					if len(rows) > 0 {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						s.renderExportTable(w, r, "Open", rows)
						return
					}
				} else {
					// Loose Parser über den Rohtext
					rows := parseTasksLooseFromJSONText(exp.Stdout)
					rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
					dueFilter := buildDueFilterToken(r.URL.Query())
					rows = applyDueFilter(rows, dueFilter)
					if len(rows) > 0 {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						s.renderExportTable(w, r, "Open", rows)
						return
					}
				}
			}
			// Fallback: Plaintext parsen und als Tabelle rendern
			res := s.runner.Run(username, 5_000_000_000, "show-open")
			if res.Err != nil && !res.TimedOut {
				http.Error(w, res.Stderr, http.StatusBadGateway)
				return
			}
			// Versuche zuerst JSON aus show-open zu extrahieren (manche Builds geben JSON aus)
			if tasks2, ok := decodeTasksJSON(res.Stdout); ok && len(tasks2) > 0 {
				rows := make([]map[string]string, 0, len(tasks2))
				for _, t := range tasks2 {
					id := str(firstOf(t, "id", "ID", "uuid"))
					if id == "" {
						continue
					}
					rows = append(rows, map[string]string{
						"id":       id,
						"status":   str(firstOf(t, "status", "state")),
						"summary":  trimQuotes(str(firstOf(t, "summary", "description"))),
						"project":  trimQuotes(str(firstOf(t, "project"))),
						"priority": str(firstOf(t, "priority")),
						"due":      trimQuotes(str(firstOf(t, "due"))),
						"created":  trimQuotes(str(firstOf(t, "created"))),
						"resolved": trimQuotes(str(firstOf(t, "resolved"))),
						"age":      ageInDays(trimQuotes(str(firstOf(t, "created")))),
						"tags":     joinTags(firstOf(t, "tags")),
						"notes":    trimQuotes(str(firstOf(t, "notes", "annotations", "note"))),
					})
				}
				rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
				dueFilter := buildDueFilterToken(r.URL.Query())
				rows = applyDueFilter(rows, dueFilter)
				if len(rows) > 0 {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					s.renderExportTable(w, r, "Open", rows)
					return
				}
			}
			rows := parseOpenPlain(res.Stdout)
			rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
			dueFilter := buildDueFilterToken(r.URL.Query())
			rows = applyDueFilter(rows, dueFilter)
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if len(rows) > 0 {
				s.renderExportTable(w, r, "Open", rows)
			} else {
				ok := r.URL.Query().Get("ok") != ""
				// reuse list renderer with Ok flag
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				t := template.Must(s.layoutTpl.Clone())
				_, _ = t.New("content").Parse(`<h2>Open</h2>{{if .Ok}}<div style="background:#d4edda;border:1px solid #c3e6cb;color:#155724;padding:8px;margin-bottom:8px;">Action successful</div>{{end}}<pre style="white-space: pre-wrap;">{{.Body}}</pre>`)
				_ = t.Execute(w, map[string]any{"Ok": ok, "Body": res.Stdout, "Active": activeFromPath(r.URL.Path)})
			}
			return
		}
		// Plaintext
		res := s.runner.Run(username, 5_000_000_000, "show-open")
		if res.Err != nil && !res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(res.Stdout))
	})

	s.mux.HandleFunc("/active", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		s.cmdStore.Append(username, "List active tasks", []string{"show-active"})
		if r.URL.Query().Get("html") == "1" {
			exp := s.runner.Run(username, 5_000_000_000, "export")
			if exp.Err == nil && exp.ExitCode == 0 && !exp.TimedOut {
				if tasks, ok := decodeTasksJSONFlexible(exp.Stdout); ok && len(tasks) > 0 {
					rows := buildRowsFromTasks(tasks, "active")
					rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
					dueFilter := buildDueFilterToken(r.URL.Query())
					rows = applyDueFilter(rows, dueFilter)
					if len(rows) > 0 {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						s.renderExportTable(w, r, "Active", rows)
						return
					}
				}
			}
			res := s.runner.Run(username, 5_000_000_000, "show-active")
			if res.Err != nil && !res.TimedOut {
				http.Error(w, res.Stderr, http.StatusBadGateway)
				return
			}
			// Versuch: JSON direkt aus show-active-Stdout extrahieren
			if tasks2, ok := decodeTasksJSONFlexible(res.Stdout); ok && len(tasks2) > 0 {
				rows := buildRowsFromTasks(tasks2, "active")
				rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
				dueFilter := buildDueFilterToken(r.URL.Query())
				rows = applyDueFilter(rows, dueFilter)
				if len(rows) > 0 {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					s.renderExportTable(w, r, "Active", rows)
					return
				}
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			s.renderListHTML(w, r, "Active", res.Stdout)
			return
		}
		res := s.runner.Run(username, 5_000_000_000, "show-active")
		if res.Err != nil && !res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(res.Stdout))
	})

	s.mux.HandleFunc("/paused", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		s.cmdStore.Append(username, "List paused tasks", []string{"show-paused"})
		if r.URL.Query().Get("html") == "1" {
			exp := s.runner.Run(username, 5_000_000_000, "export")
			if exp.Err == nil && exp.ExitCode == 0 && !exp.TimedOut {
				if tasks, ok := decodeTasksJSON(exp.Stdout); ok && len(tasks) > 0 {
					rows := buildRowsFromTasks(tasks, "paused")
					rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
					dueFilter := buildDueFilterToken(r.URL.Query())
					rows = applyDueFilter(rows, dueFilter)
					if len(rows) > 0 {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						s.renderExportTable(w, r, "Paused", rows)
						return
					}
				}
			}
			res := s.runner.Run(username, 5_000_000_000, "show-paused")
			if res.Err != nil && !res.TimedOut {
				http.Error(w, res.Stderr, http.StatusBadGateway)
				return
			}
			// Versuch: JSON direkt aus show-paused-Stdout extrahieren
			if tasks2, ok := decodeTasksJSONFlexible(res.Stdout); ok && len(tasks2) > 0 {
				rows := buildRowsFromTasks(tasks2, "paused")
				rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
				dueFilter := buildDueFilterToken(r.URL.Query())
				rows = applyDueFilter(rows, dueFilter)
				if len(rows) > 0 {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					s.renderExportTable(w, r, "Paused", rows)
					return
				}
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			s.renderListHTML(w, r, "Paused", res.Stdout)
			return
		}
		res := s.runner.Run(username, 5_000_000_000, "show-paused")
		if res.Err != nil && !res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(res.Stdout))
	})

	s.mux.HandleFunc("/resolved", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		s.cmdStore.Append(username, "List resolved tasks", []string{"show-resolved"})
		if r.URL.Query().Get("html") == "1" {
			exp := s.runner.Run(username, 5_000_000_000, "export")
			if exp.Err == nil && exp.ExitCode == 0 && !exp.TimedOut {
				if tasks, ok := decodeTasksJSON(exp.Stdout); ok && len(tasks) > 0 {
					rows := buildRowsFromTasks(tasks, "resolved")
					rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
					if len(rows) > 0 {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						s.renderExportTable(w, r, "Resolved", rows)
						return
					}
				}
			}
			res := s.runner.Run(username, 5_000_000_000, "show-resolved")
			if res.Err != nil && !res.TimedOut {
				http.Error(w, res.Stderr, http.StatusBadGateway)
				return
			}
			// Versuch: JSON direkt aus show-resolved-Stdout extrahieren
			if tasks2, ok := decodeTasksJSONFlexible(res.Stdout); ok && len(tasks2) > 0 {
				rows := buildRowsFromTasks(tasks2, "resolved")
				rows = applyQueryFilter(rows, r.URL.Query().Get("q"))
				if len(rows) > 0 {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					s.renderExportTable(w, r, "Resolved", rows)
					return
				}
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			s.renderListHTML(w, r, "Resolved", res.Stdout)
			return
		}
		res := s.runner.Run(username, 5_000_000_000, "show-resolved")
		if res.Err != nil && !res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte(res.Stdout))
	})

	s.mux.HandleFunc("/tags", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		res := s.runner.Run(username, 5_000_000_000, "show-tags")
		s.cmdStore.Append(username, "List tags", []string{"show-tags"})
		if res.Err != nil && !res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadGateway)
			return
		}
		if r.URL.Query().Get("raw") != "1" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			t := template.Must(s.layoutTpl.Clone())
			_, _ = t.New("content").Parse(`<h2>Tags</h2>
<pre style="white-space:pre-wrap;">{{.Out}}</pre>`)
			uname, _ := auth.UsernameFromRequest(r)
			show, entries, moreURL, canMore, ret := s.footerData(r, uname)
			_ = t.Execute(w, map[string]any{
				"Out":         strings.TrimSpace(res.Stdout),
				"Active":      activeFromPath(r.URL.Path),
				"Flash":       s.getFlash(r),
				"ShowCmdLog":  show,
				"CmdEntries":  entries,
				"MoreURL":     moreURL,
				"CanShowMore": canMore,
				"ReturnURL":   ret,
			})
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		out := strings.TrimSpace(res.Stdout)
		if out == "" {
			out = "Keine Tags vorhanden"
		}
		_, _ = w.Write([]byte(out))
	})

	s.mux.HandleFunc("/projects", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		res := s.runner.Run(username, 5_000_000_000, "show-projects")
		s.cmdStore.Append(username, "List projects", []string{"show-projects"})
		if res.Err != nil && !res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadGateway)
			return
		}
		if r.URL.Query().Get("raw") != "1" {
			// Versuche JSON zu erkennen und als Tabelle zu rendern
			if arr, ok := decodeTasksJSONFlexible(res.Stdout); ok && len(arr) > 0 {
				rows := make([]map[string]string, 0, len(arr))
				for _, m := range arr {
					name := trimQuotes(str(firstOf(m, "name", "project")))
					if name == "" {
						continue
					}
					rows = append(rows, map[string]string{
						"name":          name,
						"taskCount":     str(firstOf(m, "taskCount")),
						"resolvedCount": str(firstOf(m, "resolvedCount")),
						"active":        str(firstOf(m, "active")),
						"priority":      str(firstOf(m, "priority")),
					})
				}
				if len(rows) > 0 {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					s.renderProjectsTable(w, r, "Projects", rows)
					return
				}
			}
			// Fallback: HTML mit Pre
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			t := template.Must(s.layoutTpl.Clone())
			_, _ = t.New("content").Parse(`<h2>Projects</h2>
<pre style="white-space:pre-wrap;">{{.Out}}</pre>`)
			uname, _ := auth.UsernameFromRequest(r)
			show, entries, moreURL, canMore, ret := s.footerData(r, uname)
			_ = t.Execute(w, map[string]any{
				"Out":         strings.TrimSpace(res.Stdout),
				"Active":      activeFromPath(r.URL.Path),
				"Flash":       s.getFlash(r),
				"ShowCmdLog":  show,
				"CmdEntries":  entries,
				"MoreURL":     moreURL,
				"CanShowMore": canMore,
				"ReturnURL":   ret,
			})
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		out := strings.TrimSpace(res.Stdout)
		if out == "" {
			out = "Keine Projekte vorhanden"
		}
		_, _ = w.Write([]byte(out))
	})

	// Templates anzeigen
	s.mux.HandleFunc("/templates", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			// POST: Template erstellen
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			summary := strings.TrimSpace(r.FormValue("summary"))
			if summary == "" {
				http.Error(w, "summary required", http.StatusBadRequest)
				return
			}
			username, _ := auth.UsernameFromRequest(r)
			tags := strings.TrimSpace(r.FormValue("tags"))
			project := strings.TrimSpace(r.FormValue("project"))
			// Prefer selected existing project if provided
			if ps := strings.TrimSpace(r.FormValue("projectSelect")); ps != "" {
				project = ps
			}
			// Due: prefer date picker if present
			dueDate := strings.TrimSpace(r.FormValue("dueDate"))
			due := sanitizeDueValue(r.FormValue("due"))
			if dueDate != "" {
				due = sanitizeDueValue(dueDate)
			}

			// Compose args per dstask template Syntax: template <summary tokens...> +tags project: due:
			args := []string{"template"}
			args = append(args, summaryTokens(summary)...)
			// Collect tags from existing checkboxes
			for _, t := range r.Form["tagsExisting"] {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				t = normalizeTag(t)
				if !strings.HasPrefix(t, "+") {
					args = append(args, "+"+t)
				} else {
					args = append(args, t)
				}
			}
			if tags != "" {
				for _, t := range strings.Split(tags, ",") {
					t = strings.TrimSpace(t)
					if t == "" {
						continue
					}
					t = normalizeTag(t)
					if !strings.HasPrefix(t, "+") {
						args = append(args, "+"+t)
					} else {
						args = append(args, t)
					}
				}
			}
			if project != "" {
				args = append(args, "project:"+project)
			}
			if due != "" {
				args = append(args, "due:"+quoteIfNeeded(due))
			}

			res := s.runner.Run(username, 10_000_000_000, args...) // 10s
			s.cmdStore.Append(username, "Create template", args)
			if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
				applog.Warnf("template creation failed: code=%d timeout=%v err=%v", res.ExitCode, res.TimedOut, res.Err)
				s.setFlash(w, "error", "Template creation failed: "+res.Stderr)
				http.Redirect(w, r, "/templates/new", http.StatusSeeOther)
				return
			}
			applog.Infof("template created successfully")
			s.setFlash(w, "success", "Template created successfully")
			http.Redirect(w, r, "/templates", http.StatusSeeOther)
			return
		}

		// GET: Templates anzeigen
		username, _ := auth.UsernameFromRequest(r)
		res := s.runner.Run(username, 5_000_000_000, "show-templates")
		s.cmdStore.Append(username, "List templates", []string{"show-templates"})
		if res.Err != nil && !res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadGateway)
			return
		}
		templates := parseTemplatesFromOutput(res.Stdout)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t := template.Must(s.layoutTpl.Clone())
		_, _ = t.New("content").Parse(`
<h2>Templates <a href="/templates/new" style="font-size:14px;font-weight:normal;margin-left:8px;">(New template)</a></h2>
{{if .Templates}}
<table border="1" cellpadding="4" cellspacing="0">
  <thead><tr>
    <th style="width:64px;">ID</th>
    <th>Summary</th>
    <th>Project</th>
    <th>Tags</th>
    <th style="width:200px;">Actions</th>
  </tr></thead>
  <tbody>
  {{range .Templates}}
    <tr>
      <td>{{index . "id"}}</td>
      <td><pre style="margin:0;white-space:pre-wrap;">{{index . "summary"}}</pre></td>
      <td>{{index . "project"}}</td>
      <td>{{index . "tags"}}</td>
      <td>
        <form method="get" action="/tasks/new" style="display:inline">
          <input type="hidden" name="template" value="{{index . "id"}}" />
          <button type="submit">use</button>
        </form>
         · <form method="get" action="/templates/{{index . "id"}}/edit" style="display:inline">
           <button type="submit">edit</button>
         </form>
         · <form method="post" action="/templates/{{index . "id"}}/delete" style="display:inline" onsubmit="return confirm('Delete this template?');">
           <button type="submit">delete</button>
         </form>
      </td>
    </tr>
  {{end}}
  </tbody>
</table>
{{else}}
<p>No templates found.</p>
{{end}}
`)
		uname, _ := auth.UsernameFromRequest(r)
		show, entries, moreURL, canMore, ret := s.footerData(r, uname)
		_ = t.Execute(w, map[string]any{
			"Templates":   templates,
			"Active":      activeFromPath(r.URL.Path),
			"Flash":       s.getFlash(r),
			"ShowCmdLog":  show,
			"CmdEntries":  entries,
			"MoreURL":     moreURL,
			"CanShowMore": canMore,
			"ReturnURL":   ret,
		})
	})

	// Template erstellen (Form)
	s.mux.HandleFunc("/templates/new", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		// Fetch existing projects and tags
		projRes := s.runner.Run(username, 5_000_000_000, "show-projects")
		tagRes := s.runner.Run(username, 5_000_000_000, "show-tags")
		projects := parseProjectsFromOutput(projRes.Stdout)
		tags := parseTagsFromOutput(tagRes.Stdout)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t := template.Must(s.layoutTpl.Clone())
		_, _ = t.New("content").Parse(`
<h2>New template</h2>
<form method="post" action="/templates">
  <div><label>Summary: <input name="summary" required style="width:60%"></label></div>
  <div>
    <label>Project:</label>
    <select name="projectSelect">
      <option value="">(none)</option>
      {{range .Projects}}<option value="{{.}}">{{.}}</option>{{end}}
    </select>
    <span style="margin:0 8px;">or</span>
    <input name="project" placeholder="new project" />
  </div>
  <div>
    <label>Tags:</label>
    <div style="max-height:140px;overflow:auto;border:1px solid #ddd;padding:6px;">
      {{range .Tags}}<label style="display:inline-block;margin-right:8px;">
        <input type="checkbox" name="tagsExisting" value="{{.}}"/> {{.}}
      </label>{{end}}
    </div>
    <div style="margin-top:6px;">
      <label>Add tags (comma-separated): <input name="tags"></label>
    </div>
  </div>
  <div>
    <label>Due:</label>
    <input type="date" name="dueDate" />
    <span style="margin:0 8px;">or</span>
    <input name="due" placeholder="e.g. friday / 2025-12-31" />
  </div>
  <div style="margin-top:8px;">
    <button type="submit">Create template</button>
    <form method="get" action="/templates" style="display:inline;margin-left:8px;">
      <button type="submit">cancel</button>
    </form>
  </div>
 </form>
        `)
		uname, _ := auth.UsernameFromRequest(r)
		show, entries, moreURL, canMore, ret := s.footerData(r, uname)
		_ = t.Execute(w, map[string]any{
			"Active":      activeFromPath(r.URL.Path),
			"Projects":    projects,
			"Tags":        tags,
			"ShowCmdLog":  show,
			"CmdEntries":  entries,
			"MoreURL":     moreURL,
			"CanShowMore": canMore,
			"ReturnURL":   ret,
		})
	})

	// Template bearbeiten (Form)
	s.mux.HandleFunc("/templates/", func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		if len(parts) < 3 || parts[0] != "templates" {
			http.NotFound(w, r)
			return
		}
		templateID := parts[1]
		action := parts[2]

		username, _ := auth.UsernameFromRequest(r)

		// GET /templates/{id}/edit - Bearbeitungsformular anzeigen
		if action == "edit" && r.Method == http.MethodGet {
			// Hole aktuelles Template
			res := s.runner.Run(username, 5_000_000_000, "show-templates")
			if res.Err != nil && !res.TimedOut {
				http.Error(w, res.Stderr, http.StatusBadGateway)
				return
			}
			templates := parseTemplatesFromOutput(res.Stdout)
			var currentTemplate map[string]string
			for _, t := range templates {
				if t["id"] == templateID {
					currentTemplate = t
					break
				}
			}
			if currentTemplate == nil {
				s.setFlash(w, "error", "Template not found")
				http.Redirect(w, r, "/templates", http.StatusSeeOther)
				return
			}

			// Fetch existing projects and tags
			projRes := s.runner.Run(username, 5_000_000_000, "show-projects")
			tagRes := s.runner.Run(username, 5_000_000_000, "show-tags")
			projects := parseProjectsFromOutput(projRes.Stdout)
			tags := parseTagsFromOutput(tagRes.Stdout)

			// Parse existing tags from template
			existingTags := make(map[string]bool)
			if tagStr := currentTemplate["tags"]; tagStr != "" {
				for _, tag := range strings.Split(tagStr, ", ") {
					tag = strings.TrimSpace(tag)
					if tag != "" {
						existingTags[tag] = true
					}
				}
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			t := template.Must(s.layoutTpl.Clone())
			_, _ = t.New("content").Parse(`
<h2>Edit template #{{.TemplateID}}</h2>
<form method="post" action="/templates/{{.TemplateID}}/edit">
  <div><label>Summary: <input name="summary" value="{{.Summary}}" required style="width:60%"></label></div>
  <div>
    <label>Project:</label>
    <select name="projectSelect">
      <option value="">(none)</option>
      {{range .Projects}}
        <option value="{{.}}" {{if eq $.Project .}}selected{{end}}>{{.}}</option>
      {{end}}
    </select>
    <span style="margin:0 8px;">or</span>
    <input name="project" value="{{if not .HasProjectInSelect}}{{.Project}}{{end}}" placeholder="new project" />
  </div>
  <div>
    <label>Tags:</label>
    <div style="max-height:140px;overflow:auto;border:1px solid #ddd;padding:6px;">
      {{range .Tags}}
        <label style="display:inline-block;margin-right:8px;">
          <input type="checkbox" name="tagsExisting" value="{{.}}" {{if index $.ExistingTags .}}checked{{end}}/> {{.}}
        </label>
      {{end}}
    </div>
    <div style="margin-top:6px;">
      <label>Add tags (comma-separated): <input name="tags" placeholder="additional tags"></label>
    </div>
  </div>
  <div>
    <label>Due:</label>
    <input type="date" name="dueDate" value="{{.DueDate}}" />
    <span style="margin:0 8px;">or</span>
    <input name="due" value="{{.Due}}" placeholder="e.g. friday / 2025-12-31" />
  </div>
  <div style="margin-top:8px;">
    <button type="submit">Update template</button>
    <form method="get" action="/templates" style="display:inline;margin-left:8px;">
      <button type="submit">cancel</button>
    </form>
  </div>
</form>
`)
			uname, _ := auth.UsernameFromRequest(r)
			show, entries, moreURL, canMore, ret := s.footerData(r, uname)

			// Parse due date to YYYY-MM-DD format if it's a date
			dueDateValue := ""
			dueValue := sanitizeDueValue(currentTemplate["due"])
			if dueValue != "" {
				if t := parseTimeOrZero(dueValue); !t.IsZero() {
					dueDateValue = t.Format("2006-01-02")
					dueValue = "" // Clear due field if we can parse it as date
				}
			}

			hasProjectInSelect := false
			for _, p := range projects {
				if p == currentTemplate["project"] {
					hasProjectInSelect = true
					break
				}
			}

			_ = t.Execute(w, map[string]any{
				"TemplateID":         templateID,
				"Summary":            currentTemplate["summary"],
				"Project":            currentTemplate["project"],
				"HasProjectInSelect": hasProjectInSelect,
				"DueDate":            dueDateValue,
				"Due":                dueValue,
				"Projects":           projects,
				"Tags":               tags,
				"ExistingTags":       existingTags,
				"Active":             activeFromPath(r.URL.Path),
				"ShowCmdLog":         show,
				"CmdEntries":         entries,
				"MoreURL":            moreURL,
				"CanShowMore":        canMore,
				"ReturnURL":          ret,
			})
			return
		}

		// POST /templates/{id}/edit - Template aktualisieren
		if action == "edit" && r.Method == http.MethodPost {
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			summary := strings.TrimSpace(r.FormValue("summary"))
			if summary == "" {
				http.Error(w, "summary required", http.StatusBadRequest)
				return
			}

			tags := strings.TrimSpace(r.FormValue("tags"))
			project := strings.TrimSpace(r.FormValue("project"))
			if ps := strings.TrimSpace(r.FormValue("projectSelect")); ps != "" {
				project = ps
			}
			dueDate := strings.TrimSpace(r.FormValue("dueDate"))
			due := sanitizeDueValue(r.FormValue("due"))
			if dueDate != "" {
				due = sanitizeDueValue(dueDate)
			}

			// Lösche altes Template
			delRes := s.runner.Run(username, 5_000_000_000, "delete", templateID)
			if delRes.Err != nil && !delRes.TimedOut {
				applog.Warnf("template delete failed: %v", delRes.Err)
				// Continue anyway - maybe template doesn't exist or already deleted
			}

			// Erstelle neues Template mit aktualisierten Werten
			args := []string{"template"}
			args = append(args, summaryTokens(summary)...)
			for _, t := range r.Form["tagsExisting"] {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				t = normalizeTag(t)
				if !strings.HasPrefix(t, "+") {
					args = append(args, "+"+t)
				} else {
					args = append(args, t)
				}
			}
			if tags != "" {
				for _, t := range strings.Split(tags, ",") {
					t = strings.TrimSpace(t)
					if t == "" {
						continue
					}
					t = normalizeTag(t)
					if !strings.HasPrefix(t, "+") {
						args = append(args, "+"+t)
					} else {
						args = append(args, t)
					}
				}
			}
			if project != "" {
				args = append(args, "project:"+project)
			}
			if due != "" {
				args = append(args, "due:"+quoteIfNeeded(due))
			}

			res := s.runner.Run(username, 10_000_000_000, args...)
			s.cmdStore.Append(username, "Update template", args)
			if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
				applog.Warnf("template update failed: code=%d timeout=%v err=%v", res.ExitCode, res.TimedOut, res.Err)
				s.setFlash(w, "error", "Template update failed: "+res.Stderr)
				http.Redirect(w, r, "/templates/"+templateID+"/edit", http.StatusSeeOther)
				return
			}
			s.setFlash(w, "success", "Template updated successfully")
			http.Redirect(w, r, "/templates", http.StatusSeeOther)
			return
		}

		// POST /templates/{id}/delete - Template löschen
		if action == "delete" && r.Method == http.MethodPost {
			res := s.runner.Run(username, 5_000_000_000, "delete", templateID)
			s.cmdStore.Append(username, "Delete template", []string{"delete", templateID})
			if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
				applog.Warnf("template deletion failed: code=%d timeout=%v err=%v", res.ExitCode, res.TimedOut, res.Err)
				s.setFlash(w, "error", "Template deletion failed: "+res.Stderr)
			} else {
				s.setFlash(w, "success", "Template deleted successfully")
			}
			http.Redirect(w, r, "/templates", http.StatusSeeOther)
			return
		}

		http.NotFound(w, r)
	})

	// Context anzeigen/setzen
	s.mux.HandleFunc("/context", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			username, _ := auth.UsernameFromRequest(r)
			res := s.runner.Run(username, 5_000_000_000, "context")
			s.cmdStore.Append(username, "Show context", []string{"context"})
			if res.Err != nil && !res.TimedOut {
				http.Error(w, res.Stderr, http.StatusBadGateway)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			t := template.Must(s.layoutTpl.Clone())
			_, _ = t.New("content").Parse(`<h2>Context</h2>
<pre>{{.Out}}</pre>
<form method="post" action="/context">
  <div><label>New context (e.g. +work project:dstask): <input name="value"></label></div>
  <div>
    <button type="submit">Apply</button>
    <button type="submit" name="clear" value="1">Clear</button>
  </div>
</form>`)
			uname, _ := auth.UsernameFromRequest(r)
			show, entries, moreURL, canMore, ret := s.footerData(r, uname)
			_ = t.Execute(w, map[string]any{
				"Out":         strings.TrimSpace(res.Stdout),
				"Active":      activeFromPath(r.URL.Path),
				"Flash":       s.getFlash(r),
				"ShowCmdLog":  show,
				"CmdEntries":  entries,
				"MoreURL":     moreURL,
				"CanShowMore": canMore,
				"ReturnURL":   ret,
			})
		case http.MethodPost:
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			username, _ := auth.UsernameFromRequest(r)
			if r.FormValue("clear") == "1" {
				res := s.runner.Run(username, 5_000_000_000, "context", "none")
				s.cmdStore.Append(username, "Clear context", []string{"context", "none"})
				if res.Err != nil && !res.TimedOut {
					s.setFlash(w, "error", "Failed to clear context")
					http.Redirect(w, r, "/context", http.StatusSeeOther)
					return
				}
				s.setFlash(w, "success", "Context cleared")
				http.Redirect(w, r, "/context", http.StatusSeeOther)
				return
			}
			val := strings.TrimSpace(r.FormValue("value"))
			if val == "" {
				http.Error(w, "value required", http.StatusBadRequest)
				return
			}
			res := s.runner.Run(username, 5_000_000_000, "context", val)
			s.cmdStore.Append(username, "Set context", []string{"context", val})
			if res.Err != nil && !res.TimedOut {
				s.setFlash(w, "error", "Failed to set context")
				http.Redirect(w, r, "/context", http.StatusSeeOther)
				return
			}
			s.setFlash(w, "success", "Context set")
			http.Redirect(w, r, "/context", http.StatusSeeOther)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Task erstellen (Form)
	s.mux.HandleFunc("/tasks/new", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		// Fetch existing projects and tags
		projRes := s.runner.Run(username, 5_000_000_000, "show-projects")
		tagRes := s.runner.Run(username, 5_000_000_000, "show-tags")
		tmplRes := s.runner.Run(username, 5_000_000_000, "show-templates")
		projects := parseProjectsFromOutput(projRes.Stdout)
		tags := parseTagsFromOutput(tagRes.Stdout)
		templates := parseTemplatesFromOutput(tmplRes.Stdout)
		selectedTemplate := r.URL.Query().Get("template")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t := template.Must(s.layoutTpl.Clone())
		_, _ = t.New("content").Parse(`
<h2>New task</h2>
<form method="post" action="/tasks">
  <div><label>Summary: <input name="summary" required style="width:60%"></label></div>
  <div>
    <label>Project:</label>
    <select name="projectSelect">
      <option value="">(none)</option>
      {{range .Projects}}<option value="{{.}}">{{.}}</option>{{end}}
    </select>
    <span style="margin:0 8px;">or</span>
    <input name="project" placeholder="new project" />
  </div>
  <div>
    <label>Tags:</label>
    <div style="max-height:140px;overflow:auto;border:1px solid #ddd;padding:6px;">
      {{range .Tags}}<label style="display:inline-block;margin-right:8px;">
        <input type="checkbox" name="tagsExisting" value="{{.}}"/> {{.}}
      </label>{{end}}
    </div>
    <div style="margin-top:6px;">
      <label>Add tags (comma-separated): <input name="tags"></label>
    </div>
  </div>
  <div>
    <label>Due:</label>
    <input type="date" name="dueDate" />
    <span style="margin:0 8px;">or</span>
    <input name="due" placeholder="e.g. friday / 2025-12-31" />
  </div>
  <div>
    <label>Template:</label>
    <select name="template">
      <option value="">(none)</option>
      {{range .Templates}}<option value="{{index . "id"}}" {{if eq $.SelectedTemplate (index . "id")}}selected{{end}}>#{{index . "id"}}: {{index . "summary"}}</option>{{end}}
    </select>
  </div>
  <fieldset style="margin-top:12px;">
    <legend>Music (radio station)</legend>
    <div>
      <label>Type:
        <select name="music_type">
          <option value="">(none)</option>
          <option value="radio">radio</option>
        </select>
      </label>
    </div>
    <div style="margin-top:6px;"><label>Name: <input name="music_name" placeholder="Station label"><button type="button" id="new_music_name_search" style="margin-left:6px;">Search</button></label></div>
    <div style="margin-top:6px;"><label>Stream URL: <input name="music_url" style="width:60%" placeholder="https://…"></label></div>
    <ul id="new_music_search_results" style="max-height:160px; overflow:auto; border:1px solid #ddd; padding:6px; margin-top:6px;"></ul>
  </fieldset>
  <script>
  (function(){
    var typeSel = document.querySelector('select[name="music_type"]');
    var nameInp = document.querySelector('input[name="music_name"]');
    var urlInp  = document.querySelector('input[name="music_url"]');
    var btn     = document.getElementById('new_music_name_search');
    var ul      = document.getElementById('new_music_search_results');
    if(!btn || !nameInp) return;
    btn.addEventListener('click', function(){
      if(typeSel && typeSel.value !== 'radio') return;
      var q = (nameInp.value||'').trim(); if(!q) return;
      ul.textContent = 'Searching…';
      fetch('/music/search?q='+encodeURIComponent(q))
        .then(function(r){ return r.json(); })
        .then(function(arr){
          ul.innerHTML = '';
          (arr||[]).forEach(function(st){
            var li = document.createElement('li');
            li.style.cursor='pointer';
            var nm = st.name || '(no name)';
            var u  = st.url_resolved || st.url || '';
            li.textContent = nm + ' — ' + u;
            li.onclick = function(){ nameInp.value = nm; if(u) urlInp.value = u; };
            ul.appendChild(li);
          });
          if(!ul.childElementCount){ ul.textContent = 'No results'; }
        })
        .catch(function(){ ul.textContent = 'Search failed'; });
    });
  })();
  </script>
  <div style="margin-top:8px;"><button type="submit">Create</button></div>
 </form>
        `)
		uname, _ := auth.UsernameFromRequest(r)
		show, entries, moreURL, canMore, ret := s.footerData(r, uname)
		_ = t.Execute(w, map[string]any{
			"Active": activeFromPath(r.URL.Path), "Projects": projects, "Tags": tags, "Templates": templates, "SelectedTemplate": selectedTemplate,
			"ShowCmdLog": show, "CmdEntries": entries, "MoreURL": moreURL, "CanShowMore": canMore, "ReturnURL": ret,
		})
	})

	// Task erstellen (POST)
	s.mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		summary := strings.TrimSpace(r.FormValue("summary"))
		if summary == "" {
			http.Error(w, "summary required", http.StatusBadRequest)
			return
		}
		tags := strings.TrimSpace(r.FormValue("tags"))
		project := strings.TrimSpace(r.FormValue("project"))
		// Prefer selected existing project if provided
		if ps := strings.TrimSpace(r.FormValue("projectSelect")); ps != "" {
			project = ps
		}
		// Due: prefer date picker if present
		dueDate := strings.TrimSpace(r.FormValue("dueDate"))
		due := sanitizeDueValue(r.FormValue("due"))
		if dueDate != "" {
			due = sanitizeDueValue(dueDate)
		}

		// Compose args per dstask add Syntax: add <summary tokens...> +tags project: due:
		args := []string{"add"}
		args = append(args, summaryTokens(summary)...)
		// Collect tags from existing checkboxes
		for _, t := range r.Form["tagsExisting"] {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			t = normalizeTag(t)
			if !strings.HasPrefix(t, "+") {
				args = append(args, "+"+t)
			} else {
				args = append(args, t)
			}
		}
		if tags != "" {
			for _, t := range strings.Split(tags, ",") {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				t = normalizeTag(t)
				if strings.HasPrefix(t, "+") { // allow user to prefix (rare)
					args = append(args, t)
				} else {
					args = append(args, "+"+t)
				}
			}
		}
		if project != "" {
			args = append(args, "project:"+quoteIfNeeded(project))
		}
		if due != "" {
			args = append(args, "due:"+quoteIfNeeded(due))
		}
		// Template support
		if templateID := strings.TrimSpace(r.FormValue("template")); templateID != "" {
			args = append(args, "template:"+templateID)
		}
		username, _ := auth.UsernameFromRequest(r)
		res := s.runner.Run(username, 10_000_000_000, args...) // 10s
		s.cmdStore.Append(username, "New task", append([]string{"add"}, args[1:]...))
		if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
			s.setFlash(w, "error", "Failed to create task")
			http.Redirect(w, r, "/tasks/new", http.StatusSeeOther)
			return
		}
		s.setFlash(w, "success", "Task created")
		http.Redirect(w, r, "/open?html=1", http.StatusSeeOther)
	})

	// Task Aktionen: /tasks/{id}/start|stop|done|remove|log|note
	// Open URLs: /tasks/{id}/open
	s.mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		applog.Infof("/tasks: %s %s", r.Method, r.URL.Path)
		// Parse path parts
		parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
		// Handle task actions via GET or POST first (start|stop|done|remove|log)
		if len(parts) == 3 && parts[0] == "tasks" {
			act := strings.TrimSpace(parts[2])
			if act == "start" || act == "stop" || act == "done" || act == "remove" || act == "log" {
				id := strings.TrimSpace(parts[1])
				if id == "" {
					http.NotFound(w, r)
					return
				}
				username, _ := auth.UsernameFromRequest(r)
				res := s.runner.Run(username, 10*time.Second, act, id)
				if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
					applog.Warnf("/tasks action failed: %s %s code=%d timeout=%v err=%v", act, id, res.ExitCode, res.TimedOut, res.Err)
					s.setFlash(w, "error", "Task action failed")
				} else {
					applog.Infof("/tasks action ok: %s %s", act, id)
					var token string
					if act == "start" || act == "stop" {
						if m, _, err := music.LoadForUser(s.cfg, username); err == nil && m.Tasks != nil {
							if tm, ok := m.Tasks[id]; ok && tm.Type == "radio" && tm.URL != "" {
								if act == "start" {
									// include id and persisted volume/muted in token (always send volume to restore exact state)
									token = "__MUSIC_START__" + tm.Name + "|" + tm.URL + "|id=" + id + "|vol=" + strconv.FormatFloat(float64(tm.Volume), 'f', 2, 32)
									if tm.Muted {
										token += "|muted=1"
									}
									applog.Infof("music token set for task %s start: %s", id, tm.URL)
								} else {
									token = "__MUSIC_STOP__"
									applog.Infof("music token set for task %s stop", id)
								}
							} else {
								applog.Debugf("no music mapping for task %s or URL empty", id)
							}
						} else if err != nil {
							applog.Warnf("loading music map failed for %s: %v", username, err)
						}
					}
					msg := "Task action applied"
					if token != "" {
						s.setFlash(w, "success", token+"\n"+msg)
					} else {
						s.setFlash(w, "success", msg)
					}
				}
				http.Redirect(w, r, "/open?html=1", http.StatusSeeOther)
				return
			}
		}
		// Prüfe auf GET /tasks/{id}/open für URL-Anzeige

		if len(parts) == 3 && parts[0] == "tasks" && parts[2] == "open" && r.Method == http.MethodGet {
			id := parts[1]
			if id == "" {
				http.NotFound(w, r)
				return
			}
			username, _ := auth.UsernameFromRequest(r)
			res := s.runner.Run(username, 5*time.Second, "export")

			if res.Err != nil || res.ExitCode != 0 {
				http.Error(w, "Failed to fetch task", http.StatusBadGateway)
				return
			}

			tasks, ok := decodeTasksJSONFlexible(res.Stdout)
			if !ok {
				http.Error(w, "Failed to parse tasks", http.StatusInternalServerError)
				return
			}

			var task map[string]any
			for _, t := range tasks {
				if str(firstOf(t, "id", "ID")) == id {
					task = t
					break
				}
			}

			if task == nil {
				http.Error(w, "Task not found", http.StatusNotFound)
				return
			}

			// Extract URLs from summary and notes
			summary := str(firstOf(task, "summary", "description"))
			notes := str(firstOf(task, "notes", "annotations"))

			allText := summary + " " + notes
			urls := extractURLs(allText)

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			tpl := template.Must(s.layoutTpl.Clone())
			_, _ = tpl.New("content").Parse(`
<h2>URLs in Task #{{.ID}}</h2>
<p><strong>Summary:</strong> {{.Summary}}</p>
{{if .URLs}}
<ul>
{{range .URLs}}
  <li><a href="{{.}}" target="_blank" rel="noopener">{{.}}</a></li>
{{end}}
</ul>
{{else}}
<p>No URLs found in this task.</p>
{{end}}
<p><a href="/open?html=1">Back to tasks</a></p>
`)

			uname, _ := auth.UsernameFromRequest(r)
			show, entries, moreURL, canMore, ret := s.footerData(r, uname)
			_ = tpl.Execute(w, map[string]any{
				"ID":          id,
				"Summary":     summary,
				"URLs":        urls,
				"Active":      activeFromPath(r.URL.Path),
				"ShowCmdLog":  show,
				"CmdEntries":  entries,
				"MoreURL":     moreURL,
				"CanShowMore": canMore,
				"ReturnURL":   ret,
			})
			return
		}

		// GET /tasks/{id}/edit - Bearbeitungsformular anzeigen
		if len(parts) == 3 && parts[0] == "tasks" && parts[2] == "edit" && r.Method == http.MethodGet {
			id := parts[1]
			if id == "" {
				http.NotFound(w, r)
				return
			}
			username, _ := auth.UsernameFromRequest(r)

			var task map[string]any

			// Try export first (gets all non-resolved tasks)
			res := s.runner.Run(username, 5*time.Second, "export")
			if res.Err == nil && res.ExitCode == 0 && !res.TimedOut {
				if tasks, ok := decodeTasksJSONFlexible(res.Stdout); ok {
					for _, t := range tasks {
						taskID := str(firstOf(t, "id", "ID", "Id", "uuid", "UUID"))
						if taskID == id {
							task = t
							break
						}
					}
				}
			}

			// Fallback 1: try dstask <id> directly (shows single task details, works for any status)
			if task == nil {
				showRes := s.runner.Run(username, 5*time.Second, id)
				if showRes.Err == nil && showRes.ExitCode == 0 && !showRes.TimedOut {
					if tasks, ok := decodeTasksJSONFlexible(showRes.Stdout); ok && len(tasks) > 0 {
						task = tasks[0]
					} else {
						// If JSON parsing fails, log for debugging
						applog.Warnf("task %s: JSON parse failed from 'dstask %s', stdout=%q", id, id, truncate(showRes.Stdout, 200))
					}
				} else {
					applog.Warnf("task %s: 'dstask %s' failed, code=%d err=%v stderr=%q", id, id, showRes.ExitCode, showRes.Err, truncate(showRes.Stderr, 200))
				}
			}

			// Fallback 2: try show-resolved in case task is resolved and not in export
			if task == nil {
				resolvedRes := s.runner.Run(username, 5*time.Second, "show-resolved")
				if resolvedRes.Err == nil && resolvedRes.ExitCode == 0 && !resolvedRes.TimedOut {
					if tasks, ok := decodeTasksJSONFlexible(resolvedRes.Stdout); ok {
						for _, t := range tasks {
							taskID := str(firstOf(t, "id", "ID", "Id", "uuid", "UUID"))
							if taskID == id {
								task = t
								break
							}
						}
					}
				}
			}

			if task == nil {
				applog.Warnf("task %s: not found in export, 'dstask %s', or show-resolved", id, id)
				http.Error(w, "Task not found", http.StatusNotFound)
				return
			}

			// Fetch existing projects and tags
			projRes := s.runner.Run(username, 5_000_000_000, "show-projects")
			tagRes := s.runner.Run(username, 5_000_000_000, "show-tags")
			projects := parseProjectsFromOutput(projRes.Stdout)
			tags := parseTagsFromOutput(tagRes.Stdout)

			// Parse task data
			summary := trimQuotes(str(firstOf(task, "summary", "Summary", "description", "Description")))
			project := trimQuotes(str(firstOf(task, "project", "Project")))
			priority := str(firstOf(task, "priority", "Priority"))
			dueValue := sanitizeDueValue(trimQuotes(str(firstOf(task, "due", "Due", "dueDate", "DueDate"))))
			notes := trimQuotes(str(firstOf(task, "notes", "annotations", "note")))

			// Parse existing tags from task
			existingTags := make(map[string]bool)
			taskTags := firstOf(task, "tags", "Tags")
			if tagList, ok := taskTags.([]any); ok {
				for _, tag := range tagList {
					if tagStr := str(tag); tagStr != "" {
						existingTags[tagStr] = true
					}
				}
			} else if tagStr := str(taskTags); tagStr != "" {
				for _, tag := range strings.Split(tagStr, ", ") {
					tag = strings.TrimSpace(tag)
					if tag != "" {
						existingTags[tag] = true
					}
				}
			}

			// Parse due date to YYYY-MM-DD format if it's a date
			dueDateValue := ""
			if dueValue != "" {
				if t := parseTimeOrZero(dueValue); !t.IsZero() {
					dueDateValue = t.Format("2006-01-02")
					dueValue = "" // Clear due field if we can parse it as date
				}
			}

			hasProjectInSelect := false
			for _, p := range projects {
				if p == project {
					hasProjectInSelect = true
					break
				}
			}

			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			t := template.Must(s.layoutTpl.Clone())
			// Load existing music mapping for this task
			mm, _, _ := music.LoadForUser(s.cfg, username)
			var mtype, mname, murl, mpath string
			if mm != nil {
				if tm, ok := mm.Tasks[id]; ok {
					mtype, mname, murl, mpath = tm.Type, tm.Name, tm.URL, tm.Path
				}
			}
			_, _ = t.New("content").Parse(`
<h2>Edit task #{{.TaskID}}</h2>
<form method="post" action="/tasks/{{.TaskID}}/edit">
  <input type="hidden" name="return_to" value="{{.Referer}}"/>
  <div><label>Summary: <input name="summary" value="{{.Summary}}" required style="width:60%"></label></div>
  <div>
    <label>Project:</label>
    <select name="projectSelect">
      <option value="">(none)</option>
      {{range .Projects}}
        <option value="{{.}}" {{if eq $.Project .}}selected{{end}}>{{.}}</option>
      {{end}}
    </select>
    <span style="margin:0 8px;">or</span>
    <input name="project" value="{{if not .HasProjectInSelect}}{{.Project}}{{end}}" placeholder="new project" />
  </div>
  <div>
    <label>Priority:</label>
    <select name="priority">
      <option value="">(none)</option>
      <option value="P0" {{if eq .Priority "P0"}}selected{{end}}>P0 (Critical)</option>
      <option value="P1" {{if eq .Priority "P1"}}selected{{end}}>P1 (High)</option>
      <option value="P2" {{if eq .Priority "P2"}}selected{{end}}>P2 (Normal)</option>
      <option value="P3" {{if eq .Priority "P3"}}selected{{end}}>P3 (Low)</option>
    </select>
  </div>
  <div>
    <label>Tags:</label>
    <div style="max-height:140px;overflow:auto;border:1px solid #ddd;padding:6px;">
      {{range .Tags}}
        <label style="display:inline-block;margin-right:8px;">
          <input type="checkbox" name="tagsExisting" value="{{.}}" {{if index $.ExistingTags .}}checked{{end}}/> {{.}}
        </label>
      {{end}}
    </div>
    <div style="margin-top:6px;">
      <label>Add tags (comma-separated): <input name="tags" placeholder="additional tags"></label>
    </div>
  </div>
  <div>
    <label>Due:</label>
    <input type="date" name="dueDate" value="{{.DueDate}}" />
    <span style="margin:0 8px;">or</span>
    <input name="due" value="{{.Due}}" placeholder="e.g. friday / 2025-12-31" />
  </div>
  <div>
    <label>Notes (markdown supported):
      <textarea name="notes" rows="10" style="width:100%;font-family:monospace;">{{.Notes}}</textarea>
    </label>
    {{if .Notes}}
    <div style="margin-top:8px;">
      <strong>Preview:</strong>
      <div class="notes-content">{{renderMarkdown .Notes}}</div>
    </div>
    {{end}}
  </div>
  <fieldset style="margin-top:12px;">
    <legend>Music (radio station or local folder)</legend>
    <div>
      <label>Type:
        <select name="music_type">
          <option value="">(none)</option>
          <option value="radio" {{if eq .MusicType "radio"}}selected{{end}}>radio</option>
        </select>
      </label>
    </div>
    <div style="margin-top:6px;"><label>Name: <input name="music_name" value="{{.MusicName}}" placeholder="Station or folder label"><button type="button" id="music_name_search" style="margin-left:6px;">Search</button></label></div>
    <div style="margin-top:6px;"><label>Stream URL: <input name="music_url" value="{{.MusicURL}}" style="width:60%" placeholder="https://… (for type=radio)"></label></div>
    <ul id="music_search_results" style="max-height:160px; overflow:auto; border:1px solid #ddd; padding:6px; margin-top:6px;"></ul>
  </fieldset>
  <script>
  (function(){
    var typeSel = document.querySelector('select[name="music_type"]');
    var nameInp = document.querySelector('input[name="music_name"]');
    var urlInp  = document.querySelector('input[name="music_url"]');
    var btn     = document.getElementById('music_name_search');
    var ul      = document.getElementById('music_search_results');
    if(!btn || !nameInp) return;
    btn.addEventListener('click', function(){
      if(typeSel && typeSel.value !== 'radio') return;
      var q = (nameInp.value||'').trim(); if(!q) return;
      ul.textContent = 'Searching…';
      fetch('/music/search?q='+encodeURIComponent(q))
        .then(function(r){ return r.json(); })
        .then(function(arr){
          ul.innerHTML = '';
          (arr||[]).forEach(function(st){
            var li = document.createElement('li');
            li.style.cursor='pointer';
            var nm = st.name || '(no name)';
            var u  = st.url_resolved || st.url || '';
            li.textContent = nm + ' — ' + u;
            li.onclick = function(){ nameInp.value = nm; if(u) urlInp.value = u; };
            ul.appendChild(li);
          });
          if(!ul.childElementCount){ ul.textContent = 'No results'; }
        })
        .catch(function(){ ul.textContent = 'Search failed'; });
    });
  })();
  </script>
  <div style="margin-top:8px;">
    <button type="submit" title="Save changes to task">Update task</button>
    <button type="button" onclick="window.location.href='{{.Referer}}'; return false;" style="margin-left:8px;" title="Cancel editing and return to previous page">cancel</button>
  </div>
</form>
`)

			// Get referer for cancel button
			referer := r.Header.Get("Referer")
			if referer == "" {
				referer = "/open?html=1"
			}

			uname, _ := auth.UsernameFromRequest(r)
			show, entries, moreURL, canMore, ret := s.footerData(r, uname)
			_ = t.Execute(w, map[string]any{
				"TaskID":             id,
				"Summary":            summary,
				"Project":            project,
				"Priority":           priority,
				"HasProjectInSelect": hasProjectInSelect,
				"DueDate":            dueDateValue,
				"Due":                dueValue,
				"Notes":              notes,
				"Projects":           projects,
				"Tags":               tags,
				"ExistingTags":       existingTags,
				"Referer":            referer,
				"MusicType":          mtype,
				"MusicName":          mname,
				"MusicURL":           murl,
				"MusicPath":          mpath,
				"Active":             activeFromPath(r.URL.Path),
				"ShowCmdLog":         show,
				"CmdEntries":         entries,
				"MoreURL":            moreURL,
				"CanShowMore":        canMore,
				"ReturnURL":          ret,
			})
			return
		}

		// POST /tasks/{id}/edit - Task aktualisieren
		if len(parts) == 3 && parts[0] == "tasks" && parts[2] == "edit" && r.Method == http.MethodPost {
			id := parts[1]
			if id == "" {
				http.NotFound(w, r)
				return
			}
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			username, _ := auth.UsernameFromRequest(r)

			summary := strings.TrimSpace(r.FormValue("summary"))
			if summary == "" {
				http.Error(w, "summary required", http.StatusBadRequest)
				return
			}

			project := strings.TrimSpace(r.FormValue("project"))
			if ps := strings.TrimSpace(r.FormValue("projectSelect")); ps != "" {
				project = ps
			}
			priority := strings.TrimSpace(r.FormValue("priority"))
			dueDate := strings.TrimSpace(r.FormValue("dueDate"))
			due := sanitizeDueValue(r.FormValue("due"))
			if dueDate != "" {
				due = sanitizeDueValue(dueDate)
			}
			notes := r.FormValue("notes") // Don't trim - preserve newlines
			// Music mapping from form
			mtype := strings.TrimSpace(r.FormValue("music_type"))
			mname := strings.TrimSpace(r.FormValue("music_name"))
			murl := strings.TrimSpace(r.FormValue("music_url"))
			mpath := strings.TrimSpace(r.FormValue("music_path"))

			// Build modify args: dstask <id> modify <summary> project: priority due: +tags
			args := []string{id, "modify"}
			// Summary tokens
			args = append(args, summaryTokens(summary)...)
			// Project
			if project != "" {
				args = append(args, "project:"+quoteIfNeeded(project))
			}
			// Priority
			if priority != "" {
				args = append(args, priority)
			}
			// Due
			if due != "" {
				args = append(args, "due:"+quoteIfNeeded(due))
			}
			// Tags from checkboxes
			for _, t := range r.Form["tagsExisting"] {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				t = normalizeTag(t)
				if !strings.HasPrefix(t, "+") {
					args = append(args, "+"+t)
				} else {
					args = append(args, t)
				}
			}
			// Additional tags
			if tags := strings.TrimSpace(r.FormValue("tags")); tags != "" {
				for _, t := range strings.Split(tags, ",") {
					t = strings.TrimSpace(t)
					if t == "" {
						continue
					}
					t = normalizeTag(t)
					if !strings.HasPrefix(t, "+") {
						args = append(args, "+"+t)
					} else {
						args = append(args, t)
					}
				}
			}

			res := s.runner.Run(username, 10*time.Second, args...)
			s.cmdStore.Append(username, "Edit task", args)
			if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
				applog.Warnf("task edit failed: code=%d timeout=%v err=%v stderr=%q", res.ExitCode, res.TimedOut, res.Err, truncate(res.Stderr, 200))
				// Strip ANSI codes from stderr before setting flash message
				cleanStderr := stripANSI(res.Stderr)
				if cleanStderr != "" {
					s.setFlash(w, "error", "Task update failed: "+cleanStderr)
				} else {
					s.setFlash(w, "error", "Task update failed")
				}
				http.Redirect(w, r, "/tasks/"+id+"/edit", http.StatusSeeOther)
				return
			}
			// Update notes if provided
			notesUpdateSuccess := true
			if strings.TrimSpace(notes) != "" {
				applog.Infof("attempting to update notes for task %s, notes length: %d, first 100 chars: %q", id, len(notes), truncate(notes, 100))

				// Use direct YAML file editing method (most reliable for multi-line notes)
				if err := s.runner.UpdateTaskNotesDirectly(username, id, notes); err != nil {
					applog.Warnf("task note update failed: %v", err)
					notesUpdateSuccess = false
				} else {
					s.cmdStore.Append(username, "Edit task notes", []string{"note", id})
					applog.Infof("task note update succeeded (notes saved directly to YAML file)")
				}
			}
			// Save music mapping based on form
			if mtype != "" {
				if mm, _, err := music.LoadForUser(s.cfg, username); err == nil {
					if mm.Tasks == nil {
						mm.Tasks = map[string]music.TaskMusic{}
					}
					tm := music.TaskMusic{Type: mtype, Name: mname}
					if mtype == "radio" {
						tm.URL = murl
					} else if mtype == "folder" {
						tm.Path = mpath
					}
					mm.Tasks[id] = tm
					if _, err := music.SaveForUser(s.cfg, username, mm); err != nil {
						applog.Warnf("failed to save music mapping: %v", err)
					}
				} else {
					applog.Warnf("failed to load music map: %v", err)
				}
			}
			if notesUpdateSuccess {
				s.setFlash(w, "success", "Task updated successfully")
			} else {
				s.setFlash(w, "warning", "Task updated, but notes update may have failed. Check the task to verify.")
			}
			// Redirect to return_to from form if available, otherwise try referer, otherwise /open
			returnTo := strings.TrimSpace(r.FormValue("return_to"))
			if returnTo == "" {
				returnTo = r.Header.Get("Referer")
			}
			// If return_to points to the edit page itself, redirect to /open instead
			if returnTo == "" || strings.Contains(returnTo, "/tasks/"+id+"/edit") {
				returnTo = "/open?html=1"
			}
			http.Redirect(w, r, returnTo, http.StatusSeeOther)
			return
		}

		http.NotFound(w, r)
	})

	// Einfache Aktionsseite (UI-Politur): ID + Aktion auswählen
	s.mux.HandleFunc("/tasks/action", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			t := template.Must(s.layoutTpl.Clone())
			_, _ = t.New("content").Parse(`
<h2>Task actions</h2>
<form method="post" action="/tasks/submit">
  <div><label>Task ID: <input name="id" required></label></div>
  <div><label>Action:
    <select name="action">
      <option value="start">start</option>
      <option value="stop">stop</option>
      <option value="done">done</option>
      <option value="remove">remove</option>
      <option value="log">log</option>
      <option value="note">note</option>
    </select>
  </label></div>
  <div><label>Note (for action "note"):<br><textarea name="note" rows="3" cols="40"></textarea></label></div>
  <div><button type="submit">Execute</button></div>
</form>`)
			uname, _ := auth.UsernameFromRequest(r)
			show, entries, moreURL, canMore, ret := s.footerData(r, uname)
			_ = t.Execute(w, map[string]any{
				"Active":     activeFromPath(r.URL.Path),
				"ShowCmdLog": show, "CmdEntries": entries, "MoreURL": moreURL, "CanShowMore": canMore, "ReturnURL": ret,
			})
		case http.MethodPost:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Submission der Aktionsseite
	s.mux.HandleFunc("/tasks/submit", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		id := strings.TrimSpace(r.FormValue("id"))
		action := strings.TrimSpace(r.FormValue("action"))
		note := strings.TrimSpace(r.FormValue("note"))
		if id == "" || action == "" {
			http.Error(w, "id/action required", http.StatusBadRequest)
			return
		}
		username, _ := auth.UsernameFromRequest(r)
		timeout := 10 * time.Second
		var res dstask.Result
		switch action {
		case "start", "stop", "done", "remove", "log":
			res = s.runner.Run(username, timeout, action, id)
		case "note":
			if note == "" {
				http.Error(w, "note required", http.StatusBadRequest)
				return
			}
			res = s.runner.Run(username, timeout, "note", id, note)
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}
		if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadRequest)
			return
		}
		s.setFlash(w, "success", "Task action applied")
		http.Redirect(w, r, "/open?html=1", http.StatusSeeOther)
	})

	// Task ändern (Project/Priority/Due/Tags)
	s.mux.HandleFunc("/tasks/modify", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		id := strings.TrimSpace(r.FormValue("id"))
		if id == "" {
			http.Error(w, "id required", http.StatusBadRequest)
			return
		}
		project := strings.TrimSpace(r.FormValue("project"))
		priority := strings.TrimSpace(r.FormValue("priority"))
		due := sanitizeDueValue(r.FormValue("due"))
		addTags := strings.TrimSpace(r.FormValue("addTags"))
		removeTags := strings.TrimSpace(r.FormValue("removeTags"))

		args := []string{"modify"}
		if project != "" {
			args = append(args, "project:"+quoteIfNeeded(project))
		}
		if priority != "" {
			args = append(args, priority)
		}
		if due != "" {
			args = append(args, "due:"+quoteIfNeeded(due))
		}
		if addTags != "" {
			for _, t := range strings.Split(addTags, ",") {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				t = normalizeTag(t)
				if !strings.HasPrefix(t, "+") {
					t = "+" + t
				}
				args = append(args, t)
			}
		}
		if removeTags != "" {
			for _, t := range strings.Split(removeTags, ",") {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				t = normalizeTag(t)
				if !strings.HasPrefix(t, "-") {
					t = "-" + t
				}
				args = append(args, t)
			}
		}
		username, _ := auth.UsernameFromRequest(r)
		// dstask modify erwartet Syntax: dstask <id> modify ... (laut usage)
		full := append([]string{id}, args...)
		res := s.runner.Run(username, 10*time.Second, full...)
		if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadRequest)
			return
		}
		s.setFlash(w, "success", "Task modified")
		http.Redirect(w, r, "/open?html=1", http.StatusSeeOther)
	})

	// Version anzeigen
	s.mux.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		username, _ := auth.UsernameFromRequest(r)
		res := s.runner.Run(username, 5_000_000_000, "version")
		s.cmdStore.Append(username, "Show version", []string{"version"})
		if res.Err != nil && !res.TimedOut {
			http.Error(w, res.Stderr, http.StatusBadGateway)
			return
		}
		out := strings.TrimSpace(res.Stdout)
		if out == "" {
			out = "Unknown"
		}
		if r.URL.Query().Get("raw") == "1" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			_, _ = w.Write([]byte(out))
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		t := template.Must(s.layoutTpl.Clone())
		_, _ = t.New("content").Parse(`<h2>dstask version</h2>
<pre style="white-space:pre-wrap;">{{.Out}}</pre>`)
		uname, _ := auth.UsernameFromRequest(r)
		show, entries, moreURL, canMore, ret := s.footerData(r, uname)
		_ = t.Execute(w, map[string]any{
			"Out":         out,
			"Active":      activeFromPath(r.URL.Path),
			"Flash":       s.getFlash(r),
			"ShowCmdLog":  show,
			"CmdEntries":  entries,
			"MoreURL":     moreURL,
			"CanShowMore": canMore,
			"ReturnURL":   ret,
		})
	})

	// Sync anzeigen/ausführen
	s.mux.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			t := template.Must(s.layoutTpl.Clone())
			_, _ = t.New("content").Parse(`<h2>Sync</h2>
<p>Runs <code>dstask sync</code> (pull, merge, push). The underlying repo must have a remote with an upstream branch.</p>
<form method="post" action="/sync"><button type="submit">Sync now</button></form>`)
			_ = t.Execute(w, nil)
		case http.MethodPost:
			username, _ := auth.UsernameFromRequest(r)
			applog.Infof("/sync POST from %s", username)
			// Falls kein Git-Repo vorhanden ist, biete Clone-Form an
			if uhome, ok := config.ResolveHomeForUsername(s.cfg, username); ok && uhome != "" {
				dir := uhome
				if !strings.HasSuffix(strings.ToLower(dir), ".dstask") {
					dir = dir + string('/') + ".dstask"
				}
				if _, err := os.Stat(dir + string('/') + ".git"); err != nil {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					t := template.Must(s.layoutTpl.Clone())
					_, _ = t.New("content").Parse(`<h2>Clone remote</h2>
<p>Kein Git-Repository gefunden. Bitte gib eine Remote-URL an, um sie in <code>~/.dstask</code> zu klonen.</p>
<form method="post" action="/sync/clone-remote">
  <div><label>Remote URL: <input name="url" required style="width:60%" placeholder="https://... oder git@..." /></label></div>
  <div style="margin-top:8px;">
    <button type="submit">Klonen</button>
    <a href="/" style="margin-left:8px;">abbrechen</a>
  </div>
</form>`)
					_ = t.Execute(w, nil)
					return
				}
			}
			// Falls kein Remote gesetzt ist, biete Remote-Form an
			if u, _ := s.runner.GitRemoteURL(username); strings.TrimSpace(u) == "" {
				applog.Warnf("/sync: no remote configured for %s", username)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				t := template.Must(s.layoutTpl.Clone())
				_, _ = t.New("content").Parse(`<h2>Configure remote</h2>
<p>Für Sync ist ein Remote-Repository erforderlich. Bitte gib eine Git-Remote-URL an (z. B. <code>git@github.com:user/repo.git</code> oder <code>https://github.com/user/repo.git</code>).</p>
<form method="post" action="/sync/set-remote">
  <div><label>Remote URL: <input name="url" required style="width:60%" placeholder="https://... or git@..." /></label></div>
  <div style="margin-top:8px;">
    <button type="submit">Remote speichern</button>
    <a href="/" style="margin-left:8px;">abbrechen</a>
  </div>
</form>`)
				_ = t.Execute(w, nil)
				return
			}
			// Upstream sicherstellen (best effort)
			if _, err := s.runner.GitSetUpstreamIfMissing(username); err != nil {
				applog.Warnf("/sync: upstream missing; automatic setup failed: %v", err)
			}
			applog.Infof("/sync: starting dstask sync for %s", username)
			res := s.runner.Run(username, 30_000_000_000, "sync") // 30s
			applog.Infof("/sync: finished for %s: code=%d timeout=%v err=%v", username, res.ExitCode, res.TimedOut, res.Err)
			s.cmdStore.Append(username, "Sync", []string{"sync"})
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			t := template.Must(s.layoutTpl.Clone())
			// Erkennung gängiger Git-Fehler für hilfreiche Hinweise
			hint := ""
			out := strings.TrimSpace(res.Stdout + "\n" + res.Stderr)
			if strings.Contains(out, "There is no tracking information for the current branch") {
				hint = `Es ist kein Upstream gesetzt. Setze ihn im .dstask-Repo, z. B.:<br>
<pre>git remote add origin &lt;REMOTE_URL&gt;
git push -u origin master</pre>`
			}
			status := "Erfolg"
			if res.Err != nil || res.ExitCode != 0 {
				status = "Fehler"
			}
			_, _ = t.New("content").Parse(`<h2>Sync: {{.Status}}</h2>
{{if .Hint}}<div style="background:#fff3cd;padding:8px;border:1px solid #ffeeba;margin-bottom:8px;">{{.Hint}}</div>{{end}}
<pre style="white-space: pre-wrap;">{{.Out}}</pre>
<p><a href="/open?html=1">Back to list</a></p>`)
			_ = t.Execute(w, map[string]any{
				"Status": status,
				"Out":    out,
				"Hint":   template.HTML(hint),
				"Active": activeFromPath(r.URL.Path),
				"Flash":  s.getFlash(r),
			})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	// Remote setzen
	s.mux.HandleFunc("/sync/set-remote", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// (removed stray /tasks/{id}/{action} handler here; real handler is in /tasks/ route)

		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		url := strings.TrimSpace(r.FormValue("url"))
		if url == "" {
			http.Error(w, "url required", http.StatusBadRequest)
			return
		}
		username, _ := auth.UsernameFromRequest(r)
		applog.Infof("/sync/set-remote from %s: url=%s", username, url)
		if err := s.runner.GitSetRemoteOrigin(username, url); err != nil {
			s.setFlash(w, "error", "Remote konnte nicht gesetzt werden: "+stripANSI(err.Error()))
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		// Upstream setzen, falls nötig (best effort)
		if _, err := s.runner.GitSetUpstreamIfMissing(username); err != nil {
			applog.Warnf("/sync/set-remote: failed to set upstream: %v", err)
			// Tipp geben, aber nicht als fatal behandeln
			s.setFlash(w, "warning", "Remote gesetzt. Upstream konnte nicht automatisch gesetzt werden. Bitte im Repo setzen.")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.setFlash(w, "success", "Remote konfiguriert. Du kannst jetzt Sync ausführen.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// Remote klonen
	s.mux.HandleFunc("/sync/clone-remote", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		url := strings.TrimSpace(r.FormValue("url"))
		if url == "" {
			http.Error(w, "url required", http.StatusBadRequest)
			return
		}
		username, _ := auth.UsernameFromRequest(r)
		applog.Infof("/sync/clone-remote from %s: url=%s", username, url)
		if err := s.runner.GitCloneRemote(username, url); err != nil {
			s.setFlash(w, "error", "Klonen fehlgeschlagen: "+stripANSI(err.Error()))
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		if _, err := s.runner.GitSetUpstreamIfMissing(username); err != nil {
			applog.Warnf("/sync/clone-remote: failed to set upstream: %v", err)
			s.setFlash(w, "warning", "Klonen erfolgreich. Upstream konnte nicht automatisch gesetzt werden.")
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		s.setFlash(w, "success", "Repository geklont. Du kannst jetzt Sync ausführen.")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	// Undo last action
	s.mux.HandleFunc("/undo", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		username, _ := auth.UsernameFromRequest(r)
		res := s.runner.Run(username, 10*time.Second, "undo")
		s.cmdStore.Append(username, "Undo last action", []string{"undo"})

		if res.Err != nil || res.ExitCode != 0 || res.TimedOut {
			s.setFlash(w, "error", "Undo failed: "+res.Stderr)
		} else {
			s.setFlash(w, "success", "Last action undone")
		}

		referer := r.Header.Get("Referer")
		if referer == "" {
			referer = "/open?html=1"
		}
		http.Redirect(w, r, referer, http.StatusSeeOther)
	})
}

// ensureCSRFToken ensures a CSRF token cookie exists for the request and returns the token.
// If no token exists, generates a new one and sets it as a cookie.
func (s *Server) ensureCSRFToken(w http.ResponseWriter, r *http.Request) string {
	cookie, err := r.Cookie("csrf_token")
	if err == nil && cookie.Value != "" {
		return cookie.Value
	}
	// Generate new token
	token, err := generateCSRFToken()
	if err != nil {
		applog.Warnf("failed to generate CSRF token: %v", err)
		// Fallback to a simple token (less secure but better than nothing)
		token = fmt.Sprintf("%d", time.Now().UnixNano())
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    token,
		Path:     "/",
		MaxAge:   86400 * 7, // 7 days
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	return token
}

// helpers to prepare footer data
type footerEntry struct{ When, Context, Args string }

func (s *Server) footerData(r *http.Request, username string) (show bool, entries []footerEntry, moreURL string, canShowMore bool, returnURL string) {
	// cookie vs config default
	show = s.uiCfg.ShowCommandLog
	if c, err := r.Cookie("cmdlog"); err == nil {
		if c.Value == "off" {
			show = false
		} else if c.Value == "on" {
			show = true
		}
	}
	returnURL = r.URL.Path
	// count
	n := 5
	q := r.URL.Query()
	if q.Get("all") == "1" {
		n = 0
	} else if q.Get("more") == "1" {
		n = 20
	} else {
		canShowMore = true
		// build moreURL preserving query but setting more=1
		vals := r.URL.Query()
		vals.Set("more", "1")
		vals.Del("all")
		moreURL = r.URL.Path + "?" + vals.Encode()
	}
	if !show {
		return
	}
	raw := s.cmdStore.List(username, n)
	entries = make([]footerEntry, 0, len(raw))
	for _, e := range raw {
		ts := e.When.Format("15:04:05")
		entries = append(entries, footerEntry{When: ts, Context: e.Context, Args: ui.JoinArgs(e.Args)})
	}
	return
}

// flash support
type flash struct{ Type, Text string }

func (s *Server) setFlash(w http.ResponseWriter, typ, text string) {
	if typ == "" {
		typ = "info"
	}
	// simple cookie, short-lived
	http.SetCookie(w, &http.Cookie{Name: "flash", Value: urlQueryEscape(typ + "|" + text), Path: "/", MaxAge: 5})
}

func (s *Server) getFlash(r *http.Request) *flash {
	c, err := r.Cookie("flash")
	if err != nil || c.Value == "" {
		return nil
	}
	val := urlQueryUnescape(c.Value)
	parts := strings.SplitN(val, "|", 2)
	f := &flash{}
	if len(parts) == 2 {
		f.Type = parts[0]
		f.Text = parts[1]
	} else {
		f.Type = "info"
		f.Text = val
	}
	return f
}

func urlQueryEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "|", "/"), "\n", " ")
}
func urlQueryUnescape(s string) string { return s }

func (s *Server) Handler() http.Handler {
	// Basic Auth für alle außer /healthz
	protected := http.NewServeMux()
	protected.HandleFunc("/", s.mux.ServeHTTP)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			s.mux.ServeHTTP(w, r)
			return
		}
		realm := "dstask"
		authMiddleware := auth.BasicAuthMiddleware(s.userStore, realm, protected)
		authMiddleware.ServeHTTP(w, r)
	})
}
