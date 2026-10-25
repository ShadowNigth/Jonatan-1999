package font

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"unicode/utf8"
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
	glyphs := make([][]uint8, count)
	for i := 0; i < count; i++ {
		rows := append([]uint8(nil), data[4+i*charsize:4+(i+1)*charsize]...)
		glyphs[i] = rows
		// Sin tabla Unicode solo es seguro asumir que los glifos ASCII
		// imprimibles están en sus posiciones habituales. Con tabla, mandan
		// los alias explícitos y no se deben inventar códigos por índice.
		if !hasUnicode && i >= 32 && i < 127 {
			f.glyphs[rune(i)] = rows
		}
	}
	if hasUnicode {
		if len(data) == need {
			return nil, fmt.Errorf("tabla Unicode PSF1 ausente")
		}
		applyPSF1UnicodeTable(f, data[need:], glyphs)
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
	glyphs := make([][]uint8, count)
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
		glyphs[i] = rows
		if flags&1 == 0 && i >= 32 && i < 127 {
			f.glyphs[rune(i)] = rows
		}
	}
	if flags&1 != 0 {
		if len(data) == need {
			return nil, fmt.Errorf("tabla Unicode PSF2 ausente")
		}
		if err := applyPSF2UnicodeTable(f, data[need:], glyphs); err != nil {
			return nil, err
		}
	}
	return f, nil
}

// applyPSF1UnicodeTable interpreta la tabla PSF1, codificada en palabras
// little-endian de 16 bits. Cada 0xFFFF termina un glifo; 0xFFFE marca una
// secuencia. Se conserva su primer punto como alias individual y se omiten los
// siguientes, que solo tienen sentido al dibujar la secuencia completa.
func applyPSF1UnicodeTable(f *Face, table []byte, glyphs [][]uint8) {
	glyph := 0
	inSequence := false
	sequenceStartMapped := false
	for i := 0; i+1 < len(table); i += 2 {
		r := rune(binary.LittleEndian.Uint16(table[i : i+2]))
		switch r {
		case 0xFFFE:
			inSequence = true
			sequenceStartMapped = false
		case 0xFFFF:
			inSequence = false
			sequenceStartMapped = false
			glyph++
		default:
			if glyph < len(glyphs) && r != 0 && (!inSequence || !sequenceStartMapped) {
				f.glyphs[r] = glyphs[glyph]
			}
			if inSequence {
				sequenceStartMapped = true
			}
		}
		if glyph >= len(glyphs) && !inSequence {
			return
		}
	}
}

// applyPSF2UnicodeTable interpreta la tabla PSF2, cuyos puntos de código son
// UTF-8 y cuyos glifos terminan en 0xFF. 0xFE introduce una secuencia: conserva
// su primera runa como alias y omite la continuación no representable.
func applyPSF2UnicodeTable(f *Face, table []byte, glyphs [][]uint8) error {
	glyph := 0
	for i := 0; i < len(table); {
		if glyph >= len(glyphs) {
			return fmt.Errorf("tabla Unicode PSF2 con más entradas que glifos")
		}
		switch table[i] {
		case 0xFF:
			glyph++
			i++
		case 0xFE:
			// La primera runa también es un alias individual; el resto solo
			// corresponde a la secuencia completa y no se puede dibujar aquí.
			i++
			if i < len(table) && table[i] != 0xFF && table[i] != 0xFE {
				r, size := utf8.DecodeRune(table[i:])
				if size == 1 && table[i] >= utf8.RuneSelf {
					return fmt.Errorf("secuencia UTF-8 inválida en tabla PSF2")
				}
				f.glyphs[r] = glyphs[glyph]
				i += size
			}
			for i < len(table) && table[i] != 0xFF && table[i] != 0xFE {
				_, size := utf8.DecodeRune(table[i:])
				if size == 1 && table[i] >= utf8.RuneSelf {
					return fmt.Errorf("secuencia UTF-8 inválida en tabla PSF2")
				}
				i += size
			}
		default:
			r, size := utf8.DecodeRune(table[i:])
			if size == 1 && table[i] >= utf8.RuneSelf {
				return fmt.Errorf("UTF-8 inválido en tabla PSF2")
			}
			f.glyphs[r] = glyphs[glyph]
			i += size
		}
	}
	if glyph != len(glyphs) {
		return fmt.Errorf("tabla Unicode PSF2 incompleta: %d glifos de %d", glyph, len(glyphs))
	}
	return nil
}
