package font

// Each recorre los píxeles encendidos de un texto y llama a fn con sus
// coordenadas lógicas (sin escalar), relativas a la esquina superior izquierda.
//
// scale agranda cada píxel de la fuente en un bloque scale×scale; spacing es la
// separación en píxeles entre celdas (antes de escalar).
func (f *Face) Each(s string, scale, spacing int, fn func(x, y int)) {
	if scale < 1 {
		scale = 1
	}
	if spacing < 0 {
		spacing = 0
	}
	cell := (f.W + spacing) * scale
	cx := 0
	for _, r := range s {
		rows := f.Glyph(r)
		for y := 0; y < f.H && y < len(rows); y++ {
			row := rows[y]
			if row == 0 {
				continue
			}
			for x := 0; x < f.W; x++ {
				if !rowBit(row, x) {
					continue
				}
				bx := cx + x*scale
				by := y * scale
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						fn(bx+dx, by+dy)
					}
				}
			}
		}
		cx += cell
	}
}

// Rows devuelve el alto en píxeles de la fuente escalada.
func (f *Face) Rows(scale int) int {
	if scale < 1 {
		scale = 1
	}
	return f.H * scale
}
