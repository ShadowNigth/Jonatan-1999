# swaydesk · escritorio para Arch Linux + sway escrito en Go
#
#   make            compila todos los binarios en bin/
#   make install    instala en PREFIX (por defecto ~/.local)
#   make test       ejecuta las pruebas

GO      ?= go
PREFIX  ?= $(HOME)/.local
BINDIR   = $(PREFIX)/bin
BINS     = swaydesk swaymenu swayctl swayclip swaybar swaynotify

VERSION := $(shell sed -n 's/.*const Version = "\(.*\)"/\1/p' cmd/swaydesk/main.go)
LDFLAGS := -s -w

.PHONY: all build install uninstall test vet fmt lint clean dist

all: build

build:
	@mkdir -p bin
	@for b in $(BINS); do \
		echo "  GO  build $$b"; \
		CGO_ENABLED=0 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o bin/$$b ./cmd/$$b || exit 1; \
	done
	@echo "Listo: $$(ls bin | tr '\n' ' ')"

install: build
	@mkdir -p $(BINDIR)
	@for b in $(BINS); do \
		echo "  INSTALL $(BINDIR)/$$b"; \
		install -Dm755 bin/$$b $(BINDIR)/$$b; \
	done
	@echo
	@echo "Siguiente paso:  swaydesk init && swaymsg reload"
	@echo "Asegúrate de que $(BINDIR) está en el PATH."

uninstall:
	@for b in $(BINS); do rm -f $(BINDIR)/$$b; done
	@echo "Binarios eliminados de $(BINDIR)"

test:
	$(GO) test ./... -race -count=1

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

lint: vet
	@test -z "$$(gofmt -l . | tee /dev/stderr)" || (echo "archivos sin formatear"; exit 1)

# Binarios estáticos para x86_64 y aarch64 (por si se compila en otra máquina).
dist:
	@mkdir -p dist
	@for arch in amd64 arm64; do \
		for b in $(BINS); do \
			CGO_ENABLED=0 GOOS=linux GOARCH=$$arch $(GO) build -trimpath \
				-ldflags="$(LDFLAGS)" -o dist/$$b-linux-$$arch ./cmd/$$b || exit 1; \
		done; \
	done
	@echo "Binarios en dist/"

clean:
	rm -rf bin dist
