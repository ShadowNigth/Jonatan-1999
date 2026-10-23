package launcher

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Output ejecuta un comando y devuelve su salida estándar.
func Output(name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Launch lanza una entrada de aplicación.
//
// Si la entrada requiere terminal, se abre una ventana nueva con el emulador
// configurado; en caso contrario se lanza el proceso en su propia sesión.
func Launch(entry Entry, terminal string) error {
	args := ExecArgs(entry.Exec)
	if len(args) == 0 {
		return fmt.Errorf("la entrada %q no tiene comando", entry.Name)
	}
	if entry.Terminal {
		return LaunchTerminal(terminal, "swaydesk-run", entry.Name, 0, 0, args)
	}
	return StartDetached(args[0], args[1:]...)
}

// WriteToCommand ejecuta un comando enviando text por su entrada estándar.
func WriteToCommand(text string, cmd ...string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("comando vacío")
	}
	c := exec.Command(cmd[0], cmd[1:]...)
	c.Stdin = strings.NewReader(text)
	return c.Run()
}
