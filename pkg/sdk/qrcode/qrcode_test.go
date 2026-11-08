package qrcode

import (
	"bytes"
	"fmt"
	"image/png"
	"slices"
	"strings"
	"testing"
)

// Bekannte Werte aus der Norm bzw. dem verbreiteten Rechenbeispiel „HELLO WORLD“ 1-M.
func TestReedSolomonKnownAnswer(t *testing.T) {
	data := []byte{32, 91, 11, 120, 209, 114, 220, 77, 67, 64, 236, 17, 236, 17, 236, 17}
	want := []byte{196, 35, 39, 119, 235, 215, 231, 226, 93, 23}
	if got := rsRemainder(data, rsDivisor(10)); !bytes.Equal(got, want) {
		t.Fatalf("ECC = %v, erwartet %v", got, want)
	}
}

func TestFormatAndVersionBits(t *testing.T) {
	for _, c := range []struct {
		level Level
		mask  int
		want  string
	}{
		{L, 0, "111011111000100"}, {M, 0, "101010000010010"}, {Q, 0, "011010101011111"}, {H, 0, "001011010001001"},
		{L, 7, "110100101110110"}, {M, 5, "100000011001110"}, {H, 7, "000100000111011"},
	} {
		if got := fmt.Sprintf("%015b", formatBits(c.level, c.mask)); got != c.want {
			t.Errorf("Format %d/%d = %s, erwartet %s", c.level, c.mask, got, c.want)
		}
	}
	m := newMatrix(7*4 + 17)
	m.drawFunctionPatterns(7)
	// Version 7: 000111110010010100, Bit 0 unten rechts im oberen rechten Block
	var got []byte
	for i := 17; i >= 0; i-- {
		got = append(got, map[bool]byte{true: '1', false: '0'}[m.dark[i/3][m.size-11+i%3]])
	}
	if string(got) != "000111110010010100" {
		t.Errorf("Versionsbits 7 = %s", got)
	}
}

func TestAlignmentPositions(t *testing.T) {
	for v, want := range map[int][]int{
		2: {6, 18}, 7: {6, 22, 38}, 14: {6, 26, 46, 66}, 22: {6, 26, 50, 74, 98}, 32: {6, 34, 60, 86, 112, 138},
		36: {6, 24, 50, 76, 102, 128, 154}, 40: {6, 30, 58, 86, 114, 142, 170},
	} {
		if got := alignmentPositions(v); !slices.Equal(got, want) {
			t.Errorf("Version %d: %v, erwartet %v", v, got, want)
		}
	}
}

// Kapazität im Byte-Modus (Tabelle der Norm, Version 1, 10, 40).
func TestCapacity(t *testing.T) {
	for _, c := range []struct {
		v     int
		level Level
		bytes int
	}{{1, L, 17}, {1, M, 14}, {1, Q, 11}, {1, H, 7}, {10, M, 213}, {40, L, 2953}, {40, H, 1273}} {
		n := (dataCodewords(c.v, c.level)*8 - 4 - countBits(c.v)) / 8
		if n != c.bytes {
			t.Errorf("Version %d Stufe %d: %d Bytes, erwartet %d", c.v, c.level, n, c.bytes)
		}
	}
}

// TestRoundTrip: Ein unabhängiger Leser (Format lesen, Maske entfernen,
// Codewörter einsammeln, Blöcke trennen, Syndrome prüfen) liefert den Text zurück.
func TestRoundTrip(t *testing.T) {
	vcard := "BEGIN:VCARD\r\nVERSION:3.0\r\nN:Mittrach GmbH;Dachdeckerei;;;\r\nFN:Dachdeckerei Mittrach GmbH\r\n" +
		"TEL;TYPE=WORK,VOICE:+49 3581 314042\r\nADR;TYPE=WORK:;;Rothenburger Landstraße 93;Görlitz;;02828;DE\r\nEND:VCARD\r\n"
	texts := []string{"", "tel:+493414801099", "Grüße aus Görlitz", vcard, strings.Repeat("x", 1200), strings.Repeat("Ä", 1400)}
	for _, text := range texts {
		for level := L; level <= H; level++ {
			c, err := Encode(text, level)
			if err != nil {
				if err == ErrTooLong && len(text) > [4]int{2953, 2331, 1663, 1273}[level] {
					continue // passt bei dieser Stufe in keine Version
				}
				t.Fatalf("%d Zeichen, Stufe %d: %v", len(text), level, err)
			}
			got, err := decode(c)
			if err != nil || got != text {
				t.Fatalf("Version %d Stufe %d Maske %d: %q, %v", c.Version, level, c.Mask, short(got), err)
			}
		}
	}
	// jede Maske einzeln
	for mask := 0; mask < 8; mask++ {
		c := encodeVersion([]byte("tel:+4915152579333"), 2, M, mask)
		if got, err := decode(c); err != nil || got != "tel:+4915152579333" || c.Mask != mask {
			t.Fatalf("Maske %d: %q %v", mask, got, err)
		}
	}
	if _, err := Encode(strings.Repeat("x", 3000), L); err != ErrTooLong {
		t.Fatalf("zu lang: %v", err)
	}
}

func TestSVG(t *testing.T) {
	c, _ := Encode("tel:+493414801099", M)
	s := c.SVG(4)
	if c.Version != 2 || !strings.HasPrefix(s, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 33 33"`) || !strings.Contains(s, "M4 4h7v1h-7z") {
		t.Fatalf("Version %d, SVG %s", c.Version, short(s))
	}
}

func short(s string) string {
	if len(s) > 80 {
		return s[:80] + "…"
	}
	return s
}

// --- unabhängiger Leser (nur für Tests) -------------------------------------------

func decode(c *Code) (string, error) {
	size := c.Size
	version := (size - 17) / 4
	// Format aus der ersten Kopie (um das obere linke Suchmuster)
	var bits int
	read := func(x, y int) int {
		if c.Dark(x, y) {
			return 1
		}
		return 0
	}
	coords := [][2]int{{8, 0}, {8, 1}, {8, 2}, {8, 3}, {8, 4}, {8, 5}, {8, 7}, {8, 8}, {7, 8}, {5, 8}, {4, 8}, {3, 8}, {2, 8}, {1, 8}, {0, 8}}
	for i, p := range coords {
		bits |= read(p[0], p[1]) << i
	}
	var bits2 int
	for i := 0; i < 8; i++ {
		bits2 |= read(size-1-i, 8) << i
	}
	for i := 8; i < 15; i++ {
		bits2 |= read(8, size-15+i) << i
	}
	if bits != bits2 {
		return "", fmt.Errorf("Formatkopien verschieden")
	}
	level, mask := Level(-1), -1
	for l := L; l <= H; l++ {
		for k := 0; k < 8; k++ {
			if formatBits(l, k) == bits {
				level, mask = l, k
			}
		}
	}
	if mask < 0 {
		return "", fmt.Errorf("Format unbekannt %015b", bits)
	}
	if level != c.Level || mask != c.Mask {
		return "", fmt.Errorf("Format %d/%d statt %d/%d", level, mask, c.Level, c.Mask)
	}
	// Funktionsbereiche unabhängig bestimmen
	fn := make([][]bool, size)
	for i := range fn {
		fn[i] = make([]bool, size)
	}
	mark := func(x0, y0, w, h int) {
		for y := max(y0, 0); y < min(y0+h, size); y++ {
			for x := max(x0, 0); x < min(x0+w, size); x++ {
				fn[y][x] = true
			}
		}
	}
	mark(0, 0, 9, 9)
	mark(size-8, 0, 8, 9)
	mark(0, size-8, 9, 8)
	mark(6, 0, 1, size)
	mark(0, 6, size, 1)
	pos := alignmentPositions(version)
	for i, a := range pos {
		for j, b := range pos {
			if !(i == 0 && j == 0 || i == 0 && j == len(pos)-1 || i == len(pos)-1 && j == 0) {
				mark(a-2, b-2, 5, 5)
			}
		}
	}
	if version >= 7 {
		mark(size-11, 0, 3, 6)
		mark(0, size-11, 6, 3)
	}
	// Bits einsammeln
	var stream []byte
	var cur byte
	n := 0
	for right := size - 1; right >= 1; right -= 2 {
		if right == 6 {
			right = 5
		}
		up := (right+1)&2 == 0
		for vert := 0; vert < size; vert++ {
			y := vert
			if up {
				y = size - 1 - vert
			}
			for _, x := range []int{right, right - 1} {
				if fn[y][x] {
					continue
				}
				b := c.Dark(x, y) != maskBit(mask, x, y)
				cur <<= 1
				if b {
					cur |= 1
				}
				if n++; n%8 == 0 {
					stream = append(stream, cur)
					cur = 0
				}
			}
		}
	}
	raw := rawModules(version) / 8
	if len(stream) != raw {
		return "", fmt.Errorf("%d Codewörter statt %d", len(stream), raw)
	}
	// Blöcke trennen
	nb, ecc := numECCBlocks[level][version], eccCodewordsPerBlock[level][version]
	short := nb - raw%nb
	sl := raw / nb
	blocks := make([][]byte, nb)
	k := 0
	for i := 0; i < sl-ecc+1; i++ {
		for j := 0; j < nb; j++ {
			if i < sl-ecc || j >= short {
				blocks[j] = append(blocks[j], stream[k])
				k++
			}
		}
	}
	for i := 0; i < ecc; i++ {
		for j := 0; j < nb; j++ {
			blocks[j] = append(blocks[j], stream[k])
			k++
		}
	}
	var data []byte
	for _, blk := range blocks {
		// Syndrome: das Blockpolynom an α^0 … α^(ecc-1) muss 0 sein
		alpha := byte(1)
		for i := 0; i < ecc; i++ {
			var s byte
			for _, b := range blk {
				s = gfMul(s, alpha) ^ b
			}
			if s != 0 {
				return "", fmt.Errorf("Syndrom %d ungleich 0", i)
			}
			alpha = gfMul(alpha, 2)
		}
		data = append(data, blk[:len(blk)-ecc]...)
	}
	// Byte-Modus lesen
	get := func(pos, bits int) int {
		v := 0
		for i := 0; i < bits; i++ {
			v = v<<1 | int(data[(pos+i)/8]>>(7-(pos+i)%8)&1)
		}
		return v
	}
	if get(0, 4) != 0b0100 {
		return "", fmt.Errorf("Modus %04b", get(0, 4))
	}
	cb := 8
	if version > 9 {
		cb = 16
	}
	l := get(4, cb)
	out := make([]byte, l)
	for i := range out {
		out[i] = byte(get(4+cb+8*i, 8))
	}
	return string(out), nil
}

func TestPNG(t *testing.T) {
	c, err := Encode("https://example.org/#code:7KQ4M9TX2HPA", M)
	if err != nil {
		t.Fatal(err)
	}
	b, err := c.PNG(4, 4)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	if n := (c.Size + 8) * 4; img.Bounds().Dx() != n {
		t.Fatalf("Größe %d, erwartet %d", img.Bounds().Dx(), n)
	}
	// Ecke der Ruhezone weiß, Finder oben links schwarz
	if r, _, _, _ := img.At(0, 0).RGBA(); r == 0 {
		t.Fatal("Ruhezone nicht weiß")
	}
	if r, _, _, _ := img.At(16, 16).RGBA(); r != 0 {
		t.Fatal("Finder nicht schwarz")
	}
}
