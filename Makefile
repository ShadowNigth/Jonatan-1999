.PHONY: all build run kiosk test export clean

GO ?= go
LDFLAGS := -s -w

all: build

build:
	@mkdir -p build
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o build/kitsune-de ./cmd/kitsune-de
	$(GO) build -trimpath -ldflags="$(LDFLAGS)" -o build/kitsune-ctl ./cmd/kitsune-ctl

run: build
	./build/kitsune-de --addr 0.0.0.0:8080

kiosk: build
	./build/kitsune-de --addr 127.0.0.1:8080 --kiosk

test:
	$(GO) test -v ./...

export: build
	./build/kitsune-ctl export --theme neon-tokyo --out ./dotfiles-arch

clean:
	rm -rf build bin dotfiles-arch dotfiles-export
