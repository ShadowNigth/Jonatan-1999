package wallpaper

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/swayipc"
)

func newWorkspaceTestDaemon(t *testing.T, transition Transition) *daemonState {
	t.Helper()
	engine, err := NewEngine(Spec{Kind: KindScene, Scene: "aurora", Pal: testPal})
	if err != nil {
		t.Fatal(err)
	}
	d := &daemonState{
		opt: DaemonOptions{
			Workspaces: NewWorkspaceMap("scene:aurora", map[string]string{"2": "scene:petals"}),
			Transition: transition,
			Palette:    testPal,
			FPS:        24,
		},
		eng:    engine,
		source: "scene:aurora",
		last:   canvasSolid(40, 20, 12, 24, 36),
	}
	t.Cleanup(d.closeEngines)
	return d
}

func TestDaemonWorkspaceChangeUsesRuleAndTransition(t *testing.T) {
	d := newWorkspaceTestDaemon(t, NewTransition(TransFade, 400))
	previous := d.last
	d.onWorkspace(swayipc.WorkspaceEvent{
		Change:  "focus",
		Current: swayipc.Workspace{Num: 2, Name: "2:web", Focused: true},
	})
	if d.source != "scene:petals" || d.eng.Spec().Scene != "petals" {
		t.Fatalf("no se aplicó el fondo del workspace: source=%q spec=%+v", d.source, d.eng.Spec())
	}
	if !d.moving || d.from != previous || d.opt.Transition.Kind != TransFade {
		t.Fatalf("el cambio no inició una transición: moving=%t from=%p transition=%+v", d.moving, d.from, d.opt.Transition)
	}
}

func TestDaemonWorkspaceChangeReturnsToBaseAndIgnoresOtherEvents(t *testing.T) {
	d := newWorkspaceTestDaemon(t, NewTransition(TransSlide, 300))
	d.onWorkspace(swayipc.WorkspaceEvent{
		Change:  "focus",
		Current: swayipc.Workspace{Num: 2, Name: "2:web"},
	})
	if d.source != "scene:petals" {
		t.Fatalf("fondo inicial del workspace = %q", d.source)
	}
	d.onWorkspace(swayipc.WorkspaceEvent{
		Change:  "empty",
		Current: swayipc.Workspace{Num: 1, Name: "1:term"},
	})
	if d.source != "scene:petals" {
		t.Fatalf("un evento empty no debe cambiar el fondo: %q", d.source)
	}
	d.onWorkspace(swayipc.WorkspaceEvent{
		Change:  "focus",
		Current: swayipc.Workspace{Num: 1, Name: "1:term", Focused: true},
	})
	if d.source != "scene:aurora" || d.eng.Spec().Scene != "aurora" {
		t.Fatalf("al salir de la regla debe volver al fondo base: source=%q spec=%+v", d.source, d.eng.Spec())
	}
	if !d.moving || d.opt.Transition.Kind != TransSlide {
		t.Fatalf("el retorno al fondo base debe animarse: moving=%t transition=%+v", d.moving, d.opt.Transition)
	}
}

func TestDaemonWorkspaceWithoutTransitionSwitchesImmediately(t *testing.T) {
	d := newWorkspaceTestDaemon(t, NewTransition(TransNone, 0))
	d.onWorkspace(swayipc.WorkspaceEvent{
		Change:  "init",
		Current: swayipc.Workspace{Num: 2, Name: "2:web"},
	})
	if d.source != "scene:petals" || d.moving || d.from != nil || d.last != nil {
		t.Fatalf("el cambio sin transición no fue inmediato: source=%q moving=%t from=%p last=%p", d.source, d.moving, d.from, d.last)
	}
}

func TestDaemonExpandsHomePathForWorkspaceSource(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, "wallpaper.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 8, 6))); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	d := newWorkspaceTestDaemon(t, NewTransition(TransNone, 0))
	if err := d.install("image:~/wallpaper.png", false); err != nil {
		t.Fatal(err)
	}
	if got := d.eng.Spec().Path; got != path {
		t.Fatalf("se expandió ~/ a %q, se esperaba %q", got, path)
	}
}
