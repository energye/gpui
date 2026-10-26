FLUTTER_SDK ?= $(HOME)/app/projects/gogpu/flutter
FLUTTER ?= $(FLUTTER_SDK)/bin/flutter
BUNDLE := build/linux/x64/release/bundle/ruler_flutter
PYTHON ?= python3
DURATION ?= 60
WARMUP ?= 5

.PHONY: build analyze run-spin run-list run-text measure-spin measure-list measure-text

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
