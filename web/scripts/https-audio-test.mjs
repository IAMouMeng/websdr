#!/usr/bin/env node
/**
 * HTTPS audio smoke test — serves dist + mock /ws, clicks Play, checks RMS.
 * Usage: node scripts/https-audio-test.mjs
 */
import { createServer as createHttpsServer } from 'node:https';
import { createServer as createHttpServer } from 'node:http';
import { readFileSync, existsSync, mkdirSync } from 'node:fs';
import { join, dirname, extname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execSync } from 'node:child_process';
import { spawn } from 'node:child_process';
import { WebSocketServer } from 'ws';

const __dir = dirname(fileURLToPath(import.meta.url));
const root = join(__dir, '..');
const dist = join(root, 'dist');
const audioSrc = join(root, 'src/utils/audio.js');
const certDir = join(root, '.test-certs');
const port = 18443;

const MSG_AUDIO = 0x02;
const hosts = ['127.0.0.1', 'localhost'];

const MIME = {
  '.html': 'text/html',
  '.js': 'text/javascript',
  '.css': 'text/css',
  '.json': 'application/json',
  '.png': 'image/png',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
};

function ensureCerts() {
  mkdirSync(certDir, { recursive: true });
  const key = join(certDir, 'key.pem');
  const cert = join(certDir, 'cert.pem');
  if (!existsSync(key)) {
    execSync(
      `openssl req -x509 -newkey rsa:2048 -keyout "${key}" -out "${cert}" -days 7 -nodes -subj "/CN=localhost"`,
      { stdio: 'inherit' },
    );
  }
  return {
    key: readFileSync(key),
    cert: readFileSync(cert),
  };
}

function makeAudioFrame(tick = 0) {
  const n = 2400;
  const buf = Buffer.alloc(3 + n * 2);
  buf[0] = MSG_AUDIO;
  buf.writeUInt16LE(48000, 1);
  for (let i = 0; i < n; i++) {
    const phase = (tick * n + i) / 48000;
    const s = Math.sin(2 * Math.PI * 440 * phase) * 0.85;
    buf.writeInt16LE(Math.round(s * 32767), 3 + i * 2);
  }
  return buf;
}

function statusJSON() {
  return JSON.stringify({
    type: 'status',
    centerFreq: 100_000_000,
    tuneFreq: 100_000_000,
    sampleRate: 2_048_000,
    filterBW: 150_000,
    enabled: true,
    mode: 'wfm',
    gain: 20,
  });
}

function startServer(tls, listenPort = port) {
  const wss = new WebSocketServer({ noServer: true });
  let tick = 0;

  wss.on('connection', (ws) => {
    ws.send(statusJSON());
    const iv = setInterval(() => {
      if (ws.readyState === ws.OPEN) ws.send(makeAudioFrame(tick++));
    }, 50);
    ws.on('close', () => clearInterval(iv));
  });

  const handler = (req, res) => {
    const url = new URL(req.url, `https://127.0.0.1:${port}`);
    let filePath = url.pathname;

    if (filePath === '/utils/audio.js') {
      res.writeHead(200, { 'Content-Type': 'text/javascript' });
      res.end(readFileSync(audioSrc));
      return;
    }

    if (filePath === '/audio-test.html') {
      res.writeHead(200, { 'Content-Type': 'text/html' });
      res.end(`<!DOCTYPE html>
<html><body>
<button id="play">Play</button>
<pre id="out">idle</pre>
<script type="module">
import { AudioPlayer } from '/utils/audio.js';
const audio = new AudioPlayer(48000);
window.__audio = audio;
document.getElementById('play').onclick = async () => {
  audio.unlockFromGesture();
  await audio.start();
  let t = 0;
  setInterval(() => {
    const n = 2400;
    const pcm = new Float32Array(n);
    for (let i = 0; i < n; i++) pcm[i] = Math.sin(2 * Math.PI * 440 * (t++ / 48000)) * 0.8;
    audio.push(pcm);
  }, 50);
  document.getElementById('out').textContent = 'playing mode=' + audio.mode + ' ctx=' + audio.ctx.state;
};
</script>
</body></html>`);
      return;
    }

    if (filePath === '/ws') return; // upgraded below

    if (filePath === '/' || filePath === '') filePath = '/index.html';
    const abs = join(dist, filePath);
    if (!abs.startsWith(dist) || !existsSync(abs)) {
      // SPA fallback
      const index = join(dist, 'index.html');
      if (existsSync(index)) {
        res.writeHead(200, { 'Content-Type': 'text/html' });
        res.end(readFileSync(index));
        return;
      }
      res.writeHead(404);
      res.end('not found');
      return;
    }
    const ext = extname(abs);
    res.writeHead(200, { 'Content-Type': MIME[ext] || 'application/octet-stream' });
    res.end(readFileSync(abs));
  };

  const server = tls
    ? createHttpsServer(tls, handler)
    : createHttpServer(handler);

  server.on('upgrade', (req, socket, head) => {
    if (req.url !== '/ws') {
      socket.destroy();
      return;
    }
    wss.handleUpgrade(req, socket, head, (ws) => wss.emit('connection', ws, req));
  });

  return new Promise((resolve) => {
    server.listen(listenPort, () => resolve(server));
  });
}

async function runPlaywright(host, path, label) {
  const { chromium } = await import('playwright');
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ ignoreHTTPSErrors: true });
  const page = await context.newPage();
  const logs = [];
  page.on('console', (m) => logs.push(m.text()));
  page.on('pageerror', (e) => logs.push('PAGEERROR: ' + e.message));

  const url = `https://${host}:${port}${path}`;
  await page.goto(url, { waitUntil: 'networkidle', timeout: 30000 });

  if (path === '/radio') {
    await page.waitForSelector('button[title="播放"]', { timeout: 15000 });
    await page.click('button[title="播放"]');
  } else {
    await page.click('#play');
  }

  await page.waitForTimeout(2500);

  const result = await page.evaluate(() => {
    const audio = window.__audio;
    if (audio) {
      return {
        kind: 'isolated',
        playing: audio.playing,
        mode: audio.mode,
        ctx: audio.ctx?.state,
        db: audio.getLevelDb(),
      };
    }
    // Full app: patch via Vue provide is hard; probe AudioContext instances.
    const ctxs = window.__audioProbe || {};
    return { kind: 'app', ...ctxs, hasPlay: !!document.querySelector('button[title="播放"]') };
  });

  // Probe running audio contexts in full-app test
  if (result.kind === 'app') {
    const probe = await page.evaluate(() => {
      let db = -Infinity;
      let ctxState = 'none';
      let mode = 'unknown';
      // Walk globals set by our init hook
      if (window.__sdrAudioRef) {
        const a = window.__sdrAudioRef;
        return {
          playing: a.playing,
          mode: a.mode,
          ctx: a.ctx?.state,
          db: a.getLevelDb(),
        };
      }
      return { playing: false, mode, ctx: ctxState, db };
    });
    Object.assign(result, probe);
  }

  await browser.close();
  return { label, url, result, logs };
}

async function main() {
  if (!existsSync(join(dist, 'index.html'))) {
    console.error('Run: cd web && npm run build');
    process.exit(1);
  }

  // Install deps if needed
  try {
    await import('playwright');
    await import('ws');
  } catch {
    console.log('Installing playwright + ws...');
    execSync('npm install -D playwright ws', { cwd: root, stdio: 'inherit' });
    execSync('npx playwright install chromium', { cwd: root, stdio: 'inherit' });
  }

  const tls = ensureCerts();
  const httpPort = 18080;
  const httpsServer = await startServer(tls, port);
  const httpServer = await startServer(null, httpPort);
  console.log(`HTTPS test server https://127.0.0.1:${port}`);
  console.log(`HTTP  test server http://127.0.0.1:${httpPort}`);

  const { chromium } = await import('playwright');
  const browser = await chromium.launch({ headless: true });

  const cases = [];
  for (const host of hosts) {
    cases.push({ host, path: '/audio-test.html', label: `HTTPS ${host} isolated`, scheme: 'https', listenPort: port });
    cases.push({ host, path: '/radio', label: `HTTPS ${host} /radio`, scheme: 'https', listenPort: port });
    cases.push({ host, path: '/radio', label: `HTTP  ${host} /radio`, scheme: 'http', listenPort: httpPort });
  }

  const results = [];
  for (const c of cases) {
    const context = await browser.newContext({ ignoreHTTPSErrors: true });
    await context.addInitScript(() => {
      const orig = window.WebSocket;
      window.WebSocket = function (url, protocols) {
        const ws = new orig(url, protocols);
        return ws;
      };
      // Expose audio once Vue mounts (poll)
      const iv = setInterval(() => {
        const btn = document.querySelector('button[title="播放"]');
        if (!btn || window.__sdrAudioRef) return;
        const app = document.querySelector('#app')?.__vue_app__;
        if (!app) return;
        // Walk component tree for audio player instance
        const walk = (inst) => {
          if (!inst) return null;
          if (inst.setupState?.audio?.push) return inst.setupState.audio;
          if (inst.ctx?.audio?.push) return inst.ctx.audio;
          if (inst.subTree?.component) {
            const r = walk(inst.subTree.component);
            if (r) return r;
          }
          if (inst.subTree?.children) {
            for (const ch of inst.subTree.children) {
              if (ch?.component) {
                const r = walk(ch.component);
                if (r) return r;
              }
            }
          }
          return null;
        };
        const audio = walk(app._instance);
        if (audio) {
          window.__sdrAudioRef = audio;
          clearInterval(iv);
        }
      }, 100);
    });

    const page = await context.newPage();
    const logs = [];
    page.on('console', (m) => logs.push(m.text()));
    page.on('pageerror', (e) => logs.push('ERR: ' + e.message));

    const url = `${c.scheme}://${c.host}:${c.listenPort}${c.path}${c.path === '/radio' ? '?audiodebug=1' : ''}`;
    process.stdout.write(`Testing ${c.label} ... `);
    try {
      await page.goto(url, { waitUntil: 'networkidle', timeout: 30000 });
      if (c.path === '/radio') {
        await page.waitForSelector('button[title="播放"]', { timeout: 15000 });
        await page.click('button[title="播放"]');
      } else {
        await page.click('#play');
      }
      await page.waitForTimeout(3000);

      const result = await page.evaluate(() => {
        const audio = window.__audio || window.__sdrAudioRef;
        if (!audio) return { error: 'no audio ref', playing: false, db: -Infinity };
        return {
          playing: audio.playing,
          mode: audio.mode,
          ctx: audio.ctx?.state,
          db: audio.getLevelDb(),
          isSecure: window.isSecureContext,
        };
      });

      const ok = result.playing && result.ctx === 'running' && result.db > -50;
      console.log(ok ? 'PASS' : 'FAIL', JSON.stringify(result));
      if (!ok) console.log('  logs:', logs.filter((l) => /audio|Audio|worklet|ERR/i.test(l)).slice(0, 8));
      results.push({ ...c, result, ok, logs });
    } catch (err) {
      console.log('ERROR', err.message);
      results.push({ ...c, error: err.message, ok: false, logs });
    }
    await context.close();
  }

  await browser.close();
  httpsServer.close();
  httpServer.close();

  console.log('\n=== Summary ===');
  for (const r of results) {
    console.log(`${r.ok ? '✓' : '✗'} ${r.label}`);
  }

  const failed = results.filter((r) => !r.ok);
  process.exit(failed.length ? 1 : 0);
}

main().catch((e) => {
  console.error(e);
  process.exit(1);
});
