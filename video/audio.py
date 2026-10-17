"""Synthesizes the trailer soundtrack (128 BPM, A minor, 36 bars + tail).

Everything is generated from numpy/scipy - no samples, nothing to license.
Section map (bars): 0-1 cold open riser | 2 impact + title | 2-5 groove |
6-9 +16th hats/arp | 10-17 full groove | 18-25 +lead | 26-27 breakdown |
28-29 snare-roll build | 30-33 DROP | 34 final hit | 35+ tail.
The same bar numbers drive the scene cuts in index.html.
"""
import os

import numpy as np
from scipy import signal
from scipy.io import wavfile

SR = 44100
BPM = 128
BEAT = 60 / BPM
BAR = BEAT * 4
BARS = 36
TAIL = 3.0
N = int((BARS * BAR + TAIL) * SR)
rng = np.random.default_rng(7)

# drum bus (never ducked), tonal bus (sidechained by the kick), reverb send
DL, DR = np.zeros(N), np.zeros(N)
TL, TR = np.zeros(N), np.zeros(N)
SL, SRV = np.zeros(N), np.zeros(N)
kicks = []


def tsec(bar, beat=0.0):
    return bar * BAR + beat * BEAT


def put(buf, x, t0, gain=1.0):
    i = int(t0 * SR)
    if i < 0 or i >= len(buf):
        return
    x = x[: len(buf) - i]
    buf[i : i + len(x)] += x * gain


def mix(x, t0, gain=1.0, pan=0.0, send=0.0, tonal=False):
    gl = gain * np.cos((pan + 1) * np.pi / 4)
    gr = gain * np.sin((pan + 1) * np.pi / 4)
    put(TL if tonal else DL, x, t0, gl)
    put(TR if tonal else DR, x, t0, gr)
    if send:
        put(SL, x, t0, gl * send)
        put(SRV, x, t0, gr * send)


def bp(x, lo, hi, order=3):
    return signal.sosfilt(signal.butter(order, [lo, hi], "bandpass", fs=SR, output="sos"), x)


def lp(x, f, order=3):
    return signal.sosfilt(signal.butter(order, f, "lowpass", fs=SR, output="sos"), x)


def hp(x, f, order=3):
    return signal.sosfilt(signal.butter(order, f, "highpass", fs=SR, output="sos"), x)


def tt(n):
    return np.arange(n) / SR


def midi(m):
    return 440 * 2 ** ((m - 69) / 12)


# ------------------------------------------------------------------ drums
def kick(vol=1.0):
    n = int(0.42 * SR)
    t = tt(n)
    f = 46 + 120 * np.exp(-t * 32)
    body = np.sin(2 * np.pi * np.cumsum(f) / SR) * np.exp(-t * 8.5)
    click = hp(rng.standard_normal(n), 2500) * np.exp(-t * 400) * 0.25
    return np.tanh((body + click) * 1.4) * vol


def clap(vol=1.0):
    n = int(0.3 * SR)
    t = tt(n)
    noise = bp(rng.standard_normal(n), 1100, 3800)
    bursts = sum(np.exp(-np.clip(t - d, 0, None) * 70) * (t >= d) for d in (0, 0.011, 0.022))
    env = bursts * 0.5 + np.exp(-t * 16) * 0.55
    return noise * env * vol * 1.6


def hat(open_=False, vol=1.0):
    n = int((0.28 if open_ else 0.07) * SR)
    t = tt(n)
    return hp(rng.standard_normal(n), 7500, 2) * np.exp(-t * (16 if open_ else 95)) * vol


def snare(vol=1.0):
    n = int(0.3 * SR)
    t = tt(n)
    tone = np.sin(2 * np.pi * 190 * t) * np.exp(-t * 28) * 0.5
    noise = bp(rng.standard_normal(n), 1500, 7500) * np.exp(-t * 22)
    return (tone + noise) * vol * 1.3


def impact(vol=1.0):
    n = int(3.2 * SR)
    t = tt(n)
    f = 38 + 90 * np.exp(-t * 6)
    boom = np.sin(2 * np.pi * np.cumsum(f) / SR) * np.exp(-t * 1.9)
    crash = hp(rng.standard_normal(n), 3500) * np.exp(-t * 2.6) * 0.5
    sub = lp(rng.standard_normal(n), 400) * np.exp(-t * 3) * 0.6
    return np.tanh(boom * 1.2 + crash + sub) * vol


def riser(dur, vol=1.0):
    n = int(dur * SR)
    k = tt(n) / dur
    noise = hp(rng.standard_normal(n), 1800, 2) * (k ** 2.2)
    f = 180 * (14 ** k)
    sweep = np.sin(2 * np.pi * np.cumsum(f) / SR) * (k ** 3) * 0.35
    return (noise * 0.7 + sweep) * vol


def whoosh(dur=0.9, vol=1.0):
    n = int(dur * SR)
    k = tt(n) / dur
    env = np.sin(np.pi * np.clip(k, 0, 1)) ** 2
    return bp(rng.standard_normal(n), 500, 9000, 2) * env * vol


# ------------------------------------------------------------------ tonal
def saw(f, n, detune=(0.0,)):
    t = tt(n)
    out = np.zeros(n)
    for d in detune:
        out += signal.sawtooth(2 * np.pi * f * (1 + d) * t + rng.uniform(0, 6.28))
    return out / len(detune)


def gate(t, dur, rel=0.03):
    return np.clip((dur - t) / rel, 0, 1)


def bass_note(f, dur, vol=1.0):
    n = int(dur * SR)
    t = tt(n)
    x = lp(saw(f, n), 380, 4) * 0.8 + np.sin(2 * np.pi * f * t) * 0.9
    env = np.minimum(t / 0.004, 1) * np.exp(-t * 3.2) * gate(t, dur)
    return np.tanh(x * env * 1.3) * vol


def pluck(f, dur=0.35, vol=1.0, bright=3600):
    n = int(dur * SR)
    t = tt(n)
    x = saw(f, n, (-0.004, 0.004)) * 0.6 + signal.square(2 * np.pi * f * t) * 0.25
    return lp(x, bright, 2) * np.exp(-t * 11) * np.minimum(t / 0.002, 1) * vol


def lead_note(f, dur, vol=1.0):
    n = int(dur * SR)
    t = tt(n)
    vib = 1 + 0.0035 * np.sin(2 * np.pi * 5.2 * t) * np.minimum(t / 0.2, 1)
    x = lp(saw(f, n, (-0.006, 0.0, 0.006)) * vib, 4200, 2)
    return x * np.minimum(t / 0.01, 1) * np.exp(-t * 4.5) * gate(t, dur, 0.04) * vol


def pad_chord(freqs, dur, vol=1.0):
    n = int(dur * SR)
    t = tt(n)
    x = sum(saw(f, n, (-0.007, 0.0, 0.007)) for f in freqs) / len(freqs)
    x = lp(x, 1500, 2)
    return x * np.minimum(t / 0.35, 1) * gate(t, dur, 0.4) * vol


# A minor progression, one chord per bar: Am F C G
CHORDS = [
    dict(root=33, tones=[57, 60, 64]),
    dict(root=29, tones=[57, 60, 65]),
    dict(root=36, tones=[55, 60, 64]),
    dict(root=31, tones=[55, 59, 62]),
]


def chord(bar):
    return CHORDS[bar % 4]


# ------------------------------------------------------------------ arrangement
def add_kick(t0, vol=1.0):
    kicks.append((t0, vol))


def groove(bar, level):
    for b in range(4):
        add_kick(tsec(bar, b))
        if b in (1, 3):
            mix(clap(1.5), tsec(bar, b), 1.0, 0.0, send=0.35)
        mix(hat(False, 1.1), tsec(bar, b + 0.5), 1.0, 0.25)
        if level >= 2:
            mix(hat(False, 0.6), tsec(bar, b + 0.25), 1.0, -0.2)
            mix(hat(False, 0.6), tsec(bar, b + 0.75), 1.0, 0.2)
    mix(hat(True, 1.0), tsec(bar, 3.5), 1.0, 0.1)


def bass(bar, level):
    f = midi(chord(bar)["root"])
    if level >= 3:
        for i in range(8):
            if i % 2 == 1 or level >= 4:
                oct_ = 2 if i in (5, 7) else 1
                mix(bass_note(f * oct_, BEAT * 0.45, 0.5), tsec(bar, i * 0.5), tonal=True)
    else:
        for b in range(4):
            mix(bass_note(f, BEAT * 0.8, 0.48), tsec(bar, b + 0.5), tonal=True)


def arp(bar, vol=0.5, octave=12):
    tones = chord(bar)["tones"]
    notes = [tones[0], tones[1], tones[2], tones[1] + 12, tones[2], tones[1], tones[0] + 12, tones[2]]
    for i in range(16):
        m = notes[i % 8] + octave
        mix(pluck(midi(m), 0.3, vol * 2.3), tsec(bar, i * 0.25), 1.0, -0.45 if i % 2 else 0.45, send=0.25, tonal=True)


def pad(bar, vol=0.3):
    c = chord(bar)
    mix(pad_chord([midi(m) for m in c["tones"]], BAR, vol * 1.5), tsec(bar), 1.0, 0.0, send=0.4, tonal=True)


def lead(bar, vol=0.4):
    tones = chord(bar)["tones"]
    pool = [tones[2] + 12, tones[1] + 12, tones[0] + 12, tones[2] + 24]
    pattern = [0, 0, 1, 2, 1, 0, 2, 3] if bar % 2 == 0 else [3, 2, 1, 2, 0, 1, 0, 0]
    for i, p in enumerate(pattern):
        mix(lead_note(midi(pool[p]), BEAT * 0.48, vol * 1.9), tsec(bar, i * 0.5), 1.0, 0.0, send=0.3, tonal=True)


def stab(bar, vol=0.28):
    freqs = [midi(m + 12) for m in chord(bar)["tones"]]
    n = int(0.16 * SR)
    t = tt(n)
    for beat in (0.5, 2.5, 3.5):
        x = sum(lp(saw(f, n), 2600, 2) for f in freqs) / 3 * np.exp(-t * 16)
        mix(x, tsec(bar, beat), vol * 2.0, 0.15, send=0.3, tonal=True)


# cold open: heartbeat + atmosphere + riser
for b in range(8):
    add_kick(tsec(0, b), 0.35 + 0.08 * b)
mix(pad_chord([midi(m) for m in chord(0)["tones"]], BAR * 2.2, 0.35), 0, 1.0, 0.0, send=0.5, tonal=True)
mix(riser(BAR * 2, 0.8), 0, 1.0, 0.0, send=0.3)
mix(whoosh(0.9, 0.5), tsec(1, 2.0), 1.0, 0.0, send=0.3)

mix(impact(1.0), tsec(2), 1.0, 0.0, send=0.35)          # title hit

for bar in range(2, 34):
    level = 1 if bar < 6 else 2 if bar < 10 else 3
    if 26 <= bar < 28:                                   # breakdown: air, no kick
        for b in range(4):
            mix(hat(False, 0.4), tsec(bar, b + 0.5), 1.0, 0.2)
        pad(bar, 0.45)
        arp(bar, 0.32, 12)
        continue
    if 28 <= bar < 30:                                   # build under the snare roll
        pad(bar, 0.45)
        arp(bar, 0.34, 12)
        add_kick(tsec(bar, 0), 0.8)
        continue
    groove(bar, level)
    if bar >= 4:
        bass(bar, 3 if bar >= 10 else 1)
        arp(bar, 0.42 if bar >= 6 else 0.3, 12)
    pad(bar, 0.3)
    if bar >= 10:
        stab(bar)
    if 18 <= bar < 26 or bar >= 30:
        lead(bar, 0.34 if bar < 30 else 0.4)
    if bar >= 30:
        bass(bar, 4)
        arp(bar, 0.3, 24)

# snare roll: 8ths -> 16ths -> 32nds, volume ramping up
roll = [tsec(28, i * 0.5) for i in range(8)]
roll += [tsec(29, i * 0.25) for i in range(8)]
roll += [tsec(29, 2 + i * 0.125) for i in range(16)]
for i, t0 in enumerate(roll):
    mix(snare(0.25 + 0.6 * i / len(roll)), t0, 1.0, 0.0, send=0.25)
mix(riser(BAR * 2, 0.9), tsec(28), 1.0, 0.0, send=0.2)

# scene-change sweetener: whoosh just before each cut, small hit on it
for bar in (6, 10, 16, 22, 26):
    mix(whoosh(BEAT * 1.6, 0.55), tsec(bar) - BEAT * 1.4, 1.0, 0.0, send=0.3)
    mix(impact(0.38), tsec(bar), 1.0, 0.0, send=0.2)
mix(impact(1.0), tsec(30), 1.0, 0.0, send=0.4)          # the drop
for bar in (12, 14, 18, 20, 24):                         # end-of-phrase fills
    for i in range(4):
        mix(snare(0.35 + 0.1 * i), tsec(bar, 3 + i * 0.25), 1.0, 0.0, send=0.2)

# finale
mix(impact(1.1), tsec(34), 1.0, 0.0, send=0.5)
mix(pad_chord([midi(m) for m in (57, 60, 64, 69)], BAR * 2 + TAIL, 0.55), tsec(34), 1.0, 0.0, send=0.5, tonal=True)
mix(pluck(midi(81), 1.8, 0.5), tsec(34), 1.0, 0.3, send=0.5, tonal=True)
add_kick(tsec(34), 1.0)

# ------------------------------------------------------------------ sidechain + kick bus
duck = np.ones(N)
KL, KR = np.zeros(N), np.zeros(N)
for t0, vol in kicks:
    i = int(t0 * SR)
    seg = min(int(0.28 * SR), N - i)
    if seg > 0:
        duck[i : i + seg] = np.minimum(duck[i : i + seg], 1 - 0.78 * np.exp(-tt(seg) * 11))
    kk = kick(vol * 0.8)
    put(KL, kk, t0)
    put(KR, kk, t0)


def make_ir(seed, length=2.2):
    g = np.random.default_rng(seed)
    n = int(length * SR)
    ir = lp(g.standard_normal(n) * np.exp(-tt(n) * 2.6), 7000, 2)
    ramp = int(0.012 * SR)
    ir[:ramp] *= np.linspace(0, 1, ramp)
    return ir / np.sqrt(np.sum(ir ** 2))


wetL = signal.fftconvolve(SL, make_ir(1))[:N]
wetR = signal.fftconvolve(SRV, make_ir(2))[:N]


def echo(x, delay, fb, taps=4):
    out = x.copy()
    d = int(delay * SR)
    for k in range(1, taps + 1):
        out[d * k :] += x[: N - d * k] * (fb ** k)
    return out


tonL = echo(TL * duck, BEAT * 0.75, 0.28)
tonR = echo(TR * duck, BEAT * 1.0, 0.28)
left = DL * 0.9 + KL * 1.0 + tonL * 0.95 + wetL * 0.7
right = DR * 0.9 + KR * 1.0 + tonR * 0.95 + wetR * 0.7

out = np.stack([hp(left, 28, 2), hp(right, 28, 2)], axis=1)
out = np.tanh(out * 1.1)
out *= 0.93 / np.max(np.abs(out))
fi = int(0.03 * SR)
out[:fi] *= np.linspace(0, 1, fi)[:, None]
fo = int(1.8 * SR)
out[-fo:] *= np.linspace(1, 0, fo)[:, None]

os.makedirs("out", exist_ok=True)
wavfile.write("out/music.wav", SR, (out * 32767).astype(np.int16))

seconds = len(out) / SR
print(f"wrote out/music.wav  {seconds:.1f}s  peak {np.max(np.abs(out)):.2f}  "
      f"rms {np.sqrt(np.mean(out ** 2)):.3f}  nan={bool(np.isnan(out).any())}  kicks={len(kicks)}")
