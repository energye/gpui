// Package audio freezes the CPU-side 2D positional math every 2.5D play feeds.
//
// Frozen 2026-09-15 (capability 15.1, P0, S13/W1): PosSound, NewPosSound,
// Mix, MixAll, Stereo, DefaultAttenuation, DefaultPanStrength.
// Additive changes only.
//
// Frozen 2026-09-15 (capability 15.2, P0, S14/W1): MusicPlayer,
// NewMusicPlayer, Play, Push, Pop, CrossfadeTo, Stop, Update, Depth, Top,
// From, IsFading, Progress, Blend, Bus, NewBus, Name, Volume, SetVolume,
// SetMute, Muted, Gain, Mixer, NewMixer, MaxVoices, SetMaxVoices, Mix,
// MixResult, Ducker, NewDucker, Trigger, Active, Depth, Gain,
// CombineGains, DefaultBusVolume, DefaultMaxVoices, MaxMixerVoices,
// DefaultDuckDepth, DefaultFade, MaxMusicStack. Additive changes only.
//
// This package only computes numbers; it plays nothing. The caller feeds
// the returned gain and pan into the platform audio backend (or a test
// oscillator). Only core numbers are used; no new Vec2/Color/AssetID is
// defined here. The old render and ui paths stay untouched.
//
// Math (main reference Godot AudioStreamPlayer2D three knobs, Go replay):
//
//	d = |source - listener| (Euclidean game units)
//	t = d / Range, Range > 0
//	gain = 1 inside Range when Range == 0 or Attenuation == 0,
//	       0 when d >= Range,
//	       pow(1-t, Attenuation) otherwise (1 linear, 2 squared)
//	pan = clamp(dx*PanStrength, -1, 1) when Range == 0,
//	      clamp(dx/Range*PanStrength, -1, 1) otherwise
//	      (dx = source.x - listener.x; -1 left, +1 right, 0 center)
//	stereo = linear balance: l = mono*gain*min(1,1-pan),
//	         r = mono*gain*min(1,1+pan)
//
// Intentional differences from Godot (documented, not drift):
//   - Distance is Euclidean. Godot 2D only looks at the horizontal gap;
//     2.5D needs near/far depth (Y) to hush far sounds, so Y joins the mix.
//   - Falloff is pow(1-t, att): bigger att hushes faster (1 linear,
//     2 squared). att 0 means no falloff inside the range.
//   - Range 0 means never attenuate (gain 1, Godot parity) but pan still
//     applies; UI clicks should use PanStrength 0 for mono.
//   - Pan sign: source right of the listener pans right (+1). Center is 0.
//
// Errors use game/core codes: bad constructor arguments are InvalidArg
// and store nothing. Mix reports ok=false for bad inputs (non-finite
// positions, negative or non-finite range/attenuation/strength) with a
// silent mix, never NaN and never a panic. Out-of-range is valid data:
// ok=true with gain 0 and Audible=false, so callers tell far apart from bad.
//
// Music chain (capability 15.2, main reference Godot music buses plus a
// game music stack, Go replay): the player is a LIFO stack of AssetID
// with linear crossfade; buses carry volume plus mute; the mixer keeps
// the loudest MaxVoices and ceilings at 1; the ducker holds music down
// while hits ring then climbs back; CombineGains multiplies one voice by
// its bus and duck gains. Empty track ids never play: Play/Push/
// CrossfadeTo reject them with InvalidArg and keep the old stack.
// Pop on an empty stack is NotFound. Push beyond MaxMusicStack is
// OutOfMemory. New switches while fading snap the previous fade to its
// target first, then restart from full target, so gains never jump
// through NaN. Crossfade gains are 1-t/t; fade-ins climb from silence,
// fade-outs fall to silence. Nil receivers never panic: players report
// silence, buses report 0, mixers report zero, duckers report 1.
package audio
