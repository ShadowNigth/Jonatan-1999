package font

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
)

// Load lee una fuente de consola en formato PSF1 o PSF2, con o sin la tabla
// Unicode y comprimida en gzip o no.
func Load(path string) (*Face, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Se descomprime si hace falta (las .psf.gz de Arch).
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("%s: gzip: %w", filepath.Base(path), err)
		}
		defer zr.Close()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(zr); err != nil {
			return nil, fmt.Errorf("%s: gzip: %w", filepath.Base(path), err)
		}
		data = buf.Bytes()
	}
	var face *Face
	switch {
	case len(data) >= 4 && data[0] == 0x36 && data[1] == 0x04:
		face, err = parsePSF1(data)
	case len(data) >= 4 && binary.LittleEndian.Uint32(data[0:4]) == 0x864ab572:
		face, err = parsePSF2(data)
	default:
		return nil, fmt.Errorf("%s: formato PSF desconocido", filepath.Base(path))
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	face.Name = filepath.Base(path)
	return face, nil
}

// parsePSF1 lee el formato antiguo: 256 o 512 glifos de 8 píxeles de ancho.
func parsePSF1(data []byte) (*Face, error) {
	if len(data) < 4 {
		return nil, fmt.Errorf("archivo truncado")
	}
	mode := data[2]
	charsize := int(data[3])
	count := 256
	if mode&0x01 != 0 {
		count = 512
	}
	hasUnicode := mode&0x02 != 0
	need := 4 + count*charsize
	if len(data) < need {
		return nil, fmt.Errorf("archivo truncado: %d < %d bytes", len(data), need)
	}
	f := &Face{W: 8, H: charsize, glyphs: make(map[rune][]uint8, count)}
	for i := 0; i < count; i++ {
		f.glyphs[rune(i)] = append([]uint8(nil), data[4+i*charsize:4+(i+1)*charsize]...)
	}
	if hasUnicode && len(data) > need {
		applyUnicodeTable(f, data[need:], nil)
	}
	return f, nil
}

// parsePSF2 lee el formato moderno, con ancho variable.
func parsePSF2(data []byte) (*Face, error) {
	if len(data) < 32 {
		return nil, fmt.Errorf("cabecera truncada")
	}
	headerSize := int(binary.LittleEndian.Uint32(data[8:12]))
	flags := binary.LittleEndian.Uint32(data[12:16])
	count := int(binary.LittleEndian.Uint32(data[16:20]))
	charSize := int(binary.LittleEndian.Uint32(data[20:24]))
	height := int(binary.LittleEndian.Uint32(data[24:28]))
	width := int(binary.LittleEndian.Uint32(data[28:32]))
	if width <= 0 || height <= 0 || count <= 0 || charSize < 1 || headerSize < 32 {
		return nil, fmt.Errorf("cabecera incoherente (%dx%d, %d glifos)", width, height, count)
	}
	if width > 8 {
		return nil, fmt.Errorf("ancho de glifo %d no soportado (máximo 8)", width)
	}
	start := headerSize
	need := start + count*charSize
	if len(data) < need {
		return nil, fmt.Errorf("archivo truncado: %d < %d bytes", len(data), need)
	}
	f := &Face{W: width, H: height, glyphs: make(map[rune][]uint8, count)}
	perRow := (width + 7) / 8
	for i := 0; i < count; i++ {
		raw := data[start+i*charSize : start+(i+1)*charSize]
		rows := make([]uint8, height)
		for y := 0; y < height; y++ {
			idx := y * perRow
			if idx < len(raw) {
				rows[y] = raw[idx]
			}
		}
		f.glyphs[rune(i)] = rows
	}
	if flags&1 != 0 && len(data) > need {
		applyUnicodeTable(f, data[need:], nil)
	}
	return f, nil
}

// applyUnicodeTable interpreta la tabla de mapeo Unicode→glifo de PSF.
//
// Formato: palabras de 16 bits. 0xFFFE abre una secuencia de puntos de código
// que se asignan a glifos consecutivos; 0xFFFF la cierra. El resto de valores
// son puntos de código sueltos, uno por glifo, en orden.
func applyUnicodeTable(f *Face, table []byte, _ []byte) {
	glyph := 0
	var seq []rune
	inSeq := false
	for i := 0; i+1 < len(table); i += 2 {
		v := rune(binary.LittleEndian.Uint16(table[i : i+2]))
		switch v {
		case 0xFFFE:
			inSeq, seq = true, seq[:0]
		case 0xFFFF:
			inSeq = false
			if g, ok := f.glyphs[rune(glyph)]; ok {
				for _, r := range seq {
					f.glyphs[r] = g
				}
			}
			glyph++
		default:
			if inSeq {
				seq = append(seq, v)
			} else if v != 0 {
				if g, ok := f.glyphs[rune(glyph)]; ok {
					f.glyphs[v] = g
				}
				glyph++
			} else {
				glyph++
			}
		}
	}
}
