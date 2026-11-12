package gateway

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/DIYgod/pinky/internal/screen"
)

func el(role, label string, x, y, w, h int) screen.Element {
	return screen.Element{Role: role, Label: label, Bounds: screen.Rect{X: x, Y: y, W: w, H: h}}
}

func ocr(label string, x, y int) screen.Element {
	return screen.Element{Role: screen.RoleText, Label: label, Source: screen.SourceOCR, Bounds: screen.Rect{X: x, Y: y, W: 200, H: 40}}
}

// formBehind is the review form as an iPhone once gave it: the title, the
// tab bar, and for the rest only what OCR read.
func formBehind() *screen.Screen {
	return &screen.Screen{Width: 1170, Height: 2532, App: screen.App{ID: "app.songs"}, Elements: []screen.Element{
		el(screen.RoleButton, "Song", 40, 180, 200, 60),
		el(screen.RoleText, "Write a review", 400, 180, 370, 60),
		el(screen.RoleButton, "Home", 40, 2340, 200, 80),
		ocr("3.0", 100, 500), ocr("Detailed rating", 180, 820), ocr("Thoughts", 150, 1610),
		ocr("OO", 300, 2360), // an icon in the tab bar, read as letters
	}}
}

// formWhole is the same form once the tree has caught up.
func formWhole() *screen.Screen {
	sc := formBehind()
	sc.Elements = []screen.Element{
		el(screen.RoleButton, "Song", 40, 180, 200, 60),
		el(screen.RoleText, "Write a review", 400, 180, 370, 60),
		el(screen.RoleText, "3.0", 90, 480, 220, 80),
		el(screen.RoleText, "Detailed rating", 170, 800, 300, 80),
		el(screen.RoleButton, "Thoughts", 140, 1590, 220, 80),
		el(screen.RoleButton, "Home", 40, 2340, 200, 80),
		ocr("OO", 300, 2360),
	}
	return sc
}

func TestOrphans(t *testing.T) {
	if got := strings.Join(orphans(formBehind()), "|"); got != "3.0|Detailed rating|Thoughts|OO" {
		t.Errorf("a form whose tree is behind: %s", got)
	}
	if got := strings.Join(orphans(formWhole()), "|"); got != "OO" {
		t.Errorf("the whole form: %s", got)
	}
	// Text on a picture the tree has is the picture's; a page-sized
	// element covers nothing; one letter is no text.
	sc := &screen.Screen{Width: 1000, Height: 2000, Elements: []screen.Element{
		el(screen.RoleOther, "", 0, 0, 1000, 2000),
		el(screen.RoleImage, "Poster", 100, 300, 600, 400),
		ocr("LIVE TONIGHT", 200, 450), ocr("Free", 150, 1200), ocr("X", 150, 1500),
	}}
	if got := strings.Join(orphans(sc), "|"); got != "Free" {
		t.Errorf("text on a picture: %s", got)
	}
	if orphans(nil) != nil {
		t.Error("no screen has orphans")
	}
}

func TestFinishRead(t *testing.T) {
	ctx := context.Background()
	reads := 0
	script := func(screens ...*screen.Screen) func() (*screen.Screen, error) {
		reads = 0
		return func() (*screen.Screen, error) {
			reads++
			if reads > len(screens) {
				return nil, errors.New("read more often than the test expects")
			}
			return screens[reads-1], nil
		}
	}

	// The tree was behind: one more read has all of it, and nothing was
	// learned to be drawn by the app.
	out, drawn := finishRead(ctx, formBehind(), nil, time.Millisecond, script(formWhole()))
	if treeSize(out) != 6 || reads != 1 || drawn != nil {
		t.Fatalf("a tree that was behind: %d elements after %d reads, drawn %v", treeSize(out), reads, drawn)
	}
	// A whole screen costs no read.
	if out, _ = finishRead(ctx, formWhole(), nil, time.Millisecond, script()); reads != 0 || treeSize(out) != 6 {
		t.Fatalf("a whole screen was read %d more times", reads)
	}
	// An app that draws its text itself: the second read is no bigger,
	// the text is remembered, and the same screen costs no read again.
	out, drawn = finishRead(ctx, formBehind(), nil, time.Millisecond, script(formBehind()))
	if reads != 1 || len(drawn) != 4 || treeSize(out) != 3 {
		t.Fatalf("text the app draws: %d reads, drawn %v", reads, drawn)
	}
	known := map[string]bool{}
	for _, d := range drawn {
		known[d] = true
	}
	if _, drawn = finishRead(ctx, formBehind(), known, time.Millisecond, script()); reads != 0 || drawn != nil {
		t.Fatalf("known text cost %d reads", reads)
	}
	// A tree that keeps growing is read twice at most.
	half := formBehind()
	half.Elements = append(half.Elements, el(screen.RoleText, "Rating", 90, 380, 200, 60))
	growing := formBehind()
	growing.Elements = append(growing.Elements, el(screen.RoleText, "Rating", 90, 380, 200, 60), el(screen.RoleText, "More", 90, 1000, 200, 60))
	if out, _ = finishRead(ctx, formBehind(), nil, time.Millisecond, script(half, growing, formWhole())); reads != rereadTries || treeSize(out) != 5 {
		t.Fatalf("a tree that keeps growing: %d reads, %d elements", reads, treeSize(out))
	}
	// A read that fails leaves the screen as it was.
	out, _ = finishRead(ctx, formBehind(), nil, time.Millisecond, func() (*screen.Screen, error) { return nil, errors.New("gone") })
	if treeSize(out) != 3 {
		t.Fatalf("after a failed read: %d elements", treeSize(out))
	}
}

// A screen full of elements is not read again for text in its pictures:
// that is what a feed looks like, not a tree that is behind.
func TestFinishReadLeavesFullScreensAlone(t *testing.T) {
	sc := formBehind()
	for i := 0; i < sparseTree; i++ {
		sc.Elements = append(sc.Elements, el(screen.RoleButton, "Like", 900, 300+i*60, 100, 50))
	}
	if len(orphans(sc)) < orphanLines {
		t.Fatal("the test's screen has no text without an element")
	}
	reads := 0
	out, drawn := finishRead(context.Background(), sc, nil, time.Millisecond, func() (*screen.Screen, error) {
		reads++
		return formWhole(), nil
	})
	if reads != 0 || out != sc || drawn != nil {
		t.Fatalf("a full screen was read %d more times", reads)
	}
}
