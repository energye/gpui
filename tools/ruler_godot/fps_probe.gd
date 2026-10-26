extends Node
# Ruler-only FPS probe. Prints one machine-readable line every 2 seconds.
# t= is wall time since the autoload started, so the sampler can drop startup.

var _window := 0.0
var _total := 0.0


func _process(delta: float) -> void:
	_window += delta
	_total += delta
	if _window >= 2.0:
		print("GODOT_FPS=%.1f t=%.1f" % [Engine.get_frames_per_second(), _total])
		_window = 0.0
