// KitsuneDE Procedural WebAudio Anime Lo-Fi & Synthwave Engine
(function () {
  const TRACKS = [
    {
      id: 'neon-drive',
      title: 'Neo-Tokyo Midnight Rain',
      jp: 'ネオ東京・夜雨',
      bpm: 84,
      wave: 'triangle',
      chords: [
        [220.0, 261.63, 329.63, 392.0],  // Am7
        [174.61, 220.0, 261.63, 329.63], // Fmaj7
        [196.0, 246.94, 293.66, 392.0],  // G6
        [164.81, 196.0, 246.94, 329.63], // Em7
      ],
      arp: [523.25, 659.25, 783.99, 659.25, 587.33, 523.25, 440.0, 493.88],
    },
    {
      id: 'sakura-tea',
      title: 'Sakura Rooftop Lofi',
      jp: '桜色の放課後',
      bpm: 72,
      wave: 'sine',
      chords: [
        [261.63, 329.63, 392.0, 493.88], // Cmaj7
        [220.0, 261.63, 329.63, 392.0],  // Am7
        [174.61, 220.0, 261.63, 349.23], // Fmaj7
        [196.0, 246.94, 293.66, 349.23], // G7
      ],
      arp: [523.25, 587.33, 659.25, 783.99, 659.25, 587.33, 523.25, 392.0],
    },
    {
      id: 'crimson-katana',
      title: 'Crimson Moon Shinobi',
      jp: '紅蓮の月影',
      bpm: 98,
      wave: 'sawtooth',
      chords: [
        [146.83, 220.0, 293.66, 349.23], // Dm
        [116.54, 174.61, 233.08, 293.66],// Bbmaj7
        [130.81, 196.0, 261.63, 329.63], // C
        [110.0, 164.81, 220.0, 277.18],  // A
      ],
      arp: [587.33, 698.46, 880.0, 698.46, 659.25, 587.33, 523.25, 440.0],
    },
    {
      id: 'eva-geofront',
      title: 'Tokyo-3 GeoFront Pulse',
      jp: '第3新東京・作戦室',
      bpm: 92,
      wave: 'triangle',
      chords: [
        [164.81, 246.94, 329.63, 392.0], // Em
        [130.81, 196.0, 261.63, 392.0],  // Cadd9
        [146.83, 220.0, 293.66, 369.99], // D
        [123.47, 185.0, 246.94, 329.63], // Bm7
      ],
      arp: [659.25, 783.99, 987.77, 783.99, 739.99, 659.25, 587.33, 493.88],
    },
  ];

  class KitsuneSynth {
    constructor() {
      this.ctx = null;
      this.master = null;
      this.analyser = null;
      this.playing = false;
      this.trackIndex = 0;
      this.volume = 0.28;
      this.step = 0;
      this.timer = null;
      this.listeners = new Set();
    }

    tracks() {
      return TRACKS;
    }

    currentTrack() {
      return TRACKS[this.trackIndex] || TRACKS[0];
    }

    onChange(fn) {
      this.listeners.add(fn);
      return () => this.listeners.delete(fn);
    }

    emit() {
      const state = {
        playing: this.playing,
        track: this.currentTrack(),
        trackIndex: this.trackIndex,
        volume: this.volume,
      };
      this.listeners.forEach((fn) => fn(state));
    }

    ensureContext() {
      if (!this.ctx) {
        const AudioCtx = window.AudioContext || window.webkitAudioContext;
        if (!AudioCtx) return false;
        this.ctx = new AudioCtx();
        this.master = this.ctx.createGain();
        this.master.gain.value = this.volume;
        this.analyser = this.ctx.createAnalyser();
        this.analyser.fftSize = 64;
        this.master.connect(this.analyser);
        this.analyser.connect(this.ctx.destination);
      }
      if (this.ctx.state === 'suspended') {
        this.ctx.resume();
      }
      return true;
    }

    setVolume(val) {
      this.volume = Math.max(0, Math.min(1, val));
      if (this.master && this.ctx) {
        this.master.gain.setTargetAtTime(this.volume, this.ctx.currentTime, 0.05);
      }
      this.emit();
    }

    selectTrack(idx) {
      this.trackIndex = (idx + TRACKS.length) % TRACKS.length;
      this.step = 0;
      if (this.playing) {
        this.restartLoop();
      }
      this.emit();
    }

    toggle() {
      if (this.playing) {
        this.stop();
      } else {
        this.play();
      }
    }

    play() {
      if (!this.ensureContext()) return;
      this.playing = true;
      this.restartLoop();
      this.emit();
    }

    stop() {
      this.playing = false;
      if (this.timer) {
        clearInterval(this.timer);
        this.timer = null;
      }
      this.emit();
    }

    restartLoop() {
      if (this.timer) clearInterval(this.timer);
      const track = this.currentTrack();
      const stepMs = (60 / track.bpm / 2) * 1000; // eighth notes
      this.tick();
      this.timer = setInterval(() => this.tick(), stepMs);
    }

    tick() {
      if (!this.playing || !this.ctx) return;
      const track = this.currentTrack();
      const now = this.ctx.currentTime;

      // Play warm pad chord every 8 eighth-notes
      if (this.step % 8 === 0) {
        const chord = track.chords[Math.floor(this.step / 8) % track.chords.length];
        chord.forEach((freq) => {
          const osc = this.ctx.createOscillator();
          const filter = this.ctx.createBiquadFilter();
          const gain = this.ctx.createGain();
          osc.type = track.wave;
          osc.frequency.setValueAtTime(freq, now);
          filter.type = 'lowpass';
          filter.frequency.setValueAtTime(900, now);

          gain.gain.setValueAtTime(0.001, now);
          gain.gain.linearRampToValueAtTime(0.09, now + 0.18);
          gain.gain.exponentialRampToValueAtTime(0.001, now + 2.4);

          osc.connect(filter);
          filter.connect(gain);
          gain.connect(this.master);
          osc.start(now);
          osc.stop(now + 2.45);
        });
      }

      // Gentle pentatonic bell note
      const noteFreq = track.arp[this.step % track.arp.length];
      if (noteFreq && (this.step % 2 === 0 || Math.random() > 0.35)) {
        const osc = this.ctx.createOscillator();
        const gain = this.ctx.createGain();
        osc.type = 'sine';
        osc.frequency.setValueAtTime(noteFreq, now);
        gain.gain.setValueAtTime(0.001, now);
        gain.gain.linearRampToValueAtTime(0.08, now + 0.03);
        gain.gain.exponentialRampToValueAtTime(0.001, now + 0.55);
        osc.connect(gain);
        gain.connect(this.master);
        osc.start(now);
        osc.stop(now + 0.6);
      }

      this.step++;
    }

    readSpectrum(outArray) {
      if (!this.analyser || !this.playing) {
        outArray.fill(4);
        return;
      }
      this.analyser.getByteFrequencyData(outArray);
    }
  }

  window.KitsuneAudio = new KitsuneSynth();
})();
