package wayland

import (
	"fmt"
	"os"
	"sync"
	"syscall"
)

// Buffer es un búfer de memoria compartida ligado a un wl_buffer.
//
// Los píxeles se escriben directamente en Mem (formato ARGB8888 little-endian,
// es decir, bytes B,G,R,A en memoria).
type Buffer struct {
	ID     uint32
	W, H   int
	Stride int
	Mem    []byte
	file   *os.File
	busy   bool
	pool   uint32
}

// Shm gestiona un pool de memoria compartida con varios búferes en rotación.
type Shm struct {
	conn    *Conn
	shmID   uint32
	poolID  uint32
	buffers []*Buffer
	size    int
	file    *os.File
	mu      sync.Mutex
}

// NewShm crea un pool de memoria compartida para búferes del tamaño indicado.
//
// Se usa un archivo temporal en /dev/shm (memoria) que se elimina después de
// mapearlo: el compositor recibe el descriptor y accede a la misma memoria.
func NewShm(c *Conn, w, h, count int) (*Shm, error) {
	g, ok := c.FindGlobal("wl_shm")
	if !ok {
		return nil, fmt.Errorf("el compositor no ofrece wl_shm")
	}
	stride := w * 4
	size := stride * h * count
	f, err := createShmFile(size)
	if err != nil {
		return nil, err
	}
	mem, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("mapeando la memoria compartida: %w", err)
	}

	s := &Shm{conn: c, buffers: make([]*Buffer, 0, count), size: size, file: f}
	s.shmID = c.Bind(g, 1, nil)
	poolID := c.allocID("wl_shm_pool", nil)
	s.poolID = poolID
	// wl_shm.create_pool(new_id, fd, size)
	msg := newEncoder(s.shmID, shmCreatePool).newID(poolID).uint32(uint32(size)).bytes()
	if err := c.SendWithFD(msg, int(f.Fd())); err != nil {
		return nil, err
	}
	for i := 0; i < count; i++ {
		id := c.allocID("wl_buffer", nil)
		buf := &Buffer{
			ID: id, W: w, H: h, Stride: stride,
			Mem:  mem[i*stride*h : (i+1)*stride*h],
			file: f, pool: poolID,
		}
		// wl_shm_pool.create_buffer(new_id, offset, width, height, stride, format)
		msg := newEncoder(poolID, poolCreateBuffer).
			newID(id).
			int32(int32(i * stride * h)).
			int32(int32(w)).int32(int32(h)).
			int32(int32(stride)).
			uint32(formatARGB8888).
			bytes()
		if err := c.Send(msg); err != nil {
			return nil, err
		}
		s.buffers = append(s.buffers, buf)
	}
	if err := c.Roundtrip(); err != nil {
		return nil, err
	}
	return s, nil
}

// createShmFile crea un archivo anónimo en memoria.
func createShmFile(size int) (*os.File, error) {
	dirs := []string{"/dev/shm", os.TempDir()}
	var lastErr error
	for _, dir := range dirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		f, err := os.CreateTemp(dir, "swaydesk-shm-*")
		if err != nil {
			lastErr = err
			continue
		}
		// Se borra del sistema de archivos: el descriptor sigue siendo válido.
		_ = os.Remove(f.Name())
		if err := f.Truncate(int64(size)); err != nil {
			f.Close()
			lastErr = err
			continue
		}
		return f, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no hay directorio para memoria compartida")
	}
	return nil, lastErr
}

// Next devuelve el siguiente búfer libre, esperando si todos están en uso.
//
// El bucle de eventos debe llamar a Release cuando llegue wl_buffer.release.
func (s *Shm) Next() *Buffer {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.buffers {
		if !b.busy {
			b.busy = true
			return b
		}
	}
	// Ninguno libre: se reutiliza el primero (el compositor lo ignorará si aún
	// lo está mostrando, pero evita bloquear el renderizado).
	b := s.buffers[0]
	b.busy = true
	return b
}

// Release marca un búfer como disponible.
func (s *Shm) Release(id uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.buffers {
		if b.ID == id {
			b.busy = false
			return
		}
	}
}

// BindReleases registra el manejador de wl_buffer.release en todos los búferes.
func (s *Shm) BindReleases() {
	for _, b := range s.buffers {
		bufferID := b.ID
		s.conn.setHandler(bufferID, "wl_buffer", func(op uint16, d *decoder) {
			if op == bufferReleaseEvt {
				s.Release(bufferID)
			}
		})
	}
}

// Close libera la memoria y destruye los objetos del protocolo.
func (s *Shm) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, b := range s.buffers {
		_ = s.conn.Send(newEncoder(b.ID, bufferDestroy).bytes())
	}
	_ = s.conn.Send(newEncoder(s.poolID, poolDestroy).bytes())
	if s.file != nil {
		s.file.Close()
	}
}

// Fill copia una imagen RGB (3 bytes por píxel) escalándola al tamaño del búfer.
//
// El búfer es opaco (alfa = 255).
func (b *Buffer) FillRGB(pix []uint8, srcW, srcH int) {
	if srcW <= 0 || srcH <= 0 {
		return
	}
	for y := 0; y < b.H; y++ {
		sy := y * srcH / b.H
		if sy >= srcH {
			sy = srcH - 1
		}
		rowOut := b.Mem[y*b.Stride:]
		for x := 0; x < b.W; x++ {
			sx := x * srcW / b.W
			if sx >= srcW {
				sx = srcW - 1
			}
			si := (sy*srcW + sx) * 3
			di := x * 4
			rowOut[di] = pix[si+2]   // B
			rowOut[di+1] = pix[si+1] // G
			rowOut[di+2] = pix[si]   // R
			rowOut[di+3] = 0xff      // A
		}
	}
}
