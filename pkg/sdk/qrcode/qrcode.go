// Package qrcode erzeugt QR-Codes (ISO/IEC 18004, Modell 2) im Byte-Modus –
// eigene Umsetzung ohne fremde Bibliothek. Texte werden als UTF-8 kodiert;
// Handys lesen so tel:-Links, vCards und Links.
//
//	c, err := qrcode.Encode("tel:+493414801099", qrcode.M)
//	svg := c.SVG(4) // Ruhezone 4 Module
//
// Umfang: Versionen 1–40, Fehlerkorrektur L/M/Q/H, alle acht Masken (die mit
// der kleinsten Strafpunktzahl gewinnt). Kein Micro-QR, kein Kanji- oder
// Alphanumerik-Modus (Byte-Modus genügt für die Anwendungen).
package qrcode

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"strings"
)

// Level ist die Stufe der Fehlerkorrektur.
type Level int

const (
	L Level = iota // ca. 7 % der Codewörter wiederherstellbar
	M              // ca. 15 %
	Q              // ca. 25 %
	H              // ca. 30 %
)

// ErrTooLong: Der Text passt in keine Version.
var ErrTooLong = errors.New("qrcode: Text zu lang")

// Code ist ein fertiger QR-Code: Size×Size Module, true = dunkel.
type Code struct {
	Version int
	Level   Level
	Mask    int
	Size    int
	modules [][]bool
}

// Dark: Ist das Modul (x = Spalte, y = Zeile) dunkel?
func (c *Code) Dark(x, y int) bool { return c.modules[y][x] }

// Encode kodiert text in der kleinsten Version, die bei level passt.
func Encode(text string, level Level) (*Code, error) {
	data := []byte(text)
	for v := 1; v <= 40; v++ {
		if bitsNeeded(len(data), v) <= dataCodewords(v, level)*8 {
			return encodeVersion(data, v, level, -1), nil
		}
	}
	return nil, ErrTooLong
}

// bitsNeeded: Modus (4) + Zeichenzahl + Daten.
func bitsNeeded(n, version int) int { return 4 + countBits(version) + 8*n }

// countBits: Länge der Zeichenzahl im Byte-Modus.
func countBits(version int) int {
	if version <= 9 {
		return 8
	}
	return 16
}

// encodeVersion: mask < 0 wählt die beste Maske.
func encodeVersion(data []byte, version int, level Level, mask int) *Code {
	codewords := addECC(dataBits(data, version, level), version, level)
	c := &Code{Version: version, Level: level, Size: version*4 + 17}
	m := newMatrix(c.Size)
	m.drawFunctionPatterns(version)
	m.drawCodewords(codewords)
	if mask < 0 {
		best := -1
		for k := 0; k < 8; k++ {
			m.applyMask(k)
			m.drawFormat(level, k)
			if p := m.penalty(); best < 0 || p < best {
				best, mask = p, k
			}
			m.applyMask(k) // XOR: wieder entfernen
		}
	}
	m.applyMask(mask)
	m.drawFormat(level, mask)
	c.Mask, c.modules = mask, m.dark
	return c
}

// --- Daten und Fehlerkorrektur -------------------------------------------------

// dataBits: Modus, Länge, Daten, Abschluss und Füllbytes – genau die
// Datencodewörter der Version.
func dataBits(data []byte, version int, level Level) []byte {
	capacity := dataCodewords(version, level)
	var b bitBuffer
	b.append(0b0100, 4) // Byte-Modus
	b.append(len(data), countBits(version))
	for _, x := range data {
		b.append(int(x), 8)
	}
	b.append(0, min(4, capacity*8-b.n)) // Abschluss
	b.append(0, (8-b.n%8)%8)
	out := b.bytes()
	for pad := 0; len(out) < capacity; pad++ {
		out = append(out, [2]byte{0xEC, 0x11}[pad%2])
	}
	return out
}

type bitBuffer struct {
	data []byte
	n    int
}

func (b *bitBuffer) append(v, bits int) {
	for i := bits - 1; i >= 0; i-- {
		if b.n%8 == 0 {
			b.data = append(b.data, 0)
		}
		if v>>i&1 == 1 {
			b.data[b.n/8] |= 0x80 >> (b.n % 8)
		}
		b.n++
	}
}

func (b *bitBuffer) bytes() []byte { return b.data }

// addECC teilt die Daten in Blöcke, hängt je Block die Reed-Solomon-Codewörter
// an und verschränkt die Blöcke.
func addECC(data []byte, version int, level Level) []byte {
	numBlocks := numECCBlocks[level][version]
	eccLen := eccCodewordsPerBlock[level][version]
	raw := rawModules(version) / 8
	numShort := numBlocks - raw%numBlocks
	shortLen := raw / numBlocks
	div := rsDivisor(eccLen)
	blocks := make([][]byte, numBlocks)
	k := 0
	for i := range blocks {
		n := shortLen - eccLen
		if i >= numShort {
			n++
		}
		dat := append([]byte(nil), data[k:k+n]...)
		k += n
		ecc := rsRemainder(dat, div)
		if i < numShort {
			dat = append(dat, 0) // Platzhalter: kurze Blöcke sind ein Byte kürzer
		}
		blocks[i] = append(dat, ecc...)
	}
	var out []byte
	for i := 0; i < len(blocks[0]); i++ {
		for j, blk := range blocks {
			if i != shortLen-eccLen || j >= numShort {
				out = append(out, blk[i])
			}
		}
	}
	return out
}

// rsDivisor: Generatorpolynom vom Grad degree (Koeffizienten ohne den führenden).
func rsDivisor(degree int) []byte {
	res := make([]byte, degree)
	res[degree-1] = 1
	root := byte(1)
	for i := 0; i < degree; i++ {
		for j := range res {
			res[j] = gfMul(res[j], root)
			if j+1 < len(res) {
				res[j] ^= res[j+1]
			}
		}
		root = gfMul(root, 0x02)
	}
	return res
}

func rsRemainder(data, div []byte) []byte {
	res := make([]byte, len(div))
	for _, b := range data {
		f := b ^ res[0]
		copy(res, res[1:])
		res[len(res)-1] = 0
		for i, d := range div {
			res[i] ^= gfMul(d, f)
		}
	}
	return res
}

// gfMul: Multiplikation in GF(2^8) mit dem Polynom x^8+x^4+x^3+x^2+1.
func gfMul(x, y byte) byte {
	z := 0
	for i := 7; i >= 0; i-- {
		z = z<<1 ^ (z>>7)*0x11D
		z ^= int(y>>i&1) * int(x)
	}
	return byte(z)
}

// rawModules: Module für Daten und Fehlerkorrektur (ohne Funktionsmuster).
func rawModules(version int) int {
	r := (16*version+128)*version + 64
	if version >= 2 {
		n := version/7 + 2
		r -= (25*n-10)*n - 55
		if version >= 7 {
			r -= 36
		}
	}
	return r
}

func dataCodewords(version int, level Level) int {
	return rawModules(version)/8 - eccCodewordsPerBlock[level][version]*numECCBlocks[level][version]
}

// Tabellen der Norm (Index = Version, 0 unbenutzt).
var eccCodewordsPerBlock = [4][41]int{
	{0, 7, 10, 15, 20, 26, 18, 20, 24, 30, 18, 20, 24, 26, 30, 22, 24, 28, 30, 28, 28, 28, 28, 30, 30, 26, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
	{0, 10, 16, 26, 18, 24, 16, 18, 22, 22, 26, 30, 22, 22, 24, 24, 28, 28, 26, 26, 26, 26, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28, 28},
	{0, 13, 22, 18, 26, 18, 24, 18, 22, 20, 24, 28, 26, 24, 20, 30, 24, 28, 28, 26, 30, 28, 30, 30, 30, 30, 28, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
	{0, 17, 28, 22, 16, 22, 28, 26, 26, 24, 28, 24, 28, 22, 24, 24, 30, 28, 28, 26, 28, 30, 24, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30, 30},
}

var numECCBlocks = [4][41]int{
	{0, 1, 1, 1, 1, 1, 2, 2, 2, 2, 4, 4, 4, 4, 4, 6, 6, 6, 6, 7, 8, 8, 9, 9, 10, 12, 12, 12, 13, 14, 15, 16, 17, 18, 19, 19, 20, 21, 22, 24, 25},
	{0, 1, 1, 1, 2, 2, 4, 4, 4, 5, 5, 5, 8, 9, 9, 10, 10, 11, 13, 14, 16, 17, 17, 18, 20, 21, 23, 25, 26, 28, 29, 31, 33, 35, 37, 38, 40, 43, 45, 47, 49},
	{0, 1, 1, 2, 2, 4, 4, 6, 6, 8, 8, 8, 10, 12, 16, 12, 17, 16, 18, 21, 20, 23, 23, 25, 27, 29, 34, 34, 35, 38, 40, 43, 45, 48, 51, 53, 56, 59, 62, 65, 68},
	{0, 1, 1, 2, 4, 4, 4, 5, 6, 8, 8, 11, 11, 16, 16, 18, 16, 19, 21, 25, 25, 25, 34, 30, 32, 35, 37, 40, 42, 45, 48, 51, 54, 57, 60, 63, 66, 70, 74, 77, 81},
}

// --- Matrix ----------------------------------------------------------------------

type matrix struct {
	size     int
	dark     [][]bool
	function [][]bool // Funktionsmuster: nicht maskiert, keine Daten
}

func newMatrix(size int) *matrix {
	m := &matrix{size: size, dark: make([][]bool, size), function: make([][]bool, size)}
	for i := range size {
		m.dark[i], m.function[i] = make([]bool, size), make([]bool, size)
	}
	return m
}

func (m *matrix) set(x, y int, dark bool) {
	m.dark[y][x], m.function[y][x] = dark, true
}

func (m *matrix) drawFunctionPatterns(version int) {
	for i := 0; i < m.size; i++ { // Taktlinien
		m.set(6, i, i%2 == 0)
		m.set(i, 6, i%2 == 0)
	}
	m.drawFinder(3, 3)
	m.drawFinder(m.size-4, 3)
	m.drawFinder(3, m.size-4)
	pos := alignmentPositions(version)
	last := len(pos) - 1
	for i := range pos {
		for j := range pos {
			if i == 0 && j == 0 || i == 0 && j == last || i == last && j == 0 {
				continue // Ecken mit Suchmustern
			}
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					m.set(pos[i]+dx, pos[j]+dy, max(abs(dx), abs(dy)) != 1)
				}
			}
		}
	}
	m.drawFormat(L, 0) // reserviert; echte Werte nach der Maskenwahl
	if version >= 7 {
		rem := version
		for range 12 {
			rem = rem<<1 ^ (rem>>11)*0x1F25
		}
		bits := version<<12 | rem
		for i := 0; i < 18; i++ {
			d := bits>>i&1 == 1
			a, b := m.size-11+i%3, i/3
			m.set(a, b, d)
			m.set(b, a, d)
		}
	}
}

// drawFinder: Suchmuster mit Trennlinie um den Mittelpunkt (cx, cy).
func (m *matrix) drawFinder(cx, cy int) {
	for dy := -4; dy <= 4; dy++ {
		for dx := -4; dx <= 4; dx++ {
			x, y := cx+dx, cy+dy
			if x >= 0 && x < m.size && y >= 0 && y < m.size {
				d := max(abs(dx), abs(dy))
				m.set(x, y, d != 2 && d != 4)
			}
		}
	}
}

func alignmentPositions(version int) []int {
	if version == 1 {
		return nil
	}
	n := version/7 + 2
	step := (version*8 + n*3 + 5) / (n*4 - 4) * 2
	pos := make([]int, n)
	pos[0] = 6
	for i, p := n-1, version*4+10; i >= 1; i, p = i-1, p-step {
		pos[i] = p
	}
	return pos
}

// formatBits: Stufe und Maske mit BCH-Code, maskiert mit 0x5412.
func formatBits(level Level, mask int) int {
	data := [4]int{1, 0, 3, 2}[level]<<3 | mask
	rem := data
	for range 10 {
		rem = rem<<1 ^ (rem>>9)*0x537
	}
	return (data<<10 | rem) ^ 0x5412
}

func (m *matrix) drawFormat(level Level, mask int) {
	bits := formatBits(level, mask)
	bit := func(i int) bool { return bits>>i&1 == 1 }
	for i := 0; i <= 5; i++ {
		m.set(8, i, bit(i))
	}
	m.set(8, 7, bit(6))
	m.set(8, 8, bit(7))
	m.set(7, 8, bit(8))
	for i := 9; i < 15; i++ {
		m.set(14-i, 8, bit(i))
	}
	for i := 0; i < 8; i++ {
		m.set(m.size-1-i, 8, bit(i))
	}
	for i := 8; i < 15; i++ {
		m.set(8, m.size-15+i, bit(i))
	}
	m.set(8, m.size-8, true) // dunkles Modul
}

// drawCodewords: Bits im Zickzack von unten rechts, je zwei Spalten.
func (m *matrix) drawCodewords(data []byte) {
	i := 0
	for right := m.size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5 // senkrechte Taktlinie überspringen
		}
		for vert := 0; vert < m.size; vert++ {
			for j := 0; j < 2; j++ {
				x := right - j
				y := vert
				if (right+1)&2 == 0 {
					y = m.size - 1 - vert // aufwärts
				}
				if !m.function[y][x] && i < len(data)*8 {
					m.dark[y][x] = data[i>>3]>>(7-i&7)&1 == 1
					i++
				}
			}
		}
	}
}

func maskBit(mask, x, y int) bool {
	switch mask {
	case 0:
		return (x+y)%2 == 0
	case 1:
		return y%2 == 0
	case 2:
		return x%3 == 0
	case 3:
		return (x+y)%3 == 0
	case 4:
		return (x/3+y/2)%2 == 0
	case 5:
		return x*y%2+x*y%3 == 0
	case 6:
		return (x*y%2+x*y%3)%2 == 0
	default:
		return ((x+y)%2+x*y%3)%2 == 0
	}
}

func (m *matrix) applyMask(mask int) {
	for y := range m.size {
		for x := range m.size {
			if !m.function[y][x] && maskBit(mask, x, y) {
				m.dark[y][x] = !m.dark[y][x]
			}
		}
	}
}

// penalty: Strafpunkte der Norm (Läufe, 2×2-Blöcke, suchmusterähnliche Folgen,
// Anteil dunkler Module).
func (m *matrix) penalty() int {
	p, darkCount := 0, 0
	line := func(get func(i int) bool) {
		run, prev := 0, false
		for i := 0; i < m.size; i++ {
			d := get(i)
			if i > 0 && d == prev {
				run++
			} else {
				if run >= 5 {
					p += 3 + run - 5
				}
				run, prev = 1, d
			}
		}
		if run >= 5 {
			p += 3 + run - 5
		}
		// 1:1:3:1:1 mit vier hellen Modulen davor oder danach (außen gilt als hell)
		var s strings.Builder
		s.WriteString("0000")
		for i := 0; i < m.size; i++ {
			if get(i) {
				s.WriteByte('1')
			} else {
				s.WriteByte('0')
			}
		}
		s.WriteString("0000")
		str := s.String()
		for i := 0; i+11 <= len(str); i++ {
			if w := str[i : i+11]; w == "10111010000" || w == "00001011101" {
				p += 40
			}
		}
	}
	for y := range m.size {
		line(func(i int) bool { return m.dark[y][i] })
	}
	for x := range m.size {
		line(func(i int) bool { return m.dark[i][x] })
	}
	for y := range m.size {
		for x := range m.size {
			d := m.dark[y][x]
			if d {
				darkCount++
			}
			if x+1 < m.size && y+1 < m.size && d == m.dark[y][x+1] && d == m.dark[y+1][x] && d == m.dark[y+1][x+1] {
				p += 3
			}
		}
	}
	total := m.size * m.size
	k := (abs(darkCount*20-total*10)+total-1)/total - 1
	return p + max(k, 0)*10
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// --- Ausgabe ---------------------------------------------------------------------

// SVG: der Code als SVG (ein Pfad, je Modul eine Einheit), quiet = Ruhezone in
// Modulen (die Norm verlangt 4). Größe bestimmt die Einbettung (CSS).
func (c *Code) SVG(quiet int) string {
	n := c.Size + 2*quiet
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" shape-rendering="crispEdges">`, n, n)
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#fff"/><path fill="#000" d="`, n, n)
	for y := range c.Size {
		for x := 0; x < c.Size; x++ {
			if !c.modules[y][x] {
				continue
			}
			w := 1 // waagerechte Läufe als ein Rechteck
			for x+w < c.Size && c.modules[y][x+w] {
				w++
			}
			fmt.Fprintf(&b, "M%d %dh%dv1h-%dz", x+quiet, y+quiet, w, w)
			x += w - 1
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String()
}

// PNG: der Code als PNG (schwarz auf weiß, 1 Bit je Pixel), scale Pixel je Modul,
// quiet Ruhezone in Modulen – für Briefe (LaTeX) und eingebettete Bilder in Mails.
func (c *Code) PNG(scale, quiet int) ([]byte, error) {
	if scale < 1 {
		scale = 1
	}
	n := (c.Size + 2*quiet) * scale
	img := image.NewPaletted(image.Rect(0, 0, n, n), color.Palette{color.White, color.Black})
	for y := range c.Size {
		for x := range c.Size {
			if !c.modules[y][x] {
				continue
			}
			for dy := range scale {
				for dx := range scale {
					img.SetColorIndex((x+quiet)*scale+dx, (y+quiet)*scale+dy, 1)
				}
			}
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
