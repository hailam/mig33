package formatting

import (
	"strings"
	"testing"
)

func TestHistoricalProfileAuthoringAndPlainFallback(t *testing.T) {
	doc, err := Parse(`<body bgcolor="FF69B4" background="photo:12"><p style="color:yellow"><i>Hello &amp; welcome</i></p><img src="photo:12" width="48" height="48"><a href="mig33:joinChatroom(Anime People)">Visit room</a></body>`)
	if err != nil {
		t.Fatal(err)
	}
	if doc.Background != "#ff69b4" || doc.BackgroundImage != "photo:12" {
		t.Fatalf("document metadata: %+v", doc)
	}
	if got := PlainText(doc); got != "Hello & welcome\nVisit room" {
		t.Fatalf("plain fallback=%q", got)
	}
	if UniformColor(doc) != nil {
		t.Fatal("mixed default/yellow text claimed one color")
	}
	if doc.Nodes[0].Color != "#ffff00" || doc.Nodes[1].Width != 48 {
		t.Fatalf("nodes %+v", doc.Nodes)
	}
}

func TestFormattingRejectsUnsupportedOrAmbiguousInput(t *testing.T) {
	bad := []string{
		`<script>alert(1)</script>`,
		`<p onclick="evil">x</p>`,
		`<p style="color:red;color:blue">x</p>`,
		`<p style="position:absolute">x</p>`,
		`<a href="javascript:alert(1)">x</a>`,
		`<a href="mig33:auth()">x</a>`,
		`<a href="mig33:privateChat(bob%29)">x</a>`,
		`<img src="https://example.test/track.png">`,
		`<body background="/assets/photos/1">x</body>`,
		`<img src="photo:01">`,
		`<img src="photo:1" width="241">`,
		`<p><b>x</p></b>`,
		`<b/>`,
		`<body><body>x</body></body>`,
		`<p style="color:red" style="color:blue">x</p>`,
		`<p style=color:red>x</p>`,
		`<p title="&quot; onclick=evil">x</p>`,
		"<p>\x01</p>",
		`<p>&#8238;</p>`,
		strings.Repeat("x", MaxSource+1),
		strings.Repeat("<b>", 9) + "x" + strings.Repeat("</b>", 9),
		strings.Repeat("<br>", MaxNodes+1),
	}
	for _, source := range bad {
		t.Run(source[:min(len(source), 60)], func(t *testing.T) {
			if _, err := Parse(source); err == nil {
				t.Fatalf("accepted %q", source)
			}
		})
	}
}

func TestFormattingColorPropagationAndBBCodeStaysLiteral(t *testing.T) {
	for _, source := range []string{
		`<font color="red">hello</font>`,
		`<p style="color:#ff0000"><b>hello</b> <i>world</i></p>`,
		`<span style="color:red">a<span style="color:#ff0000">b</span></span>`,
	} {
		doc, err := Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		color := UniformColor(doc)
		if color == nil || *color != 0xff0000 {
			t.Fatalf("uniform %q: %v", source, color)
		}
	}
	for _, source := range []string{
		`<span style="color:red">a</span>b`,
		`<span style="color:red">a<span style="color:blue">b</span></span>`,
		`[color=red]hello[/color]`,
	} {
		doc, err := Parse(source)
		if err != nil {
			t.Fatal(err)
		}
		if UniformColor(doc) != nil {
			t.Fatalf("false uniform %q", source)
		}
	}
	doc, err := Parse(`[color=red]hello[/color]`)
	if err != nil || PlainText(doc) != `[color=red]hello[/color]` {
		t.Fatal("invented BBCode parser")
	}
}

func TestFormattingWalkCanResolveOwnedImages(t *testing.T) {
	doc, err := Parse(`<p><img src="photo:3"></p>`)
	if err != nil {
		t.Fatal(err)
	}
	err = Walk(&doc, func(n *Node) error {
		if n.Tag == "img" {
			n.Src = "http://127.0.0.1/assets/photos/3"
		}
		return nil
	})
	if err != nil || doc.Nodes[0].Children[0].Src != "http://127.0.0.1/assets/photos/3" {
		t.Fatal("image resolve did not mutate nested node")
	}
}
