(() => {
  'use strict';
  if (window.__hookInstalled) return;
  window.__hookInstalled = true;

  const TOPIC = '__wb';
  const MAGIC_DATA = 0xbe;
  const MAGIC_END = 0xed;
  const CHUNK = 8192;
  const log = (...a) => console.log('[HOOK]', ...a);

  const outDCs = { reliable: null, lossy: null };
  const inDCs = { reliable: null, lossy: null };

  function writeVarint(arr, n) {
    while (n >= 0x80) { arr.push((n & 0x7f) | 0x80); n = Math.floor(n / 128); }
    arr.push(n & 0x7f);
  }
  function readVarint(u8, o) {
    let n = 0, shift = 0, i = o.i;
    while (i < u8.length) {
      const b = u8[i++];
      n += (b & 0x7f) * Math.pow(2, shift);
      if (!(b & 0x80)) { o.i = i; return n; }
      shift += 7;
      if (shift > 49) return -1;
    }
    return -1;
  }
  function encodeDP(payload) {
    const topic = new TextEncoder().encode(TOPIC);
    const up = [];
    up.push(0x12); writeVarint(up, payload.length);
    for (let i = 0; i < payload.length; i++) up.push(payload[i]);
    up.push(0x22); writeVarint(up, topic.length);
    for (let i = 0; i < topic.length; i++) up.push(topic[i]);
    const dp = [];
    dp.push(0x12); writeVarint(dp, up.length);
    for (let i = 0; i < up.length; i++) dp.push(up[i]);
    return new Uint8Array(dp);
  }
  function skipField(u8, o, wt) {
    if (wt === 0) return readVarint(u8, o) >= 0;
    if (wt === 2) { const l = readVarint(u8, o); if (l < 0) return false; o.i += l; return true; }
    if (wt === 1) { o.i += 8; return true; }
    if (wt === 5) { o.i += 4; return true; }
    return false;
  }
  function decodeDP(buf) {
    const u8 = buf instanceof Uint8Array ? buf : new Uint8Array(buf);
    const o = { i: 0 };
    let userBuf = null;
    while (o.i < u8.length) {
      const tag = readVarint(u8, o); if (tag < 0) return null;
      const fn = tag >>> 3, wt = tag & 7;
      if (fn === 2 && wt === 2) {
        const l = readVarint(u8, o); if (l < 0) return null;
        userBuf = u8.subarray(o.i, o.i + l); o.i += l;
      } else if (!skipField(u8, o, wt)) return null;
    }
    if (!userBuf) return null;
    const oo = { i: 0 };
    let payload = null, topic = null;
    while (oo.i < userBuf.length) {
      const tag = readVarint(userBuf, oo); if (tag < 0) return null;
      const fn = tag >>> 3, wt = tag & 7;
      if (fn === 2 && wt === 2) {
        const l = readVarint(userBuf, oo); if (l < 0) return null;
        payload = userBuf.subarray(oo.i, oo.i + l); oo.i += l;
      } else if (fn === 4 && wt === 2) {
        const l = readVarint(userBuf, oo); if (l < 0) return null;
        topic = new TextDecoder().decode(userBuf.subarray(oo.i, oo.i + l)); oo.i += l;
      } else if (!skipField(userBuf, oo, wt)) return null;
    }
    return { payload: payload, topic: topic };
  }

  const benches = {
    reliable: { start: 0, bytes: 0, expected: 0, seen: 0, done: false },
    lossy: { start: 0, bytes: 0, expected: 0, seen: 0, done: false },
  };

  function handlePayload(payload, kind) {
    if (payload.length >= 9 && payload[0] === MAGIC_DATA) {
      const seq = (payload[1] << 24) | (payload[2] << 16) | (payload[3] << 8) | payload[4];
      const total = (payload[5] << 24) | (payload[6] << 16) | (payload[7] << 8) | payload[8];
      const b = benches[kind];
      if (seq === 0) {
        b.start = performance.now();
        b.bytes = 0;
        b.seen = 0;
        b.expected = total;
        b.done = false;
        log('bench started', kind, 'expecting', total, 'chunks');
      }
      b.bytes += payload.length;
      b.seen++;
      return;
    }
    if (payload.length >= 5 && payload[0] === MAGIC_END) {
      const b = benches[kind];
      if (b.done) return;
      b.done = true;
      const total = (payload[1] << 24) | (payload[2] << 16) | (payload[3] << 8) | payload[4];
      const dt = (performance.now() - b.start) / 1000;
      const mbps = (b.bytes * 8) / dt / 1e6;
      const lossPct = total ? ((total - b.seen) / total * 100) : 0;
      log('bench done', kind, 'recv', b.seen, 'of', total, 'chunks', b.bytes, 'bytes in', dt.toFixed(3), 's', mbps.toFixed(2), 'Mbps loss', lossPct.toFixed(2), 'pct');
      return;
    }
    log('recv', kind, payload.length, 'bytes', new TextDecoder().decode(payload.subarray(0, Math.min(64, payload.length))));
  }

  function bindOut(dc, kind) {
    if (outDCs[kind] === dc) return;
    outDCs[kind] = dc;
    if (dc.readyState === 'open') log('outgoing', kind, 'already open');
    dc.addEventListener('open', () => { log('outgoing', kind, 'open'); });
    dc.addEventListener('close', () => { log('outgoing', kind, 'closed'); if (outDCs[kind] === dc) outDCs[kind] = null; });
  }
  function bindIn(dc, kind) {
    if (inDCs[kind] === dc) return;
    inDCs[kind] = dc;
    dc.addEventListener('message', (e) => {
      const dec = decodeDP(e.data);
      if (!dec || dec.topic !== TOPIC || !dec.payload) return;
      handlePayload(dec.payload, kind);
    });
    dc.addEventListener('close', () => { if (inDCs[kind] === dc) inDCs[kind] = null; });
  }

  function labelKind(label) {
    if (label === '_reliable') return 'reliable';
    if (label === '_lossy') return 'lossy';
    return null;
  }

  const OrigPC = window.RTCPeerConnection;
  const origCreateDC = OrigPC.prototype.createDataChannel;
  OrigPC.prototype.createDataChannel = function (label, opts) {
    const dc = origCreateDC.call(this, label, opts);
    const kind = labelKind(label);
    if (kind) {
      log('captured outgoing', kind);
      dc.binaryType = 'arraybuffer';
      bindOut(dc, kind);
    }
    return dc;
  };
  window.RTCPeerConnection = function (config) {
    log('New PeerConnection created');
    const pc = new OrigPC(config);
    pc.addEventListener('datachannel', (e) => {
      const kind = labelKind(e.channel.label);
      if (kind) {
        log('captured incoming', kind);
        e.channel.binaryType = 'arraybuffer';
        bindIn(e.channel, kind);
      }
    });
    return pc;
  };
  Object.keys(OrigPC).forEach((k) => { window.RTCPeerConnection[k] = OrigPC[k]; });
  window.RTCPeerConnection.prototype = OrigPC.prototype;

  async function bench(totalBytes, kind) {
    kind = kind || 'reliable';
    const dc = outDCs[kind];
    if (!dc || dc.readyState !== 'open') { log('not open', kind); return; }

    const headerSize = 9;
    const dataSize = CHUNK - headerSize;
    const total = Math.ceil(totalBytes / dataSize);
    const buf = new Uint8Array(CHUNK);
    buf[0] = MAGIC_DATA;
    buf[5] = (total >>> 24) & 0xff;
    buf[6] = (total >>> 16) & 0xff;
    buf[7] = (total >>> 8) & 0xff;
    buf[8] = total & 0xff;
    for (let i = headerSize; i < CHUNK; i++) buf[i] = i & 0xff;

    dc.bufferedAmountLowThreshold = 256 * 1024;
    const t0 = performance.now();
    for (let seq = 0; seq < total; seq++) {
      while (dc.bufferedAmount > 1024 * 1024) {
        await new Promise((r) => dc.addEventListener('bufferedamountlow', r, { once: true }));
      }
      buf[1] = (seq >>> 24) & 0xff;
      buf[2] = (seq >>> 16) & 0xff;
      buf[3] = (seq >>> 8) & 0xff;
      buf[4] = seq & 0xff;
      dc.send(encodeDP(buf));
    }

    const end = new Uint8Array(5);
    end[0] = MAGIC_END;
    end[1] = (total >>> 24) & 0xff;
    end[2] = (total >>> 16) & 0xff;
    end[3] = (total >>> 8) & 0xff;
    end[4] = total & 0xff;
    for (let i = 0; i < 5; i++) {
      while (dc.bufferedAmount > 1024 * 1024) {
        await new Promise((r) => dc.addEventListener('bufferedamountlow', r, { once: true }));
      }
      dc.send(encodeDP(end));
    }

    const dt = (performance.now() - t0) / 1000;
    log('send done', kind, total, 'chunks', (total * CHUNK), 'bytes in', dt.toFixed(3), 's send-rate', ((total * CHUNK * 8) / dt / 1e6).toFixed(2), 'Mbps');
  }

  function send(text, kind) {
    kind = kind || 'reliable';
    const dc = outDCs[kind];
    if (!dc || dc.readyState !== 'open') { log('not open', kind); return; }
    const buf = typeof text === 'string' ? new TextEncoder().encode(text) : new Uint8Array(text);
    dc.send(encodeDP(buf));
  }

  window.__wb = {
    send: send,
    bench: bench,
    state: () => ({
      outReliable: !!outDCs.reliable && outDCs.reliable.readyState === 'open',
      outLossy: !!outDCs.lossy && outDCs.lossy.readyState === 'open',
      inReliable: !!inDCs.reliable,
      inLossy: !!inDCs.lossy,
    }),
  };
  log('test snippet installed - join call, then __wb.bench 10485760 reliable or __wb.bench 10485760 lossy');
})();
