package setup

import (
	"strings"
	"testing"

	"github.com/ShadowNigth/swaydesk/internal/config"
)

func TestSwaylockTemplateIsCompatibleWithStandardSwaylock(t *testing.T) {
	files, err := Render(config.Default())
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	lock := files["swaylock/config"]
	for _, unsupported := range []string{
		"show-clock=", "clock-font=", "clock-color=", "clock-font-size=",
		"timestr=", "datestr=", "indicator-idle-visible=",
	} {
		if strings.Contains(lock, unsupported) {
			t.Errorf("la plantilla estándar no debe contener %q:\n%s", unsupported, lock)
		}
	}
	for _, option := range []string{
		"indicator-idle-visible\n", "indicator-caps-lock\n",
		"ignore-empty-password\n", "show-failed-attempts\n",
		"indicator-radius=148\n", "indicator-thickness=7\n",
	} {
		if !strings.Contains(lock, option) {
			t.Errorf("falta la opción compatible %q en swaylock/config", strings.TrimSpace(option))
		}
	}
	if strings.HasSuffix(strings.TrimSpace(lock), "}") {
		t.Fatal("swaylock/config no debe terminar con una llave suelta")
	}
}
