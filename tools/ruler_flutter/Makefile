FLUTTER_SDK ?= $(HOME)/app/projects/gogpu/flutter
FLUTTER ?= $(FLUTTER_SDK)/bin/flutter
BUNDLE := build/linux/x64/release/bundle/ruler_flutter
PYTHON ?= python3
DURATION ?= 60
WARMUP ?= 5

.PHONY: build analyze run-spin run-list run-text measure-spin measure-list measure-text run-full run-grid run-boundary run-overlay run-resize run-min run-cp measure-full measure-grid measure-boundary measure-overlay measure-resize measure-min measure-cp

build:
	$(FLUTTER) pub get
	$(FLUTTER) build linux --release

analyze:
	$(FLUTTER) analyze

run-spin: build
	RULER_SCENE=spin RULER_SECONDS=$${RULER_SECONDS:-0} ./$(BUNDLE)

run-list: build
	RULER_SCENE=list RULER_SECONDS=$${RULER_SECONDS:-0} ./$(BUNDLE)

run-text: build
	RULER_SCENE=text RULER_SECONDS=$${RULER_SECONDS:-0} ./$(BUNDLE)

measure-spin: build
	$(PYTHON) scripts/measure.py --scene spin --window-match RULER_SPIN \
		--duration $(DURATION) --warmup $(WARMUP) --out out/spin

measure-list: build
	$(PYTHON) scripts/measure.py --scene list --window-match RULER_LIST \
		--duration $(DURATION) --warmup 20 --out out/list

measure-text: build
	$(PYTHON) scripts/measure.py --scene text --window-match RULER_TEXT \
		--duration $(DURATION) --warmup $(WARMUP) --out out/text

# P4-A rows 4/5/6/8/7/9/13 arms. Runs are 1200x800 like the gpui counterparts.
# The resize/min arms rely on runner-side RULER_ACTIONS (GTK resize/iconify at
# scheduled times); see scripts/README P4 section.
run-full: build
	RULER_SCENE=full RULER_SECONDS=$${RULER_SECONDS:-0} ./$(BUNDLE)

run-grid: build
	RULER_SCENE=grid RULER_SECONDS=$${RULER_SECONDS:-0} ./$(BUNDLE)

run-boundary: build
	RULER_SCENE=boundary RULER_SECONDS=$${RULER_SECONDS:-0} ./$(BUNDLE)

run-overlay: build
	RULER_SCENE=overlay RULER_SECONDS=$${RULER_SECONDS:-0} ./$(BUNDLE)

run-resize: build
	RULER_SCENE=resize RULER_SECONDS=$${RULER_SECONDS:-0} ./$(BUNDLE)

run-min: build
	RULER_SCENE=min RULER_SECONDS=$${RULER_SECONDS:-0} ./$(BUNDLE)

run-cp: build
	RULER_SCENE=cp RULER_SECONDS=$${RULER_SECONDS:-0} ./$(BUNDLE)

measure-full: build
	$(PYTHON) scripts/measure.py --scene full --window-match RULER_FULL \
		--duration $(DURATION) --warmup $(WARMUP) --out out/full

measure-grid: build
	$(PYTHON) scripts/measure.py --scene grid --window-match RULER_GRID \
		--duration $(DURATION) --warmup 20 --out out/grid

measure-boundary: build
	$(PYTHON) scripts/measure.py --scene boundary --window-match RULER_BOUNDARY \
		--duration $(DURATION) --warmup $(WARMUP) --out out/boundary

measure-overlay: build
	$(PYTHON) scripts/measure.py --scene overlay --window-match RULER_OVERLAY \
		--duration $(DURATION) --warmup 5 --out out/overlay

measure-resize: build
	$(PYTHON) scripts/measure.py --scene resize --window-match RULER_RESIZE \
		--duration $(DURATION) --warmup $(WARMUP) \
		--env RULER_ACTIONS=res:1000x700@8;res:1200x800@16 --out out/resize

measure-min: build
	$(PYTHON) scripts/measure.py --scene min --window-match RULER_MIN \
		--duration 30 --warmup 5 \
		--env RULER_ACTIONS=minimize@22 --out out/min

measure-cp: build
	$(PYTHON) scripts/measure.py --scene cp --window-match RULER_CP \
		--duration $(DURATION) --warmup $(WARMUP) --out out/cp

# Row 10 multi-instance arm (1/2/4/8). Window titles are RULER_SPIN_<i>.
measure-instances-%:
	$(PYTHON) scripts/measure.py --scene spin --window-match RULER_SPIN_ \
		--duration $(DURATION) --warmup 5 --instances $* --out out/multi_$*
